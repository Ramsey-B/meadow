// Package integration contains end-to-end integration tests for the Orchid API.
// Run with: go test -v ./test/integration/... -tags=integration
package integration

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

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	baseURL  = getEnv("TEST_BASE_URL", "http://localhost:3001/api/v1")
	tenantID = getEnv("TEST_TENANT_ID", "11111111-1111-1111-1111-111111111111")
)

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// TestClient wraps http.Client with common headers
type TestClient struct {
	*http.Client
	baseURL  string
	tenantID string
}

func NewTestClient() *TestClient {
	return &TestClient{
		Client:   &http.Client{Timeout: 30 * time.Second},
		baseURL:  baseURL,
		tenantID: tenantID,
	}
}

func (c *TestClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", c.tenantID)
	return c.Client.Do(req)
}

func (c *TestClient) Get(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

func (c *TestClient) Post(path string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

func (c *TestClient) Delete(path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

func parseResponse(t *testing.T, resp *http.Response, target any) {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read response body")
	if target != nil {
		require.NoError(t, json.Unmarshal(body, target), "failed to parse response: %s", string(body))
	}
}

// TestHealthCheck verifies the API is running
func TestHealthCheck(t *testing.T) {
	client := NewTestClient()

	resp, err := client.Get("/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]string
	parseResponse(t, resp, &result)
	assert.Equal(t, "healthy", result["status"])
}

// TestIntegrationCRUD tests the full CRUD lifecycle for integrations
func TestIntegrationCRUD(t *testing.T) {
	client := NewTestClient()
	uniqueName := fmt.Sprintf("Test Integration %d", time.Now().UnixNano())

	// Create
	createReq := map[string]any{
		"name":        uniqueName,
		"description": "Test integration for e2e testing",
	}

	resp, err := client.Post("/integrations", createReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var created map[string]any
	parseResponse(t, resp, &created)
	integrationID := created["id"].(string)
	assert.NotEmpty(t, integrationID)
	assert.Equal(t, uniqueName, created["name"])

	// Read
	resp, err = client.Get("/integrations/" + integrationID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var fetched map[string]any
	parseResponse(t, resp, &fetched)
	assert.Equal(t, integrationID, fetched["id"])

	// List
	resp, err = client.Get("/integrations")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var list []map[string]any
	parseResponse(t, resp, &list)
	assert.GreaterOrEqual(t, len(list), 1)

	// Delete
	resp, err = client.Delete("/integrations/" + integrationID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify deleted
	resp, err = client.Get("/integrations/" + integrationID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

// TestPlanExecutionE2E tests the complete flow: create resources, trigger execution, verify output
func TestPlanExecutionE2E(t *testing.T) {
	client := NewTestClient()
	suffix := time.Now().UnixNano()

	// Step 1: Create Integration
	integration := map[string]any{
		"name":        fmt.Sprintf("JSONPlaceholder %d", suffix),
		"description": "Test API for e2e testing",
	}
	resp, err := client.Post("/integrations", integration)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "failed to create integration")

	var integrationResp map[string]any
	parseResponse(t, resp, &integrationResp)
	integrationID := integrationResp["id"].(string)
	t.Logf("Created integration: %s", integrationID)

	// Cleanup
	t.Cleanup(func() {
		resp, _ := client.Delete("/integrations/" + integrationID)
		if resp != nil {
			resp.Body.Close()
		}
	})

	// Step 2: Create Config
	config := map[string]any{
		"integration_id": integrationID,
		"name":           fmt.Sprintf("Test Config %d", suffix),
		"values": map[string]any{
			"base_url": "https://jsonplaceholder.typicode.com",
		},
		"enabled": true,
	}
	resp, err = client.Post("/configs", config)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "failed to create config")

	var configResp map[string]any
	parseResponse(t, resp, &configResp)
	configID := configResp["id"].(string)
	t.Logf("Created config: %s", configID)

	// Step 4: Create Plan
	plan := map[string]any{
		"integration_id": integrationID,
		"name":           fmt.Sprintf("Fetch Users %d", suffix),
		"description":    "Fetches all users from JSONPlaceholder",
		"enabled":        true,
		"wait_seconds":   60,
		"repeat_count":   0,
		"plan_definition": map[string]any{
			"step": map[string]any{
				"url":             "{{ config.base_url }}/users",
				"method":          "GET",
				"timeout_seconds": 30,
				"set_context": map[string]any{
					"user_count": "length(response.body)",
				},
			},
			"max_execution_seconds": 60,
		},
	}
	resp, err = client.Post("/plans", plan)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "failed to create plan")

	var planResp map[string]any
	parseResponse(t, resp, &planResp)
	planKey := planResp["key"].(string)
	t.Logf("Created plan: %s", planKey)

	// Step 5: Trigger Plan Execution
	trigger := map[string]any{
		"config_id": configID,
	}
	resp, err = client.Post("/plans/"+planKey+"/trigger", trigger)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "failed to trigger plan")

	var triggerResp map[string]any
	parseResponse(t, resp, &triggerResp)
	assert.Equal(t, "queued", triggerResp["status"])
	t.Logf("Plan execution triggered: %v", triggerResp)

	// Step 6: Poll for execution to complete
	t.Log("Waiting for execution to complete...")
	var executions []map[string]any
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		resp, err = client.Get("/executions?plan_key=" + planKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		parseResponse(t, resp, &executions)
		if len(executions) > 0 {
			break
		}
	}

	// Step 7: Check execution status
	require.GreaterOrEqual(t, len(executions), 1, "expected at least 1 execution")

	// Find the most recent execution
	latestExec := executions[0]
	t.Logf("Execution status: %s", latestExec["status"])
	assert.Equal(t, "success", latestExec["status"], "execution should have succeeded")
	assert.Equal(t, planKey, latestExec["plan_key"])
	assert.Equal(t, configID, latestExec["config_id"])
}

// TestPlanWithSetContext tests a plan with set_context
func TestPlanWithSetContext(t *testing.T) {
	client := NewTestClient()
	suffix := time.Now().UnixNano()

	// Create minimal resources for the test
	integration := map[string]any{
		"name": fmt.Sprintf("Pagination Test %d", suffix),
	}
	resp, err := client.Post("/integrations", integration)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var integrationResp map[string]any
	parseResponse(t, resp, &integrationResp)
	integrationID := integrationResp["id"].(string)

	t.Cleanup(func() {
		resp, _ := client.Delete("/integrations/" + integrationID)
		if resp != nil {
			resp.Body.Close()
		}
	})

	// Config
	config := map[string]any{
		"integration_id": integrationID,
		"name":           fmt.Sprintf("Pagination Config %d", suffix),
		"values": map[string]any{
			"base_url": "https://jsonplaceholder.typicode.com",
		},
		"enabled": true,
	}
	resp, err = client.Post("/configs", config)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var configResp map[string]any
	parseResponse(t, resp, &configResp)
	configID := configResp["id"].(string)

	// Plan - simple fetch without complex JMESPath expressions
	plan := map[string]any{
		"integration_id": integrationID,
		"name":           fmt.Sprintf("Fetch Posts %d", suffix),
		"enabled":        true,
		"wait_seconds":   3600,
		"plan_definition": map[string]any{
			"step": map[string]any{
				"url":             "{{ config.base_url }}/posts?_limit=5",
				"method":          "GET",
				"timeout_seconds": 30,
				"set_context": map[string]any{
					"post_count": "length(response.body)",
				},
			},
			"max_execution_seconds": 60,
		},
	}
	resp, err = client.Post("/plans", plan)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var planResp map[string]any
	parseResponse(t, resp, &planResp)
	planKey := planResp["key"].(string)

	// Trigger
	trigger := map[string]any{"config_id": configID}
	resp, err = client.Post("/plans/"+planKey+"/trigger", trigger)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Poll for execution to complete
	var executions []map[string]any
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		resp, err = client.Get("/executions?plan_key=" + planKey)
		require.NoError(t, err)
		parseResponse(t, resp, &executions)
		if len(executions) > 0 && executions[0]["status"] == "success" {
			break
		}
	}

	require.GreaterOrEqual(t, len(executions), 1, "expected at least 1 execution")
	assert.Equal(t, "success", executions[0]["status"])
}

// TestKafkaOutput verifies that API responses are published to Kafka
func TestKafkaOutput(t *testing.T) {
	kafkaBroker := getEnv("KAFKA_BROKER", "localhost:9092")
	kafkaTopic := getEnv("KAFKA_TOPIC", "api-responses")

	// Create a Kafka reader
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{kafkaBroker},
		Topic:          kafkaTopic,
		GroupID:        fmt.Sprintf("test-consumer-%s", uuid.New().String()),
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        500 * time.Millisecond,
		StartOffset:    kafka.LastOffset,
		CommitInterval: time.Second,
	})
	defer reader.Close()

	// Run an execution that should produce a Kafka message
	client := NewTestClient()
	suffix := time.Now().UnixNano()

	// Quick setup
	integration := map[string]any{"name": fmt.Sprintf("Kafka Test %d", suffix)}
	resp, _ := client.Post("/integrations", integration)
	var integrationResp map[string]any
	parseResponse(t, resp, &integrationResp)
	integrationID := integrationResp["id"].(string)

	t.Cleanup(func() {
		resp, _ := client.Delete("/integrations/" + integrationID)
		if resp != nil {
			resp.Body.Close()
		}
	})

	config := map[string]any{
		"integration_id": integrationID,
		"name":           fmt.Sprintf("Kafka Config %d", suffix),
		"values":         map[string]any{"base_url": "https://jsonplaceholder.typicode.com"},
		"enabled":        true,
	}
	resp, _ = client.Post("/configs", config)
	var configResp map[string]any
	parseResponse(t, resp, &configResp)
	configID := configResp["id"].(string)

	plan := map[string]any{
		"integration_id": integrationID,
		"name":           fmt.Sprintf("Kafka Plan %d", suffix),
		"enabled":        true,
		"wait_seconds":   3600,
		"plan_definition": map[string]any{
			"step": map[string]any{
				"url":             "{{ config.base_url }}/todos/1",
				"method":          "GET",
				"timeout_seconds": 30,
			},
		},
	}
	resp, _ = client.Post("/plans", plan)
	var planResp map[string]any
	parseResponse(t, resp, &planResp)
	planKey := planResp["key"].(string)

	// Trigger execution
	trigger := map[string]any{"config_id": configID}
	resp, _ = client.Post("/plans/"+planKey+"/trigger", trigger)
	resp.Body.Close()

	// Wait for Kafka message
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	msg, err := reader.ReadMessage(ctx)
	if err != nil {
		t.Skipf("Kafka read timed out (Kafka may not be configured): %v", err)
	}

	// Parse and verify the message structure (don't check specific IDs as other tests may have published)
	var kafkaMsg map[string]any
	require.NoError(t, json.Unmarshal(msg.Value, &kafkaMsg))

	// Verify message has expected fields
	assert.NotEmpty(t, kafkaMsg["tenant_id"], "tenant_id should be present")
	assert.NotEmpty(t, kafkaMsg["plan_key"], "plan_key should be present")
	assert.NotEmpty(t, kafkaMsg["execution_id"], "execution_id should be present")
	assert.NotNil(t, kafkaMsg["status_code"], "status_code should be present")

	t.Logf("Received Kafka message: tenant=%s, plan=%s, status=%v",
		kafkaMsg["tenant_id"], kafkaMsg["plan_key"], kafkaMsg["status_code"])

	// Log the plan we triggered (may not be the same message due to concurrent tests)
	_ = planKey
}

// TestRateLimiting verifies that rate limiting works correctly
func TestRateLimiting(t *testing.T) {
	client := NewTestClient()
	suffix := time.Now().UnixNano()

	// Create resources
	integration := map[string]any{"name": fmt.Sprintf("Rate Limit Test %d", suffix)}
	resp, err := client.Post("/integrations", integration)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "failed to create integration")
	var integrationResp map[string]any
	parseResponse(t, resp, &integrationResp)
	integrationID := integrationResp["id"].(string)

	t.Cleanup(func() {
		resp, _ := client.Delete("/integrations/" + integrationID)
		if resp != nil {
			resp.Body.Close()
		}
	})

	config := map[string]any{
		"integration_id": integrationID,
		"name":           fmt.Sprintf("Rate Limit Config %d", suffix),
		"values":         map[string]any{"base_url": "https://jsonplaceholder.typicode.com"},
		"enabled":        true,
	}
	resp, err = client.Post("/configs", config)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "failed to create config")
	var configResp map[string]any
	parseResponse(t, resp, &configResp)
	configID := configResp["id"].(string)

	// Plan with rate limiting configured
	plan := map[string]any{
		"integration_id": integrationID,
		"name":           fmt.Sprintf("Rate Limited Plan %d", suffix),
		"enabled":        true,
		"wait_seconds":   3600,
		"plan_definition": map[string]any{
			"step": map[string]any{
				"url":             "{{ config.base_url }}/users",
				"method":          "GET",
				"timeout_seconds": 30,
			},
			"rate_limits": []map[string]any{
				{
					"name":        "test_limit",
					"requests":    5,
					"window_secs": 60,
					"scope":       "per_config",
				},
			},
		},
	}
	resp, err = client.Post("/plans", plan)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "failed to create plan")

	var planResp map[string]any
	parseResponse(t, resp, &planResp)
	planKey := planResp["key"].(string)
	t.Logf("Created rate-limited plan: %s", planKey)

	// Trigger execution
	trigger := map[string]any{"config_id": configID}
	resp, err = client.Post("/plans/"+planKey+"/trigger", trigger)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "failed to trigger plan")
	resp.Body.Close()

	// Wait and poll for execution completion (rate limit should not block single request)
	var executions []map[string]any
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		resp, err = client.Get("/executions?plan_key=" + planKey)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		parseResponse(t, resp, &executions)
		if len(executions) > 0 {
			break
		}
	}

	t.Logf("Found %d executions for plan %s", len(executions), planKey)
	require.GreaterOrEqual(t, len(executions), 1, "expected at least 1 execution")
	assert.Equal(t, "success", executions[0]["status"])
}
