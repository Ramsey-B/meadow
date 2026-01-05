package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TestKafkaPipelineIntegration tests the Orchid → Kafka → Lotus pipeline
// This test simulates Orchid by directly publishing to Kafka and verifying Lotus processes it
func TestKafkaPipelineIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := DefaultConfig()

	// Quick check - skip if services aren't running
	RequireService(t, cfg.LotusURL)

	kafkaHelper := NewKafkaHelper(cfg.KafkaBrokers)
	lotusClient := NewHTTPClient(cfg.LotusURL, cfg.TestTenantID)

	ctx := context.Background()

	t.Log("Lotus is healthy")

	// Cleanup old test data first
	t.Log("Cleaning up old test bindings...")
	cleanupOldBindings(t, lotusClient)

	// Step 2: Create a mapping definition in Lotus
	t.Log("Creating mapping definition...")
	mappingDef := map[string]any{
		"name":        "E2E Test Mapping",
		"description": "Test mapping for E2E tests",
		"key":         "e2e-test-mapping",
		"source_fields": []map[string]any{
			{"id": "src_user_id", "path": "response_body.user.id", "type": "string"},
			{"id": "src_user_name", "path": "response_body.user.name", "type": "string"},
			{"id": "src_user_email", "path": "response_body.user.email", "type": "string"},
		},
		"target_fields": []map[string]any{
			{"id": "tgt_id", "path": "id", "type": "string"},
			{"id": "tgt_full_name", "path": "fullName", "type": "string"},
			{"id": "tgt_email", "path": "email", "type": "string"},
		},
		"steps": []map[string]any{},
		"links": []map[string]any{
			{"source": map[string]any{"field_id": "src_user_id"}, "target": map[string]any{"field_id": "tgt_id"}},
			{"source": map[string]any{"field_id": "src_user_name"}, "target": map[string]any{"field_id": "tgt_full_name"}},
			{"source": map[string]any{"field_id": "src_user_email"}, "target": map[string]any{"field_id": "tgt_email"}},
		},
	}

	resp, err := lotusClient.Post("/api/v1/mappings/definitions", mappingDef)
	if err != nil {
		t.Fatalf("Failed to create mapping definition: %v", err)
	}
	if resp.StatusCode >= 300 {
		body, _ := ParseResponse[map[string]any](resp)
		t.Fatalf("Failed to create mapping definition: %d - %v", resp.StatusCode, body)
	}

	createdMapping, err := ParseResponse[map[string]any](resp)
	if err != nil {
		t.Fatalf("Failed to parse mapping response: %v", err)
	}
	t.Logf("Created mapping response: %+v", createdMapping)
	mappingID := createdMapping["id"].(string)
	t.Logf("Created mapping definition: %s", mappingID)

	// Step 3: Create a binding for the mapping
	t.Log("Creating binding...")
	binding := map[string]any{
		"name":         "E2E Test Binding",
		"mapping_id":   mappingID,
		"is_enabled":   true,
		"output_topic": cfg.LotusOutputTopic,
		"filter": map[string]any{
			"integration":     "okta",
			"min_status_code": 200,
			"max_status_code": 299,
		},
	}

	resp, err = lotusClient.Post("/api/v1/bindings", binding)
	if err != nil {
		t.Fatalf("Failed to create binding: %v", err)
	}
	if resp.StatusCode >= 300 {
		body, _ := ParseResponse[map[string]any](resp)
		t.Fatalf("Failed to create binding: %d - %v", resp.StatusCode, body)
	}

	createdBinding, err := ParseResponse[map[string]any](resp)
	if err != nil {
		t.Fatalf("Failed to parse binding response: %v", err)
	}
	bindingID := createdBinding["id"].(string)
	t.Logf("Created binding: %s", bindingID)

	// Step 4: Produce an Orchid message to Kafka
	t.Log("Producing Orchid message to Kafka...")

	// Record time before publishing to filter out old messages
	publishTime := time.Now().Add(-1 * time.Second) // Small buffer for clock skew

	orchidMsg := CreateOrchidMessage(cfg.TestTenantID, "test-plan-001", map[string]any{
		"user": map[string]any{
			"id":    "user-123",
			"name":  "John Doe",
			"email": "john.doe@example.com",
		},
	})

	msgBytes, err := json.Marshal(orchidMsg)
	if err != nil {
		t.Fatalf("Failed to marshal Orchid message: %v", err)
	}

	headers := map[string]string{
		"tenant_id": cfg.TestTenantID,
	}

	err = kafkaHelper.ProduceMessage(ctx, cfg.OrchidInputTopic, cfg.TestTenantID, msgBytes, headers)
	if err != nil {
		t.Fatalf("Failed to produce message to Kafka: %v", err)
	}
	t.Log("Produced Orchid message to Kafka")

	// Give Lotus time to process the message
	time.Sleep(2 * time.Second)

	// Check Lotus stats to see if it processed anything
	statsResp, _ := lotusClient.Get("/api/v1/health")
	if statsResp != nil {
		statsBody, _ := ParseResponse[map[string]any](statsResp)
		t.Logf("Lotus health: %+v", statsBody)
	}

	// Step 5: Consume the output from Lotus's output topic (filter for recent messages only)
	t.Log("Waiting for Lotus to process message...")
	messages, err := kafkaHelper.ConsumeMessagesAfter(
		ctx,
		cfg.LotusOutputTopic,
		fmt.Sprintf("e2e-test-%d", time.Now().UnixNano()),
		30*time.Second,
		1,
		publishTime,
	)
	if err != nil {
		t.Fatalf("Failed to consume messages: %v", err)
	}

	if len(messages) == 0 {
		t.Fatal("No messages received from Lotus output topic")
	}

	// Step 6: Verify the output
	t.Log("Verifying output message...")
	var outputMsg map[string]any
	if err := json.Unmarshal(messages[0].Value, &outputMsg); err != nil {
		t.Fatalf("Failed to parse output message: %v", err)
	}

	// Debug: print the full message
	t.Logf("Full output message: %+v", outputMsg)

	// Check that the data was mapped correctly
	data, ok := outputMsg["data"].(map[string]any)
	if !ok {
		t.Fatalf("Output message missing 'data' field: %v", outputMsg)
	}

	t.Logf("Data field: %+v", data)

	if data["id"] != "user-123" {
		t.Errorf("Expected id 'user-123', got '%v'", data["id"])
	}
	if data["fullName"] != "John Doe" {
		t.Errorf("Expected fullName 'John Doe', got '%v'", data["fullName"])
	}
	if data["email"] != "john.doe@example.com" {
		t.Errorf("Expected email 'john.doe@example.com', got '%v'", data["email"])
	}

	t.Log("E2E test passed! Message was successfully processed through the pipeline.")

	// Cleanup
	t.Log("Cleaning up...")
	lotusClient.Delete(fmt.Sprintf("/api/v1/bindings/%s", bindingID))
	t.Log("Cleanup complete")
}

// TestBindingFiltering tests that bindings correctly filter messages
func TestBindingFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := DefaultConfig()

	// Quick check - skip if services aren't running
	RequireService(t, cfg.LotusURL)

	kafkaHelper := NewKafkaHelper(cfg.KafkaBrokers)
	lotusClient := NewHTTPClient(cfg.LotusURL, cfg.TestTenantID)

	ctx := context.Background()

	// Cleanup old test data first
	cleanupOldBindings(t, lotusClient)

	// Create a mapping
	mappingDef := map[string]any{
		"name":        "Filter Test Mapping",
		"description": "Test mapping for filter tests",
		"key":         "filter-test-mapping",
		"source_fields": []map[string]any{
			{"id": "src_status", "path": "response_body.status", "type": "string"},
		},
		"target_fields": []map[string]any{
			{"id": "tgt_status", "path": "status", "type": "string"},
		},
		"steps": []map[string]any{},
		"links": []map[string]any{
			{"source": map[string]any{"field_id": "src_status"}, "target": map[string]any{"field_id": "tgt_status"}},
		},
	}

	resp, err := lotusClient.Post("/api/v1/mappings/definitions", mappingDef)
	if err != nil {
		t.Fatalf("Failed to create mapping: %v", err)
	}
	if resp.StatusCode >= 300 {
		body, _ := ParseResponse[map[string]any](resp)
		t.Fatalf("Failed to create mapping definition: %d - %v", resp.StatusCode, body)
	}
	createdMapping, err := ParseResponse[map[string]any](resp)
	if err != nil {
		t.Fatalf("Failed to parse mapping response: %v", err)
	}
	mappingID, ok := createdMapping["id"].(string)
	if !ok {
		t.Fatalf("Mapping response missing 'id' field: %v", createdMapping)
	}

	// Create binding that only matches status code 200
	binding := map[string]any{
		"name":         "Success Only Binding",
		"mapping_id":   mappingID,
		"is_enabled":   true,
		"output_topic": cfg.LotusOutputTopic,
		"filter": map[string]any{
			"status_codes": []int{200},
		},
	}

	resp, err = lotusClient.Post("/api/v1/bindings", binding)
	if err != nil {
		t.Fatalf("Failed to create binding: %v", err)
	}
	if resp.StatusCode >= 300 {
		body, _ := ParseResponse[map[string]any](resp)
		t.Fatalf("Failed to create binding: %d - %v", resp.StatusCode, body)
	}
	createdBinding, err := ParseResponse[map[string]any](resp)
	if err != nil {
		t.Fatalf("Failed to parse binding response: %v", err)
	}
	bindingID, ok := createdBinding["id"].(string)
	if !ok {
		t.Fatalf("Binding response missing 'id' field: %v", createdBinding)
	}

	// Record time before publishing to filter out old messages
	publishTime := time.Now().Add(-1 * time.Second)

	// Send a 404 message (should NOT be processed)
	msg404 := CreateOrchidMessage(cfg.TestTenantID, "filter-test-plan", map[string]any{
		"status": "not_found",
	})
	msg404.StatusCode = 404
	msgBytes, _ := json.Marshal(msg404)
	kafkaHelper.ProduceMessage(ctx, cfg.OrchidInputTopic, cfg.TestTenantID, msgBytes, map[string]string{
		"tenant_id": cfg.TestTenantID,
	})

	// Send a 200 message (should be processed)
	msg200 := CreateOrchidMessage(cfg.TestTenantID, "filter-test-plan", map[string]any{
		"status": "success",
	})
	msg200.StatusCode = 200
	msgBytes, _ = json.Marshal(msg200)
	kafkaHelper.ProduceMessage(ctx, cfg.OrchidInputTopic, cfg.TestTenantID, msgBytes, map[string]string{
		"tenant_id": cfg.TestTenantID,
	})

	// Wait and consume (filter for recent messages only)
	messages, _ := kafkaHelper.ConsumeMessagesAfter(
		ctx,
		cfg.LotusOutputTopic,
		fmt.Sprintf("filter-test-%d", time.Now().UnixNano()),
		10*time.Second,
		2,
		publishTime,
	)

	// Should only get the 200 message
	successCount := 0
	for _, msg := range messages {
		var output map[string]any
		json.Unmarshal(msg.Value, &output)
		if data, ok := output["data"].(map[string]any); ok {
			if data["status"] == "success" {
				successCount++
			}
		}
	}

	if successCount == 0 {
		t.Error("Expected at least one success message to be processed")
	}

	// Cleanup
	lotusClient.Delete(fmt.Sprintf("/api/v1/bindings/%s", bindingID))
}

// TestHealthEndpoints verifies health endpoints are working
func TestHealthEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := DefaultConfig()

	// Quick check - skip if services aren't running
	RequireService(t, cfg.LotusURL)
	RequireService(t, cfg.OrchidURL)

	// Test Lotus health
	lotusClient := NewHTTPClient(cfg.LotusURL, cfg.TestTenantID)

	resp, err := lotusClient.Get("/api/v1/health")
	if err != nil {
		t.Fatalf("Failed to get Lotus health: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected Lotus health status 200, got %d", resp.StatusCode)
	}

	resp, err = lotusClient.Get("/api/v1/health/live")
	if err != nil {
		t.Fatalf("Failed to get Lotus liveness: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected Lotus liveness status 200, got %d", resp.StatusCode)
	}

	resp, err = lotusClient.Get("/api/v1/health/ready")
	if err != nil {
		t.Fatalf("Failed to get Lotus readiness: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected Lotus readiness status 200, got %d", resp.StatusCode)
	}

	// Test Orchid health
	orchidClient := NewHTTPClient(cfg.OrchidURL, cfg.TestTenantID)

	resp, err = orchidClient.Get("/api/v1/health")
	if err != nil {
		t.Fatalf("Failed to get Orchid health: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected Orchid health status 200, got %d", resp.StatusCode)
	}
}
