package handlers

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/Ramsey-B/orchid/pkg/repositories"
	"github.com/Ramsey-B/stem/pkg/database"
)

// IntegrationHandler handles integration-related API requests
type IntegrationHandler struct {
	repo repositories.IntegrationRepo
}

// NewIntegrationHandler creates a new integration handler
func NewIntegrationHandler(repo repositories.IntegrationRepo) *IntegrationHandler {
	return &IntegrationHandler{
		repo: repo,
	}
}

// CreateIntegrationRequest is the request body for creating an integration
type CreateIntegrationRequest struct {
	Name        string  `json:"name" validate:"required"`
	Description *string `json:"description,omitempty"`
	ConfigSchema *CreateConfigSchemaInlineRequest `json:"config_schema,omitempty"`
}

type CreateConfigSchemaInlineRequest struct {
	Name   string         `json:"name" validate:"required"`
	Schema map[string]any `json:"schema" validate:"required"`
}

// UpdateIntegrationRequest is the request body for updating an integration
type UpdateIntegrationRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	ConfigSchema *CreateConfigSchemaInlineRequest `json:"config_schema,omitempty"`
}

// RegisterRoutes registers the integration routes
func (h *IntegrationHandler) RegisterRoutes(g *echo.Group) {
	integrations := g.Group("/integrations")
	integrations.POST("", h.Create)
	integrations.GET("", h.List)
	integrations.GET("/:id", h.Get)
	integrations.PUT("/:id", h.Update)
	integrations.DELETE("/:id", h.Delete)
}

// Create handles POST /integrations
func (h *IntegrationHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()

	tenantID, err := GetTenantID(c)
	if err != nil {
		return err
	}

	var req CreateIntegrationRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest("invalid request body")
	}

	if req.Name == "" {
		return BadRequest("name is required")
	}

	integration := &models.Integration{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
	}

	if req.ConfigSchema != nil {
		if req.ConfigSchema.Name == "" {
			return BadRequest("config_schema.name is required")
		}
		if req.ConfigSchema.Schema == nil {
			return BadRequest("config_schema.schema is required")
		}
		integration.ConfigSchema = database.JSONB[map[string]any]{Data: map[string]any{
			"name":   req.ConfigSchema.Name,
			"schema": req.ConfigSchema.Schema,
		}}
	}

	if err := h.repo.Create(ctx, integration); err != nil {
		return err
	}

	return CreatedResponse(c, integration)
}

// List handles GET /integrations
func (h *IntegrationHandler) List(c echo.Context) error {
	ctx := c.Request().Context()

	integrations, err := h.repo.List(ctx)
	if err != nil {
		return err
	}

	return SuccessResponse(c, integrations)
}

// Get handles GET /integrations/:id
func (h *IntegrationHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := ParseUUID(c, "id")
	if err != nil {
		return err
	}

	integration, err := h.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	return SuccessResponse(c, integration)
}

// Update handles PUT /integrations/:id
func (h *IntegrationHandler) Update(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := ParseUUID(c, "id")
	if err != nil {
		return err
	}

	// First get existing integration
	existing, err := h.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	var req UpdateIntegrationRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest("invalid request body")
	}

	// Update fields if provided
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = req.Description
	}

	if req.ConfigSchema != nil {
		if req.ConfigSchema.Name == "" {
			return BadRequest("config_schema.name is required")
		}
		if req.ConfigSchema.Schema == nil {
			return BadRequest("config_schema.schema is required")
		}
		existing.ConfigSchema = database.JSONB[map[string]any]{Data: map[string]any{
			"name":   req.ConfigSchema.Name,
			"schema": req.ConfigSchema.Schema,
		}}
	}

	if err := h.repo.Update(ctx, existing); err != nil {
		return err
	}

	return SuccessResponse(c, existing)
}

// Delete handles DELETE /integrations/:id
func (h *IntegrationHandler) Delete(c echo.Context) error {
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
