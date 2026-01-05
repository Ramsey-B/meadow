package handlers

import (
	"net/http"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/Ramsey-B/orchid/pkg/queue"
	"github.com/Ramsey-B/orchid/pkg/redis"
	"github.com/Ramsey-B/orchid/pkg/repositories"
	appctx "github.com/Ramsey-B/stem/pkg/context"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

// PlanHandler handles plan API endpoints
type PlanHandler struct {
	repo     repositories.PlanRepo
	streams  *redis.Streams
	jobQueue string
	logger   ectologger.Logger
}

// NewPlanHandler creates a new plan handler
func NewPlanHandler(
	repo repositories.PlanRepo,
	streams *redis.Streams,
	jobQueue string,
	logger ectologger.Logger,
) *PlanHandler {
	return &PlanHandler{
		repo:     repo,
		streams:  streams,
		jobQueue: jobQueue,
		logger:   logger,
	}
}

// CreatePlanRequest represents the create plan request body
type CreatePlanRequest struct {
	IntegrationID  string         `json:"integration_id" validate:"required"`
	Key            string         `json:"key" validate:"required"`
	Name           string         `json:"name" validate:"required"`
	Description    *string        `json:"description,omitempty"`
	PlanDefinition map[string]any `json:"plan_definition" validate:"required"`
	Enabled        *bool          `json:"enabled,omitempty"`
	WaitSeconds    *int           `json:"wait_seconds,omitempty"`
	RepeatCount    *int           `json:"repeat_count,omitempty"`
}

// UpdatePlanRequest represents the update plan request body
type UpdatePlanRequest struct {
	Key            string         `json:"key" validate:"required"`
	Name           string         `json:"name" validate:"required"`
	Description    *string        `json:"description,omitempty"`
	PlanDefinition map[string]any `json:"plan_definition" validate:"required"`
	Enabled        *bool          `json:"enabled,omitempty"`
	WaitSeconds    *int           `json:"wait_seconds,omitempty"`
	RepeatCount    *int           `json:"repeat_count,omitempty"`
}

// TriggerPlanRequest represents the trigger plan request body
type TriggerPlanRequest struct {
	ConfigID        string         `json:"config_id" validate:"required"`
	ContextOverride map[string]any `json:"context_override,omitempty"`
}

// SetEnabledRequest represents the set enabled request body
type SetEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

// Register registers plan routes
func (h *PlanHandler) Register(g *echo.Group) {
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:key", h.GetByID)
	g.PUT("/:key", h.Update)
	g.DELETE("/:key", h.Delete)
	g.PATCH("/:key/enabled", h.SetEnabled)
	g.POST("/:key/trigger", h.Trigger)
}

// List returns all plans for the current tenant
func (h *PlanHandler) List(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "PlanHandler.List")
	defer span.End()
	c.SetRequest(c.Request().WithContext(ctx))

	integrationIDStr := c.QueryParam("integration_id")
	if integrationIDStr != "" {
		integrationID, err := uuid.Parse(integrationIDStr)
		if err != nil {
			return BadRequest("invalid integration_id")
		}
		plans, err := h.repo.ListByIntegration(ctx, integrationID)
		if err != nil {
			h.logger.WithContext(ctx).WithError(err).Error("Failed to list plans by integration")
			return err
		}
		return SuccessResponse(c, plans)
	}

	enabledOnly := c.QueryParam("enabled") == "true"
	if enabledOnly {
		plans, err := h.repo.ListEnabled(ctx)
		if err != nil {
			h.logger.WithContext(ctx).WithError(err).Error("Failed to list enabled plans")
			return err
		}
		return SuccessResponse(c, plans)
	}

	return BadRequest("integration_id or enabled=true query parameter is required")
}

// Create creates a new plan
func (h *PlanHandler) Create(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "PlanHandler.Create")
	defer span.End()
	c.SetRequest(c.Request().WithContext(ctx))

	var req CreatePlanRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest("invalid request body")
	}

	if req.Name == "" || req.IntegrationID == "" {
		return BadRequest("name and integration_id are required")
	}

	integrationID, err := uuid.Parse(req.IntegrationID)
	if err != nil {
		return BadRequest("invalid integration_id")
	}

	plan := &models.Plan{
		IntegrationID: integrationID,
		Key:           req.Key,
		Name:          req.Name,
		Description:   req.Description,
		Enabled:       false,
		WaitSeconds:   req.WaitSeconds,
		RepeatCount:   req.RepeatCount,
	}
	plan.PlanDefinition.Data = req.PlanDefinition

	if req.Enabled != nil {
		plan.Enabled = *req.Enabled
	}

	if err := h.repo.Create(ctx, plan); err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to create plan")
		return err
	}

	h.logger.WithContext(ctx).Infof("Created plan: %s", plan.Key)
	return CreatedResponse(c, plan)
}

// GetByID returns a plan by ID
func (h *PlanHandler) GetByID(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "PlanHandler.GetByID")
	defer span.End()
	c.SetRequest(c.Request().WithContext(ctx))

	key := c.Param("key")
	if key == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "key is required")
	}

	plan, err := h.repo.GetByKey(ctx, key)
	if err != nil {
		return err
	}

	return SuccessResponse(c, plan)
}

// Update updates an existing plan
func (h *PlanHandler) Update(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "PlanHandler.Update")
	defer span.End()
	c.SetRequest(c.Request().WithContext(ctx))

	key := c.Param("key")
	if key == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "key is required")
	}

	var req UpdatePlanRequest
	if err := c.Bind(&req); err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Name == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "name is required")
	}

	plan, err := h.repo.GetByKey(ctx, key)
	if err != nil {
		return err
	}

	plan.Key = req.Key
	plan.Name = req.Name
	plan.Description = req.Description
	plan.PlanDefinition = database.JSONB[map[string]any]{Data: req.PlanDefinition}
	plan.WaitSeconds = req.WaitSeconds
	plan.RepeatCount = req.RepeatCount
	if req.Enabled != nil {
		plan.Enabled = *req.Enabled
	}

	if err := h.repo.Update(ctx, plan); err != nil {
		return err
	}
	return SuccessResponse(c, plan)
}

// Delete deletes a plan by key
func (h *PlanHandler) Delete(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "PlanHandler.Delete")
	defer span.End()
	c.SetRequest(c.Request().WithContext(ctx))

	key := c.Param("key")
	if key == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "key is required")
	}

	if err := h.repo.Delete(ctx, key); err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to delete plan")
		return err
	}

	h.logger.WithContext(ctx).Infof("Deleted plan: %s", key)
	return NoContentResponse(c)
}

// SetEnabled enables or disables a plan
func (h *PlanHandler) SetEnabled(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "PlanHandler.SetEnabled")
	defer span.End()
	c.SetRequest(c.Request().WithContext(ctx))

	key := c.Param("key")
	if key == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "key is required")
	}

	var req SetEnabledRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest("invalid request body")
	}

	if err := h.repo.SetEnabled(ctx, key, req.Enabled); err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to set plan enabled status")
		return err
	}

	h.logger.WithContext(ctx).Infof("Set plan %s enabled=%v", key, req.Enabled)
	return SuccessResponse(c, map[string]bool{"enabled": req.Enabled})
}

// Trigger manually triggers a plan execution
func (h *PlanHandler) Trigger(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "PlanHandler.Trigger")
	defer span.End()
	c.SetRequest(c.Request().WithContext(ctx))

	planKey := c.Param("key")
	if planKey == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "plan key is required")
	}

	var req TriggerPlanRequest
	if err := c.Bind(&req); err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.ConfigID == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "config_id is required")
	}

	configID, err := uuid.Parse(req.ConfigID)
	if err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, "invalid config_id")
	}

	tenantIDStr := appctx.GetTenantID(ctx)
	if tenantIDStr == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant ID not found")
	}

	plan, err := h.repo.GetByKey(ctx, planKey)
	if err != nil {
		return err
	}

	job := queue.PlanExecutionJob{
		TenantID:        tenantIDStr,
		Integration:     plan.Integration,
		PlanKey:         plan.Key,
		ConfigID:        configID.String(),
		ContextOverride: req.ContextOverride,
	}

	messageID, err := queue.PublishPlanExecution(ctx, h.streams, h.jobQueue, job)
	if err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to publish plan execution job")
		return httperror.WrapError(500, err)
	}

	h.logger.WithContext(ctx).Infof("Triggered plan %s with config %s (message_id=%s)", planKey, configID, messageID)

	return SuccessResponse(c, map[string]string{
		"message_id": messageID,
		"plan_key":   planKey,
		"config_id":  configID.String(),
		"status":     "queued",
	})
}

// ExecutionHandler handles plan execution API endpoints
type ExecutionHandler struct {
	repo   repositories.PlanExecutionRepo
	logger ectologger.Logger
}

// NewExecutionHandler creates a new execution handler
func NewExecutionHandler(repo repositories.PlanExecutionRepo, logger ectologger.Logger) *ExecutionHandler {
	return &ExecutionHandler{
		repo:   repo,
		logger: logger,
	}
}

// Register registers execution routes
func (h *ExecutionHandler) Register(g *echo.Group) {
	g.GET("", h.List)
	g.GET("/:id", h.GetByID)
	g.GET("/:id/children", h.ListChildren)
}

// List returns plan executions
func (h *ExecutionHandler) List(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "ExecutionHandler.List")
	defer span.End()
	c.SetRequest(c.Request().WithContext(ctx))

	planKey := c.QueryParam("plan_key")
	if planKey != "" {
		executions, err := h.repo.ListByPlan(ctx, planKey, 100)
		if err != nil {
			h.logger.WithContext(ctx).WithError(err).Error("Failed to list executions by plan")
			return err
		}
		return SuccessResponse(c, executions)
	}

	statusStr := c.QueryParam("status")
	if statusStr != "" {
		status := models.ExecutionStatus(statusStr)
		executions, err := h.repo.ListByStatus(ctx, status, 100)
		if err != nil {
			h.logger.WithContext(ctx).WithError(err).Error("Failed to list executions by status")
			return err
		}
		return SuccessResponse(c, executions)
	}

	return httperror.NewHTTPError(http.StatusBadRequest, "plan_key or status query parameter is required")
}

// GetByID returns an execution by ID
func (h *ExecutionHandler) GetByID(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "ExecutionHandler.GetByID")
	defer span.End()
	c.SetRequest(c.Request().WithContext(ctx))

	id, err := ParseUUID(c, "id")
	if err != nil {
		return err
	}

	exec, err := h.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	return SuccessResponse(c, exec)
}

// ListChildren returns child executions (sub-steps) for an execution
func (h *ExecutionHandler) ListChildren(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "ExecutionHandler.ListChildren")
	defer span.End()
	c.SetRequest(c.Request().WithContext(ctx))

	parentID, err := ParseUUID(c, "id")
	if err != nil {
		return err
	}

	children, err := h.repo.ListChildren(ctx, parentID)
	if err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to list child executions")
		return err
	}

	return SuccessResponse(c, children)
}

// StatisticsHandler handles plan statistics API endpoints
type StatisticsHandler struct {
	repo   repositories.PlanStatisticsRepo
	logger ectologger.Logger
}

// NewStatisticsHandler creates a new statistics handler
func NewStatisticsHandler(repo repositories.PlanStatisticsRepo, logger ectologger.Logger) *StatisticsHandler {
	return &StatisticsHandler{
		repo:   repo,
		logger: logger,
	}
}

// Register registers statistics routes
func (h *StatisticsHandler) Register(g *echo.Group) {
	g.GET("", h.List)
	g.GET("/:plan_key/:config_id", h.GetByPlanAndConfig)
}

// List returns statistics for a plan
func (h *StatisticsHandler) List(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "StatisticsHandler.List")
	defer span.End()
	c.SetRequest(c.Request().WithContext(ctx))

	planKey := c.QueryParam("plan_key")
	if planKey == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "plan_key query parameter is required")
	}

	stats, err := h.repo.ListByPlan(ctx, planKey)
	if err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to list statistics")
		return err
	}

	return SuccessResponse(c, stats)
}

// GetByPlanAndConfig returns statistics for a specific plan/config combination
func (h *StatisticsHandler) GetByPlanAndConfig(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "StatisticsHandler.GetByPlanAndConfig")
	defer span.End()
	c.SetRequest(c.Request().WithContext(ctx))

	planKey := c.Param("plan_key")
	if planKey == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "plan_key is required")
	}

	configID, err := ParseUUID(c, "config_id")
	if err != nil {
		return err
	}

	stats, err := h.repo.GetByPlanAndConfig(ctx, planKey, configID)
	if err != nil {
		return err
	}

	return SuccessResponse(c, stats)
}
