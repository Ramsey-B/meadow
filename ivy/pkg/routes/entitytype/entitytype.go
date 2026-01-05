package entitytype

import (
	"net/http"
	"strconv"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectoinject"
	"github.com/Ramsey-B/ivy/internal/repositories/entitytype"
	"github.com/Ramsey-B/ivy/pkg/models"
	ctxmiddleware "github.com/Ramsey-B/stem/pkg/context"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

var validate = validator.New()

// Register registers entity type routes
func Register(g *echo.Group) {
	g.GET("", List)
	g.POST("", Create)
	g.GET("/:id", Get)
	g.PUT("/:id", Update)
	g.DELETE("/:id", Delete)
	g.GET("/:key/schema", GetSchema)
}

// List returns all entity types for the tenant
func List(c echo.Context) error {
	ctx := c.Request().Context()
	ctx, span := tracing.StartSpan(ctx, "entitytype_handler.List")
	defer span.End()

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

	ctx, repo, err := ectoinject.GetContext[*entitytype.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to get repository")
	}

	items, totalCount, err := repo.List(ctx, tenantID, page, pageSize)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to list entity types")
	}

	return c.JSON(http.StatusOK, models.EntityTypeListResponse{
		Items:      items,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	})
}

// Create creates a new entity type
func Create(c echo.Context) error {
	ctx := c.Request().Context()
	ctx, span := tracing.StartSpan(ctx, "entitytype_handler.Create")
	defer span.End()

	tenantID := ctxmiddleware.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant_id is required")
	}

	var req models.CreateEntityTypeRequest
	if err := c.Bind(&req); err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := validate.Struct(req); err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	ctx, repo, err := ectoinject.GetContext[*entitytype.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to get repository")
	}

	// Check if key already exists
	existing, err := repo.GetByKey(ctx, tenantID, req.Key)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to check existing entity type")
	}
	if existing != nil {
		return httperror.NewHTTPError(http.StatusConflict, "entity type with this key already exists")
	}

	result, err := repo.Create(ctx, tenantID, req)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to create entity type")
	}

	return c.JSON(http.StatusCreated, models.EntityTypeResponse{EntityType: *result})
}

// Get returns a single entity type by ID
func Get(c echo.Context) error {
	ctx := c.Request().Context()
	ctx, span := tracing.StartSpan(ctx, "entitytype_handler.Get")
	defer span.End()

	tenantID := ctxmiddleware.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant_id is required")
	}
	id := c.Param("id")

	ctx, repo, err := ectoinject.GetContext[*entitytype.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to get repository")
	}

	result, err := repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to get entity type")
	}
	if result == nil {
		return httperror.NewHTTPError(http.StatusNotFound, "entity type not found")
	}

	return c.JSON(http.StatusOK, models.EntityTypeResponse{EntityType: *result})
}

// Update updates an entity type
func Update(c echo.Context) error {
	ctx := c.Request().Context()
	ctx, span := tracing.StartSpan(ctx, "entitytype_handler.Update")
	defer span.End()

	tenantID := ctxmiddleware.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant_id is required")
	}

	id := c.Param("id")

	var req models.UpdateEntityTypeRequest
	if err := c.Bind(&req); err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	ctx, repo, err := ectoinject.GetContext[*entitytype.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to get repository")
	}

	result, err := repo.Update(ctx, tenantID, id, req)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to update entity type")
	}
	if result == nil {
		return httperror.NewHTTPError(http.StatusNotFound, "entity type not found")
	}

	return c.JSON(http.StatusOK, models.EntityTypeResponse{EntityType: *result})
}

// Delete soft deletes an entity type
func Delete(c echo.Context) error {
	ctx := c.Request().Context()
	ctx, span := tracing.StartSpan(ctx, "entitytype_handler.Delete")
	defer span.End()

	tenantID := ctxmiddleware.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant_id is required")
	}

	id := c.Param("id")

	ctx, repo, err := ectoinject.GetContext[*entitytype.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to get repository")
	}

	// Check if exists first
	existing, err := repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to get entity type")
	}
	if existing == nil {
		return httperror.NewHTTPError(http.StatusNotFound, "entity type not found")
	}

	if err := repo.Delete(ctx, tenantID, id); err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete entity type")
	}

	return c.NoContent(http.StatusNoContent)
}

// GetSchema exports the entity type schema in Lotus-compatible format
func GetSchema(c echo.Context) error {
	ctx := c.Request().Context()
	ctx, span := tracing.StartSpan(ctx, "entitytype_handler.GetSchema")
	defer span.End()

	tenantID := ctxmiddleware.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant_id is required")
	}

	key := c.Param("key")
	if key == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "key is required")
	}

	ctx, repo, err := ectoinject.GetContext[*entitytype.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to get repository")
	}

	result, err := repo.GetSchemaExport(ctx, tenantID, key)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to export schema")
	}
	if result == nil {
		return httperror.NewHTTPError(http.StatusNotFound, "entity type not found")
	}

	return c.JSON(http.StatusOK, result)
}
