# Multi-Tier Load Balancing Architecture

In modern enterprise architectures, a single load balancer is rarely enough. Large-scale systems typically employ a **Multi-Tier Load Balancing** strategy that combines the raw speed of a Layer 4 (L4) load balancer with the intelligent routing of a Layer 7 (L7) load balancer.

This is exactly how companies like Google, Cloudflare, Facebook, and AWS handle traffic at massive scale.

## Tier 1: The L4 Load Balancers
- **What they do:** They sit at the very edge of the network. They look at nothing but IP addresses and Ports (the TCP/IP headers) and do not inspect the application payload (like HTTP headers or JSON).
- **Their job:** Absorb massive amounts of traffic, distribute connections evenly, and route the raw TCP packets to the next tier as fast as physically possible. Because they don't do expensive TLS decryption, they can process millions of packets per second.
- **Examples:** Our project (in Go), Google's Maglev, Facebook's Katran, AWS Network Load Balancer (NLB).

## Tier 2: The L7 Load Balancers
- **What they do:** The L4 LB routes traffic to a massive cluster of L7 LBs. These L7 LBs terminate the TCP connection, perform heavy TLS decryption, and actually parse the HTTP request.
- **Their job:** Read the HTTP path (e.g., `/api/payments` vs `/images`), check API keys or cookies, and intelligently route the request to the correct backend microservice.
- **Examples:** Envoy, NGINX, HAProxy, AWS Application Load Balancer (ALB).

## The Best of Both Worlds
By stacking an L4 load balancer in front of a cluster of L7 load balancers, you get the advantages of both:
1. **Infinite Scalability:** If a massive surge of traffic hits, your L4 LB effortlessly sprays the connections across hundreds of different L7 LBs.
2. **Intelligent Routing:** The L7 LBs then carefully parse the HTTP and send it to the right backend application.
3. **Security & Resiliency:** If an attacker sends a flood of junk TCP packets (DDoS), the L4 LB just drops them instantly before they can exhaust the CPU of your expensive L7 servers.

Our project focuses on building the ultra-fast Tier 1 (L4) component!
