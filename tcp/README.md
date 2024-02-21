A simple Go TCP server (one routine per user). Clients are able to save/retrieve arbitrary data in/from Memcached & broadcast to all clients connected to this service. Kafka is utilized to forward broadcasting events to all running instances of the software (k8s pods) & log custom messages to Elasticsearch through `logger/` middleware. `infrastructure/local-dev` has docker compose files to set Memcached & Kafka.

### Interacting

The server interacts to the following message prefixes:
- `token`: generates a uuid, stores it in memcached alongside clients remote address
- `msg:{content}`: echoes back the content
- `mcset:{token}:{key}:{value}`: sets the key/value pair in memcached if the token is valid (exists as key in memcached and its value matches client's remote address). Key is prefixed with "c-" to disallow clients directly writing/reading any key/value.
- `mcget:{token}:{key}`: sends to the client the value of "key" from memcached ("c-" prefixed)
- `brd:{token}:{value}`: publishes to a Kafka topic if the token is valid (the contents are received by all running instances and are broadcasted to their connected clients)

Additionally
- Consumes a Kafka topic and sends the contents of its messages to all connected TCP clients
- Sends a specified message (`reconnect`) to all TCP connected clients if `SIGTERM` has been received, to warn clients that they need to reconnect / the pod is going to shutdown. Some time is passed before actually exiting, to give time to the load balancer to change its ip routes
- Logs are published to Kafka, `logger/` listens to the specified topic and writes them to Elasticsearch
- Scales relatively to cpu & memory utilization, count of established connections

To test it with netcat:
```
nc localhost 40000
msg:test
```

Or run the Go application in `client/` directory, as described in its README.

# Tests

```
go test *.go
```