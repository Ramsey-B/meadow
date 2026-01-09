package repositories

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"
	"github.com/huandu/go-sqlbuilder"

	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

const integrationsTable = "integrations"

var integrationStruct = database.NewStruct(new(models.Integration))

// IntegrationRepository handles database operations for integrations
type IntegrationRepository struct {
	*Repository
}

// NewIntegrationRepository creates a new integration repository
func NewIntegrationRepository(db database.DB, logger ectologger.Logger) *IntegrationRepository {
	return &IntegrationRepository{
		Repository: NewRepository(db, logger),
	}
}

// Create creates a new integration
func (r *IntegrationRepository) Create(ctx context.Context, integration *models.Integration) error {
	ctx, span := tracing.StartSpan(ctx, "IntegrationRepository.Create")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}
	integration.TenantID = tenantID

	if integration.ID == uuid.Nil {
		integration.ID = uuid.New()
	}

	ib := database.NewInsertBuilder()
	ib.InsertInto(integrationsTable).
		Cols("id", "tenant_id", "name", "description", "config_schema", "created_at", "updated_at").
		Values(integration.ID, integration.TenantID, integration.Name, integration.Description, integration.ConfigSchema,
			sqlbuilder.Raw("NOW()"), sqlbuilder.Raw("NOW()")).
		Returning("created_at", "updated_at")

	query, args := ib.Build()
	err = r.DB().QueryRowContext(ctx, query, args...).Scan(&integration.CreatedAt, &integration.UpdatedAt)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"integration_id": integration.ID,
		}).Error("failed to create integration")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to create integration")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"integration_id": integration.ID,
	}).Debugf("Created %s", integrationsTable)
	return nil
}

// GetByID retrieves an integration by ID (tenant-scoped)
func (r *IntegrationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Integration, error) {
	ctx, span := tracing.StartSpan(ctx, "IntegrationRepository.GetByID")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	sb := integrationStruct.SelectFrom(integrationsTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("id", id))

	query, args := sb.Build()
	var integration models.Integration
	err = r.DB().GetContext(ctx, &integration, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, httperror.NewHTTPErrorf(http.StatusNotFound, "integration %s does not exist", id)
	}
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"integration_id": id,
		}).Error("failed to get integration by ID")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get integration by ID")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"integration_id": id,
	}).Debugf("Retrieved %s by ID: %s", integrationsTable, id)
	return &integration, nil
}

// GetByName retrieves an integration by name (tenant-scoped)
func (r *IntegrationRepository) GetByName(ctx context.Context, name string) (*models.Integration, error) {
	ctx, span := tracing.StartSpan(ctx, "IntegrationRepository.GetByName")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	sb := integrationStruct.SelectFrom(integrationsTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("name", name))

	query, args := sb.Build()
	var integration models.Integration
	err = r.DB().GetContext(ctx, &integration, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, httperror.NewHTTPErrorf(http.StatusNotFound, "integration '%s' does not exist", name)
	}
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"integration_name": name,
		}).Error("failed to get integration by name")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get integration by name")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"integration_name": name,
	}).Debugf("Retrieved %s by name: %s", integrationsTable, name)
	return &integration, nil
}

// List retrieves all integrations for the current tenant
func (r *IntegrationRepository) List(ctx context.Context) ([]models.Integration, error) {
	ctx, span := tracing.StartSpan(ctx, "IntegrationRepository.List")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	sb := integrationStruct.SelectFrom(integrationsTable)
	sb.Where(sb.Equal("tenant_id", tenantID))
	sb.OrderBy("name")

	query, args := sb.Build()
	var integrations []models.Integration
	err = r.DB().SelectContext(ctx, &integrations, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"integration_count": len(integrations),
		}).Error("failed to list integrations")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list integrations")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"integration_count": len(integrations),
	}).Debugf("Listed %s", integrationsTable)
	return integrations, nil
}

// Update updates an existing integration
func (r *IntegrationRepository) Update(ctx context.Context, integration *models.Integration) error {
	ctx, span := tracing.StartSpan(ctx, "IntegrationRepository.Update")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}

	ub := database.NewUpdateBuilder()
	ub.Update(integrationsTable).
		Set(
			ub.Assign("name", integration.Name),
			ub.Assign("description", integration.Description),
			ub.Assign("config_schema", integration.ConfigSchema),
			ub.Assign("updated_at", sqlbuilder.Raw("NOW()")),
		).
		Where(ub.Equal("tenant_id", tenantID), ub.Equal("id", integration.ID))
	ub.SQL("RETURNING updated_at")

	query, args := ub.Build()
	result := r.DB().QueryRowContext(ctx, query, args...)

	err = result.Scan(&integration.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "integration %s does not exist", integration.ID)
	}
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"integration_id": integration.ID,
		}).Error("failed to update integration")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to update integration")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"integration_id": integration.ID,
	}).Debugf("Updated %s", integrationsTable)
	return nil
}

// Delete deletes an integration by ID
func (r *IntegrationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := tracing.StartSpan(ctx, "IntegrationRepository.Delete")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}

	db := database.NewDeleteBuilder()
	db.DeleteFrom(integrationsTable).
		Where(db.Equal("tenant_id", tenantID), db.Equal("id", id))

	query, args := db.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"integration_id": id,
		}).Error("failed to delete integration")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete integration")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"integration_id": id,
		}).Error("failed to delete integration")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete integration")
	}
	if rows == 0 {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "integration %s does not exist", id)
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"integration_id": id,
	}).Debugf("Deleted %s", integrationsTable)
	return nil
}

// DeleteByTenantID deletes all integrations for a tenant (for testing cleanup)
func (r *IntegrationRepository) DeleteByTenantID(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	ctx, span := tracing.StartSpan(ctx, "IntegrationRepository.DeleteByTenantID")
	defer span.End()

	db := database.NewDeleteBuilder()
	db.DeleteFrom(integrationsTable).
		Where(db.Equal("tenant_id", tenantID))

	query, args := db.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"tenant_id": tenantID,
		}).Error("failed to delete integrations by tenant")
		return 0, err
	}

	rows, _ := result.RowsAffected()
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id": tenantID,
		"count":     rows,
	}).Info("Deleted integrations by tenant")
	return rows, nil
}
