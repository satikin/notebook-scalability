package utils

import (
	"context"
	"encoding/json"
	"github.com/IBM/sarama"
	"github.com/bradfitz/gomemcache/memcache"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

type GeneralLog struct {
	Pod       string `json:"pod"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

// writes logs to kafka
func KafkaLogger(
	ctx context.Context,
	logChan chan string,
	kafkaProd sarama.SyncProducer,
	topic string,
	podName string,
) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-logChan:
			l := GeneralLog{podName, msg, time.Now().Unix()}
			b, err := json.Marshal(l)
			if err != nil {
				log.Println(err)
			}
			err = KafkaProduce(kafkaProd, topic, podName, b)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func KafkaProduce(prod sarama.SyncProducer, topic string, key string, value []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(value),
	}
	_, _, err := prod.SendMessage(msg)
	if err != nil {
		return err
	}
	return nil
}

// producer to publish messages to kafka
func KafkaProducer(
	brokers []string,
	user string,
	password string,
) (sarama.SyncProducer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true
	config.Net.SASL.Enable = true
	config.Net.SASL.Mechanism = "PLAIN"
	config.Net.SASL.User = user
	config.Net.SASL.Password = password
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}
	return producer, nil
}

// kafka consumer
func KafkaConsumer(
	brokers []string,
	user string,
	password string,
) (sarama.Consumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	config.Net.SASL.Enable = true
	config.Net.SASL.Mechanism = "PLAIN"
	config.Net.SASL.User = user
	config.Net.SASL.Password = password

	consumer, err := sarama.NewConsumer(brokers, config)
	if err != nil {
		return nil, err
	}
	return consumer, err
}

// consumes the given topic, executing the given callback on message
func KafkaConsume(
	ctx context.Context,
	consumer sarama.Consumer,
	topic string,
	OnMessage func(msg *sarama.ConsumerMessage),
) error {
	pConsumer, err := consumer.ConsumePartition(topic, 0, sarama.OffsetOldest)
	if err != nil {
		return err
	}
	active := true
	for active == true {
		select {
		case err := <-pConsumer.Errors():
			log.Println(err)
		case msg := <-pConsumer.Messages():
			OnMessage(msg)
		case <-ctx.Done():
			active = false
		}
	}
	return nil
}

// initializes memcached connection
func InitMemcached(service string, port int) (*memcache.Client, *memcache.ServerList, error) {
	portS := strconv.Itoa(port)
	ss := new(memcache.ServerList)
	ipAddrs, err := ResolveEndpoint(service)
	if err != nil {
		return nil, nil, err
	}
	for i := range ipAddrs {
		ipAddrs[i] = ipAddrs[i] + ":" + portS
	}
	ss.SetServers(ipAddrs...)
	mc := memcache.NewFromSelector(ss)
	return mc, ss, nil
}

// resolves hostnames to IP addresses
func ResolveEndpoint(endpoint string) (addrs []string, err error) {
	ips, err := net.LookupIP(endpoint)
	if err != nil {
		return nil, err
	}
	for i := range ips {
		addrs = append(addrs, ips[i].String())
	}
	return addrs, nil
}

func ParseNum(str string) int {
	num, err := strconv.Atoi(str)
	if err != nil {
		log.Fatal(err)
	}
	return num
}

func SetDefault(env string, val string) (str string) {
	if str = os.Getenv(env); str == "" {
		str = val
	}
	return str
}
