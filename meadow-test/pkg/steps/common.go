package steps

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// toFloat converts common numeric types (and numeric strings) to float64 for comparisons.
func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// TestContext interface for steps to interact with test execution context
type TestContext interface {
	Set(key string, value interface{})
	Get(key string) (interface{}, bool)
	Interpolate(input interface{}) interface{}
	HTTPRequest(method, serviceOrURL, path string, headers map[string]string, body interface{}) (*http.Response, error)
	Log(format string, args ...interface{})
	Error(format string, args ...interface{})
	StartKafkaConsumer(topic string, startOffset int64) error
	GetKafkaConsumer() interface{}                        // Returns *kafka.BackgroundConsumer but using interface{} to avoid import cycle
	GetKafkaConsumerForTopic(topic string) interface{}    // Returns *kafka.BackgroundConsumer for a specific topic
}

// Wait implements the wait step
func Wait(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("wait params must be a map")
	}

	durationStr, ok := paramsMap["duration"].(string)
	if !ok {
		return fmt.Errorf("wait duration must be a string")
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	reason := ""
	if r, ok := paramsMap["reason"].(string); ok {
		reason = r
	}

	if reason != "" {
		ctx.Log("Waiting %s (%s)", duration, reason)
	} else {
		ctx.Log("Waiting %s", duration)
	}

	time.Sleep(duration)
	return nil
}

// Assert implements the assert step
func Assert(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("assert params must be a map")
	}

	// Get error message
	message := "Assertion failed"
	if msg, ok := paramsMap["message"].(string); ok {
		message = ctx.Interpolate(msg).(string)
	}

	// Check for condition (boolean expression)
	if condition, ok := paramsMap["condition"].(string); ok {
		// Simple condition evaluation
		interpolated := ctx.Interpolate(condition).(string)

		// Basic checks
		if strings.Contains(interpolated, "!=") {
			parts := strings.Split(interpolated, "!=")
			if len(parts) == 2 {
				left := strings.TrimSpace(parts[0])
				right := strings.TrimSpace(parts[1])
				if left == right {
					return fmt.Errorf("%s: %s != %s failed (both are %s)", message, parts[0], parts[1], left)
				}
			}
		} else if strings.Contains(interpolated, "==") {
			parts := strings.Split(interpolated, "==")
			if len(parts) == 2 {
				left := strings.TrimSpace(parts[0])
				right := strings.TrimSpace(parts[1])
				if left != right {
					return fmt.Errorf("%s: %s == %s failed (%s vs %s)", message, parts[0], parts[1], left, right)
				}
			}
		}
		return nil
	}

	// Check for variable comparison
	if variable, ok := paramsMap["variable"].(string); ok {
		// Support nested variable paths (e.g., "response.id")
		val := resolveNestedVariable(ctx, variable)

		if equals, ok := paramsMap["equals"]; ok {
			expectedVal := ctx.Interpolate(equals)
			if !compareValuesForAssert(val, expectedVal) {
				// For array access failures, provide more context
				if strings.Contains(variable, "[") && val == nil {
					// Try to get the parent array to see if it's empty
					parts := strings.Split(variable, "[")
					if len(parts) > 0 {
						arrayName := parts[0]
						if arrayVar, found := ctx.Get(arrayName); found {
							if arr, ok := arrayVar.([]interface{}); ok {
								return fmt.Errorf("%s: %s is nil (array %s has %d elements)", message, variable, arrayName, len(arr))
							}
						}
					}
				}
				return fmt.Errorf("%s: %s = %v (type %T), expected %v (type %T)", message, variable, val, val, expectedVal, expectedVal)
			}
		}

		// Numeric comparisons
		if gt, ok := paramsMap["is_greater_than"]; ok {
			expectedVal := ctx.Interpolate(gt)
			actualNum, okA := toFloat(val)
			expectedNum, okE := toFloat(expectedVal)
			if !okA || !okE {
				return fmt.Errorf("%s: %s = %v (type %T), expected numeric > %v (type %T)", message, variable, val, val, expectedVal, expectedVal)
			}
			if !(actualNum > expectedNum) {
				return fmt.Errorf("%s: %s = %v is not > %v", message, variable, actualNum, expectedNum)
			}
		}

		if lt, ok := paramsMap["is_less_than"]; ok {
			expectedVal := ctx.Interpolate(lt)
			actualNum, okA := toFloat(val)
			expectedNum, okE := toFloat(expectedVal)
			if !okA || !okE {
				return fmt.Errorf("%s: %s = %v (type %T), expected numeric < %v (type %T)", message, variable, val, val, expectedVal, expectedVal)
			}
			if !(actualNum < expectedNum) {
				return fmt.Errorf("%s: %s = %v is not < %v", message, variable, actualNum, expectedNum)
			}
		}

		if notEquals, ok := paramsMap["not_equals"]; ok {
			unexpectedVal := ctx.Interpolate(notEquals)
			if compareValuesForAssert(val, unexpectedVal) {
				return fmt.Errorf("%s: %s = %v, expected not equal to %v", message, variable, val, unexpectedVal)
			}
		}

		// Check not_empty
		if notEmpty, ok := paramsMap["not_empty"].(bool); ok && notEmpty {
			if val == nil {
				return fmt.Errorf("%s: %s is nil", message, variable)
			}
			// Check for empty string
			if str, ok := val.(string); ok && str == "" {
				return fmt.Errorf("%s: %s is empty string", message, variable)
			}
			// Check for empty slice/array
			if reflect.ValueOf(val).Kind() == reflect.Slice && reflect.ValueOf(val).Len() == 0 {
				return fmt.Errorf("%s: %s is empty array", message, variable)
			}
		}

		// Check contains (for string substring matching)
		if contains, ok := paramsMap["contains"]; ok {
			containsStr := ctx.Interpolate(contains).(string)
			if val == nil {
				return fmt.Errorf("%s: %s is nil, cannot check contains", message, variable)
			}
			valStr := fmt.Sprintf("%v", val)
			if !strings.Contains(valStr, containsStr) {
				return fmt.Errorf("%s: %s = %q does not contain %q", message, variable, valStr, containsStr)
			}
		}

		return nil
	}

	return fmt.Errorf("assert requires either 'condition' or 'variable' parameter")
}

// resolveNestedVariable resolves a variable path like "response.id", "response.data.name", or "executions[0].status"
func resolveNestedVariable(ctx TestContext, path string) interface{} {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil
	}

	// Check if root has array index (e.g., "executions[0]")
	rootPart := parts[0]
	var val interface{}
	var found bool

	if strings.Contains(rootPart, "[") && strings.HasSuffix(rootPart, "]") {
		// Parse root variable and array index
		bracketIdx := strings.Index(rootPart, "[")
		rootVar := rootPart[:bracketIdx]
		indexStr := rootPart[bracketIdx+1 : len(rootPart)-1]
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			return nil
		}

		val, found = ctx.Get(rootVar)
		if !found {
			return nil
		}

		// Access array element (support common slice types)
		switch arr := val.(type) {
		case []interface{}:
			if index < 0 || index >= len(arr) {
				return nil
			}
			val = arr[index]
		case []map[string]interface{}:
			if index < 0 || index >= len(arr) {
				return nil
			}
			val = arr[index]
		default:
			rv := reflect.ValueOf(val)
			if rv.IsValid() && rv.Kind() == reflect.Slice {
				if index < 0 || index >= rv.Len() {
					return nil
				}
				val = rv.Index(index).Interface()
			} else {
				return nil
			}
		}
	} else {
		val, found = ctx.Get(parts[0])
		if !found {
			return nil
		}
	}

	// Navigate nested paths
	for i := 1; i < len(parts); i++ {
		if val == nil {
			return nil
		}

		part := parts[i]

		// Check for array index in path part (e.g., "items[0]")
		if strings.Contains(part, "[") && strings.HasSuffix(part, "]") {
			bracketIdx := strings.Index(part, "[")
			fieldName := part[:bracketIdx]
			indexStr := part[bracketIdx+1 : len(part)-1]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return nil
			}

			// First get the field
			switch v := val.(type) {
			case map[string]interface{}:
				val = v[fieldName]
			default:
				return nil
			}

			// Then access array element (support common slice types)
			switch arr := val.(type) {
			case []interface{}:
				if index < 0 || index >= len(arr) {
					return nil
				}
				val = arr[index]
			case []map[string]interface{}:
				if index < 0 || index >= len(arr) {
					return nil
				}
				val = arr[index]
			default:
				rv := reflect.ValueOf(val)
				if rv.IsValid() && rv.Kind() == reflect.Slice {
					if index < 0 || index >= rv.Len() {
						return nil
					}
					val = rv.Index(index).Interface()
				} else {
					return nil
				}
			}
		} else {
			switch v := val.(type) {
			case map[string]interface{}:
				val = v[part]
			default:
				// Can't navigate further
				return nil
			}
		}
	}

	return val
}

// compareValuesForAssert compares two values, handling numeric type differences
// (YAML parses numbers as int, JSON as float64)
func compareValuesForAssert(actual, expected interface{}) bool {
	// Try numeric comparison first
	actualNum, actualIsNum := toFloat64Assert(actual)
	expectedNum, expectedIsNum := toFloat64Assert(expected)

	if actualIsNum && expectedIsNum {
		return actualNum == expectedNum
	}

	// Try boolean comparison
	if actualBool, ok := actual.(bool); ok {
		if expectedBool, ok := expected.(bool); ok {
			return actualBool == expectedBool
		}
	}

	// Fall back to string comparison for mixed types
	if actualIsNum != expectedIsNum {
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	}

	// Use reflect.DeepEqual for non-numeric types
	return reflect.DeepEqual(actual, expected)
}

// toFloat64Assert converts numeric types to float64 for comparison
func toFloat64Assert(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	default:
		return 0, false
	}
}

// PollUntil implements the poll_until step - repeatedly checks a condition until it's true or timeout
func PollUntil(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("poll_until params must be a map")
	}

	// Parse timeout (default 30s)
	timeoutStr := "30s"
	if t, ok := paramsMap["timeout"].(string); ok {
		timeoutStr = t
	}
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return fmt.Errorf("invalid timeout duration: %w", err)
	}

	// Parse interval (default 1s)
	intervalStr := "1s"
	if i, ok := paramsMap["interval"].(string); ok {
		intervalStr = i
	}
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		return fmt.Errorf("invalid interval duration: %w", err)
	}

	// Get the condition to check - must be http_request or assert
	check, ok := paramsMap["check"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("poll_until requires 'check' parameter with http_request or assert")
	}

	reason := ""
	if r, ok := paramsMap["reason"].(string); ok {
		reason = r
	}

	if reason != "" {
		ctx.Log("Polling until condition met: %s (timeout=%s, interval=%s)", reason, timeout, interval)
	} else {
		ctx.Log("Polling until condition met (timeout=%s, interval=%s)", timeout, interval)
	}

	startTime := time.Now()
	attempts := 0

	for {
		attempts++
		elapsed := time.Since(startTime)

		if elapsed >= timeout {
			return fmt.Errorf("poll_until timed out after %s (%d attempts)", timeout, attempts)
		}

		// Execute the check - can be http_request followed by optional assert
		var checkErr error

		// First handle http_request if present
		if httpReq, hasHTTP := check["http_request"]; hasHTTP {
			checkErr = HTTPRequest(ctx, httpReq)
			if checkErr != nil {
				// HTTP request failed, retry
				if attempts%5 == 0 {
					ctx.Log("Poll attempt %d: HTTP request failed (will retry): %v", attempts, checkErr)
				}
				time.Sleep(interval)
				continue
			}

			// HTTP succeeded, now check assertion if present
			if assertCheck, hasAssert := check["assert"]; hasAssert {
				checkErr = Assert(ctx, assertCheck)
			}
		} else if assertCheck, hasAssert := check["assert"]; hasAssert {
			// Only assertion, no http_request
			checkErr = Assert(ctx, assertCheck)
		} else {
			return fmt.Errorf("poll_until check must contain 'http_request' and/or 'assert'")
		}

		if checkErr == nil {
			ctx.Log("Poll condition met after %d attempts (%s)", attempts, elapsed.Round(time.Millisecond))
			return nil
		}

		// Log failure every 5 attempts to avoid spam
		if attempts%5 == 0 {
			ctx.Log("Poll attempt %d: condition not met (will retry): %v", attempts, checkErr)
		}

		// Wait before next attempt
		time.Sleep(interval)
	}
}

// HTTPRequest implements the http_request step
func HTTPRequest(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("http_request params must be a map")
	}

	// Extract parameters
	service, ok := paramsMap["service"].(string)
	if !ok {
		// Try url field
		if url, ok := paramsMap["url"].(string); ok {
			service = url
		} else {
			return fmt.Errorf("http_request requires 'service' or 'url'")
		}
	}

	method := "GET"
	if m, ok := paramsMap["method"].(string); ok {
		method = strings.ToUpper(m)
	}

	path := ""
	if p, ok := paramsMap["path"].(string); ok {
		path = p
	}

	headers := make(map[string]string)
	if h, ok := paramsMap["headers"].(map[string]interface{}); ok {
		for k, v := range h {
			if str, ok := v.(string); ok {
				headers[k] = str
			}
		}
	}

	var body interface{}
	if b, ok := paramsMap["body"]; ok {
		body = b
	}

	// Make request
	resp, err := ctx.HTTPRequest(method, service, path, headers, body)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Save response if requested
	if saveAs, ok := paramsMap["save_as"].(string); ok {
		var responseData interface{}
		if err := json.Unmarshal(respBody, &responseData); err == nil {
			ctx.Set(saveAs, responseData)
		} else {
			// Not JSON, save as string
			ctx.Set(saveAs, string(respBody))
		}
	}

	// Check expectations
	if expect, ok := paramsMap["expect"].(map[string]interface{}); ok {
		// Check status code (YAML may parse as int or float64)
		if status, ok := expect["status"]; ok {
			var expectedStatus int
			switch s := status.(type) {
			case int:
				expectedStatus = s
			case float64:
				expectedStatus = int(s)
			case int64:
				expectedStatus = int(s)
			default:
				return fmt.Errorf("invalid status type: %T", status)
			}

			if resp.StatusCode != expectedStatus {
				return fmt.Errorf("expected status %d, got %d: %s", expectedStatus, resp.StatusCode, string(respBody))
			}
		}

		// Check status_one_of - allows multiple acceptable status codes
		if statusList, ok := expect["status_one_of"]; ok {
			var validStatuses []int
			switch sl := statusList.(type) {
			case []interface{}:
				for _, s := range sl {
					switch sv := s.(type) {
					case int:
						validStatuses = append(validStatuses, sv)
					case float64:
						validStatuses = append(validStatuses, int(sv))
					case int64:
						validStatuses = append(validStatuses, int(sv))
					}
				}
			}
			found := false
			for _, valid := range validStatuses {
				if resp.StatusCode == valid {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("expected status to be one of %v, got %d: %s", validStatuses, resp.StatusCode, string(respBody))
			}
		}

		// Check response body
		if expectedBody, ok := expect["body"]; ok {
			var actualBody interface{}
			if err := json.Unmarshal(respBody, &actualBody); err != nil {
				return fmt.Errorf("failed to parse response as JSON: %w", err)
			}

			// Compare (simplified - just check some fields)
			if err := compareJSON(expectedBody, actualBody, ctx); err != nil {
				return fmt.Errorf("response body mismatch: %w", err)
			}
		}
	}

	return nil
}

// compareJSON compares expected and actual JSON values (simplified comparison)
func compareJSON(expected, actual interface{}, ctx TestContext) error {
	// Interpolate expected values
	expected = ctx.Interpolate(expected)

	switch exp := expected.(type) {
	case map[string]interface{}:
		actMap, ok := actual.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected map, got %T", actual)
		}

		for key, expectedVal := range exp {
			actualVal, ok := actMap[key]
			if !ok {
				return fmt.Errorf("missing field: %s", key)
			}

			if err := compareJSON(expectedVal, actualVal, ctx); err != nil {
				return fmt.Errorf("field %s: %w", key, err)
			}
		}
		return nil

	case []interface{}:
		actArr, ok := actual.([]interface{})
		if !ok {
			return fmt.Errorf("expected array, got %T", actual)
		}

		if len(exp) != len(actArr) {
			return fmt.Errorf("expected array length %d, got %d", len(exp), len(actArr))
		}

		for i, expectedVal := range exp {
			if err := compareJSON(expectedVal, actArr[i], ctx); err != nil {
				return fmt.Errorf("index %d: %w", i, err)
			}
		}
		return nil

	default:
		if !reflect.DeepEqual(expected, actual) {
			return fmt.Errorf("expected %v, got %v", expected, actual)
		}
		return nil
	}
}
