package balancer

import (
	"io"
	"log"
	"net"
)

// HandleConnection proxies the TCP connection from client to the chosen backend.
func (p *Pool) HandleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()
	log.Printf("New connection from client: %s", clientAddr)

	// We use just the IP address (stripping the port) so that all 
	// connections from the same user hash to the exact same backend!
	clientIP, _, err := net.SplitHostPort(clientAddr)
	if err != nil {
		clientIP = clientAddr
	}

	backend := p.GetBackendForClient(clientIP)
	if backend == nil {
		log.Printf("No backend available for client: %s", clientAddr)
		return
	}

	backendConn, err := net.Dial("tcp", backend.Address)
	if err != nil {
		log.Printf("Failed to connect to backend %s: %v", backend.Address, err)
		return
	}
	defer backendConn.Close()

	backend.ActiveConnections.Add(1)
	
	defer backend.ActiveConnections.Add(-1)

	done := make(chan struct{})

	// Copy from Client to Backend
	go func() {
		io.Copy(backendConn, clientConn)
		done <- struct{}{}
	}()

	// Copy from Backend to Client
	go func() {
		io.Copy(clientConn, backendConn)
		done <- struct{}{}
	}()

	<-done
	log.Printf("Connection from %s to %s closed", clientAddr, backend.Address)
}
