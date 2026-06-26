package balancer

import (
	"fmt"
	"math/rand"
	"testing"
)

func generateRandomIPs(n int) []string {
	ips := make([]string, n)
	for i := 0; i < n; i++ {
		ips[i] = fmt.Sprintf("%d.%d.%d.%d", rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))
	}
	return ips
}

func TestDistribution(t *testing.T) {
	numBackends := 10
	numRequests := 100000

	maglev := NewMaglevRing(65537)
	hashRing := NewHashRing()

	for i := 0; i < numBackends; i++ {
		backend := fmt.Sprintf("backend-%d", i)
		maglev.AddNode(backend)
		hashRing.AddNode(backend)
	}

	ips := generateRandomIPs(numRequests)

	maglevCounts := make(map[string]int)
	hashRingCounts := make(map[string]int)

	for _, ip := range ips {
		maglevCounts[maglev.GetNode(ip)]++
		hashRingCounts[hashRing.GetNode(ip)]++
	}

	t.Logf("--- Distribution Test (%d requests, %d backends) ---", numRequests, numBackends)
	
	// Print Maglev Stats
	maglevMin, maglevMax := numRequests, 0
	for _, v := range maglevCounts {
		if v < maglevMin { maglevMin = v }
		if v > maglevMax { maglevMax = v }
	}
	t.Logf("Maglev   : min=%d, max=%d, diff=%d", maglevMin, maglevMax, maglevMax-maglevMin)

	// Print HashRing Stats
	hrMin, hrMax := numRequests, 0
	for _, v := range hashRingCounts {
		if v < hrMin { hrMin = v }
		if v > hrMax { hrMax = v }
	}
	t.Logf("HashRing : min=%d, max=%d, diff=%d", hrMin, hrMax, hrMax-hrMin)
}

func TestDisruption(t *testing.T) {
	numBackends := 1000 // Realistic cluster size from the paper
	numRequests := 100000

	maglev := NewMaglevRing(65537)
	hashRing := NewHashRing()

	backends := make([]string, numBackends)
	for i := 0; i < numBackends; i++ {
		backends[i] = fmt.Sprintf("backend-%d", i)
	}

	ips := generateRandomIPs(numRequests)

	t.Logf("--- Realistic Churn Test (%d backends) ---", numBackends)
	t.Logf("Simulating concurrent backend failures like Figure 12 in the paper...")
	t.Logf("| Failures | Maglev Changed  | HashRing Changed |")
	t.Logf("|----------|-----------------|------------------|")

	// Test 1%, 2%, 3%, 4%, 5% concurrent failures
	for percentFailed := 1; percentFailed <= 5; percentFailed++ {
		// Ensure all backends are present for baseline
		for i := 0; i < numBackends; i++ {
			maglev.AddNode(backends[i])
			hashRing.AddNode(backends[i])
		}

		// Record the baseline routing for 100,000 IPs
		maglevRoutes := make(map[string]string)
		hashRingRoutes := make(map[string]string)
		for _, ip := range ips {
			maglevRoutes[ip] = maglev.GetNode(ip)
			hashRingRoutes[ip] = hashRing.GetNode(ip)
		}

		// Calculate how many to fail
		numToFail := (numBackends * percentFailed) / 100

		// Remove backends randomly (just removing the first 'n' for simulation)
		for i := 0; i < numToFail; i++ {
			maglev.RemoveNode(backends[i])
			hashRing.RemoveNode(backends[i])
		}

		// Measure Total Disruption (including necessary changes)
		maglevTotalChanged := 0
		hashRingTotalChanged := 0

		for _, ip := range ips {
			if maglevRoutes[ip] != maglev.GetNode(ip) {
				maglevTotalChanged++
			}
			if hashRingRoutes[ip] != hashRing.GetNode(ip) {
				hashRingTotalChanged++
			}
		}

		maglevDisruptionPercent := float64(maglevTotalChanged) / float64(numRequests) * 100
		hashRingDisruptionPercent := float64(hashRingTotalChanged) / float64(numRequests) * 100

		t.Logf("|    %d%%    |      %.2f%%      |       %.2f%%      |", percentFailed, maglevDisruptionPercent, hashRingDisruptionPercent)
	}
}

func BenchmarkLookupMaglev(b *testing.B) {
	maglev := NewMaglevRing(65537)
	for i := 0; i < 100; i++ {
		maglev.AddNode(fmt.Sprintf("backend-%d", i))
	}
	ip := "192.168.1.1"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		maglev.GetNode(ip)
	}
}

func BenchmarkLookupHashRing(b *testing.B) {
	hr := NewHashRing()
	for i := 0; i < 100; i++ {
		hr.AddNode(fmt.Sprintf("backend-%d", i))
	}
	ip := "192.168.1.1"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hr.GetNode(ip)
	}
}
