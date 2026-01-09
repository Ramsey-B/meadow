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

	// Delete bindings first (may reference mapping definitions)
	bindingsResult, err := db.ExecContext(ctx, "DELETE FROM bindings WHERE tenant_id = $1", tenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to delete bindings",
		})
	}
	bindingsCount, _ := bindingsResult.RowsAffected()

	// Delete mapping definitions
	mappingsResult, err := db.ExecContext(ctx, "DELETE FROM mapping_definitions WHERE tenant_id = $1", tenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to delete mapping definitions",
		})
	}
	mappingsCount, _ := mappingsResult.RowsAffected()

	if logger != nil {
		logger.WithContext(ctx).WithFields(map[string]any{
			"tenant_id":           tenantID,
			"bindings":            bindingsCount,
			"mapping_definitions": mappingsCount,
		}).Info("Tenant data deleted")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":             "tenant data deleted",
		"tenant_id":           tenantID,
		"bindings":            bindingsCount,
		"mapping_definitions": mappingsCount,
	})
}

