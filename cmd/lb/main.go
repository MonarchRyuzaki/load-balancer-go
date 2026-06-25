package main

import (
	"io"
	"log"
	"net"
)

const backendAddr = "127.0.0.1:8081"
const listenAddr = ":8080"

func main() {
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
		go handleConnection(clientConn)
	}
}

func handleConnection(clientConn net.Conn) {
	defer clientConn.Close()
	log.Printf("New connection from client : %s", clientConn.RemoteAddr())

	backendAddr, err := net.Dial("tcp", backendAddr)
	if err != nil {
		log.Printf("Failed to connect to backend %s : %v", backendAddr, err)
		return
	}
	defer backendAddr.Close()

	done := make(chan struct{})

	// Client => Backend
	go func() {
		io.Copy(backendAddr, clientConn)
		done <- struct{}{}
	}()

	// Backend => Client
	go func() {
		io.Copy(clientConn, backendAddr)
		done <- struct{}{}
	}()

	<-done
	log.Printf("Connection from %s closed", clientConn.RemoteAddr())
}
