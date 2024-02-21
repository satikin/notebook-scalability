package main

import (
	"context"
	"satikin.com/utils"
	"strings"
	"log"
)

func main() {
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
	err := ConsumeLogs(context.Background(), conf)
	if err != nil {
		log.Fatal(err)
	}
}
