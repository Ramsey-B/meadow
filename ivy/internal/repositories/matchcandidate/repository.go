package matchcandidate

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

// Repository handles match candidate persistence
type Repository struct {
	db     database.DB
	logger ectologger.Logger
}

// NewRepository creates a new match candidate repository
func NewRepository(db database.DB, logger ectologger.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new match candidate
func (r *Repository) Create(ctx context.Context, candidate *models.MatchCandidate) (*models.MatchCandidate, error) {
	ctx, span := tracing.StartSpan(ctx, "matchcandidate.Repository.Create")
	defer span.End()

	if candidate.ID == "" {
		candidate.ID = uuid.New().String()
	}
	candidate.CreatedAt = time.Now().UTC()
	candidate.UpdatedAt = candidate.CreatedAt
	candidate.Status = models.MatchCandidateStatusPending

	sb := sqlbuilder.PostgreSQL.NewInsertBuilder()
	sb.InsertInto("match_candidates")
	sb.Cols("id", "tenant_id", "entity_type", "source_entity_id", "candidate_entity_id", "match_score", "match_details", "status", "matched_rules", "created_at", "updated_at")
	sb.Values(candidate.ID, candidate.TenantID, candidate.EntityType, candidate.SourceEntityID, candidate.CandidateEntityID, candidate.MatchScore, candidate.MatchDetails, candidate.Status, candidate.MatchedRules, candidate.CreatedAt, candidate.UpdatedAt)

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{"candidate_id": candidate.ID}).Error("Failed to create match candidate")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to create match candidate")
	}

	return candidate, nil
}

// CreateBatch creates multiple match candidates efficiently
func (r *Repository) CreateBatch(ctx context.Context, candidates []*models.MatchCandidate) error {
	ctx, span := tracing.StartSpan(ctx, "matchcandidate.Repository.CreateBatch")
	defer span.End()

	if len(candidates) == 0 {
		return nil
	}

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewInsertBuilder()
	sb.InsertInto("match_candidates")
	sb.Cols("id", "tenant_id", "entity_type", "source_entity_id", "candidate_entity_id", "match_score", "match_details", "status", "matched_rules", "created_at", "updated_at")

	for _, c := range candidates {
		if c.ID == "" {
			c.ID = uuid.New().String()
		}
		c.CreatedAt = now
		c.UpdatedAt = now
		if c.Status == "" {
			c.Status = models.MatchCandidateStatusPending
		}
		sb.Values(c.ID, c.TenantID, c.EntityType, c.SourceEntityID, c.CandidateEntityID, c.MatchScore, c.MatchDetails, c.Status, c.MatchedRules, c.CreatedAt, c.UpdatedAt)
	}

	query, args := sb.Build()
	// Add ON CONFLICT to skip duplicates
	query += " ON CONFLICT (tenant_id, source_entity_id, candidate_entity_id) DO UPDATE SET match_score = GREATEST(match_candidates.match_score, EXCLUDED.match_score), updated_at = EXCLUDED.updated_at"

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to create match candidates batch")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to create match candidates")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{"count": len(candidates)}).Debug("Created match candidates batch")
	return nil
}

// Get retrieves a match candidate by ID
func (r *Repository) Get(ctx context.Context, tenantID string, id string) (*models.MatchCandidate, error) {
	ctx, span := tracing.StartSpan(ctx, "matchcandidate.Repository.Get")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "source_entity_id", "candidate_entity_id", "match_score", "match_details", "status", "matched_rules", "created_at", "updated_at", "resolved_at", "resolved_by")
	sb.From("match_candidates")
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
	)

	query, args := sb.Build()
	var candidate models.MatchCandidate
	if err := r.db.GetContext(ctx, &candidate, query, args...); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf("match candidate %s not found", id))
		}
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get match candidate")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get match candidate")
	}

	return &candidate, nil
}

// ListPending retrieves pending match candidates for review (simple version with limit)
func (r *Repository) ListPending(ctx context.Context, tenantID string, limit int) ([]models.MatchCandidate, error) {
	ctx, span := tracing.StartSpan(ctx, "matchcandidate.Repository.ListPending")
	defer span.End()

	if limit < 1 || limit > 500 {
		limit = 100
	}

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "source_entity_id", "candidate_entity_id", "match_score", "match_details", "status", "matched_rules", "created_at", "updated_at", "resolved_at", "resolved_by")
	sb.From("match_candidates")
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("status", models.MatchCandidateStatusPending),
	)
	sb.OrderBy("match_score DESC", "created_at DESC")
	sb.Limit(limit)

	query, args := sb.Build()
	var candidates []models.MatchCandidate
	if err := r.db.SelectContext(ctx, &candidates, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to list pending match candidates")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list pending match candidates")
	}

	return candidates, nil
}

// ListByEntity retrieves match candidates involving a specific entity
func (r *Repository) ListByEntity(ctx context.Context, tenantID string, entityID string, status string) ([]models.MatchCandidate, error) {
	ctx, span := tracing.StartSpan(ctx, "matchcandidate.Repository.ListByEntity")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "source_entity_id", "candidate_entity_id", "match_score", "match_details", "status", "matched_rules", "created_at", "updated_at", "resolved_at", "resolved_by")
	sb.From("match_candidates")

	where := []string{
		sb.Equal("tenant_id", tenantID),
		fmt.Sprintf("(source_entity_id = '%s' OR candidate_entity_id = '%s')", entityID, entityID),
	}
	if status != "" {
		where = append(where, sb.Equal("status", status))
	}
	sb.Where(where...)
	sb.OrderBy("match_score DESC", "created_at DESC")

	query, args := sb.Build()
	var candidates []models.MatchCandidate
	if err := r.db.SelectContext(ctx, &candidates, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to list match candidates by entity")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list match candidates")
	}

	return candidates, nil
}

// GetByEntityPair gets an existing candidate between two entities (regardless of order)
func (r *Repository) GetByEntityPair(ctx context.Context, tenantID string, entityA, entityB string) (*models.MatchCandidate, error) {
	ctx, span := tracing.StartSpan(ctx, "matchcandidate.Repository.GetByEntityPair")
	defer span.End()

	query := `
		SELECT id, tenant_id, entity_type, source_entity_id, candidate_entity_id, match_score, match_details, status, matched_rules, created_at, updated_at, resolved_at, resolved_by
		FROM match_candidates
		WHERE tenant_id = $1
		AND ((source_entity_id = $2 AND candidate_entity_id = $3) OR (source_entity_id = $3 AND candidate_entity_id = $2))
		LIMIT 1
	`

	var candidate models.MatchCandidate
	if err := r.db.GetContext(ctx, &candidate, query, tenantID, entityA, entityB); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil // No existing candidate
		}
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get match candidate by entity pair")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get match candidate")
	}

	return &candidate, nil
}

// UpdateStatus updates the status of a match candidate by entity pair
func (r *Repository) UpdateStatus(ctx context.Context, tenantID string, entityA, entityB string, status string) error {
	ctx, span := tracing.StartSpan(ctx, "matchcandidate.Repository.UpdateStatus")
	defer span.End()

	now := time.Now().UTC()
	query := `
		UPDATE match_candidates
		SET status = $1, resolved_at = $2, updated_at = $2
		WHERE tenant_id = $3
		AND ((source_entity_id = $4 AND candidate_entity_id = $5) OR (source_entity_id = $5 AND candidate_entity_id = $4))
	`

	result, err := r.db.ExecContext(ctx, query, status, now, tenantID, entityA, entityB)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to update match candidate status")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to update match candidate status")
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return httperror.NewHTTPError(http.StatusNotFound, "match candidate not found")
	}

	return nil
}

// UpdateStatusByID updates the status of a match candidate by ID
func (r *Repository) UpdateStatusByID(ctx context.Context, tenantID string, id string, status string, resolvedBy *string) error {
	ctx, span := tracing.StartSpan(ctx, "matchcandidate.Repository.UpdateStatusByID")
	defer span.End()

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("match_candidates")
	sb.Set(
		sb.Assign("status", status),
		sb.Assign("resolved_at", now),
		sb.Assign("resolved_by", resolvedBy),
		sb.Assign("updated_at", now),
	)
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
	)

	query, args := sb.Build()
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to update match candidate status")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to update match candidate status")
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf("match candidate %s not found", id))
	}

	return nil
}

// MarkAutoMerged marks candidates as auto-merged (used when score exceeds threshold)
func (r *Repository) MarkAutoMerged(ctx context.Context, tenantID string, ids []string) error {
	ctx, span := tracing.StartSpan(ctx, "matchcandidate.Repository.MarkAutoMerged")
	defer span.End()

	if len(ids) == 0 {
		return nil
	}

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("match_candidates")
	sb.Set(
		sb.Assign("status", models.MatchCandidateStatusAutoMerged),
		sb.Assign("resolved_at", now),
		sb.Assign("updated_at", now),
	)
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.In("id", uuidsToAny(ids)...),
	)

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to mark candidates as auto-merged")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to mark candidates as auto-merged")
	}

	return nil
}

// DeleteByEntityID removes all candidates involving an entity (used when entity is deleted)
func (r *Repository) DeleteByEntityID(ctx context.Context, tenantID string, entityID string) error {
	ctx, span := tracing.StartSpan(ctx, "matchcandidate.Repository.DeleteByEntityID")
	defer span.End()

	query := `
		DELETE FROM match_candidates
		WHERE tenant_id = $1
		AND (source_entity_id = $2 OR candidate_entity_id = $2)
	`

	if _, err := r.db.ExecContext(ctx, query, tenantID, entityID); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to delete match candidates by entity_id")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete match candidates")
	}

	return nil
}

func uuidsToAny(ids []string) []any {
	result := make([]any, len(ids))
	for i, id := range ids {
		result[i] = id
	}
	return result
}
