package stagedentity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/google/uuid"
	"github.com/huandu/go-sqlbuilder"
	"github.com/jmoiron/sqlx"

	"github.com/Ramsey-B/ivy/pkg/fingerprint"
	"github.com/Ramsey-B/ivy/pkg/models"
)

// Repository handles staged entity persistence
type Repository struct {
	db     database.DB
	logger ectologger.Logger
}

// FindBySourceIDAndField returns all staged entities matching (tenant_id, entity_type, source_id, source_field).
func (r *Repository) FindBySourceIDAndFields(ctx context.Context, tenantID, entityType, sourceID, sourceField string) ([]models.StagedEntity, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedentity.Repository.FindBySourceIDAndField")
	defer span.End()

	if sourceField == "" {
		sourceField = "source_id" // default to source_id if no field is provided
	}

	if sourceField == "source_id" {
		return r.FindBySourceID(ctx, tenantID, entityType, sourceID)
	}

	return r.FindByDataField(ctx, tenantID, entityType, sourceField, sourceID)
}

// FindBySourceID returns all staged entities matching (tenant_id, entity_type, source_id).
// This can be ambiguous across integration; callers can disambiguate using Integration or UpdatedAt.
// Only returns non-deleted entities.
func (r *Repository) FindBySourceID(ctx context.Context, tenantID, entityType, sourceID string) ([]models.StagedEntity, error) {
	return r.findBySourceID(ctx, tenantID, entityType, sourceID, false)
}

// FindBySourceIDIncludeDeleted returns all staged entities matching (tenant_id, entity_type, source_id),
// including soft-deleted entities. Used for relationship resolution where timing issues may cause
// the entity to be marked deleted before relationship messages arrive.
func (r *Repository) FindBySourceIDIncludeDeleted(ctx context.Context, tenantID, entityType, sourceID string) ([]models.StagedEntity, error) {
	return r.findBySourceID(ctx, tenantID, entityType, sourceID, true)
}

func (r *Repository) findBySourceID(ctx context.Context, tenantID, entityType, sourceID string, includeDeleted bool) ([]models.StagedEntity, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedentity.Repository.findBySourceID")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "source_id", "integration", "source_key", "config_id", "source_execution_id", "execution_id", "last_seen_execution", "data", "fingerprint", "previous_fingerprint", "created_at", "updated_at", "deleted_at")
	sb.From("staged_entities")
	where := []string{
		sb.Equal("tenant_id", tenantID),
		sb.Equal("entity_type", entityType),
		sb.Equal("source_id", sourceID),
	}
	if !includeDeleted {
		where = append(where, sb.IsNull("deleted_at"))
	}
	sb.Where(where...)
	sb.OrderBy("deleted_at NULLS FIRST", "updated_at DESC")
	sb.Limit(10)

	query, args := sb.Build()
	var entities []models.StagedEntity
	if err := r.db.SelectContext(ctx, &entities, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{"tenant_id": tenantID, "entity_type": entityType, "source_id": sourceID}).WithFields(map[string]any{"include_deleted": includeDeleted}).Error("Failed to find staged entities by source_id")
		return nil, httperror.NewHTTPErrorf(http.StatusInternalServerError, "failed to find staged entities: %v", err)
	}
	return entities, nil
}

// FindByDataField returns staged entities where data->>fieldName == fieldValue (exact match).
func (r *Repository) FindByDataField(ctx context.Context, tenantID, entityType, fieldName, fieldValue string) ([]models.StagedEntity, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedentity.Repository.FindByDataField")
	defer span.End()

	// Performance note:
	// - Postgres cannot use expression indexes like (data->>'serial_number') when the JSON key is parameterized (data->>$3).
	// - For high-volume workloads we constrain to a safe allowlist of lookup fields and use an indexable expression.
	fieldName = strings.TrimSpace(fieldName)
	allowed := map[string]struct{}{
		"email":         {},
		"name":          {},
		"device_id":     {},
		"serial_number": {},
	}

	var query string
	if _, ok := allowed[fieldName]; ok {
		query = fmt.Sprintf(`
			SELECT id, tenant_id, entity_type, source_id, integration, source_key, config_id, source_execution_id, execution_id, last_seen_execution, data, fingerprint, previous_fingerprint, created_at, updated_at, deleted_at
			FROM staged_entities
			WHERE tenant_id = $1
			  AND entity_type = $2
			  AND deleted_at IS NULL
			  AND (data ->> '%s') = $3
			ORDER BY updated_at DESC
			LIMIT 10
		`, fieldName)
	} else {
		// Fallback: works for arbitrary keys but will likely do a seq scan at scale.
		query = `
			SELECT id, tenant_id, entity_type, source_id, integration, source_key, config_id, source_execution_id, execution_id, last_seen_execution, data, fingerprint, previous_fingerprint, created_at, updated_at, deleted_at
			FROM staged_entities
			WHERE tenant_id = $1
			  AND entity_type = $2
			  AND deleted_at IS NULL
			  AND (data ->> $3) = $4
			ORDER BY updated_at DESC
			LIMIT 10
		`
	}

	var entities []models.StagedEntity
	var err error
	if _, ok := allowed[fieldName]; ok {
		err = r.db.SelectContext(ctx, &entities, query, tenantID, entityType, fieldValue)
	} else {
		err = r.db.SelectContext(ctx, &entities, query, tenantID, entityType, fieldName, fieldValue)
	}
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{"tenant_id": tenantID, "entity_type": entityType, "field_name": fieldName, "field_value": fieldValue}).Error("Failed to find staged entities by data field")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to find staged entities")
	}
	return entities, nil
}

// GetBySourceAndEntityType retrieves a staged entity by (entity_type, source_id, integration).
func (r *Repository) GetBySourceAndEntityType(ctx context.Context, tenantID, entityType, sourceID, integration string) (*models.StagedEntity, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedentity.Repository.GetBySourceAndEntityType")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "source_id", "integration", "source_key", "config_id", "source_execution_id", "execution_id", "last_seen_execution", "data", "fingerprint", "previous_fingerprint", "created_at", "updated_at", "deleted_at")
	sb.From("staged_entities")
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("entity_type", entityType),
		sb.Equal("source_id", sourceID),
		sb.Equal("integration", integration),
	)
	sb.Limit(1)

	query, args := sb.Build()
	var entity models.StagedEntity
	if err := r.db.GetContext(ctx, &entity, query, args...); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{"tenant_id": tenantID, "entity_type": entityType, "source_id": sourceID, "integration": integration}).Error("Failed to get staged entity by source/type/entity_type")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get staged entity")
	}
	return &entity, nil
}

// NewRepository creates a new staged entity repository
func NewRepository(db database.DB, logger ectologger.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

// UpsertResult contains the result of an upsert operation
type UpsertResult struct {
	Entity    *models.StagedEntity
	IsNew     bool
	IsChanged bool
}

// UpsertOptions contains optional parameters for upsert operations
type UpsertOptions struct {
	// ExcludeFieldsFromFingerprint contains field paths to exclude from fingerprint calculation.
	// These are typically fields that change frequently but don't represent meaningful data changes.
	ExcludeFieldsFromFingerprint map[string]bool
}

// Upsert creates or updates a staged entity based on source_id and entity_type.
// This is a convenience wrapper around UpsertWithOptions with no exclusions.
func (r *Repository) Upsert(ctx context.Context, tenantID string, req models.CreateStagedEntityRequest) (*UpsertResult, error) {
	return r.UpsertWithOptions(ctx, tenantID, req, nil)
}

// UpsertWithOptions creates or updates a staged entity with optional configuration.
// If opts is provided, fields listed in ExcludeFieldsFromFingerprint will be ignored
// when computing the fingerprint for change detection.
//
// This implementation uses a single atomic INSERT...ON CONFLICT query for optimal throughput,
// leveraging PostgreSQL's jsonb_deep_merge() function to handle merges in the database.
func (r *Repository) UpsertWithOptions(ctx context.Context, tenantID string, req models.CreateStagedEntityRequest, opts *UpsertOptions) (*UpsertResult, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedentity.Repository.UpsertWithOptions")
	defer span.End()

	log := r.logger.WithContext(ctx).WithFields(map[string]any{
		"method":      "UpsertWithOptions",
		"tenant_id":   tenantID,
		"entity_type": req.EntityType,
		"source_id":   req.SourceID,
		"integration": req.Integration,
	})

	// Get exclusion fields from options
	var excludeFields map[string]bool
	if opts != nil {
		excludeFields = opts.ExcludeFieldsFromFingerprint
	}

	now := time.Now().UTC()
	execID := req.SourceExecutionID
	id := uuid.New().String()

	// Generate fingerprint for the incoming data
	// Note: This is just for the INSERT case; we'll recompute after merge for updates
	fp, err := fingerprint.GenerateFromJSONWithExclusions(req.Data, excludeFields)
	if err != nil {
		log.WithError(err).WithFields(map[string]any{"id": id, "tenant_id": tenantID, "entity_type": req.EntityType, "source_id": req.SourceID, "integration": req.Integration}).Error("Failed to generate fingerprint")
		return nil, httperror.NewHTTPError(http.StatusBadRequest, "invalid entity data")
	}

	// Single atomic upsert with deep merge in PostgreSQL
	// Column order matches schema: id, tenant_id, config_id, entity_type, source_id, integration, source_key, ...
	query := `
		WITH upsert AS (
			INSERT INTO staged_entities (
				id, tenant_id, config_id, entity_type, source_id, integration,
				source_key, source_execution_id, execution_id, last_seen_execution,
				data, fingerprint, previous_fingerprint, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			ON CONFLICT (tenant_id, entity_type, source_id, integration, source_key, config_id)
			DO UPDATE SET
				source_key = EXCLUDED.source_key,
				source_execution_id = EXCLUDED.source_execution_id,
				execution_id = EXCLUDED.execution_id,
				last_seen_execution = EXCLUDED.last_seen_execution,
				data = staged_entities.data || EXCLUDED.data,
				deleted_at = NULL
			RETURNING
				id, tenant_id, config_id, entity_type, source_id, integration, source_key,
				source_execution_id, execution_id, last_seen_execution, data,
				fingerprint, previous_fingerprint, created_at, updated_at, deleted_at,
				(xmax = 0) AS inserted
		)
		SELECT * FROM upsert
	`

	var result struct {
		models.StagedEntity
		Inserted bool `db:"inserted"`
	}

	err = r.db.GetContext(ctx, &result, query,
		id, tenantID, req.ConfigID, req.EntityType, req.SourceID, req.Integration,
		req.SourceKey, req.SourceExecutionID, execID, execID,
		req.Data, fp, "", now, now,
	)
	if err != nil {
		log.WithError(err).WithFields(map[string]any{"id": id, "tenant_id": tenantID, "entity_type": req.EntityType, "source_id": req.SourceID, "integration": req.Integration}).Error("Failed to upsert staged entity")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to upsert staged entity")
	}

	// For inserts, fingerprint is already correct
	if result.Inserted {
		log.WithFields(map[string]any{"id": result.ID}).Info("Created staged entity")
		return &UpsertResult{Entity: &result.StagedEntity, IsNew: true, IsChanged: true}, nil
	}

	// For updates, recompute fingerprint with merged data and update if changed
	mergedFp, err := fingerprint.GenerateFromJSONWithExclusions(result.Data, excludeFields)
	if err != nil {
		log.WithError(err).WithFields(map[string]any{"id": result.ID}).Error("Failed to generate fingerprint for merged data")
		return nil, httperror.NewHTTPError(http.StatusBadRequest, "invalid merged entity data")
	}

	changed := fingerprint.HasChanged(result.Fingerprint, mergedFp)
	if changed {
		sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
		sb.Update("staged_entities")
		sb.Set(sb.Assign("fingerprint", mergedFp), sb.Assign("previous_fingerprint", result.Fingerprint), sb.Assign("updated_at", now))
		sb.Where(sb.Equal("id", result.ID))
		query, args := sb.Build()
		if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
			log.WithError(err).WithFields(map[string]any{"id": result.ID}).Error("Failed to update fingerprint after merge")
			return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to update fingerprint")
		}

		result.PreviousFingerprint = result.Fingerprint
		result.Fingerprint = mergedFp
		result.UpdatedAt = now

		log.WithFields(map[string]any{"id": result.ID}).Info("Updated staged entity")
		return &UpsertResult{Entity: &result.StagedEntity, IsNew: false, IsChanged: true}, nil
	}

	log.WithFields(map[string]any{"id": result.ID}).Debug("Marked staged entity as seen (unchanged)")
	return &UpsertResult{Entity: &result.StagedEntity, IsNew: false, IsChanged: false}, nil
}

// Get retrieves a staged entity by ID
func (r *Repository) Get(ctx context.Context, tenantID string, id string) (*models.StagedEntity, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedentity.Repository.Get")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "source_id", "integration", "source_key", "config_id", "source_execution_id", "execution_id", "last_seen_execution", "data", "fingerprint", "previous_fingerprint", "created_at", "updated_at", "deleted_at")
	sb.From("staged_entities")
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()
	var entity models.StagedEntity
	if err := r.db.GetContext(ctx, &entity, query, args...); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, httperror.NewHTTPErrorf(http.StatusNotFound, "staged entity %s not found", id)
		}
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{"id": id, "tenant_id": tenantID}).Error("Failed to get staged entity")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get staged entity")
	}

	return &entity, nil
}

// GetByIDs retrieves a staged entity by IDs
func (r *Repository) GetByIDs(ctx context.Context, tenantID string, ids []string) ([]models.StagedEntity, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedentity.Repository.GetByIDs")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "source_id", "integration", "source_key", "config_id", "source_execution_id", "execution_id", "last_seen_execution", "data", "fingerprint", "previous_fingerprint", "created_at", "updated_at", "deleted_at")
	sb.From("staged_entities")
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.In("id", sqlbuilder.Flatten(ids)...),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()
	var entities []models.StagedEntity
	if err := r.db.SelectContext(ctx, &entities, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{"tenant_id": tenantID, "ids": ids}).Error("Failed to get staged entities by IDs")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get staged entities")
	}
	return entities, nil
}

// GetBySourceID retrieves a staged entity by source_id and entity_type
func (r *Repository) GetBySourceID(ctx context.Context, tenantID, entityType, sourceID string) (*models.StagedEntity, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedentity.Repository.GetBySourceID")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "source_id", "integration", "source_key", "config_id", "source_execution_id", "execution_id", "last_seen_execution", "data", "fingerprint", "previous_fingerprint", "created_at", "updated_at", "deleted_at")
	sb.From("staged_entities")
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("entity_type", entityType),
		sb.Equal("source_id", sourceID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()
	var entity models.StagedEntity
	if err := r.db.GetContext(ctx, &entity, query, args...); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf("staged entity with source_id %s not found", sourceID))
		}
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{"tenant_id": tenantID, "entity_type": entityType, "source_id": sourceID}).Error("Failed to get staged entity by source_id")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get staged entity")
	}

	return &entity, nil
}

// List retrieves staged entities with filtering and pagination
func (r *Repository) List(ctx context.Context, tenantID string, entityType *string, page, pageSize int) (*models.StagedEntityListResponse, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedentity.Repository.List")
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
	countSb.From("staged_entities")
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
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{"tenant_id": tenantID, "entity_type": entityType, "page": page, "page_size": pageSize}).Error("Failed to count staged entities")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to count staged entities")
	}

	// Fetch page
	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "source_id", "integration", "source_key", "config_id", "source_execution_id", "execution_id", "last_seen_execution", "data", "fingerprint", "previous_fingerprint", "created_at", "updated_at", "deleted_at")
	sb.From("staged_entities")
	where := []string{
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	}
	if entityType != nil {
		where = append(where, sb.Equal("entity_type", *entityType))
	}
	sb.Where(where...)
	sb.OrderBy("created_at DESC")
	sb.Limit(pageSize).Offset(offset)

	query, args := sb.Build()
	var entities []models.StagedEntity
	if err := r.db.SelectContext(ctx, &entities, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{"tenant_id": tenantID, "entity_type": entityType, "page": page, "page_size": pageSize}).Error("Failed to list staged entities")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list staged entities")
	}

	return &models.StagedEntityListResponse{
		Items:      entities,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

// SoftDelete marks a staged entity as deleted
func (r *Repository) SoftDelete(ctx context.Context, tenantID string, id string) error {
	ctx, span := tracing.StartSpan(ctx, "stagedentity.Repository.SoftDelete")
	defer span.End()

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("staged_entities")
	sb.Set(sb.Assign("deleted_at", now))
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{"id": id, "tenant_id": tenantID}).Error("Failed to soft delete staged entity")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete staged entity")
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf("staged entity %s not found", id))
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{"id": id}).Info("Soft deleted staged entity")
	return nil
}

// MarkDeletedExceptExecution marks entities as deleted if they weren't in the given execution
// This is used for execution-based deletion strategy
func (r *Repository) MarkDeletedExceptExecution(ctx context.Context, tenantID, sourceKey, executionID string, entityType *string) (int64, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedentity.Repository.MarkDeletedExceptExecution")
	defer span.End()

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("staged_entities")
	sb.Set(sb.Assign("deleted_at", now))

	where := []string{
		sb.Equal("tenant_id", tenantID),
		sb.Equal("source_key", sourceKey),
		sb.NotEqual("source_execution_id", executionID),
		sb.IsNull("deleted_at"),
	}
	if entityType != nil {
		where = append(where, sb.Equal("entity_type", *entityType))
	}
	sb.Where(where...)

	query, args := sb.Build()
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{"tenant_id": tenantID, "source_key": sourceKey, "execution_id": executionID, "entity_type": entityType}).Error("Failed to mark deleted except execution")
		return 0, httperror.NewHTTPError(http.StatusInternalServerError, "failed to mark deleted")
	}

	rows, _ := result.RowsAffected()
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"source_key":   sourceKey,
		"execution_id": executionID,
		"count":        rows,
	}).Info("Marked entities deleted")
	return rows, nil
}

// GetDB returns the underlying database connection for transactions
func (r *Repository) GetDB() *sqlx.DB {
	return r.db.Unsafe()
}

// GetBySource retrieves a staged entity by source_id and integration
func (r *Repository) GetBySource(ctx context.Context, tenantID, sourceID, integration string) (*models.StagedEntity, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedentity.Repository.GetBySource")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "source_id", "integration", "source_key", "config_id", "source_execution_id", "execution_id", "last_seen_execution", "data", "fingerprint", "previous_fingerprint", "created_at", "updated_at", "deleted_at")
	sb.From("staged_entities")
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("source_id", sourceID),
		sb.Equal("integration", integration),
		sb.IsNull("deleted_at"),
	)

	query, args := sb.Build()
	var entity models.StagedEntity
	if err := r.db.GetContext(ctx, &entity, query, args...); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil // Not found, return nil instead of error
		}
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{"tenant_id": tenantID, "source_id": sourceID, "integration": integration}).Error("Failed to get staged entity by source")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get staged entity")
	}

	return &entity, nil
}

// Delete soft deletes a staged entity (alias for SoftDelete)
func (r *Repository) Delete(ctx context.Context, tenantID string, id string) error {
	return r.SoftDelete(ctx, tenantID, id)
}

// SelectRaw executes a raw SELECT query and scans results into dest
func (r *Repository) SelectRaw(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return r.db.SelectContext(ctx, dest, query, args...)
}

// ExecRaw executes a raw query
func (r *Repository) ExecRaw(ctx context.Context, query string, args ...interface{}) (interface{}, error) {
	return r.db.ExecContext(ctx, query, args...)
}

// deepMergeJSON performs a deep merge of two JSON objects.
// The source JSON is merged into the target JSON, with source values taking precedence.
// Returns the merged JSON or an error if unmarshaling/marshaling fails.
func deepMergeJSON(target, source json.RawMessage) (json.RawMessage, error) {
	var targetMap map[string]any
	var sourceMap map[string]any

	if err := json.Unmarshal(target, &targetMap); err != nil {
		return nil, httperror.NewHTTPErrorf(http.StatusInternalServerError, "failed to unmarshal target JSON: %v", err)
	}

	if err := json.Unmarshal(source, &sourceMap); err != nil {
		return nil, httperror.NewHTTPErrorf(http.StatusInternalServerError, "failed to unmarshal source JSON: %v", err)
	}

	deepMerge(targetMap, sourceMap)

	merged, err := json.Marshal(targetMap)
	if err != nil {
		return nil, httperror.NewHTTPErrorf(http.StatusInternalServerError, "failed to marshal merged JSON: %v", err)
	}

	return merged, nil
}

// deepMerge recursively merges source map into target map.
// For nested maps, it merges recursively. For all other types, source values overwrite target values.
func deepMerge(target, source map[string]any) {
	for key, sourceVal := range source {
		if targetVal, exists := target[key]; exists {
			// Both are maps - merge recursively
			targetMap, targetIsMap := targetVal.(map[string]any)
			sourceMap, sourceIsMap := sourceVal.(map[string]any)

			if targetIsMap && sourceIsMap {
				deepMerge(targetMap, sourceMap)
				continue
			}
		}
		// Otherwise just overwrite
		target[key] = sourceVal
	}
}

// FindByTypeAndIntegration finds all entities of a given type and integration.
// Used for criteria-based relationship evaluation against existing entities.
func (r *Repository) FindByTypeAndIntegration(ctx context.Context, tenantID, entityType, integration string) ([]models.StagedEntity, error) {
	ctx, span := tracing.StartSpan(ctx, "stagedentity.Repository.FindByTypeAndIntegration")
	defer span.End()

	query := `
		SELECT id, tenant_id, config_id, entity_type, source_id, integration, source_key,
		       source_execution_id, execution_id, last_seen_execution,
		       data, fingerprint, previous_fingerprint, created_at, updated_at, deleted_at
		FROM staged_entities
		WHERE tenant_id = $1
		  AND entity_type = $2
		  AND integration = $3
		  AND deleted_at IS NULL
		ORDER BY created_at
	`

	var entities []models.StagedEntity
	err := r.db.SelectContext(ctx, &entities, query, tenantID, entityType, integration)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"tenant_id":   tenantID,
			"entity_type": entityType,
			"integration": integration,
		}).Error("Failed to find entities by type and integration")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to find entities")
	}

	return entities, nil
}
