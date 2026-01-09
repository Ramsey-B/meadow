package tenant

import (
	"net/http"

	"github.com/Gobusters/ectoinject"
	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/labstack/echo/v4"
)

// Register registers tenant routes
func Register(g *echo.Group) {
	g.DELETE("/tenant/:tenant_id", deleteTenantData)
}

// deleteTenantData deletes all data for a specific tenant
// This is intended for testing purposes to clean up test data
func deleteTenantData(c echo.Context) error {
	ctx := c.Request().Context()

	tenantID := c.Param("tenant_id")
	if tenantID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "tenant_id is required",
		})
	}

	// Get database and logger from DI
	ctx, db, err := ectoinject.GetContext[database.DB](ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get database",
		})
	}

	ctx, logger, _ := ectoinject.GetContext[ectologger.Logger](ctx)
	if logger != nil {
		logger.WithContext(ctx).WithFields(map[string]any{"tenant_id": tenantID}).Info("Deleting all data for tenant")
	}

	counts := make(map[string]int64)

	// Delete in order respecting foreign key constraints
	// Note: staged_relationships references staged_entities

	// 1. Delete staged relationship criteria
	result, err := db.ExecContext(ctx, "DELETE FROM staged_relationship_criteria WHERE tenant_id = $1", tenantID)
	if err == nil {
		counts["staged_relationship_criteria"], _ = result.RowsAffected()
	}

	// 2. Delete relationship clusters (references staged_relationships)
	result, err = db.ExecContext(ctx, "DELETE FROM relationship_clusters WHERE tenant_id = $1", tenantID)
	if err == nil {
		counts["relationship_clusters"], _ = result.RowsAffected()
	}

	// 3. Delete staged relationships
	result, err = db.ExecContext(ctx, "DELETE FROM staged_relationships WHERE tenant_id = $1", tenantID)
	if err == nil {
		counts["staged_relationships"], _ = result.RowsAffected()
	}

	// 4. Delete staged entities
	result, err = db.ExecContext(ctx, "DELETE FROM staged_entities WHERE tenant_id = $1", tenantID)
	if err == nil {
		counts["staged_entities"], _ = result.RowsAffected()
	}

	// 5. Delete match rules
	result, err = db.ExecContext(ctx, "DELETE FROM match_rules WHERE tenant_id = $1", tenantID)
	if err == nil {
		counts["match_rules"], _ = result.RowsAffected()
	}

	// 6. Delete deletion strategies
	result, err = db.ExecContext(ctx, "DELETE FROM deletion_strategies WHERE tenant_id = $1", tenantID)
	if err == nil {
		counts["deletion_strategies"], _ = result.RowsAffected()
	}

	// 7. Delete relationship types
	result, err = db.ExecContext(ctx, "DELETE FROM relationship_types WHERE tenant_id = $1", tenantID)
	if err == nil {
		counts["relationship_types"], _ = result.RowsAffected()
	}

	// 8. Delete entity types
	result, err = db.ExecContext(ctx, "DELETE FROM entity_types WHERE tenant_id = $1", tenantID)
	if err == nil {
		counts["entity_types"], _ = result.RowsAffected()
	}

	if logger != nil {
		fields := map[string]any{"tenant_id": tenantID}
		for k, v := range counts {
			fields[k] = v
		}
		logger.WithContext(ctx).WithFields(fields).Info("Tenant data deleted")
	}

	response := map[string]interface{}{
		"message":   "tenant data deleted",
		"tenant_id": tenantID,
	}
	for k, v := range counts {
		response[k] = v
	}

	return c.JSON(http.StatusOK, response)
}

