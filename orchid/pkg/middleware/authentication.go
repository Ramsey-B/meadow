package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Gobusters/ectologger"
	utils "github.com/Ramsey-B/stem/pkg/context"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/labstack/echo/v4"
)

type UserClaims struct {
	Sub         string `json:"sub"`
	Email       string `json:"email"`
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

func Authentication(logger ectologger.Logger, issuer string, clientID string) echo.MiddlewareFunc {
	provider, err := oidc.NewProvider(context.Background(), issuer)
	if err != nil {
		log.Fatalf("oidc provider: %v", err)
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: clientID,
	})

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			ctx, span := tracing.StartSpan(ctx, "middleware.Authentication")
			defer span.End()

			auth := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				logger.WithContext(ctx).Warn("request is missing bearer token")
				return echo.NewHTTPError(http.StatusUnauthorized, "missing bearer")
			}

			raw := strings.TrimPrefix(auth, "Bearer ")
			ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
			defer cancel()

			idToken, err := verifier.Verify(ctx, raw)
			if err != nil {
				logger.WithContext(ctx).WithError(err).Warn("token is invalid")
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}

			var claims UserClaims
			if err := idToken.Claims(&claims); err != nil {
				logger.WithContext(ctx).WithError(err).Warn("failed to parse claims")
				return echo.NewHTTPError(http.StatusUnauthorized, "cannot parse claims")
			}

			ctx = utils.SetUserID(ctx, claims.Sub)
			ctx = utils.SetTenantID(ctx, claims.RealmAccess.Roles[0])

			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}
