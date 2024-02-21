package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"github.com/IBM/sarama"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"log"
	"net/http"
	"os"
	"time"
)

type GeneralLogKafka struct {
	Pod       string `json:"pod"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

type GeneralLogElastic struct {
	Pod       string `json:"pod"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

type Logger struct {
	LoggerConf
	elasticCli *elasticsearch.Client
}

type LoggerConf struct {
	KafkaUser       string
	KafkaPass       string
	KafkaBrokers    []string
	KafkaLogTopic   string
	ElasticUser     string
	ElasticPass     string
	ElasticIndex    string
	ElasticEndpoint string
	IgnoreUpTo      int64
}

func ConsumeLogs(ctx context.Context, conf LoggerConf) error {
	cli, err := ElasticClient(ctx, conf)
	if err != nil {
		return err
	}
	l := Logger{
		conf,
		cli,
	}
	if l.IndexExists(ctx) == false {
		l.SetIndex(ctx)
	}
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	config.Net.SASL.Enable = true
	config.Net.SASL.Mechanism = "PLAIN"
	config.Net.SASL.User = l.KafkaUser
	config.Net.SASL.Password = l.KafkaPass
	consumer, err := sarama.NewConsumer(l.KafkaBrokers, config)
	if err != nil {
		return err
	}
	defer consumer.Close()
	pConsumer, err := consumer.ConsumePartition(l.KafkaLogTopic, 0, sarama.OffsetOldest)
	if err != nil {
		return err
	}
	active := true
	for active == true {
		select {
		case err := <-pConsumer.Errors():
			log.Println(err)
		case msg := <-pConsumer.Messages():
			var gLog GeneralLogKafka
			err := json.Unmarshal(msg.Value, &gLog)
			if err != nil {
				log.Println(err)
			}
			if time.Now().Unix()-gLog.Timestamp > l.IgnoreUpTo {
				continue
			}
			// log.Printf("received %v", gLog)
			go l.WriteLog(ctx, gLog)
		case <-ctx.Done():
			active = false
		}
	}
	return nil
}

func (l Logger) WriteLog(ctx context.Context, gLog GeneralLogKafka) {
	var elasticLog GeneralLogElastic
	elasticLog.Pod = gLog.Pod
	elasticLog.Text = gLog.Text
	elasticLog.Timestamp = time.Unix(gLog.Timestamp, 0).Format(time.RFC3339)
	logDoced, err := json.Marshal(elasticLog)
	if err != nil {
		log.Println(err)
		return
	}
	reader := bytes.NewReader(logDoced)
	req := esapi.IndexRequest{
		Index:   l.ElasticIndex,
		Body:    reader,
		Refresh: "true",
	}
	res, err := req.Do(ctx, l.elasticCli)
	if err != nil {
		log.Println(err)
		return
	}
	if res.StatusCode != 201 {
		log.Printf("Elasticsearch response status code != 201 %v", res)
	}
	defer res.Body.Close()
}

func ElasticClient(ctx context.Context, conf LoggerConf) (*elasticsearch.Client, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{
			conf.ElasticEndpoint,
		},
		Username: conf.ElasticUser,
		Password: conf.ElasticPass,
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
		return nil, err
	}
	res, err := cli.Info()
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return cli, nil
}

func (l Logger) IndexExists(ctx context.Context) bool {
	indexes := []string{
		l.ElasticIndex,
	}
	existsRes, err := l.elasticCli.Indices.Exists(
		indexes,
		l.elasticCli.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer existsRes.Body.Close()
	if existsRes.StatusCode == 200 {
		return true
	}
	return false
}

func (l Logger) SetIndex(ctx context.Context) {
	reader, err := os.Open("general-log.elasticsearch.conf.json")
	defer reader.Close()
	if err != nil {
		log.Fatal(err)
	}
	res, err := l.elasticCli.Indices.Create(
		l.ElasticIndex,
		l.elasticCli.Indices.Create.WithBody(reader),
		l.elasticCli.Indices.Create.WithContext(ctx),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	aliasIndex := []string{
		l.ElasticIndex,
	}
	aliasReq := esapi.IndicesPutAliasRequest{
		Index: aliasIndex,
		Name:  "k8s-logs",
	}
	aliasRes, err := aliasReq.Do(ctx, l.elasticCli)
	if err != nil {
		log.Fatal(err)
	}
	defer aliasRes.Body.Close()
}
