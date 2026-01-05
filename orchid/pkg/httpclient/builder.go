package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Ramsey-B/orchid/pkg/expressions"
	"github.com/Ramsey-B/orchid/pkg/models"
)

// templatePattern matches {{ expression }} patterns
var templatePattern = regexp.MustCompile(`\{\{\s*([^}]+)\s*\}\}`)

// RequestBuilder builds HTTP requests from step definitions
type RequestBuilder struct {
	evaluator *expressions.Evaluator
}

// NewRequestBuilder creates a new request builder
func NewRequestBuilder(evaluator *expressions.Evaluator) *RequestBuilder {
	return &RequestBuilder{
		evaluator: evaluator,
	}
}

// BuildRequest builds an HTTP request from a step and execution context
func (b *RequestBuilder) BuildRequest(ctx context.Context, step *models.Step, data map[string]any) (*http.Request, error) {
	// Build URL with templating and params
	reqURL, err := b.buildURL(step.URL, step.Params, data)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL: %w", err)
	}

	// Build request body
	var bodyReader io.Reader
	if step.Body != nil {
		bodyBytes, err := b.buildBody(step.Body, data)
		if err != nil {
			return nil, fmt.Errorf("failed to build body: %w", err)
		}
		if len(bodyBytes) > MaxRequestSize {
			return nil, fmt.Errorf("request body too large: %d bytes (max %d)", len(bodyBytes), MaxRequestSize)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Determine method
	method := step.Method
	if method == "" {
		method = http.MethodGet
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range step.Headers {
		resolvedValue, err := b.resolveTemplate(value, data)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve header %s: %w", key, err)
		}
		req.Header.Set(key, resolvedValue)
	}

	// Set Content-Type if body is present and not already set
	if step.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

// buildURL constructs the URL with templating and query parameters
func (b *RequestBuilder) buildURL(urlTemplate string, params map[string]string, data map[string]any) (string, error) {
	// Resolve URL template
	resolvedURL, err := b.resolveTemplate(urlTemplate, data)
	if err != nil {
		return "", fmt.Errorf("failed to resolve URL template: %w", err)
	}

	// Parse URL to add params
	parsedURL, err := url.Parse(resolvedURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Add query parameters
	if len(params) > 0 {
		query := parsedURL.Query()
		for key, value := range params {
			resolvedValue, err := b.resolveTemplate(value, data)
			if err != nil {
				return "", fmt.Errorf("failed to resolve param %s: %w", key, err)
			}
			query.Set(key, resolvedValue)
		}
		parsedURL.RawQuery = query.Encode()
	}

	return parsedURL.String(), nil
}

// buildBody builds the request body from the step body definition
func (b *RequestBuilder) buildBody(body any, data map[string]any) ([]byte, error) {
	// If body is a string, treat it as a template
	if str, ok := body.(string); ok {
		resolved, err := b.resolveTemplate(str, data)
		if err != nil {
			return nil, err
		}
		return []byte(resolved), nil
	}

	// If body is a map, resolve all template values recursively
	if m, ok := body.(map[string]any); ok {
		resolved, err := b.resolveMapTemplates(m, data)
		if err != nil {
			return nil, err
		}
		return json.Marshal(resolved)
	}

	// Otherwise, just marshal as JSON
	return json.Marshal(body)
}

// resolveMapTemplates recursively resolves templates in a map
func (b *RequestBuilder) resolveMapTemplates(m map[string]any, data map[string]any) (map[string]any, error) {
	result := make(map[string]any)

	for key, value := range m {
		switch v := value.(type) {
		case string:
			resolved, err := b.resolveTemplate(v, data)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve %s: %w", key, err)
			}
			result[key] = resolved
		case map[string]any:
			resolved, err := b.resolveMapTemplates(v, data)
			if err != nil {
				return nil, err
			}
			result[key] = resolved
		case []any:
			resolved, err := b.resolveSliceTemplates(v, data)
			if err != nil {
				return nil, err
			}
			result[key] = resolved
		default:
			result[key] = value
		}
	}

	return result, nil
}

// resolveSliceTemplates resolves templates in a slice
func (b *RequestBuilder) resolveSliceTemplates(s []any, data map[string]any) ([]any, error) {
	result := make([]any, len(s))

	for i, value := range s {
		switch v := value.(type) {
		case string:
			resolved, err := b.resolveTemplate(v, data)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve index %d: %w", i, err)
			}
			result[i] = resolved
		case map[string]any:
			resolved, err := b.resolveMapTemplates(v, data)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		case []any:
			resolved, err := b.resolveSliceTemplates(v, data)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		default:
			result[i] = value
		}
	}

	return result, nil
}

// resolveTemplate resolves {{ expression }} patterns in a string
func (b *RequestBuilder) resolveTemplate(template string, data map[string]any) (string, error) {
	// Check if the entire template is a single expression (for non-string results)
	trimmed := strings.TrimSpace(template)
	if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") {
		// Check if there's only one expression
		matches := templatePattern.FindAllStringSubmatch(template, -1)
		if len(matches) == 1 {
			expr := strings.TrimSpace(matches[0][1])
			result, err := b.evaluator.Evaluate(expr, data)
			if err != nil {
				return "", fmt.Errorf("expression evaluation failed: %w", err)
			}
			// For strings, return directly; for other types, convert to string
			if str, ok := result.(string); ok {
				return str, nil
			}
			return fmt.Sprintf("%v", result), nil
		}
	}

	// Multiple expressions or mixed content - replace each match
	var lastErr error
	result := templatePattern.ReplaceAllStringFunc(template, func(match string) string {
		// Extract expression from {{ expr }}
		submatches := templatePattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		expr := strings.TrimSpace(submatches[1])
		evalResult, err := b.evaluator.Evaluate(expr, data)
		if err != nil {
			lastErr = err
			return match
		}

		if evalResult == nil {
			return ""
		}

		if str, ok := evalResult.(string); ok {
			return str
		}
		return fmt.Sprintf("%v", evalResult)
	})

	if lastErr != nil {
		return "", lastErr
	}

	return result, nil
}

// ResolveExpression evaluates a single JMESPath expression
func (b *RequestBuilder) ResolveExpression(expr string, data map[string]any) (any, error) {
	return b.evaluator.Evaluate(expr, data)
}
