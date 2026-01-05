package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"

	"github.com/Ramsey-B/orchid/pkg/execution"
	"github.com/Ramsey-B/orchid/pkg/expressions"
	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/Ramsey-B/orchid/pkg/redis"
	"github.com/Ramsey-B/orchid/pkg/repositories"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

var (
	// ErrAuthFlowNotFound is returned when an auth flow is not found
	ErrAuthFlowNotFound = errors.New("auth flow not found")

	// ErrTokenNotFound is returned when a cached token is not found
	ErrTokenNotFound = errors.New("cached token not found")

	// ErrTokenExpired is returned when the cached token is expired
	ErrTokenExpired = errors.New("cached token expired")

	// ErrTokenExtractionFailed is returned when token extraction from response fails
	ErrTokenExtractionFailed = errors.New("failed to extract token from response")

	// ErrAuthFlowExecutionFailed is returned when auth flow execution fails
	ErrAuthFlowExecutionFailed = errors.New("auth flow execution failed")
)

const (
	// DefaultTTLSeconds is the default cache TTL if not specified
	DefaultTTLSeconds = 3600 // 1 hour

	// DefaultSkewSeconds is the default skew for token refresh
	DefaultSkewSeconds = 60 // 1 minute before expiry

	// CacheKeyPrefix is the prefix for auth token cache keys
	CacheKeyPrefix = "auth:token:"
)

// CachedToken represents a cached authentication token
type CachedToken struct {
	Token        string            `json:"token"`
	TokenType    string            `json:"token_type,omitempty"`
	RefreshToken string            `json:"refresh_token,omitempty"`
	ExpiresAt    int64             `json:"expires_at,omitempty"`
	Headers      map[string]string `json:"headers"`
	CreatedAt    int64             `json:"created_at"`
}

// IsExpired checks if the token is expired (with skew)
func (t *CachedToken) IsExpired(skewSeconds int) bool {
	if t.ExpiresAt == 0 {
		return false // No expiry set
	}
	now := time.Now().Unix()
	return now >= (t.ExpiresAt - int64(skewSeconds))
}

// ToAuthContext converts the cached token to an execution AuthContext
func (t *CachedToken) ToAuthContext() *execution.AuthContext {
	return &execution.AuthContext{
		Token:        t.Token,
		TokenType:    t.TokenType,
		ExpiresAt:    t.ExpiresAt,
		RefreshToken: t.RefreshToken,
		Headers:      t.Headers,
	}
}

// Manager handles authentication token management
type Manager struct {
	authFlowRepo repositories.AuthFlowRepo
	redisClient  *redis.Client
	stepExecutor *execution.StepExecutor
	evaluator    *expressions.Evaluator
	logger       ectologger.Logger
}

// NewManager creates a new auth manager
func NewManager(
	authFlowRepo repositories.AuthFlowRepo,
	redisClient *redis.Client,
	stepExecutor *execution.StepExecutor,
	evaluator *expressions.Evaluator,
	logger ectologger.Logger,
) *Manager {
	return &Manager{
		authFlowRepo: authFlowRepo,
		redisClient:  redisClient,
		stepExecutor: stepExecutor,
		evaluator:    evaluator,
		logger:       logger,
	}
}

// GetAuthContext retrieves or generates an auth context for a plan execution
func (m *Manager) GetAuthContext(
	ctx context.Context,
	authFlowID uuid.UUID,
	tenantID uuid.UUID,
	configID uuid.UUID,
	config map[string]any,
) (*execution.AuthContext, error) {
	ctx, span := tracing.StartSpan(ctx, "AuthManager.GetAuthContext")
	defer span.End()

	// Load the auth flow
	authFlow, err := m.authFlowRepo.GetByID(ctx, authFlowID)
	if err != nil {
		if httperror.IsHTTPError(err) && httperror.GetStatusCode(err) == http.StatusNotFound {
			return nil, ErrAuthFlowNotFound
		}
		return nil, fmt.Errorf("failed to load auth flow: %w", err)
	}

	// Try to get cached token
	cacheKey := m.cacheKey(tenantID, authFlowID, configID)
	cachedToken, err := m.getCachedToken(ctx, cacheKey)
	if err == nil {
		// Backwards/forwards-compat: ensure cached token has the expected auth header populated.
		// Older cached tokens (or partial writes) may be missing Headers, which would cause step templates like
		// {{ auth.headers.Authorization }} to resolve to empty and trigger 401s.
		updated := false
		if cachedToken.Headers == nil {
			cachedToken.Headers = make(map[string]string)
			updated = true
		}
		if _, ok := cachedToken.Headers[authFlow.HeaderName]; !ok {
			headerValue := cachedToken.Token
			if authFlow.HeaderFormat != nil && *authFlow.HeaderFormat != "" {
				headerValue = strings.ReplaceAll(*authFlow.HeaderFormat, "{token}", cachedToken.Token)
				headerValue = strings.ReplaceAll(headerValue, "{{token}}", cachedToken.Token)
			}
			cachedToken.Headers[authFlow.HeaderName] = headerValue
			updated = true
		}
		if cachedToken.TokenType == "" && authFlow.HeaderFormat != nil {
			format := strings.ToLower(*authFlow.HeaderFormat)
			if strings.Contains(format, "bearer") {
				cachedToken.TokenType = "Bearer"
				updated = true
			} else if strings.Contains(format, "basic") {
				cachedToken.TokenType = "Basic"
				updated = true
			}
		}

		// Check if token is still valid
		skew := DefaultSkewSeconds
		if authFlow.SkewSeconds != nil {
			skew = *authFlow.SkewSeconds
		}

		if !cachedToken.IsExpired(skew) {
			m.logger.WithContext(ctx).Debugf("Using cached auth token for flow %s", authFlowID)
			// Best-effort: re-cache repaired token (keeps local dev from getting stuck with bad cached entries).
			if updated {
				ttl := m.calculateTTL(authFlow, cachedToken)
				if cacheErr := m.cacheToken(ctx, cacheKey, cachedToken, ttl); cacheErr != nil {
					m.logger.WithContext(ctx).WithError(cacheErr).Warn("Failed to update cached auth token headers")
				}
			}
			return cachedToken.ToAuthContext(), nil
		}

		m.logger.WithContext(ctx).Debugf("Cached token expired, refreshing for flow %s", authFlowID)
	}

	// Execute auth flow to get new token
	m.logger.WithContext(ctx).Infof("Executing auth flow %s to obtain token", authFlowID)
	newToken, err := m.executeAuthFlow(ctx, authFlow, config)
	if err != nil {
		return nil, fmt.Errorf("auth flow execution failed: %w", err)
	}

	// Cache the token
	ttl := m.calculateTTL(authFlow, newToken)
	if err := m.cacheToken(ctx, cacheKey, newToken, ttl); err != nil {
		m.logger.WithContext(ctx).WithError(err).Warn("Failed to cache auth token")
	}

	return newToken.ToAuthContext(), nil
}

// InvalidateToken removes a cached token
func (m *Manager) InvalidateToken(ctx context.Context, tenantID, authFlowID, configID uuid.UUID) error {
	ctx, span := tracing.StartSpan(ctx, "AuthManager.InvalidateToken")
	defer span.End()

	cacheKey := m.cacheKey(tenantID, authFlowID, configID)
	return m.redisClient.Del(ctx, cacheKey)
}

// executeAuthFlow executes an auth flow to obtain a token
func (m *Manager) executeAuthFlow(ctx context.Context, authFlow *models.AuthFlow, config map[string]any) (*CachedToken, error) {
	ctx, span := tracing.StartSpan(ctx, "AuthManager.executeAuthFlow")
	defer span.End()

	// Parse the auth flow plan definition
	if authFlow.PlanDefinition.Data == nil {
		return nil, errors.New("auth flow plan definition is empty")
	}

	stepData, err := json.Marshal(authFlow.PlanDefinition.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal auth flow definition: %w", err)
	}

	var step models.Step
	if err := json.Unmarshal(stepData, &step); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth flow step: %w", err)
	}

	// Build execution context with config
	execCtx := execution.NewExecutionContext().WithConfig(config)

	// Execute the auth step
	result, err := m.stepExecutor.Execute(ctx, &step, execCtx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAuthFlowExecutionFailed, err)
	}

	if result.Response == nil {
		return nil, fmt.Errorf("%w: no response from auth flow", ErrAuthFlowExecutionFailed)
	}

	// Build data map for JMESPath extraction
	dataMap := map[string]any{
		"response": map[string]any{
			"status_code": result.Response.StatusCode,
			"headers":     result.Response.Headers,
			"body":        result.Response.BodyJSON,
		},
	}

	// Extract token using token_path
	token, err := m.evaluator.EvaluateString(authFlow.TokenPath, dataMap)
	if err != nil || token == "" {
		return nil, fmt.Errorf("%w: token_path=%s", ErrTokenExtractionFailed, authFlow.TokenPath)
	}

	cachedToken := &CachedToken{
		Token:     token,
		CreatedAt: time.Now().Unix(),
		Headers:   make(map[string]string),
	}

	// Extract refresh token if path is specified
	if authFlow.RefreshPath != nil && *authFlow.RefreshPath != "" {
		refreshToken, _ := m.evaluator.EvaluateString(*authFlow.RefreshPath, dataMap)
		cachedToken.RefreshToken = refreshToken
	}

	// Extract expires_in if path is specified
	if authFlow.ExpiresInPath != nil && *authFlow.ExpiresInPath != "" {
		expiresIn, err := m.evaluator.EvaluateInt(*authFlow.ExpiresInPath, dataMap)
		if err == nil && expiresIn > 0 {
			cachedToken.ExpiresAt = time.Now().Unix() + int64(expiresIn)
		}
	}

	// Format auth header
	headerName := authFlow.HeaderName
	headerValue := token
	if authFlow.HeaderFormat != nil && *authFlow.HeaderFormat != "" {
		headerValue = strings.ReplaceAll(*authFlow.HeaderFormat, "{token}", token)
		// Also support {{token}} format
		headerValue = strings.ReplaceAll(headerValue, "{{token}}", token)
	}
	cachedToken.Headers[headerName] = headerValue

	// Try to determine token type from format
	if authFlow.HeaderFormat != nil {
		format := strings.ToLower(*authFlow.HeaderFormat)
		if strings.Contains(format, "bearer") {
			cachedToken.TokenType = "Bearer"
		} else if strings.Contains(format, "basic") {
			cachedToken.TokenType = "Basic"
		}
	}

	m.logger.WithContext(ctx).Infof("Successfully obtained auth token for flow %s", authFlow.ID)
	return cachedToken, nil
}

// getCachedToken retrieves a token from Redis cache
func (m *Manager) getCachedToken(ctx context.Context, key string) (*CachedToken, error) {
	data, err := m.redisClient.Get(ctx, key)
	if err != nil {
		return nil, ErrTokenNotFound
	}

	var token CachedToken
	if err := json.Unmarshal([]byte(data), &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached token: %w", err)
	}

	return &token, nil
}

// cacheToken stores a token in Redis cache
func (m *Manager) cacheToken(ctx context.Context, key string, token *CachedToken, ttl time.Duration) error {
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	return m.redisClient.Set(ctx, key, string(data), ttl)
}

// calculateTTL calculates the cache TTL for a token
func (m *Manager) calculateTTL(authFlow *models.AuthFlow, token *CachedToken) time.Duration {
	// Use explicit TTL from auth flow if set
	if authFlow.TTLSeconds != nil && *authFlow.TTLSeconds > 0 {
		return time.Duration(*authFlow.TTLSeconds) * time.Second
	}

	// Use token expiry minus skew
	if token.ExpiresAt > 0 {
		skew := DefaultSkewSeconds
		if authFlow.SkewSeconds != nil {
			skew = *authFlow.SkewSeconds
		}

		remaining := token.ExpiresAt - time.Now().Unix() - int64(skew)
		if remaining > 0 {
			return time.Duration(remaining) * time.Second
		}
	}

	// Default TTL
	return time.Duration(DefaultTTLSeconds) * time.Second
}

// cacheKey generates a cache key for auth tokens
func (m *Manager) cacheKey(tenantID, authFlowID, configID uuid.UUID) string {
	return fmt.Sprintf("%s%s:%s:%s", CacheKeyPrefix, tenantID, authFlowID, configID)
}

