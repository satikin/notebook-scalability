A simple Go TCP server (one routine per connection). Clients are able to save/retrieve arbitrary data in/from Memcached & broadcast to all clients. Kafka is utilized to forward broadcasting events to all running instances of the software (Kubernetes pods) & log custom messages to Elasticsearch through `logger/` middleware. `infrastructure/local-dev` has docker compose files to set Memcached & Kafka.

---

![Flowchart](../docs/images/tcp-server.png?raw=true "Flowchart")

## Interacting

The server interacts to the following message prefixes:
- `token`: generates a uuid, stores it in memcached alongside client's remote address
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

## Tests

```
go test *.go
```

## Autoscaling

Run the client against a minikube cluster with 2 nodes, opening 1 connection every 5 ms, sending a message for each connection every 1 second, for 240 seconds, 10000 connections. HorizontalPodAutoscaler's average value for average established connections is 5k.

As seen in the pictures, when the number of established connection was detected to be above 5k a new pod spawned. Some of the remaining connections were routed to it while the majority to the existing pod (due to the very small amount of time the client creates new connections). When the client closed its connections, the scaler decided (as configured, after 45 seconds of stabilization period) to scale down and remove the added pod.

![Add decision](../docs/images/autoscaler-tcp-add-decision.png?raw=true "Add decision")
![Added](../docs/images/autoscaler-tcp-added.png?raw=true "Added")
![Metrics](../docs/images/autoscaler-tcp-metrics.png?raw=true "Metrics")
![Scale down stabilized](../docs/images/autoscaler-tcp-remove-stabilized.png?raw=true "Scale down stabilized")
![Scale down decided](../docs/images/autoscaler-tcp-remove-decision.png?raw=true "Scale down decided")
![Scale down completed](../docs/images/autoscaler-tcp-removed.png?raw=true "Scale down completed") 