package validation

import (
	"net/http"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectoinject"
	"github.com/Ramsey-B/ivy/pkg/schema"
	ctxmiddleware "github.com/Ramsey-B/stem/pkg/context"
	"github.com/labstack/echo/v4"
)

// ValidateRequest represents a validation request
type ValidateRequest struct {
	EntityType string         `json:"entity_type" validate:"required"`
	Data       map[string]any `json:"data" validate:"required"`
}

// ValidateResponse represents a validation response
type ValidateResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// Register registers validation routes
func Register(g *echo.Group) {
	g.POST("/validate", ValidateEntityData)
}

// ValidateEntityData validates entity data against a schema
func ValidateEntityData(c echo.Context) error {
	ctx := c.Request().Context()

	tenantID := ctxmiddleware.GetTenantID(ctx)
	if tenantID == "" {
		return httperror.NewHTTPError(http.StatusUnauthorized, "tenant_id is required")
	}

	var req ValidateRequest
	if err := c.Bind(&req); err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.EntityType == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "entity_type is required")
	}
	if req.Data == nil {
		return httperror.NewHTTPError(http.StatusBadRequest, "data is required")
	}

	ctx, service, err := ectoinject.GetContext[*schema.ValidationService](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "validation service not available")
	}

	result, err := service.ValidateEntityData(ctx, tenantID, req.EntityType, req.Data)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if !result.Valid {
		// Convert ValidationError slice to string slice
		errorStrings := make([]string, len(result.Errors))
		for i, e := range result.Errors {
			errorStrings[i] = e.Field + ": " + e.Message
		}
		return c.JSON(http.StatusOK, ValidateResponse{
			Valid:  false,
			Errors: errorStrings,
		})
	}

	return c.JSON(http.StatusOK, ValidateResponse{
		Valid: true,
	})
}
