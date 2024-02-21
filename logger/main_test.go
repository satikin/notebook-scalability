package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"github.com/IBM/sarama"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"log"
	"net/http"
	"os"
	"satikin.com/utils"
	"strings"
	"testing"
	"time"
)

func DeleteTestIndex(ctx context.Context, index string, endpoint string, user string, pass string) error {
	cfg := elasticsearch.Config{
		Addresses: []string{endpoint},
		Username:  user,
		Password:  pass,
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   10,
			ResponseHeaderTimeout: 10 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			},
		},
	}
	cli, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return err
	}
	res, err := cli.Info()
	if err != nil {
		return err
	}
	defer res.Body.Close()
	req := esapi.IndicesDeleteRequest{
		Index: []string{index},
	}
	res, err = req.Do(ctx, cli)
	if err != nil {
		return err
	}
	return nil
	// cli.Indices.Delete(index)
	// res.Body.Close()
	// cli.Close()
}

func TestMain(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	endpoint := "https://localhost:9200"
	index := "general-logs"
	topic := "test-logs"
	kafkaUser := "user"
	kafkaPass := "password"
	elasticUser := "elastic"
	elasticPassword := "password"
	os.Setenv("KAFKA_LOGGING_TOPIC", topic)
	os.Setenv("ELASTICSEARCH_ENDPOINT", endpoint)
	os.Setenv("ELASTICSEARCH_LOGGING_INDEX", index)
	os.Setenv("ELASTICSEARCH_USERNAME", elasticUser)
	os.Setenv("ELASTICSEARCH_PASSWORD", elasticPassword)
	os.Setenv("KAFKA_SASL_USER", kafkaUser)
	os.Setenv("KAFKA_SASL_PASSWORD", kafkaPass)
	err := DeleteTestIndex(ctx, index, endpoint, elasticUser, elasticPassword)
	if err != nil {
		t.Fatal(err)
	}
	go main()
	brokers := []string{"localhost:9092"}
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true
	config.Net.SASL.Enable = true
	config.Net.SASL.Mechanism = "PLAIN"
	config.Net.SASL.User = kafkaUser
	config.Net.SASL.Password = kafkaPass
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		log.Panic(err)
	}
	gLog := GeneralLogKafka{
		"ns-test",
		"test-text",
		time.Now().Unix(),
	}
	b, err := json.Marshal(gLog)
	if err != nil {
		t.Fatal(err)
	}
	kMsg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder("tests"),
		Value: sarama.ByteEncoder(b),
	}
	_, _, err = producer.SendMessage(kMsg)
	if err != nil {
		t.Fatal(err)
	}
	<-ctx.Done()
}

func SetConfig() LoggerConf {
	logTopic := utils.SetDefault("KAFKA_LOGGING_TOPIC", "general-logs")
	kafkaUser := utils.SetDefault("KAFKA_SASL_USER", "user")
	kafkaPass := utils.SetDefault("KAFKA_SASL_PASSWORD", "password")
	kafkaBrokersS := utils.SetDefault("KAFKA_BROKERS", "localhost:9092")
	elasticEndpoint := utils.SetDefault("ELASTICSEARCH_ENDPOINT", "https://localhost:9200")
	elasticIndex := utils.SetDefault("ELASTICSEARCH_LOGGING_INDEX", "general-logs")
	ignoreUpToS := utils.SetDefault("IGNORE_MESSAGES_BEFORE", "30")
	elasticUser := utils.SetDefault("ELASTICSEARCH_USERNAME", "elastic")
	elasticPass := utils.SetDefault("ELASTICSEARCH_PASSWORD", "password")
	brokersSplit := strings.Split(kafkaBrokersS, ",")
	brokers := []string{}
	for i := 0; i < len(brokersSplit); i++ {
		broker := brokersSplit[i]
		brokers = append(brokers, broker)
	}
	ignoreUpTo := utils.ParseNum(ignoreUpToS)
	conf := LoggerConf{
		kafkaUser,
		kafkaPass,
		brokers,
		logTopic,
		elasticUser,
		elasticPass,
		elasticIndex,
		elasticEndpoint,
		int64(ignoreUpTo),
	}
	return conf
}

func TestMainWrongKafka(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf := SetConfig()
	conf.ElasticEndpoint = "https://localhost213:9200"
	err := ConsumeLogs(ctx, conf)
	if err == nil {
		t.Fatal("Should not be able to connect to Elasticsearch / https://localhost213:9200")
	}
}

func TestMainWrongElastic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf := SetConfig()
	conf.KafkaBrokers = []string{"localhost23:9092"}
	err := ConsumeLogs(ctx, conf)
	if err == nil {
		t.Fatal("Should not be able to connect to Kafka / localhost23:9092")
	}
}
