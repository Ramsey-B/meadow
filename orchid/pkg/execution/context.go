package execution

import (
	"encoding/json"
	"fmt"

	"github.com/Ramsey-B/orchid/pkg/httpclient"
)

const (
	// MaxContextFieldSize is the maximum size of a single context field (64KB)
	MaxContextFieldSize = 64 * 1024

	// MaxContextTotalSize is the maximum total context size (1MB)
	MaxContextTotalSize = 1024 * 1024
)

// ExecutionContext holds all data available during step execution
type ExecutionContext struct {
	// Response from current/previous step
	Response *ResponseContext `json:"response,omitempty"`

	// Previous response in while loop
	Prev *ResponseContext `json:"prev,omitempty"`

	// Parent step response (for sub-steps)
	Parent *ResponseContext `json:"parent,omitempty"`

	// Persistent context (stored between executions)
	Context map[string]any `json:"context,omitempty"`

	// Config values for the current config
	Config map[string]any `json:"config,omitempty"`

	// Auth result from auth flow
	Auth *AuthContext `json:"auth,omitempty"`

	// Current item when iterating (for sub-steps)
	Item any `json:"item,omitempty"`

	// Current item index when iterating
	ItemIndex int `json:"item_index,omitempty"`

	// Execution metadata
	Meta *ExecutionMeta `json:"meta,omitempty"`
}

// ResponseContext holds response data for JMESPath access
type ResponseContext struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       any               `json:"body"`
}

// AuthContext holds authentication data
type AuthContext struct {
	Token        string            `json:"token"`
	TokenType    string            `json:"token_type,omitempty"`
	ExpiresAt    int64             `json:"expires_at,omitempty"`
	RefreshToken string            `json:"refresh_token,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"` // Pre-formatted auth headers
}

// ExecutionMeta holds execution metadata
type ExecutionMeta struct {
	TenantID     string `json:"tenant_id"`
	PlanKey      string `json:"plan_key"`
	ConfigID     string `json:"config_id"`
	ExecutionID  string `json:"execution_id"`
	StepPath     string `json:"step_path"`
	LoopCount    int    `json:"loop_count,omitempty"`
	RetryCount   int    `json:"retry_count,omitempty"`
	NestingLevel int    `json:"nesting_level,omitempty"`
}

// NewExecutionContext creates a new execution context
func NewExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		Context: make(map[string]any),
	}
}

// WithResponse sets the response context
func (c *ExecutionContext) WithResponse(resp *httpclient.Response) *ExecutionContext {
	c.Response = &ResponseContext{
		StatusCode: resp.StatusCode,
		Headers:    resp.Headers,
		Body:       resp.BodyJSON,
	}
	return c
}

// WithPrev sets the previous response (for while loops)
func (c *ExecutionContext) WithPrev(prev *ResponseContext) *ExecutionContext {
	c.Prev = prev
	return c
}

// WithParent sets the parent response (for sub-steps)
func (c *ExecutionContext) WithParent(parent *ResponseContext) *ExecutionContext {
	c.Parent = parent
	return c
}

// WithConfig sets the config values
func (c *ExecutionContext) WithConfig(config map[string]any) *ExecutionContext {
	c.Config = config
	return c
}

// WithAuth sets the auth context
func (c *ExecutionContext) WithAuth(auth *AuthContext) *ExecutionContext {
	c.Auth = auth
	return c
}

// WithItem sets the current item (for iteration)
func (c *ExecutionContext) WithItem(item any, index int) *ExecutionContext {
	c.Item = item
	c.ItemIndex = index
	return c
}

// WithMeta sets the execution metadata
func (c *ExecutionContext) WithMeta(meta *ExecutionMeta) *ExecutionContext {
	c.Meta = meta
	return c
}

// SetContextValue sets a value in the persistent context
func (c *ExecutionContext) SetContextValue(key string, value any) error {
	// Validate field size
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to serialize context value: %w", err)
	}
	if len(data) > MaxContextFieldSize {
		return fmt.Errorf("context field '%s' too large: %d bytes (max %d)", key, len(data), MaxContextFieldSize)
	}

	c.Context[key] = value

	// Validate total size
	totalData, err := json.Marshal(c.Context)
	if err != nil {
		delete(c.Context, key)
		return fmt.Errorf("failed to serialize context: %w", err)
	}
	if len(totalData) > MaxContextTotalSize {
		delete(c.Context, key)
		return fmt.Errorf("total context too large: %d bytes (max %d)", len(totalData), MaxContextTotalSize)
	}

	return nil
}

// ToMap converts the execution context to a map for JMESPath evaluation
func (c *ExecutionContext) ToMap() map[string]any {
	result := make(map[string]any)

	toAnyMap := func(m map[string]string) map[string]any {
		if m == nil {
			return nil
		}
		out := make(map[string]any, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out
	}

	if c.Response != nil {
		result["response"] = map[string]any{
			"status_code": c.Response.StatusCode,
			"headers":     toAnyMap(c.Response.Headers),
			"body":        c.Response.Body,
		}
	}

	if c.Prev != nil {
		result["prev"] = map[string]any{
			"status_code": c.Prev.StatusCode,
			"headers":     toAnyMap(c.Prev.Headers),
			"body":        c.Prev.Body,
		}
	}

	if c.Parent != nil {
		result["parent"] = map[string]any{
			"status_code": c.Parent.StatusCode,
			"headers":     toAnyMap(c.Parent.Headers),
			"body":        c.Parent.Body,
		}
	}

	if c.Context != nil {
		result["context"] = c.Context
	}

	if c.Config != nil {
		result["config"] = c.Config
	}

	if c.Auth != nil {
		result["auth"] = map[string]any{
			"token":         c.Auth.Token,
			"token_type":    c.Auth.TokenType,
			"expires_at":    c.Auth.ExpiresAt,
			"refresh_token": c.Auth.RefreshToken,
			"headers":       toAnyMap(c.Auth.Headers),
		}
	}

	if c.Item != nil {
		result["item"] = c.Item
		result["item_index"] = c.ItemIndex
	}

	if c.Meta != nil {
		result["meta"] = map[string]any{
			"tenant_id":     c.Meta.TenantID,
			"plan_key":      c.Meta.PlanKey,
			"config_id":     c.Meta.ConfigID,
			"execution_id":  c.Meta.ExecutionID,
			"step_path":     c.Meta.StepPath,
			"loop_count":    c.Meta.LoopCount,
			"retry_count":   c.Meta.RetryCount,
			"nesting_level": c.Meta.NestingLevel,
		}
	}

	return result
}

// Clone creates a deep copy of the execution context
func (c *ExecutionContext) Clone() *ExecutionContext {
	data, _ := json.Marshal(c)
	var clone ExecutionContext
	json.Unmarshal(data, &clone)
	return &clone
}

// Serialize converts the context to JSON for queue messages
func (c *ExecutionContext) Serialize() ([]byte, error) {
	return json.Marshal(c)
}

// Deserialize creates an execution context from JSON
func Deserialize(data []byte) (*ExecutionContext, error) {
	var ctx ExecutionContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("failed to deserialize execution context: %w", err)
	}
	if ctx.Context == nil {
		ctx.Context = make(map[string]any)
	}
	return &ctx, nil
}
