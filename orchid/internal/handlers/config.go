package handlers

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/Ramsey-B/orchid/pkg/repositories"
)

// ConfigHandler handles config-related API requests
type ConfigHandler struct {
	repo repositories.ConfigRepo
}

// NewConfigHandler creates a new config handler
func NewConfigHandler(repo repositories.ConfigRepo) *ConfigHandler {
	return &ConfigHandler{repo: repo}
}

// CreateConfigRequest is the request body for creating a config
type CreateConfigRequest struct {
	IntegrationID  uuid.UUID      `json:"integration_id" validate:"required"`
	Name           string         `json:"name" validate:"required"`
	Values         map[string]any `json:"values" validate:"required"`
	Enabled        bool           `json:"enabled"`
}

// UpdateConfigRequest is the request body for updating a config
type UpdateConfigRequest struct {
	Name    string         `json:"name"`
	Values  map[string]any `json:"values"`
	Enabled *bool          `json:"enabled"`
}

// RegisterRoutes registers the config routes
func (h *ConfigHandler) RegisterRoutes(g *echo.Group) {
	configs := g.Group("/configs")
	configs.POST("", h.Create)
	configs.GET("", h.List)
	configs.GET("/:id", h.Get)
	configs.PUT("/:id", h.Update)
	configs.DELETE("/:id", h.Delete)
	configs.POST("/:id/enable", h.Enable)
	configs.POST("/:id/disable", h.Disable)
}

// Create handles POST /configs
func (h *ConfigHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()

	tenantID, err := GetTenantID(c)
	if err != nil {
		return err
	}

	var req CreateConfigRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest("invalid request body")
	}

	if req.IntegrationID == uuid.Nil {
		return BadRequest("integration_id is required")
	}
	if req.Name == "" {
		return BadRequest("name is required")
	}
	if req.Values == nil {
		return BadRequest("values is required")
	}

	config := &models.Config{
		ID:             uuid.New(),
		TenantID:       tenantID,
		IntegrationID:  req.IntegrationID,
		Name:           req.Name,
		Values:         database.JSONB[map[string]any]{Data: req.Values},
		Enabled:        req.Enabled,
	}

	if err := h.repo.Create(ctx, config); err != nil {
		return err
	}

	return CreatedResponse(c, config)
}

// List handles GET /configs
func (h *ConfigHandler) List(c echo.Context) error {
	ctx := c.Request().Context()

	// Required filter by integration
	integrationIDStr := c.QueryParam("integration_id")
	if integrationIDStr == "" {
		return BadRequest("integration_id query parameter is required")
	}

	integrationID, err := uuid.Parse(integrationIDStr)
	if err != nil {
		return BadRequest("invalid integration_id")
	}

	configs, err := h.repo.ListByIntegration(ctx, integrationID)
	if err != nil {
		return err
	}

	return SuccessResponse(c, configs)
}

// Get handles GET /configs/:id
func (h *ConfigHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := ParseUUID(c, "id")
	if err != nil {
		return err
	}

	config, err := h.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	return SuccessResponse(c, config)
}

// Update handles PUT /configs/:id
func (h *ConfigHandler) Update(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := ParseUUID(c, "id")
	if err != nil {
		return err
	}

	existing, err := h.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	var req UpdateConfigRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest("invalid request body")
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Values != nil {
		existing.Values = database.JSONB[map[string]any]{Data: req.Values}
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	if err := h.repo.Update(ctx, existing); err != nil {
		return err
	}

	return SuccessResponse(c, existing)
}

// Delete handles DELETE /configs/:id
func (h *ConfigHandler) Delete(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := ParseUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.repo.Delete(ctx, id); err != nil {
		return err
	}

	return NoContentResponse(c)
}

// Enable handles POST /configs/:id/enable
func (h *ConfigHandler) Enable(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := ParseUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.repo.SetEnabled(ctx, id, true); err != nil {
		return err
	}

	return SuccessResponse(c, map[string]bool{"enabled": true})
}

// Disable handles POST /configs/:id/disable
func (h *ConfigHandler) Disable(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := ParseUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.repo.SetEnabled(ctx, id, false); err != nil {
		return err
	}

	return SuccessResponse(c, map[string]bool{"enabled": false})
}
