package matchindex

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

// Repository handles match index persistence and lookups
type Repository struct {
	db     database.DB
	logger ectologger.Logger
}

// NewRepository creates a new match index repository
func NewRepository(db database.DB, logger ectologger.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

// Upsert creates or updates the match index for a staged entity
func (r *Repository) Upsert(ctx context.Context, tenantID string, stagedEntityID uuid.UUID, entityType string, fields map[string]*string) error {
	ctx, span := tracing.StartSpan(ctx, "matchindex.Repository.Upsert")
	defer span.End()

	log := r.logger.WithContext(ctx).WithFields(map[string]any{
		"method":           "Upsert",
		"tenant_id":        tenantID,
		"staged_entity_id": stagedEntityID,
	})

	now := time.Now().UTC()

	// Build upsert query
	sb := sqlbuilder.PostgreSQL.NewInsertBuilder()
	sb.InsertInto("entity_match_index")
	sb.Cols("id", "tenant_id", "staged_entity_id", "entity_type",
		"field_1", "field_2", "field_3", "field_4", "field_5",
		"field_3_soundex", "field_4_soundex", "field_3_metaphone", "field_4_metaphone",
		"name_combined", "created_at", "updated_at")
	
	id := uuid.New()
	sb.Values(id, tenantID, stagedEntityID, entityType,
		fields["field_1"], fields["field_2"], fields["field_3"], fields["field_4"], fields["field_5"],
		nil, nil, nil, nil, // Phonetic fields computed by trigger
		fields["name_combined"], now, now)

	// Use ON CONFLICT to upsert
	query, args := sb.Build()
	query += ` ON CONFLICT (tenant_id, staged_entity_id) DO UPDATE SET
		field_1 = EXCLUDED.field_1,
		field_2 = EXCLUDED.field_2,
		field_3 = EXCLUDED.field_3,
		field_4 = EXCLUDED.field_4,
		field_5 = EXCLUDED.field_5,
		name_combined = EXCLUDED.name_combined,
		updated_at = EXCLUDED.updated_at`

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		log.WithError(err).Error("Failed to upsert match index")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to upsert match index")
	}

	log.Debug("Upserted match index")
	return nil
}

// Delete removes the match index for a staged entity
func (r *Repository) Delete(ctx context.Context, tenantID string, stagedEntityID uuid.UUID) error {
	ctx, span := tracing.StartSpan(ctx, "matchindex.Repository.Delete")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewDeleteBuilder()
	sb.DeleteFrom("entity_match_index")
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("staged_entity_id", stagedEntityID),
	)

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to delete match index")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete match index")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{"staged_entity_id": stagedEntityID}).Debug("Deleted match index")
	return nil
}

// MatchCriteria defines criteria for finding matching entities
type MatchCriteria struct {
	EntityType    string
	ExactFields   map[string]string // field_name -> value for exact match
	FuzzyFields   map[string]string // field_name -> value for fuzzy match
	MinSimilarity float64           // minimum similarity for fuzzy matches (0.0-1.0)
}

// MatchResult represents a potential match found in the index
type MatchResult struct {
	StagedEntityID uuid.UUID `db:"staged_entity_id"`
	EntityType     string    `db:"entity_type"`
	MatchScore     float64   `db:"match_score"`
	MatchDetails   string    `db:"match_details"`
}

// FindMatches finds entities that match the given criteria
func (r *Repository) FindMatches(ctx context.Context, tenantID string, excludeEntityID *uuid.UUID, criteria MatchCriteria) ([]MatchResult, error) {
	ctx, span := tracing.StartSpan(ctx, "matchindex.Repository.FindMatches")
	defer span.End()

	log := r.logger.WithContext(ctx).WithFields(map[string]any{
		"method":      "FindMatches",
		"tenant_id":   tenantID,
		"entity_type": criteria.EntityType,
	})

	if len(criteria.ExactFields) == 0 && len(criteria.FuzzyFields) == 0 {
		return []MatchResult{}, nil
	}

	// Build query dynamically based on criteria
	var conditions []string
	var args []any
	argNum := 1

	// Base conditions
	conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", argNum))
	args = append(args, tenantID)
	argNum++

	conditions = append(conditions, fmt.Sprintf("entity_type = $%d", argNum))
	args = append(args, criteria.EntityType)
	argNum++

	// Exclude self
	if excludeEntityID != nil {
		conditions = append(conditions, fmt.Sprintf("staged_entity_id != $%d", argNum))
		args = append(args, *excludeEntityID)
		argNum++
	}

	// Build score calculation
	var scoreComponents []string

	// Exact field matches
	for field, value := range criteria.ExactFields {
		col := getColumnForField(field)
		if col == "" {
			continue
		}
		conditions = append(conditions, fmt.Sprintf("(%s = $%d OR %s IS NULL)", col, argNum, col))
		scoreComponents = append(scoreComponents, fmt.Sprintf("CASE WHEN %s = $%d THEN 1.0 ELSE 0.0 END", col, argNum))
		args = append(args, value)
		argNum++
	}

	// Fuzzy field matches (using trigram similarity)
	for field, value := range criteria.FuzzyFields {
		col := getColumnForField(field)
		if col == "" {
			continue
		}
		// Use trigram similarity with a threshold
		conditions = append(conditions, fmt.Sprintf("(similarity(%s, $%d) > $%d OR %s IS NULL)", col, argNum, argNum+1, col))
		scoreComponents = append(scoreComponents, fmt.Sprintf("COALESCE(similarity(%s, $%d), 0)", col, argNum))
		args = append(args, value, criteria.MinSimilarity)
		argNum += 2
	}

	if len(scoreComponents) == 0 {
		return []MatchResult{}, nil
	}

	// Build final query
	scoreExpr := "(" + join(scoreComponents, " + ") + ") / " + fmt.Sprintf("%d", len(scoreComponents))
	
	query := fmt.Sprintf(`
		SELECT 
			staged_entity_id,
			entity_type,
			%s as match_score,
			'' as match_details
		FROM entity_match_index
		WHERE %s
		ORDER BY match_score DESC
		LIMIT 100
	`, scoreExpr, join(conditions, " AND "))

	var results []MatchResult
	if err := r.db.SelectContext(ctx, &results, query, args...); err != nil {
		log.WithError(err).Error("Failed to find matches")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to find matches")
	}

	log.WithFields(map[string]any{"count": len(results)}).Debug("Found matches")
	return results, nil
}

// FindExactMatches finds entities with exact matches on specified fields
func (r *Repository) FindExactMatches(ctx context.Context, tenantID, entityType string, excludeEntityID *uuid.UUID, fields map[string]string) ([]uuid.UUID, error) {
	ctx, span := tracing.StartSpan(ctx, "matchindex.Repository.FindExactMatches")
	defer span.End()

	if len(fields) == 0 {
		return []uuid.UUID{}, nil
	}

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("staged_entity_id")
	sb.From("entity_match_index")

	where := []string{
		sb.Equal("tenant_id", tenantID),
		sb.Equal("entity_type", entityType),
	}

	if excludeEntityID != nil {
		where = append(where, sb.NotEqual("staged_entity_id", *excludeEntityID))
	}

	for field, value := range fields {
		col := getColumnForField(field)
		if col != "" {
			where = append(where, sb.Equal(col, value))
		}
	}

	sb.Where(where...)

	query, args := sb.Build()
	var ids []uuid.UUID
	if err := r.db.SelectContext(ctx, &ids, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to find exact matches")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to find exact matches")
	}

	return ids, nil
}

// GetFieldMappings retrieves field mappings for an entity type
func (r *Repository) GetFieldMappings(ctx context.Context, tenantID, entityType string) ([]models.MatchFieldMapping, error) {
	ctx, span := tracing.StartSpan(ctx, "matchindex.Repository.GetFieldMappings")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "source_path", "target_column", "normalizer", "array_handling", "array_filter", "include_phonetic", "include_trigram")
	sb.From("match_field_mappings")
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("entity_type", entityType),
	)
	sb.OrderBy("target_column")

	query, args := sb.Build()
	var mappings []models.MatchFieldMapping
	if err := r.db.SelectContext(ctx, &mappings, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to get field mappings")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get field mappings")
	}

	return mappings, nil
}

// CreateFieldMapping creates a new field mapping
func (r *Repository) CreateFieldMapping(ctx context.Context, tenantID string, mapping models.MatchFieldMapping) (*models.MatchFieldMapping, error) {
	ctx, span := tracing.StartSpan(ctx, "matchindex.Repository.CreateFieldMapping")
	defer span.End()

	mapping.ID = uuid.New().String()
	mapping.TenantID = tenantID

	sb := sqlbuilder.PostgreSQL.NewInsertBuilder()
	sb.InsertInto("match_field_mappings")
	sb.Cols("id", "tenant_id", "entity_type", "source_path", "target_column", "normalizer", "array_handling", "array_filter", "include_phonetic", "include_trigram")
	sb.Values(mapping.ID, mapping.TenantID, mapping.EntityType, mapping.SourcePath, mapping.TargetColumn, mapping.Normalizer, mapping.ArrayHandling, mapping.ArrayFilter, mapping.IncludePhonetic, mapping.IncludeTrigram)

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to create field mapping")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to create field mapping")
	}

	return &mapping, nil
}

// DeleteFieldMappings deletes all field mappings for an entity type
func (r *Repository) DeleteFieldMappings(ctx context.Context, tenantID, entityType string) error {
	ctx, span := tracing.StartSpan(ctx, "matchindex.Repository.DeleteFieldMappings")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewDeleteBuilder()
	sb.DeleteFrom("match_field_mappings")
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("entity_type", entityType),
	)

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to delete field mappings")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete field mappings")
	}

	return nil
}

// GetDB returns the underlying database connection
func (r *Repository) GetDB() *sqlx.DB {
	return r.db.Unsafe()
}

// Helper functions

func getColumnForField(field string) string {
	switch field {
	case "field_1", "field_2", "field_3", "field_4", "field_5":
		return field
	case "email":
		return "field_1"
	case "phone":
		return "field_2"
	case "first_name":
		return "field_3"
	case "last_name":
		return "field_4"
	case "name":
		return "name_combined"
	default:
		return ""
	}
}

func join(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}

// ExtractMatchFields extracts match index fields from entity data using field mappings
func (r *Repository) ExtractMatchFields(ctx context.Context, tenantID, entityType string, data json.RawMessage) (map[string]*string, error) {
	ctx, span := tracing.StartSpan(ctx, "matchindex.Repository.ExtractMatchFields")
	defer span.End()

	// Get field mappings
	mappings, err := r.GetFieldMappings(ctx, tenantID, entityType)
	if err != nil {
		return nil, err
	}

	if len(mappings) == 0 {
		// No mappings configured, return empty
		return map[string]*string{}, nil
	}

	// Parse entity data
	var entityData map[string]any
	if err := json.Unmarshal(data, &entityData); err != nil {
		return nil, httperror.NewHTTPError(http.StatusBadRequest, "invalid entity data JSON")
	}

	fields := make(map[string]*string)
	for _, mapping := range mappings {
		value := extractFieldValue(entityData, mapping.SourcePath)
		if value != nil {
			// Apply normalizer if configured
			if mapping.Normalizer != nil {
				value = applyNormalizer(*value, *mapping.Normalizer)
			}
			fields[mapping.TargetColumn] = value
		}
	}

	return fields, nil
}

// extractFieldValue extracts a field value using JSONPath-like syntax
func extractFieldValue(data map[string]any, path string) *string {
	// Simple dot-notation path extraction
	value := getNestedValue(data, path)
	if value == nil {
		return nil
	}

	// Convert to string
	switch v := value.(type) {
	case string:
		return &v
	case float64:
		s := fmt.Sprintf("%v", v)
		return &s
	case int:
		s := fmt.Sprintf("%d", v)
		return &s
	case bool:
		s := fmt.Sprintf("%t", v)
		return &s
	default:
		// For complex types, JSON encode
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		s := string(b)
		return &s
	}
}

// getNestedValue retrieves a nested value using dot notation
func getNestedValue(data map[string]any, path string) any {
	parts := splitPath(path)
	current := any(data)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return nil
			}
			current = val
		default:
			return nil
		}
	}

	return current
}

// splitPath splits a dot-notation path
func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, c := range path {
		if c == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// applyNormalizer applies a normalization function to a value
func applyNormalizer(value, normalizer string) *string {
	switch normalizer {
	case "lowercase":
		result := toLowerCase(value)
		return &result
	case "trim":
		result := trimSpace(value)
		return &result
	case "normalize_phone":
		result := normalizePhone(value)
		return &result
	case "normalize_email":
		result := normalizeEmail(value)
		return &result
	default:
		return &value
	}
}

func toLowerCase(s string) string {
	result := ""
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			result += string(r + 32)
		} else {
			result += string(r)
		}
	}
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}

func normalizePhone(s string) string {
	// Keep only digits
	result := ""
	for _, r := range s {
		if r >= '0' && r <= '9' {
			result += string(r)
		}
	}
	return result
}

func normalizeEmail(s string) string {
	return toLowerCase(trimSpace(s))
}
