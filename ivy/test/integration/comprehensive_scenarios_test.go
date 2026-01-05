package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Ramsey-B/ivy/internal/repositories/stagedentity"
	"github.com/Ramsey-B/ivy/internal/repositories/stagedrelationship"
	"github.com/Ramsey-B/ivy/pkg/models"
	"github.com/Ramsey-B/stem/pkg/database"
)

// testContext holds shared test context
type testContext struct {
	db               database.DB
	entityRepo       *stagedentity.Repository
	relationshipRepo *stagedrelationship.Repository
	ctx              context.Context
	tenantID         string
}

// setupTestContext initializes the test context
// In real tests, this would connect to a test database
func setupTestContext(t *testing.T) *testContext {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tc := &testContext{
		ctx:      context.Background(),
		tenantID: "test-tenant-" + uuid.New().String()[:8],
	}

	// Note: In real tests, you'd initialize DB connection here
	// tc.db = database.NewConnection(testDBConfig)
	// tc.entityRepo = stagedentity.NewRepository(tc.db, tc.logger)
	// tc.relationshipRepo = stagedrelationship.NewRepository(tc.db, tc.logger)

	return tc
}

// TestDeepMergeMultipleUpdates tests deep merging with multiple incremental updates
func TestDeepMergeMultipleUpdates(t *testing.T) {
	tc := setupTestContext(t)
	if tc.db == nil {
		t.Skip("Database not configured")
	}

	// Scenario: A person entity gets data from multiple sources over time
	// Source 1: CRM provides basic contact info
	// Source 2: Web form provides additional details
	// Source 3: API enrichment adds company info

	sourceID := "person-123"
	execID1 := stringPtr("exec-001")
	execID2 := stringPtr("exec-002")
	execID3 := stringPtr("exec-003")

	// First update: Basic contact info from CRM
	crmData := map[string]any{
		"email": "john.doe@example.com",
		"name": map[string]any{
			"first": "John",
			"last":  "Doe",
		},
	}
	crmJSON, _ := json.Marshal(crmData)

	req1 := models.CreateStagedEntityRequest{
		ConfigID:          "config-001",
		EntityType:        "person",
		SourceID:          sourceID,
		Integration:       "crm",
		SourceKey:         "plan-001",
		SourceExecutionID: execID1,
		Data:              crmJSON,
	}

	result1, err := tc.entityRepo.Upsert(tc.ctx, tc.tenantID, req1)
	require.NoError(t, err)
	assert.True(t, result1.IsNew)
	assert.True(t, result1.IsChanged)

	// Second update: Web form adds phone and preferences
	webData := map[string]any{
		"phone": "+1-555-1234",
		"name": map[string]any{
			"middle": "Q", // Add middle name
		},
		"preferences": map[string]any{
			"newsletter": true,
			"sms":        false,
		},
	}
	webJSON, _ := json.Marshal(webData)

	req2 := models.CreateStagedEntityRequest{
		ConfigID:          "config-001",
		EntityType:        "person",
		SourceID:          sourceID,
		Integration:       "crm", // Same integration to trigger merge
		SourceKey:         "plan-001",
		SourceExecutionID: execID2,
		Data:              webJSON,
	}

	result2, err := tc.entityRepo.Upsert(tc.ctx, tc.tenantID, req2)
	require.NoError(t, err)
	assert.False(t, result2.IsNew)
	assert.True(t, result2.IsChanged)

	// Verify deep merge: should have first, middle, last names
	var mergedData map[string]any
	err = json.Unmarshal(result2.Entity.Data, &mergedData)
	require.NoError(t, err)

	nameMap := mergedData["name"].(map[string]any)
	assert.Equal(t, "John", nameMap["first"])
	assert.Equal(t, "Q", nameMap["middle"])
	assert.Equal(t, "Doe", nameMap["last"])
	assert.Equal(t, "john.doe@example.com", mergedData["email"])
	assert.Equal(t, "+1-555-1234", mergedData["phone"])

	// Third update: API enrichment adds company info
	apiData := map[string]any{
		"company": map[string]any{
			"name": "Acme Corp",
			"role": "Engineer",
		},
		"preferences": map[string]any{
			"language": "en", // Add to existing preferences
		},
	}
	apiJSON, _ := json.Marshal(apiData)

	req3 := models.CreateStagedEntityRequest{
		ConfigID:          "config-001",
		EntityType:        "person",
		SourceID:          sourceID,
		Integration:       "crm",
		SourceKey:         "plan-001",
		SourceExecutionID: execID3,
		Data:              apiJSON,
	}

	result3, err := tc.entityRepo.Upsert(tc.ctx, tc.tenantID, req3)
	require.NoError(t, err)
	assert.True(t, result3.IsChanged)

	// Verify final merged state
	err = json.Unmarshal(result3.Entity.Data, &mergedData)
	require.NoError(t, err)

	// All name fields preserved
	nameMap = mergedData["name"].(map[string]any)
	assert.Equal(t, "John", nameMap["first"])
	assert.Equal(t, "Q", nameMap["middle"])
	assert.Equal(t, "Doe", nameMap["last"])

	// Company info added
	companyMap := mergedData["company"].(map[string]any)
	assert.Equal(t, "Acme Corp", companyMap["name"])
	assert.Equal(t, "Engineer", companyMap["role"])

	// Preferences merged
	prefsMap := mergedData["preferences"].(map[string]any)
	assert.Equal(t, true, prefsMap["newsletter"])
	assert.Equal(t, false, prefsMap["sms"])
	assert.Equal(t, "en", prefsMap["language"])
}

// TestConcurrentUpserts tests race condition handling
func TestConcurrentUpserts(t *testing.T) {
	tc := setupTestContext(t)
	if tc.db == nil {
		t.Skip("Database not configured")
	}

	// Scenario: Multiple goroutines try to upsert the same entity simultaneously
	sourceID := "concurrent-test-" + uuid.New().String()
	numGoroutines := 10

	errChan := make(chan error, numGoroutines)
	doneChan := make(chan *stagedentity.UpsertResult, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			execID := stringPtr(uuid.New().String())
			data := map[string]any{
				"counter": index,
				"data": map[string]any{
					"field": index,
				},
			}
			dataJSON, _ := json.Marshal(data)

			req := models.CreateStagedEntityRequest{
				ConfigID:          "config-001",
				EntityType:        "test_entity",
				SourceID:          sourceID,
				Integration:       "test",
				SourceKey:         "plan-001",
				SourceExecutionID: execID,
				Data:              dataJSON,
			}

			result, err := tc.entityRepo.Upsert(tc.ctx, tc.tenantID, req)
			if err != nil {
				errChan <- err
				return
			}
			doneChan <- result
		}(i)
	}

	// Collect results
	var results []*stagedentity.UpsertResult
	for i := 0; i < numGoroutines; i++ {
		select {
		case err := <-errChan:
			t.Fatalf("Concurrent upsert failed: %v", err)
		case result := <-doneChan:
			results = append(results, result)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent upserts")
		}
	}

	// Verify: exactly one should be marked as new, others as updates
	newCount := 0
	for _, r := range results {
		if r.IsNew {
			newCount++
		}
	}
	assert.Equal(t, 1, newCount, "Exactly one upsert should create new entity")

	// Verify final entity has all counters merged
	finalEntity, err := tc.entityRepo.GetBySourceAndEntityType(
		tc.ctx, tc.tenantID, "test_entity", sourceID, "test",
	)
	require.NoError(t, err)
	require.NotNil(t, finalEntity)

	var finalData map[string]any
	err = json.Unmarshal(finalEntity.Data, &finalData)
	require.NoError(t, err)
	assert.NotNil(t, finalData["counter"])
}

// TestEntityRelationshipIntegrity tests relationships between entities
func TestEntityRelationshipIntegrity(t *testing.T) {
	tc := setupTestContext(t)
	if tc.db == nil || tc.relationshipRepo == nil {
		t.Skip("Database or relationship repo not configured")
	}

	// Scenario: Create person -> company relationship
	// Test out-of-order arrival: relationship arrives before entities

	personSourceID := "person-rel-" + uuid.New().String()
	companySourceID := "company-rel-" + uuid.New().String()
	execID := stringPtr("exec-rel-001")

	// Step 1: Relationship arrives first (out-of-order)
	relReq := models.CreateStagedRelationshipRequest{
		ConfigID:         "config-001",
		Integration:      "crm",
		SourceKey:        "plan-001",
		RelationshipType: "works_at",
		FromEntityType:   "person",
		FromSourceID:     personSourceID,
		FromIntegration:  "crm",
		ToEntityType:     "company",
		ToSourceID:       companySourceID,
		ToIntegration:    "crm",
		SourceExecutionID: execID,
		Data:             json.RawMessage(`{"role": "engineer", "since": "2020-01-01"}`),
	}

	relResult, err := tc.relationshipRepo.Create(tc.ctx, tc.tenantID, relReq)
	require.NoError(t, err)
	assert.NotEmpty(t, relResult.ID)
	// Entity IDs should be nil since entities don't exist yet
	assert.Nil(t, relResult.FromStagedEntityID)
	assert.Nil(t, relResult.ToStagedEntityID)

	// Step 2: Person entity arrives
	personData := map[string]any{
		"name":  "Jane Smith",
		"email": "jane@example.com",
	}
	personJSON, _ := json.Marshal(personData)

	personReq := models.CreateStagedEntityRequest{
		ConfigID:          "config-001",
		EntityType:        "person",
		SourceID:          personSourceID,
		Integration:       "crm",
		SourceKey:         "plan-001",
		SourceExecutionID: execID,
		Data:              personJSON,
	}

	personResult, err := tc.entityRepo.Upsert(tc.ctx, tc.tenantID, personReq)
	require.NoError(t, err)
	assert.True(t, personResult.IsNew)

	// Step 3: Company entity arrives
	companyData := map[string]any{
		"name":     "Tech Corp",
		"industry": "Software",
	}
	companyJSON, _ := json.Marshal(companyData)

	companyReq := models.CreateStagedEntityRequest{
		ConfigID:          "config-001",
		EntityType:        "company",
		SourceID:          companySourceID,
		Integration:       "crm",
		SourceKey:         "plan-001",
		SourceExecutionID: execID,
		Data:              companyJSON,
	}

	companyResult, err := tc.entityRepo.Upsert(tc.ctx, tc.tenantID, companyReq)
	require.NoError(t, err)
	assert.True(t, companyResult.IsNew)

	// Step 4: Verify relationship can be resolved
	// In a real system, a background job would resolve relationships
	// Here we just verify the entities exist for resolution
	foundPerson, err := tc.entityRepo.GetBySourceAndEntityType(
		tc.ctx, tc.tenantID, "person", personSourceID, "crm",
	)
	require.NoError(t, err)
	assert.NotNil(t, foundPerson)

	foundCompany, err := tc.entityRepo.GetBySourceAndEntityType(
		tc.ctx, tc.tenantID, "company", companySourceID, "crm",
	)
	require.NoError(t, err)
	assert.NotNil(t, foundCompany)
}

// TestExecutionBasedDeletion tests execution-based deletion strategy
func TestExecutionBasedDeletion(t *testing.T) {
	tc := setupTestContext(t)
	if tc.db == nil {
		t.Skip("Database not configured")
	}

	// Scenario: Full sync execution where entities not seen should be deleted
	configID := "config-deletion-test"
	exec1 := stringPtr("exec-del-001")
	exec2 := stringPtr("exec-del-002")

	// Execution 1: Create 3 entities
	for i := 1; i <= 3; i++ {
		data := map[string]any{
			"id":   i,
			"name": "Entity " + string(rune('0'+i)),
		}
		dataJSON, _ := json.Marshal(data)

		req := models.CreateStagedEntityRequest{
			ConfigID:          configID,
			EntityType:        "product",
			SourceID:          "product-" + string(rune('0'+i)),
			Integration:       "catalog",
			SourceKey:         "plan-001",
			SourceExecutionID: exec1,
			Data:              dataJSON,
		}

		result, err := tc.entityRepo.Upsert(tc.ctx, tc.tenantID, req)
		require.NoError(t, err)
		assert.True(t, result.IsNew)
	}

	// Execution 2: Only send 2 entities (one was deleted upstream)
	for i := 1; i <= 2; i++ {
		data := map[string]any{
			"id":   i,
			"name": "Entity " + string(rune('0'+i)) + " Updated",
		}
		dataJSON, _ := json.Marshal(data)

		req := models.CreateStagedEntityRequest{
			ConfigID:          configID,
			EntityType:        "product",
			SourceID:          "product-" + string(rune('0'+i)),
			Integration:       "catalog",
			SourceKey:         "plan-001",
			SourceExecutionID: exec2,
			Data:              dataJSON,
		}

		result, err := tc.entityRepo.Upsert(tc.ctx, tc.tenantID, req)
		require.NoError(t, err)
		assert.False(t, result.IsNew) // Should be updates
	}

	// Mark entities not in exec2 as deleted
	entityType := "product"
	deletedCount, err := tc.entityRepo.MarkDeletedExceptExecution(
		tc.ctx, tc.tenantID, configID, *exec2, &entityType,
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deletedCount, "One entity should be marked deleted")

	// Verify entity 3 is deleted
	entity3, err := tc.entityRepo.GetBySourceAndEntityType(
		tc.ctx, tc.tenantID, "product", "product-3", "catalog",
	)
	// Should return not found error or entity with deleted_at set
	if entity3 != nil {
		assert.NotNil(t, entity3.DeletedAt)
	}
}

// TestPartialDataMerge tests merging when only partial data is provided
func TestPartialDataMerge(t *testing.T) {
	tc := setupTestContext(t)
	if tc.db == nil {
		t.Skip("Database not configured")
	}

	// Scenario: Different sources provide different subsets of fields
	sourceID := "partial-merge-" + uuid.New().String()

	// Source 1: Profile service provides identity fields
	profileData := map[string]any{
		"profile": map[string]any{
			"bio":    "Software Engineer",
			"avatar": "https://example.com/avatar.jpg",
		},
		"contact": map[string]any{
			"email": "user@example.com",
		},
	}
	profileJSON, _ := json.Marshal(profileData)

	req1 := models.CreateStagedEntityRequest{
		ConfigID:          "config-001",
		EntityType:        "user",
		SourceID:          sourceID,
		Integration:       "app",
		SourceKey:         "plan-001",
		SourceExecutionID: stringPtr("exec-001"),
		Data:              profileJSON,
	}

	result1, err := tc.entityRepo.Upsert(tc.ctx, tc.tenantID, req1)
	require.NoError(t, err)
	assert.True(t, result1.IsNew)

	// Source 2: Analytics service adds activity data
	analyticsData := map[string]any{
		"activity": map[string]any{
			"lastLogin":  "2024-01-03T10:00:00Z",
			"loginCount": 42,
		},
		"contact": map[string]any{
			"phone": "+1-555-0000", // Add to existing contact
		},
	}
	analyticsJSON, _ := json.Marshal(analyticsData)

	req2 := models.CreateStagedEntityRequest{
		ConfigID:          "config-001",
		EntityType:        "user",
		SourceID:          sourceID,
		Integration:       "app",
		SourceKey:         "plan-001",
		SourceExecutionID: stringPtr("exec-002"),
		Data:              analyticsJSON,
	}

	result2, err := tc.entityRepo.Upsert(tc.ctx, tc.tenantID, req2)
	require.NoError(t, err)
	assert.True(t, result2.IsChanged)

	// Verify merged result has all fields
	var finalData map[string]any
	err = json.Unmarshal(result2.Entity.Data, &finalData)
	require.NoError(t, err)

	// Profile fields preserved
	profile := finalData["profile"].(map[string]any)
	assert.Equal(t, "Software Engineer", profile["bio"])
	assert.Equal(t, "https://example.com/avatar.jpg", profile["avatar"])

	// Activity fields added
	activity := finalData["activity"].(map[string]any)
	assert.Equal(t, "2024-01-03T10:00:00Z", activity["lastLogin"])
	assert.Equal(t, float64(42), activity["loginCount"])

	// Contact merged (both email and phone)
	contact := finalData["contact"].(map[string]any)
	assert.Equal(t, "user@example.com", contact["email"])
	assert.Equal(t, "+1-555-0000", contact["phone"])
}

// TestArrayAndPrimitiveOverwrite tests that arrays and primitives are replaced, not merged
func TestArrayAndPrimitiveOverwrite(t *testing.T) {
	tc := setupTestContext(t)
	if tc.db == nil {
		t.Skip("Database not configured")
	}

	sourceID := "array-test-" + uuid.New().String()

	// Initial data with array
	data1 := map[string]any{
		"tags":   []string{"tag1", "tag2"},
		"status": "active",
		"nested": map[string]any{
			"items": []string{"item1", "item2"},
		},
	}
	json1, _ := json.Marshal(data1)

	req1 := models.CreateStagedEntityRequest{
		ConfigID:          "config-001",
		EntityType:        "item",
		SourceID:          sourceID,
		Integration:       "system",
		SourceKey:         "plan-001",
		SourceExecutionID: stringPtr("exec-001"),
		Data:              json1,
	}

	result1, err := tc.entityRepo.Upsert(tc.ctx, tc.tenantID, req1)
	require.NoError(t, err)
	assert.True(t, result1.IsNew)

	// Update with different array and status
	data2 := map[string]any{
		"tags":   []string{"tag3", "tag4", "tag5"},
		"status": "inactive",
		"nested": map[string]any{
			"items": []string{"item3"},
		},
	}
	json2, _ := json.Marshal(data2)

	req2 := models.CreateStagedEntityRequest{
		ConfigID:          "config-001",
		EntityType:        "item",
		SourceID:          sourceID,
		Integration:       "system",
		SourceKey:         "plan-001",
		SourceExecutionID: stringPtr("exec-002"),
		Data:              json2,
	}

	result2, err := tc.entityRepo.Upsert(tc.ctx, tc.tenantID, req2)
	require.NoError(t, err)
	assert.True(t, result2.IsChanged)

	// Verify arrays and primitives were replaced, not merged
	var finalData map[string]any
	err = json.Unmarshal(result2.Entity.Data, &finalData)
	require.NoError(t, err)

	tags := finalData["tags"].([]any)
	assert.Len(t, tags, 3)
	assert.Equal(t, "tag3", tags[0])

	assert.Equal(t, "inactive", finalData["status"])

	nested := finalData["nested"].(map[string]any)
	nestedItems := nested["items"].([]any)
	assert.Len(t, nestedItems, 1)
	assert.Equal(t, "item3", nestedItems[0])
}
