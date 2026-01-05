package handlers

import (
	"net/http"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	appctx "github.com/Ramsey-B/stem/pkg/context"
)

// ParseUUID parses a UUID from a path parameter
func ParseUUID(c echo.Context, param string) (uuid.UUID, error) {
	idStr := c.Param(param)
	if idStr == "" {
		return uuid.Nil, httperror.NewHTTPError(http.StatusBadRequest, "missing "+param)
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, httperror.NewHTTPErrorf(http.StatusBadRequest, "invalid %s: must be a valid UUID", param)
	}

	return id, nil
}

// GetTenantID extracts the tenant ID from context
func GetTenantID(c echo.Context) (uuid.UUID, error) {
	ctx := c.Request().Context()
	tenantIDStr := appctx.GetTenantID(ctx)
	if tenantIDStr == "" {
		return uuid.Nil, httperror.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return uuid.Nil, httperror.NewHTTPError(http.StatusUnauthorized, "invalid authentication token")
	}

	return tenantID, nil
}

// SuccessResponse returns a 200 OK with data
func SuccessResponse(c echo.Context, data any) error {
	return c.JSON(http.StatusOK, data)
}

// CreatedResponse returns a 201 Created with data
func CreatedResponse(c echo.Context, data any) error {
	return c.JSON(http.StatusCreated, data)
}

// NoContentResponse returns a 204 No Content
func NoContentResponse(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// BadRequest returns a 400 Bad Request error
func BadRequest(message string) error {
	return httperror.NewHTTPError(http.StatusBadRequest, message)
}

// Unauthorized returns a 401 Unauthorized error
func Unauthorized(message string) error {
	return httperror.NewHTTPError(http.StatusUnauthorized, message)
}
