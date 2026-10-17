# High-Throughput Distributed Rate Limiter & Abuse Prevention Engine

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Redis](https://img.shields.io/badge/Redis-Cluster%20%7C%20Standalone-DC382D?style=flat&logo=redis)](https://redis.io/)
[![k6 Verified](https://img.shields.io/badge/k6-Load%20Tested-7D64FF?style=flat&logo=k6)](https://k6.io/)

An ultra-low latency, distributed rate-limiting and traffic-shaping service inspired by the **Envoy Proxy Rate Limit architecture**. Engineered in Go to protect downstream microservices against cascading failures, brute-force abuse, and traffic spikes using atomic Redis Lua scripts and dynamic multi-tier descriptors.

---

## 🚀 Key Architectural Highlights

- **Atomic Sliding Window Algorithm**: Implemented directly via Redis Lua scripting to eliminate read-modify-write concurrency race conditions and hot-key contention.
- **Hierarchical Descriptor Engine**: Granular quota matching driven by dynamic YAML rules across multiple dimensions (Client IP, User Tier: Free / Pro / Enterprise, and sensitive endpoint paths).
- **Fail-Open Resilience**: Automated fallback mechanism ensures that if Redis becomes unreachable, traffic degrades gracefully without taking downstream services offline.
- **RFC Standard Compliance**: Native support for industry-standard rate-limiting headers:
  - `X-RateLimit-Limit`
  - `X-RateLimit-Remaining`
  - `Retry-After` (on `HTTP 429 Too Many Requests`)
- **Telemetry & Real-Time Observability**: Native Prometheus metrics exported at `/metrics` tracking throttle rates, RPS, and descriptor allocations.

---

## 📊 System Architecture & Data Flow

```text
[ Client Traffic ]
        │
        ▼
[ Rate Limiter Engine (Go / Gin) ]
   ├── 1. Dynamic YAML Descriptor Matcher
   ├── 2. Prometheus Metric Collector
   └── 3. Storage Layer (Redis Cluster / Standalone)
           │
           ▼ (Atomic Lua Script)
      [ Check Window Quota ]
        ├── Quota OK      ──► HTTP 200 OK + Updated Headers
        └── Limit Breached ──► HTTP 429 Too Many Requests + Retry-After

**##Benchmark Execution Summary**
checks_total.......: 111438  4456.38/s
  checks_succeeded...: 100.00% 111438 out of 111438
  ✓ status is 200 or 429
  ✓ has rate limit header

  http_req_duration..............: avg=33.04ms  med=24.36ms  p(95)=92.09ms
    { expected_response:true }...: avg=3.06ms   med=1.28ms   p(95)=7.18ms
  http_reqs......................: 55719        2228.19/s


#project structure#
distributed-rate-limiter/
├── cmd/
│   └── server/
│       └── main.go           # Server bootstrap and Gin routing
├── configs/
│   └── rules.yaml            # Rate limit quotas, tiers, and route definitions
├── internal/
│   ├── config/
│   │   └── config.go         # Dynamic YAML configuration parser
│   └── storage/
│       └── redis.go          # Redis connection pool and atomic Lua script execution
├── scripts/
│   └── benchmark.js          # k6 load testing and stress benchmark scenario
├── go.mod
├── go.sum
└── README.md          