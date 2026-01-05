package matchrule

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

// Repository handles match rule persistence
type Repository struct {
	db     database.DB
	logger ectologger.Logger
}

// NewRepository creates a new match rule repository
func NewRepository(db database.DB, logger ectologger.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new match rule
func (r *Repository) Create(ctx context.Context, tenantID string, req models.CreateMatchRuleRequest) (*models.MatchRule, error) {
	ctx, span := tracing.StartSpan(ctx, "matchrule.Repository.Create")
	defer span.End()

	log := r.logger.WithContext(ctx).WithFields(map[string]any{
		"method":      "Create",
		"tenant_id":   tenantID,
		"entity_type": req.EntityType,
		"name":        req.Name,
	})

	id := uuid.New().String()
	now := time.Now().UTC()

	rule := &models.MatchRule{
		ID:          id,
		TenantID:    tenantID,
		EntityType:  req.EntityType,
		Name:        req.Name,
		Description: req.Description,
		Priority:    req.Priority,
		IsActive:    req.IsActive,
		Conditions:  req.Conditions,
		ScoreWeight: req.ScoreWeight,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if rule.ScoreWeight == 0 {
		rule.ScoreWeight = 1.0 // Default weight
	}

	sb := sqlbuilder.PostgreSQL.NewInsertBuilder()
	sb.InsertInto("match_rules")
	sb.Cols("id", "tenant_id", "entity_type", "name", "description", "priority", "is_active", "conditions", "score_weight", "created_at", "updated_at")
	sb.Values(rule.ID, rule.TenantID, rule.EntityType, rule.Name, rule.Description, rule.Priority, rule.IsActive, rule.Conditions, rule.ScoreWeight, rule.CreatedAt, rule.UpdatedAt)

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		log.WithError(err).Error("Failed to create match rule")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to create match rule")
	}

	log.WithFields(map[string]any{"id": id}).Info("Created match rule")
	return rule, nil
}

// Get retrieves a match rule by ID
func (r *Repository) Get(ctx context.Context, tenantID string, id string) (*models.MatchRule, error) {
	ctx, span := tracing.StartSpan(ctx, "matchrule.Repository.Get")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "name", "description", "priority", "is_active", "conditions", "score_weight", "created_at", "updated_at", "deleted_at")
	sb.From("match_rules")
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()
	var rule models.MatchRule
	if err := r.db.GetContext(ctx, &rule, query, args...); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf("match rule %s not found", id))
		}
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get match rule")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get match rule")
	}

	return &rule, nil
}

// ListByEntityType retrieves all active match rules for an entity type, ordered by priority
func (r *Repository) ListByEntityType(ctx context.Context, tenantID, entityType string) ([]models.MatchRule, error) {
	ctx, span := tracing.StartSpan(ctx, "matchrule.Repository.ListByEntityType")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "name", "description", "priority", "is_active", "conditions", "score_weight", "created_at", "updated_at", "deleted_at")
	sb.From("match_rules")
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("entity_type", entityType),
		sb.Equal("is_active", true),
		sb.IsNull("deleted_at"),
	)
	sb.OrderBy("priority DESC") // Higher priority first

	query, args := sb.Build()
	var rules []models.MatchRule
	if err := r.db.SelectContext(ctx, &rules, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to list match rules")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list match rules")
	}

	return rules, nil
}

// List retrieves all match rules for a tenant
func (r *Repository) List(ctx context.Context, tenantID string, entityType *string, page, pageSize int) ([]models.MatchRule, int, error) {
	ctx, span := tracing.StartSpan(ctx, "matchrule.Repository.List")
	defer span.End()

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Count total
	countSb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	countSb.Select("COUNT(*)")
	countSb.From("match_rules")
	countWhere := []string{
		countSb.Equal("tenant_id", tenantID),
		countSb.IsNull("deleted_at"),
	}
	if entityType != nil {
		countWhere = append(countWhere, countSb.Equal("entity_type", *entityType))
	}
	countSb.Where(countWhere...)

	countQuery, countArgs := countSb.Build()
	var totalCount int
	if err := r.db.GetContext(ctx, &totalCount, countQuery, countArgs...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to count match rules")
		return nil, 0, httperror.NewHTTPError(http.StatusInternalServerError, "failed to count match rules")
	}

	// Fetch page
	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "name", "description", "priority", "is_active", "conditions", "score_weight", "created_at", "updated_at", "deleted_at")
	sb.From("match_rules")
	where := []string{
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	}
	if entityType != nil {
		where = append(where, sb.Equal("entity_type", *entityType))
	}
	sb.Where(where...)
	sb.OrderBy("priority DESC", "created_at DESC")
	sb.Limit(pageSize).Offset(offset)

	query, args := sb.Build()
	var rules []models.MatchRule
	if err := r.db.SelectContext(ctx, &rules, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to list match rules")
		return nil, 0, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list match rules")
	}

	return rules, totalCount, nil
}

// Update updates a match rule
func (r *Repository) Update(ctx context.Context, tenantID string, id string, req models.UpdateMatchRuleRequest) (*models.MatchRule, error) {
	ctx, span := tracing.StartSpan(ctx, "matchrule.Repository.Update")
	defer span.End()

	// Get existing rule
	existing, err := r.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = req.Description
	}
	if req.Priority != nil {
		existing.Priority = *req.Priority
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}
	if req.Conditions != nil {
		existing.Conditions = req.Conditions
	}
	if req.ScoreWeight != nil {
		existing.ScoreWeight = *req.ScoreWeight
	}
	existing.UpdatedAt = time.Now().UTC()

	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("match_rules")
	sb.Set(
		sb.Assign("name", existing.Name),
		sb.Assign("description", existing.Description),
		sb.Assign("priority", existing.Priority),
		sb.Assign("is_active", existing.IsActive),
		sb.Assign("conditions", existing.Conditions),
		sb.Assign("score_weight", existing.ScoreWeight),
		sb.Assign("updated_at", existing.UpdatedAt),
	)
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
	)

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to update match rule")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to update match rule")
	}

	return existing, nil
}

// Delete soft deletes a match rule
func (r *Repository) Delete(ctx context.Context, tenantID string, id string) error {
	ctx, span := tracing.StartSpan(ctx, "matchrule.Repository.Delete")
	defer span.End()

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("match_rules")
	sb.Set(sb.Assign("deleted_at", now))
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to delete match rule")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete match rule")
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf("match rule %s not found", id))
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{"id": id}).Info("Deleted match rule")
	return nil
}

// GetByID is an alias for Get
func (r *Repository) GetByID(ctx context.Context, tenantID string, id string) (*models.MatchRule, error) {
	return r.Get(ctx, tenantID, id)
}

// GetByEntityType is an alias for ListByEntityType
func (r *Repository) GetByEntityType(ctx context.Context, tenantID, entityType string) ([]models.MatchRule, error) {
	return r.ListByEntityType(ctx, tenantID, entityType)
}
