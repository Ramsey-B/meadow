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

const configsTable = "configs"

var configStruct = database.NewStruct(new(models.Config))

// ConfigRepository handles database operations for configs
type ConfigRepository struct {
	*Repository
}

// NewConfigRepository creates a new config repository
func NewConfigRepository(db database.DB, logger ectologger.Logger) *ConfigRepository {
	return &ConfigRepository{
		Repository: NewRepository(db, logger),
	}
}

// Create creates a new config
func (r *ConfigRepository) Create(ctx context.Context, config *models.Config) error {
	ctx, span := tracing.StartSpan(ctx, "ConfigRepository.Create")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}
	config.TenantID = tenantID

	if config.ID == uuid.Nil {
		config.ID = uuid.New()
	}

	ib := database.NewInsertBuilder()
	ib.InsertInto(configsTable).
		Cols("id", "tenant_id", "integration_id", "name", "values", "enabled", "created_at", "updated_at").
		Values(config.ID, config.TenantID, config.IntegrationID, config.Name, config.Values, config.Enabled,
			sqlbuilder.Raw("NOW()"), sqlbuilder.Raw("NOW()")).
		Returning("created_at", "updated_at")

	query, args := ib.Build()
	err = r.DB().QueryRowContext(ctx, query, args...).Scan(&config.CreatedAt, &config.UpdatedAt)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"config_id": config.ID,
		}).Error("failed to create config")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to create config")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"config_id": config.ID,
	}).Debugf("Created %s", configsTable)
	return nil
}

// GetByID retrieves a config by ID (tenant-scoped)
func (r *ConfigRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Config, error) {
	ctx, span := tracing.StartSpan(ctx, "ConfigRepository.GetByID")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	sb := configStruct.SelectFrom(configsTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("id", id))

	query, args := sb.Build()
	var config models.Config
	err = r.DB().GetContext(ctx, &config, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, httperror.NewHTTPErrorf(http.StatusNotFound, "config %s does not exist", id)
	}
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"config_id": id,
		}).Error("failed to get config")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get config")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"config_id": id,
	}).Debugf("Retrieved %s", configsTable)
	return &config, nil
}

// ListByIntegration retrieves all configs for an integration
func (r *ConfigRepository) ListByIntegration(ctx context.Context, integrationID uuid.UUID) ([]models.Config, error) {
	ctx, span := tracing.StartSpan(ctx, "ConfigRepository.ListByIntegration")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	sb := configStruct.SelectFrom(configsTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("integration_id", integrationID))
	sb.OrderBy("name")

	query, args := sb.Build()
	var configs []models.Config
	err = r.DB().SelectContext(ctx, &configs, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"integration_id": integrationID,
		}).Error("failed to list configs")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list configs")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"config_count": len(configs),
	}).Debugf("Listed %s", configsTable)
	return configs, nil
}

// ListEnabled retrieves all enabled configs for the current tenant
func (r *ConfigRepository) ListEnabled(ctx context.Context) ([]models.Config, error) {
	ctx, span := tracing.StartSpan(ctx, "ConfigRepository.ListEnabled")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	sb := configStruct.SelectFrom(configsTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("enabled", true))
	sb.OrderBy("name")

	query, args := sb.Build()
	var configs []models.Config
	err = r.DB().SelectContext(ctx, &configs, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"config_count": len(configs),
		}).Error("failed to list configs")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list configs")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"config_count": len(configs),
	}).Debugf("Listed %s", configsTable)
	return configs, nil
}

// Update updates an existing config
func (r *ConfigRepository) Update(ctx context.Context, config *models.Config) error {
	ctx, span := tracing.StartSpan(ctx, "ConfigRepository.Update")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}

	ub := database.NewUpdateBuilder()
	ub.Update(configsTable).
		Set(
			ub.Assign("name", config.Name),
			ub.Assign("values", config.Values),
			ub.Assign("enabled", config.Enabled),
			ub.Assign("updated_at", sqlbuilder.Raw("NOW()")),
		).
		Where(ub.Equal("tenant_id", tenantID), ub.Equal("id", config.ID))
	ub.SQL("RETURNING updated_at")

	query, args := ub.Build()
	err = r.DB().QueryRowContext(ctx, query, args...).Scan(&config.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "config %s does not exist", config.ID)
	}
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"config_id": config.ID,
		}).Error("failed to update config")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to update config")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"config_id": config.ID,
	}).Debugf("Updated %s", configsTable)
	return nil
}

// SetEnabled enables or disables a config
func (r *ConfigRepository) SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) error {
	ctx, span := tracing.StartSpan(ctx, "ConfigRepository.SetEnabled")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"config_id": id,
		}).Error("failed to set enabled")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to set enabled")
	}

	ub := database.NewUpdateBuilder()
	ub.Update(configsTable).
		Set(
			ub.Assign("enabled", enabled),
			ub.Assign("updated_at", sqlbuilder.Raw("NOW()")),
		).
		Where(ub.Equal("tenant_id", tenantID), ub.Equal("id", id))

	query, args := ub.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"config_id": id,
		}).Error("failed to set enabled")
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"config_id": id,
		}).Error("failed to set enabled")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to set enabled")
	}
	if rows == 0 {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "config %s does not exist", id)
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"config_id": id,
	}).Debugf("Updated %s", configsTable)
	return nil
}

// Delete deletes a config by ID
func (r *ConfigRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := tracing.StartSpan(ctx, "ConfigRepository.Delete")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}

	db := database.NewDeleteBuilder()
	db.DeleteFrom(configsTable).
		Where(db.Equal("tenant_id", tenantID), db.Equal("id", id))

	query, args := db.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"config_id": id,
		}).Error("failed to delete config")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete config")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"config_id": id,
		}).Error("failed to delete config")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete config")
	}
	if rows == 0 {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "config %s does not exist", id)
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"config_id": id,
	}).Debugf("Deleted %s", configsTable)
	return nil
}
