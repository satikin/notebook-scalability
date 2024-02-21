A simple program to test UDP / TCP servers.

Receives protocol, number of connections, server ip address, server port as arguments. Has hardcoded the delay between opening connections, interval between sending messages, timeout before closing the connection.

e.g.
```
go run main.go tcp 10000 192.168.58.2 40000
go run main.go udp 10000 192.168.58.2 50000
```