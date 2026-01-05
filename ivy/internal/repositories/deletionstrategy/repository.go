package deletionstrategy

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

// Repository handles deletion strategy persistence
type Repository struct {
	db     database.DB
	logger ectologger.Logger
}

// NewRepository creates a new deletion strategy repository
func NewRepository(db database.DB, logger ectologger.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new deletion strategy
func (r *Repository) Create(ctx context.Context, tenantID string, req models.CreateDeletionStrategyRequest) (*models.DeletionStrategy, error) {
	ctx, span := tracing.StartSpan(ctx, "deletionstrategy.Repository.Create")
	defer span.End()

	log := r.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":     tenantID,
		"strategy_type": req.StrategyType,
	})

	// Validation: exactly one of entity_type or relationship_type must be set
	if (req.EntityType == nil && req.RelationshipType == nil) ||
		(req.EntityType != nil && req.RelationshipType != nil) {
		return nil, httperror.NewHTTPError(http.StatusBadRequest, "exactly one of entity_type or relationship_type must be set")
	}

	id := uuid.New().String()
	now := time.Now().UTC()

	// Default config to empty JSON if not provided
	config := req.Config
	if config == nil {
		config = []byte("{}")
	}

	strategy := &models.DeletionStrategy{
		ID:               id,
		TenantID:         tenantID,
		EntityType:       req.EntityType,
		RelationshipType: req.RelationshipType,
		Integration:      req.Integration,
		SourceKey:        req.SourceKey,
		StrategyType:     req.StrategyType,
		Config:           config,
		Priority:         req.Priority,
		Enabled:          req.Enabled,
		Description:      req.Description,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	sb := sqlbuilder.PostgreSQL.NewInsertBuilder()
	sb.InsertInto("deletion_strategies")
	sb.Cols("id", "tenant_id", "entity_type", "relationship_type", "integration", "source_key", "strategy_type", "config", "priority", "enabled", "description", "created_at", "updated_at")
	sb.Values(id, tenantID, req.EntityType, req.RelationshipType, req.Integration, req.SourceKey, req.StrategyType, config, req.Priority, req.Enabled, req.Description, now, now)

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		log.WithError(err).WithFields(map[string]any{"id": id, "tenant_id": tenantID, "entity_type": req.EntityType, "relationship_type": req.RelationshipType, "integration": req.Integration, "source_key": req.SourceKey, "strategy_type": req.StrategyType}).Error("Failed to create deletion strategy")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to create deletion strategy")
	}

	log.WithFields(map[string]any{"id": id}).Info("Created deletion strategy")
	return strategy, nil
}

// Get retrieves a deletion strategy by ID
func (r *Repository) Get(ctx context.Context, tenantID, id string) (*models.DeletionStrategy, error) {
	ctx, span := tracing.StartSpan(ctx, "deletionstrategy.Repository.Get")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "relationship_type", "integration", "source_key", "strategy_type", "config", "priority", "enabled", "description", "created_at", "updated_at", "deleted_at")
	sb.From("deletion_strategies")
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()
	var strategy models.DeletionStrategy
	if err := r.db.GetContext(ctx, &strategy, query, args...); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf("deletion strategy %s not found", id))
		}
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get deletion strategy")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get deletion strategy")
	}

	return &strategy, nil
}

// GetByEntityType retrieves deletion strategies for a specific entity type
// Returns strategies with hierarchical matching: (integration, source_key) > (integration, NULL) > (NULL, NULL)
func (r *Repository) GetByEntityType(ctx context.Context, tenantID, entityType string, integration *string, sourcePlanID *string) ([]models.DeletionStrategy, error) {
	ctx, span := tracing.StartSpan(ctx, "deletionstrategy.Repository.GetByEntityType")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "relationship_type", "integration", "source_key", "strategy_type", "config", "priority", "enabled", "description", "created_at", "updated_at", "deleted_at")
	sb.From("deletion_strategies")

	where := []string{
		sb.Equal("tenant_id", tenantID),
		sb.Equal("entity_type", entityType),
		sb.Equal("enabled", true),
		sb.IsNull("deleted_at"),
	}

	// Build hierarchical source matching:
	// 1. Match exact (integration, source_key)
	// 2. Match (integration, NULL) - applies to all plans in source
	// 3. Match (NULL, NULL) - global default
	if integration != nil && sourcePlanID != nil {
		where = append(where, fmt.Sprintf("((integration = %s AND source_key = %s) OR (integration = %s AND source_key IS NULL) OR integration IS NULL)",
			sb.Var(*integration), sb.Var(*sourcePlanID), sb.Var(*integration)))
	} else if integration != nil {
		where = append(where, fmt.Sprintf("((integration = %s AND source_key IS NULL) OR integration IS NULL)", sb.Var(*integration)))
	} else {
		where = append(where, "integration IS NULL")
	}

	sb.Where(where...)
	sb.OrderBy("priority DESC", "created_at ASC")

	query, args := sb.Build()
	var strategies []models.DeletionStrategy
	if err := r.db.SelectContext(ctx, &strategies, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get deletion strategies by entity type")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get deletion strategies")
	}

	return strategies, nil
}

// GetByRelationshipType retrieves deletion strategies for a specific relationship type
// Returns strategies with hierarchical matching: (integration, source_key) > (integration, NULL) > (NULL, NULL)
func (r *Repository) GetByRelationshipType(ctx context.Context, tenantID, relationshipType string, integration *string, sourcePlanID *string) ([]models.DeletionStrategy, error) {
	ctx, span := tracing.StartSpan(ctx, "deletionstrategy.Repository.GetByRelationshipType")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "relationship_type", "integration", "source_key", "strategy_type", "config", "priority", "enabled", "description", "created_at", "updated_at", "deleted_at")
	sb.From("deletion_strategies")

	where := []string{
		sb.Equal("tenant_id", tenantID),
		sb.Equal("relationship_type", relationshipType),
		sb.Equal("enabled", true),
		sb.IsNull("deleted_at"),
	}

	// Build hierarchical source matching (same as GetByEntityType)
	if integration != nil && sourcePlanID != nil {
		where = append(where, fmt.Sprintf("((integration = %s AND source_key = %s) OR (integration = %s AND source_key IS NULL) OR integration IS NULL)",
			sb.Var(*integration), sb.Var(*sourcePlanID), sb.Var(*integration)))
	} else if integration != nil {
		where = append(where, fmt.Sprintf("((integration = %s AND source_key IS NULL) OR integration IS NULL)", sb.Var(*integration)))
	} else {
		where = append(where, "integration IS NULL")
	}

	sb.Where(where...)
	sb.OrderBy("priority DESC", "created_at ASC")

	query, args := sb.Build()
	var strategies []models.DeletionStrategy
	if err := r.db.SelectContext(ctx, &strategies, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get deletion strategies by relationship type")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get deletion strategies")
	}

	return strategies, nil
}

// List retrieves deletion strategies with filtering and pagination
func (r *Repository) List(ctx context.Context, tenantID string, entityType, relationshipType *string, page, pageSize int) (*models.DeletionStrategyListResponse, error) {
	ctx, span := tracing.StartSpan(ctx, "deletionstrategy.Repository.List")
	defer span.End()

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Build base where clause
	baseWhere := func(sb *sqlbuilder.SelectBuilder) []string {
		where := []string{
			sb.Equal("tenant_id", tenantID),
			sb.IsNull("deleted_at"),
		}
		if entityType != nil {
			where = append(where, sb.Equal("entity_type", *entityType))
		}
		if relationshipType != nil {
			where = append(where, sb.Equal("relationship_type", *relationshipType))
		}
		return where
	}

	// Count total
	countSb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	countSb.Select("COUNT(*)")
	countSb.From("deletion_strategies")
	countSb.Where(baseWhere(countSb)...)

	countQuery, countArgs := countSb.Build()
	var totalCount int
	if err := r.db.GetContext(ctx, &totalCount, countQuery, countArgs...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to count deletion strategies")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to count deletion strategies")
	}

	// Fetch page
	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "relationship_type", "integration", "source_key", "strategy_type", "config", "priority", "enabled", "description", "created_at", "updated_at", "deleted_at")
	sb.From("deletion_strategies")
	sb.Where(baseWhere(sb)...)
	sb.OrderBy("priority DESC", "created_at DESC")
	sb.Limit(pageSize).Offset(offset)

	query, args := sb.Build()
	var strategies []models.DeletionStrategy
	if err := r.db.SelectContext(ctx, &strategies, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to list deletion strategies")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list deletion strategies")
	}

	return &models.DeletionStrategyListResponse{
		Items:      strategies,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

// Update updates a deletion strategy
func (r *Repository) Update(ctx context.Context, tenantID, id string, req models.UpdateDeletionStrategyRequest) (*models.DeletionStrategy, error) {
	ctx, span := tracing.StartSpan(ctx, "deletionstrategy.Repository.Update")
	defer span.End()

	// Check if strategy exists
	existing, err := r.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("deletion_strategies")

	sets := []string{
		sb.Assign("updated_at", now),
	}

	if req.Config != nil {
		sets = append(sets, sb.Assign("config", *req.Config))
	}
	if req.Priority != nil {
		sets = append(sets, sb.Assign("priority", *req.Priority))
	}
	if req.Enabled != nil {
		sets = append(sets, sb.Assign("enabled", *req.Enabled))
	}
	if req.Description != nil {
		sets = append(sets, sb.Assign("description", *req.Description))
	}

	sb.Set(sets...)
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
	)

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to update deletion strategy")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to update deletion strategy")
	}

	// Apply updates to existing object
	if req.Config != nil {
		existing.Config = *req.Config
	}
	if req.Priority != nil {
		existing.Priority = *req.Priority
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.Description != nil {
		existing.Description = req.Description
	}
	existing.UpdatedAt = now

	r.logger.WithContext(ctx).WithFields(map[string]any{"id": id}).Info("Updated deletion strategy")
	return existing, nil
}

// Delete soft-deletes a deletion strategy
func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	ctx, span := tracing.StartSpan(ctx, "deletionstrategy.Repository.Delete")
	defer span.End()

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("deletion_strategies")
	sb.Set(sb.Assign("deleted_at", now))
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to delete deletion strategy")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete deletion strategy")
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf("deletion strategy %s not found", id))
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{"id": id}).Info("Deleted deletion strategy")
	return nil
}

// GetApplicableStrategies returns the highest-priority applicable strategy for each unique entity_type/integration combination
// This is used during execution.completed processing to determine which deletion strategy to apply
func (r *Repository) GetApplicableStrategies(ctx context.Context, tenantID string) ([]models.DeletionStrategy, error) {
	ctx, span := tracing.StartSpan(ctx, "deletionstrategy.Repository.GetApplicableStrategies")
	defer span.End()

	query := `
		SELECT DISTINCT ON (entity_type, relationship_type, integration, source_key)
			id, tenant_id, entity_type, relationship_type, integration, source_key,
			strategy_type, config, priority, enabled, description,
			created_at, updated_at, deleted_at
		FROM deletion_strategies
		WHERE tenant_id = $1
		  AND enabled = true
		  AND deleted_at IS NULL
		ORDER BY entity_type, relationship_type, integration, source_key, priority DESC, created_at ASC
	`

	var strategies []models.DeletionStrategy
	if err := r.db.SelectContext(ctx, &strategies, query, tenantID); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get applicable deletion strategies")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get applicable deletion strategies")
	}

	return strategies, nil
}
