// Package criteria provides evaluation logic for criteria-based relationship matching.
// It supports both simple equality checks and operator-based conditions.
package criteria

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Supported operators
const (
	OpEquals   = ""          // default, no prefix - simple equality
	OpContains = "$contains" // array contains value
	OpIn       = "$in"       // value is in array of options
	OpGte      = "$gte"      // greater than or equal
	OpGt       = "$gt"       // greater than
	OpLte      = "$lte"      // less than or equal
	OpLt       = "$lt"       // less than
	OpExists   = "$exists"   // field exists (value should be bool)
	OpNe       = "$ne"       // not equal
)

// Condition represents a single field condition to evaluate
type Condition struct {
	Field    string
	Operator string
	Value    any
}

// ParseCriteria converts a criteria map to structured conditions.
// Format: {"field": "value"} for equality, {"field": {"$op": "value"}} for operators
func ParseCriteria(criteria map[string]any) []Condition {
	var conditions []Condition

	for field, value := range criteria {
		switch v := value.(type) {
		case map[string]any:
			// Operator form: {"$contains": "value", "$in": [...]}
			for op, opValue := range v {
				conditions = append(conditions, Condition{
					Field:    field,
					Operator: op,
					Value:    opValue,
				})
			}
		default:
			// Simple equality
			conditions = append(conditions, Condition{
				Field:    field,
				Operator: OpEquals,
				Value:    v,
			})
		}
	}

	return conditions
}

// MatchesEntityData evaluates if entity data matches all criteria conditions.
// Returns true only if ALL conditions match (AND logic).
func MatchesEntityData(entityData json.RawMessage, conditions []Condition) bool {
	// Parse entity data into a map
	var data map[string]any
	if err := json.Unmarshal(entityData, &data); err != nil {
		return false
	}

	for _, cond := range conditions {
		if !evaluateCondition(data, cond) {
			return false
		}
	}
	return true
}

// MatchesCriteria is a convenience function that parses and evaluates in one call
func MatchesCriteria(entityData json.RawMessage, criteria map[string]any) bool {
	conditions := ParseCriteria(criteria)
	return MatchesEntityData(entityData, conditions)
}

// getNestedValue retrieves a value from a nested map using dot notation
func getNestedValue(data map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")

	var current any = data
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, exists := v[part]
			if !exists {
				return nil, false
			}
			current = val
		default:
			return nil, false
		}
	}

	return current, true
}

// evaluateCondition evaluates a single condition against entity data
func evaluateCondition(data map[string]any, cond Condition) bool {
	value, exists := getNestedValue(data, cond.Field)

	switch cond.Operator {
	case OpEquals:
		if !exists {
			return false
		}
		return valuesEqual(value, cond.Value)

	case OpNe:
		if !exists {
			return true // non-existent != any value
		}
		return !valuesEqual(value, cond.Value)

	case OpExists:
		expectExists, ok := cond.Value.(bool)
		if !ok {
			return false
		}
		return exists == expectExists

	case OpContains:
		// Array contains value
		if !exists {
			return false
		}
		arr, ok := toSlice(value)
		if !ok {
			return false
		}
		for _, item := range arr {
			if valuesEqual(item, cond.Value) {
				return true
			}
		}
		return false

	case OpIn:
		// Value is in array of options
		if !exists {
			return false
		}
		options, ok := toSlice(cond.Value)
		if !ok {
			return false
		}
		for _, opt := range options {
			if valuesEqual(value, opt) {
				return true
			}
		}
		return false

	case OpGte, OpGt, OpLte, OpLt:
		if !exists {
			return false
		}
		return compareNumeric(value, cond.Operator, cond.Value)

	default:
		// Unknown operator
		return false
	}
}

// valuesEqual compares two values with type coercion
func valuesEqual(a, b any) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Use reflect.DeepEqual for complex types
	if reflect.DeepEqual(a, b) {
		return true
	}

	// Convert both to strings for comparison (handles type differences like float64 vs int)
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// toSlice converts an interface to []any
func toSlice(v any) ([]any, bool) {
	switch arr := v.(type) {
	case []any:
		return arr, true
	case []string:
		result := make([]any, len(arr))
		for i, s := range arr {
			result[i] = s
		}
		return result, true
	case []int:
		result := make([]any, len(arr))
		for i, n := range arr {
			result[i] = n
		}
		return result, true
	case []float64:
		result := make([]any, len(arr))
		for i, n := range arr {
			result[i] = n
		}
		return result, true
	default:
		// Try to use reflection for other slice types
		val := reflect.ValueOf(v)
		if val.Kind() == reflect.Slice {
			result := make([]any, val.Len())
			for i := 0; i < val.Len(); i++ {
				result[i] = val.Index(i).Interface()
			}
			return result, true
		}
		return nil, false
	}
}

// compareNumeric performs numeric comparison
func compareNumeric(actual any, op string, expected any) bool {
	actualNum, ok := toFloat64(actual)
	if !ok {
		return false
	}

	expectedNum, ok := toFloat64(expected)
	if !ok {
		return false
	}

	switch op {
	case OpGte:
		return actualNum >= expectedNum
	case OpGt:
		return actualNum > expectedNum
	case OpLte:
		return actualNum <= expectedNum
	case OpLt:
		return actualNum < expectedNum
	default:
		return false
	}
}

// toFloat64 converts various types to float64
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// HashCriteria generates a deterministic hash of criteria for deduplication.
// The criteria is canonicalized (keys sorted) before hashing.
func HashCriteria(criteria map[string]any) string {
	canonical := canonicalizeCriteria(criteria)
	data, _ := json.Marshal(canonical)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// canonicalizeCriteria sorts keys recursively to ensure deterministic output
func canonicalizeCriteria(criteria map[string]any) map[string]any {
	result := make(map[string]any)

	// Get sorted keys
	keys := make([]string, 0, len(criteria))
	for k := range criteria {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := criteria[k]
		switch val := v.(type) {
		case map[string]any:
			result[k] = canonicalizeCriteria(val)
		default:
			result[k] = v
		}
	}

	return result
}

// SplitCriteria separates simple equality conditions from complex operator conditions.
// Simple conditions can be pushed to the database, complex ones evaluated in Go.
func SplitCriteria(criteria map[string]any) (simple map[string]any, complex []Condition) {
	simple = make(map[string]any)

	for field, value := range criteria {
		switch v := value.(type) {
		case map[string]any:
			// Has operator - evaluate in Go
			for op, opValue := range v {
				complex = append(complex, Condition{
					Field:    field,
					Operator: op,
					Value:    opValue,
				})
			}
		default:
			// Simple equality - can do in SQL with JSONB @>
			simple[field] = v
		}
	}

	return simple, complex
}

// BuildJSONBContainment creates a JSONB value for PostgreSQL @> containment query
// Only includes simple equality conditions
func BuildJSONBContainment(simple map[string]any) (json.RawMessage, error) {
	if len(simple) == 0 {
		return nil, nil
	}

	// Build nested structure for dotted paths
	result := make(map[string]any)

	for field, value := range simple {
		parts := strings.Split(field, ".")
		setNestedValue(result, parts, value)
	}

	return json.Marshal(result)
}

// setNestedValue sets a value in a nested map structure
func setNestedValue(m map[string]any, path []string, value any) {
	if len(path) == 1 {
		m[path[0]] = value
		return
	}

	key := path[0]
	if _, exists := m[key]; !exists {
		m[key] = make(map[string]any)
	}

	if nested, ok := m[key].(map[string]any); ok {
		setNestedValue(nested, path[1:], value)
	}
}
