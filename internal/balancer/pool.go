package balancer

import (
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Backend represents a single upstream server.
type Backend struct {
	Address string

	ActiveConnections atomic.Int64

	IsAlive              atomic.Bool
	ConsecutiveFailures  atomic.Uint32
	ConsecutiveSuccesses atomic.Uint32
}

// Ring defines the interface for a consistent hashing algorithm.
type Ring interface {
	AddNode(node string)
	RemoveNode(node string)
	GetNode(clientIP string) string
}

// Pool manages the collection of backends and routes traffic.
type Pool struct {
	// Backends is not strictly required for Consistent Hashing (we use backendMap),
	// but keeping an ordered slice is critical for future features like:
	// 1. Active Health Checking (iterating through all nodes to ping them) [Iterating on slice is faster and ordered].
	// 2. Metrics APIs (returning a clean list of servers and their load).
	// 3. Fallback routing algorithms like Round-Robin.
	Backends []*Backend
	Ring     Ring

	backendMap map[string]*Backend
	mu         sync.RWMutex

	healthCheckDone chan struct{}
}

// NewPool creates a new Backend Pool.
func NewPool(ring Ring) *Pool {
	return &Pool{
		Backends:        make([]*Backend, 0),
		Ring:            ring,
		backendMap:      make(map[string]*Backend),
		healthCheckDone: make(chan struct{}),
	}
}

// AddBackend registers a new backend server.
func (p *Pool) AddBackend(addr string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.backendMap[addr]; exists {
		return
	}

	backend := &Backend{
		Address: addr,
		IsAlive: atomic.Bool{},
	}
	backend.IsAlive.Store(true)

	p.Backends = append(p.Backends, backend)
	p.backendMap[addr] = backend
	
	p.Ring.AddNode(addr)
}

// RemoveBackend cleanly removes a backend from the routing pool (Connection Draining).
func (p *Pool) RemoveBackend(addr string) {
	p.mu.Lock()
	backend, exists := p.backendMap[addr]
	if !exists {
		p.mu.Unlock()
		return
	}

	p.Ring.RemoveNode(addr)
	p.mu.Unlock()

	go func() {
		for {
			if backend.ActiveConnections.Load() == 0 {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		p.mu.Lock()
		defer p.mu.Unlock()
		
		delete(p.backendMap, addr)
		
		for i, b := range p.Backends {
			if b.Address == addr {
				p.Backends = append(p.Backends[:i], p.Backends[i+1:]...)
				break
			}
		}
		log.Printf("Backend %s fully drained and removed from memory", addr)
	}()
}

// GetBackendForClient returns the appropriate backend for a client.
func (p *Pool) GetBackendForClient(clientIP string) *Backend {
	backendAddr := p.Ring.GetNode(clientIP)
	if backendAddr == "" {
		return nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return p.backendMap[backendAddr]
}

// StartHealthCheck begins the background active health checking loop.
func (p *Pool) StartHealthCheck(interval time.Duration) {
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-p.healthCheckDone:
				log.Println("Health checker stopped.")
				return
			case <-ticker.C:
				p.mu.RLock()
				backends := make([]*Backend, len(p.Backends))
				copy(backends, p.Backends)
				p.mu.RUnlock()

				failureCnt := 0
				deadPingThisTick := make([]*Backend, 0)

				for _, v := range backends {
					if pingBackend(v.Address) {
						if !v.IsAlive.Load() {
							if v.ConsecutiveSuccesses.Add(1) >= 3 {
								log.Printf("Backend %s recovered. Adding to ring.", v.Address)
								v.IsAlive.Store(true)
								p.Ring.AddNode(v.Address)
								v.ConsecutiveSuccesses.Store(0)
							}
						}
						v.ConsecutiveFailures.Store(0)

					} else {
						failureCnt++
						deadPingThisTick = append(deadPingThisTick, v)

						if v.IsAlive.Load() {
							v.ConsecutiveFailures.Add(1)
						}
						v.ConsecutiveSuccesses.Store(0)
					}
				}

				if failureCnt > 0 && 2*failureCnt <= len(backends) {
					for _, v := range deadPingThisTick {
						if v.IsAlive.Load() && v.ConsecutiveFailures.Load() >= 3 {
							log.Printf("Backend %s failed 3 health checks. Removing from ring.", v.Address)
							v.IsAlive.Store(false)
							p.Ring.RemoveNode(v.Address)

							// We delete from p.backendMap or p.Backends.
							// We keep pinging it so it can recover!
						}
					}
				} else if 2*failureCnt > len(backends) {
					log.Printf("PANIC: %d/%d backends failed pings. Freezing routing table to prevent oscillation.", failureCnt, len(backends))
				}
			}
		}
	}()
}

// StopHealthCheck gracefully terminates the background health checker.
func (p *Pool) StopHealthCheck() {
	close(p.healthCheckDone)
}

// pingBackend attempts a TCP handshake to see if the backend is reachable.
func pingBackend(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}
