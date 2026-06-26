package balancer

import (
	"hash/crc32"
	"slices"
	"sync"
)

// MaglevRing implements Google's Maglev consistent hashing algorithm.
type MaglevRing struct {
	// M is the size of the lookup table. It must be a prime number.
	// Google uses 65537 in production.
	M int

	// nodes is a list of backend addresses currently in the pool.
	nodes []string

	// lookupTable is the massive array that provides O(1) routing.
	// It stores the index of the chosen backend in the `nodes` slice.
	lookupTable []int

	mu sync.RWMutex
}

// NewMaglevRing creates a new Maglev hashing ring.
// M should be a prime number (e.g., 251 for tests, 65537 for production).
func NewMaglevRing(M int) *MaglevRing {
	table := make([]int, M)
	for i := range table {
		table[i] = -1
	}

	return &MaglevRing{
		M:           M,
		nodes:       make([]string, 0),
		lookupTable: table,
	}
}

// AddNode adds a new backend to the pool and regenerates the lookup table.
func (r *MaglevRing) AddNode(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if slices.Contains(r.nodes, node) {
		return
	}

	r.nodes = append(r.nodes, node)
	r.generate()
}

// RemoveNode removes a backend and regenerates the lookup table.
func (r *MaglevRing) RemoveNode(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := 0; i < len(r.nodes); i++ {
		if r.nodes[i] == node {
			r.nodes = append(r.nodes[:i], r.nodes[i+1:]...)
			break
		}
	}

	r.generate()
}

// GetNode returns the backend address for a given client string (e.g. IP).
// This is a blazing fast O(1) array lookup.
func (r *MaglevRing) GetNode(clientIP string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.nodes) == 0 {
		return ""
	}

	slotIndex := hash1(clientIP) % uint32(r.M)
	
	backendIndex := r.lookupTable[slotIndex]
	
	return r.nodes[backendIndex]
}

// generate is the absolute core of the Maglev algorithm.
// It recalculates the permutations and perfectly populates the lookup table.
func (r *MaglevRing) generate() {
	if len(r.nodes) == 0 {
		return
	}

	// Sort nodes to ensure deterministic turn-taking regardless of addition order
	slices.Sort(r.nodes)

	N := len(r.nodes)

	// ---------------------------------------------------------
	// GENERATE PERMUTATION TABLE (See Paper Fig 3)
	// ---------------------------------------------------------
	// permutation[i][j] represents the j-th preference of backend i.
	// We use two different hash functions (h1 and h2) to compute
	// an 'offset' and 'skip' for every single node. This guarantees
	// that every backend generates a mathematically unique, random-looking
	// sequence of "preferred slots".
	permutation := make([][]uint32, N)
	for i := range N {
		permutation[i] = make([]uint32, r.M)
		offset := hash1(r.nodes[i]) % uint32(r.M)
		skip := hash2(r.nodes[i])%(uint32(r.M)-1) + 1
		for j := 0; j < r.M; j++ {
			permutation[i][j] = (offset + uint32(j)*skip) % uint32(r.M)
		}
	}

	// ---------------------------------------------------------
	// POPULATE THE LOOKUP TABLE (See Paper Fig 4)
	// ---------------------------------------------------------
	// Initialize r.lookupTable with -1 (meaning empty).
	// Loop through the backends, letting each backend claim its next
	// preferred, empty slot, until the array is 100% full.

	// next[i] tracks which preference (j) the i-th backend should try next
	next := make([]int, N)
	for i := 0; i < r.M; i++ {
		r.lookupTable[i] = -1
	}

	n := 0
	for {
		for i := range N {
			c := int(permutation[i][next[i]])
			for r.lookupTable[c] >= 0 {
				next[i]++
				c = int(permutation[i][next[i]])
			}
			r.lookupTable[c] = i
			next[i]++
			n++
			if n == r.M {
				return
			}
		}
	}
}

func hash1(node string) uint32 {
	return crc32.ChecksumIEEE([]byte(node + "-offset"))
}

func hash2(node string) uint32 {
	return crc32.ChecksumIEEE([]byte(node + "-skip"))
}
