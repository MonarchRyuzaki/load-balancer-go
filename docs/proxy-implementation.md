# TCP Proxy Implementation Details

This document explains the core mechanics of our Layer 4 TCP proxy implementation in Go, specifically focusing on how we handle raw connections.

## 1. Streaming with `io.Copy`

Because we are building a **Layer 4** load balancer, we don't care what application-layer protocol the client and server are speaking (HTTP, gRPC, Redis, SSH). We just view the connection as a raw pipe of bytes.

`io.Copy(destination, source)` is Go's standard way of plumbing two pipes together. Under the hood, it creates a small memory buffer, reads bytes from the `source` socket, and immediately writes them to the `destination` socket in a highly optimized loop. It saves us from having to manually allocate buffers and write read/write loops.

## 2. Connection Lifecycle

A common misconception when moving from HTTP to raw TCP is that the proxy closes the connection after a single "request". **It actually doesn't!**

A TCP connection is a persistent, continuous stream. `io.Copy` **blocks** and runs continuously in a loop forever, streaming every byte that comes through. It only stops and returns when:
1. The client or backend intentionally closes the connection (sending a TCP FIN/EOF).
2. A network error occurs (like a timeout or dropped connection).

If you test the load balancer with a tool like `curl`, the `curl` command automatically closes the TCP connection as soon as it receives the HTTP response. Because `curl` closed the connection, `io.Copy` sees the EOF and stops. 

However, if you use a raw TCP tool like `nc` (netcat) to connect to the load balancer, the connection stays open indefinitely. You can type multiple messages, and the proxy will stream all of them back and forth over that single connection. 

We use `defer clientConn.Close()` and `defer backendConn.Close()` at the top of the connection handler so that whenever the client or server *finally* decides to hang up, we ensure both sockets are properly cleaned up so the OS doesn't leak file descriptors.

## 3. Synchronizing with the `done` Channel

Since a TCP connection is bidirectional, data can flow in both directions simultaneously. Therefore, we need two separate goroutines:
1. One goroutine streams: **Client -> Backend**
2. One goroutine streams: **Backend -> Client**

However, the main connection handler function needs to stay alive while those goroutines are running. If it returns prematurely, its `defer` statements will execute and aggressively kill the network sockets while the goroutines are still trying to use them.

We use a `done` channel to synchronize them:
- We launch the two `io.Copy` goroutines.
- The handler function hits `<-done` and **freezes**, waiting for a signal.
- The moment *either* the client or the backend hangs up the connection, their respective `io.Copy` loop finishes and pushes a signal into the channel (`done <- struct{}{}`).
- The handler function receives the signal, unfreezes, reaches the end of the function, and safely cleans up the sockets.
