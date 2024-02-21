package main

import (
	"context"
	"errors"
	"github.com/bradfitz/gomemcache/memcache"
	"net"
	"os"
	"satikin.com/utils"
	"strconv"
	"syscall"
	"testing"
	"time"
	// "log"
)

func TestMain(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	port := "50000"
	mcHost := "localhost"
	mcPort := "11211"
	graceTime := "5"
	os.Setenv("PORT", port)
	os.Setenv("MEMCACHED_SERVICE", mcHost)
	os.Setenv("MEMCACHED_PORT", mcPort)
	os.Setenv("SIGTERM_TIMEOUT", graceTime)

	go main()

	time.Sleep(1 * time.Second)
	UDPSuite(ctx, t, port, mcHost, mcPort)
	// "reconnect" functionality when SIGTERM
	time.Sleep(1 * time.Second)
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	<-ctx.Done()
}

// getters/setters/responses
func UDPSuite(ctx context.Context, t *testing.T, port string, mcHost string, mcPort string) net.Conn {
	servAddr := "localhost:" + port
	conn, err := net.Dial("udp", servAddr)
	if err != nil {
		t.Fatalf("Dial failed: %s", err.Error())
	}
	token, err := SetTestToken(mcHost, mcPort, conn.LocalAddr().String())
	if err != nil {
		t.Fatalf("Error setting token in Memcached %s", err.Error())
	}
	buffer := make([]byte, 64)
	time.Sleep(250 * time.Millisecond)
	// echo
	if _, err := WriteRead(conn, "msg:123", "123", buffer); err != nil {
		t.Fatal(err)
	}
	// echo wrong arg
	if _, err := WriteRead(conn, "msg", "error", buffer); err != nil {
		t.Fatal(err)
	}

	// mcset
	if b, err := WriteRead(conn, "mcset:"+token+":abcd:123", "success", buffer); err != nil {
		t.Fatalf("%s, buffer: %s", err.Error(), string(b))
	}
	// mcset wrong arg
	if _, err := WriteRead(conn, "mcset:"+token, "error", buffer); err != nil {
		t.Fatal(err)
	}
	// mcset long key
	longKey := "12345678912134561234567891213456123456789121345612345678912134561234567891213456"
	if _, err := WriteRead(conn, "mcset:"+token+":"+longKey+":"+"a", "error", buffer); err != nil {
		t.Fatal(err)
	}
	// mcset long value
	longValue := longKey + longKey + longKey + longKey + longKey
	if _, err := WriteRead(conn, "mcset:"+token+":long-val:"+longValue, "error", buffer); err != nil {
		t.Fatal(err)
	}
	// mcset invalid token
	if _, err := WriteRead(conn, "mcset:whatever:abs:123", "error", buffer); err != nil {
		t.Fatal(err)
	}

	// mcget
	if _, err := WriteRead(conn, "mcget:"+token+":abcd", "123", buffer); err != nil {
		t.Fatal(err)
	}
	// mcget unexisting key
	if _, err := WriteRead(conn, "mcget:"+token+":whatever123", "error", buffer); err != nil {
		t.Fatal(err)
	}
	// mcget wrong arg
	if _, err := WriteRead(conn, "mcget:"+token, "error", buffer); err != nil {
		t.Fatal(err)
	}
	// mcget long key
	if _, err := WriteRead(conn, "mcget:"+token+":"+longKey, "error", buffer); err != nil {
		t.Fatal(err)
	}
	// mcget invalid token
	if _, err := WriteRead(conn, "mcget:whatever:abs:123", "error", buffer); err != nil {
		t.Fatal(err)
	}
	return conn
}

func WriteRead(conn net.Conn, out string, expected interface{}, buffer []byte) ([]byte, error) {
	_, err := conn.Write([]byte(out))
	conn.SetWriteDeadline(time.Now().Add(150 * time.Millisecond))
	if err != nil {
		return nil, err // t.Fatalf("Write %s wrong args: ", err.Error())
	}
	conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	n, err := conn.Read(buffer)
	if err != nil {
		return nil, err // t.Fatal("Read %s wrong args:", err.Error())
	}
	switch expected.(type) {
	case string:
		if string(buffer[:len(expected.(string))]) != expected {
			return nil, errors.New("Unexpected response")
			// t.Fatalf("Read %s wrong arg: unexpected response: %s", string(buff[:n]))
		}
		return buffer[:n], nil
	case int:
		if len(string(buffer[:n])) != expected {
			return nil, errors.New("Unexpected response size")
		}
		return buffer[:n], nil
	}
	return nil, nil
}

func SetTestToken(mcHost string, mcPort string, lAddr string) (string, error) {
	port, err := strconv.Atoi(mcPort)
	if err != nil {
		return "", err
	}
	mc, _, err := utils.InitMemcached(mcHost, port)
	if err != nil {
		return "", err
	}
	token := "fake-36-digits-uuid-1234567891213456"
	item := &memcache.Item{Key: token, Value: []byte(lAddr), Expiration: int32(60)}
	if err := mc.Set(item); err != nil {
		return "", err
	}
	return token, nil
}
