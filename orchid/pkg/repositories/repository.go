package repositories

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	appctx "github.com/Ramsey-B/stem/pkg/context"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

// NotFound returns a 404 HTTP error with a descriptive message
func NotFound(format string, args ...any) error {
	return httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf(format, args...))
}

// Unauthorized returns a 401 HTTP error
func Unauthorized(message string) error {
	return httperror.NewHTTPError(http.StatusUnauthorized, message)
}

// BadRequest returns a 400 HTTP error
func BadRequest(message string) error {
	return httperror.NewHTTPError(http.StatusBadRequest, message)
}

// Repository provides common database operations with tenant isolation
type Repository struct {
	db     database.DB
	logger ectologger.Logger
}

// NewRepository creates a new base repository
func NewRepository(db database.DB, logger ectologger.Logger) *Repository {
	return &Repository{db: db, logger: logger}
}

// DB returns the database instance
func (r *Repository) DB() database.DB {
	return r.db
}

// GetTenantID extracts and validates tenant_id from context
func GetTenantID(ctx context.Context) (uuid.UUID, error) {
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

// MustGetTenantID extracts tenant_id from context, panics if not present
func MustGetTenantID(ctx context.Context) uuid.UUID {
	tenantID, err := GetTenantID(ctx)
	if err != nil {
		panic(err)
	}
	return tenantID
}

// StartSpan starts a new tracing span
func (r *Repository) StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return tracing.StartSpan(ctx, name)
}

// LogError logs an error for a repository operation
func (r *Repository) LogError(ctx context.Context, operation, table string, err error) {
	r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
		"operation": operation,
		"table":     table,
	}).Error("Repository operation failed")
}

// LogCreate logs a successful create operation
func (r *Repository) LogCreate(ctx context.Context, table string, id any) {
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"table": table,
		"id":    id,
	}).Debug("Created record")
}

// LogGet logs a successful get operation
func (r *Repository) LogGet(ctx context.Context, table string, id any) {
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"table": table,
		"id":    id,
	}).Debug("Retrieved record")
}

// LogUpdate logs a successful update operation
func (r *Repository) LogUpdate(ctx context.Context, table string, id any) {
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"table": table,
		"id":    id,
	}).Debug("Updated record")
}

// LogDelete logs a successful delete operation
func (r *Repository) LogDelete(ctx context.Context, table string, id any) {
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"table": table,
		"id":    id,
	}).Debug("Deleted record")
}

// LogList logs a successful list operation
func (r *Repository) LogList(ctx context.Context, table string, count int) {
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"table": table,
		"count": count,
	}).Debug("Listed records")
}
