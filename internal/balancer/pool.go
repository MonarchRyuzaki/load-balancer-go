package balancer

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Backend represents a single upstream server.
type Backend struct {
	Address string

	ActiveConnections atomic.Int64
}

// Pool manages the collection of backends and routes traffic.
type Pool struct {
	// Backends is not strictly required for Consistent Hashing (we use backendMap), 
	// but keeping an ordered slice is critical for future features like:
	// 1. Active Health Checking (iterating through all nodes to ping them) [Iterating on slice is faster and ordered].
	// 2. Metrics APIs (returning a clean list of servers and their load).
	// 3. Fallback routing algorithms like Round-Robin.
	Backends []*Backend
	Ring     *HashRing

	backendMap map[string]*Backend
	mu         sync.RWMutex
}

// NewPool creates a new Backend Pool.
func NewPool() *Pool {
	return &Pool{
		Backends:   make([]*Backend, 0),
		Ring:       NewHashRing(),
		backendMap: make(map[string]*Backend),
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
	}

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
