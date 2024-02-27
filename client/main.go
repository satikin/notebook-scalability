// go run main.go protocol numOfClients hostName hostPort
// go run main.go tcp 1000 localhost 8080
// go run main.go udp 1000 localhost 8081

package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"time"
	"math/rand"
	"errors"
	"os/signal"
	"io"
)

func shoot(
	protocol string,
	conf *Conf,
	succPktsCh chan int,
	errPktsCh chan int,
	scaledDown chan int,
) {
	conn, err := net.Dial(protocol, conf.host+":"+conf.port)
	defer conn.Close()
	if err != nil {
		log.Println("Dial err:", err)
		return
	}
	tout := time.NewTimer(conf.alive * time.Second)
	tic := time.NewTicker(conf.packetInterval * time.Millisecond)
	open := true
	buff := make([]byte, 128)
	succPkts := 0
	errPkts := 0
	for open == true {
		select {
		case <-tic.C:
			_, err := conn.Write([]byte("msg:12345"))
			if err != nil {
				errPkts++
				log.Println(err)
				open = false
			}
		case <-tout.C:
			succPktsCh <- succPkts
			errPktsCh <- errPkts
			open = false
		default:
			conn.SetReadDeadline(time.Now().Add(conf.maxRWait))
			_, err := conn.Read(buff)
			if err != nil {
				if errors.Is(err, os.ErrDeadlineExceeded) {
					continue
				} else if errors.Is(err, io.EOF) { // reconnect
					// log.Println(err)
					scaledDown <- 1
					time.Sleep(1*time.Second)
					go shoot(protocol, conf, succPktsCh, errPktsCh, scaledDown)
					open = false
					continue
				} else {
					log.Println(err)
					errPkts++
					open = false
					continue
				}
			} else {
				if string(buff[:5]) == "recon" {
					scaledDown <- 1
					time.Sleep(1*time.Second)
					go shoot(protocol, conf, succPktsCh, errPktsCh, scaledDown)
					open = false
					continue
				}
			}
			succPkts++
		}
	}
	conn.Close()
}

type Conf struct {
	connInterval time.Duration
	packetInterval time.Duration
	alive       time.Duration
	host        string
	port        string
	maxRWait     time.Duration
}

func main() {
	qq := make(chan os.Signal, 1)
	signal.Notify(qq, os.Interrupt)
	cnt := 0
	cntErr := 0
	doneClis := 0
	scaledClis := 0
	go func(){
		for range qq {
			printStats(cnt, cntErr, scaledClis)
			os.Exit(1)
		}
	}()
	rand.Seed(time.Now().UnixNano())
	args := os.Args[1:]
	c, _ := strconv.Atoi(args[1])
	start := time.Now()
	conf := Conf{20, 250, 60, args[2], args[3], 50*time.Millisecond}
	succPktsCh := make(chan int, 32)
	errPktsCh := make(chan int, 32)
	scaledDown := make(chan int, 32)
	for i := 0; i < c; i++ {
		// r := rand.Intn(10) time.Duration(r)
		time.Sleep(conf.connInterval * time.Millisecond)
		go shoot(args[0], &conf, succPktsCh, errPktsCh, scaledDown)
	}
	for {
		if doneClis == c {
			break
		}
		select {
		case <-scaledDown:
			c++
			scaledClis++
			doneClis++
		case n := <-succPktsCh:
			doneClis++
			cnt += n
		case n := <-errPktsCh:
			cntErr += n
		}
	}
	end := time.Now()
	diff := end.Sub(start)
	log.Printf("Took: %v", diff)
	tPkts := c * int(conf.alive)*1e3 / int(conf.packetInterval) - c
	log.Printf("Packets sent: %v",  tPkts )
	log.Printf("Received packets: %v",  cnt+cntErr )
	printStats(cnt, cntErr, scaledClis)
	
}
func printStats(cnt int, cntErr int, scaledClis int) {
	// log.Printf("Successfull: %v%%",  float64(cnt * 1e2) / float64(cnt+cntErr) )
	log.Printf("Timeout err: %v%%",  float64(cntErr * 1e2) / float64(cnt+cntErr) )
	log.Printf("Reset connections: %v",  scaledClis )
}