# TCP server autoscaling

Run the client against a minikube cluster with 2 nodes, opening 1 connection every 5 ms, sending a message for each connection every 1 second, for 240 seconds, 10000 connections. HorizontalPodAutoscaler's average value for average established connections is 5k.

As seen in the pictures, when the number of established connection was detected to be above 5k a new pod spawned. Some of the remaining connections were routed to it while the majority to the existing pod (due to the very small amount of time the client creates new connections). When the client closed its connections, the scaler decided (as configured, after 45 seconds of stabilization period) to scale down and remove the added pod.

![Add decision](images/autoscaler-tcp-add-decision.png?raw=true "Add decision")
![Added](images/autoscaler-tcp-added.png?raw=true "Added")
![Metrics](images/autoscaler-tcp-metrics.png?raw=true "Metrics")
![Scale down stabilized](images/autoscaler-tcp-remove-stabilized.png?raw=true "Scale down stabilized")
![Scale down decided](images/autoscaler-tcp-remove-decision.png?raw=true "Scale down decided")
![Scale down completed](images/autoscaler-tcp-removed.png?raw=true "Scale down completed") 
