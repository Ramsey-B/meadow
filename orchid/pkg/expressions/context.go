package expressions

import (
	"encoding/json"
	"time"
)

// Context holds all data available to JMESPath expressions during plan execution
type Context struct {
	// Response contains the current API response data
	Response interface{} `json:"response,omitempty"`

	// Parent contains the parent step's response (for sub-steps)
	Parent interface{} `json:"parent,omitempty"`

	// Item contains the current item when iterating (for_each)
	Item interface{} `json:"item,omitempty"`

	// ItemIndex contains the current item index when iterating
	ItemIndex int `json:"item_index,omitempty"`

	// Config contains the configuration values
	Config map[string]interface{} `json:"config,omitempty"`

	// Context contains persistent context data (stored between executions)
	Context map[string]interface{} `json:"context,omitempty"`

	// Auth contains authentication data (tokens, etc.)
	Auth map[string]interface{} `json:"auth,omitempty"`

	// Meta contains execution metadata
	Meta *ContextMeta `json:"meta,omitempty"`
}

// ContextMeta contains execution metadata
type ContextMeta struct {
	// PlanKey is the current plan key
	PlanKey string `json:"plan_key"`

	// ConfigID is the current config ID
	ConfigID string `json:"config_id,omitempty"`

	// ExecutionID is the current execution ID
	ExecutionID string `json:"execution_id,omitempty"`

	// StepPath is the current step path (e.g., "main.sub_steps[0]")
	StepPath string `json:"step_path,omitempty"`

	// Timestamp is the current execution timestamp
	Timestamp time.Time `json:"timestamp,omitempty"`

	// Attempt is the current retry attempt number
	Attempt int `json:"attempt,omitempty"`
}

// NewContext creates a new empty context
func NewContext() *Context {
	return &Context{
		Config:  make(map[string]interface{}),
		Context: make(map[string]interface{}),
		Auth:    make(map[string]interface{}),
		Meta:    &ContextMeta{},
	}
}

// WithResponse sets the response data
func (c *Context) WithResponse(response interface{}) *Context {
	c.Response = response
	return c
}

// WithParent sets the parent response data
func (c *Context) WithParent(parent interface{}) *Context {
	c.Parent = parent
	return c
}

// WithItem sets the current iteration item
func (c *Context) WithItem(item interface{}, index int) *Context {
	c.Item = item
	c.ItemIndex = index
	return c
}

// WithConfig sets the configuration values
func (c *Context) WithConfig(config map[string]interface{}) *Context {
	c.Config = config
	return c
}

// WithContext sets the persistent context data
func (c *Context) WithContext(ctx map[string]interface{}) *Context {
	c.Context = ctx
	return c
}

// WithAuth sets the authentication data
func (c *Context) WithAuth(auth map[string]interface{}) *Context {
	c.Auth = auth
	return c
}

// WithMeta sets the execution metadata
func (c *Context) WithMeta(meta *ContextMeta) *Context {
	c.Meta = meta
	return c
}

// ToMap converts the context to a map for JMESPath evaluation
func (c *Context) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if c.Response != nil {
		result["response"] = c.Response
	}
	if c.Parent != nil {
		result["parent"] = c.Parent
	}
	if c.Item != nil {
		result["item"] = c.Item
		result["item_index"] = c.ItemIndex
	}
	if c.Config != nil {
		result["config"] = c.Config
	}
	if c.Context != nil {
		result["context"] = c.Context
	}
	if c.Auth != nil {
		result["auth"] = c.Auth
	}
	if c.Meta != nil {
		result["meta"] = map[string]interface{}{
			"plan_key":     c.Meta.PlanKey,
			"config_id":    c.Meta.ConfigID,
			"execution_id": c.Meta.ExecutionID,
			"step_path":    c.Meta.StepPath,
			"timestamp":    c.Meta.Timestamp.Format(time.RFC3339),
			"attempt":      c.Meta.Attempt,
		}
	}

	return result
}

// Clone creates a deep copy of the context
func (c *Context) Clone() *Context {
	// Use JSON round-trip for deep copy
	data, _ := json.Marshal(c)
	var clone Context
	json.Unmarshal(data, &clone)
	return &clone
}
