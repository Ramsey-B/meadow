package merging

import (
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/Ramsey-B/ivy/pkg/models"
)

// FieldMerger handles field-level merge logic
type FieldMerger struct{}

// NewFieldMerger creates a new FieldMerger
func NewFieldMerger() *FieldMerger {
	return &FieldMerger{}
}

// MergeField merges a single field across multiple entities
func (m *FieldMerger) MergeField(
	field string,
	entities []entityDataWithMeta,
	strategy models.FieldMergeStrategy,
	priorities map[string]int,
) (any, *models.MergeConflict) {
	// Collect all values for this field
	values := make([]fieldValue, 0, len(entities))
	for _, entity := range entities {
		if val, ok := entity.Data[field]; ok && val != nil {
			values = append(values, fieldValue{
				Value:       val,
				UpdatedAt:   entity.UpdatedAt,
				Integration: entity.Integration,
				EntityID:    entity.EntityID,
			})
		}
	}

	if len(values) == 0 {
		return nil, nil
	}

	if len(values) == 1 {
		return values[0].Value, nil
	}

	// Check for conflicts (all values different)
	conflict := m.detectConflict(field, values)

	// Apply merge strategy
	var result any
	switch strategy.Strategy {
	case models.MergeStrategyMostRecent:
		result = m.mostRecent(values)
	case models.MergeStrategyMostTrusted:
		result = m.mostTrusted(values, priorities)
	case models.MergeStrategyCollectAll:
		result = m.collectAll(values, strategy.MaxItems, strategy.Dedup)
	case models.MergeStrategyLongestValue:
		result = m.longest(values)
	case models.MergeStrategyShortestValue:
		result = m.shortest(values)
	case models.MergeStrategyFirstValue:
		result = m.first(values)
	case models.MergeStrategyLastValue:
		result = m.last(values)
	case models.MergeStrategyMax:
		result = m.max(values)
	case models.MergeStrategyMin:
		result = m.min(values)
	case models.MergeStrategySum:
		result = m.sum(values)
	case models.MergeStrategyAverage:
		result = m.average(values)
	case models.MergeStrategyPreferNonEmpty:
		result = m.preferNonEmpty(values)
	case models.MergeStrategySourcePriority:
		result = m.mostTrusted(values, priorities)
	default:
		result = m.preferNonEmpty(values)
	}

	if conflict != nil {
		conflict.ResolvedValue = result
		conflict.Resolution = string(strategy.Strategy)
	}

	return result, conflict
}

// detectConflict checks if there's a conflict among values
func (m *FieldMerger) detectConflict(field string, values []fieldValue) *models.MergeConflict {
	if len(values) < 2 {
		return nil
	}

	// Check if all values are the same
	allSame := true
	first := fmt.Sprintf("%v", values[0].Value)
	for i := 1; i < len(values); i++ {
		if fmt.Sprintf("%v", values[i].Value) != first {
			allSame = false
			break
		}
	}

	if allSame {
		return nil
	}

	// There's a conflict
	conflictValues := make([]any, len(values))
	integrations := make([]string, len(values))
	for i, v := range values {
		conflictValues[i] = v.Value
		integrations[i] = v.Integration
	}

	return &models.MergeConflict{
		Field:        field,
		Values:       conflictValues,
		Integrations: integrations,
	}
}

// mostRecent returns the most recently updated value
func (m *FieldMerger) mostRecent(values []fieldValue) any {
	if len(values) == 0 {
		return nil
	}

	sort.Slice(values, func(i, j int) bool {
		ti := toTime(values[i].UpdatedAt)
		tj := toTime(values[j].UpdatedAt)
		return ti.After(tj)
	})

	return values[0].Value
}

// mostTrusted returns the value from the highest priority source
func (m *FieldMerger) mostTrusted(values []fieldValue, priorities map[string]int) any {
	if len(values) == 0 {
		return nil
	}

	sort.Slice(values, func(i, j int) bool {
		pi := priorities[values[i].Integration]
		pj := priorities[values[j].Integration]
		return pi > pj // Higher priority first
	})

	return values[0].Value
}

// collectAll combines all values into an array
func (m *FieldMerger) collectAll(values []fieldValue, maxItems int, dedup bool) any {
	result := make([]any, 0, len(values))
	seen := make(map[string]bool)

	for _, v := range values {
		key := fmt.Sprintf("%v", v.Value)
		if dedup {
			if seen[key] {
				continue
			}
			seen[key] = true
		}

		// Flatten if value is already an array
		if reflect.TypeOf(v.Value).Kind() == reflect.Slice {
			rv := reflect.ValueOf(v.Value)
			for i := 0; i < rv.Len(); i++ {
				elem := rv.Index(i).Interface()
				elemKey := fmt.Sprintf("%v", elem)
				if dedup && seen[elemKey] {
					continue
				}
				if dedup {
					seen[elemKey] = true
				}
				result = append(result, elem)
				if maxItems > 0 && len(result) >= maxItems {
					return result
				}
			}
		} else {
			result = append(result, v.Value)
			if maxItems > 0 && len(result) >= maxItems {
				return result
			}
		}
	}

	return result
}

// longest returns the longest string value
func (m *FieldMerger) longest(values []fieldValue) any {
	if len(values) == 0 {
		return nil
	}

	var longest any
	maxLen := -1

	for _, v := range values {
		s := fmt.Sprintf("%v", v.Value)
		if len(s) > maxLen {
			maxLen = len(s)
			longest = v.Value
		}
	}

	return longest
}

// shortest returns the shortest non-empty string value
func (m *FieldMerger) shortest(values []fieldValue) any {
	if len(values) == 0 {
		return nil
	}

	var shortest any
	minLen := int(^uint(0) >> 1) // Max int

	for _, v := range values {
		s := fmt.Sprintf("%v", v.Value)
		if len(s) > 0 && len(s) < minLen {
			minLen = len(s)
			shortest = v.Value
		}
	}

	return shortest
}

// first returns the first value (oldest by update time)
func (m *FieldMerger) first(values []fieldValue) any {
	if len(values) == 0 {
		return nil
	}

	sort.Slice(values, func(i, j int) bool {
		ti := toTime(values[i].UpdatedAt)
		tj := toTime(values[j].UpdatedAt)
		return ti.Before(tj)
	})

	return values[0].Value
}

// last returns the last value (most recent by update time)
func (m *FieldMerger) last(values []fieldValue) any {
	return m.mostRecent(values)
}

// max returns the maximum numeric value
func (m *FieldMerger) max(values []fieldValue) any {
	if len(values) == 0 {
		return nil
	}

	var maxVal float64
	var found bool

	for _, v := range values {
		num, ok := toNumber(v.Value)
		if !ok {
			continue
		}
		if !found || num > maxVal {
			maxVal = num
			found = true
		}
	}

	if !found {
		return nil
	}
	return maxVal
}

// min returns the minimum numeric value
func (m *FieldMerger) min(values []fieldValue) any {
	if len(values) == 0 {
		return nil
	}

	var minVal float64
	var found bool

	for _, v := range values {
		num, ok := toNumber(v.Value)
		if !ok {
			continue
		}
		if !found || num < minVal {
			minVal = num
			found = true
		}
	}

	if !found {
		return nil
	}
	return minVal
}

// sum returns the sum of numeric values
func (m *FieldMerger) sum(values []fieldValue) any {
	var sum float64

	for _, v := range values {
		num, ok := toNumber(v.Value)
		if ok {
			sum += num
		}
	}

	return sum
}

// average returns the average of numeric values
func (m *FieldMerger) average(values []fieldValue) any {
	var sum float64
	var count int

	for _, v := range values {
		num, ok := toNumber(v.Value)
		if ok {
			sum += num
			count++
		}
	}

	if count == 0 {
		return nil
	}
	return sum / float64(count)
}

// preferNonEmpty returns the first non-empty value
func (m *FieldMerger) preferNonEmpty(values []fieldValue) any {
	for _, v := range values {
		if !isEmpty(v.Value) {
			return v.Value
		}
	}
	if len(values) > 0 {
		return values[0].Value
	}
	return nil
}

// Helper types and functions

type fieldValue struct {
	Value       any
	UpdatedAt   interface{}
	Integration string
	EntityID    string
}

func toTime(v interface{}) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t
	default:
		return time.Time{}
	}
}

func toNumber(v any) (float64, bool) {
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
	default:
		return 0, false
	}
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case []any:
		return len(val) == 0
	case map[string]any:
		return len(val) == 0
	default:
		return false
	}
}
