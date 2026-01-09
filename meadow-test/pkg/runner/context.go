package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TestContext holds the state and configuration for test execution
type TestContext struct {
	ctx context.Context

	// Service URLs
	OrchidURL string
	LotusURL  string
	IvyURL    string
	MocksURL  string

	// Kafka configuration
	KafkaBrokers []string

	// Test configuration
	TestTenant string
	Verbose    bool

	// Variable storage (from save_as)
	vars map[string]interface{}

	// Fixtures loaded from helpers
	fixtures map[string]interface{}

	// Templates loaded from helpers
	templates map[string]interface{}

	// HTTP client
	httpClient *http.Client
}

// NewTestContext creates a new test context with a unique tenant ID
func NewTestContext(ctx context.Context, config Config) *TestContext {
	// Generate a unique tenant ID for this test to ensure isolation
	testTenant := uuid.New().String()
	if config.Verbose {
		fmt.Printf("  [TENANT] Using unique tenant ID: %s\n", testTenant)
	}

	return &TestContext{
		ctx:          ctx,
		OrchidURL:    config.OrchidURL,
		LotusURL:     config.LotusURL,
		IvyURL:       config.IvyURL,
		MocksURL:     config.MocksURL,
		KafkaBrokers: config.KafkaBrokers,
		TestTenant:   testTenant,
		Verbose:      config.Verbose,
		vars:         make(map[string]interface{}),
		fixtures:     make(map[string]interface{}),
		templates:    make(map[string]interface{}),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Set stores a variable value
func (tc *TestContext) Set(key string, value interface{}) {
	tc.vars[key] = value
	if tc.Verbose {
		fmt.Printf("  [VAR] %s = %v\n", key, value)
	}
}

// Get retrieves a variable value
func (tc *TestContext) Get(key string) (interface{}, bool) {
	val, ok := tc.vars[key]
	return val, ok
}

// Interpolate replaces {{variable}} placeholders with actual values
func (tc *TestContext) Interpolate(input interface{}) interface{} {
	switch v := input.(type) {
	case string:
		return tc.interpolateString(v)
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			result[key] = tc.Interpolate(val)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = tc.Interpolate(val)
		}
		return result
	default:
		return v
	}
}

var varPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

func (tc *TestContext) interpolateString(s string) string {
	return varPattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract variable name (remove {{ and }})
		varName := strings.TrimSpace(match[2 : len(match)-2])

		// Check for fixture reference
		if strings.HasPrefix(varName, "fixture:") {
			fixtureName := strings.TrimPrefix(varName, "fixture:")
			if fixture, ok := tc.fixtures[fixtureName]; ok {
				// If fixture is a string, return it; otherwise JSON encode
				if str, ok := fixture.(string); ok {
					return str
				}
				jsonBytes, _ := json.Marshal(fixture)
				return string(jsonBytes)
			}
			return match // Keep original if fixture not found
		}

		// Built-in variables
		switch varName {
		case "orchid_url":
			return tc.OrchidURL
		case "lotus_url":
			return tc.LotusURL
		case "ivy_url":
			return tc.IvyURL
		case "mock_api_url", "mocks_url":
			return tc.MocksURL
		case "kafka_brokers":
			return strings.Join(tc.KafkaBrokers, ",")
		case "test_tenant":
			return tc.TestTenant
		case "timestamp":
			return fmt.Sprintf("%d", time.Now().Unix())
		case "uuid":
			return uuid.New().String()
		}

		// Check environment variables
		if envVal := os.Getenv(varName); envVal != "" {
			return envVal
		}

		// Check stored variables (support nested paths like "response.id")
		val := tc.resolveNestedPath(varName)
		if val != nil {
			// Convert to string
			switch v := val.(type) {
			case string:
				return v
			case int, int64, float64, bool:
				return fmt.Sprintf("%v", v)
			default:
				// JSON encode complex types
				jsonBytes, _ := json.Marshal(v)
				return string(jsonBytes)
			}
		}

		// Variable not found, keep original
		return match
	})
}

// LoadFixtures loads fixtures from a YAML file
func (tc *TestContext) LoadFixtures(fixtures map[string]interface{}) {
	for name, data := range fixtures {
		tc.fixtures[name] = data
		if tc.Verbose {
			fmt.Printf("  [FIXTURE] Loaded: %s\n", name)
		}
	}
}

// LoadTemplates loads templates from a YAML file
func (tc *TestContext) LoadTemplates(templates map[string]interface{}) {
	for name, data := range templates {
		tc.templates[name] = data
		if tc.Verbose {
			fmt.Printf("  [TEMPLATE] Loaded: %s\n", name)
		}
	}
}

// GetTemplate retrieves a template by name
func (tc *TestContext) GetTemplate(name string) (map[string]interface{}, bool) {
	tmpl, ok := tc.templates[name]
	if !ok {
		return nil, false
	}
	if m, ok := tmpl.(map[string]interface{}); ok {
		return m, true
	}
	return nil, false
}

// HTTPRequest makes an HTTP request with automatic service URL resolution
func (tc *TestContext) HTTPRequest(method, serviceOrURL, path string, headers map[string]string, body interface{}) (*http.Response, error) {
	// Resolve service name to URL
	var baseURL string
	switch strings.ToLower(serviceOrURL) {
	case "orchid":
		baseURL = tc.OrchidURL
	case "lotus":
		baseURL = tc.LotusURL
	case "ivy":
		baseURL = tc.IvyURL
	case "mocks", "mock", "mock_api":
		baseURL = tc.MocksURL
	default:
		// Assume it's a full URL
		baseURL = serviceOrURL
	}

	// Build URL
	url := baseURL
	if path != "" {
		url = strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
	}

	// Interpolate URL
	url = tc.interpolateString(url)

	// Prepare body
	var bodyReader *bytes.Reader
	if body != nil {
		// Interpolate body
		interpolatedBody := tc.Interpolate(body)

		jsonBytes, err := json.Marshal(interpolatedBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	// Create request
	req, err := http.NewRequestWithContext(tc.ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tc.TestTenant)
	req.Header.Set("X-User-ID", "meadow-test-user")

	for key, val := range headers {
		interpolatedVal := tc.interpolateString(val)
		req.Header.Set(key, interpolatedVal)
	}

	if tc.Verbose {
		fmt.Printf("  [HTTP] %s %s\n", method, url)
		if body != nil {
			bodyJSON, _ := json.MarshalIndent(tc.Interpolate(body), "    ", "  ")
			fmt.Printf("    Body: %s\n", string(bodyJSON))
		}
	}

	// Execute request
	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if tc.Verbose {
		fmt.Printf("    Status: %d %s\n", resp.StatusCode, resp.Status)
	}

	return resp, nil
}

// Log prints a log message if verbose mode is enabled
func (tc *TestContext) Log(format string, args ...interface{}) {
	if tc.Verbose {
		fmt.Printf("  "+format+"\n", args...)
	}
}

// Error prints an error message
func (tc *TestContext) Error(format string, args ...interface{}) {
	fmt.Printf("  [ERROR] "+format+"\n", args...)
}

// resolveNestedPath resolves a variable path like "response.id" or "response.data.name"
func (tc *TestContext) resolveNestedPath(path string) interface{} {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil
	}

	// Get the root variable
	val, found := tc.vars[parts[0]]
	if !found {
		return nil
	}

	// If only one part, return directly
	if len(parts) == 1 {
		return val
	}

	// Navigate nested paths
	for i := 1; i < len(parts); i++ {
		if val == nil {
			return nil
		}

		switch v := val.(type) {
		case map[string]interface{}:
			val = v[parts[i]]
		default:
			// Can't navigate further
			return nil
		}
	}

	return val
}

// CleanupTenant deletes all data for the test's tenant from all services
// This ensures test isolation even if the test fails or cleanup steps don't run
func (tc *TestContext) CleanupTenant() {
	if tc.Verbose {
		fmt.Printf("  [CLEANUP] Deleting tenant data: %s\n", tc.TestTenant)
	}

	// Clean up Orchid
	tc.deleteTenantFromService(tc.OrchidURL, "orchid")

	// Clean up Lotus
	tc.deleteTenantFromService(tc.LotusURL, "lotus")

	// Clean up Ivy
	tc.deleteTenantFromService(tc.IvyURL, "ivy")

	// Note: Mock endpoints are NOT cleared here because:
	// 1. Each test uses unique mock paths (e.g., /mock/foo-{{uuid}})
	// 2. Clearing all mocks would break parallel test execution
	// 3. Tests should clean up their own mocks in their cleanup section if needed
}

func (tc *TestContext) deleteTenantFromService(serviceURL, serviceName string) {
	if serviceURL == "" {
		return
	}

	url := fmt.Sprintf("%s/api/v1/tenant/%s", serviceURL, tc.TestTenant)
	req, err := http.NewRequestWithContext(tc.ctx, "DELETE", url, nil)
	if err != nil {
		if tc.Verbose {
			fmt.Printf("  [CLEANUP] Failed to create request for %s: %v\n", serviceName, err)
		}
		return
	}

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		if tc.Verbose {
			fmt.Printf("  [CLEANUP] Failed to delete tenant from %s: %v\n", serviceName, err)
		}
		return
	}
	defer resp.Body.Close()

	if tc.Verbose && resp.StatusCode == http.StatusOK {
		fmt.Printf("  [CLEANUP] Cleaned up %s tenant data\n", serviceName)
	}
}

