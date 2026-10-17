package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"distributed-rate-limiter/internal/config"
	"distributed-rate-limiter/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Prometheus metrics for real-time observability
	requestsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ratelimiter_requests_total",
			Help: "Total requests processed across limiters",
		},
		[]string{"status", "tier"},
	)
)

func init() {
	prometheus.MustRegister(requestsProcessed)
}

func main() {
	// 1. Load Dynamic YAML Rules
	rules, err := config.LoadRules("configs/rules.yaml")
	if err != nil {
		log.Printf("Warning: Unable to load rules.yaml, using baseline defaults: %v", err)
		rules = make(map[string]config.Rule)
	}

	// 2. Initialize Redis Connection Pool
	redisStore, err := storage.NewRedisStore("localhost:6379", "", 0)
	if err != nil {
		log.Printf("Redis initialization error: %v. (Check if Redis is running)", err)
	} else {
		log.Println("Redis client connection pool established.")
	}

	router := gin.Default()

	// Metrics endpoint for Prometheus scraping
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Production Rate-Limiting Middleware
	router.Use(func(c *gin.Context) {
		// Identify user tier via header; fallback to 'free'
		tier := c.GetHeader("X-User-Tier")
		if tier == "" {
			tier = "free"
		}

		descriptor := fmt.Sprintf("tier:%s", tier)
		rule, exists := rules[descriptor]
		if !exists {
			rule = config.Rule{Limit: 20, WindowSeconds: 60}
		}

		// Composite rate limit key: IP + Tier descriptor
		clientKey := fmt.Sprintf("rl:%s:%s", c.ClientIP(), descriptor)

		// Check rate limit via Atomic Lua Script
		var result *storage.RateLimitResult
		if redisStore != nil {
			result, err = redisStore.CheckLimit(c.Request.Context(), clientKey, rule.Limit, rule.WindowDuration())
		}

		// Fail-Open Resilience Strategy: Redis failure does not trigger complete outage
		if redisStore == nil || err != nil {
			log.Printf("Degraded mode: failing-open due to storage unavailability (%v)", err)
			requestsProcessed.WithLabelValues("fail_open", tier).Inc()
			c.Next()
			return
		}

		// Inject RFC Rate-Limiting Headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(rule.Limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))

		// If quota exceeded, respond with 429 and Retry-After
		if !result.Allowed {
			requestsProcessed.WithLabelValues("throttled", tier).Inc()
			c.Header("Retry-After", strconv.FormatInt(result.RetryAfter, 10))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Too Many Requests",
				"tier":        tier,
				"retry_after": fmt.Sprintf("%d seconds", result.RetryAfter),
			})
			return
		}

		requestsProcessed.WithLabelValues("allowed", tier).Inc()
		c.Next()
	})

	// Sample protected API routes
	router.GET("/api/v1/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
		})
	})

	router.POST("/api/checkout", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Order transaction authorized",
		})
	})

	log.Println("Rate Limiter Gateway listening on port :8080...")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Server startup failure: %v", err)
	}
}