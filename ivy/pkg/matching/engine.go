// Package matching implements entity matching algorithms
package matching

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/google/uuid"

	"github.com/Ramsey-B/ivy/internal/repositories/matchcandidate"
	"github.com/Ramsey-B/ivy/internal/repositories/matchindex"
	"github.com/Ramsey-B/ivy/internal/repositories/matchrule"
	"github.com/Ramsey-B/ivy/internal/repositories/stagedentity"
	"github.com/Ramsey-B/ivy/pkg/extractor"
	"github.com/Ramsey-B/ivy/pkg/models"
	"github.com/Ramsey-B/ivy/pkg/normalizers"
)

// Engine implements entity matching logic
type Engine struct {
	logger            ectologger.Logger
	entityRepo        *stagedentity.Repository
	matchRuleRepo     *matchrule.Repository
	matchIndexRepo    *matchindex.Repository
	matchCandidateRepo *matchcandidate.Repository
	extractor         *extractor.Extractor
	config            EngineConfig
}

// EngineConfig contains configuration for the match engine
type EngineConfig struct {
	AutoMergeThreshold    float64 // Score above which to auto-merge (default: 0.95)
	MinMatchScore         float64 // Minimum score to consider a match (default: 0.5)
	MaxCandidates         int     // Maximum candidates to return per entity (default: 100)
	EnableFuzzyMatching   bool    // Whether to enable fuzzy (trigram) matching
	EnablePhoneticMatching bool   // Whether to enable phonetic matching
}

// DefaultConfig returns default engine configuration
func DefaultConfig() EngineConfig {
	return EngineConfig{
		AutoMergeThreshold:    0.95,
		MinMatchScore:         0.5,
		MaxCandidates:         100,
		EnableFuzzyMatching:   true,
		EnablePhoneticMatching: true,
	}
}

// NewEngine creates a new match engine
func NewEngine(
	logger ectologger.Logger,
	entityRepo *stagedentity.Repository,
	matchRuleRepo *matchrule.Repository,
	matchIndexRepo *matchindex.Repository,
	matchCandidateRepo *matchcandidate.Repository,
	config EngineConfig,
) *Engine {
	return &Engine{
		logger:             logger,
		entityRepo:         entityRepo,
		matchRuleRepo:      matchRuleRepo,
		matchIndexRepo:     matchIndexRepo,
		matchCandidateRepo: matchCandidateRepo,
		extractor:          extractor.New(),
		config:             config,
	}
}

// FindMatches finds potential matches for an entity
func (e *Engine) FindMatches(ctx context.Context, tenantID string, entity *models.StagedEntity) (*models.MatchResult, error) {
	ctx, span := tracing.StartSpan(ctx, "matching.Engine.FindMatches")
	defer span.End()

	log := e.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":   tenantID,
		"entity_id":   entity.ID,
		"entity_type": entity.EntityType,
	})

	log.Debug("Finding matches for entity")

	// Get match rules for this entity type
	rules, err := e.matchRuleRepo.ListByEntityType(ctx, tenantID, entity.EntityType)
	if err != nil {
		return nil, err
	}

	if len(rules) == 0 {
		log.Debug("No match rules defined for entity type")
		return &models.MatchResult{
			SourceEntityID: entity.ID,
			Candidates:     []models.CandidateInfo{},
			TotalMatches:   0,
		}, nil
	}

	// Parse entity data
	var entityData map[string]any
	if err := json.Unmarshal(entity.Data, &entityData); err != nil {
		return nil, fmt.Errorf("failed to parse entity data: %w", err)
	}

	// Collect candidate scores from all rules
	candidateScores := make(map[uuid.UUID]*candidateScore)

	for _, rule := range rules {
		candidates, err := e.evaluateRule(ctx, tenantID, entity, entityData, rule)
		if err != nil {
			log.WithError(err).WithFields(map[string]any{"rule_id": rule.ID}).Warn("Failed to evaluate rule")
			continue
		}

		// Merge candidates
		for candidateID, score := range candidates {
			if existing, ok := candidateScores[candidateID]; ok {
				existing.addScore(score)
			} else {
				candidateScores[candidateID] = score
			}
		}
	}

	// Convert to result format
	candidates := make([]models.CandidateInfo, 0, len(candidateScores))
	for entityID, score := range candidateScores {
		if score.totalScore >= e.config.MinMatchScore {
			candidates = append(candidates, models.CandidateInfo{
				EntityID:     entityID,
				Score:        score.totalScore,
				RulesMatched: score.rulesMatched,
				FieldScores:  score.fieldScores,
				AutoMerge:    score.totalScore >= e.config.AutoMergeThreshold && !score.noMerge,
			})
		}
	}

	// Sort by score descending and limit
	sortCandidatesByScore(candidates)
	if len(candidates) > e.config.MaxCandidates {
		candidates = candidates[:e.config.MaxCandidates]
	}

	log.WithFields(map[string]any{"match_count": len(candidates)}).Debug("Found matches")

	return &models.MatchResult{
		SourceEntityID: entity.ID,
		Candidates:     candidates,
		TotalMatches:   len(candidates),
	}, nil
}

// ProcessMatchCandidates processes match results and stores candidates
func (e *Engine) ProcessMatchCandidates(ctx context.Context, tenantID string, entity *models.StagedEntity, result *models.MatchResult) error {
	ctx, span := tracing.StartSpan(ctx, "matching.Engine.ProcessMatchCandidates")
	defer span.End()

	if len(result.Candidates) == 0 {
		return nil
	}

	// Build match candidates
	candidates := make([]*models.MatchCandidate, 0, len(result.Candidates))
	autoMergeIDs := make([]uuid.UUID, 0)

	for _, c := range result.Candidates {
		// Check for existing candidate
		existing, err := e.matchCandidateRepo.GetByEntityPair(ctx, tenantID, entity.ID, c.EntityID)
		if err != nil {
			return err
		}

		if existing != nil {
			// Update if new score is higher
			if c.Score > existing.MatchScore {
				// Already handled by ON CONFLICT in batch insert
			}
			continue
		}

		rulesJSON, _ := json.Marshal(c.RulesMatched)
		detailsJSON, _ := json.Marshal(c.FieldScores)

		candidate := &models.MatchCandidate{
			TenantID:          tenantID,
			EntityType:        entity.EntityType,
			SourceEntityID:    entity.ID,
			CandidateEntityID: c.EntityID,
			MatchScore:        c.Score,
			MatchDetails:      string(detailsJSON),
			MatchedRules:      string(rulesJSON),
		}

		if c.AutoMerge {
			candidate.Status = models.MatchCandidateStatusAutoMerged
			autoMergeIDs = append(autoMergeIDs, candidate.ID)
		} else {
			candidate.Status = models.MatchCandidateStatusPending
		}

		candidates = append(candidates, candidate)
	}

	if len(candidates) > 0 {
		if err := e.matchCandidateRepo.CreateBatch(ctx, candidates); err != nil {
			return err
		}
	}

	return nil
}

// evaluateRule evaluates a single match rule against an entity
func (e *Engine) evaluateRule(ctx context.Context, tenantID string, entity *models.StagedEntity, entityData map[string]any, rule models.MatchRule) (map[uuid.UUID]*candidateScore, error) {
	ctx, span := tracing.StartSpan(ctx, "matching.Engine.evaluateRule")
	defer span.End()

	// Parse rule conditions
	var conditions models.MatchConditions
	if err := json.Unmarshal(rule.Conditions, &conditions); err != nil {
		return nil, fmt.Errorf("failed to parse rule conditions: %w", err)
	}

	// Build query criteria based on conditions
	exactFields := make(map[string]string)
	fuzzyFields := make(map[string]string)
	var minSimilarity float64 = 0.6

	for _, cond := range conditions.Conditions {
		// Extract field value from entity
		value, err := e.extractor.ExtractString(entityData, cond.Field)
		if err != nil || value == nil || *value == "" {
			if cond.Required {
				// Required field missing, rule doesn't apply
				return nil, nil
			}
			continue
		}

		// Normalize if configured
		normalizedValue := *value
		if cond.Normalizer != nil {
			normalizedValue = normalizers.Apply(normalizedValue, *cond.Normalizer)
		} else if !cond.CaseSensitive {
			normalizedValue = strings.ToLower(normalizedValue)
		}

		switch cond.MatchType {
		case models.MatchRuleTypeExact:
			exactFields[cond.Field] = normalizedValue
		case models.MatchRuleTypeFuzzy:
			if e.config.EnableFuzzyMatching {
				fuzzyFields[cond.Field] = normalizedValue
				if cond.Threshold > 0 {
					minSimilarity = cond.Threshold
				}
			}
		case models.MatchRuleTypePhonetic:
			if e.config.EnablePhoneticMatching {
				// For phonetic matching, we'll use fuzzy as fallback for now
				fuzzyFields[cond.Field] = normalizedValue
			}
		}
	}

	if len(exactFields) == 0 && len(fuzzyFields) == 0 {
		return nil, nil
	}

	// Query match index
	criteria := matchindex.MatchCriteria{
		EntityType:    entity.EntityType,
		ExactFields:   exactFields,
		FuzzyFields:   fuzzyFields,
		MinSimilarity: minSimilarity,
	}

	matches, err := e.matchIndexRepo.FindMatches(ctx, tenantID, &entity.ID, criteria)
	if err != nil {
		return nil, err
	}

	// Convert to candidate scores
	result := make(map[uuid.UUID]*candidateScore)
	for _, match := range matches {
		// Check for no-merge conditions
		noMerge := false
		for _, cond := range conditions.Conditions {
			if cond.NoMerge {
				noMerge = true
				break
			}
		}

		score := &candidateScore{
			totalScore:   match.MatchScore * rule.ScoreWeight,
			rulesMatched: []string{rule.Name},
			fieldScores:  make(map[string]float64),
			noMerge:      noMerge,
		}

		// Add field scores
		for field := range exactFields {
			score.fieldScores[field] = 1.0
		}
		for field := range fuzzyFields {
			score.fieldScores[field] = match.MatchScore
		}

		result[match.StagedEntityID] = score
	}

	return result, nil
}

// candidateScore tracks match scores for a candidate
type candidateScore struct {
	totalScore   float64
	rulesMatched []string
	fieldScores  map[string]float64
	noMerge      bool
}

func (c *candidateScore) addScore(other *candidateScore) {
	// Average scores for now (could be configurable)
	c.totalScore = (c.totalScore + other.totalScore) / 2
	c.rulesMatched = append(c.rulesMatched, other.rulesMatched...)
	for field, score := range other.fieldScores {
		if existing, ok := c.fieldScores[field]; ok {
			c.fieldScores[field] = (existing + score) / 2
		} else {
			c.fieldScores[field] = score
		}
	}
	if other.noMerge {
		c.noMerge = true
	}
}

// sortCandidatesByScore sorts candidates by score descending
func sortCandidatesByScore(candidates []models.CandidateInfo) {
	// Simple bubble sort for now (could use sort.Slice)
	for i := 0; i < len(candidates)-1; i++ {
		for j := 0; j < len(candidates)-i-1; j++ {
			if candidates[j].Score < candidates[j+1].Score {
				candidates[j], candidates[j+1] = candidates[j+1], candidates[j]
			}
		}
	}
}

