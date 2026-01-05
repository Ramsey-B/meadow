package relationshiptype

import (
	"net/http"
	"regexp"
	"strconv"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectoinject"
	"github.com/Ramsey-B/ivy/internal/repositories/relationshiptype"
	"github.com/Ramsey-B/ivy/pkg/models"
	ctxmiddleware "github.com/Ramsey-B/stem/pkg/context"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

var validate = validator.New()

var relationshipTypeKeyRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Register registers relationship type routes
func Register(g *echo.Group) {
	g.GET("", List)
	g.POST("", Create)
	g.GET("/:id", Get)
	g.PUT("/:id", Update)
	g.DELETE("/:id", Delete)
}

// List returns all relationship types for the tenant
func List(c echo.Context) error {
	ctx := c.Request().Context()

	tenantID := ctxmiddleware.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant_id is required")
	}

	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	ctx, repo, err := ectoinject.GetContext[*relationshiptype.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to get repository")
	}

	items, totalCount, err := repo.List(ctx, tenantID, page, pageSize)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to list relationship types")
	}

	return c.JSON(http.StatusOK, models.RelationshipTypeListResponse{
		Items:      items,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	})
}

// Create creates a new relationship type
func Create(c echo.Context) error {
	ctx := c.Request().Context()

	tenantID := ctxmiddleware.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant_id is required")
	}

	var req models.CreateRelationshipTypeRequest
	if err := c.Bind(&req); err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := validate.Struct(req); err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if !relationshipTypeKeyRE.MatchString(req.Key) {
		return httperror.NewHTTPError(http.StatusBadRequest, "key must be snake_case (lowercase letters, numbers, underscores), starting with a letter")
	}

	ctx, repo, err := ectoinject.GetContext[*relationshiptype.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to get repository")
	}

	// Check if key already exists
	existing, err := repo.GetByKey(ctx, tenantID, req.Key)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to check existing relationship type")
	}
	if existing != nil {
		return httperror.NewHTTPError(http.StatusConflict, "relationship type with this key already exists")
	}

	result, err := repo.Create(ctx, tenantID, req)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to create relationship type")
	}

	return c.JSON(http.StatusCreated, models.RelationshipTypeResponse{RelationshipType: *result})
}

// Get returns a single relationship type by ID
func Get(c echo.Context) error {
	ctx := c.Request().Context()

	tenantID := ctxmiddleware.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant_id is required")
	}

	relationshipTypeID := c.Param("id")

	ctx, repo, err := ectoinject.GetContext[*relationshiptype.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to get repository")
	}

	result, err := repo.GetByID(ctx, tenantID, relationshipTypeID)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to get relationship type")
	}
	if result == nil {
		return httperror.NewHTTPError(http.StatusNotFound, "relationship type not found")
	}

	return c.JSON(http.StatusOK, models.RelationshipTypeResponse{RelationshipType: *result})
}

// Update updates a relationship type
func Update(c echo.Context) error {
	ctx := c.Request().Context()

	tenantID := ctxmiddleware.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant_id is required")
	}

	relationshipTypeID := c.Param("id")

	var req models.UpdateRelationshipTypeRequest
	if err := c.Bind(&req); err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	ctx, repo, err := ectoinject.GetContext[*relationshiptype.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to get repository")
	}

	result, err := repo.Update(ctx, tenantID, relationshipTypeID, req)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to update relationship type")
	}
	if result == nil {
		return httperror.NewHTTPError(http.StatusNotFound, "relationship type not found")
	}

	return c.JSON(http.StatusOK, models.RelationshipTypeResponse{RelationshipType: *result})
}

// Delete soft deletes a relationship type
func Delete(c echo.Context) error {
	ctx := c.Request().Context()

	tenantID := ctxmiddleware.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant_id is required")
	}

	relationshipTypeID := c.Param("id")

	ctx, repo, err := ectoinject.GetContext[*relationshiptype.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to get repository")
	}

	// Check if exists first
	existing, err := repo.GetByID(ctx, tenantID, relationshipTypeID)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to get relationship type")
	}
	if existing == nil {
		return httperror.NewHTTPError(http.StatusNotFound, "relationship type not found")
	}

	if err := repo.Delete(ctx, tenantID, relationshipTypeID); err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete relationship type")
	}

	return c.NoContent(http.StatusNoContent)
}
