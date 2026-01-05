package integration

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Ramsey-B/ivy/pkg/models"
)

func TestDeletionStrategyTypes(t *testing.T) {
	t.Run("AllDeletionTypes", func(t *testing.T) {
		strategies := []models.DeletionStrategyType{
			models.DeletionStrategyExplicit,
			models.DeletionStrategyExecutionBased,
			models.DeletionStrategyStaleness,
			models.DeletionStrategyRetention,
			models.DeletionStrategyComposite,
		}

		for _, s := range strategies {
			assert.NotEmpty(t, string(s))
		}
	})
}

func TestDeletionStrategy_JSON(t *testing.T) {
	entityType1 := "person"
	integration1 := "crm"
	entityType2 := "product"
	integration2 := "api"

	strategies := []models.DeletionStrategy{
		{
			ID:           uuid.New().String(),
			TenantID:     "test-tenant",
			Integration:  &integration1,
			EntityType:   &entityType1,
			StrategyType: models.DeletionStrategyExplicit,
			Enabled:      true,
		},
		{
			ID:           uuid.New().String(),
			TenantID:     "test-tenant",
			Integration:  &integration2,
			EntityType:   &entityType2,
			StrategyType: models.DeletionStrategyExecutionBased,
			Enabled:      true,
		},
	}

	for _, s := range strategies {
		data, err := json.Marshal(s)
		require.NoError(t, err)

		var parsed models.DeletionStrategy
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Equal(t, s.StrategyType, parsed.StrategyType)
	}
}

func TestDeletionStrategyConfigTypes(t *testing.T) {
	t.Run("StalenessConfig", func(t *testing.T) {
		config := models.StalenessConfig{
			MaxAgeDays: 90,
			CheckField: "updated_at",
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)

		var parsed models.StalenessConfig
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Equal(t, 90, parsed.MaxAgeDays)
		assert.Equal(t, "updated_at", parsed.CheckField)
	})

	t.Run("RetentionConfig", func(t *testing.T) {
		config := models.RetentionConfig{
			RetentionDays: 365,
			CheckField:    "created_at",
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)

		var parsed models.RetentionConfig
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Equal(t, 365, parsed.RetentionDays)
		assert.Equal(t, "created_at", parsed.CheckField)
	})

	t.Run("CompositeConfig", func(t *testing.T) {
		maxAge := 90
		retentionDays := 365
		config := models.CompositeConfig{
			Operator: "AND",
			Strategies: []models.CompositeStrategyItem{
				{Type: models.DeletionStrategyStaleness, MaxAgeDays: &maxAge},
				{Type: models.DeletionStrategyRetention, RetentionDays: &retentionDays},
			},
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)

		var parsed models.CompositeConfig
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Equal(t, "AND", parsed.Operator)
		assert.Len(t, parsed.Strategies, 2)
		assert.Equal(t, models.DeletionStrategyStaleness, parsed.Strategies[0].Type)
		assert.Equal(t, 90, *parsed.Strategies[0].MaxAgeDays)
	})
}
