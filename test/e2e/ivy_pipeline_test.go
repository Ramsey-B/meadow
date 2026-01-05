package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TestFullPipelineOrchidToIvy tests the complete pipeline:
// Orchid (API polling) → Kafka → Lotus (mapping) → Kafka → Ivy (entity resolution)
func TestFullPipelineOrchidToIvy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := DefaultConfig()

	// Check all services are running
	RequireService(t, cfg.LotusURL)
	RequireService(t, cfg.OrchidURL)
	RequireService(t, cfg.IvyURL)

	kafkaHelper := NewKafkaHelper(cfg.KafkaBrokers)
	lotusClient := NewHTTPClient(cfg.LotusURL, cfg.TestTenantID)
	ivyClient := NewHTTPClient(cfg.IvyURL, cfg.TestTenantID)

	ctx := context.Background()

	// Step 1: Set up Ivy entity types
	t.Log("Setting up Ivy entity type...")
	entityTypeReq := map[string]any{
		"key":         "person",
		"name":        "Person",
		"description": "A person entity",
		"schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"email":      map[string]any{"type": "string", "format": "email"},
				"first_name": map[string]any{"type": "string"},
				"last_name":  map[string]any{"type": "string"},
				"phone":      map[string]any{"type": "string"},
			},
			"required": []string{"email"},
		},
	}

	resp, err := ivyClient.Post("/api/v1/entity-types", entityTypeReq)
	if err != nil {
		t.Logf("Note: Could not create entity type (may already exist): %v", err)
	} else if resp.StatusCode >= 300 && resp.StatusCode != 409 {
		body, _ := ParseResponse[map[string]any](resp)
		t.Logf("Note: Entity type creation returned %d: %v", resp.StatusCode, body)
	}

	// Step 2: Set up match rules in Ivy
	t.Log("Setting up match rules...")
	matchRuleReq := map[string]any{
		"entity_type": "person",
		"name":        "E2E Email Match",
		"description": "Match persons by email",
		"priority":    10,
		"is_active":   true,
		"conditions": []map[string]any{
			{
				"field":      "email",
				"match_type": "exact",
				"weight":     1.0,
				"required":   true,
			},
		},
		"score_weight": 1.0,
	}

	resp, err = ivyClient.Post("/api/v1/match-rules", matchRuleReq)
	if err != nil {
		t.Logf("Note: Could not create match rule: %v", err)
	}
	var matchRuleID string
	if resp != nil && resp.StatusCode < 300 {
		matchRule, _ := ParseResponse[map[string]any](resp)
		if id, ok := matchRule["id"].(string); ok {
			matchRuleID = id
			t.Logf("Created match rule: %s", matchRuleID)
		}
	}

	// Step 3: Set up merge strategy in Ivy
	t.Log("Setting up merge strategy...")
	mergeStrategyReq := map[string]any{
		"entity_type": "person",
		"name":        "E2E Merge Strategy",
		"description": "Default merge strategy for E2E tests",
		"is_default":  true,
		"field_strategies": []map[string]any{
			{"field": "email", "strategy": "most_recent"},
			{"field": "first_name", "strategy": "most_recent"},
			{"field": "last_name", "strategy": "most_recent"},
			{"field": "phone", "strategy": "prefer_non_empty"},
		},
	}

	resp, err = ivyClient.Post("/api/v1/merge-strategies", mergeStrategyReq)
	if err != nil {
		t.Logf("Note: Could not create merge strategy: %v", err)
	}

	// Step 4: Set up Lotus mapping for Ivy consumption
	t.Log("Setting up Lotus mapping...")
	cleanupOldBindings(t, lotusClient)

	mappingDef := map[string]any{
		"name":        "Ivy Pipeline Mapping",
		"description": "Maps data for Ivy entity resolution",
		"key":         "ivy-pipeline-mapping",
		"source_fields": []map[string]any{
			{"id": "src_email", "path": "response_body.user.email", "type": "string"},
			{"id": "src_first_name", "path": "response_body.user.first_name", "type": "string"},
			{"id": "src_last_name", "path": "response_body.user.last_name", "type": "string"},
			{"id": "src_phone", "path": "response_body.user.phone", "type": "string"},
			{"id": "src_id", "path": "response_body.user.id", "type": "string"},
		},
		"target_fields": []map[string]any{
			{"id": "tgt_email", "path": "email", "type": "string"},
			{"id": "tgt_first_name", "path": "first_name", "type": "string"},
			{"id": "tgt_last_name", "path": "last_name", "type": "string"},
			{"id": "tgt_phone", "path": "phone", "type": "string"},
			{"id": "tgt_source_id", "path": "_source_id", "type": "string"},
		},
		"steps": []map[string]any{},
		"links": []map[string]any{
			{"source": map[string]any{"field_id": "src_email"}, "target": map[string]any{"field_id": "tgt_email"}},
			{"source": map[string]any{"field_id": "src_first_name"}, "target": map[string]any{"field_id": "tgt_first_name"}},
			{"source": map[string]any{"field_id": "src_last_name"}, "target": map[string]any{"field_id": "tgt_last_name"}},
			{"source": map[string]any{"field_id": "src_phone"}, "target": map[string]any{"field_id": "tgt_phone"}},
			{"source": map[string]any{"field_id": "src_id"}, "target": map[string]any{"field_id": "tgt_source_id"}},
		},
	}

	resp, err = lotusClient.Post("/api/v1/mappings/definitions", mappingDef)
	if err != nil {
		t.Fatalf("Failed to create mapping: %v", err)
	}
	if resp.StatusCode >= 300 {
		body, _ := ParseResponse[map[string]any](resp)
		t.Fatalf("Failed to create mapping: %d - %v", resp.StatusCode, body)
	}

	createdMapping, _ := ParseResponse[map[string]any](resp)
	mappingID := createdMapping["id"].(string)
	t.Logf("Created mapping: %s", mappingID)

	// Create binding
	binding := map[string]any{
		"name":         "Ivy Pipeline Binding",
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
	createdBinding, _ := ParseResponse[map[string]any](resp)
	bindingID := createdBinding["id"].(string)
	t.Logf("Created binding: %s", bindingID)

	// Step 5: Simulate Orchid messages for multiple users
	t.Log("Sending user data through pipeline...")
	publishTime := time.Now().Add(-1 * time.Second)

	// User 1 - First record
	user1v1 := CreateOrchidMessage(cfg.TestTenantID, "ivy-test-plan", map[string]any{
		"user": map[string]any{
			"id":         "crm-001",
			"email":      "john.doe@example.com",
			"first_name": "John",
			"last_name":  "Doe",
			"phone":      "",
		},
	})
	msgBytes, _ := json.Marshal(user1v1)
	kafkaHelper.ProduceMessage(ctx, cfg.OrchidInputTopic, cfg.TestTenantID, msgBytes, map[string]string{
		"tenant_id": cfg.TestTenantID,
	})

	// User 1 - Second record (same email, different source - should match)
	user1v2 := CreateOrchidMessage(cfg.TestTenantID, "ivy-test-plan", map[string]any{
		"user": map[string]any{
			"id":         "web-001",
			"email":      "john.doe@example.com",
			"first_name": "Johnny",
			"last_name":  "Doe",
			"phone":      "+1-555-1234",
		},
	})
	msgBytes, _ = json.Marshal(user1v2)
	kafkaHelper.ProduceMessage(ctx, cfg.OrchidInputTopic, cfg.TestTenantID, msgBytes, map[string]string{
		"tenant_id": cfg.TestTenantID,
	})

	// User 2 - Different person
	user2 := CreateOrchidMessage(cfg.TestTenantID, "ivy-test-plan", map[string]any{
		"user": map[string]any{
			"id":         "crm-002",
			"email":      "jane.smith@example.com",
			"first_name": "Jane",
			"last_name":  "Smith",
			"phone":      "+1-555-5678",
		},
	})
	msgBytes, _ = json.Marshal(user2)
	kafkaHelper.ProduceMessage(ctx, cfg.OrchidInputTopic, cfg.TestTenantID, msgBytes, map[string]string{
		"tenant_id": cfg.TestTenantID,
	})

	t.Log("Waiting for messages to flow through Lotus...")
	time.Sleep(3 * time.Second)

	// Step 6: Verify Lotus output
	messages, err := kafkaHelper.ConsumeMessagesAfter(
		ctx,
		cfg.LotusOutputTopic,
		fmt.Sprintf("ivy-e2e-%d", time.Now().UnixNano()),
		30*time.Second,
		3,
		publishTime,
	)

	if err != nil {
		t.Fatalf("Failed to consume Lotus output: %v", err)
	}

	t.Logf("Received %d messages from Lotus", len(messages))
	for i, msg := range messages {
		var output map[string]any
		json.Unmarshal(msg.Value, &output)
		t.Logf("Message %d: %v", i+1, output)
	}

	if len(messages) < 3 {
		t.Errorf("Expected at least 3 messages from Lotus, got %d", len(messages))
	}

	// Step 7: Check Ivy for match candidates (if Ivy is consuming)
	t.Log("Checking Ivy for match candidates...")
	time.Sleep(2 * time.Second)

	resp, err = ivyClient.Get("/api/v1/match-candidates?entity_type=person")
	if err == nil && resp != nil {
		candidates, _ := ParseResponse[[]map[string]any](resp)
		t.Logf("Ivy match candidates: %d found", len(candidates))
		for _, c := range candidates {
			t.Logf("  - Candidate: %v", c)
		}
	}

	// Cleanup
	t.Log("Cleaning up...")
	lotusClient.Delete(fmt.Sprintf("/api/v1/bindings/%s", bindingID))
	if matchRuleID != "" {
		ivyClient.Delete(fmt.Sprintf("/api/v1/match-rules/%s", matchRuleID))
	}

	t.Log("Full pipeline E2E test complete!")
}

// TestIvyMatchCandidateReview tests the match candidate review workflow
func TestIvyMatchCandidateReview(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := DefaultConfig()
	RequireService(t, cfg.IvyURL)

	ivyClient := NewHTTPClient(cfg.IvyURL, cfg.TestTenantID)

	// Get pending candidates
	resp, err := ivyClient.Get("/api/v1/match-candidates")
	if err != nil {
		t.Fatalf("Failed to get match candidates: %v", err)
	}

	candidates, _ := ParseResponse[[]map[string]any](resp)
	t.Logf("Found %d pending match candidates", len(candidates))

	// If there are candidates, test the approve/reject workflow
	if len(candidates) > 0 {
		candidate := candidates[0]
		entityAID := candidate["source_entity_id"]
		entityBID := candidate["candidate_entity_id"]

		t.Logf("Testing approval of candidate: %v <-> %v", entityAID, entityBID)

		// Test defer action
		resp, err = ivyClient.Post(
			fmt.Sprintf("/api/v1/match-candidates/defer?entity_a_id=%v&entity_b_id=%v", entityAID, entityBID),
			nil,
		)
		if err == nil && resp.StatusCode < 300 {
			t.Log("Successfully deferred candidate")
		}

		// Test approve action (in real test, would verify merge happened)
		resp, err = ivyClient.Post(
			fmt.Sprintf("/api/v1/match-candidates/approve?entity_a_id=%v&entity_b_id=%v", entityAID, entityBID),
			nil,
		)
		if err == nil && resp.StatusCode < 300 {
			t.Log("Successfully approved candidate")
		}
	}
}

// TestIvyDeletionStrategies tests the deletion strategy configuration
func TestIvyDeletionStrategies(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := DefaultConfig()
	RequireService(t, cfg.IvyURL)

	ivyClient := NewHTTPClient(cfg.IvyURL, cfg.TestTenantID)

	// Create various deletion strategies
	strategies := []map[string]any{
		{
			"integration": "crm",
			"entity_type": "person",
			"strategy":    "explicit",
			"enabled":     true,
		},
		{
			"integration":        "api",
			"entity_type":        "product",
			"strategy":           "execution_based",
			"grace_period_hours": 24,
			"enabled":            true,
		},
		{
			"integration":    "legacy",
			"entity_type":    "order",
			"strategy":       "staleness",
			"retention_days": 90,
			"enabled":        true,
		},
	}

	var createdIDs []string
	for _, strategy := range strategies {
		resp, err := ivyClient.Post("/api/v1/deletion-strategies", strategy)
		if err != nil {
			t.Logf("Note: Could not create deletion strategy: %v", err)
			continue
		}
		if resp.StatusCode < 300 {
			created, _ := ParseResponse[map[string]any](resp)
			if id, ok := created["id"].(string); ok {
				createdIDs = append(createdIDs, id)
				t.Logf("Created deletion strategy: %s", id)
			}
		}
	}

	// List and verify
	resp, err := ivyClient.Get("/api/v1/deletion-strategies")
	if err != nil {
		t.Fatalf("Failed to list deletion strategies: %v", err)
	}

	allStrategies, _ := ParseResponse[[]map[string]any](resp)
	t.Logf("Total deletion strategies: %d", len(allStrategies))

	// Cleanup
	for _, id := range createdIDs {
		ivyClient.Delete(fmt.Sprintf("/api/v1/deletion-strategies/%s", id))
	}
}

// TestIvyGraphQueries tests the graph query API
func TestIvyGraphQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := DefaultConfig()
	RequireService(t, cfg.IvyURL)

	ivyClient := NewHTTPClient(cfg.IvyURL, cfg.TestTenantID)

	// Test simple query
	queryReq := map[string]any{
		"query": "MATCH (n:person) WHERE n.tenant_id = $tenant_id RETURN n LIMIT 10",
		"params": map[string]any{
			"tenant_id": cfg.TestTenantID,
		},
	}

	resp, err := ivyClient.Post("/api/v1/graph/query", queryReq)
	if err != nil {
		t.Logf("Graph query not available: %v", err)
		return
	}

	if resp.StatusCode == 200 {
		result, _ := ParseResponse[map[string]any](resp)
		t.Logf("Graph query result: %v", result)
	} else {
		t.Logf("Graph query returned status %d", resp.StatusCode)
	}
}

// TestExecutionCompletedHandling tests how Ivy handles execution.completed events
func TestExecutionCompletedHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := DefaultConfig()
	RequireService(t, cfg.IvyURL)
	RequireService(t, cfg.OrchidURL)

	kafkaHelper := NewKafkaHelper(cfg.KafkaBrokers)
	ctx := context.Background()

	// Simulate an execution.completed event from Orchid
	executionEvent := map[string]any{
		"tenant_id":    cfg.TestTenantID,
		"plan_id":      "test-plan-001",
		"execution_id": fmt.Sprintf("exec-%d", time.Now().UnixNano()),
		"completed_at": time.Now().Format(time.RFC3339),
		"status":       "success",
		"entity_counts": map[string]int{
			"person":  10,
			"company": 5,
		},
	}

	msgBytes, _ := json.Marshal(executionEvent)
	err := kafkaHelper.ProduceMessage(
		ctx,
		cfg.OrchidEventsTopic, // execution events topic
		cfg.TestTenantID,
		msgBytes,
		map[string]string{
			"tenant_id":  cfg.TestTenantID,
			"event_type": "execution.completed",
		},
	)

	if err != nil {
		t.Logf("Note: Could not produce execution event: %v", err)
	} else {
		t.Log("Produced execution.completed event")

		// Wait for Ivy to process
		time.Sleep(2 * time.Second)

		// Verify Ivy processed the event (would need specific endpoint)
		t.Log("Execution event handling test complete")
	}
}

// TestIvyHealthCheck verifies Ivy health endpoints
func TestIvyHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := DefaultConfig()
	RequireService(t, cfg.IvyURL)

	ivyClient := NewHTTPClient(cfg.IvyURL, cfg.TestTenantID)

	endpoints := []string{
		"/api/v1/health",
		"/api/v1/health/live",
		"/api/v1/health/ready",
		"/api/v1/status",
	}

	for _, endpoint := range endpoints {
		resp, err := ivyClient.Get(endpoint)
		if err != nil {
			t.Errorf("Failed to reach %s: %v", endpoint, err)
			continue
		}

		if resp.StatusCode != 200 {
			t.Errorf("Expected 200 for %s, got %d", endpoint, resp.StatusCode)
		}
	}
}
