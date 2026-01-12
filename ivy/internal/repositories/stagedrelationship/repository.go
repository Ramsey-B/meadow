package stagedrelationship

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/google/uuid"
	"github.com/huandu/go-sqlbuilder"
	"github.com/jmoiron/sqlx"

	"github.com/Ramsey-B/ivy/pkg/models"
)

var relationshipIDNamespace = uuid.MustParse("7c1b2f58-9d6a-4d41-a15a-2d35f4d23270")

// ComputeDeterministicID returns the deterministic relationship ID used for upserts.
// Unique per: tenant_id, relationship_type, from (type, source_id, integration), to (type, source_id, integration), config_id
func ComputeDeterministicID(tenantID, relationshipType, fromEntityType, fromSourceID, fromIntegration, toEntityType, toSourceID, toIntegration, configID string) string {
	return uuid.NewSHA1(relationshipIDNamespace, []byte(fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s",
		tenantID, relationshipType, fromEntityType, fromSourceID, fromIntegration, toEntityType, toSourceID, toIntegration, configID))).String()
}

// Repository handles staged relationship persistence
type Repository struct {
	db     database.DB
	logger ectologger.Logger
}

// NewRepository creates a new staged relationship repository
func NewRepository(db database.DB, logger ectologger.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

// allColumns is the standard column list for SELECT queries
const allColumns = `id, tenant_id, config_id, integration, source_key, relationship_type,
	from_entity_type, from_source_id, from_integration, from_staged_entity_id,
	to_entity_type, to_source_id, to_integration, to_staged_entity_id,
	criteria_id, source_execution_id, execution_id, last_seen_execution,
	data, created_at, updated_at, deleted_at`

// Create creates or updates a direct staged relationship.
func (r *Repository) Create(ctx context.Context, tenantID string, req models.CreateStagedRelationshipRequest) (*models.StagedRelationship, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationship.Repository.Create")
	defer span.End()

	log := r.logger.WithContext(ctx).WithFields(map[string]any{
		"method":            "Create",
		"tenant_id":         tenantID,
		"config_id":         req.ConfigID,
		"relationship_type": req.RelationshipType,
		"from_source_id":    req.FromSourceID,
		"to_source_id":      req.ToSourceID,
	})

	// Deterministic ID based on natural key
	id := ComputeDeterministicID(tenantID, req.RelationshipType,
		req.FromEntityType, req.FromSourceID, req.FromIntegration,
		req.ToEntityType, req.ToSourceID, req.ToIntegration, req.ConfigID)
	now := time.Now().UTC()

	// Handle nil/empty data - PostgreSQL JSONB requires valid JSON or NULL
	// Use json.RawMessage("null") for empty data to avoid "invalid input syntax for type json" errors
	var data json.RawMessage
	if len(req.Data) > 0 {
		data = req.Data
	} else {
		data = json.RawMessage("null")
	}

	relationship := &models.StagedRelationship{
		ID:               id,
		TenantID:         tenantID,
		ConfigID:         req.ConfigID,
		Integration:      req.Integration,
		SourceKey:        req.SourceKey,
		RelationshipType: req.RelationshipType,

		FromEntityType:  req.FromEntityType,
		FromSourceID:    req.FromSourceID,
		FromIntegration: req.FromIntegration,

		ToEntityType:  req.ToEntityType,
		ToSourceID:    req.ToSourceID,
		ToIntegration: req.ToIntegration,

		CriteriaID: req.CriteriaID,

		SourceExecutionID: req.SourceExecutionID,
		ExecutionID:       req.SourceExecutionID,
		LastSeenExecution: req.SourceExecutionID,

		Data:      data,
		CreatedAt: now,
		UpdatedAt: now,
	}

	query := `
		INSERT INTO staged_relationships (
			id, tenant_id, config_id, integration, source_key, relationship_type,
			from_entity_type, from_source_id, from_integration, from_staged_entity_id,
			to_entity_type, to_source_id, to_integration, to_staged_entity_id,
			criteria_id, source_execution_id, execution_id, last_seen_execution,
			data, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
		ON CONFLICT (tenant_id, relationship_type, 
		             from_entity_type, from_source_id, from_integration,
		             to_entity_type, to_source_id, to_integration, config_id)
		DO UPDATE SET
			from_staged_entity_id = COALESCE(EXCLUDED.from_staged_entity_id, staged_relationships.from_staged_entity_id),
			to_staged_entity_id = COALESCE(EXCLUDED.to_staged_entity_id, staged_relationships.to_staged_entity_id),
			criteria_id = COALESCE(EXCLUDED.criteria_id, staged_relationships.criteria_id),
			source_execution_id = EXCLUDED.source_execution_id,
			execution_id = EXCLUDED.execution_id,
			last_seen_execution = EXCLUDED.last_seen_execution,
			data = EXCLUDED.data,
			updated_at = EXCLUDED.updated_at,
			deleted_at = NULL
	`

	if _, err := r.db.ExecContext(ctx, query,
		relationship.ID, relationship.TenantID, relationship.ConfigID, relationship.Integration, relationship.SourceKey, relationship.RelationshipType,
		relationship.FromEntityType, relationship.FromSourceID, relationship.FromIntegration, relationship.FromStagedEntityID,
		relationship.ToEntityType, relationship.ToSourceID, relationship.ToIntegration, relationship.ToStagedEntityID,
		relationship.CriteriaID, relationship.SourceExecutionID, relationship.ExecutionID, relationship.LastSeenExecution,
		relationship.Data, relationship.CreatedAt, relationship.UpdatedAt,
	); err != nil {
		log.WithError(err).Error("Failed to create staged relationship")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to create staged relationship")
	}

	log.WithFields(map[string]any{"id": id}).Info("Upserted staged relationship")
	return r.Get(ctx, tenantID, id)
}

// Get retrieves a staged relationship by ID
func (r *Repository) Get(ctx context.Context, tenantID string, id string) (*models.StagedRelationship, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationship.Repository.Get")
	defer span.End()

	query := fmt.Sprintf(`SELECT %s FROM staged_relationships WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, allColumns)

	var relationship models.StagedRelationship
	if err := r.db.GetContext(ctx, &relationship, query, id, tenantID); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, httperror.NewHTTPErrorf(http.StatusNotFound, "staged relationship %s not found", id)
		}
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get staged relationship")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get staged relationship")
	}

	return &relationship, nil
}

// GetBySourceIDs retrieves a relationship by its source identifiers
func (r *Repository) GetBySourceIDs(ctx context.Context, tenantID, relationshipType, fromEntityType, fromSourceID, fromIntegration, toEntityType, toSourceID, toIntegration string) (*models.StagedRelationship, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationship.Repository.GetBySourceIDs")
	defer span.End()

	query := fmt.Sprintf(`
		SELECT %s FROM staged_relationships
		WHERE tenant_id = $1
		  AND relationship_type = $2
		  AND from_entity_type = $3
		  AND from_source_id = $4
		  AND from_integration = $5
		  AND to_entity_type = $6
		  AND to_source_id = $7
		  AND to_integration = $8
		  AND deleted_at IS NULL
	`, allColumns)

	var relationship models.StagedRelationship
	if err := r.db.GetContext(ctx, &relationship, query,
		tenantID, relationshipType, fromEntityType, fromSourceID, fromIntegration, toEntityType, toSourceID, toIntegration,
	); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get staged relationship by source IDs")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get staged relationship")
	}

	return &relationship, nil
}

// GetByEntityID retrieves all relationships for a resolved entity (from or to)
func (r *Repository) GetByEntityID(ctx context.Context, tenantID string, entityID string) ([]models.StagedRelationship, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationship.Repository.GetByEntityID")
	defer span.End()

	query := fmt.Sprintf(`
		SELECT %s FROM staged_relationships
		WHERE tenant_id = $1
		  AND (from_staged_entity_id = $2 OR to_staged_entity_id = $2)
		  AND deleted_at IS NULL
	`, allColumns)

	var relationships []models.StagedRelationship
	if err := r.db.SelectContext(ctx, &relationships, query, tenantID, entityID); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get staged relationships by entity_id")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get staged relationships")
	}

	return relationships, nil
}

// GetUnresolvedRelationships returns relationships where entity references haven't been resolved yet.
func (r *Repository) GetUnresolvedRelationships(ctx context.Context, tenantID string, limit int) ([]models.StagedRelationship, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationship.Repository.GetUnresolvedRelationships")
	defer span.End()

	var query string
	var args []any

	if tenantID == "" {
		query = fmt.Sprintf(`
			SELECT %s FROM staged_relationships
			WHERE (from_staged_entity_id IS NULL OR to_staged_entity_id IS NULL)
			  AND deleted_at IS NULL
			ORDER BY created_at ASC
			LIMIT $1
		`, allColumns)
		args = []any{limit}
	} else {
		query = fmt.Sprintf(`
			SELECT %s FROM staged_relationships
			WHERE tenant_id = $1
			  AND (from_staged_entity_id IS NULL OR to_staged_entity_id IS NULL)
			  AND deleted_at IS NULL
			ORDER BY created_at ASC
			LIMIT $2
		`, allColumns)
		args = []any{tenantID, limit}
	}

	var relationships []models.StagedRelationship
	if err := r.db.SelectContext(ctx, &relationships, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get unresolved relationships")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get unresolved relationships")
	}

	return relationships, nil
}

// GetIncompleteByFromSource finds incomplete relationships where from_staged_entity_id is NULL
func (r *Repository) GetIncompleteByFromSource(ctx context.Context, tenantID, fromEntityType, fromSourceID, fromIntegration string) ([]models.StagedRelationship, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationship.Repository.GetIncompleteByFromSource")
	defer span.End()

	query := fmt.Sprintf(`
		SELECT %s FROM staged_relationships
		WHERE tenant_id = $1
		  AND from_entity_type = $2
		  AND from_source_id = $3
		  AND from_integration = $4
		  AND from_staged_entity_id IS NULL
		  AND deleted_at IS NULL
	`, allColumns)

	var rels []models.StagedRelationship
	if err := r.db.SelectContext(ctx, &rels, query, tenantID, fromEntityType, fromSourceID, fromIntegration); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get incomplete relationships by from source")
		return nil, err
	}

	return rels, nil
}

// GetIncompleteByToSource finds incomplete relationships where to_staged_entity_id is NULL
func (r *Repository) GetIncompleteByToSource(ctx context.Context, tenantID, toEntityType, toSourceID, toIntegration string) ([]models.StagedRelationship, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationship.Repository.GetIncompleteByToSource")
	defer span.End()

	query := fmt.Sprintf(`
		SELECT %s FROM staged_relationships
		WHERE tenant_id = $1
		  AND to_entity_type = $2
		  AND to_source_id = $3
		  AND to_integration = $4
		  AND to_staged_entity_id IS NULL
		  AND deleted_at IS NULL
	`, allColumns)

	var rels []models.StagedRelationship
	if err := r.db.SelectContext(ctx, &rels, query, tenantID, toEntityType, toSourceID, toIntegration); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get incomplete relationships by to source")
		return nil, err
	}

	return rels, nil
}

// UpdateEntityReferences updates the resolved entity ID references for a relationship
func (r *Repository) UpdateEntityReferences(ctx context.Context, tenantID string, id string, fromEntityID, toEntityID *string) error {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationship.Repository.UpdateEntityReferences")
	defer span.End()

	now := time.Now().UTC()
	query := `
		UPDATE staged_relationships 
		SET from_staged_entity_id = COALESCE($3, from_staged_entity_id),
		    to_staged_entity_id = COALESCE($4, to_staged_entity_id),
		    updated_at = $5
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, id, tenantID, fromEntityID, toEntityID, now)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to update entity references")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to update entity references")
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf("staged relationship %s not found", id))
	}

	return nil
}

// SoftDelete marks a staged relationship as deleted
func (r *Repository) SoftDelete(ctx context.Context, tenantID string, id string) error {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationship.Repository.SoftDelete")
	defer span.End()

	now := time.Now().UTC()
	query := `UPDATE staged_relationships SET deleted_at = $3 WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, id, tenantID, now)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to soft delete staged relationship")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete staged relationship")
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf("staged relationship %s not found", id))
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{"id": id}).Info("Soft deleted staged relationship")
	return nil
}

// SoftDeleteByEntityID marks all relationships involving an entity as deleted
func (r *Repository) SoftDeleteByEntityID(ctx context.Context, tenantID string, entityID string) (int64, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationship.Repository.SoftDeleteByEntityID")
	defer span.End()

	now := time.Now().UTC()
	query := `
		UPDATE staged_relationships SET deleted_at = $3 
		WHERE tenant_id = $1 
		  AND (from_staged_entity_id = $2 OR to_staged_entity_id = $2) 
		  AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, tenantID, entityID, now)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to soft delete staged relationships by entity_id")
		return 0, httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete staged relationships")
	}

	rows, _ := result.RowsAffected()
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"entity_id": entityID,
		"count":     rows,
	}).Info("Soft deleted staged relationships by entity_id")
	return rows, nil
}

// SoftDeleteByEntityIDAndSourceKey marks all relationships involving an entity as deleted, scoped to a source_key
func (r *Repository) SoftDeleteByEntityIDAndSourceKey(ctx context.Context, tenantID string, entityID string, sourceKey string) (int64, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationship.Repository.SoftDeleteByEntityIDAndSourceKey")
	defer span.End()

	now := time.Now().UTC()
	query := `
		UPDATE staged_relationships SET deleted_at = $4 
		WHERE tenant_id = $1 
		  AND source_key = $3 
		  AND (from_staged_entity_id = $2 OR to_staged_entity_id = $2) 
		  AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, tenantID, entityID, sourceKey, now)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to soft delete staged relationships by entity_id and source_key")
		return 0, httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete staged relationships")
	}

	rows, _ := result.RowsAffected()
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"entity_id":  entityID,
		"source_key": sourceKey,
		"count":      rows,
	}).Info("Soft deleted staged relationships by entity_id and source_key")
	return rows, nil
}

// MarkDeletedExceptExecution marks relationships as deleted if they weren't seen in the given execution
func (r *Repository) MarkDeletedExceptExecution(ctx context.Context, tenantID, sourceKey, executionID string, relationshipType *string) (int64, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationship.Repository.MarkDeletedExceptExecution")
	defer span.End()

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("staged_relationships")
	sb.Set(sb.Assign("deleted_at", now))

	where := []string{
		sb.Equal("tenant_id", tenantID),
		sb.Equal("source_key", sourceKey),
		sb.NotEqual("last_seen_execution", executionID),
		sb.IsNull("deleted_at"),
	}
	if relationshipType != nil {
		where = append(where, sb.Equal("relationship_type", *relationshipType))
	}
	sb.Where(where...)

	query, args := sb.Build()
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to mark deleted except execution")
		return 0, httperror.NewHTTPError(http.StatusInternalServerError, "failed to mark deleted")
	}

	rows, _ := result.RowsAffected()
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"source_key":   sourceKey,
		"execution_id": executionID,
		"count":        rows,
	}).Info("Marked relationships deleted")
	return rows, nil
}

// MarkSeenInExecution updates the last_seen_execution field for relationships seen in an execution
func (r *Repository) MarkSeenInExecution(ctx context.Context, tenantID, executionID string, relationshipIDs []string) error {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationship.Repository.MarkSeenInExecution")
	defer span.End()

	if len(relationshipIDs) == 0 {
		return nil
	}

	query := `
		UPDATE staged_relationships
		SET last_seen_execution = $1, updated_at = NOW()
		WHERE tenant_id = $2 AND id = ANY($3)
	`

	_, err := r.db.ExecContext(ctx, query, executionID, tenantID, relationshipIDs)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to mark relationships as seen")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to mark relationships as seen")
	}

	return nil
}

// FindNotSeenInExecution finds relationships that were not seen in the given execution
func (r *Repository) FindNotSeenInExecution(ctx context.Context, tenantID, sourceKey, executionID string) ([]models.StagedRelationship, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationship.Repository.FindNotSeenInExecution")
	defer span.End()

	query := fmt.Sprintf(`
		SELECT %s FROM staged_relationships
		WHERE tenant_id = $1 
		  AND source_key = $2
		  AND deleted_at IS NULL
		  AND (last_seen_execution IS NULL OR last_seen_execution != $3)
		ORDER BY created_at
	`, allColumns)

	var results []models.StagedRelationship
	err := r.db.SelectContext(ctx, &results, query, tenantID, sourceKey, executionID)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to find relationships not seen in execution")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to find relationships")
	}

	return results, nil
}

// GetDB returns the underlying database connection for transactions
func (r *Repository) GetDB() *sqlx.DB {
	return r.db.Unsafe()
}

// ExecRaw executes a raw query
func (r *Repository) ExecRaw(ctx context.Context, query string, args ...interface{}) (interface{}, error) {
	return r.db.ExecContext(ctx, query, args...)
}
