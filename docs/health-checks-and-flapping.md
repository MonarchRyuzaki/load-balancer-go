# Active Health Checks & System Oscillation (Flapping)

*These notes are extracted from our research into Cloudflare's Unimog L4 Load Balancer and apply directly to the design of our Go-based Load Balancer's active health checking system.*

## 1. Multi-Level Health Checking
Before allowing a backend to receive traffic, a robust load balancer must perform two distinct layers of health checks:
- **Node-Level Health:** Is the physical server alive, and is the OS responding?
- **Service-Level Health:** Is the specific application (e.g., the web server or cache daemon) actually listening on its assigned port and functioning correctly?

By separating these, the load balancer can safely drain a single crashing service without pulling the entire physical machine out of the cluster.

## 2. Transient Failures and False Positives
Health checks are inherently noisy and prone to false negatives. It is highly dangerous to immediately react to a single failed ping.
- A health agent might restart for a routine software upgrade. During this restart, it might temporarily fail to report health, making the server look "dead" even though the actual application is serving traffic perfectly fine.
- **The Mitigation:** The load balancer's control plane must implement specific logic to detect and disregard these transient blips. In a custom Go implementation, this takes the form of a **"debounce" threshold**—e.g., requiring a server to fail 3 consecutive TCP pings over 10 seconds before officially declaring it dead and removing it from the routing ring.

## 3. The Flapping / Oscillation Loop (Cascading Failure)
A highly efficient load balancer can inadvertently crash an entire data center due to a dangerous feedback loop known as "flapping":
1. **Overload:** A data center receives a massive spike in traffic.
2. **Degradation:** A small group of servers hit 100% CPU. Because they are starving for CPU cycles, they fail to respond to the health check pings in time and are marked as "degraded".
3. **Redistribution:** The load balancer stops sending them new connections, dumping all that extra traffic onto the remaining "healthy" servers.
4. **Cascading Failure:** The remaining servers are instantly crushed by the extra traffic, fail their health checks, and are marked "degraded". Meanwhile, the original servers—now completely drained of new traffic—recover their CPU, start passing health checks again, and are marked "healthy".
5. **Oscillation:** The system gets stuck in a perpetual loop, violently throwing the massive traffic spike back and forth between servers as they alternate between crashing and recovering.

## 4. Mitigating Systemic Oscillation
- **The Solution:** The control plane must be able to distinguish between an **isolated degraded server** (e.g., a hardware failure on one machine) and a **data center-wide overload** (where a large percentage of servers are failing simultaneously). 
- If your Go health checker sees a massive percentage of your backends failing simultaneously, the safest fallback is to **freeze the routing ring**. It is better to blindly distribute traffic across all degraded nodes than to funnel 100% of a massive traffic spike onto a single surviving node, instantly killing it.
