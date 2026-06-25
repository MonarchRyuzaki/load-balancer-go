package balancer

import (
	"fmt"
	"hash/crc32"
	"slices"
	"sort"
	"sync"
)

// HashRing represents a consistent hash ring.
type HashRing struct {
	Ring         []uint32
	HashToServer map[uint32]string

	mu sync.RWMutex
}

// NewHashRing creates a new HashRing.
func NewHashRing() *HashRing {
	return &HashRing{
		Ring:         make([]uint32, 0),
		HashToServer: make(map[uint32]string),
	}
}

// AddNode adds a new backend node to the hash ring.
func (h *HashRing) AddNode(backendAddr string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range 10 {
		virtual_addr := fmt.Sprintf("%v-%v", backendAddr, i)
		hashedAddr := crc32.ChecksumIEEE([]byte(virtual_addr))
		h.Ring = append(h.Ring, hashedAddr)
		h.HashToServer[hashedAddr] = backendAddr
	}
	slices.Sort(h.Ring)
}

// RemoveNode removes a backend node from the hash ring.
func (h *HashRing) RemoveNode(backendAddr string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range 10 {
		virtual_addr := fmt.Sprintf("%v-%v", backendAddr, i)
		hashedAddr := crc32.ChecksumIEEE([]byte(virtual_addr))
		delete(h.HashToServer, hashedAddr)
		h.Ring = slices.DeleteFunc(h.Ring, func(hash uint32) bool {
			return hash == hashedAddr
		})
	}
}

// GetNode gets the closest backend node for the given client IP.
func (h *HashRing) GetNode(clientIP string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.Ring) == 0 {
		return ""
	}

	hashedClientIP := crc32.ChecksumIEEE([]byte(clientIP))
	index := sort.Search(len(h.Ring), func(i int) bool {
		return h.Ring[i] >= hashedClientIP
	})
	index %= len(h.Ring)

	return h.HashToServer[h.Ring[index]]
}
