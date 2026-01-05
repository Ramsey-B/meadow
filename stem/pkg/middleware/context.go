package middleware

import (
	"github.com/Ramsey-B/stem/pkg/context"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const (
	// HeaderTenantID is the header key for tenant ID
	HeaderTenantID = "X-Tenant-ID"
	// HeaderUserID is the header key for user ID
	HeaderUserID = "X-User-ID"
)

func Context() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			req := c.Request()

			// get request id from header
			requestID := req.Header.Get(echo.HeaderXRequestID)
			if requestID == "" {
				requestID = uuid.New().String()
			}

			// get tenant id from header
			tenantID := req.Header.Get(HeaderTenantID)

			// get user id from header
			userID := req.Header.Get(HeaderUserID)

			ctx := req.Context()
			ctx = context.SetRequestID(ctx, requestID)
			ctx = context.SetMethod(ctx, req.Method)
			ctx = context.SetRoute(ctx, req.URL.Path)
			ctx = context.SetRemoteIP(ctx, c.RealIP())
			ctx = context.SetReferer(ctx, req.Referer())
			ctx = context.SetTenantID(ctx, tenantID)
			ctx = context.SetUserID(ctx, userID)

			c.SetRequest(req.WithContext(ctx))

			return next(c)
		}
	}
}

