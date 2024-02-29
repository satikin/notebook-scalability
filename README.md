# What is this

A playground with the following toys:
- **Go** programing language for the core / source code
- **Kafka** as a message broker, to achieve communication between services
- **Memcached** as a cache, to store/serve arbitrary data
- **Elasticsearch** as a software log database
- **Envoy gateway** as a gateway / load balancer
- **Kubernetes** to deploy, manage & scale the infrastructure
- **Prometheus** / **cAdvisor** to monitor the infrastructure
- **Helm** to automate the infrastructure deployment
- **ArgoCD** for continuous deployment

Being a playground, the purpose is the research of these technologies with **scalability** in mind:
- Go's concurrency is exploited to handle connections/packets in two major internet layer's communication protocols.
- The full strength of Kubernetes' horizontal scaling capabilities (combined with the correct setup of monitoring software) is extracted by taking into account not only cpu/memory utilization, but also protocol specific metrics (e.g. established TCP state or packets per second)

Each subfolder has a dedicated README:
- `tcp`, `udp` are the TCP & UDP servers' source code
- `logger` is a Kafka consumer which writes to Elasticsearch on message
- `utils` is a module with common functionality
- `infrastructure/docker` has the dockerfiles to build the images
- `infrastructure/helm-chart` is a root chart which depends on either remote (Kafka, Prometheus, cAdvisor) or local charts (defined in `charts` subdirectory - they manage the TCP, UDP servers & logger deployments)
- `client` is the application to test the servers

##

![Architecture](docs/images/architecture.png?raw=true "Architecture")


# Autoscaling

Using Kubernetes' [HorizontalPodAutoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) to add/remove pods once criteria are met.

`SIGTERM` is appropriately handled to gracefully shut down the containers.

cAdvisor is deployed in a pod in each node - it is a DaemonSet. Once a new node is added, it replicates there, and its metrics are scraped from Prometheus using the pod's label. Test it out by manually adding a node (`minikube node add -p {profile name}`)


[Envoy gateway](https://gateway.envoyproxy.io/) has been set to use "Least request"; when a new pod spawns new connections/packets are redirected to it. Its Kubernetes CRDs' were bugging as of 0.6.0; it kept a maximum of 1024 connections no matter if CircuitBreaker / BackendTrafficPolicy is set. Hence, it is deployed by a Kubernetes DaemonSet (with the required resources).

# Go tests / coverage / Sonarqube integration

Make sure Kafka, Elasticsearch, Memcached are listening. `infrastructure/local-dev` has docker compose files (edit your /etc/hostnames to match hostname kafka with Kafka's container IP address).

`go test -coverprofile cover.out *.go` in `tcp`,`udp`,`logger`,`utils` directories.


To init Sonarqube container:
```
docker run -d --name sonarqube -e SONAR_ES_BOOTSTRAP_CHECKS_DISABLE=true -p 9000:9000 sonarqube:latest
```

Login `http://localhost:9000` (admin/admin default combination), create a project, generate an access token and keep it aside. Then, in repository's root directory:
```
SONAR_TOKEN={token from GUI}
OPTS=$( cat <<-EndOfMessage
-Dsonar.projectKey=playground
-Dsonar.sources=tcp/,udp/,logger/,utils/,infrastructure/docker
-Dsonar.exclusions=**/*_test.go
-Dsonar.go.coverage.reportPaths=**/cover.out
-Dsonar.coverage.exclusions=**/*_test.go,infrastructure/docker
EndOfMessage )

docker run \
    --rm \
    -e SONAR_HOST_URL="http://localhost:9000" \
    -e SONAR_SCANNER_OPTS="$OPTS" \
    -e SONAR_TOKEN=$SONAR_TOKEN \
    -v "./:/usr/src" \
    --network=host \
    sonarsource/sonar-scanner-cli
```