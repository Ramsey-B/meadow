package matchrule

import (
	"encoding/json"
	"net/http"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectoinject"
	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/context"
	"github.com/labstack/echo/v4"

	"github.com/Ramsey-B/ivy/internal/repositories/matchrule"
	"github.com/Ramsey-B/ivy/pkg/models"
)

// Register registers match rule routes
func Register(g *echo.Group) {
	g.GET("", ListMatchRules)
	g.GET("/:id", GetMatchRule)
	g.POST("", CreateMatchRule)
	g.PUT("/:id", UpdateMatchRule)
	g.DELETE("/:id", DeleteMatchRule)
}

// ListMatchRules lists match rules by entity type
func ListMatchRules(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	entityType := c.QueryParam("entity_type")
	if entityType == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "entity_type query parameter is required")
	}

	ctx, repo, err := ectoinject.GetContext[*matchrule.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	rules, err := repo.GetByEntityType(ctx, tenantID, entityType)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, rules)
}

// GetMatchRule gets a match rule by ID
func GetMatchRule(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	id := c.Param("id")

	ctx, repo, err := ectoinject.GetContext[*matchrule.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	rule, err := repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, rule)
}

// CreateMatchRuleRequest is the request body for creating a match rule
type CreateMatchRuleRequest struct {
	EntityType  string                  `json:"entity_type" validate:"required"`
	Name        string                  `json:"name" validate:"required"`
	Description *string                 `json:"description,omitempty"`
	Priority    int                     `json:"priority"`
	IsActive    bool                    `json:"is_active"`
	Conditions  []models.MatchCondition `json:"conditions" validate:"required,min=1"`
	ScoreWeight float64                 `json:"score_weight"`
}

// CreateMatchRule creates a new match rule
func CreateMatchRule(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	var req CreateMatchRuleRequest
	if err := c.Bind(&req); err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.EntityType == "" || req.Name == "" || len(req.Conditions) == 0 {
		return httperror.NewHTTPError(http.StatusBadRequest, "entity_type, name, and at least one condition are required")
	}

	// Convert conditions to JSON
	conditionsJSON, err := json.Marshal(req.Conditions)
	if err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, "invalid conditions")
	}

	ctx, repo, err := ectoinject.GetContext[*matchrule.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	modelReq := models.CreateMatchRuleRequest{
		EntityType:  req.EntityType,
		Name:        req.Name,
		Description: req.Description,
		Priority:    req.Priority,
		IsActive:    req.IsActive,
		Conditions:  conditionsJSON,
		ScoreWeight: req.ScoreWeight,
	}

	created, err := repo.Create(ctx, tenantID, modelReq)
	if err != nil {
		return err
	}

	ctx, logger, _ := ectoinject.GetContext[ectologger.Logger](ctx)
	if logger != nil {
		logger.WithContext(ctx).WithFields(map[string]any{"id": created.ID}).Info("Created match rule")
	}

	return c.JSON(http.StatusCreated, created)
}

// UpdateMatchRuleRequest is the request body for updating a match rule
type UpdateMatchRuleRequest struct {
	Name        *string                 `json:"name,omitempty"`
	Description *string                 `json:"description,omitempty"`
	Priority    *int                    `json:"priority,omitempty"`
	IsActive    *bool                   `json:"is_active,omitempty"`
	Conditions  []models.MatchCondition `json:"conditions,omitempty"`
	ScoreWeight *float64                `json:"score_weight,omitempty"`
}

// UpdateMatchRule updates a match rule
func UpdateMatchRule(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	ruleID := c.Param("id")

	var req UpdateMatchRuleRequest
	if err := c.Bind(&req); err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	ctx, repo, err := ectoinject.GetContext[*matchrule.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	// Convert conditions to JSON if provided
	var conditionsJSON json.RawMessage
	if len(req.Conditions) > 0 {
		conditionsJSON, _ = json.Marshal(req.Conditions)
	}

	modelReq := models.UpdateMatchRuleRequest{
		Name:        req.Name,
		Description: req.Description,
		Priority:    req.Priority,
		IsActive:    req.IsActive,
		Conditions:  conditionsJSON,
		ScoreWeight: req.ScoreWeight,
	}

	updated, err := repo.Update(ctx, tenantID, ruleID, modelReq)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, updated)
}

// DeleteMatchRule deletes a match rule
func DeleteMatchRule(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	ruleID := c.Param("id")

	ctx, repo, err := ectoinject.GetContext[*matchrule.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	if err := repo.Delete(ctx, tenantID, ruleID); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}
