package mapping

import (
	"testing"

	"github.com/Ramsey-B/lotus/pkg/actions"
	"github.com/Ramsey-B/lotus/pkg/actions/registry"
	"github.com/Ramsey-B/lotus/pkg/fields"
	"github.com/Ramsey-B/lotus/pkg/links"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Register all actions for tests
	for _, action := range actions.ActionDefinitions {
		registry.Actions[action.Key] = action.Factory
	}
}

// =============================================================================
// CHAINED TRANSFORMATION TESTS
// =============================================================================

func TestChainedTextTransformations(t *testing.T) {
	// Test: source text -> trim -> to_upper -> concat with suffix
	sourceFields := fields.Fields{
		{ID: "raw_name", Name: "Raw Name", Path: "name", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "formatted_name", Name: "Formatted Name", Path: "formatted_name", Type: models.ValueTypeString},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "trim_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "text_trim",
			},
		},
		{
			ID:   "upper_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "text_to_upper",
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "raw_name"}, Target: links.LinkDirection{StepID: "trim_step"}},
		{Priority: 1, Source: links.LinkDirection{StepID: "trim_step"}, Target: links.LinkDirection{StepID: "upper_step"}},
		{Priority: 2, Source: links.LinkDirection{StepID: "upper_step"}, Target: links.LinkDirection{FieldID: "formatted_name"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "chained-text"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"name": "  hello world  ",
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "HELLO WORLD", result.TargetRaw["formatted_name"])
}

func TestChainedNumberTransformations(t *testing.T) {
	// Test: two numbers -> add -> multiply by 2 -> to_string
	sourceFields := fields.Fields{
		{ID: "num1", Name: "Number 1", Path: "a", Type: models.ValueTypeNumber},
		{ID: "num2", Name: "Number 2", Path: "b", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "result_str", Name: "Result String", Path: "result", Type: models.ValueTypeString},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "add_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "number_add",
			},
		},
		{
			ID:   "to_string_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "number_to_string",
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "num1"}, Target: links.LinkDirection{StepID: "add_step"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "num2"}, Target: links.LinkDirection{StepID: "add_step"}},
		{Priority: 2, Source: links.LinkDirection{StepID: "add_step"}, Target: links.LinkDirection{StepID: "to_string_step"}},
		{Priority: 3, Source: links.LinkDirection{StepID: "to_string_step"}, Target: links.LinkDirection{FieldID: "result_str"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "chained-number"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"a": 10,
		"b": 5,
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "15", result.TargetRaw["result"])
}

// =============================================================================
// NESTED OBJECT ACCESS TESTS
// =============================================================================

func TestDeeplyNestedObjectAccess(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "user_name", Name: "User Name", Path: "data.user.profile.name", Type: models.ValueTypeString},
		{ID: "user_email", Name: "User Email", Path: "data.user.profile.contact.email", Type: models.ValueTypeString},
		{ID: "company_name", Name: "Company Name", Path: "data.user.employment.company.name", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "name", Name: "Name", Path: "name", Type: models.ValueTypeString},
		{ID: "email", Name: "Email", Path: "email", Type: models.ValueTypeString},
		{ID: "company", Name: "Company", Path: "company", Type: models.ValueTypeString},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "user_name"}, Target: links.LinkDirection{FieldID: "name"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "user_email"}, Target: links.LinkDirection{FieldID: "email"}},
		{Priority: 2, Source: links.LinkDirection{FieldID: "company_name"}, Target: links.LinkDirection{FieldID: "company"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "nested-access"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	sourceData := map[string]any{
		"data": map[string]any{
			"user": map[string]any{
				"profile": map[string]any{
					"name": "John Doe",
					"contact": map[string]any{
						"email": "john@example.com",
						"phone": "555-1234",
					},
				},
				"employment": map[string]any{
					"company": map[string]any{
						"name":     "Acme Corp",
						"industry": "Technology",
					},
					"title": "Engineer",
				},
			},
		},
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "John Doe", result.TargetRaw["name"])
	assert.Equal(t, "john@example.com", result.TargetRaw["email"])
	assert.Equal(t, "Acme Corp", result.TargetRaw["company"])
}

// =============================================================================
// ARRAY TRANSFORMATION TESTS
// =============================================================================

func TestArrayPushTransformation(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "existing_tags", Name: "Existing Tags", Path: "tags", Type: models.ValueTypeArray},
		{ID: "new_tag", Name: "New Tag", Path: "new_tag", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "all_tags", Name: "All Tags", Path: "all_tags", Type: models.ValueTypeArray},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "push_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "array_push",
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "existing_tags"}, Target: links.LinkDirection{StepID: "push_step"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "new_tag"}, Target: links.LinkDirection{StepID: "push_step"}},
		{Priority: 2, Source: links.LinkDirection{StepID: "push_step"}, Target: links.LinkDirection{FieldID: "all_tags"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "array-push"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"tags":    []any{"go", "kafka"},
		"new_tag": "postgres",
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	allTags, ok := result.TargetRaw["all_tags"].([]any)
	require.True(t, ok)
	assert.Len(t, allTags, 3)
	assert.Contains(t, allTags, "go")
	assert.Contains(t, allTags, "kafka")
	assert.Contains(t, allTags, "postgres")
}

func TestArrayLengthTransformation(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "items", Name: "Items", Path: "items", Type: models.ValueTypeArray},
	}

	targetFields := fields.Fields{
		{ID: "count", Name: "Count", Path: "count", Type: models.ValueTypeNumber},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "length_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "array_length",
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "items"}, Target: links.LinkDirection{StepID: "length_step"}},
		{Priority: 1, Source: links.LinkDirection{StepID: "length_step"}, Target: links.LinkDirection{FieldID: "count"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "array-length"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"items": []any{"a", "b", "c", "d", "e"},
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 5, result.TargetRaw["count"])
}

func TestArrayReverseAndDistinct(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "numbers", Name: "Numbers", Path: "numbers", Type: models.ValueTypeArray},
	}

	targetFields := fields.Fields{
		{ID: "unique_reversed", Name: "Unique Reversed", Path: "unique_reversed", Type: models.ValueTypeArray},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "distinct_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "array_distinct",
			},
		},
		{
			ID:   "reverse_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "array_reverse",
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "numbers"}, Target: links.LinkDirection{StepID: "distinct_step"}},
		{Priority: 1, Source: links.LinkDirection{StepID: "distinct_step"}, Target: links.LinkDirection{StepID: "reverse_step"}},
		{Priority: 2, Source: links.LinkDirection{StepID: "reverse_step"}, Target: links.LinkDirection{FieldID: "unique_reversed"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "array-chain"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"numbers": []any{1, 2, 2, 3, 3, 3, 4},
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	uniqueReversed, ok := result.TargetRaw["unique_reversed"].([]any)
	require.True(t, ok)
	assert.Len(t, uniqueReversed, 4) // [1, 2, 3, 4] reversed = [4, 3, 2, 1]
}

// =============================================================================
// CONDITIONAL LOGIC TESTS
// =============================================================================

func TestCoalesceTransformation(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "preferred_name", Name: "Preferred Name", Path: "preferred_name", Type: models.ValueTypeString},
		{ID: "first_name", Name: "First Name", Path: "first_name", Type: models.ValueTypeString},
		{ID: "username", Name: "Username", Path: "username", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "display_name", Name: "Display Name", Path: "display_name", Type: models.ValueTypeString},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "coalesce_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "any_coalesce",
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "preferred_name"}, Target: links.LinkDirection{StepID: "coalesce_step"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "first_name"}, Target: links.LinkDirection{StepID: "coalesce_step"}},
		{Priority: 2, Source: links.LinkDirection{FieldID: "username"}, Target: links.LinkDirection{StepID: "coalesce_step"}},
		{Priority: 3, Source: links.LinkDirection{StepID: "coalesce_step"}, Target: links.LinkDirection{FieldID: "display_name"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "coalesce"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	// Test 1: first value is nil, second exists
	sourceData1 := map[string]any{
		"preferred_name": nil,
		"first_name":     "John",
		"username":       "johnd",
	}

	result1, err := mapping.ExecuteMapping(sourceData1)
	require.NoError(t, err)
	assert.Equal(t, "John", result1.TargetRaw["display_name"])

	// Test 2: all values nil except last
	sourceData2 := map[string]any{
		"preferred_name": nil,
		"first_name":     nil,
		"username":       "fallback_user",
	}

	result2, err := mapping.ExecuteMapping(sourceData2)
	require.NoError(t, err)
	assert.Equal(t, "fallback_user", result2.TargetRaw["display_name"])
}

// =============================================================================
// MATH OPERATION TESTS
// =============================================================================

func TestComplexMathOperations(t *testing.T) {
	// Calculate: (a + b) * c
	sourceFields := fields.Fields{
		{ID: "a", Name: "A", Path: "a", Type: models.ValueTypeNumber},
		{ID: "b", Name: "B", Path: "b", Type: models.ValueTypeNumber},
		{ID: "c", Name: "C", Path: "c", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "result", Name: "Result", Path: "result", Type: models.ValueTypeNumber},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "add_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "number_add",
			},
		},
		{
			ID:   "multiply_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "number_multiply",
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "a"}, Target: links.LinkDirection{StepID: "add_step"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "b"}, Target: links.LinkDirection{StepID: "add_step"}},
		{Priority: 2, Source: links.LinkDirection{StepID: "add_step"}, Target: links.LinkDirection{StepID: "multiply_step"}},
		{Priority: 3, Source: links.LinkDirection{FieldID: "c"}, Target: links.LinkDirection{StepID: "multiply_step"}},
		{Priority: 4, Source: links.LinkDirection{StepID: "multiply_step"}, Target: links.LinkDirection{FieldID: "result"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "complex-math"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"a": 3,
		"b": 2,
		"c": 4,
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	// (3 + 2) * 4 = 20
	assert.Equal(t, float64(20), result.TargetRaw["result"])
}

func TestNumberClampAndRound(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "value", Name: "Value", Path: "value", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "clamped", Name: "Clamped", Path: "clamped", Type: models.ValueTypeNumber},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "round_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "number_round",
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "value"}, Target: links.LinkDirection{StepID: "round_step"}},
		{Priority: 1, Source: links.LinkDirection{StepID: "round_step"}, Target: links.LinkDirection{FieldID: "clamped"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "clamp-round"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"value": 3.7,
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, float64(4), result.TargetRaw["clamped"])
}

// =============================================================================
// TEXT TRANSFORMATION TESTS
// =============================================================================

func TestTextConcatWithSeparator(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "first_name", Name: "First Name", Path: "first_name", Type: models.ValueTypeString},
		{ID: "last_name", Name: "Last Name", Path: "last_name", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "full_name", Name: "Full Name", Path: "full_name", Type: models.ValueTypeString},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "concat_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "text_concat",
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "first_name"}, Target: links.LinkDirection{StepID: "concat_step"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "last_name"}, Target: links.LinkDirection{StepID: "concat_step"}},
		{Priority: 2, Source: links.LinkDirection{StepID: "concat_step"}, Target: links.LinkDirection{FieldID: "full_name"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "text-concat"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"first_name": "John",
		"last_name":  "Doe",
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "JohnDoe", result.TargetRaw["full_name"])
}

func TestTextSplitAndLength(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "csv_data", Name: "CSV Data", Path: "csv", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "parts", Name: "Parts", Path: "parts", Type: models.ValueTypeArray},
		{ID: "count", Name: "Count", Path: "count", Type: models.ValueTypeNumber},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "split_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key:       "text_split",
				Arguments: map[string]any{"separator": ","},
			},
		},
		{
			ID:   "length_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "array_length",
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "csv_data"}, Target: links.LinkDirection{StepID: "split_step"}},
		{Priority: 1, Source: links.LinkDirection{StepID: "split_step"}, Target: links.LinkDirection{FieldID: "parts"}},
		{Priority: 2, Source: links.LinkDirection{StepID: "split_step"}, Target: links.LinkDirection{StepID: "length_step"}},
		{Priority: 3, Source: links.LinkDirection{StepID: "length_step"}, Target: links.LinkDirection{FieldID: "count"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "text-split"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"csv": "apple,banana,cherry,date",
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	parts, ok := result.TargetRaw["parts"].([]any)
	require.True(t, ok)
	assert.Len(t, parts, 4)
	assert.Equal(t, 4, result.TargetRaw["count"])
}

// =============================================================================
// REAL-WORLD SCENARIO TESTS
// =============================================================================

func TestAPIResponseToUserDTO(t *testing.T) {
	// Simulate transforming an API response to a standardized user DTO
	sourceFields := fields.Fields{
		{ID: "user_id", Name: "User ID", Path: "response_body.data.id", Type: models.ValueTypeNumber},
		{ID: "user_email", Name: "User Email", Path: "response_body.data.email", Type: models.ValueTypeString},
		{ID: "user_first", Name: "First Name", Path: "response_body.data.first_name", Type: models.ValueTypeString},
		{ID: "user_last", Name: "Last Name", Path: "response_body.data.last_name", Type: models.ValueTypeString},
		{ID: "created_at", Name: "Created At", Path: "response_body.data.created_at", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "id", Name: "ID", Path: "id", Type: models.ValueTypeString},
		{ID: "email", Name: "Email", Path: "email", Type: models.ValueTypeString},
		{ID: "display_name", Name: "Display Name", Path: "display_name", Type: models.ValueTypeString},
		{ID: "created_date", Name: "Created Date", Path: "created_date", Type: models.ValueTypeString},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "id_to_string",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "number_to_string",
			},
		},
		{
			ID:   "concat_name",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "text_concat",
			},
		},
		{
			ID:   "email_lower",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "text_to_lower",
			},
		},
	}

	linkList := links.Links{
		// ID: number -> string
		{Priority: 0, Source: links.LinkDirection{FieldID: "user_id"}, Target: links.LinkDirection{StepID: "id_to_string"}},
		{Priority: 1, Source: links.LinkDirection{StepID: "id_to_string"}, Target: links.LinkDirection{FieldID: "id"}},
		// Email: to lowercase
		{Priority: 2, Source: links.LinkDirection{FieldID: "user_email"}, Target: links.LinkDirection{StepID: "email_lower"}},
		{Priority: 3, Source: links.LinkDirection{StepID: "email_lower"}, Target: links.LinkDirection{FieldID: "email"}},
		// Display Name: first + last
		{Priority: 4, Source: links.LinkDirection{FieldID: "user_first"}, Target: links.LinkDirection{StepID: "concat_name"}},
		{Priority: 5, Source: links.LinkDirection{FieldID: "user_last"}, Target: links.LinkDirection{StepID: "concat_name"}},
		{Priority: 6, Source: links.LinkDirection{StepID: "concat_name"}, Target: links.LinkDirection{FieldID: "display_name"}},
		// Created date: direct pass-through
		{Priority: 7, Source: links.LinkDirection{FieldID: "created_at"}, Target: links.LinkDirection{FieldID: "created_date"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "api-to-dto"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"response_body": map[string]any{
			"data": map[string]any{
				"id":         12345,
				"email":      "John.Doe@Example.COM",
				"first_name": "John",
				"last_name":  "Doe",
				"created_at": "2024-01-15T10:30:00Z",
			},
		},
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "12345", result.TargetRaw["id"])
	assert.Equal(t, "john.doe@example.com", result.TargetRaw["email"])
	assert.Equal(t, "JohnDoe", result.TargetRaw["display_name"])
	assert.Equal(t, "2024-01-15T10:30:00Z", result.TargetRaw["created_date"])
}

func TestOrderCalculation(t *testing.T) {
	// Calculate order totals: subtotal * quantity -> total
	sourceFields := fields.Fields{
		{ID: "price", Name: "Price", Path: "item.price", Type: models.ValueTypeNumber},
		{ID: "quantity", Name: "Quantity", Path: "item.quantity", Type: models.ValueTypeNumber},
		{ID: "tax_rate", Name: "Tax Rate", Path: "tax_rate", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "subtotal", Name: "Subtotal", Path: "subtotal", Type: models.ValueTypeNumber},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "calc_subtotal",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "number_multiply",
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "price"}, Target: links.LinkDirection{StepID: "calc_subtotal"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "quantity"}, Target: links.LinkDirection{StepID: "calc_subtotal"}},
		{Priority: 2, Source: links.LinkDirection{StepID: "calc_subtotal"}, Target: links.LinkDirection{FieldID: "subtotal"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "order-calc"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"item": map[string]any{
			"price":    29.99,
			"quantity": 3,
		},
		"tax_rate": 0.08,
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	// 29.99 * 3 = 89.97
	subtotal, ok := result.TargetRaw["subtotal"].(float64)
	require.True(t, ok)
	assert.InDelta(t, 89.97, subtotal, 0.01)
}

func TestMultiOutputMapping(t *testing.T) {
	// Same source field going to multiple targets with different transformations
	sourceFields := fields.Fields{
		{ID: "email", Name: "Email", Path: "email", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "email_original", Name: "Original Email", Path: "email_original", Type: models.ValueTypeString},
		{ID: "email_lower", Name: "Lowercase Email", Path: "email_lower", Type: models.ValueTypeString},
		{ID: "email_upper", Name: "Uppercase Email", Path: "email_upper", Type: models.ValueTypeString},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "to_lower",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "text_to_lower",
			},
		},
		{
			ID:   "to_upper",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "text_to_upper",
			},
		},
	}

	linkList := links.Links{
		// Original (pass-through)
		{Priority: 0, Source: links.LinkDirection{FieldID: "email"}, Target: links.LinkDirection{FieldID: "email_original"}},
		// Lowercase
		{Priority: 1, Source: links.LinkDirection{FieldID: "email"}, Target: links.LinkDirection{StepID: "to_lower"}},
		{Priority: 2, Source: links.LinkDirection{StepID: "to_lower"}, Target: links.LinkDirection{FieldID: "email_lower"}},
		// Uppercase
		{Priority: 3, Source: links.LinkDirection{FieldID: "email"}, Target: links.LinkDirection{StepID: "to_upper"}},
		{Priority: 4, Source: links.LinkDirection{StepID: "to_upper"}, Target: links.LinkDirection{FieldID: "email_upper"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "multi-output"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"email": "Test@Example.com",
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "Test@Example.com", result.TargetRaw["email_original"])
	assert.Equal(t, "test@example.com", result.TargetRaw["email_lower"])
	assert.Equal(t, "TEST@EXAMPLE.COM", result.TargetRaw["email_upper"])
}

// =============================================================================
// OBJECT TRANSFORMATION TESTS
// =============================================================================

func TestObjectPickFields(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "full_user", Name: "Full User", Path: "user", Type: models.ValueTypeObject},
	}

	targetFields := fields.Fields{
		{ID: "user_subset", Name: "User Subset", Path: "user_subset", Type: models.ValueTypeObject},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "pick_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key:       "object_pick",
				Arguments: map[string]any{"keys": []any{"id", "name"}},
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "full_user"}, Target: links.LinkDirection{StepID: "pick_step"}},
		{Priority: 1, Source: links.LinkDirection{StepID: "pick_step"}, Target: links.LinkDirection{FieldID: "user_subset"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "object-pick"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"user": map[string]any{
			"id":       123,
			"name":     "John",
			"email":    "john@example.com",
			"password": "secret",
			"role":     "admin",
		},
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	subset, ok := result.TargetRaw["user_subset"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 123, subset["id"])
	assert.Equal(t, "John", subset["name"])
	assert.NotContains(t, subset, "password")
	assert.NotContains(t, subset, "email")
}

func TestObjectMerge(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "base_config", Name: "Base Config", Path: "base", Type: models.ValueTypeObject},
		{ID: "overrides", Name: "Overrides", Path: "overrides", Type: models.ValueTypeObject},
	}

	targetFields := fields.Fields{
		{ID: "merged_config", Name: "Merged Config", Path: "config", Type: models.ValueTypeObject},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "merge_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "object_merge",
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "base_config"}, Target: links.LinkDirection{StepID: "merge_step"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "overrides"}, Target: links.LinkDirection{StepID: "merge_step"}},
		{Priority: 2, Source: links.LinkDirection{StepID: "merge_step"}, Target: links.LinkDirection{FieldID: "merged_config"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "object-merge"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"base": map[string]any{
			"timeout":  30,
			"retries":  3,
			"debug":    false,
			"endpoint": "https://api.example.com",
		},
		"overrides": map[string]any{
			"debug":   true,
			"retries": 5,
		},
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	config, ok := result.TargetRaw["config"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 30, config["timeout"])
	assert.Equal(t, 5, config["retries"])     // Overridden
	assert.Equal(t, true, config["debug"])    // Overridden
	assert.Equal(t, "https://api.example.com", config["endpoint"])
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

func TestMissingSourceField(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "name", Name: "Name", Path: "name", Type: models.ValueTypeString},
		{ID: "optional_field", Name: "Optional", Path: "optional", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "output_name", Name: "Output Name", Path: "output_name", Type: models.ValueTypeString},
		{ID: "output_optional", Name: "Output Optional", Path: "output_optional", Type: models.ValueTypeString},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "name"}, Target: links.LinkDirection{FieldID: "output_name"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "optional_field"}, Target: links.LinkDirection{FieldID: "output_optional"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "missing-field"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	// Source data missing "optional" field
	sourceData := map[string]any{
		"name": "Test",
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "Test", result.TargetRaw["output_name"])
	// Missing field should be empty/nil
	assert.Equal(t, "", result.TargetRaw["output_optional"])
}

func TestEmptySourceData(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "name", Name: "Name", Path: "name", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "output", Name: "Output", Path: "output", Type: models.ValueTypeString},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "name"}, Target: links.LinkDirection{FieldID: "output"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "empty-source"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	sourceData := map[string]any{}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestSpecialCharactersInData(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "text", Name: "Text", Path: "text", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "output", Name: "Output", Path: "output", Type: models.ValueTypeString},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "text"}, Target: links.LinkDirection{FieldID: "output"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "special-chars"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	specialChars := "Hello! @#$%^&*() 日本語 émojis: 🎉🚀"
	sourceData := map[string]any{
		"text": specialChars,
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, specialChars, result.TargetRaw["output"])
}

func TestLargeNumberHandling(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "big_num", Name: "Big Number", Path: "big", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "output", Name: "Output", Path: "output", Type: models.ValueTypeNumber},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "big_num"}, Target: links.LinkDirection{FieldID: "output"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "big-number"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	largeNum := float64(9999999999999999)
	sourceData := map[string]any{
		"big": largeNum,
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, largeNum, result.TargetRaw["output"])
}

// =============================================================================
// PARALLEL TRANSFORM TESTS
// =============================================================================

func TestParallelTransformations(t *testing.T) {
	// Test two parallel transformation chains
	sourceFields := fields.Fields{
		{ID: "name", Name: "Name", Path: "name", Type: models.ValueTypeString},
		{ID: "email", Name: "Email", Path: "email", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "upper_name", Name: "Upper Name", Path: "upper_name", Type: models.ValueTypeString},
		{ID: "lower_email", Name: "Lower Email", Path: "lower_email", Type: models.ValueTypeString},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "to_upper",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "text_to_upper",
			},
		},
		{
			ID:   "to_lower",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "text_to_lower",
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "name"}, Target: links.LinkDirection{StepID: "to_upper"}},
		{Priority: 1, Source: links.LinkDirection{StepID: "to_upper"}, Target: links.LinkDirection{FieldID: "upper_name"}},
		{Priority: 2, Source: links.LinkDirection{FieldID: "email"}, Target: links.LinkDirection{StepID: "to_lower"}},
		{Priority: 3, Source: links.LinkDirection{StepID: "to_lower"}, Target: links.LinkDirection{FieldID: "lower_email"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "parallel-transforms"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"name":  "John Doe",
		"email": "JOHN@EXAMPLE.COM",
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "JOHN DOE", result.TargetRaw["upper_name"])
	assert.Equal(t, "john@example.com", result.TargetRaw["lower_email"])
}

func TestComplexMappingHighThroughput(t *testing.T) {
	// Test that compiled mappings with transforms can be reused many times
	sourceFields := fields.Fields{
		{ID: "name", Name: "Name", Path: "name", Type: models.ValueTypeString},
		{ID: "email", Name: "Email", Path: "email", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "upper_name", Name: "Upper Name", Path: "upper_name", Type: models.ValueTypeString},
		{ID: "lower_email", Name: "Lower Email", Path: "lower_email", Type: models.ValueTypeString},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "to_upper",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "text_to_upper",
			},
		},
		{
			ID:   "to_lower",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "text_to_lower",
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "name"}, Target: links.LinkDirection{StepID: "to_upper"}},
		{Priority: 1, Source: links.LinkDirection{StepID: "to_upper"}, Target: links.LinkDirection{FieldID: "upper_name"}},
		{Priority: 2, Source: links.LinkDirection{FieldID: "email"}, Target: links.LinkDirection{StepID: "to_lower"}},
		{Priority: 3, Source: links.LinkDirection{StepID: "to_lower"}, Target: links.LinkDirection{FieldID: "lower_email"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "complex-throughput"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	// Pre-compile for performance
	err := mapping.Compile()
	require.NoError(t, err)

	sourceData := map[string]any{
		"name":  "John Doe",
		"email": "JOHN@EXAMPLE.COM",
	}

	// Run 1000 mappings - this verifies the Step state bug is fixed
	for i := 0; i < 1000; i++ {
		result, err := mapping.ExecuteMapping(sourceData)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, "JOHN DOE", result.TargetRaw["upper_name"])
		assert.Equal(t, "john@example.com", result.TargetRaw["lower_email"])
	}

	t.Logf("Successfully executed 1000 complex mappings with transforms")
}

// TestConditionalBreakWithMultiInput tests that when a conditional step breaks,
// other inputs to a multi-input step are still processed correctly.
// Scenario: number_add receives inputs from two sources, one with a conditional
// that filters odd numbers. When an odd number is passed, only the even number
// should be added.
func TestConditionalBreakWithMultiInput(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "value_a", Name: "Value A", Path: "value_a", Type: models.ValueTypeNumber},
		{ID: "value_b", Name: "Value B", Path: "value_b", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "result", Name: "Result", Path: "result", Type: models.ValueTypeNumber},
	}

	// Steps:
	// - is_even: condition that breaks if input is odd
	// - add: adds numbers from both sources
	stepDefs := []models.StepDefinition{
		{
			ID:   "is_even",
			Type: models.StepTypeCondition,
			Action: models.ActionDefinition{
				Key: "number_is_even",
			},
		},
		{
			ID:   "add",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "number_add",
			},
		},
	}

	// Links:
	// value_a (odd number) -> is_even -> add -> result
	// value_b (any number) ---------> add
	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "value_a"}, Target: links.LinkDirection{StepID: "is_even"}},
		{Priority: 1, Source: links.LinkDirection{StepID: "is_even"}, Target: links.LinkDirection{StepID: "add"}},
		{Priority: 2, Source: links.LinkDirection{FieldID: "value_b"}, Target: links.LinkDirection{StepID: "add"}},
		{Priority: 3, Source: links.LinkDirection{StepID: "add"}, Target: links.LinkDirection{FieldID: "result"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "conditional-multi-input"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	require.NoError(t, mapping.Compile())

	// Test 1: Both values are even - should add them
	t.Run("both even", func(t *testing.T) {
		result, err := mapping.ExecuteMapping(map[string]any{
			"value_a": 4,
			"value_b": 6,
		})
		require.NoError(t, err)
		assert.Equal(t, float64(10), result.TargetRaw["result"])
	})

	// Test 2: value_a is odd - conditional breaks, only value_b should be in result
	t.Run("value_a odd", func(t *testing.T) {
		result, err := mapping.ExecuteMapping(map[string]any{
			"value_a": 5, // odd - will be filtered
			"value_b": 6,
		})
		require.NoError(t, err)
		// Only value_b (6) should make it to the add step
		assert.Equal(t, float64(6), result.TargetRaw["result"])
	})

	// Test 3: Pooled execution - verify state is reset correctly
	t.Run("pooled execution", func(t *testing.T) {
		// First with odd (filtered)
		result1, err := mapping.ExecuteMappingPooled(map[string]any{
			"value_a": 3, // odd
			"value_b": 10,
		})
		require.NoError(t, err)
		assert.Equal(t, float64(10), result1.TargetRaw["result"])
		ReleaseMapping(result1)

		// Then with even (should add)
		result2, err := mapping.ExecuteMappingPooled(map[string]any{
			"value_a": 4, // even
			"value_b": 6,
		})
		require.NoError(t, err)
		assert.Equal(t, float64(10), result2.TargetRaw["result"])
		ReleaseMapping(result2)

		// Then with odd again (verify no state leak)
		result3, err := mapping.ExecuteMappingPooled(map[string]any{
			"value_a": 7, // odd
			"value_b": 2,
		})
		require.NoError(t, err)
		assert.Equal(t, float64(2), result3.TargetRaw["result"])
		ReleaseMapping(result3)
	})
}

// TestArrayItemsToMultiInputStep tests that array items are correctly collected
// and passed to a multi-input step (like number_add to sum all array elements).
func TestArrayItemsToMultiInputStep(t *testing.T) {
	sourceFields := fields.Fields{
		{
			ID:   "numbers",
			Name: "Numbers",
			Path: "numbers",
			Type: models.ValueTypeArray,
			Items: &fields.Field{
				ID:   "number_item",
				Name: "Number Item",
				Path: "", // item path is relative
				Type: models.ValueTypeNumber,
			},
		},
	}

	targetFields := fields.Fields{
		{ID: "sum", Name: "Sum", Path: "sum", Type: models.ValueTypeNumber},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "add_all",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "number_add",
			},
		},
	}

	// Single link from array items to add step
	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "number_item"}, Target: links.LinkDirection{StepID: "add_all"}},
		{Priority: 1, Source: links.LinkDirection{StepID: "add_all"}, Target: links.LinkDirection{FieldID: "sum"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "array-sum"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	require.NoError(t, mapping.Compile())

	t.Run("sum array of numbers", func(t *testing.T) {
		result, err := mapping.ExecuteMapping(map[string]any{
			"numbers": []any{1, 2, 3, 4, 5},
		})
		require.NoError(t, err)
		// 1 + 2 + 3 + 4 + 5 = 15
		assert.Equal(t, float64(15), result.TargetRaw["sum"])
	})

	t.Run("sum single element array", func(t *testing.T) {
		result, err := mapping.ExecuteMapping(map[string]any{
			"numbers": []any{42},
		})
		require.NoError(t, err)
		assert.Equal(t, float64(42), result.TargetRaw["sum"])
	})

	t.Run("sum with floats", func(t *testing.T) {
		result, err := mapping.ExecuteMapping(map[string]any{
			"numbers": []any{1.5, 2.5, 3.0},
		})
		require.NoError(t, err)
		assert.Equal(t, float64(7), result.TargetRaw["sum"])
	})

	t.Run("pooled execution", func(t *testing.T) {
		result, err := mapping.ExecuteMappingPooled(map[string]any{
			"numbers": []any{10, 20, 30},
		})
		require.NoError(t, err)
		assert.Equal(t, float64(60), result.TargetRaw["sum"])
		ReleaseMapping(result)
	})
}

