// Package health provides health check endpoints for the Orchid service.
package health

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// Status represents the health status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// CheckResult represents the result of a health check
type CheckResult struct {
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// Response represents a health check response
type Response struct {
	Status     Status                 `json:"status"`
	Version    string                 `json:"version,omitempty"`
	Uptime     string                 `json:"uptime,omitempty"`
	Checks     map[string]CheckResult `json:"checks,omitempty"`
	ReportedAt time.Time              `json:"reported_at"`
}

// Checker provides health check functionality
type Checker struct {
	db        *sqlx.DB
	redis     *redis.Client
	startTime time.Time
	version   string
	mu        sync.RWMutex
	ready     bool
}

// NewChecker creates a new health checker
func NewChecker(db *sqlx.DB, redisClient *redis.Client, version string) *Checker {
	return &Checker{
		db:        db,
		redis:     redisClient,
		startTime: time.Now(),
		version:   version,
		ready:     false,
	}
}

// SetReady marks the service as ready to receive traffic
func (c *Checker) SetReady(ready bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ready = ready
}

// IsReady returns whether the service is ready
func (c *Checker) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ready
}

// LivenessHandler returns the liveness probe handler
// Liveness: Is the process running and not deadlocked?
func (c *Checker) LivenessHandler(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, Response{
		Status:     StatusHealthy,
		Version:    c.version,
		Uptime:     time.Since(c.startTime).Round(time.Second).String(),
		ReportedAt: time.Now(),
	})
}

// ReadinessHandler returns the readiness probe handler
// Readiness: Is the service ready to accept traffic?
func (c *Checker) ReadinessHandler(ctx echo.Context) error {
	if !c.IsReady() {
		return ctx.JSON(http.StatusServiceUnavailable, Response{
			Status:     StatusUnhealthy,
			Version:    c.version,
			ReportedAt: time.Now(),
			Checks: map[string]CheckResult{
				"startup": {Status: StatusUnhealthy, Message: "service is still starting up"},
			},
		})
	}

	checks := c.runChecks(ctx.Request().Context())
	overallStatus := c.calculateOverallStatus(checks)

	statusCode := http.StatusOK
	if overallStatus == StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	return ctx.JSON(statusCode, Response{
		Status:     overallStatus,
		Version:    c.version,
		Uptime:     time.Since(c.startTime).Round(time.Second).String(),
		Checks:     checks,
		ReportedAt: time.Now(),
	})
}

// HealthHandler returns a detailed health check handler
func (c *Checker) HealthHandler(ctx echo.Context) error {
	checks := c.runChecks(ctx.Request().Context())
	overallStatus := c.calculateOverallStatus(checks)

	statusCode := http.StatusOK
	if overallStatus == StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	return ctx.JSON(statusCode, Response{
		Status:     overallStatus,
		Version:    c.version,
		Uptime:     time.Since(c.startTime).Round(time.Second).String(),
		Checks:     checks,
		ReportedAt: time.Now(),
	})
}

// runChecks runs all health checks
func (c *Checker) runChecks(ctx context.Context) map[string]CheckResult {
	checks := make(map[string]CheckResult)

	// Database check
	checks["database"] = c.checkDatabase(ctx)

	// Redis check
	checks["redis"] = c.checkRedis(ctx)

	return checks
}

// checkDatabase checks database connectivity
func (c *Checker) checkDatabase(ctx context.Context) CheckResult {
	if c.db == nil {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: "database not configured",
		}
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.db.PingContext(ctx); err != nil {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: err.Error(),
			Latency: time.Since(start).String(),
		}
	}

	return CheckResult{
		Status:  StatusHealthy,
		Latency: time.Since(start).String(),
	}
}

// checkRedis checks Redis connectivity
func (c *Checker) checkRedis(ctx context.Context) CheckResult {
	if c.redis == nil {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: "redis not configured",
		}
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.redis.Ping(ctx).Err(); err != nil {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: err.Error(),
			Latency: time.Since(start).String(),
		}
	}

	return CheckResult{
		Status:  StatusHealthy,
		Latency: time.Since(start).String(),
	}
}

// calculateOverallStatus determines the overall health status
func (c *Checker) calculateOverallStatus(checks map[string]CheckResult) Status {
	hasUnhealthy := false
	hasDegraded := false

	for _, check := range checks {
		switch check.Status {
		case StatusUnhealthy:
			hasUnhealthy = true
		case StatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return StatusUnhealthy
	}
	if hasDegraded {
		return StatusDegraded
	}
	return StatusHealthy
}

// RegisterRoutes registers health check routes under /api/v1
func (c *Checker) RegisterRoutes(e *echo.Echo) {
	health := e.Group("/api/v1/health")

	// Detailed health check
	health.GET("", c.HealthHandler)

	// Kubernetes-style probes
	health.GET("/live", c.LivenessHandler)
	health.GET("/ready", c.ReadinessHandler)
}

