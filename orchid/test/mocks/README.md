# Mock API Framework for Testing

This package provides a framework for creating mock HTTP APIs for testing Orchid's plan execution engine.

## Overview

The mock API framework allows you to:

- Create mock HTTP servers for testing
- Define API endpoints with configurable responses
- Test various scenarios (pagination, rate limiting, errors, etc.)
- Validate requests made by Orchid
- Track request history

## Quick Start

```go
import "github.com/Ramsey-B/orchid/test/mocks"

func TestPlanExecution(t *testing.T) {
    // Create a mock API server
    mockAPI := mocks.NewServer(t)
    defer mockAPI.Close()

    // Define an endpoint
    endpoint := mockAPI.NewAPI("/api/users").
        WithMethod("GET").
        WithResponse(200, map[string]interface{}{
            "users": []map[string]interface{}{
                {"id": 1, "name": "Alice"},
                {"id": 2, "name": "Bob"},
            },
        })

    // Use in your test
    plan := &models.Plan{
        URL: mockAPI.URL() + "/api/users",
    }

    executor.Execute(ctx, plan)

    // Assertions
    assert.Equal(t, 1, endpoint.RequestCount())
    assert.Equal(t, "GET", endpoint.LastRequest().Method)
}
```

## Common Scenarios

### Pagination

```go
endpoint := mockAPI.NewAPI("/api/users").
    WithPagination(10, "page", "next_page_token").
    WithPageResponse(1, []map[string]interface{}{...}, "token123").
    WithPageResponse(2, []map[string]interface{}{...}, "")
```

### Rate Limiting

```go
endpoint := mockAPI.NewAPI("/api/users").
    WithRateLimit(10, 60). // 10 requests per 60 seconds
    WithRateLimitHeaders("X-RateLimit-Remaining", "X-RateLimit-Reset")
```

### Authentication

```go
authAPI := mockAPI.NewAPI("/api/auth/token").
    WithMethod("POST").
    WithResponse(200, map[string]interface{}{
        "access_token": "test-token",
        "expires_in": 3600,
    })
```

### Errors

```go
endpoint := mockAPI.NewAPI("/api/users").
    WithErrorResponse(429, "Rate limit exceeded", map[string]interface{}{
        "retry_after": 60,
    })
```

### Dynamic Responses

```go
endpoint := mockAPI.NewAPI("/api/users/{id}").
    WithDynamicResponse(func(r *http.Request) (int, interface{}) {
        id := mux.Vars(r)["id"]
        if id == "1" {
            return 200, map[string]interface{}{"id": 1, "name": "Alice"}
        }
        return 404, map[string]interface{}{"error": "Not found"}
    })
```

## API Reference

### Server

```go
// NewServer creates a new mock API server
server := mocks.NewServer(t)

// URL returns the base URL of the mock server
baseURL := server.URL()

// Close shuts down the server
defer server.Close()
```

### API Endpoint

```go
// NewAPI creates a new API endpoint
endpoint := server.NewAPI("/api/users")

// WithMethod sets the HTTP method
endpoint.WithMethod("GET")

// WithResponse sets a static response
endpoint.WithResponse(200, map[string]interface{}{...})

// WithDynamicResponse sets a dynamic response handler
endpoint.WithDynamicResponse(func(r *http.Request) (int, interface{}) {...})

// WithPagination enables pagination
endpoint.WithPagination(pageSize, pageParam, tokenParam)

// WithRateLimit adds rate limiting
endpoint.WithRateLimit(maxRequests, windowSeconds)

// RequestCount returns number of requests received
count := endpoint.RequestCount()

// LastRequest returns the last request received
req := endpoint.LastRequest()

// AllRequests returns all requests received
requests := endpoint.AllRequests()
```

## Test Fixtures

Pre-built fixtures are available for common scenarios:

```go
import "github.com/Ramsey-B/orchid/test/mocks/fixtures"

// Users API with pagination
usersAPI := fixtures.NewUsersAPI(mockAPI)

// Auth API with token refresh
authAPI := fixtures.NewAuthAPI(mockAPI)

// Rate-limited API
rateLimitedAPI := fixtures.NewRateLimitedAPI(mockAPI, 10, 60)
```

## Best Practices

1. **Use mock APIs for all external API calls** - Don't make real HTTP calls in tests
2. **Reset state between tests** - Use `server.Reset()` or create new servers
3. **Validate requests** - Check that Orchid makes the expected requests
4. **Test edge cases** - Use mock APIs to simulate errors, rate limits, etc.
5. **Keep fixtures reusable** - Create common fixtures for shared scenarios

## Examples

See `test/mocks/examples/` for complete examples:

- `users_api_test.go` - Testing user list with pagination
- `fanout_test.go` - Testing sub-step fanout
- `rate_limiting_test.go` - Testing rate limit handling
- `auth_test.go` - Testing auth flows
