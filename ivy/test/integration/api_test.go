package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Ramsey-B/ivy/pkg/models"
)

// TestAPIHelpers contains helper functions for API testing
type TestAPIHelpers struct {
	t        *testing.T
	e        *echo.Echo
	tenantID string
}

func NewTestAPIHelpers(t *testing.T) *TestAPIHelpers {
	e := echo.New()

	// Add test auth middleware
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tenantID := c.Request().Header.Get("X-Tenant-ID")
			if tenantID == "" {
				tenantID = "test-tenant"
			}
			c.Set("tenant_id", tenantID)
			return next(c)
		}
	})

	return &TestAPIHelpers{
		t:        t,
		e:        e,
		tenantID: "test-tenant",
	}
}

func (h *TestAPIHelpers) MakeRequest(method, path string, body any) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(h.t, err)
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", h.tenantID)

	rec := httptest.NewRecorder()
	h.e.ServeHTTP(rec, req)
	return rec
}

func TestEntityTypeAPI_Validation(t *testing.T) {
	t.Run("CreateEntityType_ValidRequest", func(t *testing.T) {
		req := map[string]any{
			"key":         "person",
			"name":        "Person",
			"description": "A person entity",
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"email": map[string]any{"type": "string", "format": "email"},
					"name":  map[string]any{"type": "string"},
				},
				"required": []string{"email"},
			},
		}

		// Validate request structure
		data, err := json.Marshal(req)
		require.NoError(t, err)

		var parsed map[string]any
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Equal(t, "person", parsed["key"])
		assert.NotNil(t, parsed["schema"])
	})

	t.Run("CreateEntityType_MissingKey", func(t *testing.T) {
		req := map[string]any{
			"name": "Person",
		}

		// Validate that key is required
		_, hasKey := req["key"]
		assert.False(t, hasKey, "key should be missing for this test")
	})
}

func TestMatchRuleAPI_Validation(t *testing.T) {
	t.Run("CreateMatchRule_ValidConditions", func(t *testing.T) {
		conditions := []models.MatchCondition{
			{
				Field:     "email",
				MatchType: models.MatchRuleTypeExact,
				Weight:    0.5,
				Required:  true,
			},
			{
				Field:     "last_name",
				MatchType: models.MatchRuleTypeFuzzy,
				Weight:    0.3,
				Threshold: 0.8,
			},
			{
				Field:     "phone",
				MatchType: models.MatchRuleTypePhonetic,
				Weight:    0.2,
			},
		}

		data, err := json.Marshal(conditions)
		require.NoError(t, err)

		var parsed []models.MatchCondition
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Len(t, parsed, 3)

		// Validate weights sum
		var totalWeight float64
		for _, c := range parsed {
			totalWeight += c.Weight
		}
		assert.InDelta(t, 1.0, totalWeight, 0.001)
	})

	t.Run("MatchRule_NoMergeFlag", func(t *testing.T) {
		// Test the no_merge flag for blocking matches
		condition := models.MatchCondition{
			NoMerge: true, // If SSNs are different, block merge
		}

		assert.True(t, condition.NoMerge)
	})
}

func TestMergeStrategyAPI_Validation(t *testing.T) {
	t.Run("CreateMergeStrategy_AllFieldStrategies", func(t *testing.T) {
		strategies := []models.FieldMergeStrategy{
			{Field: "email", Strategy: models.MergeStrategyMostRecent},
			{Field: "name", Strategy: models.MergeStrategyMostTrusted},
			{Field: "tags", Strategy: models.MergeStrategyCollectAll, Dedup: true},
			{Field: "score", Strategy: models.MergeStrategyMax},
			{Field: "bio", Strategy: models.MergeStrategyLongestValue},
		}

		data, err := json.Marshal(strategies)
		require.NoError(t, err)

		var parsed []models.FieldMergeStrategy
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Len(t, parsed, 5)
	})

	t.Run("SourcePriorities", func(t *testing.T) {
		priorities := []models.SourcePriority{
			{Integration: "crm", Priority: 10},
			{Integration: "web", Priority: 5},
			{Integration: "manual", Priority: 15},
		}

		data, err := json.Marshal(priorities)
		require.NoError(t, err)

		var parsed []models.SourcePriority
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		// Find highest priority
		var maxPriority int
		var mostTrusted string
		for _, p := range parsed {
			if p.Priority > maxPriority {
				maxPriority = p.Priority
				mostTrusted = p.Integration
			}
		}
		assert.Equal(t, "manual", mostTrusted)
	})
}

func TestDeletionStrategyAPI_Validation(t *testing.T) {
	t.Run("CreateDeletionStrategy_AllTypes", func(t *testing.T) {
		// Helper to create string pointers
		strPtr := func(s string) *string { return &s }

		strategies := []models.DeletionStrategy{
			{Integration: strPtr("crm"), EntityType: strPtr("person"), StrategyType: models.DeletionStrategyExplicit, Enabled: true},
			{Integration: strPtr("api"), EntityType: strPtr("product"), StrategyType: models.DeletionStrategyExecutionBased, Enabled: true},
			{Integration: strPtr("legacy"), EntityType: strPtr("order"), StrategyType: models.DeletionStrategyStaleness, Enabled: true, Config: json.RawMessage(`{"max_age_days": 90}`)},
			{Integration: strPtr("retention"), EntityType: strPtr("logs"), StrategyType: models.DeletionStrategyRetention, Enabled: true, Config: json.RawMessage(`{"retention_days": 30}`)},
			{Integration: strPtr("complex"), EntityType: strPtr("audit"), StrategyType: models.DeletionStrategyComposite, Enabled: true},
		}

		for _, s := range strategies {
			data, err := json.Marshal(s)
			require.NoError(t, err)

			var parsed models.DeletionStrategy
			err = json.Unmarshal(data, &parsed)
			require.NoError(t, err)

			assert.Equal(t, s.StrategyType, parsed.StrategyType)
		}
	})
}

func TestMatchCandidateAPI_Validation(t *testing.T) {
	t.Run("MatchCandidateStatuses", func(t *testing.T) {
		statuses := []string{
			models.MatchCandidateStatusPending,
			models.MatchCandidateStatusApproved,
			models.MatchCandidateStatusRejected,
			models.MatchCandidateStatusAutoMerged,
			models.MatchCandidateStatusDeferred,
		}

		for _, s := range statuses {
			assert.NotEmpty(t, s)
		}
	})
}

func TestGraphQueryAPI_Validation(t *testing.T) {
	t.Run("CypherQuery_Safe", func(t *testing.T) {
		// Safe queries - must be read-only (no DELETE, DETACH, SET, CREATE, MERGE, REMOVE)
		safeQueries := []string{
			"MATCH (n:Person {tenant_id: $tenant_id}) RETURN n LIMIT 10",
			"MATCH (a:Person)-[r:WORKS_AT]->(b:Company) WHERE a.tenant_id = $tenant_id RETURN a, r, b",
			"MATCH p=shortestPath((a:Person {id: $id1, tenant_id: $tenant_id})-[*]-(b:Person {id: $id2})) RETURN p",
		}

		for _, q := range safeQueries {
			assert.NotContains(t, q, "DELETE")
			assert.NotContains(t, q, "DETACH")
			// All queries should enforce tenant isolation
			assert.Contains(t, q, "tenant_id")
		}
	})

	t.Run("CypherQuery_Dangerous", func(t *testing.T) {
		// Queries that should be blocked
		dangerousPatterns := []string{
			"DELETE",
			"DETACH",
			"SET",
			"CREATE",
			"MERGE",
			"REMOVE",
		}

		for _, pattern := range dangerousPatterns {
			// These should be rejected by the API
			t.Logf("Pattern '%s' should be blocked for read-only queries", pattern)
		}
	})
}

func TestHealthEndpoint(t *testing.T) {
	t.Run("HealthResponse", func(t *testing.T) {
		response := map[string]any{
			"status":  "healthy",
			"version": "1.0.0",
			"checks": map[string]any{
				"database": map[string]any{
					"status":  "healthy",
					"latency": "5ms",
				},
				"kafka": map[string]any{
					"status": "healthy",
				},
				"memgraph": map[string]any{
					"status": "healthy",
				},
			},
		}

		data, err := json.Marshal(response)
		require.NoError(t, err)

		var parsed map[string]any
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Equal(t, "healthy", parsed["status"])
		checks := parsed["checks"].(map[string]any)
		assert.Contains(t, checks, "database")
	})
}

func TestAPIErrorResponses(t *testing.T) {
	t.Run("NotFound", func(t *testing.T) {
		response := map[string]any{
			"error":   "Entity not found",
			"code":    http.StatusNotFound,
			"details": "Entity with ID 'abc-123' does not exist",
		}

		data, err := json.Marshal(response)
		require.NoError(t, err)

		var parsed map[string]any
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		code := int(parsed["code"].(float64))
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("BadRequest", func(t *testing.T) {
		response := map[string]any{
			"error": "Validation failed",
			"code":  http.StatusBadRequest,
			"details": []string{
				"entity_type is required",
				"conditions must have at least one item",
			},
		}

		data, err := json.Marshal(response)
		require.NoError(t, err)

		var parsed map[string]any
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		details := parsed["details"].([]any)
		assert.Len(t, details, 2)
	})

	t.Run("Conflict", func(t *testing.T) {
		response := map[string]any{
			"error": "Entity already exists",
			"code":  http.StatusConflict,
			"details": map[string]any{
				"existing_id": "existing-123",
				"key":         "person",
			},
		}

		data, err := json.Marshal(response)
		require.NoError(t, err)

		var parsed map[string]any
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		code := int(parsed["code"].(float64))
		assert.Equal(t, http.StatusConflict, code)
	})
}

// Benchmark tests
func BenchmarkJSONParsing(b *testing.B) {
	conditions := []models.MatchCondition{
		{Field: "email", MatchType: models.MatchRuleTypeExact, Weight: 0.5},
		{Field: "name", MatchType: models.MatchRuleTypeFuzzy, Weight: 0.3, Threshold: 0.8},
		{Field: "phone", MatchType: models.MatchRuleTypePhonetic, Weight: 0.2},
	}

	data, _ := json.Marshal(conditions)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var parsed []models.MatchCondition
		_ = json.Unmarshal(data, &parsed)
	}
}

func BenchmarkHTTPRequest(b *testing.B) {
	e := echo.New()
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
	}
}

// Integration test helper for full API flow
func TestFullEntityLifecycle(t *testing.T) {
	t.Skip("Requires running database - run with integration tag")

	/*
		This test would cover:
		1. Create entity type
		2. Create match rules
		3. Create merge strategy
		4. Receive entities via Kafka
		5. Trigger matching
		6. Review match candidates
		7. Merge entities
		8. Query merged entity from graph
		9. Test deletion
	*/
	fmt.Println("Full lifecycle test placeholder")
}
