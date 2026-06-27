package balancer

import (
	"fmt"
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

func TestHealthCheckDebounceFailure(t *testing.T) {
	pool := NewPool(NewMaglevRing(251))

	// Start a real backend so the failure percentage is 50% (1 out of 2),
	// otherwise our Oscillation Protection will kick in and freeze the routing table!
	realListener := mockBackendServer(t)
	defer realListener.Close()
	pool.AddBackend(realListener.Addr().String())

	// Add a fake address that will definitely fail to ping
	fakeAddr := "127.0.0.1:54321"
	pool.AddBackend(fakeAddr)

	backend := pool.backendMap[fakeAddr]
	if !backend.IsAlive.Load() {
		t.Fatal("Expected backend to be initially alive")
	}

	// Start health checking with a very fast 50ms tick
	pool.StartHealthCheck(50 * time.Millisecond)
	defer pool.StopHealthCheck()

	// Wait 250ms to allow at least 4-5 ticks
	time.Sleep(250 * time.Millisecond)

	if backend.IsAlive.Load() {
		t.Fatal("Expected backend to be marked dead after 3 consecutive failures")
	}
}

func TestHealthCheckDebounceRecovery(t *testing.T) {
	pool := NewPool(NewMaglevRing(251))

	// Start a real server so pings succeed
	l := mockBackendServer(t)
	defer l.Close()
	realAddr := l.Addr().String()

	pool.AddBackend(realAddr)

	backend := pool.backendMap[realAddr]
	// Manually simulate that it was previously marked dead
	backend.IsAlive.Store(false)

	pool.StartHealthCheck(50 * time.Millisecond)
	defer pool.StopHealthCheck()

	time.Sleep(250 * time.Millisecond)

	if !backend.IsAlive.Load() {
		t.Fatal("Expected backend to be marked alive after 3 consecutive successful pings")
	}
}

func TestHealthCheckOscillationProtection(t *testing.T) {
	pool := NewPool(NewMaglevRing(251))

	// Add 10 fake addresses that will all fail
	for i := 0; i < 10; i++ {
		pool.AddBackend(fmt.Sprintf("127.0.0.1:5000%d", i))
	}

	pool.StartHealthCheck(50 * time.Millisecond)
	defer pool.StopHealthCheck()

	time.Sleep(250 * time.Millisecond)

	// Since 10/10 (100%) of the backends failed, oscillation protection should freeze the routing table
	// and NOT mark them as dead (so it doesn't trigger RemoveNode).
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	for _, b := range pool.Backends {
		if !b.IsAlive.Load() {
			t.Fatalf("Backend %s was marked dead! Oscillation protection failed.", b.Address)
		}
	}
}
