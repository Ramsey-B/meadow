package mergedentity

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

	"github.com/Ramsey-B/ivy/pkg/models"
)

// Repository handles merged entity persistence
type Repository struct {
	db     database.DB
	logger ectologger.Logger
}

// NewRepository creates a new merged entity repository
func NewRepository(db database.DB, logger ectologger.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

// DB exposes the underlying database handle for transactional operations.
func (r *Repository) DB() database.DB {
	return r.db
}

// Create creates a new merged entity
func (r *Repository) Create(ctx context.Context, entity *models.MergedEntity) (*models.MergedEntity, error) {
	ctx, span := tracing.StartSpan(ctx, "mergedentity.Repository.Create")
	defer span.End()

	if entity.ID == "" {
		entity.ID = uuid.New().String()
	}
	entity.CreatedAt = time.Now().UTC()
	entity.UpdatedAt = entity.CreatedAt
	entity.Version = 1

	sb := sqlbuilder.PostgreSQL.NewInsertBuilder()
	sb.InsertInto("merged_entities")
	sb.Cols("id", "tenant_id", "entity_type", "data", "source_count", "primary_source_id", "created_at", "updated_at", "version")
	sb.Values(entity.ID, entity.TenantID, entity.EntityType, entity.Data, entity.SourceCount, entity.PrimarySourceID, entity.CreatedAt, entity.UpdatedAt, entity.Version)

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to create merged entity")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to create merged entity")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{"id": entity.ID}).Info("Created merged entity")
	return entity, nil
}

// Get retrieves a merged entity by ID
func (r *Repository) Get(ctx context.Context, tenantID string, id string) (*models.MergedEntity, error) {
	ctx, span := tracing.StartSpan(ctx, "mergedentity.Repository.Get")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "data", "source_count", "primary_source_id", "created_at", "updated_at", "deleted_at", "version")
	sb.From("merged_entities")
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()
	var entity models.MergedEntity
	if err := r.db.GetContext(ctx, &entity, query, args...); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf("merged entity %s not found", id))
		}
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get merged entity")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get merged entity")
	}

	return &entity, nil
}

// Update updates a merged entity's data and increments version
func (r *Repository) Update(ctx context.Context, id string, tenantID string, data json.RawMessage, sourceCount int) (*models.MergedEntity, error) {
	ctx, span := tracing.StartSpan(ctx, "mergedentity.Repository.Update")
	defer span.End()

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("merged_entities")
	sb.Set(
		sb.Assign("data", data),
		sb.Assign("source_count", sourceCount),
		sb.Assign("updated_at", now),
		sb.Add("version", 1),
	)
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to update merged entity")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to update merged entity")
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf("merged entity %s not found", id))
	}

	// Fetch and return updated entity
	return r.Get(ctx, tenantID, id)
}

// SoftDelete marks a merged entity as deleted
func (r *Repository) SoftDelete(ctx context.Context, tenantID string, id string) error {
	ctx, span := tracing.StartSpan(ctx, "mergedentity.Repository.SoftDelete")
	defer span.End()

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("merged_entities")
	sb.Set(sb.Assign("deleted_at", now))
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to soft delete merged entity")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete merged entity")
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf("merged entity %s not found", id))
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{"id": id}).Info("Soft deleted merged entity")
	return nil
}

// AddToCluster adds a staged entity to a merged entity's cluster
func (r *Repository) AddToCluster(ctx context.Context, tenantID string, mergedEntityID, stagedEntityID string, isPrimary bool) error {
	ctx, span := tracing.StartSpan(ctx, "mergedentity.Repository.AddToCluster")
	defer span.End()

	id := uuid.New()
	now := time.Now().UTC()

	sb := sqlbuilder.PostgreSQL.NewInsertBuilder()
	sb.InsertInto("entity_clusters")
	sb.Cols("id", "tenant_id", "merged_entity_id", "staged_entity_id", "is_primary", "added_at")
	sb.Values(id, tenantID, mergedEntityID, stagedEntityID, isPrimary, now)

	query, args := sb.Build()
	query += " ON CONFLICT (tenant_id, merged_entity_id, staged_entity_id) DO UPDATE SET removed_at = NULL, is_primary = EXCLUDED.is_primary"

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to add to cluster")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to add to cluster")
	}

	return nil
}

// RemoveFromCluster removes a staged entity from a merged entity's cluster
func (r *Repository) RemoveFromCluster(ctx context.Context, tenantID string, mergedEntityID, stagedEntityID string) error {
	ctx, span := tracing.StartSpan(ctx, "mergedentity.Repository.RemoveFromCluster")
	defer span.End()

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("entity_clusters")
	sb.Set(sb.Assign("removed_at", now))
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("merged_entity_id", mergedEntityID),
		sb.Equal("staged_entity_id", stagedEntityID),
		sb.IsNull("removed_at"),
	)

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to remove from cluster")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to remove from cluster")
	}

	return nil
}

// GetClusterMembers gets all staged entities in a merged entity's cluster
func (r *Repository) GetClusterMembers(ctx context.Context, tenantID string, mergedEntityID string) ([]models.EntityCluster, error) {
	ctx, span := tracing.StartSpan(ctx, "mergedentity.Repository.GetClusterMembers")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "merged_entity_id", "staged_entity_id", "is_primary", "added_at", "removed_at")
	sb.From("entity_clusters")
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("merged_entity_id", mergedEntityID),
		sb.IsNull("removed_at"),
	)
	sb.OrderBy("is_primary DESC", "added_at ASC")

	query, args := sb.Build()
	var members []models.EntityCluster
	if err := r.db.SelectContext(ctx, &members, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get cluster members")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get cluster members")
	}

	return members, nil
}

// GetMergedEntityByStagedID finds the merged entity that contains a staged entity.
// If multiple staged entity IDs are provided, returns one of the merged entities (use GetMergedEntitiesByStagedIDs for all).
func (r *Repository) GetMergedEntityByStagedID(ctx context.Context, tenantID string, stagedEntityIDs ...string) (*models.MergedEntity, error) {
	ctx, span := tracing.StartSpan(ctx, "mergedentity.Repository.GetMergedEntityByStagedID")
	defer span.End()

	entities, err := r.GetMergedEntitiesByStagedIDs(ctx, tenantID, stagedEntityIDs)
	if err != nil {
		return nil, err
	}
	if len(entities) == 0 {
		return nil, nil
	}
	return &entities[0], nil
}

// GetMergedEntitiesByStagedIDs finds ALL distinct merged entities that contain any of the given staged entity IDs.
// This is used to detect when entities that need to be merged are currently in different clusters,
// requiring cluster consolidation.
func (r *Repository) GetMergedEntitiesByStagedIDs(ctx context.Context, tenantID string, stagedEntityIDs []string) ([]models.MergedEntity, error) {
	ctx, span := tracing.StartSpan(ctx, "mergedentity.Repository.GetMergedEntitiesByStagedIDs")
	defer span.End()

	if len(stagedEntityIDs) == 0 {
		return nil, nil
	}

	// Use DISTINCT to get unique merged entities even if multiple staged IDs map to the same one
	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select(
		"DISTINCT merged_entities.id",
		"merged_entities.tenant_id",
		"merged_entities.entity_type",
		"merged_entities.data",
		"merged_entities.source_count",
		"merged_entities.primary_source_id",
		"merged_entities.created_at",
		"merged_entities.updated_at",
		"merged_entities.deleted_at",
		"merged_entities.version",
	)
	sb.From("merged_entities")
	sb.Join("entity_clusters", "merged_entities.id = entity_clusters.merged_entity_id")
	sb.Where(
		sb.Equal("entity_clusters.tenant_id", tenantID),
		sb.In("entity_clusters.staged_entity_id", sqlbuilder.Flatten(stagedEntityIDs)...),
		sb.IsNull("entity_clusters.removed_at"),
		sb.IsNull("merged_entities.deleted_at"),
	)
	sb.OrderBy("merged_entities.created_at ASC") // Oldest first (will be the "surviving" cluster)

	query, args := sb.Build()
	var entities []models.MergedEntity
	if err := r.db.SelectContext(ctx, &entities, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{"tenant_id": tenantID, "staged_entity_ids": stagedEntityIDs}).Error("Failed to get merged entities by staged IDs")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get merged entities")
	}

	return entities, nil
}

// MoveClusterMembers moves all members from one merged entity's cluster to another.
// This is used during cluster consolidation when merging entities that were previously in different clusters.
func (r *Repository) MoveClusterMembers(ctx context.Context, tenantID string, fromMergedID, toMergedID string) error {
	ctx, span := tracing.StartSpan(ctx, "mergedentity.Repository.MoveClusterMembers")
	defer span.End()

	// Update all cluster memberships to point to the new merged entity
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("entity_clusters")
	sb.Set(sb.Assign("merged_entity_id", toMergedID))
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("merged_entity_id", fromMergedID),
		sb.IsNull("removed_at"),
	)

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to move cluster members")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to move cluster members")
	}

	return nil
}

// StagedToMergedMapping represents a mapping from staged entity ID to merged entity ID
type StagedToMergedMapping struct {
	StagedEntityID string `db:"staged_entity_id"`
	MergedEntityID string `db:"merged_entity_id"`
}

// GetMergedEntityIDsForRuleCriteria efficiently retrieves merged entity IDs for all staged entities
// matching the given rule criteria (entity_type, source_id, and optionally a data field).
// This is optimized to fetch all matching entities and their merged IDs in a single JOIN query.
//
// sourceField specifies which field to match:
// - "source_id" (default): matches staged_entities.source_id = sourceID
// - any other field: matches staged_entities.data->>sourceField = sourceID
func (r *Repository) GetMergedEntityIDsForRuleCriteria(ctx context.Context, tenantID, entityType, sourceID, sourceField string) ([]StagedToMergedMapping, error) {
	ctx, span := tracing.StartSpan(ctx, "mergedentity.Repository.GetMergedEntityIDsForRuleCriteria")
	defer span.End()

	if sourceField == "" {
		sourceField = "source_id"
	}

	var query string
	var args []interface{}

	if sourceField == "source_id" {
		// Use indexed source_id lookup
		query = `
			SELECT
				ec.staged_entity_id,
				ec.merged_entity_id
			FROM entity_clusters ec
			JOIN staged_entities se ON se.id = ec.staged_entity_id
			WHERE ec.tenant_id = $1
			  AND se.entity_type = $2
			  AND se.source_id = $3
			  AND ec.removed_at IS NULL
			  AND se.deleted_at IS NULL
		`
		args = []interface{}{tenantID, entityType, sourceID}
	} else {
		// Use JSONB field lookup
		query = `
			SELECT
				ec.staged_entity_id,
				ec.merged_entity_id
			FROM entity_clusters ec
			JOIN staged_entities se ON se.id = ec.staged_entity_id
			WHERE ec.tenant_id = $1
			  AND se.entity_type = $2
			  AND (se.data ->> $3) = $4
			  AND ec.removed_at IS NULL
			  AND se.deleted_at IS NULL
		`
		args = []interface{}{tenantID, entityType, sourceField, sourceID}
	}

	var mappings []StagedToMergedMapping
	if err := r.db.SelectContext(ctx, &mappings, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"tenant_id":    tenantID,
			"entity_type":  entityType,
			"source_id":    sourceID,
			"source_field": sourceField,
		}).Error("Failed to get merged entity IDs for rule criteria")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get merged entity IDs")
	}

	return mappings, nil
}
