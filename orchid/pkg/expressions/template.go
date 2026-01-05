package expressions

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// templatePattern matches {{ expression }} patterns
	templatePattern = regexp.MustCompile(`\{\{\s*(.+?)\s*\}\}`)
)

// Template handles string interpolation with JMESPath expressions
type Template struct {
	evaluator *Evaluator
}

// NewTemplate creates a new template processor
func NewTemplate(evaluator *Evaluator) *Template {
	return &Template{
		evaluator: evaluator,
	}
}

// Render replaces {{ expression }} patterns in a string with evaluated values
func (t *Template) Render(template string, data interface{}) (string, error) {
	var lastErr error

	result := templatePattern.ReplaceAllStringFunc(template, func(match string) string {
		// Extract expression from {{ expression }}
		submatch := templatePattern.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}

		expression := strings.TrimSpace(submatch[1])
		value, err := t.evaluator.EvaluateString(expression, data)
		if err != nil {
			lastErr = fmt.Errorf("failed to evaluate %q: %w", expression, err)
			return match
		}

		return value
	})

	return result, lastErr
}

// RenderMap renders all string values in a map
func (t *Template) RenderMap(input map[string]interface{}, data interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range input {
		rendered, err := t.RenderValue(value, data)
		if err != nil {
			return nil, fmt.Errorf("failed to render key %q: %w", key, err)
		}
		result[key] = rendered
	}

	return result, nil
}

// RenderValue renders a value, handling strings, maps, and slices
func (t *Template) RenderValue(value interface{}, data interface{}) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return t.Render(v, data)

	case map[string]interface{}:
		return t.RenderMap(v, data)

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			rendered, err := t.RenderValue(item, data)
			if err != nil {
				return nil, err
			}
			result[i] = rendered
		}
		return result, nil

	default:
		return value, nil
	}
}

// HasTemplates checks if a string contains template expressions
func HasTemplates(s string) bool {
	return templatePattern.MatchString(s)
}

// ExtractExpressions extracts all expressions from a template string
func ExtractExpressions(template string) []string {
	matches := templatePattern.FindAllStringSubmatch(template, -1)
	expressions := make([]string, 0, len(matches))

	for _, match := range matches {
		if len(match) >= 2 {
			expressions = append(expressions, strings.TrimSpace(match[1]))
		}
	}

	return expressions
}

