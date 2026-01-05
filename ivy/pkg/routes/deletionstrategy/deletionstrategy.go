package deletionstrategy

import (
	"net/http"
	"strconv"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectoinject"
	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/context"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/labstack/echo/v4"

	deletionstrategyrepo "github.com/Ramsey-B/ivy/internal/repositories/deletionstrategy"
	"github.com/Ramsey-B/ivy/pkg/models"
)

// Register registers deletion strategy routes
func Register(g *echo.Group) {
	g.GET("", ListDeletionStrategies)
	g.GET("/:id", GetDeletionStrategy)
	g.POST("", CreateDeletionStrategy)
	g.PUT("/:id", UpdateDeletionStrategy)
	g.DELETE("/:id", DeleteDeletionStrategy)
}

// ListDeletionStrategies lists all deletion strategies for a tenant
func ListDeletionStrategies(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "deletionstrategy.ListDeletionStrategies")
	defer span.End()

	tenantID := context.GetTenantID(ctx)

	ctx, repo, err := ectoinject.GetContext[*deletionstrategyrepo.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	// Parse query parameters
	var entityType, relationshipType *string
	if et := c.QueryParam("entity_type"); et != "" {
		entityType = &et
	}
	if rt := c.QueryParam("relationship_type"); rt != "" {
		relationshipType = &rt
	}

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	resp, err := repo.List(ctx, tenantID, entityType, relationshipType, page, pageSize)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, resp)
}

// GetDeletionStrategy gets a deletion strategy by ID
func GetDeletionStrategy(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "deletionstrategy.GetDeletionStrategy")
	defer span.End()

	tenantID := context.GetTenantID(ctx)
	id := c.Param("id")

	ctx, repo, err := ectoinject.GetContext[*deletionstrategyrepo.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	strategy, err := repo.Get(ctx, tenantID, id)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, strategy)
}

// CreateDeletionStrategy creates a new deletion strategy
func CreateDeletionStrategy(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "deletionstrategy.CreateDeletionStrategy")
	defer span.End()

	tenantID := context.GetTenantID(ctx)

	var req models.CreateDeletionStrategyRequest
	if err := c.Bind(&req); err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Validation: exactly one of entity_type or relationship_type must be set
	if (req.EntityType == nil && req.RelationshipType == nil) ||
		(req.EntityType != nil && req.RelationshipType != nil) {
		return httperror.NewHTTPError(http.StatusBadRequest, "exactly one of entity_type or relationship_type must be set")
	}

	// Validate strategy type
	switch req.StrategyType {
	case models.DeletionStrategyExecutionBased, models.DeletionStrategyExplicit,
		models.DeletionStrategyStaleness, models.DeletionStrategyRetention,
		models.DeletionStrategyComposite:
	// Valid
	default:
		return httperror.NewHTTPError(http.StatusBadRequest, "invalid strategy type")
	}

	ctx, repo, err := ectoinject.GetContext[*deletionstrategyrepo.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	created, err := repo.Create(ctx, tenantID, req)
	if err != nil {
		return err
	}

	ctx, logger, _ := ectoinject.GetContext[ectologger.Logger](ctx)
	if logger != nil {
		logger.WithContext(ctx).WithFields(map[string]any{"id": created.ID}).Info("Created deletion strategy")
	}

	return c.JSON(http.StatusCreated, created)
}

// UpdateDeletionStrategy updates a deletion strategy
func UpdateDeletionStrategy(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "deletionstrategy.UpdateDeletionStrategy")
	defer span.End()

	tenantID := context.GetTenantID(ctx)
	id := c.Param("id")

	var req models.UpdateDeletionStrategyRequest
	if err := c.Bind(&req); err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	ctx, repo, err := ectoinject.GetContext[*deletionstrategyrepo.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	updated, err := repo.Update(ctx, tenantID, id, req)
	if err != nil {
		return err
	}

	ctx, logger, _ := ectoinject.GetContext[ectologger.Logger](ctx)
	if logger != nil {
		logger.WithContext(ctx).WithFields(map[string]any{"id": updated.ID}).Info("Updated deletion strategy")
	}

	return c.JSON(http.StatusOK, updated)
}

// DeleteDeletionStrategy deletes a deletion strategy
func DeleteDeletionStrategy(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "deletionstrategy.DeleteDeletionStrategy")
	defer span.End()

	tenantID := context.GetTenantID(ctx)
	id := c.Param("id")

	ctx, repo, err := ectoinject.GetContext[*deletionstrategyrepo.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	if err := repo.Delete(ctx, tenantID, id); err != nil {
		return err
	}

	ctx, logger, _ := ectoinject.GetContext[ectologger.Logger](ctx)
	if logger != nil {
		logger.WithContext(ctx).WithFields(map[string]any{"id": id}).Info("Deleted deletion strategy")
	}

	return c.NoContent(http.StatusNoContent)
}
