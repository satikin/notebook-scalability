package utils

import (
	"context"
	"encoding/json"
	"github.com/IBM/sarama"
	"os"
	"testing"
	"time"
)

const USER = "user"
const PASS = "password"
const TOPIC = "tests"

func TestKafkaProducer(t *testing.T) {
	brokers := []string{"localhost:9092"}
	prod, err := KafkaProducer(brokers, USER, PASS)
	if err != nil {
		t.Fatal(err)
	}
	defer prod.Close()
}

func TestKafkaConsumer(t *testing.T) {
	brokers := []string{"localhost:9092"}
	consumer, err := KafkaConsumer(brokers, USER, PASS)
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()
}

func TestKafkaLogger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	brokers := []string{"localhost:9092"}
	msgBody := "test-log"
	prod, err := KafkaProducer(brokers, USER, PASS)
	if err != nil {
		t.Fatal(err)
	}
	defer prod.Close()
	consumer, err := KafkaConsumer(brokers, USER, PASS)
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()
	consumed := make(chan bool)
	go KafkaConsume(ctx, consumer, TOPIC, func(msg *sarama.ConsumerMessage) {
		var gL GeneralLog
		if err := json.Unmarshal(msg.Value, &gL); err != nil {
			t.Fatal(err)
		}
		if string(gL.Text) != msgBody {
			t.Fatalf("Consumed message %s is different from sent %s", string(msg.Value), msgBody)
		}
		consumed <- true
	})
	logCh := make(chan string, 2)
	go KafkaLogger(ctx, logCh, prod, TOPIC, "tests-pod")
	logCh <- msgBody
	<-consumed
}

func TestInitMemcached(t *testing.T) {
	mc, _, err := InitMemcached("localhost", 11211)
	if err != nil {
		t.Fatal(err)
	}
	defer mc.Close()

	mc, _, err = InitMemcached("localhost1111", 11211)
	if err == nil {
		mc.Close()
		t.Fatal("Should not have resolved localhost1111")
	}
}

func TestKafkaProducerWrongConf(t *testing.T) {
	brokers := []string{"localhost1:9092"}
	prod, err := KafkaProducer(brokers, USER, PASS)
	if err == nil {
		prod.Close()
		t.Fatal("Shoud not have created a producer / localhost1:9092")
	}
}

func TestKafkaConsumerWrongConf(t *testing.T) {
	brokers := []string{"localhost1:9092"}
	consumer, err := KafkaConsumer(brokers, USER, PASS)
	if err == nil {
		consumer.Close()
		t.Fatal("Shoud not have created a consumer / localhost1:9092")
	}
}

func TestKafkaConsumerCancelCtx(t *testing.T) {
	brokers := []string{"localhost:9092"}
	ctx, cancel := context.WithCancel(context.Background())
	consumer, err := KafkaConsumer(brokers, USER, PASS)
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()
	go KafkaConsume(ctx, consumer, TOPIC, func(msg *sarama.ConsumerMessage) {})
	time.Sleep(250 * time.Millisecond)
	cancel()
	<-ctx.Done()
}

func TestKafkaLoggerCancelCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	brokers := []string{"localhost:9092"}
	prod, err := KafkaProducer(brokers, USER, PASS)
	if err != nil {
		t.Fatal(err)
	}
	defer prod.Close()
	go KafkaLogger(ctx, make(chan string), prod, TOPIC, "tests-pod")
	time.Sleep(250 * time.Millisecond)
	cancel()
	<-ctx.Done()
}

func TestSetDefault(t *testing.T) {
	varName := "test-env-process"
	v := SetDefault(varName, "str")
	if v != "str" {
		t.Fatalf("SetDefault: Not setting default values in empty process variables")
	}
	varVal := "test-val"
	os.Setenv(varName, varVal)
	v = SetDefault("test-env-process", "str")
	if v != varVal {
		t.Fatalf("SetDefault: Does not keep existing values in process variables")
	}
}

func TestParseNum(t *testing.T) {
	if ParseNum("123") != 123 {
		t.Fatalf("ParseNum: Does not parse strings to int")
	}
}
