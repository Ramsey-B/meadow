package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

// Config holds test configuration
type Config struct {
	OrchidURL         string
	LotusURL          string
	IvyURL            string
	KafkaBrokers      []string
	OrchidInputTopic  string
	LotusOutputTopic  string
	OrchidEventsTopic string // For execution.completed events
	IvyEventsTopic    string // For entity/relationship events
	TestTenantID      string
}

// DefaultConfig returns default test configuration
func DefaultConfig() Config {
	return Config{
		OrchidURL:         getEnv("ORCHID_URL", "http://localhost:3001"),
		LotusURL:          getEnv("LOTUS_URL", "http://localhost:3000"),
		IvyURL:            getEnv("IVY_URL", "http://localhost:3002"),
		KafkaBrokers:      []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
		OrchidInputTopic:  getEnv("ORCHID_INPUT_TOPIC", "api-responses"),
		LotusOutputTopic:  getEnv("LOTUS_OUTPUT_TOPIC", "mapped-data"),
		OrchidEventsTopic: getEnv("ORCHID_EVENTS_TOPIC", "orchid-events"),
		IvyEventsTopic:    getEnv("IVY_EVENTS_TOPIC", "ivy-events"),
		TestTenantID:      getEnv("TEST_TENANT_ID", "test-tenant-e2e"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// HTTPClient wraps http.Client with helper methods
type HTTPClient struct {
	client   *http.Client
	baseURL  string
	tenantID string
}

// NewHTTPClient creates a new HTTP client for a service
func NewHTTPClient(baseURL, tenantID string) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:  baseURL,
		tenantID: tenantID,
	}
}

// Get performs a GET request
func (c *HTTPClient) Get(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.addHeaders(req)
	return c.client.Do(req)
}

// Post performs a POST request with JSON body
func (c *HTTPClient) Post(path string, body any) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	c.addHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}

// Put performs a PUT request with JSON body
func (c *HTTPClient) Put(path string, body any) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("PUT", c.baseURL+path, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	c.addHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}

// Delete performs a DELETE request
func (c *HTTPClient) Delete(path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.addHeaders(req)
	return c.client.Do(req)
}

func (c *HTTPClient) addHeaders(req *http.Request) {
	// Test auth headers - used when AUTH_ENABLED=false
	req.Header.Set("X-Tenant-ID", c.tenantID)
	req.Header.Set("X-User-ID", "e2e-test-user")
}

// ParseResponse parses a JSON response into the given type
func ParseResponse[T any](resp *http.Response) (T, error) {
	var result T
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	
	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}
	return result, nil
}

// KafkaHelper provides Kafka testing utilities
type KafkaHelper struct {
	brokers []string
}

// NewKafkaHelper creates a new Kafka helper
func NewKafkaHelper(brokers []string) *KafkaHelper {
	return &KafkaHelper{brokers: brokers}
}

// ProduceMessage sends a message to a topic
func (k *KafkaHelper) ProduceMessage(ctx context.Context, topic string, key string, value []byte, headers map[string]string) error {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(k.brokers...),
		Topic:    topic,
		Balancer: &kafka.Hash{},
	}
	defer writer.Close()

	kafkaHeaders := make([]kafka.Header, 0, len(headers))
	for k, v := range headers {
		kafkaHeaders = append(kafkaHeaders, kafka.Header{Key: k, Value: []byte(v)})
	}

	return writer.WriteMessages(ctx, kafka.Message{
		Key:     []byte(key),
		Value:   value,
		Headers: kafkaHeaders,
	})
}

// ConsumeMessages consumes messages from a topic with a timeout
// Only returns messages published after 'afterTime' to filter out stale messages
func (k *KafkaHelper) ConsumeMessages(ctx context.Context, topic, groupID string, timeout time.Duration, maxMessages int) ([]kafka.Message, error) {
	return k.ConsumeMessagesAfter(ctx, topic, groupID, timeout, maxMessages, time.Time{})
}

// ConsumeMessagesAfter consumes messages from a topic, filtering for messages after a specific time
func (k *KafkaHelper) ConsumeMessagesAfter(ctx context.Context, topic, groupID string, timeout time.Duration, maxMessages int, afterTime time.Time) ([]kafka.Message, error) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  k.brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
		MaxWait:  500 * time.Millisecond,
	})
	defer reader.Close()

	messages := make([]kafka.Message, 0, maxMessages)
	deadline := time.Now().Add(timeout)

	for len(messages) < maxMessages && time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		msg, err := reader.FetchMessage(ctx)
		cancel()
		
		if err != nil {
			if ctx.Err() != nil {
				continue // Timeout, try again
			}
			return messages, err
		}
		
		// Commit all messages to advance offset, but only keep recent ones
		reader.CommitMessages(context.Background(), msg)
		
		// Filter: only keep messages after the specified time
		if !afterTime.IsZero() && msg.Time.Before(afterTime) {
			continue // Skip old messages
		}
		
		messages = append(messages, msg)
	}

	return messages, nil
}

// WaitForService waits for a service to be healthy
// Returns true if service is available, false otherwise
func WaitForService(t *testing.T, url string, timeout time.Duration) bool {
	t.Helper()
	
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	
	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/api/v1/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return true
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	
	return false
}

// RequireService skips the test if the service is not available
// Waits up to 10 seconds for service to become ready (handles 503 during startup)
func RequireService(t *testing.T, url string) {
	t.Helper()
	
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(10 * time.Second)
	
	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/api/v1/health")
		if err != nil {
			// Service not running at all
			t.Skipf("Skipping: service at %s is not available. Start services with 'make dev-orchid' and 'make dev-lotus'", url)
			return
		}
		
		status := resp.StatusCode
		resp.Body.Close()
		
		if status == http.StatusOK {
			return // Service is ready
		}
		
		if status == http.StatusServiceUnavailable {
			// Service is starting up, wait and retry
			t.Logf("Service at %s is starting (503), waiting...", url)
			time.Sleep(1 * time.Second)
			continue
		}
		
		// Other error status
		t.Skipf("Skipping: service at %s returned status %d", url, status)
		return
	}
	
	t.Skipf("Skipping: service at %s did not become ready within 10s", url)
}

// OrchidMessage represents a message from Orchid
type OrchidMessage struct {
	TenantID      string         `json:"tenant_id"`
	PlanID        string         `json:"plan_id"`
	ExecutionID   string         `json:"execution_id"`
	StepPath      string         `json:"step_path"`
	IntegrationID string         `json:"integration_id"`
	StatusCode    int            `json:"status_code"`
	ResponseBody  map[string]any `json:"response_body"`
	Timestamp     time.Time      `json:"timestamp"`
	DurationMs    int64          `json:"duration_ms"`
	TraceID       string         `json:"trace_id,omitempty"`
	SpanID        string         `json:"span_id,omitempty"`
}

// CreateOrchidMessage creates a test Orchid message
func CreateOrchidMessage(tenantID, planID string, responseBody map[string]any) OrchidMessage {
	return OrchidMessage{
		TenantID:      tenantID,
		PlanID:        planID,
		ExecutionID:   fmt.Sprintf("exec-%d", time.Now().UnixNano()),
		StepPath:      "root",
		IntegrationID: "test-integration",
		StatusCode:    200,
		ResponseBody:  responseBody,
		Timestamp:     time.Now().UTC(),
		DurationMs:    100,
	}
}

// cleanupOldBindings deletes all bindings for the test tenant
// This is needed because old bindings from previous test runs may interfere
func cleanupOldBindings(t *testing.T, client *HTTPClient) {
	t.Helper()
	
	// List all bindings
	resp, err := client.Get("/api/v1/bindings")
	if err != nil {
		t.Logf("Warning: failed to list bindings for cleanup: %v", err)
		return
	}
	
	if resp.StatusCode != 200 {
		t.Logf("Warning: failed to list bindings, status: %d", resp.StatusCode)
		return
	}
	
	var bindings []map[string]any
	if err := parseResponseBody(resp, &bindings); err != nil {
		t.Logf("Warning: failed to parse bindings list: %v", err)
		return
	}
	
	// Delete each binding
	for _, binding := range bindings {
		id, ok := binding["id"].(string)
		if !ok {
			continue
		}
		
		_, err := client.Delete(fmt.Sprintf("/api/v1/bindings/%s", id))
		if err != nil {
			t.Logf("Warning: failed to delete binding %s: %v", id, err)
		}
	}
	
	if len(bindings) > 0 {
		t.Logf("Cleaned up %d old bindings", len(bindings))
	}
}

func parseResponseBody(resp *http.Response, v any) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.Unmarshal(body, v)
}

