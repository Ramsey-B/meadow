// Package extractor provides tools for extracting values from nested JSON data
package extractor

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Extractor handles extracting values from nested data structures
type Extractor struct{}

// New creates a new Extractor
func New() *Extractor {
	return &Extractor{}
}

// Extract extracts a value from data using a JSONPath-like expression
// Supported syntax:
// - Simple path: "name", "address.city", "user.profile.email"
// - Array access: "items[0]", "users[*].name" (first match), "data.results[2].value"
// - Wildcard: "users[*].email" returns first non-nil match
func (e *Extractor) Extract(data any, path string) (any, error) {
	if path == "" {
		return data, nil
	}

	parts := parsePath(path)
	current := data

	for _, part := range parts {
		var err error
		current, err = e.extractPart(current, part)
		if err != nil {
			return nil, err
		}
		if current == nil {
			return nil, nil
		}
	}

	return current, nil
}

// ExtractString extracts a value and converts it to a string
func (e *Extractor) ExtractString(data any, path string) (*string, error) {
	value, err := e.Extract(data, path)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}

	s := toString(value)
	return &s, nil
}

// ExtractAll extracts all matching values for a path with wildcards
func (e *Extractor) ExtractAll(data any, path string) ([]any, error) {
	if path == "" {
		return []any{data}, nil
	}

	parts := parsePath(path)
	results := []any{data}

	for _, part := range parts {
		var newResults []any
		for _, current := range results {
			if current == nil {
				continue
			}
			
			if part.isWildcard {
				// Expand array
				arr, ok := toArray(current)
				if ok {
					newResults = append(newResults, arr...)
				}
			} else {
				value, err := e.extractPart(current, part)
				if err != nil {
					continue
				}
				if value != nil {
					newResults = append(newResults, value)
				}
			}
		}
		results = newResults
	}

	return results, nil
}

// pathPart represents a parsed path segment
type pathPart struct {
	key        string
	isArray    bool
	arrayIndex int
	isWildcard bool
}

// parsePath parses a JSONPath-like expression into parts
func parsePath(path string) []pathPart {
	var parts []pathPart
	
	segments := splitPath(path)
	for _, seg := range segments {
		part := pathPart{key: seg}
		
		// Check for array notation
		if idx := strings.Index(seg, "["); idx != -1 {
			part.key = seg[:idx]
			indexPart := seg[idx+1 : len(seg)-1]
			
			if indexPart == "*" {
				part.isWildcard = true
				part.isArray = true
			} else {
				i, err := strconv.Atoi(indexPart)
				if err == nil {
					part.isArray = true
					part.arrayIndex = i
				}
			}
		}
		
		parts = append(parts, part)
	}
	
	return parts
}

// splitPath splits a dot-notation path, respecting array brackets
func splitPath(path string) []string {
	var parts []string
	var current strings.Builder
	
	inBracket := false
	for _, c := range path {
		switch c {
		case '[':
			inBracket = true
			current.WriteRune(c)
		case ']':
			inBracket = false
			current.WriteRune(c)
		case '.':
			if !inBracket {
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
			} else {
				current.WriteRune(c)
			}
		default:
			current.WriteRune(c)
		}
	}
	
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	
	return parts
}

// extractPart extracts a value for a single path part
func (e *Extractor) extractPart(data any, part pathPart) (any, error) {
	// First, get the value by key if there is one
	var value any = data
	
	if part.key != "" {
		switch v := data.(type) {
		case map[string]any:
			val, ok := v[part.key]
			if !ok {
				return nil, nil
			}
			value = val
		case map[string]string:
			val, ok := v[part.key]
			if !ok {
				return nil, nil
			}
			value = val
		default:
			return nil, fmt.Errorf("cannot extract key %q from type %T", part.key, data)
		}
	}
	
	// Then apply array indexing if needed
	if part.isArray && !part.isWildcard {
		arr, ok := toArray(value)
		if !ok {
			return nil, fmt.Errorf("expected array for index access, got %T", value)
		}
		if part.arrayIndex < 0 || part.arrayIndex >= len(arr) {
			return nil, nil
		}
		return arr[part.arrayIndex], nil
	}
	
	return value, nil
}

// toArray converts a value to an array
func toArray(v any) ([]any, bool) {
	switch arr := v.(type) {
	case []any:
		return arr, true
	case []string:
		result := make([]any, len(arr))
		for i, s := range arr {
			result[i] = s
		}
		return result, true
	case []map[string]any:
		result := make([]any, len(arr))
		for i, m := range arr {
			result[i] = m
		}
		return result, true
	default:
		return nil, false
	}
}

// toString converts any value to a string
func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%v", val)
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case nil:
		return ""
	default:
		// For complex types, JSON encode
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// FromJSON parses JSON data and returns it as a map
func FromJSON(data json.RawMessage) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// ArrayHandling defines how to handle array values
type ArrayHandling string

const (
	// ArrayFirst uses the first element
	ArrayFirst ArrayHandling = "first"
	// ArrayLast uses the last element
	ArrayLast ArrayHandling = "last"
	// ArrayJoin joins elements with a separator
	ArrayJoin ArrayHandling = "join"
	// ArrayFilter applies a filter expression
	ArrayFilter ArrayHandling = "filter"
)

// HandleArray processes an array value according to the handling strategy
func HandleArray(arr []any, handling ArrayHandling, separator string, filterExpr string) (any, error) {
	if len(arr) == 0 {
		return nil, nil
	}
	
	switch handling {
	case ArrayFirst:
		return arr[0], nil
	case ArrayLast:
		return arr[len(arr)-1], nil
	case ArrayJoin:
		var parts []string
		for _, v := range arr {
			parts = append(parts, toString(v))
		}
		return strings.Join(parts, separator), nil
	case ArrayFilter:
		// Filter by expression (simplified: key=value)
		// Full implementation would use a proper expression parser
		if filterExpr == "" {
			return arr[0], nil
		}
		for _, v := range arr {
			if matches(v, filterExpr) {
				return v, nil
			}
		}
		return nil, nil
	default:
		return arr[0], nil
	}
}

// matches checks if a value matches a simple filter expression
func matches(v any, expr string) bool {
	// Simple key=value matching
	parts := strings.SplitN(expr, "=", 2)
	if len(parts) != 2 {
		return false
	}
	
	key, expectedValue := parts[0], parts[1]
	
	switch m := v.(type) {
	case map[string]any:
		if val, ok := m[key]; ok {
			return toString(val) == expectedValue
		}
	}
	
	return false
}

