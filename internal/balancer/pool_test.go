package balancer

import (
	"net"
	"testing"
	"time"
)

// mockBackendServer starts a simple TCP server on a random port and returns its listener
func mockBackendServer(t *testing.T) net.Listener {
	l, err := net.Listen("tcp", "127.0.0.1:0") // 0 means pick a random free port
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return // Listener closed
			}
			// Just accept and hold the connection open to simulate an active session
			go func(c net.Conn) {
				buf := make([]byte, 10)
				c.Read(buf) // Block until client closes
				c.Close()
			}(conn)
		}
	}()

	return l
}

func TestPoolDraining(t *testing.T) {
	pool := NewPool(NewMaglevRing(251))
	listeners := make([]net.Listener, 10)
	backendAddrs := make([]string, 10)

	// 1. Spin up 10 backend servers and add them to the pool
	for i := 0; i < 10; i++ {
		listeners[i] = mockBackendServer(t)
		backendAddrs[i] = listeners[i].Addr().String()
		pool.AddBackend(backendAddrs[i])
	}

	// Verify all 10 are in the pool
	pool.mu.RLock()
	if len(pool.backendMap) != 10 {
		t.Fatalf("expected 10 backends in map, got %d", len(pool.backendMap))
	}
	pool.mu.RUnlock()

	// 2. Simulate 5 active TCP connections on Backend 0
	targetBackend := backendAddrs[0]
	backend := pool.backendMap[targetBackend]
	
	for i := 0; i < 5; i++ {
		backend.ActiveConnections.Add(1)
	}

	// 3. Drop/Remove Backend 0 (Initiate the draining process)
	pool.RemoveBackend(targetBackend)

	// 4. Verify it is still in the map because it has active connections
	pool.mu.RLock()
	_, stillInMap := pool.backendMap[targetBackend]
	pool.mu.RUnlock()

	if !stillInMap {
		t.Fatalf("Backend %s should still be in map while draining", targetBackend)
	}

	// 5. Simulate the 5 connections closing naturally over time
	for i := 0; i < 5; i++ {
		backend.ActiveConnections.Add(-1)
	}

	// 6. Give the background draining goroutine a moment to poll and delete it
	// (Our goroutine polls every 100ms, so 300ms is safe)
	time.Sleep(300 * time.Millisecond)

	// 7. Verify it is fully removed from memory
	pool.mu.RLock()
	_, existsNow := pool.backendMap[targetBackend]
	pool.mu.RUnlock()

	if existsNow {
		t.Fatalf("Backend %s should have been fully deleted from map after draining", targetBackend)
	}

	// Cleanup listeners
	for _, l := range listeners {
		l.Close()
	}
}
