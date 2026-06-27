# Maglev vs HashRing (Empirical Test Results)

We ran a churn test comparing our Go `MaglevRing` against a standard Karger's `HashRing` (using 10 virtual nodes per backend) across a 1,000-node cluster with 100,000 simulated requests.

## 1. Load Balancing Efficiency
- **Maglev:** Distributed traffic evenly (difference of 257 requests between the most and least loaded servers).
- **HashRing:** Irregular distribution (difference of 45,793 requests). Some nodes received significantly more load while others were underutilized.

## 2. Resilience to Backend Churn
Simulating concurrent backend failures (similar to Figure 12 in the Maglev paper), we measured the percentage of traffic that had to be re-routed:

| Failures | Maglev Changed  | HashRing Changed |
|----------|-----------------|------------------|
|    1%    |      3.34%      |       0.57%      |
|    2%    |      4.67%      |       1.47%      |
|    3%    |      5.89%      |       2.62%      |
|    4%    |      7.11%      |       3.80%      |
|    5%    |      8.24%      |       4.95%      |

**Observations on HashRing Churn:** 
Standard consistent hashing guarantees zero unnecessary churn. The traffic that broke on the HashRing (e.g., 0.57% for 1% failures) belonged only to the dead nodes. The variance from exactly 1% occurred because HashRing distributes traffic unevenly; the 10 dead nodes happened to control only 0.57% of the total hash space.

**The Maglev Tradeoff:**
Maglev distributes the load evenly at the cost of minimal additional churn across healthy nodes (e.g., an extra ~2.3% for a 1% failure). When combined with an active Connection Tracking Table, this additional churn is absorbed, making it an optimal algorithm for load balancing.

## 3. Latency
- **Maglev:** `O(1)` array lookup (~53 ns/op).
- **HashRing:** `O(log N)` binary search (~60 ns/op).

Maglev avoids the binary search required by standard hash rings, offering more predictable latency scaling.
