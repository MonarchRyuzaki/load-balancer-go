package main

import (
	"flag"
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

func main() {
	target := flag.String("target", "127.0.0.1:8080", "The load balancer address")
	rate := flag.Int("rate", 500, "Connections to open per second")
	maxConn := flag.Int("max", 20000, "Maximum connections to attempt")
	flag.Parse()

	fmt.Printf("Starting Concurrent Connection Stress Test against %s...\n", *target)
	fmt.Printf("Ramping up at %d conn/sec until %d total connections...\n\n", *rate, *maxConn)

	var activeConnections atomic.Int64
	var failedConnections atomic.Int64
	var totalAttempted atomic.Int64

	// Print stats every second
	go func() {
		for {
			time.Sleep(1 * time.Second)
			fmt.Printf("[Stats] Active: %d | Failed: %d | Attempted: %d/%d\n", 
				activeConnections.Load(), failedConnections.Load(), totalAttempted.Load(), *maxConn)
		}
	}()

	// The rate limiter ticker
	ticker := time.NewTicker(time.Second / time.Duration(*rate))
	defer ticker.Stop()

	for i := 0; i < *maxConn; i++ {
		<-ticker.C
		totalAttempted.Add(1)
		
		go func() {
			// Attempt to dial the load balancer
			conn, err := net.DialTimeout("tcp", *target, 5*time.Second)
			if err != nil {
				failedConnections.Add(1)
				return
			}
			activeConnections.Add(1)
			
			// Write a quick byte to wake up the backend
			conn.Write([]byte("PING\n"))

			// Keep the connection held open indefinitely by reading slowly
			buf := make([]byte, 1)
			for {
				conn.SetReadDeadline(time.Now().Add(15 * time.Second))
				_, err := conn.Read(buf)
				if err != nil {
					// The connection broke or timed out
					conn.Close()
					activeConnections.Add(-1)
					failedConnections.Add(1)
					return
				}
			}
		}()
	}

	fmt.Println("\nAll connection attempts launched! Holding peak load for 10 seconds...")
	time.Sleep(10 * time.Second)
	
	fmt.Println("\n==================================")
	fmt.Println("       FINAL STRESS RESULTS       ")
	fmt.Println("==================================")
	fmt.Printf("Target: %s\n", *target)
	fmt.Printf("Peak Active Connections Sustained: %d\n", activeConnections.Load())
	fmt.Printf("Total Failures / Dropped Conns : %d\n", failedConnections.Load())
	
	if activeConnections.Load() < int64(*maxConn) {
		fmt.Println("\nNOTE: You likely hit your OS ulimit for File Descriptors (usually 1024 or 4096 on Mac/Linux).")
		fmt.Println("To test higher limits, run: `ulimit -n 65535` in your terminal before running the LB and this test.")
	}
}
