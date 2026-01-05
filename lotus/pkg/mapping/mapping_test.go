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

func TestSimpleMapping(t *testing.T) {
	sourceFields := fields.Fields{
		{
			ID:   "source_name",
			Name: "Source Name",
			Path: "name",
			Type: models.ValueTypeString,
		},
		{
			ID:   "source_count",
			Name: "Source Count",
			Path: "count",
			Type: models.ValueTypeNumber,
		},
	}

	// Note: The mapping engine uses FieldID as the output key, so ID and desired output key should match
	targetFields := fields.Fields{
		{
			ID:   "output_name",
			Name: "Target Name",
			Path: "output_name",
			Type: models.ValueTypeString,
		},
		{
			ID:   "output_count",
			Name: "Target Count",
			Path: "output_count",
			Type: models.ValueTypeNumber,
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "source_name"}, Target: links.LinkDirection{FieldID: "output_name"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "source_count"}, Target: links.LinkDirection{FieldID: "output_count"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "test-simple"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	sourceData := map[string]any{
		"name":  "Test Value",
		"count": 42,
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "Test Value", result.TargetRaw["output_name"])
	assert.Equal(t, 42, result.TargetRaw["output_count"])
}

func TestMappingWithTransform(t *testing.T) {
	sourceFields := fields.Fields{
		{
			ID:   "source_text",
			Name: "Source Text",
			Path: "text",
			Type: models.ValueTypeString,
		},
	}

	targetFields := fields.Fields{
		{
			ID:   "output",
			Name: "Target Text",
			Path: "output",
			Type: models.ValueTypeString,
		},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "to_upper_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "text_to_upper",
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "source_text"}, Target: links.LinkDirection{StepID: "to_upper_step"}},
		{Priority: 1, Source: links.LinkDirection{StepID: "to_upper_step"}, Target: links.LinkDirection{FieldID: "output"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "test-transform"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	sourceData := map[string]any{
		"text": "hello world",
	}

	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "HELLO WORLD", result.TargetRaw["output"])
}

func TestOrchidMessageMapping(t *testing.T) {
	// Simulate an Orchid API response message
	orchidMessage := map[string]any{
		"tenant_id":    "550e8400-e29b-41d4-a716-446655440000",
		"plan_key":     "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		"execution_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		"status_code":  200,
		"response_body": map[string]any{
			"users": []any{
				map[string]any{"id": 1, "name": "Alice", "email": "alice@example.com"},
				map[string]any{"id": 2, "name": "Bob", "email": "bob@example.com"},
			},
			"total": 2,
			"page":  1,
		},
		"duration_ms": 150,
	}

	// Define mapping for the response_body
	sourceFields := fields.Fields{
		{
			ID:   "total_records",
			Name: "Total Records",
			Path: "response_body.total",
			Type: models.ValueTypeNumber,
		},
		{
			ID:   "page_number",
			Name: "Page Number",
			Path: "response_body.page",
			Type: models.ValueTypeNumber,
		},
		{
			ID:   "duration",
			Name: "Duration",
			Path: "duration_ms",
			Type: models.ValueTypeNumber,
		},
	}

	targetFields := fields.Fields{
		{
			ID:   "record_count",
			Name: "Record Count",
			Path: "record_count",
			Type: models.ValueTypeNumber,
		},
		{
			ID:   "current_page",
			Name: "Current Page",
			Path: "current_page",
			Type: models.ValueTypeNumber,
		},
		{
			ID:   "response_time_ms",
			Name: "Response Time",
			Path: "response_time_ms",
			Type: models.ValueTypeNumber,
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "total_records"}, Target: links.LinkDirection{FieldID: "record_count"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "page_number"}, Target: links.LinkDirection{FieldID: "current_page"}},
		{Priority: 2, Source: links.LinkDirection{FieldID: "duration"}, Target: links.LinkDirection{FieldID: "response_time_ms"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "orchid-test"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	result, err := mapping.ExecuteMapping(orchidMessage)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 2, result.TargetRaw["record_count"])
	assert.Equal(t, 1, result.TargetRaw["current_page"])
	assert.Equal(t, 150, result.TargetRaw["response_time_ms"])
}

func TestHighThroughput(t *testing.T) {
	// Create a simple mapping - IDs match expected output keys
	sourceFields := fields.Fields{
		{ID: "f1", Name: "Field 1", Path: "field1", Type: models.ValueTypeString},
		{ID: "f2", Name: "Field 2", Path: "field2", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "out1", Name: "Target 1", Path: "out1", Type: models.ValueTypeString},
		{ID: "out2", Name: "Target 2", Path: "out2", Type: models.ValueTypeNumber},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "f1"}, Target: links.LinkDirection{FieldID: "out1"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "f2"}, Target: links.LinkDirection{FieldID: "out2"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "throughput-test"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	sourceData := map[string]any{
		"field1": "test",
		"field2": 123,
	}

	// Run 10,000 mappings
	iterations := 10000
	for i := 0; i < iterations; i++ {
		result, err := mapping.ExecuteMapping(sourceData)
		require.NoError(t, err)
		require.NotNil(t, result)
	}

	t.Logf("Successfully executed %d mappings", iterations)
}

func TestCompileMethod(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "source_name", Name: "Source Name", Path: "name", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "output_name", Name: "Target Name", Path: "output_name", Type: models.ValueTypeString},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "source_name"}, Target: links.LinkDirection{FieldID: "output_name"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "test-compile"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	// Initially not compiled
	assert.False(t, mapping.IsCompiled())

	// Compile should succeed
	err := mapping.Compile()
	require.NoError(t, err)
	assert.True(t, mapping.IsCompiled())

	// Compile again should be a no-op (already compiled)
	err = mapping.Compile()
	require.NoError(t, err)

	// Execute mapping should work with compiled mapping
	sourceData := map[string]any{"name": "Test"}
	result, err := mapping.ExecuteMapping(sourceData)
	require.NoError(t, err)
	assert.Equal(t, "Test", result.TargetRaw["output_name"])
}

func TestPreCompiledHighThroughput(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "f1", Name: "Field 1", Path: "field1", Type: models.ValueTypeString},
		{ID: "f2", Name: "Field 2", Path: "field2", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "out1", Name: "Target 1", Path: "out1", Type: models.ValueTypeString},
		{ID: "out2", Name: "Target 2", Path: "out2", Type: models.ValueTypeNumber},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "f1"}, Target: links.LinkDirection{FieldID: "out1"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "f2"}, Target: links.LinkDirection{FieldID: "out2"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "throughput-test"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	// Pre-compile for performance
	err := mapping.Compile()
	require.NoError(t, err)

	sourceData := map[string]any{
		"field1": "test",
		"field2": 123,
	}

	// Run 10,000 mappings with pre-compiled mapping
	iterations := 10000
	for i := 0; i < iterations; i++ {
		result, err := mapping.ExecuteMapping(sourceData)
		require.NoError(t, err)
		require.NotNil(t, result)
	}

	t.Logf("Successfully executed %d pre-compiled mappings", iterations)
}

func TestPooledExecution(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "source_name", Name: "Source Name", Path: "name", Type: models.ValueTypeString},
		{ID: "source_count", Name: "Source Count", Path: "count", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "output_name", Name: "Target Name", Path: "output_name", Type: models.ValueTypeString},
		{ID: "output_count", Name: "Target Count", Path: "output_count", Type: models.ValueTypeNumber},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "source_name"}, Target: links.LinkDirection{FieldID: "output_name"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "source_count"}, Target: links.LinkDirection{FieldID: "output_count"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "test-pooled"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	// Pre-compile
	err := mapping.Compile()
	require.NoError(t, err)

	sourceData := map[string]any{
		"name":  "Test Value",
		"count": 42,
	}

	// Execute with pooling
	result, err := mapping.ExecuteMappingPooled(sourceData)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "Test Value", result.TargetRaw["output_name"])
	assert.Equal(t, 42, result.TargetRaw["output_count"])

	// Release back to pool
	ReleaseMapping(result)
}

func TestPooledHighThroughput(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "f1", Name: "Field 1", Path: "field1", Type: models.ValueTypeString},
		{ID: "f2", Name: "Field 2", Path: "field2", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "out1", Name: "Target 1", Path: "out1", Type: models.ValueTypeString},
		{ID: "out2", Name: "Target 2", Path: "out2", Type: models.ValueTypeNumber},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "f1"}, Target: links.LinkDirection{FieldID: "out1"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "f2"}, Target: links.LinkDirection{FieldID: "out2"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "throughput-test"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	// Pre-compile for performance
	err := mapping.Compile()
	require.NoError(t, err)

	sourceData := map[string]any{
		"field1": "test",
		"field2": 123,
	}

	// Run 10,000 mappings with pooling
	iterations := 10000
	for i := 0; i < iterations; i++ {
		result, err := mapping.ExecuteMappingPooled(sourceData)
		require.NoError(t, err)
		require.NotNil(t, result)
		ReleaseMapping(result)
	}

	t.Logf("Successfully executed %d pooled mappings", iterations)
}
