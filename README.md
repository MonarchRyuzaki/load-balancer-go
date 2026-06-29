# Maglev-inspired L4 Load Balancer in Go

A high-performance Layer 4 TCP Load Balancer written in Go, inspired by the architecture and algorithms of Google's Maglev and Cloudflare's Unimog.

This project was built to explore the fundamentals of Site Reliability Engineering (SRE), distributed systems, and network resiliency. It implements production-grade load balancing techniques including zero-downtime connection draining, Maglev consistent hashing, and an autonomous control plane with active health-checking and systemic oscillation protection.

## Features

- **L4 TCP Proxying:** Fast and concurrent TCP connection routing using Goroutines and `io.Copy`.
- **Google's Maglev Hashing:** Replaces standard consistent hashing rings with Google's O(1) Maglev lookup table algorithm, providing mathematically perfect load distribution and minimal disruption during backend churn.
- **Graceful Connection Draining:** When a backend is removed, existing TCP connections are allowed to finish naturally while new connections are seamlessly shifted to healthy nodes.
- **Autonomous Control Plane (Active Health Checks):** A background Goroutine actively probes backends using TCP handshakes.
- **Anti-Flapping / Debounce:** Requires consecutive health-check failures/successes before altering the routing ring, preventing false positives from network blips.
- **Systemic Oscillation Protection:** Inspired by Cloudflare's Unimog, the load balancer detects massive systemic failures (e.g., >50% backends down) and freezes the routing table to prevent cascading failures (the "thundering herd" problem).
- **Stress-Test Suite:** Includes a custom test harness capable of sustaining thousands of concurrent connections to evaluate OS-level limits (Ephemeral port exhaustion and File Descriptor caps).

## Architecture & Documentation

We have thoroughly documented the internal mechanics and design decisions made throughout the lifecycle of this project.

- **[Architecture Overview](docs/architecture.md):** The high-level design of the load balancer.
- **[Proxy Implementation](docs/proxy-implementation.md):** How the TCP proxy handles bidirectional byte copying.
- **[Connection Draining](docs/connection-draining.md):** The locking and tracking mechanism for zero-downtime deployments.
- **[Maglev vs. HashRing](docs/maglev-vs-hashring.md):** Empirical data proving the superiority of Maglev's lookup table over standard Consistent Hashing.
- **[Health Checks & Flapping](docs/health-checks-and-flapping.md):** The Unimog-inspired active prober and oscillation protection logic.
- **[Stress Test & Bottlenecks](docs/stress-test-bottlenecks.md):** An analysis of our 20k-connection stress test, explaining the physical limits of user-space proxies (FD limits and Ephemeral Port Exhaustion).

## Running the Project

You can run the components individually to see how they interact.

**1. Start the dummy Backend Servers:**
```bash
go run cmd/backend/server.go -port 8081
go run cmd/backend/server.go -port 8082
```

**2. Start the Load Balancer:**
The LB will automatically start routing traffic to the backends and actively health-check them.
```bash
go run cmd/lb/main.go
```

**3. Run the complete Stress-Test Suite:**
Alternatively, we provide a unified bash script that automatically spins up the LB, multiple backends, and a massive TCP connection spammer to stress test the system's physical limits.
```bash
# Make the startup script executable
chmod +x stress_test_startup.sh

# Run the suite
./stress_test_startup.sh
```

## Stress Test Results & Physical Limits

We built a custom TCP stress-testing suite (`cmd/stresstest`) to ramp up and hold tens of thousands of concurrent connections. Our results accurately mapped the physical limitations of a user-space TCP proxy:

```text
==================================
       FINAL STRESS RESULTS       
==================================
Target: 127.0.0.1:8080
Peak Active Connections Sustained: 7477
Total Failures / Dropped Conns : 17506
```

**Why the hard cap at ~7,500 connections?**
Because this load balancer acts as a 1:1 proxy, 7,500 client connections require 15,000 open file descriptors (sockets) on the host machine. This flawlessly demonstrated the standard Linux user `ulimit` (capped around ~15,000 FDs). Scaling past this in user-space also eventually leads to Ephemeral Port Exhaustion due to the mathematical constraints of the TCP 4-Tuple.

For a deeper dive into the math and how kernel-bypass solves these limits, see **[Stress Test & Bottlenecks](docs/stress-test-bottlenecks.md)**.

## Research & Reading Material

This project was built by directly studying the following industry papers and engineering blogs:

1. **Maglev: A Fast and Reliable Software Network Load Balancer (Google 2016)**
   - *Concepts Used:* Consistent Hashing via permutation tables, N+1 scale-out architecture.
   - [Original Paper PDF](https://research.google.com/pubs/archive/44824.pdf)
2. **Unimog: Cloudflare's edge load balancer**
   - *Concepts Used:* Active Health Checking, Debouncing, Cascading Failure/Oscillation mitigation.
   - [Original Blog Post](https://blog.cloudflare.com/unimog-cloudflares-edge-load-balancer/)

---
*Future Optimizations (Phase 5): To scale beyond the ~15,000 file descriptor limit encountered in our stress tests, the next evolution of this project would involve abandoning the user-space proxy (`io.Copy`) entirely in favor of Kernel Bypass (eBPF/XDP) and Direct Server Return (DSR) via GRE encapsulation.*
