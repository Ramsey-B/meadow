package mappingdefinition

import (
	"context"
	"net/http"
	"time"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/lotus/pkg/mapping"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

type MappingDefinitionRepository interface {
	Upsert(ctx context.Context, definition mapping.MappingDefinition) error
	GetMappingDefinition(ctx context.Context, id string) (mapping.MappingDefinition, error)
	GetActiveMappingDefinition(ctx context.Context, tenantID, id string) (mapping.MappingDefinition, error)
}

type Repository struct {
	db     database.DB
	logger ectologger.Logger
}

// NewRepository creates a new mapping definition repository
func NewRepository(db database.DB, logger ectologger.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

func (r *Repository) Upsert(ctx context.Context, definition mapping.MappingDefinition) error {
	ctx, span := tracing.StartSpan(ctx, "MappingDefinitionRepository.Upsert")
	defer span.End()

	definition.CreatedTS = time.Now().UTC()

	row := FromMappingDefinition(definition)
	ib := mappingDefinitionStruct.InsertInto(mappingDefinitionTable, row)
	ub := ib.OnConflict("id", "tenant_id", "version")
	ub.Set(
		ub.Assign("\"key\"", database.Excluded("\"key\"")),
		ub.Assign("name", database.Excluded("name")),
		ub.Assign("description", database.Excluded("description")),
		ub.Assign("tags", database.Excluded("tags")),
		ub.Assign("is_active", database.Excluded("is_active")),
		ub.Assign("is_deleted", database.Excluded("is_deleted")),
		ub.Assign("source_fields", database.Excluded("source_fields")),
		ub.Assign("target_fields", database.Excluded("target_fields")),
		ub.Assign("steps", database.Excluded("steps")),
		ub.Assign("links", database.Excluded("links")),
		ub.Assign("updated_at", time.Now().UTC()),
	)

	sql, args := ib.Build()

	ctx, tx, err := r.db.GetTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"id":        definition.ID,
		"tenant_id": definition.TenantID,
		"version":   definition.Version,
		"key":       definition.Key,
		"user_id":   definition.UserID,
	}).Info("Upserting mapping definition")
	_, err = tx.ExecContext(ctx, sql, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"id":        definition.ID,
			"tenant_id": definition.TenantID,
			"version":   definition.Version,
			"key":       definition.Key,
			"user_id":   definition.UserID,
		}).Error("error upserting mapping definition")
		return httperror.NewHTTPError(http.StatusInternalServerError, "error upserting mapping definition")
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (r *Repository) GetMappingDefinition(ctx context.Context, id string) (mapping.MappingDefinition, error) {
	ctx, span := tracing.StartSpan(ctx, "MappingDefinitionRepository.GetMappingDefinition")
	defer span.End()

	// Build the query to get the mapping definition
	sb := mappingDefinitionStruct.SelectFrom(mappingDefinitionTable)
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("is_deleted", false),
	)
	sb.OrderBy("version").Desc()
	sb.Limit(1)

	sql, args := sb.Build()

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"id": id,
	}).Info("Getting mapping definition")

	var row MappingDefinitionRow
	err := r.db.GetContext(ctx, &row, sql, args...)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			r.logger.WithContext(ctx).WithFields(map[string]any{
				"id": id,
			}).Warn("Mapping definition not found")
			return mapping.MappingDefinition{}, httperror.NewHTTPError(http.StatusNotFound, "mapping definition not found")
		}

		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"id": id,
		}).Error("error getting mapping definition")
		return mapping.MappingDefinition{}, httperror.NewHTTPError(http.StatusInternalServerError, "error getting mapping definition")
	}

	definition := ToMappingDefinition(&row)

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"id":        definition.ID,
		"tenant_id": definition.TenantID,
		"version":   definition.Version,
		"name":      definition.Name,
	}).Info("Successfully retrieved mapping definition")

	return definition, nil
}

func (r *Repository) GetActiveMappingDefinition(ctx context.Context, tenantID, id string) (mapping.MappingDefinition, error) {
	ctx, span := tracing.StartSpan(ctx, "MappingDefinitionRepository.GetActiveMappingDefinition")
	defer span.End()

	// Build the query to get the active mapping definition
	sb := mappingDefinitionStruct.SelectFrom(mappingDefinitionTable)
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
		sb.Equal("is_active", true),
		sb.Equal("is_deleted", false),
	)
	sb.OrderBy("version").Desc()
	sb.Limit(1)

	sql, args := sb.Build()

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"id":        id,
		"tenant_id": tenantID,
	}).Info("Getting active mapping definition")

	var row MappingDefinitionRow
	err := r.db.GetContext(ctx, &row, sql, args...)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			r.logger.WithContext(ctx).WithFields(map[string]any{
				"id":        id,
				"tenant_id": tenantID,
			}).Warn("Active mapping definition not found")
			return mapping.MappingDefinition{}, httperror.NewHTTPError(http.StatusNotFound, "active mapping definition not found")
		}

		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"id":        id,
			"tenant_id": tenantID,
		}).Error("error getting active mapping definition")
		return mapping.MappingDefinition{}, httperror.NewHTTPError(http.StatusInternalServerError, "error getting active mapping definition")
	}

	definition := ToMappingDefinition(&row)

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"id":        definition.ID,
		"tenant_id": definition.TenantID,
		"version":   definition.Version,
		"name":      definition.Name,
	}).Info("Successfully retrieved active mapping definition")

	return definition, nil
}
