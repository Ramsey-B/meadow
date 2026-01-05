package expressions

import (
	"fmt"
	"sync"

	"github.com/jmespath/go-jmespath"
)

// Evaluator wraps JMESPath expression evaluation
type Evaluator struct {
	cache map[string]*jmespath.JMESPath
	mu    sync.RWMutex
}

// NewEvaluator creates a new expression evaluator
func NewEvaluator() *Evaluator {
	return &Evaluator{
		cache: make(map[string]*jmespath.JMESPath),
	}
}

// Evaluate evaluates a JMESPath expression against data
func (e *Evaluator) Evaluate(expression string, data interface{}) (interface{}, error) {
	compiled, err := e.getOrCompile(expression)
	if err != nil {
		return nil, fmt.Errorf("invalid expression %q: %w", expression, err)
	}

	result, err := compiled.Search(data)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate expression %q: %w", expression, err)
	}

	return result, nil
}

// EvaluateString evaluates an expression and returns the result as a string
func (e *Evaluator) EvaluateString(expression string, data interface{}) (string, error) {
	result, err := e.Evaluate(expression, data)
	if err != nil {
		return "", err
	}

	if result == nil {
		return "", nil
	}

	str, ok := result.(string)
	if !ok {
		return fmt.Sprintf("%v", result), nil
	}

	return str, nil
}

// EvaluateBool evaluates an expression and returns the result as a bool
func (e *Evaluator) EvaluateBool(expression string, data interface{}) (bool, error) {
	result, err := e.Evaluate(expression, data)
	if err != nil {
		return false, err
	}

	if result == nil {
		return false, nil
	}

	switch v := result.(type) {
	case bool:
		return v, nil
	case string:
		return v != "", nil
	case float64:
		return v != 0, nil
	case []interface{}:
		return len(v) > 0, nil
	case map[string]interface{}:
		return len(v) > 0, nil
	default:
		return true, nil
	}
}

// EvaluateInt evaluates an expression and returns the result as an int
func (e *Evaluator) EvaluateInt(expression string, data interface{}) (int, error) {
	result, err := e.Evaluate(expression, data)
	if err != nil {
		return 0, err
	}

	if result == nil {
		return 0, nil
	}

	switch v := result.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", result)
	}
}

// EvaluateSlice evaluates an expression and returns the result as a slice
func (e *Evaluator) EvaluateSlice(expression string, data interface{}) ([]interface{}, error) {
	result, err := e.Evaluate(expression, data)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	slice, ok := result.([]interface{})
	if !ok {
		// Wrap single value in slice
		return []interface{}{result}, nil
	}

	return slice, nil
}

// EvaluateMap evaluates an expression and returns the result as a map
func (e *Evaluator) EvaluateMap(expression string, data interface{}) (map[string]interface{}, error) {
	result, err := e.Evaluate(expression, data)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("cannot convert %T to map", result)
	}

	return m, nil
}

// Validate checks if an expression is valid
func (e *Evaluator) Validate(expression string) error {
	_, err := e.getOrCompile(expression)
	return err
}

// getOrCompile retrieves a compiled expression from cache or compiles it
func (e *Evaluator) getOrCompile(expression string) (*jmespath.JMESPath, error) {
	// Try read lock first for cache hit
	e.mu.RLock()
	if compiled, ok := e.cache[expression]; ok {
		e.mu.RUnlock()
		return compiled, nil
	}
	e.mu.RUnlock()

	// Compile the expression
	compiled, err := jmespath.Compile(expression)
	if err != nil {
		return nil, err
	}

	// Write lock to update cache
	e.mu.Lock()
	e.cache[expression] = compiled
	e.mu.Unlock()

	return compiled, nil
}

// ClearCache clears the expression cache
func (e *Evaluator) ClearCache() {
	e.mu.Lock()
	e.cache = make(map[string]*jmespath.JMESPath)
	e.mu.Unlock()
}

