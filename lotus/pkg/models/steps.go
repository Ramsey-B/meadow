package models

// StepOutput is the result of executing a step.
//
// Fields:
//   - Output: The transformed value (for transformers) or pass-through input (for validators/conditions)
//   - Break: True if a condition step evaluated to false (signals to stop processing this chain)
//   - Inputs: The inputs that were passed to the step (for debugging)
//   - Err: Any error that occurred during execution
type StepOutput struct {
	Output any   // The result value
	Break  bool  // True if condition failed (stop chain, no error)
	Inputs []any // Original inputs (for debugging)
	Err    error // Error if validation failed or action errored
}

// StepResult wraps StepOutput with additional context about which step produced it.
type StepResult struct {
	StepDefinitionID string // ID of the step that produced this result
	Input            []any  // Original inputs
	Error            error  // Any error
	StepOutput              // Embedded output
}

// StepType defines the behavior category of a step.
type StepType string

const (
	// StepTypeValidator checks a condition and fails if false.
	// Use for required validations (e.g., "email must be valid").
	// Output: Original input (pass-through) or error.
	StepTypeValidator StepType = "validator"

	// StepTypeTransformer transforms input data.
	// Use for data manipulation (e.g., "uppercase text", "add numbers").
	// Output: Transformed value from the action.
	StepTypeTransformer StepType = "transformer"

	// StepTypeCondition checks a condition and breaks the chain if false.
	// Use for optional branches (e.g., "only process if non-empty").
	// Output: Original input (pass-through) with Break=true if condition fails.
	StepTypeCondition StepType = "condition"
)

// StepDefinition is the configuration for a step in a mapping.
//
// Example JSON:
//
//	{
//	  "id": "uppercase_name",
//	  "type": "transformer",
//	  "action": {
//	    "key": "text_to_upper"
//	  }
//	}
//
// With arguments:
//
//	{
//	  "id": "multiply_by_10",
//	  "type": "transformer",
//	  "action": {
//	    "key": "number_multiply",
//	    "arguments": {"value": 10}
//	  }
//	}
type StepDefinition struct {
	ID            string           `json:"id" validate:"required"`            // Unique identifier for this step
	Type          StepType         `json:"type" validate:"required"`          // validator, transformer, or condition
	Action        ActionDefinition `json:"action" validate:"required"`        // The action to execute
	AllowDefaults bool             `json:"allow_defaults" validate:"omitempty"` // Allow action defaults for missing inputs
	OutputType    ActionValueType  `json:"output_type"`                       // Expected output type (auto-inferred if not set)
}

// Step interface defines the contract for executable steps.
// Implemented by the steps package.
type Step interface {
	GetKey() string
	GetType() StepType
	GetOutputType() ActionValueType
	Initialize(initialValue any) error
	Execute(inputs ...any) (StepOutput, error)
}
