package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
)

func main() {
	port := flag.String("port", "8081", "Port to run the backend server on")
	flag.Parse()

	addr := ":" + *port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}
	defer listener.Close()

	log.Printf("Backend server listening on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go handleConnection(conn, *port)
	}
}

func handleConnection(conn net.Conn, port string) {
	defer conn.Close()
	log.Printf("Received connection from %s", conn.RemoteAddr())

	welcomeMsg := fmt.Sprintf("Hello! You have reached the backend server on port %s\n", port)
	conn.Write([]byte(welcomeMsg))

	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from connection: %v", err)
			}
			break
		}

		response := fmt.Sprintf("[Echo from port %s]: %s", port, string(buf[:n]))
		_, err = conn.Write([]byte(response))
		if err != nil {
			log.Printf("Error writing to connection: %v", err)
			break
		}
	}
	log.Printf("Connection from %s closed", conn.RemoteAddr())
}
