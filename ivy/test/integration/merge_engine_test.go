package integration

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Ramsey-B/ivy/pkg/models"
)

func TestMergeStrategyTypes(t *testing.T) {
	t.Run("AllStrategyTypes", func(t *testing.T) {
		strategies := []models.MergeStrategyType{
			models.MergeStrategyMostRecent,
			models.MergeStrategyMostTrusted,
			models.MergeStrategyCollectAll,
			models.MergeStrategyLongestValue,
			models.MergeStrategyShortestValue,
			models.MergeStrategyFirstValue,
			models.MergeStrategyLastValue,
			models.MergeStrategyMax,
			models.MergeStrategyMin,
			models.MergeStrategySum,
			models.MergeStrategyAverage,
			models.MergeStrategyPreferNonEmpty,
			models.MergeStrategySourcePriority,
		}

		for _, s := range strategies {
			assert.NotEmpty(t, string(s))
		}
	})
}

func TestFieldMergeStrategy_JSON(t *testing.T) {
	t.Run("SerializeFieldStrategies", func(t *testing.T) {
		strategies := []models.FieldMergeStrategy{
			{Field: "email", Strategy: models.MergeStrategyMostRecent},
			{Field: "name", Strategy: models.MergeStrategyMostTrusted},
			{Field: "tags", Strategy: models.MergeStrategyCollectAll, MaxItems: 100, Dedup: true},
			{Field: "score", Strategy: models.MergeStrategyMax},
			{Field: "bio", Strategy: models.MergeStrategyLongestValue},
		}

		data, err := json.Marshal(strategies)
		require.NoError(t, err)

		var parsed []models.FieldMergeStrategy
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Len(t, parsed, 5)
		assert.Equal(t, "email", parsed[0].Field)
		assert.Equal(t, models.MergeStrategyCollectAll, parsed[2].Strategy)
		assert.True(t, parsed[2].Dedup)
	})
}

func TestSourcePriority(t *testing.T) {
	priorities := []models.SourcePriority{
		{Integration: "manual", Priority: 15},
		{Integration: "crm", Priority: 10},
		{Integration: "web", Priority: 5},
		{Integration: "api", Priority: 3},
	}

	t.Run("FindHighestPriority", func(t *testing.T) {
		var maxPriority int
		var mostTrusted string
		for _, p := range priorities {
			if p.Priority > maxPriority {
				maxPriority = p.Priority
				mostTrusted = p.Integration
			}
		}
		assert.Equal(t, "manual", mostTrusted)
		assert.Equal(t, 15, maxPriority)
	})

	t.Run("JSON", func(t *testing.T) {
		data, err := json.Marshal(priorities)
		require.NoError(t, err)

		var parsed []models.SourcePriority
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Len(t, parsed, 4)
	})
}

func TestEntityCluster_Model(t *testing.T) {
	t.Run("CreateCluster", func(t *testing.T) {
		mergedID := uuid.New().String()
		stagedIDs := []string{uuid.New().String(), uuid.New().String(), uuid.New().String()}

		clusters := make([]models.EntityCluster, len(stagedIDs))
		for i, stagedID := range stagedIDs {
			clusters[i] = models.EntityCluster{
				TenantID:       "test-tenant",
				MergedEntityID: mergedID,
				StagedEntityID: stagedID,
			}
		}

		assert.Len(t, clusters, 3)
		for _, c := range clusters {
			assert.Equal(t, mergedID, c.MergedEntityID)
		}
	})
}

func TestMergeConflict_Model(t *testing.T) {
	t.Run("DetectConflict", func(t *testing.T) {
		conflict := models.MergeConflict{
			Field:        "email",
			Values:       []any{"john@a.com", "john@b.com", "john@c.com"},
			Integrations: []string{"crm", "web", "api"},
			Resolution:   "most_recent",
		}

		assert.Equal(t, "email", conflict.Field)
		assert.Len(t, conflict.Values, 3)
		assert.Len(t, conflict.Integrations, 3)
		assert.Equal(t, "most_recent", conflict.Resolution)
	})
}

// stringPtr is defined in helpers_test.go

// Benchmark for JSON parsing
func BenchmarkFieldMergeStrategyJSON(b *testing.B) {
	strategies := []models.FieldMergeStrategy{
		{Field: "email", Strategy: models.MergeStrategyMostRecent},
		{Field: "name", Strategy: models.MergeStrategyMostTrusted},
		{Field: "tags", Strategy: models.MergeStrategyCollectAll, MaxItems: 100, Dedup: true},
	}

	data, _ := json.Marshal(strategies)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var parsed []models.FieldMergeStrategy
		_ = json.Unmarshal(data, &parsed)
	}
}
