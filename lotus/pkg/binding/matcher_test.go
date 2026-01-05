package binding

import (
	"testing"

	"github.com/Ramsey-B/lotus/pkg/kafka"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatcherLoadBindings(t *testing.T) {
	matcher := NewMatcher()

	bindings := []*models.Binding{
		{ID: "b1", TenantID: "tenant-1", IsEnabled: true},
		{ID: "b2", TenantID: "tenant-1", IsEnabled: true},
		{ID: "b3", TenantID: "tenant-1", IsEnabled: false}, // Disabled
	}

	matcher.LoadBindings("tenant-1", bindings)

	assert.Equal(t, 2, matcher.BindingCount())
	assert.Equal(t, 1, matcher.TenantCount())
}

func TestMatcherUpdateBinding(t *testing.T) {
	matcher := NewMatcher()

	// Add initial binding
	matcher.UpdateBinding(&models.Binding{
		ID:        "b1",
		TenantID:  "tenant-1",
		IsEnabled: true,
	})

	assert.Equal(t, 1, matcher.BindingCount())

	// Update binding
	matcher.UpdateBinding(&models.Binding{
		ID:        "b1",
		TenantID:  "tenant-1",
		IsEnabled: true,
		Name:      "Updated",
	})

	assert.Equal(t, 1, matcher.BindingCount())

	// Disable binding
	matcher.UpdateBinding(&models.Binding{
		ID:        "b1",
		TenantID:  "tenant-1",
		IsEnabled: false,
	})

	assert.Equal(t, 0, matcher.BindingCount())
}

func TestMatcherMatchByTenant(t *testing.T) {
	matcher := NewMatcher()

	matcher.LoadBindings("tenant-1", []*models.Binding{
		{ID: "b1", TenantID: "tenant-1", MappingID: "m1", IsEnabled: true},
	})

	matcher.LoadBindings("tenant-2", []*models.Binding{
		{ID: "b2", TenantID: "tenant-2", MappingID: "m2", IsEnabled: true},
	})

	// Message from tenant-1
	msg := &kafka.ReceivedMessage{
		Headers: kafka.MessageHeaders{TenantID: "tenant-1"},
		Data:    map[string]any{},
	}

	results := matcher.Match(msg)
	require.Len(t, results, 1)
	assert.Equal(t, "b1", results[0].Binding.ID)

	// Message from tenant-2
	msg2 := &kafka.ReceivedMessage{
		Headers: kafka.MessageHeaders{TenantID: "tenant-2"},
		Data:    map[string]any{},
	}

	results2 := matcher.Match(msg2)
	require.Len(t, results2, 1)
	assert.Equal(t, "b2", results2[0].Binding.ID)

	// Message from unknown tenant
	msg3 := &kafka.ReceivedMessage{
		Headers: kafka.MessageHeaders{TenantID: "tenant-3"},
		Data:    map[string]any{},
	}

	results3 := matcher.Match(msg3)
	assert.Len(t, results3, 0)
}

func TestMatcherMatchByKeys(t *testing.T) {
	matcher := NewMatcher()

	matcher.LoadBindings("tenant-1", []*models.Binding{
		{
			ID:        "b1",
			TenantID:  "tenant-1",
			MappingID: "m1",
			IsEnabled: true,
			Filter: models.BindingFilter{
				Keys: []string{"contacts-plan", "users-plan"},
			},
		},
		{
			ID:        "b2",
			TenantID:  "tenant-1",
			MappingID: "m2",
			IsEnabled: true,
			// No plan filter - matches all
		},
	})

	// Message with matching plan
	msg := &kafka.ReceivedMessage{
		Headers: kafka.MessageHeaders{TenantID: "tenant-1"},
		OrchidMessage: &kafka.OrchidMessage{
			TenantID: "tenant-1",
			PlanKey:  "contacts-plan",
		},
		Data: map[string]any{"plan_key": "contacts-plan"},
	}

	results := matcher.Match(msg)
	require.Len(t, results, 2) // Both bindings match

	// Find the plan-specific binding (should have higher score)
	var planBinding *MatchResult
	var anyBinding *MatchResult
	for _, r := range results {
		if r.Binding.ID == "b1" {
			planBinding = r
		} else {
			anyBinding = r
		}
	}

	require.NotNil(t, planBinding)
	require.NotNil(t, anyBinding)
	assert.Greater(t, planBinding.Score, anyBinding.Score)

	// Message with non-matching plan
	msg2 := &kafka.ReceivedMessage{
		Headers: kafka.MessageHeaders{TenantID: "tenant-1"},
		OrchidMessage: &kafka.OrchidMessage{
			TenantID: "tenant-1",
			PlanKey:  "plan-other",
		},
		Data: map[string]any{"plan_key": "plan-other"},
	}

	results2 := matcher.Match(msg2)
	require.Len(t, results2, 1) // Only b2 matches
	assert.Equal(t, "b2", results2[0].Binding.ID)
}

func TestMatcherMatchByStatusCode(t *testing.T) {
	matcher := NewMatcher()

	matcher.LoadBindings("tenant-1", []*models.Binding{
		{
			ID:        "success",
			TenantID:  "tenant-1",
			MappingID: "m1",
			IsEnabled: true,
			Filter: models.BindingFilter{
				StatusCodes: []int{200, 201},
			},
		},
		{
			ID:        "errors",
			TenantID:  "tenant-1",
			MappingID: "m2",
			IsEnabled: true,
			Filter: models.BindingFilter{
				MinStatusCode: 400,
				MaxStatusCode: 599,
			},
		},
	})

	// Success message
	msg200 := &kafka.ReceivedMessage{
		Headers: kafka.MessageHeaders{TenantID: "tenant-1"},
		OrchidMessage: &kafka.OrchidMessage{
			TenantID:   "tenant-1",
			StatusCode: 200,
		},
		Data: map[string]any{"status_code": float64(200)},
	}

	results := matcher.Match(msg200)
	require.Len(t, results, 1)
	assert.Equal(t, "success", results[0].Binding.ID)

	// Error message
	msg500 := &kafka.ReceivedMessage{
		Headers: kafka.MessageHeaders{TenantID: "tenant-1"},
		OrchidMessage: &kafka.OrchidMessage{
			TenantID:   "tenant-1",
			StatusCode: 500,
		},
		Data: map[string]any{"status_code": float64(500)},
	}

	results2 := matcher.Match(msg500)
	require.Len(t, results2, 1)
	assert.Equal(t, "errors", results2[0].Binding.ID)
}

func TestMatcherMatchByStepPath(t *testing.T) {
	matcher := NewMatcher()

	matcher.LoadBindings("tenant-1", []*models.Binding{
		{
			ID:        "root-only",
			TenantID:  "tenant-1",
			MappingID: "m1",
			IsEnabled: true,
			Filter: models.BindingFilter{
				StepPathPrefix: "root",
			},
		},
		{
			ID:        "details",
			TenantID:  "tenant-1",
			MappingID: "m2",
			IsEnabled: true,
			Filter: models.BindingFilter{
				StepPathPrefix: "root.user_details",
			},
		},
	})

	// Root message
	msgRoot := &kafka.ReceivedMessage{
		Headers: kafka.MessageHeaders{TenantID: "tenant-1"},
		OrchidMessage: &kafka.OrchidMessage{
			TenantID: "tenant-1",
			StepPath: "root",
		},
		Data: map[string]any{"step_path": "root"},
	}

	results := matcher.Match(msgRoot)
	require.Len(t, results, 1)
	assert.Equal(t, "root-only", results[0].Binding.ID)

	// Details message
	msgDetails := &kafka.ReceivedMessage{
		Headers: kafka.MessageHeaders{TenantID: "tenant-1"},
		OrchidMessage: &kafka.OrchidMessage{
			TenantID: "tenant-1",
			StepPath: "root.user_details[0]",
		},
		Data: map[string]any{"step_path": "root.user_details[0]"},
	}

	results2 := matcher.Match(msgDetails)
	require.Len(t, results2, 2) // Both match
}

func TestMatcherMatchFirst(t *testing.T) {
	matcher := NewMatcher()

	matcher.LoadBindings("tenant-1", []*models.Binding{
		{
			ID:        "specific",
			TenantID:  "tenant-1",
			MappingID: "m1",
			IsEnabled: true,
			Filter: models.BindingFilter{
				Keys: []string{"contacts-plan"},
			},
		},
		{
			ID:        "general",
			TenantID:  "tenant-1",
			MappingID: "m2",
			IsEnabled: true,
		},
	})

	msg := &kafka.ReceivedMessage{
		Headers: kafka.MessageHeaders{TenantID: "tenant-1"},
		OrchidMessage: &kafka.OrchidMessage{
			TenantID: "tenant-1",
			PlanKey:  "plan-1",
		},
		Data: map[string]any{"plan_key": "plan-1"},
	}

	binding := matcher.MatchFirst(msg)
	require.NotNil(t, binding)
	assert.Equal(t, "specific", binding.ID) // More specific match wins
}
