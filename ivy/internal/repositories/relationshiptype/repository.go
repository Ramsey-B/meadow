package relationshiptype

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/ivy/pkg/models"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/google/uuid"
	"github.com/huandu/go-sqlbuilder"
)

// RelationshipTypeRepository defines the interface for relationship type operations
type RelationshipTypeRepository interface {
	Create(ctx context.Context, tenantID string, req models.CreateRelationshipTypeRequest) (*models.RelationshipType, error)
	GetByID(ctx context.Context, tenantID string, id string) (*models.RelationshipType, error)
	GetByKey(ctx context.Context, tenantID string, key string) (*models.RelationshipType, error)
	List(ctx context.Context, tenantID string, page, pageSize int) ([]models.RelationshipType, int, error)
	Update(ctx context.Context, tenantID string, id string, req models.UpdateRelationshipTypeRequest) (*models.RelationshipType, error)
	Delete(ctx context.Context, tenantID string, id string) error
}

// Repository implements RelationshipTypeRepository
type Repository struct {
	db     database.DB
	logger ectologger.Logger
}

// NewRepository creates a new relationship type repository
func NewRepository(db database.DB, logger ectologger.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

const tableName = "relationship_types"

// Create creates a new relationship type
func (r *Repository) Create(ctx context.Context, tenantID string, req models.CreateRelationshipTypeRequest) (*models.RelationshipType, error) {
	ctx, span := tracing.StartSpan(ctx, "RelationshipTypeRepository.Create")
	defer span.End()

	now := time.Now()
	id := uuid.New().String()

	cardinality := req.Cardinality
	if cardinality == "" {
		cardinality = models.CardinalityManyToMany
	}

	sb := sqlbuilder.NewInsertBuilder()
	sb.InsertInto(tableName)
	sb.Cols("id", "tenant_id", "key", "name", "description", "from_entity_type", "to_entity_type", "cardinality", "properties", "created_at", "updated_at")
	sb.Values(id, tenantID, req.Key, req.Name, req.Description, req.FromEntityType, req.ToEntityType, cardinality, req.Schema, now, now)

	query, args := sb.Build()

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"id":        id,
			"tenant_id": tenantID,
			"key":       req.Key,
		}).Error("failed to create relationship type")
		return nil, httperror.NewHTTPErrorf(http.StatusInternalServerError, "failed to create relationship type: %s", err.Error())
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"id":        id,
		"tenant_id": tenantID,
		"key":       req.Key,
	}).Info("created relationship type")

	return r.GetByID(ctx, tenantID, id)
}

// GetByID gets a relationship type by ID
func (r *Repository) GetByID(ctx context.Context, tenantID string, id string) (*models.RelationshipType, error) {
	ctx, span := tracing.StartSpan(ctx, "RelationshipTypeRepository.GetByID")
	defer span.End()

	sb := sqlbuilder.NewSelectBuilder()
	sb.Select("id", "tenant_id", "key", "name", "description", "from_entity_type", "to_entity_type", "cardinality", "properties", "created_at", "updated_at", "deleted_at")
	sb.From(tableName)
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()

	var rt models.RelationshipType
	err := r.db.GetContext(ctx, &rt, query, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"id":        id,
			"tenant_id": tenantID,
		}).Error("failed to get relationship type by ID")
		return nil, httperror.NewHTTPErrorf(http.StatusInternalServerError, "failed to get relationship type: %w", err)
	}

	return &rt, nil
}

// GetByKey gets a relationship type by key
func (r *Repository) GetByKey(ctx context.Context, tenantID string, key string) (*models.RelationshipType, error) {
	ctx, span := tracing.StartSpan(ctx, "RelationshipTypeRepository.GetByKey")
	defer span.End()

	sb := sqlbuilder.NewSelectBuilder()
	sb.Select("id", "tenant_id", "key", "name", "description", "from_entity_type", "to_entity_type", "cardinality", "properties", "created_at", "updated_at", "deleted_at")
	sb.From(tableName)
	sb.Where(
		sb.Equal("key", key),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()

	var rt models.RelationshipType
	err := r.db.GetContext(ctx, &rt, query, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.WithContext(ctx).WithError(err).Error("failed to get relationship type by key")
		return nil, fmt.Errorf("failed to get relationship type: %w", err)
	}

	return &rt, nil
}

// List lists relationship types for a tenant with pagination
func (r *Repository) List(ctx context.Context, tenantID string, page, pageSize int) ([]models.RelationshipType, int, error) {
	ctx, span := tracing.StartSpan(ctx, "RelationshipTypeRepository.List")
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
		r.logger.WithContext(ctx).WithError(err).Error("failed to count relationship types")
		return nil, 0, fmt.Errorf("failed to count relationship types: %w", err)
	}

	// Get items
	sb := sqlbuilder.NewSelectBuilder()
	sb.Select("id", "tenant_id", "key", "name", "description", "from_entity_type", "to_entity_type", "cardinality", "properties", "created_at", "updated_at", "deleted_at")
	sb.From(tableName)
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)
	sb.OrderBy("name ASC")
	sb.Limit(pageSize)
	sb.Offset(offset)

	query, args := sb.Build()

	var items []models.RelationshipType
	err = r.db.SelectContext(ctx, &items, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"tenant_id": tenantID,
			"page":      page,
			"page_size": pageSize,
		}).Error("failed to list relationship types")
		return nil, 0, httperror.NewHTTPErrorf(http.StatusInternalServerError, "failed to list relationship types: %w", err)
	}

	return items, totalCount, nil
}

// Update updates a relationship type
func (r *Repository) Update(ctx context.Context, tenantID string, id string, req models.UpdateRelationshipTypeRequest) (*models.RelationshipType, error) {
	ctx, span := tracing.StartSpan(ctx, "RelationshipTypeRepository.Update")
	defer span.End()

	// First get the existing relationship type
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
	if req.Cardinality != nil {
		sb.Set(sb.Assign("cardinality", *req.Cardinality))
	}
	if req.Schema != nil {
		sb.Set(sb.Assign("properties", *req.Schema))
	}

	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"id":        id,
			"tenant_id": tenantID,
		}).Error("failed to update relationship type")
		return nil, httperror.NewHTTPErrorf(http.StatusInternalServerError, "failed to update relationship type: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"id":            id,
		"tenant_id":     tenantID,
		"rows_affected": rowsAffected,
	}).Info("updated relationship type")

	return r.GetByID(ctx, tenantID, id)
}

// Delete soft deletes a relationship type
func (r *Repository) Delete(ctx context.Context, tenantID string, id string) error {
	ctx, span := tracing.StartSpan(ctx, "RelationshipTypeRepository.Delete")
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
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"id":        id,
			"tenant_id": tenantID,
		}).Error("failed to delete relationship type")
		return httperror.NewHTTPErrorf(http.StatusInternalServerError, "failed to delete relationship type: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"id":            id,
		"tenant_id":     tenantID,
		"rows_affected": rowsAffected,
	}).Info("deleted relationship type")

	return nil
}
