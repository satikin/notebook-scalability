package main

import (
	"github.com/IBM/sarama"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/google/uuid"
	"net"
	"strings"
	"sync"
	"time"
)

type Client struct {
	id      string
	conn    *net.TCPConn
	lastMsg time.Time
}

const MC_ARB_PREFIX = "c-" // prefix in arbitrary data keys

func (cli Client) ProcessPacket(
	pkt []byte,
	kafkaProd sarama.SyncProducer,
	topic string,
	mc *memcache.Client,
) error {
	echoStr := "msg"
	brdcStr := "brd"
	mcSetStr := "mcset"
	mcGetStr := "mcget"
	loginStr := "login"
	if len(pkt) >= len(echoStr) && string(pkt[:len(echoStr)]) == echoStr {
		return cli.Echo(pkt)
	} else if len(pkt) >= len(brdcStr) && string(pkt[:len(brdcStr)]) == brdcStr {
		return cli.Broadcast(mc, kafkaProd, topic, pkt)
	} else if len(pkt) >= len(mcSetStr) && string(pkt[:len(mcSetStr)]) == mcSetStr {
		return cli.SetCache(mc, pkt)
	} else if len(pkt) >= len(mcGetStr) && string(pkt[:len(mcGetStr)]) == mcGetStr {
		return cli.GetCache(mc, pkt)
	} else if len(pkt) >= len(loginStr) && string(pkt[:len(loginStr)]) == loginStr {
		return cli.Login(mc, pkt)
	}
	return nil
}

func (cli Client) ReadPacket(buffer []byte) (pkt []byte, err error) {
	cli.conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	n, err := cli.conn.Read(buffer)
	if err != nil {
		return nil, err
	}
	pkt = make([]byte, n)
	copy(pkt, buffer[:n])
	return pkt, nil
}

func (cli Client) WritePacket(pkt []byte) error {
	cli.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := cli.conn.Write(pkt) // LF append(pkt, byte(10))
	if err != nil {
		return err
	}
	return nil
}

func (cli Client) ValidateToken(mc *memcache.Client, token string) (bool, string) {
	tokenExists, err := mc.Get(token)
	if err != nil {
		return false, ""
	}
	rAddr := cli.conn.RemoteAddr().String()
	if string(tokenExists.Value) == rAddr {
		return true, rAddr
	}
	return false, ""
}

func (cli Client) Echo(pkt []byte) error {
	split := strings.Split(strings.TrimSpace(string(pkt)), ":")
	if len(split) < 2 {
		msg := []byte("error: something is missing from msg:content")
		return cli.WritePacket(msg)
	}
	return cli.WritePacket([]byte(split[1]))
}

func (cli Client) Broadcast(
	mc *memcache.Client,
	kafkaProd sarama.SyncProducer,
	topic string,
	pkt []byte,
) error {
	// brd:uuid-token:message
	trim := strings.TrimSpace(string(pkt))
	split := strings.Split(trim, ":")
	if len(split) < 3 {
		return cli.WritePacket([]byte("error: token or message missing, brd:token:message"))
	}
	token := split[1]
	cliMsg := split[2]
	if len(split[1]) != 36 {
		return cli.WritePacket([]byte("error: invalid token length"))
	}
	validToken, rAddr := cli.ValidateToken(mc, token)
	if validToken == false {
		return cli.WritePacket([]byte("error: invalid token, re-login"))
	}
	kMsg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(rAddr),
		Value: sarama.ByteEncoder([]byte(cliMsg)),
	}
	_, _, err := kafkaProd.SendMessage(kMsg)
	if err != nil {
		return err
	}
	return nil
}

func (cli Client) SetCache(mc *memcache.Client, pkt []byte) error {
	// mcset:token:key:value
	split := strings.Split(strings.TrimSpace(string(pkt)), ":")
	if len(split) < 4 {
		return cli.WritePacket([]byte("error: something is missing from mcset:token:key:value"))
	}
	token := split[1]
	key := split[2]
	value := split[3]
	validToken, _ := cli.ValidateToken(mc, token)
	if validToken == false {
		return cli.WritePacket([]byte("error: invalid token"))
	}
	if len(key) > 64 || len(value) > 256 {
		return cli.WritePacket([]byte("error: either key length > 64 or value length > 256"))
	}
	item := &memcache.Item{Key: MC_ARB_PREFIX + key, Value: []byte(value), Expiration: int32(360)}
	if err := mc.Set(item); err != nil {
		return err
	}
	return cli.WritePacket([]byte("success"))
}

func (cli Client) GetCache(mc *memcache.Client, pkt []byte) error {
	// mcget:token:key
	split := strings.Split(strings.TrimSpace(string(pkt)), ":")
	if len(split) < 3 {
		return cli.WritePacket([]byte("error: something is missing from mcget:key"))
	}
	token := split[1]
	key := split[2]
	if len(key) > 64 {
		return cli.WritePacket([]byte("error: key length > 64"))
	}
	validToken, _ := cli.ValidateToken(mc, token)
	if validToken == false {
		return cli.WritePacket([]byte("error: invalid token"))
	}
	item, err := mc.Get(MC_ARB_PREFIX + key)
	if err != nil {
		return cli.WritePacket([]byte("error: finding key " + key))
	}
	return cli.WritePacket(item.Value)
}

func (cli Client) Login(mc *memcache.Client, pkt []byte) error {
	// login
	token := uuid.New().String()
	rAddr := cli.conn.RemoteAddr().String()
	item := &memcache.Item{
		Key:        token,
		Value:      []byte(rAddr),
		Expiration: int32(20 * 60),
	}
	if err := mc.Set(item); err != nil {
		return err
	}
	return cli.WritePacket([]byte(token))
}

func (cli Client) DirectReconnect(recPkt []byte, wg *sync.WaitGroup) {
	cli.WritePacket(recPkt)
	time.Sleep(5 * time.Second)
	cli.conn.Close()
	wg.Done()
}
