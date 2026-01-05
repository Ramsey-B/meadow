// Package matching implements entity matching with a clear separation:
// - Index = facts (precomputed, normalized match fields)
// - Rules = logic (evaluated at query time, never encoded into SQL)
package matching

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"

	"github.com/Gobusters/ectologger"

	"github.com/Ramsey-B/ivy/internal/repositories/matchfield"
	"github.com/Ramsey-B/ivy/internal/repositories/matchrule"
	"github.com/Ramsey-B/ivy/pkg/extractor"
	"github.com/Ramsey-B/ivy/pkg/models"
	"github.com/Ramsey-B/ivy/pkg/normalizers"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

// Config contains configuration for the matching service.
type Config struct {
	MinMatchScore       float64 // Minimum score to consider a match (default: 0.5)
	AutoMergeThreshold  float64 // Score above which to auto-merge (default: 0.95)
	MaxCandidates       int     // Maximum candidates to return per entity (default: 100)
	CandidateCapPerRule int     // Maximum candidates per anchor condition (default: 5000)
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		MinMatchScore:       0.5,
		AutoMergeThreshold:  0.95,
		MaxCandidates:       100,
		CandidateCapPerRule: 5000,
	}
}

// Service handles entity matching with two responsibilities:
// 1. Maintain a match index when entities change (IndexEntity)
// 2. Find matches for an entity (FindMatches)
type Service struct {
	log        ectologger.Logger
	rulesRepo  *matchrule.Repository
	fieldsRepo *matchfield.Repository
	extractor  *extractor.Extractor
	scorer     *Scorer
	cfg        Config
}

// NewService creates a new matching service.
func NewService(
	log ectologger.Logger,
	rulesRepo *matchrule.Repository,
	fieldsRepo *matchfield.Repository,
	cfg Config,
) *Service {
	return &Service{
		log:        log,
		rulesRepo:  rulesRepo,
		fieldsRepo: fieldsRepo,
		extractor:  extractor.New(),
		scorer:     NewScorer(),
		cfg:        cfg,
	}
}

// =============================================================================
// 1️⃣ MAINTAIN MATCH INDEX WHEN ENTITIES CHANGE
// =============================================================================

// IndexEntity extracts match-relevant fields from an entity's JSON data,
// normalizes them, and stores them as indexed rows. This replaces any
// existing rows for this entity.
//
// Purpose: Turn flexible JSON entity data into fast, indexable match facts.
// Outcome: Matching never queries JSON. All matching uses precomputed, indexed values.
func (s *Service) IndexEntity(ctx context.Context, entity *models.StagedEntity) error {
	ctx, span := tracing.StartSpan(ctx, "matching.Service.IndexEntity")
	defer span.End()

	log := s.log.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":   entity.TenantID,
		"entity_id":   entity.ID,
		"entity_type": entity.EntityType,
	})

	// Load all active match rules for this entity type
	rules, err := s.fieldsRepo.GetActiveMatchRules(ctx, entity.TenantID, entity.EntityType)
	if err != nil {
		log.WithError(err).Error("Failed to load match rules")
		return err
	}

	if len(rules) == 0 {
		log.Debug("No match rules defined; clearing any existing match fields")
		return s.fieldsRepo.UpsertMatchFields(ctx, entity.TenantID, entity.EntityType, entity.ID, nil)
	}

	// Parse entity data
	var entityData map[string]any
	if err := json.Unmarshal(entity.Data, &entityData); err != nil {
		msg := err.Error()
		log.WithError(err).Errorf("Failed to parse entity data: %s", msg)
		return err
	}

	// Collect all unique field/matchType/normalizer combinations from rules
	type fieldSpec struct {
		Field      string
		MatchType  models.MatchRuleType
		Normalizer string
	}
	fieldSpecs := make(map[fieldSpec]struct{})

	for _, rule := range rules {
		conditions, err := parseConditions(rule.Conditions)
		if err != nil {
			log.WithError(err).WithFields(map[string]any{"rule_id": rule.ID}).Warn("Failed to parse rule conditions")
			continue
		}

		for _, cond := range conditions {
			norm := "raw"
			if cond.Normalizer != nil && *cond.Normalizer != "" {
				norm = *cond.Normalizer
			}
			fieldSpecs[fieldSpec{
				Field:      cond.Field,
				MatchType:  cond.MatchType,
				Normalizer: norm,
			}] = struct{}{}
		}
	}

	// Extract and normalize each field
	rows := make([]matchfield.MatchFieldRow, 0, len(fieldSpecs))
	for spec := range fieldSpecs {
		row, ok := s.extractMatchField(entityData, spec.Field, spec.MatchType, spec.Normalizer)
		if ok {
			rows = append(rows, row)
		}
	}

	log.WithFields(map[string]any{"field_count": len(rows)}).Debug("Indexed entity match fields")

	return s.fieldsRepo.UpsertMatchFields(ctx, entity.TenantID, entity.EntityType, entity.ID, rows)
}

// extractMatchField extracts a single field value and creates a MatchFieldRow.
func (s *Service) extractMatchField(data map[string]any, field string, matchType models.MatchRuleType, normalizer string) (matchfield.MatchFieldRow, bool) {
	value, err := s.extractor.ExtractString(data, field)
	if err != nil || value == nil || *value == "" {
		return matchfield.MatchFieldRow{}, false
	}

	rawValue := *value

	// Apply normalization
	normalizedValue := rawValue
	if normalizer != "raw" && normalizer != "" {
		normalizedValue = normalizers.Apply(rawValue, normalizer)
	}

	row := matchfield.MatchFieldRow{
		Field:      field,
		MatchType:  matchType,
		Normalizer: normalizer,
		ValueText:  sql.NullString{String: normalizedValue, Valid: true},
	}

	// For phonetic matching, also compute the token
	if matchType == models.MatchRuleTypePhonetic {
		token := s.scorer.Soundex(normalizedValue)
		row.Token = sql.NullString{String: token, Valid: true}
	}

	return row, true
}

// =============================================================================
// 2️⃣ FIND MATCHES FOR AN ENTITY
// =============================================================================

// MatchOutcome represents the result of matching a single entity.
type MatchOutcome struct {
	EntityID    string         `json:"entity_id"`
	Score       float64        `json:"score"`
	AutoMerge   bool           `json:"auto_merge"`
	Blocked     bool           `json:"blocked"`      // True if blocked by a no-merge rule
	RuleMatched string         `json:"rule_matched"` // Name of the rule that produced the best score
	Details     map[string]any `json:"details"`      // Explainability: which conditions matched
}

// MatchResults contains all match outcomes for an entity.
type MatchResults struct {
	SourceEntityID  string         `json:"source_entity_id"`
	Matches         []MatchOutcome `json:"matches"`
	TotalCandidates int            `json:"total_candidates"` // Before filtering by MinMatchScore
}

// FindMatches finds potential matches for an entity by:
// A) Generating candidates using anchor conditions (performance step)
// B) Evaluating each candidate against all rules (correctness step)
//
// Key principles:
// - Rules are alternatives, not additive — pick the best rule, don't average
// - No-merge always wins
// - Required conditions are hard gates
// - Candidate generation must not drop valid matches
func (s *Service) FindMatches(ctx context.Context, tenantID string, entity *models.StagedEntity) (*MatchResults, error) {
	ctx, span := tracing.StartSpan(ctx, "matching.Service.FindMatches")
	defer span.End()

	log := s.log.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":   tenantID,
		"entity_id":   entity.ID,
		"entity_type": entity.EntityType,
	})

	// Load source entity's match fields
	sourceFields, err := s.fieldsRepo.GetMatchFields(ctx, tenantID, entity.ID)
	if err != nil {
		log.WithError(err).Error("Failed to load source match fields")
		return nil, err
	}

	if len(sourceFields) == 0 {
		log.Debug("Source entity has no match fields indexed")
		return &MatchResults{SourceEntityID: entity.ID, Matches: []MatchOutcome{}}, nil
	}

	// Index source fields by field|match_type|normalizer for quick lookup
	sourceIndex := make(map[string]matchfield.MatchFieldRow)
	for _, row := range sourceFields {
		key := fieldKey(row.Field, row.MatchType, row.Normalizer)
		sourceIndex[key] = row
	}

	// Load all active rules
	rules, err := s.fieldsRepo.GetActiveMatchRules(ctx, tenantID, entity.EntityType)
	if err != nil {
		log.WithError(err).Error("Failed to load match rules")
		return nil, err
	}

	if len(rules) == 0 {
		log.Debug("No match rules defined for entity type")
		return &MatchResults{SourceEntityID: entity.ID, Matches: []MatchOutcome{}}, nil
	}

	// Parse and compile all rules
	compiledRules := make([]compiledRule, 0, len(rules))
	for _, rule := range rules {
		conditions, err := parseConditions(rule.Conditions)
		if err != nil {
			log.WithError(err).WithFields(map[string]any{"rule_id": rule.ID}).Warn("Failed to parse rule conditions")
			continue
		}
		compiledRules = append(compiledRules, compiledRule{
			rule:       *rule,
			conditions: conditions,
		})
	}

	// =========================================================================
	// A) CANDIDATE GENERATION (performance step)
	// =========================================================================
	candidates, err := s.generateCandidates(ctx, tenantID, entity, sourceIndex, compiledRules)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		log.Debug("No candidates found")
		return &MatchResults{SourceEntityID: entity.ID, Matches: []MatchOutcome{}}, nil
	}

	log.WithFields(map[string]any{"candidate_count": len(candidates)}).Debug("Generated candidates")

	// =========================================================================
	// B) RULE EVALUATION (correctness step)
	// =========================================================================
	outcomes, err := s.evaluateCandidates(ctx, tenantID, entity, sourceIndex, candidates, compiledRules)
	if err != nil {
		return nil, err
	}

	// Sort by score descending
	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].Score > outcomes[j].Score
	})

	// Limit results
	if len(outcomes) > s.cfg.MaxCandidates {
		outcomes = outcomes[:s.cfg.MaxCandidates]
	}

	log.WithFields(map[string]any{"match_count": len(outcomes)}).Debug("Found matches")

	return &MatchResults{
		SourceEntityID:  entity.ID,
		Matches:         outcomes,
		TotalCandidates: len(candidates),
	}, nil
}

// generateCandidates uses rule conditions to efficiently find candidate entities.
// This step is only for speed. It must not change rule semantics.
//
// Strategy per rule (all conditions within a rule are AND):
// 1. Pure exact/phonetic rules → single SQL query with all conditions ANDed
// 2. Pure fuzzy rules → use best fuzzy condition as anchor
// 3. Mixed rules → use exact/phonetic conditions ANDed in SQL as anchor
//
// Candidates from different rules are unioned (rules are alternatives).
func (s *Service) generateCandidates(
	ctx context.Context,
	tenantID string,
	entity *models.StagedEntity,
	sourceIndex map[string]matchfield.MatchFieldRow,
	rules []compiledRule,
) ([]string, error) {
	ctx, span := tracing.StartSpan(ctx, "matching.Service.generateCandidates")
	defer span.End()

	candidateSet := make(map[string]struct{})

	// Process each rule independently
	for i := range rules {
		if len(candidateSet) >= s.cfg.CandidateCapPerRule {
			break
		}

		rule := &rules[i]
		limit := s.cfg.CandidateCapPerRule - len(candidateSet)

		ids, err := s.generateCandidatesForRule(ctx, tenantID, entity, sourceIndex, rule, limit)
		if err != nil {
			return nil, err
		}

		for _, id := range ids {
			candidateSet[id] = struct{}{}
		}
	}

	candidateIDs := make([]string, 0, len(candidateSet))
	for id := range candidateSet {
		candidateIDs = append(candidateIDs, id)
	}

	return candidateIDs, nil
}

// generateCandidatesForRule generates candidates for a single rule using the optimal strategy.
func (s *Service) generateCandidatesForRule(
	ctx context.Context,
	tenantID string,
	entity *models.StagedEntity,
	sourceIndex map[string]matchfield.MatchFieldRow,
	rule *compiledRule,
	limit int,
) ([]string, error) {
	// Separate conditions by type (excluding no_merge and inverted conditions)
	var indexable []models.MatchCondition // exact, phonetic - can use SQL AND efficiently
	var fuzzy []models.MatchCondition     // fuzzy - need anchor approach

	for _, cond := range rule.conditions {
		if cond.NoMerge {
			continue // no-merge conditions are for blocking, not candidate generation
		}
		if cond.Invert {
			continue // inverted conditions are too broad for candidate generation ("!=" matches almost everything)
		}

		// Verify source has a value for this condition
		norm := normalizerName(cond.Normalizer)
		key := fieldKey(cond.Field, cond.MatchType, norm)
		src, srcOK := sourceIndex[key]
		if !srcOK || !s.hasValue(src, cond.MatchType) {
			// If source is missing a required condition, no candidates possible
			if cond.Required {
				return nil, nil
			}
			continue
		}

		switch cond.MatchType {
		case models.MatchRuleTypeExact, models.MatchRuleTypePhonetic:
			indexable = append(indexable, cond)
		case models.MatchRuleTypeFuzzy:
			fuzzy = append(fuzzy, cond)
			// numeric/date_range not yet implemented
		}
	}

	// Choose strategy based on condition types
	switch {
	case len(indexable) > 0:
		// Case 1 & 3: Has exact/phonetic conditions - use SQL AND for all of them
		// This handles both pure exact/phonetic rules AND mixed rules
		// Fuzzy conditions will be evaluated later in evaluateCandidates
		return s.fieldsRepo.CandidateIDsFromConditions(
			ctx,
			tenantID,
			entity.EntityType,
			entity.ID,
			sourceIndex,
			indexable,
			limit,
		)

	case len(fuzzy) > 0:
		// Case 2: Only fuzzy conditions - use best fuzzy anchor
		bestFuzzy := s.selectBestFuzzyAnchor(fuzzy)
		return s.fieldsRepo.CandidateIDsFromCondition(
			ctx,
			tenantID,
			entity.EntityType,
			entity.ID,
			sourceIndex,
			bestFuzzy,
			limit,
		)

	default:
		// No usable conditions
		return nil, nil
	}
}

// selectBestFuzzyAnchor picks the most selective fuzzy condition.
// Prefers required conditions and higher thresholds (more selective).
func (s *Service) selectBestFuzzyAnchor(conditions []models.MatchCondition) models.MatchCondition {
	best := conditions[0]
	bestScore := fuzzyAnchorScore(best)

	for _, cond := range conditions[1:] {
		score := fuzzyAnchorScore(cond)
		if score > bestScore {
			bestScore = score
			best = cond
		}
	}

	return best
}

// fuzzyAnchorScore returns a priority score for fuzzy anchor selection.
// Higher threshold = more selective = better anchor.
func fuzzyAnchorScore(c models.MatchCondition) float64 {
	score := c.Threshold
	if score <= 0 {
		score = 0.7 // default threshold
	}
	if c.Required {
		score += 1.0 // prefer required conditions
	}
	return score
}

// evaluateCandidates evaluates each candidate against all rules and returns match outcomes.
//
// For each candidate:
// 1. Check no-merge rules first — if any rule with a no_merge condition passes → block
// 2. Score all other rules, taking the best score
// 3. If best score ≥ MinMatchScore → include in results
// 4. Set AutoMerge if score ≥ AutoMergeThreshold
func (s *Service) evaluateCandidates(
	ctx context.Context,
	tenantID string,
	entity *models.StagedEntity,
	sourceIndex map[string]matchfield.MatchFieldRow,
	candidateIDs []string,
	rules []compiledRule,
) ([]MatchOutcome, error) {
	ctx, span := tracing.StartSpan(ctx, "matching.Service.evaluateCandidates")
	defer span.End()

	// Load candidate fields in batch
	candidateFields, err := s.fieldsRepo.LoadFieldsForCandidates(ctx, tenantID, candidateIDs)
	if err != nil {
		return nil, err
	}

	// Build candidate index: candidateID -> fieldKey -> row
	candIndex := make(map[string]map[string]matchfield.MatchFieldRow)
	for _, row := range candidateFields {
		m, ok := candIndex[row.StagedEntityID]
		if !ok {
			m = make(map[string]matchfield.MatchFieldRow)
			candIndex[row.StagedEntityID] = m
		}
		key := fieldKey(row.Field, row.MatchType, row.Normalizer)
		m[key] = row
	}

	// Precompute fuzzy similarities in batch
	fuzzySims, err := s.precomputeFuzzySimilarities(ctx, tenantID, entity.EntityType, sourceIndex, candidateIDs, rules)
	if err != nil {
		return nil, err
	}

	// Evaluate each candidate
	outcomes := make([]MatchOutcome, 0, len(candidateIDs))

	for _, candID := range candidateIDs {
		candFields := candIndex[candID]
		if candFields == nil {
			continue
		}

		outcome := s.evaluateSingleCandidate(candID, sourceIndex, candFields, fuzzySims, rules)

		// Only include if score meets threshold or if blocked (for explainability)
		if outcome.Score >= s.cfg.MinMatchScore || outcome.Blocked {
			outcomes = append(outcomes, outcome)
		}
	}

	return outcomes, nil
}

// evaluateSingleCandidate evaluates one candidate against all rules.
func (s *Service) evaluateSingleCandidate(
	candID string,
	sourceIndex map[string]matchfield.MatchFieldRow,
	candFields map[string]matchfield.MatchFieldRow,
	fuzzySims map[fuzzyKey]map[string]float64,
	rules []compiledRule,
) MatchOutcome {
	// First pass: check for no-merge conditions
	for _, rule := range rules {
		if s.ruleHasNoMergeMatch(sourceIndex, candFields, fuzzySims, candID, rule) {
			return MatchOutcome{
				EntityID:    candID,
				Score:       0,
				AutoMerge:   false,
				Blocked:     true,
				RuleMatched: rule.rule.Name,
				Details:     map[string]any{"blocked_by": rule.rule.Name, "reason": "no_merge condition matched"},
			}
		}
	}

	// Second pass: score each rule, pick the best
	var bestScore float64
	var bestRuleName string
	var bestDetails map[string]any

	for _, rule := range rules {
		score, details, passed := s.scoreRule(sourceIndex, candFields, fuzzySims, candID, rule)
		if passed && score > bestScore {
			bestScore = score
			bestRuleName = rule.rule.Name
			bestDetails = details
		}
	}

	return MatchOutcome{
		EntityID:    candID,
		Score:       bestScore,
		AutoMerge:   bestScore >= s.cfg.AutoMergeThreshold,
		Blocked:     false,
		RuleMatched: bestRuleName,
		Details:     bestDetails,
	}
}

// ruleHasNoMergeMatch checks if a rule has any no_merge condition that matches.
// With Invert=true, the condition triggers when values DON'T match (useful for
// "block merge if departments differ").
func (s *Service) ruleHasNoMergeMatch(
	sourceIndex map[string]matchfield.MatchFieldRow,
	candFields map[string]matchfield.MatchFieldRow,
	fuzzySims map[fuzzyKey]map[string]float64,
	candID string,
	rule compiledRule,
) bool {
	for _, cond := range rule.conditions {
		if !cond.NoMerge {
			continue
		}

		// Check if this no-merge condition passes
		norm := normalizerName(cond.Normalizer)
		key := fieldKey(cond.Field, cond.MatchType, norm)

		src, srcOK := sourceIndex[key]
		cand, candOK := candFields[key]

		// For inverted conditions, missing values mean "different" → condition triggers
		if !srcOK || !candOK {
			if cond.Invert {
				// Values are different (one is missing) → inverted condition passes → block
				return true
			}
			continue
		}

		var rawMatch bool
		switch cond.MatchType {
		case models.MatchRuleTypeExact:
			rawMatch = src.ValueText.Valid && cand.ValueText.Valid && src.ValueText.String == cand.ValueText.String
		case models.MatchRuleTypePhonetic:
			rawMatch = src.Token.Valid && cand.Token.Valid && src.Token.String == cand.Token.String
		case models.MatchRuleTypeFuzzy:
			fk := fuzzyKey{Field: cond.Field, Normalizer: norm}
			if sims, ok := fuzzySims[fk]; ok {
				threshold := cond.Threshold
				if threshold <= 0 {
					threshold = 0.7
				}
				rawMatch = sims[candID] >= threshold
			}
		default:
			continue
		}

		// Apply invert: if invert=true, condition passes when values DON'T match
		conditionPasses := rawMatch
		if cond.Invert {
			conditionPasses = !rawMatch
		}

		if conditionPasses {
			return true
		}
	}

	return false
}

// scoreRule scores a single rule for a candidate.
// Returns (score, details, passed) where passed=false if required conditions fail.
// With Invert=true, conditions pass when values DON'T match.
func (s *Service) scoreRule(
	sourceIndex map[string]matchfield.MatchFieldRow,
	candFields map[string]matchfield.MatchFieldRow,
	fuzzySims map[fuzzyKey]map[string]float64,
	candID string,
	rule compiledRule,
) (float64, map[string]any, bool) {
	var totalWeight float64
	var scoreSum float64
	details := make(map[string]any)

	for _, cond := range rule.conditions {
		// Skip no-merge conditions for scoring (they're only for blocking)
		if cond.NoMerge {
			continue
		}

		norm := normalizerName(cond.Normalizer)
		key := fieldKey(cond.Field, cond.MatchType, norm)

		src, srcOK := sourceIndex[key]
		cand, candOK := candFields[key]

		// Source missing value: if required, rule fails (unless inverted - missing = different = passes)
		if !srcOK || !s.hasValue(src, cond.MatchType) {
			if cond.Required && !cond.Invert {
				return 0, nil, false
			}
			// For inverted conditions, missing source means "different" which passes
			if cond.Invert {
				totalWeight += cond.Weight
				scoreSum += 1.0 * cond.Weight
				details[cond.Field] = map[string]any{"type": string(cond.MatchType), "match": true, "inverted": true, "reason": "source missing, inverted passes"}
			}
			continue
		}

		totalWeight += cond.Weight

		switch cond.MatchType {
		case models.MatchRuleTypeExact:
			if !candOK || !cand.ValueText.Valid {
				// Candidate missing: for inverted, this means "different" which passes
				if cond.Invert {
					scoreSum += 1.0 * cond.Weight
					details[cond.Field] = map[string]any{"type": "exact", "match": true, "inverted": true, "reason": "candidate missing, inverted passes"}
					continue
				}
				if cond.Required {
					return 0, nil, false
				}
				details[cond.Field] = map[string]any{"type": "exact", "match": false, "reason": "candidate missing"}
				continue
			}
			rawMatch := src.ValueText.String == cand.ValueText.String
			match := rawMatch
			if cond.Invert {
				match = !rawMatch
			}
			if !match && cond.Required {
				return 0, nil, false
			}
			if match {
				scoreSum += 1.0 * cond.Weight
			}
			details[cond.Field] = map[string]any{"type": "exact", "match": match, "inverted": cond.Invert}

		case models.MatchRuleTypeFuzzy:
			fk := fuzzyKey{Field: cond.Field, Normalizer: norm}
			sim := 0.0
			if sims, ok := fuzzySims[fk]; ok {
				sim = sims[candID]
			}
			threshold := cond.Threshold
			if threshold <= 0 {
				threshold = 0.7
			}
			rawPass := sim >= threshold
			pass := rawPass
			if cond.Invert {
				pass = !rawPass
			}
			if !pass && cond.Required {
				return 0, nil, false
			}
			if pass {
				// For inverted fuzzy, use (1 - sim) as the score contribution
				if cond.Invert {
					scoreSum += (1.0 - sim) * cond.Weight
				} else {
					scoreSum += sim * cond.Weight
				}
			}
			details[cond.Field] = map[string]any{"type": "fuzzy", "similarity": sim, "threshold": threshold, "pass": pass, "inverted": cond.Invert}

		case models.MatchRuleTypePhonetic:
			if !candOK || !cand.Token.Valid || !src.Token.Valid {
				// Candidate missing: for inverted, this means "different" which passes
				if cond.Invert {
					scoreSum += 1.0 * cond.Weight
					details[cond.Field] = map[string]any{"type": "phonetic", "match": true, "inverted": true, "reason": "candidate missing, inverted passes"}
					continue
				}
				if cond.Required {
					return 0, nil, false
				}
				details[cond.Field] = map[string]any{"type": "phonetic", "match": false, "reason": "candidate missing"}
				continue
			}
			rawMatch := src.Token.String == cand.Token.String
			match := rawMatch
			if cond.Invert {
				match = !rawMatch
			}
			if !match && cond.Required {
				return 0, nil, false
			}
			if match {
				scoreSum += 1.0 * cond.Weight
			}
			details[cond.Field] = map[string]any{"type": "phonetic", "match": match, "inverted": cond.Invert}

		case models.MatchRuleTypeNumeric:
			// TODO: implement numeric matching
			continue

		case models.MatchRuleTypeDateRange:
			// TODO: implement date range matching
			continue
		}
	}

	if totalWeight == 0 {
		return 0, nil, false
	}

	// Normalize to 0..1 and apply rule weight
	ruleScore := (scoreSum / totalWeight) * rule.rule.ScoreWeight

	return ruleScore, details, true
}

// precomputeFuzzySimilarities computes fuzzy similarities in batch for all fuzzy conditions.
func (s *Service) precomputeFuzzySimilarities(
	ctx context.Context,
	tenantID, entityType string,
	sourceIndex map[string]matchfield.MatchFieldRow,
	candidateIDs []string,
	rules []compiledRule,
) (map[fuzzyKey]map[string]float64, error) {
	// Collect unique fuzzy field/normalizer pairs
	fuzzyConds := make(map[fuzzyKey]struct{})
	for _, rule := range rules {
		for _, cond := range rule.conditions {
			if cond.MatchType == models.MatchRuleTypeFuzzy {
				norm := normalizerName(cond.Normalizer)
				fuzzyConds[fuzzyKey{Field: cond.Field, Normalizer: norm}] = struct{}{}
			}
		}
	}

	result := make(map[fuzzyKey]map[string]float64)

	for fk := range fuzzyConds {
		srcKey := fieldKey(fk.Field, models.MatchRuleTypeFuzzy, fk.Normalizer)
		src, ok := sourceIndex[srcKey]
		if !ok || !src.ValueText.Valid {
			continue
		}

		sims, err := s.fieldsRepo.BatchSimilarity(ctx, tenantID, entityType, fk.Field, fk.Normalizer, candidateIDs, src.ValueText.String)
		if err != nil {
			return nil, err
		}
		result[fk] = sims
	}

	return result, nil
}

// =============================================================================
// HELPER TYPES AND FUNCTIONS
// =============================================================================

// fuzzyKey identifies a fuzzy condition by field and normalizer.
type fuzzyKey struct {
	Field      string
	Normalizer string
}

// compiledRule holds a parsed rule with its conditions.
type compiledRule struct {
	rule       models.MatchRule
	conditions []models.MatchCondition
}

// parseConditions parses the JSON conditions from a match rule.
func parseConditions(raw json.RawMessage) ([]models.MatchCondition, error) {
	// Try parsing as MatchConditions wrapper first
	var wrapper models.MatchConditions
	if err := json.Unmarshal(raw, &wrapper); err == nil && len(wrapper.Conditions) > 0 {
		return wrapper.Conditions, nil
	}

	// Fall back to direct array
	var conditions []models.MatchCondition
	if err := json.Unmarshal(raw, &conditions); err != nil {
		return nil, err
	}
	return conditions, nil
}

// conditionPriority returns a priority score for anchor selection.
// Higher is better for candidate generation.
func conditionPriority(c models.MatchCondition) int {
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
	}
	return base
}

// fieldKey creates a lookup key for match field rows.
func fieldKey(field string, matchType models.MatchRuleType, normalizer string) string {
	return field + "|" + string(matchType) + "|" + normalizer
}

// normalizerName returns the normalizer name or "raw" if empty.
func normalizerName(n *string) string {
	if n == nil || *n == "" {
		return "raw"
	}
	return *n
}

// hasValue checks if a row has a value for the given match type.
func (s *Service) hasValue(row matchfield.MatchFieldRow, matchType models.MatchRuleType) bool {
	switch matchType {
	case models.MatchRuleTypeExact, models.MatchRuleTypeFuzzy:
		return row.ValueText.Valid
	case models.MatchRuleTypePhonetic:
		return row.Token.Valid
	case models.MatchRuleTypeNumeric:
		return row.ValueNum.Valid
	case models.MatchRuleTypeDateRange:
		return row.ValueTS.Valid
	default:
		return false
	}
}
