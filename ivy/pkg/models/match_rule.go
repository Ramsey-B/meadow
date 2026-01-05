package models

import (
	"encoding/json"
	"time"
)

// MatchRuleType defines the type of matching algorithm
type MatchRuleType string

const (
	MatchRuleTypeExact     MatchRuleType = "exact"      // Exact string match
	MatchRuleTypeFuzzy     MatchRuleType = "fuzzy"      // Fuzzy string match (trigram)
	MatchRuleTypePhonetic  MatchRuleType = "phonetic"   // Soundex/Metaphone match
	MatchRuleTypeNumeric   MatchRuleType = "numeric"    // Numeric comparison
	MatchRuleTypeDateRange MatchRuleType = "date_range" // Date proximity match
)

// MatchRule defines how to identify matching entities
type MatchRule struct {
	ID          string          `json:"id" db:"id"`
	TenantID    string          `json:"tenant_id" db:"tenant_id"`
	EntityType  string          `json:"entity_type" db:"entity_type"`
	Name        string          `json:"name" db:"name"`
	Description *string         `json:"description,omitempty" db:"description"`
	Priority    int             `json:"priority" db:"priority"`
	IsActive    bool            `json:"is_active" db:"is_active"`
	Conditions  json.RawMessage `json:"conditions" db:"conditions"`
	ScoreWeight float64         `json:"score_weight" db:"score_weight"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
	DeletedAt   *time.Time      `json:"deleted_at,omitempty" db:"deleted_at"`
}

// MatchCondition defines a single matching condition
type MatchCondition struct {
	Field         string        `json:"field"`           // Field name in entity data (dot notation)
	MatchType     MatchRuleType `json:"match_type"`      // Type of match
	Weight        float64       `json:"weight"`          // Weight in overall score (0.0-1.0)
	Required      bool          `json:"required"`        // If true, condition must match for rule to apply
	Threshold     float64       `json:"threshold"`       // Minimum similarity threshold (for fuzzy/phonetic)
	CaseSensitive bool          `json:"case_sensitive"`  // For exact matches
	DateRangeDays int           `json:"date_range_days"` // For date proximity matching
	Normalizer    *string       `json:"normalizer"`      // Normalizer to apply before comparison
	NoMerge       bool          `json:"no_merge"`        // If true and match, block merge (user rule to prevent matches)
	Invert        bool          `json:"invert"`          // If true, condition passes when values DON'T match
}

// MatchConditions is a collection of conditions with operator
type MatchConditions struct {
	Operator   string           `json:"operator"` // "AND" or "OR"
	Conditions []MatchCondition `json:"conditions"`
}

// CreateMatchRuleRequest is the request to create a match rule
type CreateMatchRuleRequest struct {
	EntityType  string          `json:"entity_type" validate:"required"`
	Name        string          `json:"name" validate:"required"`
	Description *string         `json:"description,omitempty"`
	Priority    int             `json:"priority"`
	IsActive    bool            `json:"is_active"`
	Conditions  json.RawMessage `json:"conditions" validate:"required"`
	ScoreWeight float64         `json:"score_weight"`
}

// UpdateMatchRuleRequest is the request to update a match rule
type UpdateMatchRuleRequest struct {
	Name        *string         `json:"name,omitempty"`
	Description *string         `json:"description,omitempty"`
	Priority    *int            `json:"priority,omitempty"`
	IsActive    *bool           `json:"is_active,omitempty"`
	Conditions  json.RawMessage `json:"conditions,omitempty"`
	ScoreWeight *float64        `json:"score_weight,omitempty"`
}

// MatchCandidate represents a potential match between two entities
type MatchCandidate struct {
	ID                string     `json:"id" db:"id"`
	TenantID          string     `json:"tenant_id" db:"tenant_id"`
	EntityType        string     `json:"entity_type" db:"entity_type"`
	SourceEntityID    string     `json:"source_entity_id" db:"source_entity_id"`
	CandidateEntityID string     `json:"candidate_entity_id" db:"candidate_entity_id"`
	MatchScore        float64    `json:"match_score" db:"match_score"`
	MatchDetails      string     `json:"match_details" db:"match_details"`
	Status            string     `json:"status" db:"status"` // pending, approved, rejected, auto_merged
	MatchedRules      string     `json:"matched_rules" db:"matched_rules"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
	ResolvedAt        *time.Time `json:"resolved_at,omitempty" db:"resolved_at"`
	ResolvedBy        *string    `json:"resolved_by,omitempty" db:"resolved_by"`
}

// MatchCandidateStatus constants
const (
	MatchCandidateStatusPending    = "pending"
	MatchCandidateStatusApproved   = "approved"
	MatchCandidateStatusRejected   = "rejected"
	MatchCandidateStatusAutoMerged = "auto_merged"
	MatchCandidateStatusDeferred   = "deferred"
)

// MatchResult represents the result of matching an entity
type MatchResult struct {
	StagedEntityID string
	EntityType     string
	Score          float64
	Details        map[string]any
}

// CandidateInfo contains information about a match candidate
type CandidateInfo struct {
	EntityID     string             `json:"entity_id"`
	Score        float64            `json:"score"`
	RulesMatched []string           `json:"rules_matched"`
	FieldScores  map[string]float64 `json:"field_scores"`
	AutoMerge    bool               `json:"auto_merge"`
}
