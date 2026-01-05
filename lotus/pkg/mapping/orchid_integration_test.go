package mapping

import (
	"encoding/json"
	"testing"
	"time"

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

// OrchidMessage represents the exact schema from Orchid's Kafka output
type OrchidMessage struct {
	// Metadata
	TenantID    string    `json:"tenant_id"`
	Integration string    `json:"integration"`
	PlanKey     string    `json:"plan_key"`
	ConfigID    string    `json:"config_id"`
	ExecutionID string    `json:"execution_id"`
	StepPath    string    `json:"step_path"`
	Timestamp   time.Time `json:"timestamp"`

	// Tracing
	TraceID string `json:"trace_id,omitempty"`
	SpanID  string `json:"span_id,omitempty"`

	// Request details
	RequestURL     string            `json:"request_url"`
	RequestMethod  string            `json:"request_method"`
	RequestHeaders map[string]string `json:"request_headers,omitempty"`

	// Response details
	StatusCode      int                    `json:"status_code"`
	ResponseBody    map[string]interface{} `json:"response_body"`
	ResponseHeaders map[string]string      `json:"response_headers,omitempty"`
	ResponseSize    int64                  `json:"response_size"`
	DurationMs      int64                  `json:"duration_ms"`

	// Extracted data (optional)
	ExtractedData map[string]interface{} `json:"extracted_data,omitempty"`
}

// Sample Orchid messages for testing
func createSampleOrchidMessage() map[string]any {
	return map[string]any{
		"tenant_id":      "550e8400-e29b-41d4-a716-446655440000",
		"integration":    "test-integration",
		"plan_key":       "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		"config_id":      "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"execution_id":   "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		"step_path":      "root",
		"timestamp":      "2025-01-15T10:30:00Z",
		"request_url":    "https://api.example.com/users",
		"request_method": "GET",
		"request_headers": map[string]any{
			"Authorization": "[REDACTED]",
			"Content-Type":  "application/json",
		},
		"status_code": 200,
		"response_body": map[string]any{
			"users": []any{
				map[string]any{"id": 1, "name": "Alice", "email": "alice@example.com"},
				map[string]any{"id": 2, "name": "Bob", "email": "bob@example.com"},
			},
			"total": 2,
			"page":  1,
		},
		"response_headers": map[string]any{
			"Content-Type":          "application/json",
			"X-RateLimit-Remaining": "99",
		},
		"response_size": 256,
		"duration_ms":   145,
		"trace_id":      "4bf92f3577b34da6a3ce929d0e0e4736",
		"span_id":       "00f067aa0ba902b7",
	}
}

func createNestedOrchidMessage() map[string]any {
	return map[string]any{
		"tenant_id":      "550e8400-e29b-41d4-a716-446655440000",
		"plan_key":       "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		"config_id":      "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"execution_id":   "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		"step_path":      "root.user_details[0]",
		"timestamp":      "2025-01-15T10:30:01Z",
		"request_url":    "https://api.example.com/users/1/details",
		"request_method": "GET",
		"status_code":    200,
		"response_body": map[string]any{
			"id":         1,
			"name":       "Alice",
			"department": "Engineering",
			"manager":    "Carol",
		},
		"duration_ms": 89,
	}
}

func createErrorOrchidMessage() map[string]any {
	return map[string]any{
		"tenant_id":      "550e8400-e29b-41d4-a716-446655440000",
		"plan_key":       "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		"config_id":      "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"execution_id":   "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		"step_path":      "root",
		"timestamp":      "2025-01-15T10:30:00Z",
		"request_url":    "https://api.example.com/users",
		"request_method": "GET",
		"status_code":    429,
		"response_body": map[string]any{
			"error":   "rate_limit_exceeded",
			"message": "Too many requests",
		},
		"duration_ms": 15,
	}
}

// TestOrchidMetadataExtraction tests extracting Orchid metadata fields
func TestOrchidMetadataExtraction(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "tenant_id", Name: "Tenant ID", Path: "tenant_id", Type: models.ValueTypeString},
		{ID: "plan_key", Name: "Plan Key", Path: "plan_key", Type: models.ValueTypeString},
		{ID: "execution_id", Name: "Execution ID", Path: "execution_id", Type: models.ValueTypeString},
		{ID: "status_code", Name: "Status Code", Path: "status_code", Type: models.ValueTypeNumber},
		{ID: "duration_ms", Name: "Duration", Path: "duration_ms", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "source_tenant", Name: "Source Tenant", Path: "source_tenant", Type: models.ValueTypeString},
		{ID: "source_plan", Name: "Source Plan", Path: "source_plan", Type: models.ValueTypeString},
		{ID: "batch_id", Name: "Batch ID", Path: "batch_id", Type: models.ValueTypeString},
		{ID: "http_status", Name: "HTTP Status", Path: "http_status", Type: models.ValueTypeNumber},
		{ID: "latency_ms", Name: "Latency", Path: "latency_ms", Type: models.ValueTypeNumber},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "tenant_id"}, Target: links.LinkDirection{FieldID: "source_tenant"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "plan_key"}, Target: links.LinkDirection{FieldID: "source_plan"}},
		{Priority: 2, Source: links.LinkDirection{FieldID: "execution_id"}, Target: links.LinkDirection{FieldID: "batch_id"}},
		{Priority: 3, Source: links.LinkDirection{FieldID: "status_code"}, Target: links.LinkDirection{FieldID: "http_status"}},
		{Priority: 4, Source: links.LinkDirection{FieldID: "duration_ms"}, Target: links.LinkDirection{FieldID: "latency_ms"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "orchid-metadata"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	require.NoError(t, mapping.Compile())

	msg := createSampleOrchidMessage()
	result, err := mapping.ExecuteMappingPooled(msg)
	require.NoError(t, err)
	defer ReleaseMapping(result)

	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", result.TargetRaw["source_tenant"])
	assert.Equal(t, "contacts-plan", result.TargetRaw["source_plan"])
	assert.Equal(t, "a1b2c3d4-e5f6-7890-abcd-ef1234567890", result.TargetRaw["batch_id"])
	assert.Equal(t, 200, result.TargetRaw["http_status"])
	assert.Equal(t, 145, result.TargetRaw["latency_ms"])
}

// TestOrchidResponseBodyExtraction tests extracting nested response body fields
func TestOrchidResponseBodyExtraction(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "total", Name: "Total", Path: "response_body.total", Type: models.ValueTypeNumber},
		{ID: "page", Name: "Page", Path: "response_body.page", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "record_count", Name: "Record Count", Path: "record_count", Type: models.ValueTypeNumber},
		{ID: "current_page", Name: "Current Page", Path: "current_page", Type: models.ValueTypeNumber},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "total"}, Target: links.LinkDirection{FieldID: "record_count"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "page"}, Target: links.LinkDirection{FieldID: "current_page"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "orchid-response-body"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	require.NoError(t, mapping.Compile())

	msg := createSampleOrchidMessage()
	result, err := mapping.ExecuteMappingPooled(msg)
	require.NoError(t, err)
	defer ReleaseMapping(result)

	assert.Equal(t, 2, result.TargetRaw["record_count"])
	assert.Equal(t, 1, result.TargetRaw["current_page"])
}

// TestOrchidNestedDetailsMapping tests mapping nested user details
func TestOrchidNestedDetailsMapping(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "step_path", Name: "Step Path", Path: "step_path", Type: models.ValueTypeString},
		{ID: "user_name", Name: "User Name", Path: "response_body.name", Type: models.ValueTypeString},
		{ID: "department", Name: "Department", Path: "response_body.department", Type: models.ValueTypeString},
		{ID: "manager", Name: "Manager", Path: "response_body.manager", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "source_step", Name: "Source Step", Path: "source_step", Type: models.ValueTypeString},
		{ID: "employee_name", Name: "Employee Name", Path: "employee_name", Type: models.ValueTypeString},
		{ID: "team", Name: "Team", Path: "team", Type: models.ValueTypeString},
		{ID: "reports_to", Name: "Reports To", Path: "reports_to", Type: models.ValueTypeString},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "step_path"}, Target: links.LinkDirection{FieldID: "source_step"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "user_name"}, Target: links.LinkDirection{FieldID: "employee_name"}},
		{Priority: 2, Source: links.LinkDirection{FieldID: "department"}, Target: links.LinkDirection{FieldID: "team"}},
		{Priority: 3, Source: links.LinkDirection{FieldID: "manager"}, Target: links.LinkDirection{FieldID: "reports_to"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "orchid-nested-details"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	require.NoError(t, mapping.Compile())

	msg := createNestedOrchidMessage()
	result, err := mapping.ExecuteMappingPooled(msg)
	require.NoError(t, err)
	defer ReleaseMapping(result)

	assert.Equal(t, "root.user_details[0]", result.TargetRaw["source_step"])
	assert.Equal(t, "Alice", result.TargetRaw["employee_name"])
	assert.Equal(t, "Engineering", result.TargetRaw["team"])
	assert.Equal(t, "Carol", result.TargetRaw["reports_to"])
}

// TestOrchidErrorResponse tests handling error responses
func TestOrchidErrorResponse(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "status_code", Name: "Status Code", Path: "status_code", Type: models.ValueTypeNumber},
		{ID: "error_type", Name: "Error Type", Path: "response_body.error", Type: models.ValueTypeString},
		{ID: "error_message", Name: "Error Message", Path: "response_body.message", Type: models.ValueTypeString},
	}

	targetFields := fields.Fields{
		{ID: "http_status", Name: "HTTP Status", Path: "http_status", Type: models.ValueTypeNumber},
		{ID: "error_code", Name: "Error Code", Path: "error_code", Type: models.ValueTypeString},
		{ID: "description", Name: "Description", Path: "description", Type: models.ValueTypeString},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "status_code"}, Target: links.LinkDirection{FieldID: "http_status"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "error_type"}, Target: links.LinkDirection{FieldID: "error_code"}},
		{Priority: 2, Source: links.LinkDirection{FieldID: "error_message"}, Target: links.LinkDirection{FieldID: "description"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "orchid-error"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	require.NoError(t, mapping.Compile())

	msg := createErrorOrchidMessage()
	result, err := mapping.ExecuteMappingPooled(msg)
	require.NoError(t, err)
	defer ReleaseMapping(result)

	assert.Equal(t, 429, result.TargetRaw["http_status"])
	assert.Equal(t, "rate_limit_exceeded", result.TargetRaw["error_code"])
	assert.Equal(t, "Too many requests", result.TargetRaw["description"])
}

// TestOrchidBatchProcessing tests processing multiple Orchid messages
func TestOrchidBatchProcessing(t *testing.T) {
	sourceFields := fields.Fields{
		{ID: "execution_id", Name: "Execution ID", Path: "execution_id", Type: models.ValueTypeString},
		{ID: "status_code", Name: "Status Code", Path: "status_code", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "batch_id", Name: "Batch ID", Path: "batch_id", Type: models.ValueTypeString},
		{ID: "status", Name: "Status", Path: "status", Type: models.ValueTypeNumber},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "execution_id"}, Target: links.LinkDirection{FieldID: "batch_id"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "status_code"}, Target: links.LinkDirection{FieldID: "status"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "orchid-batch"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	require.NoError(t, mapping.Compile())

	// Process multiple messages
	messages := []map[string]any{
		createSampleOrchidMessage(),
		createNestedOrchidMessage(),
		createErrorOrchidMessage(),
	}

	expectedStatuses := []int{200, 200, 429}

	for i, msg := range messages {
		result, err := mapping.ExecuteMappingPooled(msg)
		require.NoError(t, err)
		assert.Equal(t, "a1b2c3d4-e5f6-7890-abcd-ef1234567890", result.TargetRaw["batch_id"])
		assert.Equal(t, expectedStatuses[i], result.TargetRaw["status"])
		ReleaseMapping(result)
	}
}

// TestOrchidJSONSerialization tests that we can work with raw JSON
func TestOrchidJSONSerialization(t *testing.T) {
	// Simulate receiving JSON from Kafka
	jsonMessage := `{
		"tenant_id": "550e8400-e29b-41d4-a716-446655440000",
		"plan_key": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		"execution_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		"status_code": 200,
		"response_body": {
			"data": {
				"users": [
					{"id": 1, "name": "Alice"},
					{"id": 2, "name": "Bob"}
				]
			}
		},
		"duration_ms": 150
	}`

	var msg map[string]any
	err := json.Unmarshal([]byte(jsonMessage), &msg)
	require.NoError(t, err)

	sourceFields := fields.Fields{
		{ID: "tenant_id", Name: "Tenant ID", Path: "tenant_id", Type: models.ValueTypeString},
		{ID: "status", Name: "Status", Path: "status_code", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "source", Name: "Source", Path: "source", Type: models.ValueTypeString},
		{ID: "http_status", Name: "HTTP Status", Path: "http_status", Type: models.ValueTypeNumber},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "tenant_id"}, Target: links.LinkDirection{FieldID: "source"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "status"}, Target: links.LinkDirection{FieldID: "http_status"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "orchid-json"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	require.NoError(t, mapping.Compile())

	result, err := mapping.ExecuteMappingPooled(msg)
	require.NoError(t, err)
	defer ReleaseMapping(result)

	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", result.TargetRaw["source"])

	// JSON numbers decode as float64
	assert.Equal(t, float64(200), result.TargetRaw["http_status"])
}

// TestOrchidHighVolume tests processing many Orchid messages efficiently
func TestOrchidHighVolume(t *testing.T) {
	// Complex mapping with multiple transformations:
	// 1. execution_id -> uppercase -> batch_id
	// 2. status_code + duration_ms -> add -> multiply by 10 -> score
	// 3. duration_ms -> round -> latency
	// 4. status_code -> to_string -> status_text

	sourceFields := fields.Fields{
		{ID: "execution_id", Name: "Execution ID", Path: "execution_id", Type: models.ValueTypeString},
		{ID: "status_code", Name: "Status Code", Path: "status_code", Type: models.ValueTypeNumber},
		{ID: "duration_ms", Name: "Duration", Path: "duration_ms", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "batch_id", Name: "Batch ID", Path: "batch_id", Type: models.ValueTypeString},
		{ID: "score", Name: "Score", Path: "score", Type: models.ValueTypeNumber},
		{ID: "latency", Name: "Latency", Path: "latency", Type: models.ValueTypeNumber},
		{ID: "status_text", Name: "Status Text", Path: "status_text", Type: models.ValueTypeString},
	}

	stepDefs := []models.StepDefinition{
		// Transform execution_id to uppercase
		{
			ID:   "uppercase_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "text_to_upper",
			},
		},
		// Add status_code and duration_ms
		{
			ID:   "add_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "number_add",
			},
		},
		// Multiply the sum by 10
		{
			ID:   "multiply_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key:       "number_multiply",
				Arguments: map[string]any{"value": float64(10)},
			},
		},
		// Round duration_ms
		{
			ID:   "round_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "number_round",
			},
		},
		// Convert status_code to string
		{
			ID:   "to_string_step",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "number_to_string",
			},
		},
	}

	linkList := links.Links{
		// Chain 1: execution_id -> uppercase -> batch_id
		{Priority: 0, Source: links.LinkDirection{FieldID: "execution_id"}, Target: links.LinkDirection{StepID: "uppercase_step"}},
		{Priority: 1, Source: links.LinkDirection{StepID: "uppercase_step"}, Target: links.LinkDirection{FieldID: "batch_id"}},

		// Chain 2: (status_code + duration_ms) * 10 -> score
		{Priority: 2, Source: links.LinkDirection{FieldID: "status_code"}, Target: links.LinkDirection{StepID: "add_step"}},
		{Priority: 3, Source: links.LinkDirection{FieldID: "duration_ms"}, Target: links.LinkDirection{StepID: "add_step"}},
		{Priority: 4, Source: links.LinkDirection{StepID: "add_step"}, Target: links.LinkDirection{StepID: "multiply_step"}},
		{Priority: 5, Source: links.LinkDirection{StepID: "multiply_step"}, Target: links.LinkDirection{FieldID: "score"}},

		// Chain 3: duration_ms -> round -> latency
		{Priority: 6, Source: links.LinkDirection{FieldID: "duration_ms"}, Target: links.LinkDirection{StepID: "round_step"}},
		{Priority: 7, Source: links.LinkDirection{StepID: "round_step"}, Target: links.LinkDirection{FieldID: "latency"}},

		// Chain 4: status_code -> to_string -> status_text
		{Priority: 8, Source: links.LinkDirection{FieldID: "status_code"}, Target: links.LinkDirection{StepID: "to_string_step"}},
		{Priority: 9, Source: links.LinkDirection{StepID: "to_string_step"}, Target: links.LinkDirection{FieldID: "status_text"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "orchid-high-volume-complex"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	require.NoError(t, mapping.Compile())

	msg := createSampleOrchidMessage()

	// Verify the mapping works correctly first
	testResult, err := mapping.ExecuteMappingPooled(msg)
	require.NoError(t, err)
	assert.NotEmpty(t, testResult.TargetRaw["batch_id"])
	assert.NotNil(t, testResult.TargetRaw["score"])
	assert.NotNil(t, testResult.TargetRaw["latency"])
	assert.NotEmpty(t, testResult.TargetRaw["status_text"])
	ReleaseMapping(testResult)

	// Process 100,000 messages with complex transformations
	iterations := 100000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		result, err := mapping.ExecuteMappingPooled(msg)
		require.NoError(t, err)
		ReleaseMapping(result)
	}

	elapsed := time.Since(start)
	throughput := float64(iterations) / elapsed.Seconds()

	t.Logf("Processed %d Orchid messages with complex transformations in %v", iterations, elapsed)
	t.Logf("Throughput: %.0f messages/second", throughput)
	t.Logf("Transformations per message: 5 steps, 4 output fields")

	// With complex transformations, expect lower throughput but still good performance
	// Adjusted threshold to 50K/sec for complex mappings
	assert.Greater(t, throughput, float64(50000), "Throughput should exceed 50K messages/second for complex mappings")
}

// BenchmarkOrchidFullPipeline benchmarks a realistic Orchid message processing pipeline
func BenchmarkOrchidFullPipeline(b *testing.B) {
	sourceFields := fields.Fields{
		{ID: "tenant_id", Name: "Tenant ID", Path: "tenant_id", Type: models.ValueTypeString},
		{ID: "execution_id", Name: "Execution ID", Path: "execution_id", Type: models.ValueTypeString},
		{ID: "status_code", Name: "Status Code", Path: "status_code", Type: models.ValueTypeNumber},
		{ID: "duration_ms", Name: "Duration", Path: "duration_ms", Type: models.ValueTypeNumber},
		{ID: "total", Name: "Total", Path: "response_body.total", Type: models.ValueTypeNumber},
		{ID: "page", Name: "Page", Path: "response_body.page", Type: models.ValueTypeNumber},
	}

	targetFields := fields.Fields{
		{ID: "source_tenant", Name: "Source Tenant", Path: "source_tenant", Type: models.ValueTypeString},
		{ID: "batch_id", Name: "Batch ID", Path: "batch_id", Type: models.ValueTypeString},
		{ID: "http_status", Name: "HTTP Status", Path: "http_status", Type: models.ValueTypeNumber},
		{ID: "latency_ms", Name: "Latency", Path: "latency_ms", Type: models.ValueTypeNumber},
		{ID: "record_count", Name: "Record Count", Path: "record_count", Type: models.ValueTypeNumber},
		{ID: "page_number", Name: "Page Number", Path: "page_number", Type: models.ValueTypeNumber},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "tenant_id"}, Target: links.LinkDirection{FieldID: "source_tenant"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "execution_id"}, Target: links.LinkDirection{FieldID: "batch_id"}},
		{Priority: 2, Source: links.LinkDirection{FieldID: "status_code"}, Target: links.LinkDirection{FieldID: "http_status"}},
		{Priority: 3, Source: links.LinkDirection{FieldID: "duration_ms"}, Target: links.LinkDirection{FieldID: "latency_ms"}},
		{Priority: 4, Source: links.LinkDirection{FieldID: "total"}, Target: links.LinkDirection{FieldID: "record_count"}},
		{Priority: 5, Source: links.LinkDirection{FieldID: "page"}, Target: links.LinkDirection{FieldID: "page_number"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "orchid-pipeline"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	if err := mapping.Compile(); err != nil {
		b.Fatal(err)
	}

	msg := createSampleOrchidMessage()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := mapping.ExecuteMappingPooled(msg)
		if err != nil {
			b.Fatal(err)
		}
		ReleaseMapping(result)
	}
}
