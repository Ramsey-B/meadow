package health

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// Checker handles health check endpoints
type Checker struct {
	db        *sqlx.DB
	redis     interface{ Ping() error }
	version   string
	startTime time.Time
	ready     atomic.Bool
}

// NewChecker creates a new health checker
func NewChecker(db *sqlx.DB, redis interface{ Ping() error }, version string) *Checker {
	return &Checker{
		db:        db,
		redis:     redis,
		version:   version,
		startTime: time.Now(),
	}
}

// SetReady sets the readiness state
func (c *Checker) SetReady(ready bool) {
	c.ready.Store(ready)
}

// RegisterRoutes registers health check endpoints
func (c *Checker) RegisterRoutes(e *echo.Echo) {
	e.GET("/api/v1/health", c.Health)
	e.GET("/api/v1/health/live", c.Live)
	e.GET("/api/v1/health/ready", c.Ready)
}

// HealthStatus represents the health check response
type HealthStatus struct {
	Status     string                  `json:"status"`
	Version    string                  `json:"version"`
	Uptime     string                  `json:"uptime"`
	Checks     map[string]*CheckResult `json:"checks"`
	ReportedAt time.Time               `json:"reported_at"`
}

// CheckResult represents an individual check result
type CheckResult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// Health returns the overall health status
func (c *Checker) Health(ctx echo.Context) error {
	status := &HealthStatus{
		Status:     "healthy",
		Version:    c.version,
		Uptime:     time.Since(c.startTime).Round(time.Second).String(),
		Checks:     make(map[string]*CheckResult),
		ReportedAt: time.Now(),
	}

	// Check database
	if c.db != nil {
		start := time.Now()
		err := c.db.Ping()
		latency := time.Since(start)

		if err != nil {
			status.Status = "unhealthy"
			status.Checks["database"] = &CheckResult{
				Status:  "unhealthy",
				Message: err.Error(),
			}
		} else {
			status.Checks["database"] = &CheckResult{
				Status:  "healthy",
				Latency: latency.String(),
			}
		}
	} else {
		status.Status = "unhealthy"
		status.Checks["database"] = &CheckResult{
			Status:  "unhealthy",
			Message: "database not configured",
		}
	}

	// Check Redis if configured
	if c.redis != nil {
		start := time.Now()
		err := c.redis.Ping()
		latency := time.Since(start)

		if err != nil {
			status.Status = "unhealthy"
			status.Checks["redis"] = &CheckResult{
				Status:  "unhealthy",
				Message: err.Error(),
			}
		} else {
			status.Checks["redis"] = &CheckResult{
				Status:  "healthy",
				Latency: latency.String(),
			}
		}
	}

	httpStatus := http.StatusOK
	if status.Status == "unhealthy" {
		httpStatus = http.StatusServiceUnavailable
	}

	return ctx.JSON(httpStatus, status)
}

// Live returns the liveness status (is the service running)
func (c *Checker) Live(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, map[string]string{"status": "alive"})
}

// Ready returns the readiness status (is the service ready to accept traffic)
func (c *Checker) Ready(ctx echo.Context) error {
	if c.ready.Load() {
		return ctx.JSON(http.StatusOK, map[string]string{"status": "ready"})
	}
	return ctx.JSON(http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
}

