package kafka

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOrchidMessage(t *testing.T) {
	jsonData := `{
		"tenant_id": "550e8400-e29b-41d4-a716-446655440000",
		"integration": "test-integration",
		"plan_key": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		"config_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"execution_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		"step_path": "root",
		"timestamp": "2025-01-15T10:30:00Z",
		"request_url": "https://api.example.com/users",
		"request_method": "GET",
		"status_code": 200,
		"response_body": {
			"users": [
				{"id": 1, "name": "Alice"}
			],
			"total": 1
		},
		"duration_ms": 150,
		"trace_id": "abc123",
		"span_id": "def456"
	}`

	msg, err := ParseOrchidMessage([]byte(jsonData))
	require.NoError(t, err)

	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", msg.TenantID)
	assert.Equal(t, "test-integration", msg.Integration)
	assert.Equal(t, "7c9e6679-7425-40de-944b-e07fc1f90ae7", msg.PlanKey)
	assert.Equal(t, "a1b2c3d4-e5f6-7890-abcd-ef1234567890", msg.ExecutionID)
	assert.Equal(t, "root", msg.StepPath)
	assert.Equal(t, 200, msg.StatusCode)
	assert.Equal(t, int64(150), msg.DurationMs)
	assert.Equal(t, "abc123", msg.TraceID)
	assert.Equal(t, "def456", msg.SpanID)
}

func TestOrchidMessageToMap(t *testing.T) {
	msg := &OrchidMessage{
		TenantID:     "tenant-1",
		Integration:  "test-integration",
		PlanKey:      "plan-1",
		ExecutionID:  "exec-1",
		StepPath:     "root",
		StatusCode:   200,
		ResponseBody: json.RawMessage(`{"users": [{"id": 1}], "total": 1}`),
		DurationMs:   100,
	}

	data, err := msg.ToMap()
	require.NoError(t, err)

	assert.Equal(t, "tenant-1", data["tenant_id"])
	assert.Equal(t, "test-integration", data["integration"])
	assert.Equal(t, "plan-1", data["plan_key"])
	assert.Equal(t, float64(200), data["status_code"])

	responseBody, ok := data["response_body"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), responseBody["total"])
}

func TestMappedMessageToJSON(t *testing.T) {
	msg := &MappedMessage{
		Source: MessageSource{
			Type:        "orchid",
			TenantID:    "tenant-1",
			Integration: "test-integration",
			Key:         "plan-1",
			ExecutionID: "exec-1",
		},
		BindingID:      "binding-1",
		MappingID:      "mapping-1",
		MappingVersion: 1,
		Timestamp:      time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Data: map[string]any{
			"transformed": "value",
		},
		TraceID: "trace-1",
	}

	data, err := msg.ToJSON()
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	source, ok := parsed["source"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "orchid", source["type"])
	assert.Equal(t, "tenant-1", source["tenant_id"])
	assert.Equal(t, "test-integration", source["integration"])

	assert.Equal(t, "binding-1", parsed["binding_id"])
	assert.Equal(t, "mapping-1", parsed["mapping_id"])
	assert.Equal(t, float64(1), parsed["mapping_version"])
}

func TestMessageHeaders(t *testing.T) {
	headers := &MessageHeaders{
		TenantID:    "tenant-1",
		Integration: "test-integration",
		PlanKey:     "plan-1",
		ExecutionID: "exec-1",
		BindingID:   "binding-1",
		TraceParent: "00-trace-span-01",
	}

	kafkaHeaders := headers.ToKafkaHeaders()

	assert.Len(t, kafkaHeaders, 6)

	headerMap := make(map[string]string)
	for _, h := range kafkaHeaders {
		headerMap[h.Key] = string(h.Value)
	}

	assert.Equal(t, "tenant-1", headerMap["tenant_id"])
	assert.Equal(t, "test-integration", headerMap["integration"])
	assert.Equal(t, "plan-1", headerMap["plan_key"])
	assert.Equal(t, "exec-1", headerMap["execution_id"])
	assert.Equal(t, "binding-1", headerMap["binding_id"])
	assert.Equal(t, "00-trace-span-01", headerMap["traceparent"])
}

func TestExtractHeaders(t *testing.T) {
	headers := []Header{
		{Key: "tenant_id", Value: []byte("tenant-1")},
		{Key: "integration", Value: []byte("test-integration")},
		{Key: "plan_key", Value: []byte("plan-1")},
		{Key: "execution_id", Value: []byte("exec-1")},
		{Key: "traceparent", Value: []byte("00-abc-def-01")},
	}

	mh := ExtractHeaders(headers)

	assert.Equal(t, "tenant-1", mh.TenantID)
	assert.Equal(t, "test-integration", mh.Integration)
	assert.Equal(t, "plan-1", mh.PlanKey)
	assert.Equal(t, "exec-1", mh.ExecutionID)
	assert.Equal(t, "00-abc-def-01", mh.TraceParent)
}
