# Layer 4 Load Balancer Architecture

## Proxy Mode vs Passthrough Mode

When building a Layer 4 Load Balancer, there are two primary architectural approaches to handling traffic:

### 1. Passthrough Mode (DSR / NAT)
- **How it works:** The load balancer does not terminate the TCP connection. Instead, it inspects the incoming packet, rewrites the destination MAC/IP address to point to a backend server, and forwards the packet directly.
- **Pros:** Fast packet processing. The return traffic from the backend can bypass the load balancer entirely (Direct Server Return - DSR).
- **Cons:** Less flexible. The load balancer cannot easily inspect connection state, gracefully handle complex retries, or terminate TLS.

### 2. Proxy Mode
- **How it works:** The load balancer terminates the client's TCP connection, establishes a second, independent TCP connection to the chosen backend server, and copies the data back and forth between the two connections.
- **Pros:** High flexibility. Enables dynamic connection tracking, advanced routing algorithms (like Maglev), active health checking, and connection draining. 
- **Cons:** Slower than passthrough. Data must be copied from the kernel space to user space (the Go application) and back to the kernel space for every packet, resulting in CPU context-switching overhead.

*Note: We implemented **Proxy Mode** in this project to prioritize learning dynamic routing, state management, and SRE resilience patterns.*

## Advanced eBPF Optimizations (Phase 5)

While pure proxy mode in user space is slower due to context switching, we can leverage advanced Linux kernel features to optimize it.

### eBPF and `sockmap`
eBPF (Extended Berkeley Packet Filter) allows sandboxed programs to run safely within the Linux kernel. While high-performance load balancers use eBPF/XDP for *passthrough* routing at the network driver level, we can use a specific eBPF feature called `sockmap` to optimize our *proxy*.

When our Go proxy accepts a client connection and dials a backend connection, it typically uses `io.Copy` to move the data. This means data goes:
`Kernel -> User Space (Go) -> Kernel`

With eBPF `sockmap`, we can inform the kernel that the client TCP socket and the backend TCP socket are linked. The kernel will then stream the data directly between the two sockets internally:
`Kernel -> Kernel`

This bypasses the Go user space entirely for the payload data transfer, reducing CPU context switches while maintaining the flexible architecture of a Proxy-mode load balancer.
