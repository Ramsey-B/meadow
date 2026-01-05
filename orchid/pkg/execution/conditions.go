package execution

import (
	"github.com/Ramsey-B/orchid/pkg/expressions"
	"github.com/Ramsey-B/orchid/pkg/models"
)

// ConditionType represents the type of condition
type ConditionType string

const (
	ConditionWhile ConditionType = "while"
	ConditionAbort ConditionType = "abort_when"
	ConditionRetry ConditionType = "retry_when"
	ConditionBreak ConditionType = "break_when"
)

// ConditionResult holds the result of condition evaluation
type ConditionResult struct {
	Type   ConditionType
	Expr   string
	Result bool
	Error  error
}

// ConditionEvaluator evaluates step conditions
type ConditionEvaluator struct {
	evaluator *expressions.Evaluator
}

// NewConditionEvaluator creates a new condition evaluator
func NewConditionEvaluator(evaluator *expressions.Evaluator) *ConditionEvaluator {
	return &ConditionEvaluator{
		evaluator: evaluator,
	}
}

// EvaluateAll evaluates all conditions for a step
func (c *ConditionEvaluator) EvaluateAll(step *models.Step, data map[string]any) []ConditionResult {
	var results []ConditionResult

	if step.While != "" {
		results = append(results, c.evaluate(ConditionWhile, step.While, data))
	}

	if step.AbortWhen != "" {
		results = append(results, c.evaluate(ConditionAbort, step.AbortWhen, data))
	}

	if step.RetryWhen != "" {
		results = append(results, c.evaluate(ConditionRetry, step.RetryWhen, data))
	}

	if step.BreakWhen != "" {
		results = append(results, c.evaluate(ConditionBreak, step.BreakWhen, data))
	}

	return results
}

// Evaluate evaluates a single condition
func (c *ConditionEvaluator) Evaluate(condType ConditionType, expr string, data map[string]any) ConditionResult {
	return c.evaluate(condType, expr, data)
}

func (c *ConditionEvaluator) evaluate(condType ConditionType, expr string, data map[string]any) ConditionResult {
	result := ConditionResult{
		Type: condType,
		Expr: expr,
	}

	value, err := c.evaluator.EvaluateBool(expr, data)
	if err != nil {
		result.Error = err
		return result
	}

	result.Result = value
	return result
}

// ShouldContinueWhile checks if a while loop should continue
func (c *ConditionEvaluator) ShouldContinueWhile(step *models.Step, data map[string]any) (bool, error) {
	if step.While == "" {
		return false, nil
	}
	return c.evaluator.EvaluateBool(step.While, data)
}

// ShouldAbort checks if execution should abort
func (c *ConditionEvaluator) ShouldAbort(step *models.Step, data map[string]any) (bool, error) {
	if step.AbortWhen == "" {
		return false, nil
	}
	return c.evaluator.EvaluateBool(step.AbortWhen, data)
}

// ShouldRetry checks if the step should be retried
func (c *ConditionEvaluator) ShouldRetry(step *models.Step, data map[string]any) (bool, error) {
	if step.RetryWhen == "" {
		return false, nil
	}
	return c.evaluator.EvaluateBool(step.RetryWhen, data)
}

// ShouldBreak checks if a while loop should break
func (c *ConditionEvaluator) ShouldBreak(step *models.Step, data map[string]any) (bool, error) {
	if step.BreakWhen == "" {
		return false, nil
	}
	return c.evaluator.EvaluateBool(step.BreakWhen, data)
}

// ValidateConditions validates all condition expressions in a step
func (c *ConditionEvaluator) ValidateConditions(step *models.Step) []error {
	var errors []error

	if step.While != "" {
		if err := c.evaluator.Validate(step.While); err != nil {
			errors = append(errors, err)
		}
	}

	if step.AbortWhen != "" {
		if err := c.evaluator.Validate(step.AbortWhen); err != nil {
			errors = append(errors, err)
		}
	}

	if step.RetryWhen != "" {
		if err := c.evaluator.Validate(step.RetryWhen); err != nil {
			errors = append(errors, err)
		}
	}

	if step.BreakWhen != "" {
		if err := c.evaluator.Validate(step.BreakWhen); err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}
