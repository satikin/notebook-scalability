A simple Go UDP server. A worker pool is utilized to wait packets. Clients are able to save/retrieve arbitrary data in/from Memcached. Kafka is utilized to save software log to Elasticsearch through `logger/` middleware. `infrastructure/local-dev` has docker compose files to set Memcached & Kafka. `infrastructure/local-dev` has docker compose files to set Memcached & Kafka.

### Interacting

The server interacts to the following message prefixes:
- `msg:{content}`: echoes back the content
- `mcset:{token}:{key}:{value}`: sets the key/value pair in memcached if the token is valid (exists as key in memcached). Key is prefixed with "c-" to disallow clients directly writing/reading any key/value.
- `mcget:{token}:{key}`: sends to the client the value of "key" from memcached ("c-" prefixed)

To receive tokens, send a `login` message to the TCP server as described there.

Additionally
- Like the TCP server, packets are accepted for a while if `SIGTERM` has been received
- Logs are published to Kafka, `logger/` listens to the specified topic and writes them to Elasticsearch
- Scales relatively to cpu & memory utilization, count of established connections

To test it with netcat:
```
nc -u localhost 50000
```

Or run the Go application in `client/` directory, as described in its README.


# Tests

```
go test *.go
```