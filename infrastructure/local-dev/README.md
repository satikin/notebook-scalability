Run `docker compose up` in `kafka-memcached`, `elasticsearch` & `envoy` directories to run Kafka, Memcached, Elasticsearch/Elasticvue, Envoy gateway containers. Configure `envoy/envoy.yaml` if the ports or the hostnames of tcp/udp containers are changed.

Edit your `/etc/hostnames` to match hostname kafka with Kafka's container IP address or set this address in `KAFKA_CFG_LISTENERS`, `KAFKA_CFG_ADVERTISED_LISTENERS` instead of `kafka` and run again `docker compose up`. SASL plaintext authentication is used, the credentials are set by process variables.

[How to get a container's IP address](https://stackoverflow.com/questions/17157721/how-to-get-a-docker-containers-ip-address-from-the-host):
```
docker inspect   -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' container_name_or_id
```

The gateway should be listening at the ports assigned in `envoy/docker-compose.yaml` and `envoy/envoy.yaml` (10000 TCP, 10001 UDP, 20000 admin page)

### Dev notes
Sonarqube local token `sqp_c3ebf1413bac27c11e7987417c07f819dfe1fd96`

