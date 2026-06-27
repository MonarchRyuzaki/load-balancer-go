# Multi-Tier Load Balancing Architecture

In enterprise architectures, a single load balancer is often insufficient. Large-scale systems typically employ a **Multi-Tier Load Balancing** strategy that combines the routing speed of a Layer 4 (L4) load balancer with the intelligent routing of a Layer 7 (L7) load balancer.

## Tier 1: The L4 Load Balancers
- **Function:** They sit at the edge of the network. They process IP addresses and Ports (the TCP/IP headers) and do not inspect the application payload (like HTTP headers or JSON).
- **Purpose:** Absorb traffic, distribute connections, and route raw TCP packets to the next tier efficiently. Because they do not perform TLS decryption, they can process a high volume of packets per second.
- **Examples:** Our project (in Go), Google's Maglev, Facebook's Katran, AWS Network Load Balancer (NLB).

## Tier 2: The L7 Load Balancers
- **Function:** The L4 LB routes traffic to a cluster of L7 LBs. These L7 LBs terminate the TCP connection, perform TLS decryption, and parse the HTTP request.
- **Purpose:** Read the HTTP path (e.g., `/api/payments` vs `/images`), check API keys or cookies, and route the request to the correct backend microservice.
- **Examples:** Envoy, NGINX, HAProxy, AWS Application Load Balancer (ALB).

## The Best of Both Worlds
By stacking an L4 load balancer in front of a cluster of L7 load balancers, the architecture gains the advantages of both:
1. **Scalability:** The L4 LB distributes incoming connections across numerous L7 LBs.
2. **Intelligent Routing:** The L7 LBs parse the application-layer data and send it to the correct backend service.
3. **Resiliency:** The L4 LB can drop malformed or malicious TCP packets before they consume resources on the L7 servers.

Our project focuses on building the Tier 1 (L4) component.
