package matchfield

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/ivy/pkg/models"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/huandu/go-sqlbuilder"
	"github.com/jmoiron/sqlx"
)

type MatchFieldRow struct {
	TenantID       string               `db:"tenant_id"`
	EntityType     string               `db:"entity_type"`
	StagedEntityID string               `db:"staged_entity_id"`
	Field          string               `db:"field"`
	MatchType      models.MatchRuleType `db:"match_type"`
	Normalizer     string               `db:"normalizer"`

	ValueText sql.NullString  `db:"value_text"`
	ValueNum  sql.NullFloat64 `db:"value_num"`
	ValueTS   sql.NullTime    `db:"value_ts"`
	Token     sql.NullString  `db:"token"`
}

// Repository handles match index persistence and lookups
type Repository struct {
	db     database.DB
	logger ectologger.Logger
}

func NewRepository(db database.DB, logger ectologger.Logger) *Repository {
	return &Repository{db: db, logger: logger}
}

// UpsertMatchFields replaces all match fields for an entity with the provided rows.
func (r *Repository) UpsertMatchFields(ctx context.Context, tenantID string, entityType string, stagedEntityID string, rows []MatchFieldRow) error {
	ctx, span := tracing.StartSpan(ctx, "matchindex.Repository.UpsertMatchFields")
	defer span.End()

	now := time.Now().UTC()
	for i := range rows {
		rows[i].TenantID = tenantID
		rows[i].EntityType = entityType
		rows[i].StagedEntityID = stagedEntityID
		// created_at/updated_at are DB default; we’ll set updated_at on insert
		_ = now
	}

	ctx, tx, err := r.db.GetTx(ctx, nil)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to start transaction")
	}
	defer tx.Rollback(ctx)

	sb := sqlbuilder.PostgreSQL.NewDeleteBuilder()
	sb.DeleteFrom("staged_entity_match_fields")
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("staged_entity_id", stagedEntityID))
	query, args := sb.Build()
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"tenant_id":        tenantID,
			"staged_entity_id": stagedEntityID,
		}).Error("Failed to delete existing match fields")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete existing match fields")
	}

	if len(rows) == 0 {
		if err := tx.Commit(ctx); err != nil {
			r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
				"tenant_id":        tenantID,
				"staged_entity_id": stagedEntityID,
			}).Error("Failed to commit transaction")
			return httperror.NewHTTPError(http.StatusInternalServerError, "failed to commit")
		}
		return nil
	}

	// bulk insert in batches
	const batchSize = 500
	for i := 0; i < len(rows); i += batchSize {
		end := i + batchSize
		if end > len(rows) {
			end = len(rows)
		}

		sb := sqlbuilder.PostgreSQL.NewInsertBuilder()
		sb.InsertInto("staged_entity_match_fields")
		sb.Cols("tenant_id", "entity_type", "staged_entity_id", "field", "match_type", "normalizer", "value_text", "value_num", "value_ts", "token", "created_at", "updated_at")
		for _, row := range rows[i:end] {
			sb.Values(row.TenantID, row.EntityType, row.StagedEntityID, row.Field, row.MatchType, row.Normalizer, row.ValueText, row.ValueNum, row.ValueTS, row.Token, now, now)
		}
		query, args := sb.Build()
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return httperror.NewHTTPError(http.StatusInternalServerError, "failed to insert match fields")
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to commit")
	}
	return nil
}

func (r *Repository) GetActiveMatchRules(ctx context.Context, tenantID, entityType string) ([]*models.MatchRule, error) {
	ctx, span := tracing.StartSpan(ctx, "matchindex.Repository.GetActiveMatchRules")
	defer span.End()

	var rules []*models.MatchRule
	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "entity_type", "priority", "is_active", "score_weight", "conditions")
	sb.From("match_rules")
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("entity_type", entityType), sb.Equal("is_active", true), sb.IsNull("deleted_at"))
	sb.OrderBy("priority DESC")
	query, args := sb.Build()
	err := r.db.SelectContext(ctx, &rules, query, args...)
	if err != nil {
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to load match rules")
	}
	return rules, nil
}

func (r *Repository) GetMatchFields(ctx context.Context, tenantID string, stagedEntityID string) ([]MatchFieldRow, error) {
	var rows []MatchFieldRow
	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("tenant_id", "entity_type", "staged_entity_id", "field", "match_type", "normalizer", "value_text", "value_num", "value_ts", "token")
	sb.From("staged_entity_match_fields")
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("staged_entity_id", stagedEntityID))
	query, args := sb.Build()
	err := r.db.SelectContext(ctx, &rows, query, args...)
	if err != nil {
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to load match fields")
	}
	return rows, nil
}

// CandidateIDsFromCondition returns candidate entity IDs matching a single condition.
// This is used for anchor-based candidate generation in the matching service.
func (r *Repository) CandidateIDsFromCondition(
	ctx context.Context,
	tenantID, entityType string,
	exclude string,
	sourceIndex map[string]MatchFieldRow, // key: field|match_type|normalizer
	c models.MatchCondition,
	limit int,
) ([]string, error) {

	norm := "raw"
	if c.Normalizer != nil && *c.Normalizer != "" {
		norm = *c.Normalizer
	}

	key := fmt.Sprintf("%s|%s|%s", c.Field, c.MatchType, norm)
	src, ok := sourceIndex[key]
	if !ok {
		// if source entity lacks this field, no candidates from this condition
		return nil, nil
	}

	switch c.MatchType {
	case models.MatchRuleTypeExact:
		if !src.ValueText.Valid {
			return nil, nil
		}

		sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
		sb.Select("staged_entity_id")
		sb.From("staged_entity_match_fields")
		sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("entity_type", entityType), sb.NotEqual("staged_entity_id", exclude), sb.Equal("field", c.Field), sb.Equal("match_type", models.MatchRuleTypeExact), sb.Equal("normalizer", norm), sb.Equal("value_text", src.ValueText.String))
		sb.Limit(limit)
		query, args := sb.Build()
		var ids []string
		err := r.db.SelectContext(ctx, &ids, query, args...)
		if err != nil {
			return nil, err
		}
		return ids, nil

	case models.MatchRuleTypeFuzzy:
		if !src.ValueText.Valid {
			return nil, nil
		}
		// threshold: use SQL similarity() filter so we don't bring back junk
		thr := 0.7
		if c.Threshold > 0 {
			thr = c.Threshold
		}
		sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
		sb.Select("staged_entity_id")
		sb.From("staged_entity_match_fields")
		sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("entity_type", entityType), sb.NotEqual("staged_entity_id", exclude), sb.Equal("field", c.Field), sb.Equal("match_type", models.MatchRuleTypeFuzzy), sb.Equal("normalizer", norm), sb.Equal("value_text", src.ValueText.String), sb.GTE("similarity(value_text, $6)", thr))
		sb.Limit(limit)
		query, args := sb.Build()
		var ids []string
		err := r.db.SelectContext(ctx, &ids, query, args...)
		if err != nil {
			return nil, err
		}
		return ids, nil

	case models.MatchRuleTypePhonetic:
		// Here you can store token using soundex/metaphone at extraction time.
		if !src.Token.Valid {
			return nil, nil
		}
		sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
		sb.Select("staged_entity_id")
		sb.From("staged_entity_match_fields")
		sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("entity_type", entityType), sb.NotEqual("staged_entity_id", exclude), sb.Equal("field", c.Field), sb.Equal("match_type", models.MatchRuleTypePhonetic), sb.Equal("normalizer", norm), sb.Equal("token", src.Token.String))
		sb.Limit(limit)
		query, args := sb.Build()
		var ids []string
		err := r.db.SelectContext(ctx, &ids, query, args...)
		if err != nil {
			return nil, err
		}
		return ids, nil

	default:
		// numeric/date_range later
		return nil, nil
	}
}

// CandidateIDsFromConditions returns candidate entity IDs matching ALL given conditions (AND logic).
// This is more efficient than anchor-based generation for rules with only exact/phonetic conditions.
// For fuzzy conditions, use CandidateIDsFromCondition with the best anchor instead.
func (r *Repository) CandidateIDsFromConditions(
	ctx context.Context,
	tenantID, entityType string,
	exclude string,
	sourceIndex map[string]MatchFieldRow,
	conditions []models.MatchCondition,
	limit int,
) ([]string, error) {
	if len(conditions) == 0 {
		return nil, nil
	}

	// Build a query that joins the match_fields table for each condition
	// SELECT DISTINCT t0.staged_entity_id
	// FROM staged_entity_match_fields t0
	// JOIN staged_entity_match_fields t1 ON t0.staged_entity_id = t1.staged_entity_id
	// ...
	// WHERE t0.tenant_id = ? AND t0.entity_type = ? AND t0.staged_entity_id != ?
	//   AND t0.field = ? AND t0.match_type = ? AND t0.normalizer = ? AND t0.value_text = ?
	//   AND t1.field = ? AND t1.match_type = ? AND t1.normalizer = ? AND t1.token = ?
	//   ...

	var args []any
	argIdx := 1

	// Start building the query
	query := "SELECT DISTINCT t0.staged_entity_id FROM staged_entity_match_fields t0"

	// Add JOINs for additional conditions
	for i := 1; i < len(conditions); i++ {
		query += fmt.Sprintf(" JOIN staged_entity_match_fields t%d ON t0.staged_entity_id = t%d.staged_entity_id", i, i)
	}

	// Build WHERE clause
	query += fmt.Sprintf(" WHERE t0.tenant_id = $%d AND t0.entity_type = $%d AND t0.staged_entity_id != $%d", argIdx, argIdx+1, argIdx+2)
	args = append(args, tenantID, entityType, exclude)
	argIdx += 3

	// Add condition predicates
	for i, c := range conditions {
		alias := fmt.Sprintf("t%d", i)

		norm := "raw"
		if c.Normalizer != nil && *c.Normalizer != "" {
			norm = *c.Normalizer
		}

		// Get source value for this condition
		key := fmt.Sprintf("%s|%s|%s", c.Field, c.MatchType, norm)
		src, ok := sourceIndex[key]
		if !ok {
			// Source doesn't have this field - no candidates possible
			return nil, nil
		}

		switch c.MatchType {
		case models.MatchRuleTypeExact:
			if !src.ValueText.Valid {
				return nil, nil
			}
			query += fmt.Sprintf(" AND %s.field = $%d AND %s.match_type = $%d AND %s.normalizer = $%d AND %s.value_text = $%d",
				alias, argIdx, alias, argIdx+1, alias, argIdx+2, alias, argIdx+3)
			args = append(args, c.Field, models.MatchRuleTypeExact, norm, src.ValueText.String)
			argIdx += 4

		case models.MatchRuleTypePhonetic:
			if !src.Token.Valid {
				return nil, nil
			}
			query += fmt.Sprintf(" AND %s.field = $%d AND %s.match_type = $%d AND %s.normalizer = $%d AND %s.token = $%d",
				alias, argIdx, alias, argIdx+1, alias, argIdx+2, alias, argIdx+3)
			args = append(args, c.Field, models.MatchRuleTypePhonetic, norm, src.Token.String)
			argIdx += 4

		case models.MatchRuleTypeFuzzy:
			// Fuzzy requires similarity computation - not suitable for pure SQL AND
			// Caller should filter these out or use anchor approach
			if !src.ValueText.Valid {
				return nil, nil
			}
			thr := 0.7
			if c.Threshold > 0 {
				thr = c.Threshold
			}
			// Use similarity filter but note this is less efficient than exact/phonetic
			query += fmt.Sprintf(" AND %s.field = $%d AND %s.match_type = $%d AND %s.normalizer = $%d AND similarity(%s.value_text, $%d) >= $%d",
				alias, argIdx, alias, argIdx+1, alias, argIdx+2, alias, argIdx+3, argIdx+4)
			args = append(args, c.Field, models.MatchRuleTypeFuzzy, norm, src.ValueText.String, thr)
			argIdx += 5

		default:
			// Numeric/date_range not yet implemented
			continue
		}
	}

	query += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, limit)

	var ids []string
	if err := r.db.SelectContext(ctx, &ids, query, args...); err != nil {
		return nil, err
	}

	return ids, nil
}

// LoadFieldsForCandidates loads all match fields for the given entity IDs.
func (r *Repository) LoadFieldsForCandidates(ctx context.Context, tenantID string, ids []string) ([]MatchFieldRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	// sqlx.In for ANY()
	query, args, err := sqlx.In(`
	  SELECT tenant_id, entity_type, staged_entity_id, field, match_type, normalizer,
			 value_text, value_num, value_ts, token
	  FROM staged_entity_match_fields
	  WHERE tenant_id=? AND staged_entity_id IN (?)
	`, tenantID, ids)
	if err != nil {
		return nil, err
	}
	query = r.db.Unsafe().Rebind(query)

	var rows []MatchFieldRow
	err = r.db.SelectContext(ctx, &rows, query, args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// BatchSimilarity computes fuzzy similarity scores for candidates against a query value.
func (r *Repository) BatchSimilarity(
	ctx context.Context,
	tenantID, entityType string,
	field, normalizer string,
	candidateIDs []string,
	queryValue string,
) (map[string]float64, error) {

	if len(candidateIDs) == 0 {
		return map[string]float64{}, nil
	}

	q, args, err := sqlx.In(`
	  SELECT staged_entity_id, similarity(value_text, ?) AS sim
	  FROM staged_entity_match_fields
	  WHERE tenant_id=? AND entity_type=?
		AND field=? AND match_type='fuzzy' AND normalizer=?
		AND staged_entity_id IN (?)
	`, queryValue, tenantID, entityType, field, normalizer, candidateIDs)
	if err != nil {
		return nil, err
	}
	q = r.db.Unsafe().Rebind(q)

	type row struct {
		ID  string  `db:"staged_entity_id"`
		Sim float64 `db:"sim"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(rows))
	for _, rr := range rows {
		out[rr.ID] = rr.Sim
	}
	return out, nil
}

type compiledRule struct {
	rule       models.MatchRule
	conditions []models.MatchCondition
}

func parseConditions(raw json.RawMessage) ([]models.MatchCondition, error) {
	var conds []models.MatchCondition
	if err := json.Unmarshal(raw, &conds); err != nil {
		return nil, err
	}
	return conds, nil
}

func conditionPriority(c models.MatchCondition) int {
	// Prefer required, then exact/phonetic/numeric/date, fuzzy last.
	base := 0
	if c.Required {
		base += 1000
	}
	switch c.MatchType {
	case models.MatchRuleTypeExact:
		base += 500
	case models.MatchRuleTypePhonetic:
		base += 400
	case models.MatchRuleTypeNumeric:
		base += 300
	case models.MatchRuleTypeDateRange:
		base += 200
	case models.MatchRuleTypeFuzzy:
		base += 100
	default:
		base += 0
	}
	return base
}

func (r *Repository) FindMatchesForEntity(ctx context.Context, tenantID string, stagedEntityID string, limit int) ([]models.MatchResult, error) {
	ctx, span := tracing.StartSpan(ctx, "matchindex.Repository.FindMatchesForEntity")
	defer span.End()

	// 1) load source fields
	sourceRows, err := r.GetMatchFields(ctx, tenantID, stagedEntityID)
	if err != nil {
		return nil, err
	}
	if len(sourceRows) == 0 {
		return []models.MatchResult{}, nil
	}

	entityType := sourceRows[0].EntityType // match fields include entity_type

	// index source rows by field|match_type|normalizer
	sourceIndex := map[string]MatchFieldRow{}
	for _, r0 := range sourceRows {
		key := fmt.Sprintf("%s|%s|%s", r0.Field, r0.MatchType, r0.Normalizer)
		sourceIndex[key] = r0
	}

	// 2) load rules
	rules, err := r.GetActiveMatchRules(ctx, tenantID, entityType)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return []models.MatchResult{}, nil
	}

	// compile + sort conditions by usefulness
	var allConds []models.MatchCondition
	for _, rule := range rules {
		conds, err := parseConditions(rule.Conditions)
		if err != nil {
			return nil, httperror.NewHTTPError(http.StatusBadRequest, "invalid match rule conditions")
		}
		allConds = append(allConds, conds...)
	}
	sort.Slice(allConds, func(i, j int) bool {
		return conditionPriority(allConds[i]) > conditionPriority(allConds[j])
	})

	// 3) generate candidates from best available anchor condition(s)
	candidateSet := map[string]struct{}{}
	const candidateLimit = 5000

	for _, c := range allConds {
		ids, err := r.CandidateIDsFromCondition(ctx, tenantID, entityType, stagedEntityID, sourceIndex, c, candidateLimit)
		if err != nil {
			return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to generate candidates")
		}
		if len(ids) == 0 {
			continue
		}
		for _, id := range ids {
			candidateSet[id] = struct{}{}
		}
		// if we already have some candidates from a strong anchor, stop early
		if len(candidateSet) >= 50 && c.Required {
			break
		}
		if len(candidateSet) >= 500 {
			break
		}
	}

	if len(candidateSet) == 0 {
		return []models.MatchResult{}, nil
	}

	candidateIDs := make([]string, 0, len(candidateSet))
	for id := range candidateSet {
		candidateIDs = append(candidateIDs, id)
	}

	// 4) batch fetch candidate fields (optional; for exact checks)
	candidateRows, err := r.LoadFieldsForCandidates(ctx, tenantID, candidateIDs)
	if err != nil {
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to load candidate fields")
	}

	// build candidate index: candidate -> key -> row
	candIndex := map[string]map[string]MatchFieldRow{}
	for _, rr := range candidateRows {
		m, ok := candIndex[rr.StagedEntityID]
		if !ok {
			m = map[string]MatchFieldRow{}
			candIndex[rr.StagedEntityID] = m
		}
		key := fmt.Sprintf("%s|%s|%s", rr.Field, rr.MatchType, rr.Normalizer)
		m[key] = rr
	}

	// 5) score candidates using rules
	// For fuzzy: compute batch similarity per fuzzy condition
	// Collect unique fuzzy conditions
	type fuzzyKey struct{ Field, Normalizer string }
	fuzzyConds := map[fuzzyKey]models.MatchCondition{}

	for _, rule := range rules {
		conds, _ := parseConditions(rule.Conditions)
		for _, c := range conds {
			if c.MatchType == models.MatchRuleTypeFuzzy {
				norm := "raw"
				if c.Normalizer != nil && *c.Normalizer != "" {
					norm = *c.Normalizer
				}
				fuzzyConds[fuzzyKey{c.Field, norm}] = c
			}
		}
	}

	fuzzySims := map[fuzzyKey]map[string]float64{}
	for fk, c := range fuzzyConds {
		srcKey := fmt.Sprintf("%s|%s|%s", fk.Field, models.MatchRuleTypeFuzzy, fk.Normalizer)
		src, ok := sourceIndex[srcKey]
		if !ok || !src.ValueText.Valid {
			continue
		}
		sims, err := r.BatchSimilarity(ctx, tenantID, entityType, fk.Field, fk.Normalizer, candidateIDs, src.ValueText.String)
		if err != nil {
			return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to compute similarity")
		}
		fuzzySims[fk] = sims
		_ = c
	}

	results := make([]models.MatchResult, 0, len(candidateIDs))

	for _, candID := range candidateIDs {
		cfields := candIndex[candID]
		if cfields == nil {
			continue
		}

		bestScore := 0.0
		bestDetails := map[string]any{}
		matchedAnyRule := false

		// evaluate each rule and take best
		for _, rule := range rules {
			conds, _ := parseConditions(rule.Conditions)

			totalWeight := 0.0
			scoreSum := 0.0
			failedRequired := false
			details := map[string]any{}

			for _, c := range conds {
				norm := "raw"
				if c.Normalizer != nil && *c.Normalizer != "" {
					norm = *c.Normalizer
				}

				key := fmt.Sprintf("%s|%s|%s", c.Field, c.MatchType, norm)
				src, srcOK := sourceIndex[key]
				cand, candOK := cfields[key]

				// if source missing value => this condition can’t contribute; if required, treat as fail
				if !srcOK || (c.MatchType == models.MatchRuleTypeExact && !src.ValueText.Valid) || (c.MatchType == models.MatchRuleTypeFuzzy && !src.ValueText.Valid) {
					if c.Required {
						failedRequired = true
					}
					continue
				}

				totalWeight += c.Weight

				switch c.MatchType {
				case models.MatchRuleTypeExact:
					if !candOK || !cand.ValueText.Valid {
						if c.Required {
							failedRequired = true
						}
						continue
					}
					ok := cand.ValueText.String == src.ValueText.String
					if !ok && c.Required {
						failedRequired = true
					}
					if ok {
						scoreSum += 1.0 * c.Weight
					}
					details[c.Field] = map[string]any{"type": "exact", "match": ok}

				case models.MatchRuleTypeFuzzy:
					fk := fuzzyKey{c.Field, norm}
					sim := 0.0
					if m := fuzzySims[fk]; m != nil {
						sim = m[candID]
					}
					thr := 0.7
					if c.Threshold > 0 {
						thr = c.Threshold
					}
					ok := sim >= thr
					if !ok && c.Required {
						failedRequired = true
					}
					if ok {
						scoreSum += sim * c.Weight
					}
					details[c.Field] = map[string]any{"type": "fuzzy", "sim": sim, "threshold": thr, "pass": ok}

				case models.MatchRuleTypePhonetic:
					if !candOK || !cand.Token.Valid || !src.Token.Valid {
						if c.Required {
							failedRequired = true
						}
						continue
					}
					ok := cand.Token.String == src.Token.String
					if !ok && c.Required {
						failedRequired = true
					}
					if ok {
						scoreSum += 1.0 * c.Weight
					}
					details[c.Field] = map[string]any{"type": "phonetic", "match": ok}

				default:
					// numeric/date_range later
				}
			}

			if failedRequired || totalWeight == 0 {
				continue
			}

			// normalize to 0..1 and apply rule weight
			ruleScore := (scoreSum / totalWeight) * rule.ScoreWeight
			if ruleScore > bestScore {
				bestScore = ruleScore
				bestDetails = details
			}
			matchedAnyRule = true
		}

		if matchedAnyRule {
			results = append(results, models.MatchResult{
				StagedEntityID: candID,
				EntityType:     entityType,
				Score:          bestScore,
				Details:        bestDetails,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })

	if limit <= 0 {
		limit = 100
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}
