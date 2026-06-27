# TCP Proxy Implementation Details

This document explains the core mechanics of our Layer 4 TCP proxy implementation in Go, specifically focusing on how we handle raw connections.

## 1. Streaming with `io.Copy`

Because we are building a Layer 4 load balancer, we do not inspect the application-layer protocol the client and server are speaking (HTTP, gRPC, Redis, SSH). We view the connection as a raw pipe of bytes.

`io.Copy(destination, source)` is Go's standard way of connecting two pipes. Under the hood, it creates a memory buffer, reads bytes from the `source` socket, and writes them to the `destination` socket in an optimized loop. This avoids the need to manually allocate buffers and write read/write loops.

## 2. Connection Lifecycle

A common misconception when moving from HTTP to raw TCP is that the proxy closes the connection after a single request. This is incorrect.

A TCP connection is a persistent stream. `io.Copy` blocks and runs continuously in a loop, streaming every byte that comes through. It only stops and returns when:
1. The client or backend intentionally closes the connection (sending a TCP FIN/EOF).
2. A network error occurs (like a timeout or dropped connection).

When testing the load balancer with a tool like `curl`, the `curl` command automatically closes the TCP connection as soon as it receives the HTTP response. Because `curl` closed the connection, `io.Copy` receives the EOF and stops. 

However, when using a raw TCP tool like `nc` (netcat) to connect to the load balancer, the connection stays open indefinitely. Multiple messages can be sent, and the proxy will stream them back and forth over that single connection. 

We use `defer clientConn.Close()` and `defer backendConn.Close()` at the top of the connection handler to ensure both sockets are properly cleaned up when the client or server closes the connection, preventing the OS from leaking file descriptors.

## 3. Synchronizing with the `done` Channel

Since a TCP connection is bidirectional, data can flow in both directions simultaneously. Therefore, we need two separate goroutines:
1. One goroutine streams: **Client -> Backend**
2. One goroutine streams: **Backend -> Client**

However, the main connection handler function needs to stay active while those goroutines are running. If it returns prematurely, its `defer` statements will execute and close the network sockets while the goroutines are still trying to use them.

We use a `done` channel to synchronize them:
- We launch the two `io.Copy` goroutines.
- The handler function reaches `<-done` and waits for a signal.
- When either the client or the backend closes the connection, their respective `io.Copy` loop finishes and sends a signal into the channel (`done <- struct{}{}`).
- The handler function receives the signal, reaches the end of the function, and safely cleans up the sockets.
