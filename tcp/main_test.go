package main

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
	"time"
	// "log"
	// "fmt"
	"os/signal"
	"satikin.com/utils"
)

func TestMain(t *testing.T) {
	os.Setenv("SIGTERM_TIMEOUT", "5")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port := "40000"
	os.Setenv("PORT", port)
	go main()
	time.Sleep(2 * time.Second)

	// connection Close testing
	servAddr := "localhost:" + port
	conn, err := net.Dial("tcp", servAddr)
	if err != nil {
		t.Fatal("Dial failed:", err.Error())
	}
	err = conn.Close()
	if err != nil {
		t.Fatal(err)
	}

	// messages testing
	conn = TCPCli(ctx, t, port)
	// "reconnect" functionality when SIGTERM
	go func() {
		buffer := make([]byte, 32)
		conn.SetReadDeadline(time.Now().Add(1 * time.Minute))
		n, err := conn.Read(buffer)
		if err != nil {
			t.Fatalf("Error reading 'reconnect' %v", err)
		}
		if string(buffer[:n]) != "reconnect" {
			t.Fatalf("Unexepected response while waiting reconnect msg: %s", string(buffer[:n]))
		}
		conn.Close()
		cancel()
	}()
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	<-ctx.Done()
	time.Sleep(3 * time.Second)
}

// getters/setters/responses
func TCPCli(ctx context.Context, t *testing.T, port string) net.Conn {
	servAddr := "localhost:" + port
	conn, err := net.Dial("tcp", servAddr)
	if err != nil {
		t.Fatal("Dial failed:", err.Error())
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

	// login
	tokenB := make([]byte, 36)
	if tokenB, err = WriteRead(conn, "login", 36, buffer); err != nil {
		t.Fatal(err)
	}
	token := string(tokenB)

	// broadcast
	if _, err := WriteRead(conn, "brd:"+token+":abcd", "abcd", buffer); err != nil {
		t.Fatal(err)
	}
	// broadcast wrong arg
	if _, err := WriteRead(conn, "brd:"+token, "error", buffer); err != nil {
		t.Fatal(err)
	}
	// broadcast wrong token length
	if _, err := WriteRead(conn, "brd:dsadsa:3132", "error", buffer); err != nil {
		t.Fatal(err)
	}
	// broadcast invalid token
	invToken := "fake-36-digits-uuid-1234567891213456"
	if _, err := WriteRead(conn, "brd:"+invToken+":3132", "error", buffer); err != nil {
		t.Fatal(err)
	}

	// mcset
	if _, err := WriteRead(conn, "mcset:"+token+":abcd:123", "success", buffer); err != nil {
		t.Fatal(err)
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

	// random packet / should not expect a response
	if _, err := WriteRead(conn, "whatever", "", buffer); err == nil {
		t.Fatal("Should not get a response after sending a non-command")
	}
	return conn
}

func WriteRead(conn net.Conn, out string, expected interface{}, buffer []byte) ([]byte, error) {
	_, err := conn.Write([]byte(out))
	conn.SetWriteDeadline(time.Now().Add(50 * time.Millisecond))
	if err != nil {
		return nil, err // t.Fatalf("Write %s wrong args: ", err.Error())
	}
	conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
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

func SetConfig() ServerConf {
	buffSizeS := utils.SetDefault("BUFF_SIZE", "512")
	mcTopic := utils.SetDefault("KAFKA_MC_TOPIC_PREFIX", "memcache-update-server")
	logTopic := utils.SetDefault("KAFKA_LOGGING_TOPIC", "general-logs")
	bcastTopic := utils.SetDefault("KAFKA_BCAST_TOPIC", "broadcasts")
	kafkaUser := utils.SetDefault("KAFKA_SASL_USER", "user")
	kafkaPass := utils.SetDefault("KAFKA_SASL_PASSWORD", "password")
	sTermTimeoutS := utils.SetDefault("SIGTERM_TIMEOUT", "20")
	brokers := []string{"localhost:9092"}
	mcPortS := utils.SetDefault("MEMCACHED_PORT", "11211")
	mcService := utils.SetDefault("MEMCACHED_SERVICE", "localhost")
	buffSize := utils.ParseNum(buffSizeS)
	sTermTimeout := utils.ParseNum(sTermTimeoutS)
	mcPort := utils.ParseNum(mcPortS)
	srvrCfg := ServerConf{
		net.IPv4(0, 0, 0, 0),
		40000,
		uint16(buffSize),
		brokers,
		kafkaUser,
		kafkaPass,
		bcastTopic,
		logTopic,
		mcTopic,
		time.Duration(sTermTimeout),
		mcService,
		mcPort,
	}
	return srvrCfg
}

func TestMainWrongListener(t *testing.T) {
	conf := SetConfig()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	conf.Port = 1
	err := InitServer(ctx, conf, sigTerm, "ns")
	if err == nil {
		t.Fatal("Should not be able to listen on port 1")
	}
}

func TestMainWrongMemcached(t *testing.T) {
	conf := SetConfig()
	ctx, cancel := context.WithCancel(context.Background())
	conf.Port = 40001
	conf.MCService = "localhost2"
	defer cancel()
	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	err := InitServer(ctx, conf, sigTerm, "ns")
	if err == nil {
		t.Fatal("Should not be able to connect to localhost2 Memcached")
	}
}

func TestMainWrongKafka(t *testing.T) {
	conf := SetConfig()
	ctx, cancel := context.WithCancel(context.Background())
	conf.Port = 40002
	conf.KafkaBrokers = []string{"localhost23:9092"}
	defer cancel()
	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	err := InitServer(ctx, conf, sigTerm, "ns")
	if err == nil {
		t.Fatal("Should not be able to connect to localhost2 Kafka broker")
	}
}
