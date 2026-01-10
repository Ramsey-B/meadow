// Package middleware provides HTTP middleware for Lotus.
package middleware

import (
	stemcontext "github.com/Ramsey-B/stem/pkg/context"
	"github.com/labstack/echo/v4"
)

// TestAuth middleware extracts tenant_id and user_id from headers when auth is disabled.
// This allows testing the API without a real JWT auth system.
// Headers:
//   - X-Tenant-ID: The tenant ID
//   - X-User-ID: The user ID
//
// WARNING: Only use this when AUTH_ENABLED=false. Do not enable in production.
func TestAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()

			// Extract tenant ID from header
			tenantID := c.Request().Header.Get("X-Tenant-ID")
			if tenantID != "" {
				ctx = stemcontext.SetTenantID(ctx, tenantID)
			}

			// Extract user ID from header
			userID := c.Request().Header.Get("X-User-ID")
			if userID != "" {
				ctx = stemcontext.SetUserID(ctx, userID)
			}

			// Update the request context
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

