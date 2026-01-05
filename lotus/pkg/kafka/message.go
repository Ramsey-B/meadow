package kafka

import (
	"encoding/json"
	"time"
)

// OrchidMessage represents an incoming message from Orchid's Kafka output.
// This is the exact schema that Orchid produces.
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
	StatusCode      int               `json:"status_code"`
	ResponseBody    json.RawMessage   `json:"response_body"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	ResponseSize    int64             `json:"response_size"`
	DurationMs      int64             `json:"duration_ms"`

	// Extracted data (optional, from Orchid's JMESPath extraction)
	ExtractedData map[string]any `json:"extracted_data,omitempty"`
}

// ParseOrchidMessage parses a raw Kafka message into an OrchidMessage
func ParseOrchidMessage(data []byte) (*OrchidMessage, error) {
	var msg OrchidMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ToMap converts the OrchidMessage to a map[string]any for mapping execution.
// The response_body is parsed into a nested map structure.
func (m *OrchidMessage) ToMap() (map[string]any, error) {
	// First, marshal the whole message to JSON
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	// Then unmarshal to map to get proper nested structure
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// MessageSource identifies the source of a message
type MessageSource struct {
	Type        string `json:"type"` // e.g., "orchid"
	TenantID    string `json:"tenant_id"`
	Integration string `json:"integration"`
	Key         string `json:"key,omitempty"`       // Plan key
	ConfigID    string `json:"config_id,omitempty"` // Integration configuration ID
	ExecutionID string `json:"execution_id,omitempty"`
}

// MappedMessage represents the output message after mapping transformation.
// This is what Lotus produces to the output Kafka topic.
type MappedMessage struct {
	// Source information
	Source MessageSource `json:"source"`

	// Binding/mapping information
	BindingID      string `json:"binding_id"`
	MappingID      string `json:"mapping_id"`
	MappingVersion int    `json:"mapping_version"`

	// Output
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`

	// Tracing
	TraceID string `json:"trace_id,omitempty"`
	SpanID  string `json:"span_id,omitempty"`
}

// ToJSON serializes the MappedMessage to JSON bytes
func (m *MappedMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// MessageHeaders contains Kafka message headers for efficient filtering
type MessageHeaders struct {
	TenantID    string
	Integration string
	PlanKey     string
	ExecutionID string
	BindingID   string
	TraceParent string
	TraceState  string
}

// ToKafkaHeaders converts MessageHeaders to a slice of header key-value pairs
func (h *MessageHeaders) ToKafkaHeaders() []Header {
	headers := make([]Header, 0, 7)

	if h.TenantID != "" {
		headers = append(headers, Header{Key: "tenant_id", Value: []byte(h.TenantID)})
	}
	if h.Integration != "" {
		headers = append(headers, Header{Key: "integration", Value: []byte(h.Integration)})
	}
	if h.PlanKey != "" {
		headers = append(headers, Header{Key: "plan_key", Value: []byte(h.PlanKey)})
	}
	if h.ExecutionID != "" {
		headers = append(headers, Header{Key: "execution_id", Value: []byte(h.ExecutionID)})
	}
	if h.BindingID != "" {
		headers = append(headers, Header{Key: "binding_id", Value: []byte(h.BindingID)})
	}
	if h.TraceParent != "" {
		headers = append(headers, Header{Key: "traceparent", Value: []byte(h.TraceParent)})
	}
	if h.TraceState != "" {
		headers = append(headers, Header{Key: "tracestate", Value: []byte(h.TraceState)})
	}

	return headers
}

// Header represents a Kafka message header
type Header struct {
	Key   string
	Value []byte
}

// ExtractHeaders extracts MessageHeaders from Kafka headers
func ExtractHeaders(headers []Header) MessageHeaders {
	var mh MessageHeaders
	for _, h := range headers {
		switch h.Key {
		case "tenant_id":
			mh.TenantID = string(h.Value)
		case "integration":
			mh.Integration = string(h.Value)
		case "plan_key":
			mh.PlanKey = string(h.Value)
		case "execution_id":
			mh.ExecutionID = string(h.Value)
		case "binding_id":
			mh.BindingID = string(h.Value)
		case "traceparent":
			mh.TraceParent = string(h.Value)
		case "tracestate":
			mh.TraceState = string(h.Value)
		}
	}
	return mh
}
