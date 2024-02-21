package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/IBM/sarama"
	"github.com/bradfitz/gomemcache/memcache"
	"log"
	"net"
	"os"
	"satikin.com/utils"
	"time"
)

type ServerConf struct {
	IPAddr        net.IP
	port          int
	buffSize      uint16
	workers       int
	STermTimeout  time.Duration
	MCService     string
	MCPort        int
	KafkaBrokers  []string
	KafkaUser     string
	KafkaPass     string
	KafkaLogTopic string
}

type Server struct {
	ServerConf
	log       chan string
	kafkaProd sarama.SyncProducer
	mc        *memcache.Client
}

const ERR_MSG = "Error: %v"

// initiates socket listener and the workers who listen to it
func InitServer(
	parCtx context.Context,
	srvrCfg ServerConf,
	sigTerm chan os.Signal,
	podName string,
) error {
	ctx, cancel := context.WithCancel(parCtx)
	defer cancel()
	mc, _, err := utils.InitMemcached(srvrCfg.MCService, srvrCfg.MCPort)
	if err != nil {
		return err
	}
	defer mc.Close()
	prod, err := utils.KafkaProducer(
		srvrCfg.KafkaBrokers,
		srvrCfg.KafkaUser,
		srvrCfg.KafkaPass,
	)
	if err != nil {
		return err
	}
	s := Server{
		srvrCfg,
		make(chan string, 8),
		prod,
		mc,
	}
	defer s.kafkaProd.Close()
	sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)
	go utils.KafkaLogger(
		ctx,
		s.log,
		s.kafkaProd,
		s.KafkaLogTopic,
		podName,
	)
	addr := net.UDPAddr{
		Port: srvrCfg.port,
		IP:   srvrCfg.IPAddr,
	}
	c, err := net.ListenUDP("udp4", &addr)
	if err != nil {
		s.log <- err.Error()
		return err
	}
	defer c.Close()
	// create worker pool
	for i := 0; i < srvrCfg.workers; i++ {
		go s.Worker(ctx, c)
	}
	s.log <- "Listening"
	listening := true
	for listening == true {
		select {
		case sig := <-sigTerm:
			s.log <- fmt.Sprintf("Received %v, terminating", sig)
			go func() {
				// wait a bit, k8s updating iptables / rules
				time.Sleep(srvrCfg.STermTimeout * time.Second)
				cancel()
			}()
		case <-ctx.Done():
			listening = false
		}
	}
	return nil
}

// listens to the socket for incoming messages
func (s Server) Worker(
	ctx context.Context,
	c *net.UDPConn,
) {
	listening := true
	buffer := make([]byte, s.buffSize)
	for listening == true {
		select {
		case <-ctx.Done():
			listening = false
		default:
			c.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, rAddr, err := c.ReadFromUDP(buffer)
			if err != nil && errors.Is(err, err.(*net.OpError)) {
				continue
			} else if err != nil {
				s.log <- err.Error()
				listening = false
				continue
			}
			tmpBuff := make([]byte, n)
			copy(tmpBuff, buffer[:n])
			cliInfo := Client{c, rAddr}
			go cliInfo.ProcessPacket(s.mc, tmpBuff, s.log)
		}
	}
}
