// Package steps provides the execution layer for mapping transformation steps.
//
// # Overview
//
// A Step wraps an Action with execution context, input/output type tracking,
// and step-type-specific behavior. Steps are stateless and can be shared
// across multiple mapping executions.
//
// # Step Types
//
// There are three types of steps, each with different behavior:
//
// ## Transformer (StepTypeTransformer)
//
// Transforms input data and outputs the transformed result.
//
//	Input: "hello" -> text_to_upper -> Output: "HELLO"
//
// ## Validator (StepTypeValidator)
//
// Checks a condition and either passes through the input or fails.
// If the action returns false, the step returns an error.
// The output is the original input (pass-through), not the boolean result.
//
//	Input: "test@email.com" -> text_is_email -> Output: "test@email.com" (or error)
//
// ## Condition (StepTypeCondition)
//
// Checks a condition and either continues or breaks the chain.
// Unlike Validator, a false result doesn't error - it sets Break=true.
// Use for conditional branching without failing the entire mapping.
//
//	Input: "" -> text_is_not_empty -> Output: nil, Break: true
//
// # Multi-Input Steps
//
// Steps can receive multiple inputs from different links. For example,
// number_add receives two numbers from two different source fields:
//
//	field1 (5) -> add_step
//	field2 (3) -> add_step
//	add_step -> target_field (8)
//
// Inputs are accumulated in the Mapping.StepInputs map during execution.
// The step only executes when it has received all expected inputs.
//
// # Action Inversion
//
// For Validator and Condition steps, the action result can be inverted
// using ActionDefinition.Invert. This turns "must be true" into "must be false".
//
// # Example
//
//	stepDef := models.StepDefinition{
//	    ID:   "validate_email",
//	    Type: models.StepTypeValidator,
//	    Action: models.ActionDefinition{Key: "text_is_email"},
//	}
//
//	step, _ := NewStep(stepDef, models.ActionValueType{Type: models.ValueTypeString})
//	result, _ := step.Execute("test@example.com")
//	// result.Output == "test@example.com", result.Err == nil
package steps

import (
	"github.com/Ramsey-B/lotus/pkg/actions/registry"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

// NewStep creates a Step from a StepDefinition and expected input types.
//
// Parameters:
//   - stepDefinition: The step configuration (ID, type, action)
//   - inputTypes: Expected types for each input (order matters for multi-input steps)
//
// Returns an error if:
//   - The action key is not found in the registry
//   - A Validator/Condition step uses an action that doesn't return bool
//   - Input types don't match the action's requirements
func NewStep(stepDefinition models.StepDefinition, inputTypes ...models.ActionValueType) (*Step, error) {
	action, err := registry.GetAction(stepDefinition.Action.Key, stepDefinition.Action.Arguments, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddStep(stepDefinition.ID).AddAction(stepDefinition.Action.Key)
	}

	outputType := action.GetOutputType()

	if stepDefinition.Type == models.StepTypeValidator && outputType.Type != models.ValueTypeBool {
		return nil, errors.NewMappingErrorf("validator step actions must return a boolean, got %s", outputType.ToString()).AddStep(stepDefinition.ID).AddAction(stepDefinition.Action.Key)
	}

	if stepDefinition.Type == models.StepTypeCondition && outputType.Type != models.ValueTypeBool {
		return nil, errors.NewMappingErrorf("condition step actions must return a boolean, got %s", outputType.ToString()).AddStep(stepDefinition.ID).AddAction(stepDefinition.Action.Key)
	}

	return &Step{
		expectedInputTypes: inputTypes,
		outputType:         outputType,
		stepDefinition:     stepDefinition,
		action:             action,
	}, nil
}

// Step represents a compiled transformation step ready for execution.
//
// Steps are stateless and thread-safe. The same Step can be executed
// concurrently with different inputs. Input accumulation for multi-input
// steps is managed externally in the Mapping.StepInputs map.
//
// Fields:
//   - expectedInputTypes: Types of inputs this step expects (determines when step is ready)
//   - outputType: Type of value this step produces
//   - stepDefinition: Original configuration
//   - action: The compiled action that performs the transformation
type Step struct {
	expectedInputTypes []models.ActionValueType // Types of inputs this step expects
	outputType         models.ActionValueType   // Type this step outputs
	stepDefinition     models.StepDefinition    // Original step configuration
	action             models.Action            // The action that performs the work
}

func (s *Step) GetType() models.StepType {
	return s.stepDefinition.Type
}

func (s *Step) GetOutputType() models.ActionValueType {
	if s.stepDefinition.Type == models.StepTypeValidator || s.stepDefinition.Type == models.StepTypeCondition {
		// Validator and Condition steps pass through the input unchanged
		if len(s.expectedInputTypes) > 0 {
			return s.expectedInputTypes[0]
		}
		return models.ActionValueType{Type: models.ValueTypeAny}
	}

	// Transformer steps use the action's output type
	return s.outputType
}

// GetExpectedInputCount returns the number of inputs this step expects.
// Used by the mapping engine to determine when a step has received all inputs.
func (s *Step) GetExpectedInputCount() int {
	return len(s.expectedInputTypes)
}

// Execute runs the step's action with the provided inputs.
//
// Behavior varies by step type:
//
// Transformer: Returns the action's transformed output.
//
// Validator: Executes action, expects boolean result.
//   - true: Returns original input as output (pass-through)
//   - false: Returns error in StepOutput.Err
//
// Condition: Executes action, expects boolean result.
//   - true: Returns original input as output
//   - false: Returns with Break=true (no error)
//
// If len(inputs) != expected input count, returns empty output (step not ready).
// This allows the mapping engine to accumulate inputs from multiple links.
func (s *Step) Execute(inputs ...any) (models.StepOutput, error) {
	output := models.StepOutput{
		Output: nil,
		Break:  false,
		Inputs: inputs,
		Err:    nil,
	}

	// Note: We no longer check len(inputs) == len(s.expectedInputTypes) here.
	// The mapping engine's StepPendingInputs tracks when all links have been processed.
	// This allows array items (multiple values from 1 link) to flow through correctly.
	// The action's input validation will handle any type/count mismatches.

	if len(inputs) == 0 {
		return output, nil // no inputs, nothing to do
	}

	result, err := s.action.Execute(inputs...)
	if err != nil {
		output.Err = err
		return output, err
	}

	if s.stepDefinition.Type == models.StepTypeValidator {
		passed, err := utils.AnyToType[bool](result)
		if err != nil {
			output.Err = err
			return output, err
		}

		if s.stepDefinition.Action.Invert {
			passed = !passed
		}

		if !passed {
			output.Err = errors.NewMappingErrorf("validation action '%s' with arguments %s failed. inputs: %v, got: %v", s.action.GetKey(), utils.StringifyArgument(s.stepDefinition.Action.Arguments), inputs, result)
			return output, err
		}

		// For validator steps, pass through the original input unchanged
		if len(inputs) > 0 {
			output.Output = inputs[0]
		}
	} else if s.stepDefinition.Type == models.StepTypeCondition {
		passed, err := utils.AnyToType[bool](result)
		if err != nil {
			output.Err = err
			return output, err
		}

		if s.stepDefinition.Action.Invert {
			passed = !passed
		}

		if !passed {
			output.Break = true
			return output, err
		}

		// For condition steps, pass through the original input unchanged
		if len(inputs) > 0 {
			output.Output = inputs[0]
		}
	} else {
		// For transformer steps, use the action's output
		output.Output = result
	}

	return output, nil
}
