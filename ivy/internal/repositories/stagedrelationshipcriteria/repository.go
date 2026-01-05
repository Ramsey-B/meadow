// Package stagedrelationshipcriteria provides database operations for criteria-based relationships.
package stagedrelationshipcriteria

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"
	"github.com/huandu/go-sqlbuilder"

	"github.com/Ramsey-B/ivy/pkg/criteria"
	"github.com/Ramsey-B/ivy/pkg/models"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

// Repository handles database operations for criteria-based relationships
type Repository struct {
	db     database.DB
	logger ectologger.Logger
}

// New creates a new criteria relationship repository
func New(db database.DB, logger ectologger.Logger) *Repository {
	return &Repository{db: db, logger: logger}
}

// UpsertResult contains the result of an upsert operation
type UpsertResult struct {
	Criteria  *models.StagedRelationshipCriteria
	IsNew     bool
	IsChanged bool
}

// Upsert creates or updates a criteria-based relationship definition
func (r *Repository) Upsert(ctx context.Context, tenantID string, req models.CreateStagedRelationshipCriteriaRequest) (*UpsertResult, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationshipcriteria.Repository.Upsert")
	defer span.End()

	log := r.logger.WithContext(ctx).WithFields(map[string]any{
		"method":            "Upsert",
		"tenant_id":         tenantID,
		"relationship_type": req.RelationshipType,
		"from_entity_type":  req.FromEntityType,
		"from_source_id":    req.FromSourceID,
		"to_entity_type":    req.ToEntityType,
		"to_integration":    req.ToIntegration,
	})

	now := time.Now().UTC()
	id := uuid.New().String()

	// Generate criteria hash for deduplication
	criteriaHash := criteria.HashCriteria(req.Criteria)

	// Marshal criteria to JSON
	criteriaJSON, err := json.Marshal(req.Criteria)
	if err != nil {
		log.WithError(err).Error("Failed to marshal criteria")
		return nil, httperror.NewHTTPError(http.StatusBadRequest, "invalid criteria")
	}

	execID := req.SourceExecutionID

	query := `
		WITH upsert AS (
			INSERT INTO staged_relationship_criteria (
				id, tenant_id, config_id, integration, source_key,
				relationship_type,
				from_entity_type, from_source_id, from_integration,
				to_entity_type, to_integration, criteria, criteria_hash,
				source_execution_id, execution_id, last_seen_execution,
				data, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
			ON CONFLICT (tenant_id, relationship_type, 
			             from_entity_type, from_source_id, from_integration,
			             to_entity_type, to_integration, criteria_hash, config_id)
			DO UPDATE SET
				source_execution_id = EXCLUDED.source_execution_id,
				execution_id = EXCLUDED.execution_id,
				last_seen_execution = EXCLUDED.last_seen_execution,
				data = COALESCE(EXCLUDED.data, staged_relationship_criteria.data),
				updated_at = EXCLUDED.updated_at,
				deleted_at = NULL
			RETURNING *, (xmax = 0) AS inserted
		)
		SELECT * FROM upsert
	`

	var result struct {
		models.StagedRelationshipCriteria
		Inserted bool `db:"inserted"`
	}

	err = r.db.GetContext(ctx, &result, query,
		id, tenantID, req.ConfigID, req.Integration, req.SourceKey,
		req.RelationshipType,
		req.FromEntityType, req.FromSourceID, req.FromIntegration,
		req.ToEntityType, req.ToIntegration, criteriaJSON, criteriaHash,
		execID, execID, execID,
		req.Data, now, now,
	)
	if err != nil {
		log.WithError(err).Error("Failed to upsert criteria relationship")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to upsert criteria relationship")
	}

	if result.Inserted {
		log.WithFields(map[string]any{"id": result.ID}).Info("Created criteria relationship")
	} else {
		log.WithFields(map[string]any{"id": result.ID}).Debug("Updated criteria relationship")
	}

	return &UpsertResult{
		Criteria:  &result.StagedRelationshipCriteria,
		IsNew:     result.Inserted,
		IsChanged: true,
	}, nil
}

// Get retrieves a criteria relationship by ID
func (r *Repository) Get(ctx context.Context, tenantID, id string) (*models.StagedRelationshipCriteria, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationshipcriteria.Repository.Get")
	defer span.End()

	query := `
		SELECT * FROM staged_relationship_criteria
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
	`

	var result models.StagedRelationshipCriteria
	err := r.db.GetContext(ctx, &result, query, tenantID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, httperror.NewHTTPError(http.StatusNotFound, "criteria relationship not found")
	}
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get criteria relationship")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get criteria relationship")
	}

	return &result, nil
}

// FindByTarget finds all criteria definitions targeting a specific entity type and integration.
// This is used for backfill when a new entity arrives.
func (r *Repository) FindByTarget(ctx context.Context, tenantID, entityType, integration string) ([]models.StagedRelationshipCriteria, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationshipcriteria.Repository.FindByTarget")
	defer span.End()

	query := `
		SELECT * FROM staged_relationship_criteria
		WHERE tenant_id = $1 
		  AND to_entity_type = $2 
		  AND to_integration = $3
		  AND deleted_at IS NULL
		ORDER BY created_at
	`

	var results []models.StagedRelationshipCriteria
	err := r.db.SelectContext(ctx, &results, query, tenantID, entityType, integration)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to find criteria by target")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to find criteria relationships")
	}

	return results, nil
}

// UpdateFromEntityID updates the from_staged_entity_id when the from entity is resolved
func (r *Repository) UpdateFromEntityID(ctx context.Context, tenantID, criteriaID, entityID string) error {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationshipcriteria.Repository.UpdateFromEntityID")
	defer span.End()

	query := `
		UPDATE staged_relationship_criteria
		SET from_staged_entity_id = $1, updated_at = NOW()
		WHERE tenant_id = $2 AND id = $3
	`

	_, err := r.db.ExecContext(ctx, query, entityID, tenantID, criteriaID)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to update from entity ID")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to update criteria relationship")
	}

	return nil
}

// AddMatchResult contains the result of an AddMatch operation
type AddMatchResult struct {
	Match               *models.StagedRelationshipCriteriaMatch
	StagedRelationship  *models.StagedRelationship
	IsNew               bool
}

// AddMatch adds a match record between a criteria and a target entity.
// It also creates a staged_relationship that flows through the normal merge pipeline.
func (r *Repository) AddMatch(ctx context.Context, tenantID string, crit *models.StagedRelationshipCriteria, toEntity *models.StagedEntity, stagedRelID string) (*AddMatchResult, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationshipcriteria.Repository.AddMatch")
	defer span.End()

	now := time.Now().UTC()

	log := r.logger.WithContext(ctx).WithFields(map[string]any{
		"criteria_id":   crit.ID,
		"to_entity_id":  toEntity.ID,
		"staged_rel_id": stagedRelID,
	})

	// Insert the match record with reference to staged_relationship
	query := `
		INSERT INTO staged_relationship_criteria_matches (
			criteria_id, to_staged_entity_id, staged_relationship_id,
			tenant_id, relationship_type, created_at, last_verified_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (criteria_id, to_staged_entity_id)
		DO UPDATE SET 
			last_verified_at = EXCLUDED.last_verified_at,
			staged_relationship_id = EXCLUDED.staged_relationship_id
		RETURNING (xmax = 0) AS inserted
	`

	var inserted bool
	err := r.db.GetContext(ctx, &inserted, query, 
		crit.ID, toEntity.ID, stagedRelID,
		tenantID, crit.RelationshipType, now, now,
	)
	if err != nil {
		log.WithError(err).Error("Failed to add criteria match")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to add criteria match")
	}

	match := &models.StagedRelationshipCriteriaMatch{
		CriteriaID:           crit.ID,
		ToStagedEntityID:     toEntity.ID,
		StagedRelationshipID: stagedRelID,
		TenantID:             tenantID,
		RelationshipType:     crit.RelationshipType,
		CreatedAt:            now,
		LastVerifiedAt:       now,
	}

	return &AddMatchResult{
		Match: match,
		IsNew: inserted,
	}, nil
}

// RemoveMatch removes a match record and soft-deletes the associated staged_relationship
func (r *Repository) RemoveMatch(ctx context.Context, tenantID, criteriaID, toEntityID string) error {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationshipcriteria.Repository.RemoveMatch")
	defer span.End()

	log := r.logger.WithContext(ctx).WithFields(map[string]any{
		"criteria_id":  criteriaID,
		"to_entity_id": toEntityID,
	})

	// First get the staged_relationship_id so we can soft-delete it
	var stagedRelID string
	err := r.db.GetContext(ctx, &stagedRelID, 
		`SELECT staged_relationship_id FROM staged_relationship_criteria_matches 
		 WHERE criteria_id = $1 AND to_staged_entity_id = $2`,
		criteriaID, toEntityID,
	)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			log.Debug("Match not found, nothing to remove")
			return nil
		}
		log.WithError(err).Error("Failed to get staged_relationship_id")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to remove criteria match")
	}

	// Delete the match record (cascades from staged_relationships FK)
	_, err = r.db.ExecContext(ctx, 
		`DELETE FROM staged_relationship_criteria_matches WHERE criteria_id = $1 AND to_staged_entity_id = $2`,
		criteriaID, toEntityID,
	)
	if err != nil {
		log.WithError(err).Error("Failed to remove criteria match")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to remove criteria match")
	}

	// Soft-delete the staged_relationship (triggers CDC -> removes from cluster)
	_, err = r.db.ExecContext(ctx,
		`UPDATE staged_relationships SET deleted_at = NOW() WHERE id = $1 AND tenant_id = $2`,
		stagedRelID, tenantID,
	)
	if err != nil {
		log.WithError(err).Warn("Failed to soft-delete staged_relationship")
		// Don't fail - match was removed
	}

	log.WithFields(map[string]any{"staged_rel_id": stagedRelID}).Debug("Removed criteria match and staged_relationship")
	return nil
}

// GetMatches retrieves all matches for a criteria
func (r *Repository) GetMatches(ctx context.Context, criteriaID string) ([]models.StagedRelationshipCriteriaMatch, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationshipcriteria.Repository.GetMatches")
	defer span.End()

	query := `
		SELECT * FROM staged_relationship_criteria_matches
		WHERE criteria_id = $1
		ORDER BY created_at
	`

	var results []models.StagedRelationshipCriteriaMatch
	err := r.db.SelectContext(ctx, &results, query, criteriaID)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get criteria matches")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get criteria matches")
	}

	return results, nil
}

// GetMatchesByEntity retrieves all criteria matches for a specific entity
func (r *Repository) GetMatchesByEntity(ctx context.Context, entityID string) ([]models.StagedRelationshipCriteriaMatch, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationshipcriteria.Repository.GetMatchesByEntity")
	defer span.End()

	query := `
		SELECT * FROM staged_relationship_criteria_matches
		WHERE to_staged_entity_id = $1
		ORDER BY created_at
	`

	var results []models.StagedRelationshipCriteriaMatch
	err := r.db.SelectContext(ctx, &results, query, entityID)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get matches by entity")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get matches by entity")
	}

	return results, nil
}

// SoftDelete marks a criteria relationship as deleted
func (r *Repository) SoftDelete(ctx context.Context, tenantID, id string) error {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationshipcriteria.Repository.SoftDelete")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("staged_relationship_criteria")
	sb.Set(sb.Assign("deleted_at", time.Now().UTC()))
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("id", id))

	query, args := sb.Build()
	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to soft delete criteria relationship")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete criteria relationship")
	}

	return nil
}

// MarkSeenInExecution updates the last_seen_execution field for criteria seen in an execution
func (r *Repository) MarkSeenInExecution(ctx context.Context, tenantID, executionID string, criteriaIDs []string) error {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationshipcriteria.Repository.MarkSeenInExecution")
	defer span.End()

	if len(criteriaIDs) == 0 {
		return nil
	}

	query := `
		UPDATE staged_relationship_criteria
		SET last_seen_execution = $1, updated_at = NOW()
		WHERE tenant_id = $2 AND id = ANY($3)
	`

	_, err := r.db.ExecContext(ctx, query, executionID, tenantID, criteriaIDs)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to mark criteria as seen")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to mark criteria as seen")
	}

	return nil
}

// FindNotSeenInExecution finds criteria that were not seen in the given execution
// Used for execution-based deletion strategy
func (r *Repository) FindNotSeenInExecution(ctx context.Context, tenantID, sourceKey, executionID string) ([]models.StagedRelationshipCriteria, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedrelationshipcriteria.Repository.FindNotSeenInExecution")
	defer span.End()

	query := `
		SELECT * FROM staged_relationship_criteria
		WHERE tenant_id = $1 
		  AND source_key = $2
		  AND deleted_at IS NULL
		  AND (last_seen_execution IS NULL OR last_seen_execution != $3)
		ORDER BY created_at
	`

	var results []models.StagedRelationshipCriteria
	err := r.db.SelectContext(ctx, &results, query, tenantID, sourceKey, executionID)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to find criteria not seen in execution")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to find criteria relationships")
	}

	return results, nil
}

