# Active Health Checks & System Oscillation (Flapping)

*These notes are extracted from our research into Cloudflare's Unimog L4 Load Balancer and apply directly to the design of our Go-based Load Balancer's active health checking system.*

## 1. Multi-Level Health Checking
Before allowing a backend to receive traffic, a robust load balancer must perform two distinct layers of health checks:
- **Node-Level Health:** Is the physical server alive, and is the OS responding?
- **Service-Level Health:** Is the specific application (e.g., the web server or cache daemon) listening on its assigned port and functioning correctly?

By separating these, the load balancer can drain a single crashing service without pulling the entire physical machine out of the cluster.

## 2. Transient Failures and False Positives
Health checks are prone to false negatives. It is inefficient to react to a single failed ping.
- A health agent might restart for a software upgrade. During this restart, it might temporarily fail to report health, making the server appear unresponsive even though the application is serving traffic.
- **The Mitigation:** The load balancer's control plane must implement logic to detect and disregard these transient blips. In our Go implementation, this takes the form of a **"debounce" threshold**—e.g., requiring a server to fail 3 consecutive TCP pings over 10 seconds before officially declaring it dead and removing it from the routing ring.

## 3. The Flapping / Oscillation Loop (Cascading Failure)
A load balancer can inadvertently impact an entire data center due to a feedback loop known as "flapping":
1. **Overload:** A data center receives a spike in traffic.
2. **Degradation:** A small group of servers hit 100% CPU. Because they are starving for CPU cycles, they fail to respond to the health check pings in time and are marked as "degraded".
3. **Redistribution:** The load balancer stops sending them new connections, distributing the traffic onto the remaining "healthy" servers.
4. **Cascading Failure:** The remaining servers receive the extra traffic, fail their health checks, and are marked "degraded". Meanwhile, the original servers—now drained of new traffic—recover their CPU, start passing health checks again, and are marked "healthy".
5. **Oscillation:** The system gets stuck in a perpetual loop, distributing the traffic spike back and forth between servers as they alternate between crashing and recovering.

## 4. Mitigating Systemic Oscillation
- **The Solution:** The control plane must distinguish between an **isolated degraded server** (e.g., a hardware failure on one machine) and a **data center-wide overload** (where a large percentage of servers are failing simultaneously). 
- If our Go health checker observes a high percentage of our backends failing simultaneously, the fallback is to **freeze the routing ring**. Distributing traffic across all degraded nodes prevents funneling all traffic onto a single surviving node.
