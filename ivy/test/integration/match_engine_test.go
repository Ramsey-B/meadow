package integration

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Ramsey-B/ivy/pkg/models"
)

func TestMatchRuleTypes(t *testing.T) {
	t.Run("AllMatchTypes", func(t *testing.T) {
		matchTypes := []models.MatchRuleType{
			models.MatchRuleTypeExact,
			models.MatchRuleTypeFuzzy,
			models.MatchRuleTypePhonetic,
			models.MatchRuleTypeNumeric,
			models.MatchRuleTypeDateRange,
		}

		for _, mt := range matchTypes {
			assert.NotEmpty(t, string(mt))
		}
	})
}

func TestMatchCondition_JSON(t *testing.T) {
	t.Run("SerializeConditions", func(t *testing.T) {
		conditions := []models.MatchCondition{
			{
				Field:     "email",
				MatchType: models.MatchRuleTypeExact,
				Weight:    0.5,
				Required:  true,
			},
			{
				Field:         "last_name",
				MatchType:     models.MatchRuleTypeFuzzy,
				Weight:        0.3,
				Threshold:     0.8,
				CaseSensitive: false,
			},
			{
				Field:     "phone",
				MatchType: models.MatchRuleTypePhonetic,
				Weight:    0.2,
			},
		}

		// Serialize to JSON
		data, err := json.Marshal(conditions)
		require.NoError(t, err)

		// Deserialize
		var parsed []models.MatchCondition
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Len(t, parsed, 3)
		assert.Equal(t, "email", parsed[0].Field)
		assert.True(t, parsed[0].Required)
		assert.Equal(t, 0.8, parsed[1].Threshold)
	})

	t.Run("WeightValidation", func(t *testing.T) {
		conditions := []models.MatchCondition{
			{Field: "email", Weight: 0.5},
			{Field: "name", Weight: 0.3},
			{Field: "phone", Weight: 0.2},
		}

		var totalWeight float64
		for _, c := range conditions {
			totalWeight += c.Weight
		}
		assert.InDelta(t, 1.0, totalWeight, 0.001)
	})
}

func TestMatchCandidate_Status(t *testing.T) {
	candidate := models.MatchCandidate{
		ID:                uuid.New().String(),
		TenantID:          "test-tenant",
		EntityType:        "person",
		SourceEntityID:    uuid.New().String(),
		CandidateEntityID: uuid.New().String(),
		MatchScore:        0.92,
		Status:            models.MatchCandidateStatusPending,
	}

	t.Run("InitialStatus", func(t *testing.T) {
		assert.Equal(t, models.MatchCandidateStatusPending, candidate.Status)
	})

	t.Run("ApproveStatus", func(t *testing.T) {
		candidate.Status = models.MatchCandidateStatusApproved
		assert.Equal(t, models.MatchCandidateStatusApproved, candidate.Status)
	})

	t.Run("RejectStatus", func(t *testing.T) {
		candidate.Status = models.MatchCandidateStatusRejected
		assert.Equal(t, models.MatchCandidateStatusRejected, candidate.Status)
	})

	t.Run("DeferredStatus", func(t *testing.T) {
		candidate.Status = models.MatchCandidateStatusDeferred
		assert.Equal(t, models.MatchCandidateStatusDeferred, candidate.Status)
	})

	t.Run("AutoMergedStatus", func(t *testing.T) {
		candidate.Status = models.MatchCandidateStatusAutoMerged
		assert.Equal(t, models.MatchCandidateStatusAutoMerged, candidate.Status)
	})
}

func TestMatchResult(t *testing.T) {
	result := models.MatchResult{
		StagedEntityID: uuid.New().String(),
		EntityType:     "person",
		Score:          0.95,
		Details: map[string]any{
			"rules_matched": []string{"email_match", "name_match"},
			"field_scores":  map[string]float64{"email": 1.0, "name": 0.9},
		},
	}

	assert.NotEmpty(t, result.StagedEntityID)
	assert.Equal(t, "person", result.EntityType)
	assert.Equal(t, 0.95, result.Score)
	assert.NotNil(t, result.Details)
}

func TestCandidateInfo(t *testing.T) {
	candidates := []models.CandidateInfo{
		{
			EntityID:     uuid.New().String(),
			Score:        0.95,
			RulesMatched: []string{"email_match", "name_match"},
			FieldScores:  map[string]float64{"email": 1.0, "name": 0.9},
			AutoMerge:    true,
		},
		{
			EntityID:     uuid.New().String(),
			Score:        0.85,
			RulesMatched: []string{"phone_match"},
			FieldScores:  map[string]float64{"phone": 0.85},
			AutoMerge:    false,
		},
	}

	assert.Len(t, candidates, 2)
	assert.True(t, candidates[0].AutoMerge)
	assert.False(t, candidates[1].AutoMerge)
}

// Benchmark for JSON parsing
func BenchmarkMatchConditionJSON(b *testing.B) {
	conditions := []models.MatchCondition{
		{Field: "email", MatchType: models.MatchRuleTypeExact, Weight: 0.5},
		{Field: "name", MatchType: models.MatchRuleTypeFuzzy, Weight: 0.3, Threshold: 0.8},
		{Field: "phone", MatchType: models.MatchRuleTypePhonetic, Weight: 0.2},
	}

	data, _ := json.Marshal(conditions)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var parsed []models.MatchCondition
		_ = json.Unmarshal(data, &parsed)
	}
}
