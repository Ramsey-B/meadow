package binding

import (
	"net/http"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectoinject"
	"github.com/Ramsey-B/lotus/internal/repositories/binding"
	bindingMatcher "github.com/Ramsey-B/lotus/pkg/binding"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
	"github.com/Ramsey-B/stem/pkg/context"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/labstack/echo/v4"
)

// Register registers the binding routes
func Register(g *echo.Group) {
	g.GET("", List)
	g.POST("", Create)
	g.GET("/:id", Get)
	g.PUT("/:id", Update)
	g.DELETE("/:id", Delete)
}

// CreateBindingRequest is the request body for creating a binding
type CreateBindingRequest struct {
	Name        string               `json:"name" validate:"required"`
	MappingID   string               `json:"mapping_id" validate:"required"`
	IsEnabled   bool                 `json:"is_enabled"`
	OutputTopic string               `json:"output_topic"`
	Filter      models.BindingFilter `json:"filter"`
}

// UpdateBindingRequest is the request body for updating a binding
type UpdateBindingRequest struct {
	Name        *string               `json:"name"`
	MappingID   *string               `json:"mapping_id"`
	IsEnabled   *bool                 `json:"is_enabled"`
	OutputTopic *string               `json:"output_topic"`
	Filter      *models.BindingFilter `json:"filter"`
}

// BindingResponse is the response for a binding
type BindingResponse struct {
	ID          string               `json:"id"`
	TenantID    string               `json:"tenant_id"`
	Name        string               `json:"name"`
	MappingID   string               `json:"mapping_id"`
	IsEnabled   bool                 `json:"is_enabled"`
	OutputTopic string               `json:"output_topic,omitempty"`
	Filter      models.BindingFilter `json:"filter"`
	CreatedAt   string               `json:"created_at"`
	UpdatedAt   string               `json:"updated_at"`
}

// toResponse converts a binding model to a response
func toResponse(b *models.Binding) *BindingResponse {
	return &BindingResponse{
		ID:          b.ID,
		TenantID:    b.TenantID,
		Name:        b.Name,
		MappingID:   b.MappingID,
		IsEnabled:   b.IsEnabled,
		OutputTopic: b.OutputTopic,
		Filter:      b.Filter,
		CreatedAt:   b.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   b.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// List handles GET /bindings
func List(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "BindingHandler.List")
	defer span.End()

	tenantID := context.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant ID required")
	}

	ctx, repo, err := ectoinject.GetContext[binding.BindingRepository](ctx)
	if err != nil {
		return err
	}

	bindings, err := repo.List(ctx, tenantID)
	if err != nil {
		return err
	}

	responses := make([]*BindingResponse, len(bindings))
	for i, b := range bindings {
		responses[i] = toResponse(b)
	}

	return c.JSON(http.StatusOK, responses)
}

// Create handles POST /bindings
func Create(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "BindingHandler.Create")
	defer span.End()

	tenantID := context.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant ID required")
	}

	req, err := utils.BindRequest[CreateBindingRequest](c)
	if err != nil {
		return err
	}

	ctx, repo, err := ectoinject.GetContext[binding.BindingRepository](ctx)
	if err != nil {
		return err
	}

	b := &models.Binding{
		TenantID:    tenantID,
		Name:        req.Name,
		MappingID:   req.MappingID,
		IsEnabled:   req.IsEnabled,
		OutputTopic: req.OutputTopic,
		Filter:      req.Filter,
	}

	created, err := repo.Create(ctx, b)
	if err != nil {
		return err
	}

	// Update the in-memory matcher cache
	ctx, matcher, err := ectoinject.GetContext[*bindingMatcher.Matcher](ctx)
	if err == nil && matcher != nil {
		matcher.UpdateBinding(created)
	}

	return c.JSON(http.StatusCreated, toResponse(created))
}

// Get handles GET /bindings/:id
func Get(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "BindingHandler.Get")
	defer span.End()

	tenantID := context.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant ID required")
	}

	id := c.Param("id")
	if id == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "binding ID required")
	}

	ctx, repo, err := ectoinject.GetContext[binding.BindingRepository](ctx)
	if err != nil {
		return err
	}

	b, err := repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, toResponse(b))
}

// Update handles PUT /bindings/:id
func Update(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "BindingHandler.Update")
	defer span.End()

	tenantID := context.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant ID required")
	}

	id := c.Param("id")
	if id == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "binding ID required")
	}

	ctx, repo, err := ectoinject.GetContext[binding.BindingRepository](ctx)
	if err != nil {
		return err
	}

	// Get existing binding
	existing, err := repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}

	var req UpdateBindingRequest
	if err := c.Bind(&req); err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Apply updates
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.MappingID != nil {
		existing.MappingID = *req.MappingID
	}
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}
	if req.OutputTopic != nil {
		existing.OutputTopic = *req.OutputTopic
	}
	if req.Filter != nil {
		existing.Filter = *req.Filter
	}

	updated, err := repo.Update(ctx, existing)
	if err != nil {
		return err
	}

	// Update the in-memory matcher cache
	ctx, matcher, err := ectoinject.GetContext[*bindingMatcher.Matcher](ctx)
	if err == nil && matcher != nil {
		matcher.UpdateBinding(updated)
	}

	return c.JSON(http.StatusOK, toResponse(updated))
}

// Delete handles DELETE /bindings/:id
func Delete(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "BindingHandler.Delete")
	defer span.End()

	tenantID := context.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant ID required")
	}

	id := c.Param("id")
	if id == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "binding ID required")
	}

	ctx, repo, err := ectoinject.GetContext[binding.BindingRepository](ctx)
	if err != nil {
		return err
	}

	if err := repo.Delete(ctx, tenantID, id); err != nil {
		return err
	}

	// Remove from the in-memory matcher cache
	ctx, matcher, err := ectoinject.GetContext[*bindingMatcher.Matcher](ctx)
	if err == nil && matcher != nil {
		matcher.RemoveBinding(tenantID, id)
	}

	return c.NoContent(http.StatusNoContent)
}

