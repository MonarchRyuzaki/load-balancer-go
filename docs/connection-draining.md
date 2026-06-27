# Connection Draining & Graceful Shutdown

When managing a Layer 4 load balancer pool, a critical question arises when a backend server needs to be removed (e.g., for maintenance, scaling down, or failure): **Can we redistribute the active clients on that server to another healthy server?**

The answer is **No**. 

This document explains why we cannot transfer live TCP connections and how we solve this problem using Connection Draining.

## The TCP State Problem

A TCP connection is a stateful agreement tightly bound to two specific machines. It is defined by the "4-tuple" (Source IP, Source Port, Destination IP, Destination Port) and relies on highly specific sequence and acknowledgment numbers negotiated during the initial 3-way handshake.

If Client X is downloading a file from Server A, the load balancer cannot route Client X's packets to Server B. Server B lacks the memory of the TCP handshake, does not know the expected sequence numbers, and does not have the application state. If it receives these packets, the TCP stack will reject them and send a TCP RST (Reset), severing the user's connection.

## The Solution: Connection Draining

Because we cannot move live connections, we must prevent them from being severed. We achieve this using a two-step process called **Graceful Shutdown** or **Connection Draining**:

### 1. Remove from the Hash Ring (Drain Mode)
When a server is flagged for removal, the load balancer deletes it from the consistent hashing ring. This guarantees that zero new connections will be routed to this server.

### 2. Wait for Active Sessions to Finish
The load balancer maintains a connection tracking map (e.g., a `sync.Map`) that counts how many active TCP sessions are currently tied to each backend server. 

Even though the server is out of the hash ring, the load balancer continues to proxy the *existing* active TCP connections. It simply waits. As clients finish their downloads or naturally close their apps, the active connection count drops.

### 3. Safe Shutdown
Only when the active connection count for that server hits `0` is the server completely decommissioned and shut down. This ensures maintenance can be performed without disconnecting active users.
