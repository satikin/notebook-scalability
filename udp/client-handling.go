package main

import (
	"github.com/bradfitz/gomemcache/memcache"
	"net"
	"strings"
)

type Client struct {
	conn  *net.UDPConn
	rAddr *net.UDPAddr
}

const MC_ARB_PREFIX = "c-" // prefix in arbitrary data keys

func (cli Client) ProcessPacket(mc *memcache.Client, pkt []byte, logChan chan string) {
	echoStr := "msg"
	mcSetStr := "mcset"
	mcGetStr := "mcget"
	var err error
	if len(pkt) >= len(echoStr) && string(pkt[:len(echoStr)]) == echoStr {
		err = cli.Echo(pkt)
	} else if len(pkt) >= len(mcSetStr) && string(pkt[:len(mcSetStr)]) == mcSetStr {
		err = cli.SetCache(mc, pkt)
	} else if len(pkt) >= len(mcGetStr) && string(pkt[:len(mcGetStr)]) == mcGetStr {
		err = cli.GetCache(mc, pkt)
	}
	if err != nil {
		logChan <- err.Error()
	}
}

func (cli Client) Echo(pkt []byte) error {
	split := strings.Split(strings.TrimSpace(string(pkt)), ":")
	if len(split) < 2 {
		msg := []byte("error: something is missing from msg:content")
		return cli.WritePacket(msg)
	}
	_, err := cli.conn.WriteToUDP([]byte(split[1]), cli.rAddr)
	if err != nil {
		return err
	}
	return nil
}

func (cli Client) SetCache(mc *memcache.Client, pkt []byte) error {
	// mcset:token:key:value
	split := strings.Split(strings.TrimSpace(string(pkt)), ":")
	if len(split) < 4 {
		msg := []byte("error: something is missing from mcset:token:key:value")
		return cli.WritePacket(msg)
	}
	token := split[1]
	key := split[2]
	value := split[3]
	validToken := cli.ValidateToken(mc, token)
	if validToken == false {
		msg := []byte("error: invalid token")
		return cli.WritePacket(msg)
	}
	if len(key) > 64 || len(value) > 256 {
		msg := []byte("error: invalid token length")
		return cli.WritePacket(msg)
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
		msg := []byte("error: something is missing from mcget:key")
		return cli.WritePacket(msg)
	}
	token := split[1]
	key := split[2]
	if len(key) > 64 {
		msg := []byte("error: key length > 64")
		return cli.WritePacket(msg)
	}
	validToken := cli.ValidateToken(mc, token)
	if validToken == false {
		msg := []byte("error: invalid token")
		return cli.WritePacket(msg)
	}
	item, err := mc.Get(MC_ARB_PREFIX + key)
	if err != nil {
		msg := []byte("error: finding key " + key)
		return cli.WritePacket(msg)
	}
	_, err = cli.conn.WriteToUDP(item.Value, cli.rAddr)
	if err != nil {
		return err
	}
	return nil
}

// validate just existence, token issued by TCP / different remote port
func (cli Client) ValidateToken(mc *memcache.Client, token string) bool {
	_, err := mc.Get(token)
	if err != nil {
		return false
	}
	return true
}

func (cli Client) WritePacket(pkt []byte) error {
	_, err := cli.conn.WriteToUDP(pkt, cli.rAddr)
	if err != nil {
		return err
	}
	return nil
}
