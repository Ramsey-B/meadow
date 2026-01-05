package handlers

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/Ramsey-B/orchid/pkg/repositories"
	"github.com/Ramsey-B/stem/pkg/database"
)

// AuthFlowHandler handles auth flow API endpoints
type AuthFlowHandler struct {
	repo repositories.AuthFlowRepo
}

// NewAuthFlowHandler creates a new auth flow handler
func NewAuthFlowHandler(repo repositories.AuthFlowRepo) *AuthFlowHandler {
	return &AuthFlowHandler{repo: repo}
}

// CreateAuthFlowRequest represents the create auth flow request body
type CreateAuthFlowRequest struct {
	IntegrationID string         `json:"integration_id" validate:"required"`
	Name          string         `json:"name" validate:"required"`
	PlanDefinition map[string]any `json:"plan_definition" validate:"required"` // This is a models.Step encoded as JSON

	TokenPath     string  `json:"token_path" validate:"required"`
	HeaderName    string  `json:"header_name" validate:"required"`
	HeaderFormat  *string `json:"header_format,omitempty"`
	RefreshPath   *string `json:"refresh_path,omitempty"`
	ExpiresInPath *string `json:"expires_in_path,omitempty"`
	TTLSeconds    *int    `json:"ttl_seconds,omitempty"`
	SkewSeconds   *int    `json:"skew_seconds,omitempty"`
}

// UpdateAuthFlowRequest represents the update auth flow request body
type UpdateAuthFlowRequest struct {
	Name          string         `json:"name" validate:"required"`
	PlanDefinition map[string]any `json:"plan_definition" validate:"required"` // This is a models.Step encoded as JSON

	TokenPath     string  `json:"token_path" validate:"required"`
	HeaderName    string  `json:"header_name" validate:"required"`
	HeaderFormat  *string `json:"header_format,omitempty"`
	RefreshPath   *string `json:"refresh_path,omitempty"`
	ExpiresInPath *string `json:"expires_in_path,omitempty"`
	TTLSeconds    *int    `json:"ttl_seconds,omitempty"`
	SkewSeconds   *int    `json:"skew_seconds,omitempty"`
}

// RegisterRoutes registers auth flow routes
func (h *AuthFlowHandler) RegisterRoutes(g *echo.Group) {
	flows := g.Group("/auth-flows")
	flows.POST("", h.Create)
	flows.GET("", h.List)
	flows.GET("/:id", h.GetByID)
	flows.PUT("/:id", h.Update)
	flows.DELETE("/:id", h.Delete)
}

// Create handles POST /auth-flows
func (h *AuthFlowHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()

	tenantID, err := GetTenantID(c)
	if err != nil {
		return err
	}

	var req CreateAuthFlowRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest("invalid request body")
	}

	integrationID, err := uuid.Parse(req.IntegrationID)
	if err != nil {
		return BadRequest("invalid integration_id")
	}

	if req.Name == "" {
		return BadRequest("name is required")
	}
	if req.PlanDefinition == nil {
		return BadRequest("plan_definition is required")
	}
	if req.TokenPath == "" {
		return BadRequest("token_path is required")
	}
	if req.HeaderName == "" {
		return BadRequest("header_name is required")
	}

	authFlow := &models.AuthFlow{
		ID:            uuid.New(),
		TenantID:      tenantID,
		IntegrationID: integrationID,
		Name:          req.Name,
		PlanDefinition: database.JSONB[map[string]any]{Data: req.PlanDefinition},
		TokenPath:     req.TokenPath,
		HeaderName:    req.HeaderName,
		HeaderFormat:  req.HeaderFormat,
		RefreshPath:   req.RefreshPath,
		ExpiresInPath: req.ExpiresInPath,
		TTLSeconds:    req.TTLSeconds,
		SkewSeconds:   req.SkewSeconds,
	}

	if err := h.repo.Create(ctx, authFlow); err != nil {
		return err
	}

	return CreatedResponse(c, authFlow)
}

// List handles GET /auth-flows?integration_id=...
func (h *AuthFlowHandler) List(c echo.Context) error {
	ctx := c.Request().Context()

	integrationIDStr := c.QueryParam("integration_id")
	if integrationIDStr == "" {
		return BadRequest("integration_id query parameter is required")
	}

	integrationID, err := uuid.Parse(integrationIDStr)
	if err != nil {
		return BadRequest("invalid integration_id")
	}

	flows, err := h.repo.ListByIntegration(ctx, integrationID)
	if err != nil {
		return err
	}

	return SuccessResponse(c, flows)
}

// GetByID handles GET /auth-flows/:id
func (h *AuthFlowHandler) GetByID(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := ParseUUID(c, "id")
	if err != nil {
		return err
	}

	flow, err := h.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	return SuccessResponse(c, flow)
}

// Update handles PUT /auth-flows/:id
func (h *AuthFlowHandler) Update(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := ParseUUID(c, "id")
	if err != nil {
		return err
	}

	existing, err := h.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	var req UpdateAuthFlowRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest("invalid request body")
	}

	if req.Name == "" {
		return BadRequest("name is required")
	}
	if req.PlanDefinition == nil {
		return BadRequest("plan_definition is required")
	}
	if req.TokenPath == "" {
		return BadRequest("token_path is required")
	}
	if req.HeaderName == "" {
		return BadRequest("header_name is required")
	}

	existing.Name = req.Name
	existing.PlanDefinition = database.JSONB[map[string]any]{Data: req.PlanDefinition}
	existing.TokenPath = req.TokenPath
	existing.HeaderName = req.HeaderName
	existing.HeaderFormat = req.HeaderFormat
	existing.RefreshPath = req.RefreshPath
	existing.ExpiresInPath = req.ExpiresInPath
	existing.TTLSeconds = req.TTLSeconds
	existing.SkewSeconds = req.SkewSeconds

	if err := h.repo.Update(ctx, existing); err != nil {
		return err
	}

	return SuccessResponse(c, existing)
}

// Delete handles DELETE /auth-flows/:id
func (h *AuthFlowHandler) Delete(c echo.Context) error {
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


