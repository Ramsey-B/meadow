package binding

import (
	"context"
	"net/http"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/google/uuid"
)

// BindingRepository defines the interface for binding data access
type BindingRepository interface {
	Create(ctx context.Context, binding *models.Binding) (*models.Binding, error)
	GetByID(ctx context.Context, tenantID, id string) (*models.Binding, error)
	List(ctx context.Context, tenantID string) ([]*models.Binding, error)
	ListEnabled(ctx context.Context, tenantID string) ([]*models.Binding, error)
	Update(ctx context.Context, binding *models.Binding) (*models.Binding, error)
	Delete(ctx context.Context, tenantID, id string) error
}

// Repository implements BindingRepository
type Repository struct {
	db     database.DB
	logger ectologger.Logger
}

// NewRepository creates a new binding repository
func NewRepository(db database.DB, logger ectologger.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new binding
func (r *Repository) Create(ctx context.Context, binding *models.Binding) (*models.Binding, error) {
	ctx, span := tracing.StartSpan(ctx, "BindingRepository.Create")
	defer span.End()

	// Generate ID if not provided
	if binding.ID == "" {
		binding.ID = uuid.New().String()
	}

	now := Now()
	binding.CreatedAt = now
	binding.UpdatedAt = now

	row := FromBinding(binding)
	ib := bindingStruct.InsertInto(bindingsTable, row)
	sql, args := ib.Build()

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"id":         binding.ID,
		"tenant_id":  binding.TenantID,
		"mapping_id": binding.MappingID,
		"name":       binding.Name,
	}).Debug("Creating binding")

	_, err := r.db.ExecContext(ctx, sql, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to create binding")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to create binding")
	}

	return binding, nil
}

// GetByID retrieves a binding by ID
func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*models.Binding, error) {
	ctx, span := tracing.StartSpan(ctx, "BindingRepository.GetByID")
	defer span.End()

	sb := bindingStruct.SelectFrom(bindingsTable)
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
	)

	sql, args := sb.Build()

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"id":        id,
		"tenant_id": tenantID,
	}).Debug("Getting binding by ID")

	var row BindingRow
	err := r.db.GetContext(ctx, &row, sql, args...)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, httperror.NewHTTPError(http.StatusNotFound, "binding not found")
		}
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get binding")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get binding")
	}

	return ToBinding(&row), nil
}

// List retrieves all bindings for a tenant
func (r *Repository) List(ctx context.Context, tenantID string) ([]*models.Binding, error) {
	ctx, span := tracing.StartSpan(ctx, "BindingRepository.List")
	defer span.End()

	sb := bindingStruct.SelectFrom(bindingsTable)
	sb.Where(sb.Equal("tenant_id", tenantID))
	sb.OrderBy("created_at").Desc()

	sql, args := sb.Build()

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id": tenantID,
	}).Debug("Listing bindings")

	var rows []BindingRow
	err := r.db.SelectContext(ctx, &rows, sql, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to list bindings")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list bindings")
	}

	return ToBindings(rows), nil
}

// ListEnabled retrieves all enabled bindings for a tenant
func (r *Repository) ListEnabled(ctx context.Context, tenantID string) ([]*models.Binding, error) {
	ctx, span := tracing.StartSpan(ctx, "BindingRepository.ListEnabled")
	defer span.End()

	sb := bindingStruct.SelectFrom(bindingsTable)
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("is_enabled", true),
	)
	sb.OrderBy("created_at").Desc()

	sql, args := sb.Build()

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id": tenantID,
	}).Debug("Listing enabled bindings")

	var rows []BindingRow
	err := r.db.SelectContext(ctx, &rows, sql, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to list enabled bindings")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list enabled bindings")
	}

	return ToBindings(rows), nil
}

// Update updates an existing binding
func (r *Repository) Update(ctx context.Context, binding *models.Binding) (*models.Binding, error) {
	ctx, span := tracing.StartSpan(ctx, "BindingRepository.Update")
	defer span.End()

	binding.UpdatedAt = Now()

	ub := bindingStruct.Update(bindingsTable, FromBinding(binding))
	ub.Where(
		ub.Equal("id", binding.ID),
		ub.Equal("tenant_id", binding.TenantID),
	)

	sql, args := ub.Build()

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"id":        binding.ID,
		"tenant_id": binding.TenantID,
	}).Debug("Updating binding")

	result, err := r.db.ExecContext(ctx, sql, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to update binding")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to update binding")
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, httperror.NewHTTPError(http.StatusNotFound, "binding not found")
	}

	return binding, nil
}

// Delete deletes a binding
func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	ctx, span := tracing.StartSpan(ctx, "BindingRepository.Delete")
	defer span.End()

	db := bindingStruct.DeleteFrom(bindingsTable)
	db.Where(
		db.Equal("id", id),
		db.Equal("tenant_id", tenantID),
	)

	sql, args := db.Build()

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"id":        id,
		"tenant_id": tenantID,
	}).Debug("Deleting binding")

	result, err := r.db.ExecContext(ctx, sql, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to delete binding")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete binding")
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return httperror.NewHTTPError(http.StatusNotFound, "binding not found")
	}

	return nil
}

