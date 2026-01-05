package entity

import (
	"net/http"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectoinject"
	"github.com/Ramsey-B/stem/pkg/context"
	"github.com/labstack/echo/v4"

	"github.com/Ramsey-B/ivy/internal/repositories/mergedentity"
	"github.com/Ramsey-B/ivy/pkg/graph"
)

// Register registers entity routes
func Register(g *echo.Group) {
	g.GET("/:entityType/:id", GetEntity)
	g.GET("/:entityType/:id/sources", GetEntitySources)
	g.GET("/:entityType/:id/relationships", GetEntityRelationships)
}

// GetEntity gets a merged entity by ID
func GetEntity(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	entityType := c.Param("entityType")
	id := c.Param("id")

	// Try graph database first for fast reads
	ctx, graphService, err := ectoinject.GetContext[*graph.EntityService](ctx)
	if err == nil && graphService != nil {
		entityData, err := graphService.Get(ctx, tenantID, id, entityType)
		if err == nil && entityData != nil {
			return c.JSON(http.StatusOK, entityData)
		}
	}

	// Fall back to PostgreSQL
	ctx, repo, err := ectoinject.GetContext[*mergedentity.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	entity, err := repo.Get(ctx, tenantID, id)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, entity)
}

// GetEntitySources gets the source entities for a merged entity
func GetEntitySources(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	id := c.Param("id")

	ctx, repo, err := ectoinject.GetContext[*mergedentity.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	sources, err := repo.GetClusterMembers(ctx, tenantID, id)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, sources)
}

// GetEntityRelationships gets all relationships for an entity
func GetEntityRelationships(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	entityType := c.Param("entityType")
	id := c.Param("id")

	direction := c.QueryParam("direction") // incoming, outgoing, both
	if direction == "" {
		direction = "both"
	}

	ctx, relService, err := ectoinject.GetContext[*graph.RelationshipService](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "graph service unavailable")
	}

	relationships, err := relService.GetRelationships(ctx, tenantID, id, entityType, direction)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, relationships)
}
