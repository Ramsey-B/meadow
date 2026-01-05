package entitytype

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/ivy/pkg/models"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/google/uuid"
	"github.com/huandu/go-sqlbuilder"
)

// EntityTypeRepository defines the interface for entity type operations
type EntityTypeRepository interface {
	Create(ctx context.Context, tenantID string, req models.CreateEntityTypeRequest) (*models.EntityType, error)
	GetByID(ctx context.Context, tenantID string, id string) (*models.EntityType, error)
	GetByKey(ctx context.Context, tenantID string, key string) (*models.EntityType, error)
	List(ctx context.Context, tenantID string, page, pageSize int) ([]models.EntityType, int, error)
	Update(ctx context.Context, tenantID string, id string, req models.UpdateEntityTypeRequest) (*models.EntityType, error)
	Delete(ctx context.Context, tenantID string, id string) error
	GetSchemaExport(ctx context.Context, tenantID string, key string) (*models.SchemaExportResponse, error)
}

// Repository implements EntityTypeRepository
type Repository struct {
	db     database.DB
	logger ectologger.Logger
}

// NewRepository creates a new entity type repository
func NewRepository(db database.DB, logger ectologger.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

const tableName = "entity_types"

// Create creates a new entity type
func (r *Repository) Create(ctx context.Context, tenantID string, req models.CreateEntityTypeRequest) (*models.EntityType, error) {
	ctx, span := tracing.StartSpan(ctx, "EntityTypeRepository.Create")
	defer span.End()

	now := time.Now()
	id := uuid.New().String()

	sb := sqlbuilder.NewInsertBuilder()
	sb.InsertInto(tableName)
	sb.Cols("id", "tenant_id", "key", "name", "description", "schema", "version", "created_at", "updated_at")
	sb.Values(id, tenantID, req.Key, req.Name, req.Description, req.Schema, 1, now, now)

	query, args := sb.Build()

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("failed to create entity type")
		return nil, fmt.Errorf("failed to create entity type: %w", err)
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"id":        id,
		"tenant_id": tenantID,
		"key":       req.Key,
	}).Info("created entity type")

	return r.GetByID(ctx, tenantID, id)
}

// GetByID gets an entity type by ID
func (r *Repository) GetByID(ctx context.Context, tenantID string, id string) (*models.EntityType, error) {
	ctx, span := tracing.StartSpan(ctx, "EntityTypeRepository.GetByID")
	defer span.End()

	sb := sqlbuilder.NewSelectBuilder()
	sb.Select("id", "tenant_id", "key", "name", "description", "schema", "version", "created_at", "updated_at", "deleted_at")
	sb.From(tableName)
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()

	var et models.EntityType
	err := r.db.GetContext(ctx, &et, query, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.WithContext(ctx).WithError(err).Error("failed to get entity type by ID")
		return nil, fmt.Errorf("failed to get entity type: %w", err)
	}

	return &et, nil
}

// GetByKey gets an entity type by key
func (r *Repository) GetByKey(ctx context.Context, tenantID string, key string) (*models.EntityType, error) {
	ctx, span := tracing.StartSpan(ctx, "EntityTypeRepository.GetByKey")
	defer span.End()

	sb := sqlbuilder.NewSelectBuilder()
	sb.Select("id", "tenant_id", "key", "name", "description", "schema", "version", "created_at", "updated_at", "deleted_at")
	sb.From(tableName)
	sb.Where(
		sb.Equal("key", key),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()

	var et models.EntityType
	err := r.db.GetContext(ctx, &et, query, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.WithContext(ctx).WithError(err).Error("failed to get entity type by key")
		return nil, fmt.Errorf("failed to get entity type: %w", err)
	}

	return &et, nil
}

// List lists entity types for a tenant with pagination
func (r *Repository) List(ctx context.Context, tenantID string, page, pageSize int) ([]models.EntityType, int, error) {
	ctx, span := tracing.StartSpan(ctx, "EntityTypeRepository.List")
	defer span.End()

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	// Get total count
	countSb := sqlbuilder.NewSelectBuilder()
	countSb.Select("COUNT(*)")
	countSb.From(tableName)
	countSb.Where(
		countSb.Equal("tenant_id", tenantID),
		countSb.IsNull("deleted_at"),
	)
	countQuery, countArgs := countSb.Build()

	var totalCount int
	err := r.db.GetContext(ctx, &totalCount, countQuery, countArgs...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("failed to count entity types")
		return nil, 0, fmt.Errorf("failed to count entity types: %w", err)
	}

	// Get items
	sb := sqlbuilder.NewSelectBuilder()
	sb.Select("id", "tenant_id", "key", "name", "description", "schema", "version", "created_at", "updated_at", "deleted_at")
	sb.From(tableName)
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)
	sb.OrderBy("name ASC")
	sb.Limit(pageSize)
	sb.Offset(offset)

	query, args := sb.Build()

	var items []models.EntityType
	err = r.db.SelectContext(ctx, &items, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("failed to list entity types")
		return nil, 0, fmt.Errorf("failed to list entity types: %w", err)
	}

	return items, totalCount, nil
}

// Update updates an entity type
func (r *Repository) Update(ctx context.Context, tenantID string, id string, req models.UpdateEntityTypeRequest) (*models.EntityType, error) {
	ctx, span := tracing.StartSpan(ctx, "EntityTypeRepository.Update")
	defer span.End()

	// First get the existing entity type
	existing, err := r.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	sb := sqlbuilder.NewUpdateBuilder()
	sb.Update(tableName)
	sb.Set(sb.Assign("updated_at", time.Now()))

	if req.Name != nil {
		sb.Set(sb.Assign("name", *req.Name))
	}
	if req.Description != nil {
		sb.Set(sb.Assign("description", *req.Description))
	}
	if req.Schema != nil {
		// Schema update increments version
		sb.Set(
			sb.Assign("schema", *req.Schema),
			sb.Assign("version", existing.Version+1),
		)
	}

	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("failed to update entity type")
		return nil, fmt.Errorf("failed to update entity type: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"id":            id,
		"tenant_id":     tenantID,
		"rows_affected": rowsAffected,
	}).Info("updated entity type")

	return r.GetByID(ctx, tenantID, id)
}

// Delete soft deletes an entity type
func (r *Repository) Delete(ctx context.Context, tenantID string, id string) error {
	ctx, span := tracing.StartSpan(ctx, "EntityTypeRepository.Delete")
	defer span.End()

	sb := sqlbuilder.NewUpdateBuilder()
	sb.Update(tableName)
	sb.Set(sb.Assign("deleted_at", time.Now()))
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("failed to delete entity type")
		return fmt.Errorf("failed to delete entity type: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"id":            id,
		"tenant_id":     tenantID,
		"rows_affected": rowsAffected,
	}).Info("deleted entity type")

	return nil
}

// GetSchemaExport exports the schema in Lotus-compatible format
func (r *Repository) GetSchemaExport(ctx context.Context, tenantID string, key string) (*models.SchemaExportResponse, error) {
	ctx, span := tracing.StartSpan(ctx, "EntityTypeRepository.GetSchemaExport")
	defer span.End()

	et, err := r.GetByKey(ctx, tenantID, key)
	if err != nil {
		return nil, err
	}
	if et == nil {
		return nil, nil
	}

	// Parse the schema
	var schema models.EntityTypeSchema
	if err := json.Unmarshal(et.Schema, &schema); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("failed to parse entity type schema")
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	// Build required set for quick lookup
	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	// Convert to Lotus-compatible fields
	fields := make([]models.SchemaField, 0, len(schema.Properties))
	for name, prop := range schema.Properties {
		fields = append(fields, models.SchemaField{
			ID:       name,
			Name:     name, // Could be enhanced with display names
			Path:     name,
			Type:     prop.Type,
			Required: requiredSet[name],
		})
	}

	return &models.SchemaExportResponse{
		EntityType: et.Key,
		Version:    et.Version,
		Fields:     fields,
	}, nil
}
