package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"satikin.com/utils"
	"strings"
	"syscall"
	"time"
)

func main() {
	podName := utils.SetDefault("POD_NAME", "ns")
	log.SetPrefix(fmt.Sprintf("[%s] ", podName))
	portS := utils.SetDefault("PORT", "40000")
	buffSizeS := utils.SetDefault("BUFF_SIZE", "512")
	mcTopic := utils.SetDefault("KAFKA_MC_TOPIC_PREFIX", "memcache-update-server")
	logTopic := utils.SetDefault("KAFKA_LOGGING_TOPIC", "general-logs")
	bcastTopic := utils.SetDefault("KAFKA_BCAST_TOPIC", "broadcasts")
	kafkaUser := utils.SetDefault("KAFKA_SASL_USER", "user")
	kafkaPass := utils.SetDefault("KAFKA_SASL_PASSWORD", "password")
	sTermTimeoutS := utils.SetDefault("SIGTERM_TIMEOUT", "20")
	brokersS := utils.SetDefault("KAFKA_BROKERS", "localhost:9092")
	mcPortS := utils.SetDefault("MEMCACHED_PORT", "11211")
	mcService := utils.SetDefault("MEMCACHED_SERVICE", "localhost")
	brokersArr := strings.Split(brokersS, ",")
	brokers := []string{}
	for i := 0; i < len(brokersArr); i++ {
		broker := brokersArr[i]
		brokers = append(brokers, broker)
	}
	port := utils.ParseNum(portS)
	buffSize := utils.ParseNum(buffSizeS)
	sTermTimeout := utils.ParseNum(sTermTimeoutS)
	mcPort := utils.ParseNum(mcPortS)
	srvrCfg := ServerConf{
		net.IPv4(0, 0, 0, 0),
		port,
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	err := InitServer(ctx, srvrCfg, sigTerm, podName)
	if err != nil {
		log.Fatal(err)
	}
}
