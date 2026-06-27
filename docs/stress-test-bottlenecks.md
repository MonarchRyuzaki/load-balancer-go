# Stress Testing & The Limits of User-Space Proxies

## The Experiment
We developed a custom TCP stress-testing suite (`cmd/stresstest/main.go`) designed to ramp up and hold tens of thousands of concurrent connections against our Go load balancer.

**The output from the test:**
```text
Starting Concurrent Connection Stress Test against 127.0.0.1:8080...
Ramping up at 500 conn/sec until 20000 total connections...

[Stats] Active: 499 | Failed: 0 | Attempted: 500/20000
...
[Stats] Active: 6979 | Failed: 0 | Attempted: 6979/20000
[Stats] Active: 7477 | Failed: 2 | Attempted: 7480/20000
[Stats] Active: 7471 | Failed: 500 | Attempted: 7972/20000
[Stats] Active: 7465 | Failed: 1003 | Attempted: 8468/20000
...
[Stats] Active: 7469 | Failed: 12461 | Attempted: 19931/20000

==================================
       FINAL STRESS RESULTS       
==================================
Target: 127.0.0.1:8080
Peak Active Connections Sustained: 7477
Total Failures / Dropped Conns : 17506
```
*Note: The test consistently bottlenecks and caps at exactly ~7,477 active connections.*

## Why does it fail at ~7,500 connections?

In our current Phase 4 architecture, the Load Balancer acts as a **Proxy** (using `io.Copy`). For every 1 logical connection a client makes to the Load Balancer, the Load Balancer has to open 1 downstream connection to the Backend. 

This means **7,500 client connections = 15,000 open file descriptors (sockets)** on the Load Balancer process. 

Linux has strict limits on how many open files/sockets a single process can hold (the `ulimit`). Even if a start-up script attempts to raise it (`ulimit -n 65535`), standard users usually hit a hard cap enforced by the OS (often around 16,384 or 4,096 depending on the distro). Once the LB process hits that FD cap, the OS refuses to allocate any more sockets, and all subsequent `net.Dial` or `net.Accept` calls fail.

## Is the system strictly bounded by the 65,536 port limit?

Yes and no. It is bounded by Ephemeral Port Exhaustion, but the limit is actually much lower than 65k.

A TCP connection is uniquely identified by a **4-Tuple**:
`{Source IP, Source Port, Destination IP, Destination Port}`

When our Load Balancer connects to Backend 1:
- Source IP: `127.0.0.1` (Fixed)
- Dest IP: `127.0.0.1` (Fixed)
- Dest Port: `8081` (Fixed)

The *only* variable left to differentiate connections is the **Source Port** (Ephemeral Port). Mathematically, because ports are 16-bit integers, there are 65,535 possible ports. However, the Linux kernel restricts the ephemeral port range (usually `32768` to `60999`). This means the load balancer actually only has about **~28,000 available ports** per backend IP. 

If hundreds of thousands of clients try to connect, a proxy-based load balancer will literally run out of ephemeral ports to talk to the backend, resulting in complete exhaustion.

## The Solution: Kernel Bypass and Encapsulation
These physical limitations of user-space proxying are exactly why hyperscalers like Google (Maglev) and Cloudflare (Unimog) abandoned this architecture.

To break past the File Descriptor limit and the Ephemeral Port Exhaustion limit, modern L4 load balancers utilize **eBPF/XDP** and **Direct Server Return (DSR)**:
1. **No Sockets:** The Load Balancer *never establishes a TCP connection*. It processes packets at the NIC driver level.
2. **Encapsulation:** It wraps the incoming packet in a GRE or GUE tunnel and forwards it.
3. **DSR:** The backend unpacks the tunnel and replies *directly* to the client, bypassing the load balancer on the return path.

Because the Load Balancer doesn't open standard POSIX sockets, it uses zero file descriptors and zero local ephemeral ports, allowing it to scale to tens of millions of concurrent connections on commodity hardware.
