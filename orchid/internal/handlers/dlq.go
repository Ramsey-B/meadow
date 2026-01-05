package handlers

import (
	"net/http"
	"strconv"

	"github.com/Gobusters/ectologger"
	"github.com/labstack/echo/v4"

	appctx "github.com/Ramsey-B/stem/pkg/context"
	"github.com/Ramsey-B/orchid/pkg/redis"
	"github.com/Ramsey-B/orchid/pkg/repositories"
)

// DLQHandler handles dead letter queue API requests
type DLQHandler struct {
	dlq      *redis.DeadLetterQueue
	streams  *redis.Streams
	jobQueue string
	logger   ectologger.Logger
}

// NewDLQHandler creates a new DLQ handler
func NewDLQHandler(
	dlq *redis.DeadLetterQueue,
	streams *redis.Streams,
	jobQueue string,
	logger ectologger.Logger,
) *DLQHandler {
	return &DLQHandler{
		dlq:      dlq,
		streams:  streams,
		jobQueue: jobQueue,
		logger:   logger,
	}
}

// DLQListResponse represents the response for listing DLQ entries
type DLQListResponse struct {
	Entries []redis.DLQEntry `json:"entries"`
	Count   int              `json:"count"`
	Total   int64            `json:"total"`
}

// List returns dead letter queue entries
// GET /api/v1/dlq
func (h *DLQHandler) List(c echo.Context) error {
	ctx := c.Request().Context()

	// Parse count parameter
	countStr := c.QueryParam("count")
	count := int64(100)
	if countStr != "" {
		if parsed, err := strconv.ParseInt(countStr, 10, 64); err == nil && parsed > 0 {
			count = parsed
		}
	}

	// Check if filtering by tenant
	tenantID := appctx.GetTenantID(ctx)
	var entries []redis.DLQEntry
	var err error

	if tenantID != "" {
		entries, err = h.dlq.ListByTenant(ctx, tenantID, count)
	} else {
		entries, err = h.dlq.List(ctx, count)
	}

	if err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to list DLQ entries")
		return err
	}

	// Get total count
	total, _ := h.dlq.Count(ctx)

	return c.JSON(http.StatusOK, DLQListResponse{
		Entries: entries,
		Count:   len(entries),
		Total:   total,
	})
}

// Get returns a specific DLQ entry
// GET /api/v1/dlq/:id
func (h *DLQHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()
	messageID := c.Param("id")

	entry, err := h.dlq.Get(ctx, messageID)
	if err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to get DLQ entry")
		return err
	}

	if entry == nil {
		return repositories.NotFound("DLQ entry %s not found", messageID)
	}

	return c.JSON(http.StatusOK, entry)
}

// Retry re-enqueues a DLQ entry
// POST /api/v1/dlq/:id/retry
func (h *DLQHandler) Retry(c echo.Context) error {
	ctx := c.Request().Context()
	messageID := c.Param("id")

	if err := h.dlq.Retry(ctx, messageID, h.streams, h.jobQueue); err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to retry DLQ entry")
		return err
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "retried",
		"message": "Job re-enqueued successfully",
	})
}

// Delete removes a DLQ entry
// DELETE /api/v1/dlq/:id
func (h *DLQHandler) Delete(c echo.Context) error {
	ctx := c.Request().Context()
	messageID := c.Param("id")

	if err := h.dlq.Delete(ctx, messageID); err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to delete DLQ entry")
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

// Stats returns DLQ statistics
// GET /api/v1/dlq/stats
func (h *DLQHandler) Stats(c echo.Context) error {
	ctx := c.Request().Context()

	count, err := h.dlq.Count(ctx)
	if err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to get DLQ stats")
		return err
	}

	return c.JSON(http.StatusOK, map[string]int64{
		"total_entries": count,
	})
}

// RegisterRoutes registers the DLQ routes
func (h *DLQHandler) RegisterRoutes(g *echo.Group) {
	dlq := g.Group("/dlq")
	dlq.GET("", h.List)
	dlq.GET("/stats", h.Stats)
	dlq.GET("/:id", h.Get)
	dlq.POST("/:id/retry", h.Retry)
	dlq.DELETE("/:id", h.Delete)
}

