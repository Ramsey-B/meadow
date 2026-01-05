package utils

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const (
	SplitToken     = "."
	IndexCloseChar = "]"
	IndexOpenChar  = "["
)

var (
	ErrMalformedIndex    = errors.New("malformed index key")
	ErrInvalidIndexUsage = errors.New("invalid index key usage")
	ErrIndexOutOfBounds  = errors.New("index out of bounds")
	ErrNonStringMapKey   = errors.New("map key type is not string")
)

// GetFieldByPath performs a lookup into a value, using a string. Same as `Lookup`
// but using a string with the keys separated by `.`
func GetFieldByPath(i any, path ...string) (reflect.Value, error) {
	if len(path) == 1 {
		path = strings.Split(path[0], SplitToken)
	}

	return getFieldByPath(i, false, path...)
}

// GetFieldByPathI is the same as GetFieldByPath, but the path is not case
// sensitive.
func GetFieldByPathI(i any, path ...string) (reflect.Value, error) {
	if len(path) == 1 {
		path = strings.Split(path[0], SplitToken)
	}

	return getFieldByPath(i, true, path...)
}

func getFieldByPath(i any, caseInsensitive bool, path ...string) (reflect.Value, error) {
	if len(path) == 0 || (len(path) == 1 && path[0] == "") {
		return reflect.ValueOf(i), nil
	}

	value := reflect.ValueOf(i)
	var parent reflect.Value
	var err error

	for i, part := range path {
		parent = value

		value, err = getValueByName(value, part, caseInsensitive)
		if err == nil {
			continue
		}

		if !isAggregable(parent) {
			break
		}

		value, err = aggreateAggregableValue(parent, path[i:])
		break
	}

	return value, err
}

func getValueByName(v reflect.Value, key string, caseInsensitive bool) (reflect.Value, error) {
	var value reflect.Value
	index := -1
	var err error

	prevKey := key
	key, index, err = parseIndex(key)
	if err != nil {
		return value, err
	}
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		return getValueByName(v.Elem(), prevKey, caseInsensitive)
	case reflect.Struct:
		value = v.FieldByName(key)

		if caseInsensitive && value.Kind() == reflect.Invalid {
			// Iterate over fields to find a case-insensitive match.
			for i := 0; i < v.NumField(); i++ {
				if strings.EqualFold(v.Type().Field(i).Name, key) {
					value = v.Field(i)
					break
				}
			}
		}

	case reflect.Map:
		// Ensure that the map key is of type string.
		if v.Type().Key().Kind() != reflect.String {
			return reflect.Value{}, ErrNonStringMapKey
		}
		kValue := reflect.ValueOf(key)
		value = v.MapIndex(kValue)
		if caseInsensitive && !value.IsValid() {
			iter := v.MapRange()
			for iter.Next() {
				if strings.EqualFold(key, iter.Key().String()) {
					kValue = iter.Key()
					value = v.MapIndex(kValue)
					break
				}
			}
		}
	}

	if !value.IsValid() {
		return reflect.Value{}, fmt.Errorf("unable to find the key '%s'", key)
	}

	if index != -1 {
		if value.Kind() == reflect.Ptr {
			value = value.Elem()
		}

		if value.Type().Kind() != reflect.Slice {
			return reflect.Value{}, ErrInvalidIndexUsage
		}

		if value.Len() <= index {
			return reflect.Value{}, ErrIndexOutOfBounds
		}

		value = value.Index(index)
	}

	if value.Kind() == reflect.Ptr || value.Kind() == reflect.Interface {
		value = value.Elem()
	}

	return value, nil
}

func aggreateAggregableValue(v reflect.Value, path []string) (reflect.Value, error) {
	values := make([]reflect.Value, 0)
	l := v.Len()
	// If the parent is a map and it's empty, return an error.
	if l == 0 {
		if v.Kind() == reflect.Map {
			return reflect.Value{}, fmt.Errorf("unable to find the key '%s'", strings.Join(path, "."))
		}
		ty, ok := lookupType(v.Type(), path...)
		if !ok {
			return reflect.Value{}, fmt.Errorf("unable to find the key '%s'", strings.Join(path, "."))
		}
		return reflect.MakeSlice(reflect.SliceOf(ty), 0, 0), nil
	}

	index := indexFunction(v)
	for i := 0; i < l; i++ {
		value, err := GetFieldByPath(index(i).Interface(), path...)
		if err != nil {
			return reflect.Value{}, err
		}

		values = append(values, value)
	}

	return mergeValue(values), nil
}

func indexFunction(v reflect.Value) func(i int) reflect.Value {
	switch v.Kind() {
	case reflect.Slice:
		return v.Index
	case reflect.Map:
		keys := v.MapKeys()
		return func(i int) reflect.Value {
			return v.MapIndex(keys[i])
		}
	default:
		panic("unsupported kind for index")
	}
}

func mergeValue(values []reflect.Value) reflect.Value {
	values = removeZeroValues(values)
	l := len(values)
	if l == 0 {
		return reflect.Value{}
	}

	sample := values[0]
	mergeable := isMergeable(sample)
	t := sample.Type()
	if mergeable {
		t = t.Elem()
	}

	value := reflect.MakeSlice(reflect.SliceOf(t), 0, 0)
	for i := 0; i < l; i++ {
		if !values[i].IsValid() {
			continue
		}

		if mergeable {
			value = reflect.AppendSlice(value, values[i])
		} else {
			value = reflect.Append(value, values[i])
		}
	}

	return value
}

func removeZeroValues(values []reflect.Value) []reflect.Value {
	l := len(values)
	var v []reflect.Value
	for i := 0; i < l; i++ {
		if values[i].IsValid() {
			v = append(v, values[i])
		}
	}
	return v
}

func isAggregable(v reflect.Value) bool {
	k := v.Kind()
	return k == reflect.Map || k == reflect.Slice
}

func isMergeable(v reflect.Value) bool {
	return v.Kind() == reflect.Slice
}

func hasIndex(s string) bool {
	return strings.Contains(s, IndexOpenChar)
}

func parseIndex(s string) (string, int, error) {
	start := strings.Index(s, IndexOpenChar)
	end := strings.Index(s, IndexCloseChar)

	if start == -1 && end == -1 {
		return s, -1, nil
	}

	if (start != -1 && end == -1) || (start == -1 && end != -1) {
		return "", -1, ErrMalformedIndex
	}

	indexStr := s[start+1 : end]
	// Treat wildcard indexes like `items[*]` as "no explicit index".
	// Aggregation over slices is already handled elsewhere by traversing into the slice elements.
	if indexStr == "*" {
		return s[:start], -1, nil
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return "", -1, ErrMalformedIndex
	}

	return s[:start], index, nil
}

func lookupType(ty reflect.Type, path ...string) (reflect.Type, bool) {
	if len(path) == 0 {
		return ty, true
	}

	switch ty.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		if hasIndex(path[0]) {
			return lookupType(ty.Elem(), path[1:]...)
		}
		// Aggregate.
		return lookupType(ty.Elem(), path...)
	case reflect.Ptr:
		return lookupType(ty.Elem(), path...)
	case reflect.Interface:
		// We can't know from here without a value. Let's just return this type.
		return ty, true
	case reflect.Struct:
		f, ok := ty.FieldByName(path[0])
		if ok {
			return lookupType(f.Type, path[1:]...)
		}
	}
	return nil, false
}
