package models

// Step defines a single step in a plan execution
type Step struct {
	// Optional ID for addressing sub-steps in fanout enrichment / debugging
	ID string `json:"id,omitempty"`

	// EmitToKafka controls whether Orchid emits the main step response to Kafka.
	// Defaults to true. For fanout-heavy plans, setting this to false can reduce noise,
	// letting you focus on the per-item fanout messages (root.fanout[*]).
	EmitToKafka *bool `json:"emit_to_kafka,omitempty"`

	// FanoutEmitMode controls how fanout results are emitted to Kafka when iterate_over/sub_steps are used.
	// - "record" (default): emit one message per iterated item (response_body = enriched user object)
	// - "page": emit one message per page/loop iteration (response_body = []enriched user objects)
	FanoutEmitMode string `json:"fanout_emit_mode,omitempty"`

	// Request configuration
	URL     string            `json:"url"`               // URL with optional JMESPath templating: "https://api.example.com/users/{{ response.id }}"
	Method  string            `json:"method,omitempty"`  // HTTP method (GET, POST, PUT, DELETE, PATCH). Defaults to GET
	Headers map[string]string `json:"headers,omitempty"` // Static and templated headers
	Params  map[string]string `json:"params,omitempty"`  // Query parameters (static and templated)
	Body    any               `json:"body,omitempty"`    // Request body (static or templated)

	// Timeout configuration
	TimeoutSeconds int `json:"timeout_seconds,omitempty"` // Request timeout. Defaults to 30

	// Retry configuration
	Retry *RetryConfig `json:"retry,omitempty"` // Retry configuration

	// Status code policy
	// - AbortOn: if status is in this list, publish to error topic and abort the plan.
	// - IgnoreOn: if status is in this list, publish to error topic and continue (do not send to Lotus success topic).
	AbortOn  []int `json:"abort_on,omitempty"`
	IgnoreOn []int `json:"ignore_on,omitempty"`

	// Conditions (JMESPath expressions that evaluate to bool)
	While     string `json:"while,omitempty"`      // Continue looping while true
	AbortWhen string `json:"abort_when,omitempty"` // Abort entire plan if true
	RetryWhen string `json:"retry_when,omitempty"` // Retry step if true
	IgnoreWhen string `json:"ignore_when,omitempty"` // Route to error topic and continue (do not send to Lotus success topic)
	BreakWhen string `json:"break_when,omitempty"` // Exit while loop if true

	// Context management (JMESPath expressions to extract and store values)
	SetContext map[string]string `json:"set_context,omitempty"` // key -> JMESPath expression to store in context

	// Sub-steps for fanout
	IterateOver string `json:"iterate_over,omitempty"` // JMESPath expression returning array to iterate
	SubSteps    []Step `json:"sub_steps,omitempty"`    // Steps to execute for each item
	Concurrency int    `json:"concurrency,omitempty"`  // Max concurrent sub-step executions. Defaults to 50

	// Auth configuration
	AuthFlowID string `json:"auth_flow_id,omitempty"` // Auth flow to use for this step
}

// RetryConfig defines retry behavior for a step
type RetryConfig struct {
	MaxRetries   int    `json:"max_retries,omitempty"`   // Maximum retry attempts. Defaults to 3
	BackoffType  string `json:"backoff_type,omitempty"`  // "fibonacci", "exponential", "linear". Defaults to fibonacci
	InitialDelay int    `json:"initial_delay,omitempty"` // Initial delay in milliseconds. Defaults to 1000
	MaxDelay     int    `json:"max_delay,omitempty"`     // Maximum delay in milliseconds. Defaults to 60000
}

// PlanDefinition is the full plan structure stored in JSONB
type PlanDefinition struct {
	// Optional stable key for manual testing / binding to plans across systems.
	// Stored in JSONB plan_definition, emitted in Kafka as plan_key.
	Key string `json:"key,omitempty"`

	// Main step (entry point)
	Step Step `json:"step"`

	// Global rate limit configuration
	RateLimits []RateLimitConfig `json:"rate_limits,omitempty"`

	// Global timeout for entire plan execution
	MaxExecutionSeconds int `json:"max_execution_seconds,omitempty"` // Defaults to 300 (5 minutes)

	// Maximum nesting depth for sub-steps
	MaxNestingDepth int `json:"max_nesting_depth,omitempty"` // Defaults to 5
}

// RateLimitConfig defines rate limiting for a step or group
type RateLimitConfig struct {
	Name       string `json:"name"`               // Rate limit bucket name
	Endpoint   string `json:"endpoint,omitempty"` // URL pattern to match (regex)
	Requests   int    `json:"requests"`           // Max requests per window
	WindowSecs int    `json:"window_secs"`        // Window size in seconds
	Scope      string `json:"scope,omitempty"`    // "global", "per_config", "per_endpoint". Defaults to "global"
	Priority   int    `json:"priority,omitempty"` // Priority (higher = more important)

	// MaxConcurrent limits the number of concurrent in-flight requests that match this limit.
	// Useful for APIs that cap concurrent requests rather than requests-per-time-window.
	MaxConcurrent int `json:"max_concurrent,omitempty"`

	// Dynamic rate limit extraction from response headers
	Dynamic *DynamicRateLimit `json:"dynamic,omitempty"`
}

// DynamicRateLimit extracts rate limit info from response headers
type DynamicRateLimit struct {
	RemainingHeader string `json:"remaining_header,omitempty"` // Header containing remaining requests
	ResetHeader     string `json:"reset_header,omitempty"`     // Header containing reset time
	LimitHeader     string `json:"limit_header,omitempty"`     // Header containing request limit
	RetryAfter      string `json:"retry_after,omitempty"`      // Header for retry delay (on 429)
}

// DefaultStep returns a step with default values applied
func DefaultStep() Step {
	return Step{
		Method:         "GET",
		TimeoutSeconds: 30,
		Concurrency:    50,
	}
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		BackoffType:  "fibonacci",
		InitialDelay: 1000,
		MaxDelay:     60000,
	}
}

// DefaultPlanDefinition returns a plan definition with defaults
func DefaultPlanDefinition() PlanDefinition {
	return PlanDefinition{
		MaxExecutionSeconds: 300,
		MaxNestingDepth:     5,
	}
}
