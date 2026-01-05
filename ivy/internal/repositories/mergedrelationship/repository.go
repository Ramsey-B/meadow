package mergedrelationship

import (
	"context"
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

var mergedRelIDNamespace = uuid.MustParse("a8b9c0d1-2e3f-4a5b-6c7d-8e9f0a1b2c3d")

// ComputeDeterministicID returns a deterministic ID for merged relationships based on merged entity IDs.
func ComputeDeterministicID(tenantID, relationshipType string, fromMergedEntityID, toMergedEntityID string) string {
	return uuid.NewSHA1(mergedRelIDNamespace, []byte(fmt.Sprintf("%s|%s|%s|%s",
		tenantID, relationshipType, fromMergedEntityID, toMergedEntityID))).String()
}

// Repository manages merged_relationships persistence.
type Repository struct {
	db     database.DB
	logger ectologger.Logger
}

func NewRepository(db database.DB, logger ectologger.Logger) *Repository {
	return &Repository{db: db, logger: logger}
}

// Upsert creates or updates a merged relationship (golden edge).
func (r *Repository) Upsert(ctx context.Context, tenantID string, req models.CreateMergedRelationshipRequest) (*models.MergedRelationship, error) {
	ctx, span := tracing.StartSpan(ctx, "mergedrelationship.Repository.Upsert")
	defer span.End()

	now := time.Now().UTC()
	id := ComputeDeterministicID(tenantID, req.RelationshipType, req.FromMergedEntityID, req.ToMergedEntityID)

	sb := sqlbuilder.PostgreSQL.NewInsertBuilder()
	sb.InsertInto("merged_relationships")
	sb.Cols(
		"id",
		"tenant_id",
		"relationship_type",
		"from_entity_type",
		"from_merged_entity_id",
		"to_entity_type",
		"to_merged_entity_id",
		"data",
		"created_at",
		"updated_at",
	)
	sb.Values(
		id,
		tenantID,
		req.RelationshipType,
		req.FromEntityType,
		req.FromMergedEntityID,
		req.ToEntityType,
		req.ToMergedEntityID,
		req.Data,
		now,
		now,
	)

	query, args := sb.Build()
	query += ` ON CONFLICT (tenant_id, relationship_type, from_merged_entity_id, to_merged_entity_id)
	DO UPDATE SET
		from_entity_type = EXCLUDED.from_entity_type,
		to_entity_type = EXCLUDED.to_entity_type,
		data = COALESCE(
			jsonb_strip_nulls(
				merged_relationships.data || EXCLUDED.data
			), EXCLUDED.data
		),
		updated_at = EXCLUDED.updated_at,
		deleted_at = NULL
	RETURNING id, tenant_id, relationship_type, from_entity_type, from_merged_entity_id, to_entity_type, to_merged_entity_id, data, created_at, updated_at, deleted_at`

	var out models.MergedRelationship
	if err := r.db.GetContext(ctx, &out, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to upsert merged relationship")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to upsert merged relationship")
	}
	return &out, nil
}

func (r *Repository) Get(ctx context.Context, tenantID string, id string) (*models.MergedRelationship, error) {
	ctx, span := tracing.StartSpan(ctx, "mergedrelationship.Repository.Get")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "relationship_type", "from_entity_type", "from_merged_entity_id", "to_entity_type", "to_merged_entity_id", "data", "created_at", "updated_at", "deleted_at")
	sb.From("merged_relationships")
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("id", id),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()
	var out models.MergedRelationship
	if err := r.db.GetContext(ctx, &out, query, args...); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get merged relationship")
	}
	return &out, nil
}

func (r *Repository) SoftDelete(ctx context.Context, tenantID string, mergedRelID string) error {
	ctx, span := tracing.StartSpan(ctx, "mergedrelationship.Repository.SoftDelete")
	defer span.End()

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("merged_relationships")
	sb.Set(sb.Assign("deleted_at", now))
	sb.Where(
		sb.Equal("id", mergedRelID),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to soft delete merged relationship")
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf("merged relationship %s not found", mergedRelID))
	}

	return nil
}

func (r *Repository) Validate() error {
	if r.db == nil {
		return fmt.Errorf("db is nil")
	}
	return nil
}

// AddToCluster adds a staged relationship to a merged relationship's cluster
func (r *Repository) AddToCluster(ctx context.Context, tenantID string, mergedRelID, stagedRelID string, isPrimary bool) error {
	ctx, span := tracing.StartSpan(ctx, "mergedrelationship.Repository.AddToCluster")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewInsertBuilder()
	sb.InsertInto("relationship_clusters")
	sb.Cols("tenant_id", "merged_relationship_id", "staged_relationship_id", "is_primary", "added_at")
	sb.Values(tenantID, mergedRelID, stagedRelID, isPrimary, time.Now().UTC())

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to add to relationship cluster")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to add to relationship cluster")
	}

	return nil
}

// RemoveFromCluster removes a staged relationship from a merged relationship's cluster
func (r *Repository) RemoveFromCluster(ctx context.Context, tenantID string, mergedRelID, stagedRelID string) error {
	ctx, span := tracing.StartSpan(ctx, "mergedrelationship.Repository.RemoveFromCluster")
	defer span.End()

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("relationship_clusters")
	sb.Set(sb.Assign("removed_at", now))
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("merged_relationship_id", mergedRelID),
		sb.Equal("staged_relationship_id", stagedRelID),
		sb.IsNull("removed_at"),
	)

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to remove from relationship cluster")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to remove from relationship cluster")
	}

	return nil
}

// GetClusterMembers gets all staged relationships in a merged relationship's cluster
func (r *Repository) GetClusterMembers(ctx context.Context, tenantID string, mergedRelID string) ([]models.RelationshipCluster, error) {
	ctx, span := tracing.StartSpan(ctx, "mergedrelationship.Repository.GetClusterMembers")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "merged_relationship_id", "staged_relationship_id", "is_primary", "added_at", "removed_at")
	sb.From("relationship_clusters")
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("merged_relationship_id", mergedRelID),
		sb.IsNull("removed_at"),
	)
	sb.OrderBy("is_primary DESC", "added_at ASC")

	query, args := sb.Build()
	var members []models.RelationshipCluster
	if err := r.db.SelectContext(ctx, &members, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get relationship cluster members")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get relationship cluster members")
	}

	return members, nil
}

// GetMergedRelationshipsByStagedIDs gets merged relationships by their staged relationship IDs
func (r *Repository) GetMergedRelationshipsByStagedIDs(ctx context.Context, tenantID string, stagedRelIDs []string) ([]models.MergedRelationship, error) {
	ctx, span := tracing.StartSpan(ctx, "mergedrelationship.Repository.GetMergedRelationshipsByStagedIDs")
	defer span.End()

	if len(stagedRelIDs) == 0 {
		return []models.MergedRelationship{}, nil
	}

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("DISTINCT merged_relationships.id", "merged_relationships.tenant_id", "merged_relationships.relationship_type",
		"merged_relationships.from_entity_type", "merged_relationships.from_merged_entity_id",
		"merged_relationships.to_entity_type", "merged_relationships.to_merged_entity_id",
		"merged_relationships.data", "merged_relationships.created_at", "merged_relationships.updated_at", "merged_relationships.deleted_at")
	sb.From("merged_relationships")
	sb.Join("relationship_clusters", "merged_relationships.id = relationship_clusters.merged_relationship_id")
	sb.Where(
		sb.Equal("relationship_clusters.tenant_id", tenantID),
		sb.In("relationship_clusters.staged_relationship_id", sqlbuilder.Flatten(stagedRelIDs)...),
		sb.IsNull("relationship_clusters.removed_at"),
		sb.IsNull("merged_relationships.deleted_at"),
	)

	query, args := sb.Build()
	var rels []models.MergedRelationship
	if err := r.db.SelectContext(ctx, &rels, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get merged relationships by staged IDs")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get merged relationships")
	}

	return rels, nil
}
