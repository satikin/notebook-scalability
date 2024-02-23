# Init with minikube

- `playground` cluster, 4 cpus, 4096 memory, using calico container network interface (to do the external load balancing)
```
minikube -p playground start --cpus 4 --memory 4096 --cni calico --nodes 2 # --network-plugin=cni
```
- metrics server (cpu/memory monitoring)
```
minikube -p playground addons enable metrics-server
``` 
- Kubernetes dashboard
```
minikube -p playground dashboard
```
- build the TCP & UDP server docker images:
```
docker build --build-arg APP_DIR=tcp -f infrastructure/docker/Dockerfile -t tcp-server:1.0.1
docker build --build-arg APP_DIR=udp -f infrastructure/docker/Dockerfile -t udp-server:1.0.1
docker build --build-arg APP_DIR=logger -f infrastructure/docker/Dockerfile.logger -t logger:1.0.1
```
- load the images from local registry to minikube
```
minikube -p playground image load --overwrite=true docker.io/library/tcp-server:1.0.1
minikube -p playground image load --overwrite=true docker.io/library/udp-server:1.0.1
minikube -p playground image load --overwrite=true docker.io/library/logger:1.0.1
```
- tunnel connections / use external load balancing
```
minikube -p playground tunnel
```


# Configuring

- `infrastructure/helm-chart/values.yaml` is the entry point; subcharts can be configured from there.

## Kubernetes Prometheus stack

Most of [kube-prometheus-stack](https://artifacthub.io/packages/helm/prometheus-community/kube-prometheus-stack) default services & scraping targets are disabled to decrease resource utilization. Prometheus adapter is configured to scrape cAdvisor pod(s if multiple nodes) and serve the metric to HorizontalPodAutoscaler. As seen in root chart's `values.yaml` the adapter is fetching `container_network_tcp6_usage_total` & selects `tcp_state=established` to calculate the rate for TCP. For UDP it uses `container_network_receive_packets_total`.


## cAdvisor

Only `network`, `tcp`, `udp` [metrics](https://github.com/google/cadvisor/blob/master/docs/storage/prometheus.md#prometheus-container-metrics) are enabled to decrease resource utilization.

It is deployed to the main namespace due to the remote chart's `namespaceOverride` absence.


# Deploying with Helm

- create monitoring namespace
```
kubectl create namespace monitoring
```
- create functional namespace
```
kubectl create namespace playground
```
- alias helm and kubectl commands for current terminal session:
```
alias k="kubectl --namespace playground"
alias h="helm --namespace playground"
```
- (required because eck-operator-crds need to be installed) comment everything in `templates/ek.yaml`
- build Helm dependencies and deploy, in `infrastructure/helm-chart`:
```
h dependency build
h install playground .
```
- uncomment contents of `templates/ek.yaml` and redeploy:
```
h upgrade playground .
```

To access Kibana GUI:
- `k get secret playground-es-elastic-user -o=jsonpath='{.data.elastic}' | base64 --decode; echo` to get the password
- `k port-forward services/playground-kb-http 5601`
- navigate to `https://localhost:5601`, login as user `elastic` with the password from 1st step

# ArgoCD installation simplified instructions

- installed OS client
- followed https://argo-cd.readthedocs.io/en/stable/getting_started/ using forward and not ingress (skipped steps 3-5):
    - `kubectl create namespace argocd`
    - `kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml`
    - `argocd login --core`
    - `argocd admin initial-password -n argocd`, copy password
    - `kubectl port-forward svc/argocd-server -n argocd 8080:443`
- opened localhost:8080
- set up repository using private key
- created app / synced (make sure playground/monitoring namespaces are created)

# Versions

- Docker: `24.0.7`
- Kubernetes: `v1.28.3`
- kubectl: `v1.29.1`
- Helm: `v3.13.3`
- Minikube: `v1.32.0`