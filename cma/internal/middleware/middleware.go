package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/memora/cma/internal/metrics"
)

// RequestID injects a unique request ID into each request context and response header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.New().String()
		}
		c.Set("request_id", rid)
		c.Header("X-Request-ID", rid)
		c.Next()
	}
}

// TenantExtractor extracts the user/tenant ID from the X-Tenant-ID header
// and makes it available in the request context.
func TenantExtractor() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID != "" {
			c.Set("tenant_id", tenantID)
		}
		c.Next()
	}
}

// Logger is structured request logging middleware using slog.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		rid, _ := c.Get("request_id")

		slog.Info("request",
			"method", method,
			"path", path,
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"request_id", rid,
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
		)
	}
}

// Recovery recovers from panics and returns a 500 error.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				rid, _ := c.Get("request_id")
				slog.Error("panic recovered",
					"error", fmt.Sprintf("%v", err),
					"request_id", rid,
					"path", c.Request.URL.Path,
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":      "internal server error",
					"request_id": rid,
				})
			}
		}()
		c.Next()
	}
}

// CORS configures cross-origin resource sharing headers.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Request-ID, X-Tenant-ID")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// PrometheusMiddleware records HTTP request metrics.
func PrometheusMiddleware(m *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		method := c.Request.Method

		c.Next()

		status := fmt.Sprintf("%d", c.Writer.Status())
		duration := time.Since(start).Seconds()

		m.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		m.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
	}
}
