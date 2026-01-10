package deletion

import (
	"context"
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

// PendingRepository handles pending deletion persistence
type PendingRepository struct {
	db     database.DB
	logger ectologger.Logger
}

// NewPendingRepository creates a new pending deletion repository
func NewPendingRepository(db database.DB, logger ectologger.Logger) *PendingRepository {
	return &PendingRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a pending deletion
func (r *PendingRepository) Create(ctx context.Context, pending *models.PendingDeletion) error {
	ctx, span := tracing.StartSpan(ctx, "deletion.PendingRepository.Create")
	defer span.End()

	if pending.ID == uuid.Nil {
		pending.ID = uuid.New()
	}
	pending.CreatedAt = time.Now().UTC()

	sb := sqlbuilder.PostgreSQL.NewInsertBuilder()
	sb.InsertInto("pending_deletions")
	sb.Cols("id", "tenant_id", "staged_entity_id", "merged_entity_id", "entity_type", "reason", "scheduled_for", "created_at")
	sb.Values(pending.ID, pending.TenantID, pending.StagedEntityID, pending.MergedEntityID, pending.EntityType, pending.Reason, pending.ScheduledFor, pending.CreatedAt)

	query, args := sb.Build()
	query += " ON CONFLICT (tenant_id, staged_entity_id) DO UPDATE SET scheduled_for = EXCLUDED.scheduled_for, reason = EXCLUDED.reason, cancelled = false, cancelled_reason = NULL"

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to create pending deletion")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to create pending deletion")
	}

	return nil
}

// GetDue gets pending deletions that are due for execution
func (r *PendingRepository) GetDue(ctx context.Context, tenantID string, limit int) ([]models.PendingDeletion, error) {
	ctx, span := tracing.StartSpan(ctx, "deletion.PendingRepository.GetDue")
	defer span.End()

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "staged_entity_id", "merged_entity_id", "entity_type", "reason", "scheduled_for", "executed_at", "cancelled", "cancelled_reason", "created_at")
	sb.From("pending_deletions")
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.LessEqualThan("scheduled_for", now),
		sb.IsNull("executed_at"),
		sb.Equal("cancelled", false),
	)
	sb.OrderBy("scheduled_for ASC")
	if limit > 0 {
		sb.Limit(limit)
	}

	query, args := sb.Build()
	var deletions []models.PendingDeletion
	if err := r.db.SelectContext(ctx, &deletions, query, args...); err != nil {
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get due deletions")
	}

	return deletions, nil
}

// MarkExecuted marks a pending deletion as executed
func (r *PendingRepository) MarkExecuted(ctx context.Context, id uuid.UUID) error {
	ctx, span := tracing.StartSpan(ctx, "deletion.PendingRepository.MarkExecuted")
	defer span.End()

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("pending_deletions")
	sb.Set(sb.Assign("executed_at", now))
	sb.Where(sb.Equal("id", id))

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to mark deletion executed")
	}

	return nil
}

// Cancel cancels a pending deletion
func (r *PendingRepository) Cancel(ctx context.Context, stagedEntityID uuid.UUID, reason string) error {
	ctx, span := tracing.StartSpan(ctx, "deletion.PendingRepository.Cancel")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("pending_deletions")
	sb.Set(
		sb.Assign("cancelled", true),
		sb.Assign("cancelled_reason", reason),
	)
	sb.Where(
		sb.Equal("staged_entity_id", stagedEntityID),
		sb.IsNull("executed_at"),
	)

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to cancel deletion")
	}

	return nil
}

// CancelByEntityID cancels pending deletion for an entity (when entity reappears)
func (r *PendingRepository) CancelByEntityID(ctx context.Context, tenantID string, stagedEntityID uuid.UUID) error {
	return r.Cancel(ctx, stagedEntityID, "entity reappeared in execution")
}

