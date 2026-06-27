package main

import (
	"load-balancer-go/internal/balancer"
	"log"
	"net"
	"time"
)

const listenAddr = ":8080"

func main() {
	pool := balancer.NewPool(balancer.NewMaglevRing(65537))

	pool.AddBackend("127.0.0.1:8081")
	pool.AddBackend("127.0.0.1:8082")
	pool.AddBackend("127.0.0.1:8083")

	pool.StartHealthCheck(3 * time.Second)

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to bind to port %s: %v", listenAddr, err)
	}
	defer listener.Close()

	log.Printf("Load Balancer listening on %s...", listenAddr)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go pool.HandleConnection(clientConn)
	}
}
