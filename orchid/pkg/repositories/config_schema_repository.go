package repositories

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"
	"github.com/huandu/go-sqlbuilder"

	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/orchid/pkg/models"
)

const configSchemasTable = "config_schemas"

var configSchemaStruct = database.NewStruct(new(models.ConfigSchema))

// ConfigSchemaRepository handles database operations for config schemas
type ConfigSchemaRepository struct {
	*Repository
}

// NewConfigSchemaRepository creates a new config schema repository
func NewConfigSchemaRepository(db database.DB, logger ectologger.Logger) *ConfigSchemaRepository {
	return &ConfigSchemaRepository{
		Repository: NewRepository(db, logger),
	}
}

// Create creates a new config schema
func (r *ConfigSchemaRepository) Create(ctx context.Context, schema *models.ConfigSchema) error {
	ctx, span := r.StartSpan(ctx, "ConfigSchemaRepository.Create")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.LogError(ctx, "create", configSchemasTable, err)
		return err
	}
	schema.TenantID = tenantID

	if schema.ID == uuid.Nil {
		schema.ID = uuid.New()
	}

	ib := database.NewInsertBuilder()
	ib.InsertInto(configSchemasTable).
		Cols("id", "tenant_id", "integration_id", "name", "schema", "created_at", "updated_at").
		Values(schema.ID, schema.TenantID, schema.IntegrationID, schema.Name, schema.Schema,
			sqlbuilder.Raw("NOW()"), sqlbuilder.Raw("NOW()")).
		Returning("created_at", "updated_at")

	query, args := ib.Build()
	err = r.DB().QueryRowContext(ctx, query, args...).Scan(&schema.CreatedAt, &schema.UpdatedAt)
	if err != nil {
		r.LogError(ctx, "create", configSchemasTable, err)
		return err
	}

	r.LogCreate(ctx, configSchemasTable, schema.ID)
	return nil
}

// GetByID retrieves a config schema by ID (tenant-scoped)
func (r *ConfigSchemaRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ConfigSchema, error) {
	ctx, span := r.StartSpan(ctx, "ConfigSchemaRepository.GetByID")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.LogError(ctx, "get", configSchemasTable, err)
		return nil, err
	}

	sb := configSchemaStruct.SelectFrom(configSchemasTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("id", id))

	query, args := sb.Build()
	var schema models.ConfigSchema
	err = r.DB().GetContext(ctx, &schema, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NotFound("config schema %s does not exist", id)
	}
	if err != nil {
		r.LogError(ctx, "get", configSchemasTable, err)
		return nil, err
	}

	r.LogGet(ctx, configSchemasTable, id)
	return &schema, nil
}

// ListByIntegration retrieves all config schemas for an integration
func (r *ConfigSchemaRepository) ListByIntegration(ctx context.Context, integrationID uuid.UUID) ([]models.ConfigSchema, error) {
	ctx, span := r.StartSpan(ctx, "ConfigSchemaRepository.ListByIntegration")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.LogError(ctx, "list", configSchemasTable, err)
		return nil, err
	}

	sb := configSchemaStruct.SelectFrom(configSchemasTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("integration_id", integrationID))
	sb.OrderBy("name")

	query, args := sb.Build()
	var schemas []models.ConfigSchema
	err = r.DB().SelectContext(ctx, &schemas, query, args...)
	if err != nil {
		r.LogError(ctx, "list", configSchemasTable, err)
		return nil, err
	}

	r.LogList(ctx, configSchemasTable, len(schemas))
	return schemas, nil
}

// Update updates an existing config schema
func (r *ConfigSchemaRepository) Update(ctx context.Context, schema *models.ConfigSchema) error {
	ctx, span := r.StartSpan(ctx, "ConfigSchemaRepository.Update")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.LogError(ctx, "update", configSchemasTable, err)
		return err
	}

	ub := database.NewUpdateBuilder()
	ub.Update(configSchemasTable).
		Set(
			ub.Assign("name", schema.Name),
			ub.Assign("schema", schema.Schema),
			ub.Assign("updated_at", sqlbuilder.Raw("NOW()")),
		).
		Where(ub.Equal("tenant_id", tenantID), ub.Equal("id", schema.ID))
	ub.SQL("RETURNING updated_at")

	query, args := ub.Build()
	err = r.DB().QueryRowContext(ctx, query, args...).Scan(&schema.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return NotFound("config schema %s does not exist", schema.ID)
	}
	if err != nil {
		r.LogError(ctx, "update", configSchemasTable, err)
		return err
	}

	r.LogUpdate(ctx, configSchemasTable, schema.ID)
	return nil
}

// Delete deletes a config schema by ID
func (r *ConfigSchemaRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := r.StartSpan(ctx, "ConfigSchemaRepository.Delete")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.LogError(ctx, "delete", configSchemasTable, err)
		return err
	}

	db := database.NewDeleteBuilder()
	db.DeleteFrom(configSchemasTable).
		Where(db.Equal("tenant_id", tenantID), db.Equal("id", id))

	query, args := db.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.LogError(ctx, "delete", configSchemasTable, err)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		r.LogError(ctx, "delete", configSchemasTable, err)
		return err
	}
	if rows == 0 {
		return NotFound("config schema %s does not exist", id)
	}

	r.LogDelete(ctx, configSchemasTable, id)
	return nil
}
