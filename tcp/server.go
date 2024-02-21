package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/IBM/sarama"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/google/uuid"
	"log"
	"net"
	"os"
	"satikin.com/utils"
	"sync"
	"time"
)

type ServerConf struct {
	IPAddr          net.IP
	Port            int
	BuffSize        uint16 // read size buffer
	KafkaBrokers    []string
	KafkaUser       string
	KafkaPass       string
	KafkaBcastTopic string
	KafkaLogTopic   string
	KafkaMCTopic    string
	STermTimeout    time.Duration
	MCService       string
	MCPort          int
}

type Server struct {
	ServerConf
	bcastKafka chan []byte   // from kafka
	bcastCli   chan BcastMsg // to clients
	log        chan string
	kafkaProd  sarama.SyncProducer
	mc         *memcache.Client
}

type BcastMsg struct {
	clients map[string]Client
	msg     []byte
}

const ERR_MSG = "Error: %v"

// main thread, initiates listener routine
// and listens for connections. When a new connection
// is established, its info is forwarded to a goroutine
func InitServer(
	parCtx context.Context,
	srvrCfg ServerConf,
	sigTerm chan os.Signal,
	podName string,
) error {
	ctx, cancel := context.WithCancel(parCtx)
	defer cancel()

	addr := net.TCPAddr{
		Port: srvrCfg.Port,
		IP:   srvrCfg.IPAddr,
	}
	listener, err := net.ListenTCP("tcp", &addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	mc, _, err := utils.InitMemcached(srvrCfg.MCService, srvrCfg.MCPort)
	if err != nil {
		return err
	}
	defer mc.Close()
	producer, err := utils.KafkaProducer(
		srvrCfg.KafkaBrokers,
		srvrCfg.KafkaUser,
		srvrCfg.KafkaPass,
	)
	if err != nil {
		return err
	}
	defer producer.Close()
	s := Server{
		srvrCfg,
		make(chan []byte, 256),
		make(chan BcastMsg, 256),
		make(chan string, 8),
		producer,
		mc,
	}
	sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)
	go utils.KafkaLogger(
		ctx,
		s.log,
		s.kafkaProd,
		s.KafkaLogTopic,
		podName,
	)
	consumer, err := utils.KafkaConsumer(
		s.KafkaBrokers,
		s.KafkaUser,
		s.KafkaPass,
	)
	if err != nil {
		return err
	}
	defer consumer.Close()
	bcastCb := func(msg *sarama.ConsumerMessage) {
		s.bcastKafka <- msg.Value
	}
	go utils.KafkaConsume(
		ctx,
		consumer,
		s.KafkaBcastTopic,
		bcastCb,
	)
	newConn := make(chan *net.TCPConn, 16)
	finished := make(chan bool)
	go s.ConnectionListener(
		ctx,
		newConn,
		finished,
	)
	s.log <- "Listening"
	listening := true
	for listening == true {
		select {
		case <-ctx.Done():
			listening = false
		case sig := <-sigTerm:
			s.log <- fmt.Sprintf("Received %v, terminating", sig)
			go func() {
				// wait a bit, k8s updating iptables / rules
				time.Sleep(s.STermTimeout * time.Second)
				cancel()
			}()
		default:
			listener.SetDeadline(time.Now().Add(1 * time.Second))
			conn, err := listener.AcceptTCP()
			if err != nil && errors.Is(err, err.(*net.OpError)) {
				continue
			} else if err != nil {
				s.log <- fmt.Sprintf(ERR_MSG, err)
				continue
			}
			newConn <- conn
		}
	}

	s.log <- "Closing"
	<-finished
	return nil
}

// listens for net.Conn in the given channel and spawns a routine
// to process the response. Keeps a track of existing connections
func (s Server) ConnectionListener(
	ctx context.Context,
	newConn chan *net.TCPConn,
	finished chan bool,
) {
	cliDc := make(chan string, 16)
	clients := make(map[string]Client)
	go s.BroadcastWorker(ctx)
	listening := true
	for listening == true {
		select {
		case chMsg := <-s.bcastKafka:
			dupClis := make(map[string]Client)
			for k, v := range clients {
				dupClis[k] = v
			}
			bcastMsg := BcastMsg{dupClis, chMsg}
			s.bcastCli <- bcastMsg
		case <-ctx.Done():
			listening = false
		case conn := <-newConn:
			id := uuid.New().String()
			clients[id] = Client{id, conn, time.Now()}
			go s.ConnectionHandler(
				ctx,
				clients[id],
				cliDc,
			)
		case id := <-cliDc:
			(*clients[id].conn).Close()
			delete(clients, id)
		}
	}
	recPkt := []byte("reconnect")
	var wg sync.WaitGroup
	for id := range clients {
		wg.Add(1)
		go clients[id].DirectReconnect(recPkt, &wg)
	}
	wg.Wait()
	finished <- true
}

func (s Server) ConnectionHandler(
	ctx context.Context,
	cli Client,
	cliDc chan string,
) {
	defer (*cli.conn).Close()
	buffer := make([]byte, s.BuffSize)
	active := true
	// to detect "zombie" clients
	t := time.NewTicker(120 * time.Second)
	for active == true {
		select {
		case <-ctx.Done():
			active = false
		case <-t.C:
			diff := time.Now().Sub(cli.lastMsg)
			if diff.Minutes() > 10 {
				active = false
			}
		default:
			pkt, err := cli.ReadPacket(buffer)
			if err != nil && errors.Is(err, os.ErrDeadlineExceeded) {
				continue
			} else if err != nil {
				s.log <- fmt.Sprintf(ERR_MSG, err)
				active = false
				continue
			}
			cli.lastMsg = time.Now()
			err = cli.ProcessPacket(pkt, s.kafkaProd, s.KafkaBcastTopic, s.mc)
			if err != nil {
				s.log <- fmt.Sprintf(ERR_MSG, err)
				active = false
				continue
			}
		}
	}
	cliDc <- cli.id
}

// sends the contents of the channel to all connected TCP clients
func (s Server) BroadcastWorker(
	ctx context.Context,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case bcast := <-s.bcastCli:
			s.Broadcast(bcast)
		}
	}
}

func (s Server) Broadcast(bcast BcastMsg) {
	for _, cli := range bcast.clients {
		err := cli.WritePacket(bcast.msg)
		if err != nil {
			s.log <- fmt.Sprintf(ERR_MSG, err)
		}
	}
}
