package steps

import (
	"encoding/json"
	"fmt"
	"io"
)

// CreateEntityType implements the create_entity_type step
func CreateEntityType(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("create_entity_type params must be a map")
	}

	name, ok := paramsMap["name"].(string)
	if !ok {
		return fmt.Errorf("create_entity_type requires 'name'")
	}

	// Interpolate name
	name = ctx.Interpolate(name).(string)

	ctx.Log("Creating Ivy entity type: %s", name)

	// Build request body
	body := map[string]interface{}{
		"name": name,
	}

	// Add optional fields
	if description, ok := paramsMap["description"].(string); ok {
		body["description"] = ctx.Interpolate(description)
	}

	if schema, ok := paramsMap["schema"]; ok {
		body["schema"] = ctx.Interpolate(schema)
	}

	// Make request
	resp, err := ctx.HTTPRequest("POST", "ivy", "/api/v1/entity-types", nil, body)
	if err != nil {
		return fmt.Errorf("failed to create entity type: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("create entity type failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Save entity type ID if requested
	if saveAs, ok := paramsMap["save_as"].(string); ok {
		if id, ok := result["id"]; ok {
			ctx.Set(saveAs, id)
			ctx.Log("Entity type ID saved as: %s = %v", saveAs, id)
		}
	}

	return nil
}

// CreateMatchRule implements the create_match_rule step
func CreateMatchRule(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("create_match_rule params must be a map")
	}

	entityType, ok := paramsMap["entity_type"]
	if !ok {
		return fmt.Errorf("create_match_rule requires 'entity_type'")
	}

	conditions, ok := paramsMap["conditions"].([]interface{})
	if !ok {
		return fmt.Errorf("create_match_rule requires 'conditions'")
	}

	// Interpolate values
	entityType = ctx.Interpolate(entityType)
	conditions = ctx.Interpolate(conditions).([]interface{})

	ctx.Log("Creating Ivy match rule for entity type: %v", entityType)

	// Build request body
	body := map[string]interface{}{
		"entity_type": entityType,
		"conditions":  conditions,
	}

	// Add optional fields
	if name, ok := paramsMap["name"].(string); ok {
		body["name"] = ctx.Interpolate(name)
	}

	if description, ok := paramsMap["description"].(string); ok {
		body["description"] = ctx.Interpolate(description)
	}

	if threshold, ok := paramsMap["threshold"]; ok {
		body["threshold"] = ctx.Interpolate(threshold)
	}

	// Make request
	resp, err := ctx.HTTPRequest("POST", "ivy", "/api/v1/match-rules", nil, body)
	if err != nil {
		return fmt.Errorf("failed to create match rule: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("create match rule failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Save match rule ID if requested
	if saveAs, ok := paramsMap["save_as"].(string); ok {
		if id, ok := result["id"]; ok {
			ctx.Set(saveAs, id)
			ctx.Log("Match rule ID saved as: %s = %v", saveAs, id)
		}
	}

	return nil
}

// QueryEntities implements the query_entities step
func QueryEntities(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("query_entities params must be a map")
	}

	entityType, ok := paramsMap["entity_type"]
	if !ok {
		return fmt.Errorf("query_entities requires 'entity_type'")
	}

	// Interpolate entity type
	entityType = ctx.Interpolate(entityType)

	ctx.Log("Querying Ivy entities of type: %v", entityType)

	// Build query parameters
	queryParams := map[string]interface{}{
		"entity_type": entityType,
	}

	// Add filters if provided
	if filters, ok := paramsMap["filters"].(map[string]interface{}); ok {
		for k, v := range filters {
			queryParams[k] = ctx.Interpolate(v)
		}
	}

	// Make request (assuming query is via POST with body)
	resp, err := ctx.HTTPRequest("POST", "ivy", "/api/v1/entities/query", nil, queryParams)
	if err != nil {
		return fmt.Errorf("failed to query entities: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("query entities failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Save results if requested
	if saveAs, ok := paramsMap["save_as"].(string); ok {
		ctx.Set(saveAs, result)
		ctx.Log("Query results saved as: %s", saveAs)
	}

	// Check expectations
	if expect, ok := paramsMap["expect"].(map[string]interface{}); ok {
		// Check count
		if expectedCount, ok := expect["count"].(int); ok {
			var actualCount int
			if items, ok := result["items"].([]interface{}); ok {
				actualCount = len(items)
			} else if count, ok := result["count"].(float64); ok {
				actualCount = int(count)
			}

			if actualCount != expectedCount {
				return fmt.Errorf("expected %d entities, got %d", expectedCount, actualCount)
			}
			ctx.Log("Entity count matches: %d", actualCount)
		}

		// Check items
		if expectedItems, ok := expect["items"].([]interface{}); ok {
			actualItems, ok := result["items"].([]interface{})
			if !ok {
				return fmt.Errorf("no items in query result")
			}

			if len(expectedItems) != len(actualItems) {
				return fmt.Errorf("expected %d items, got %d", len(expectedItems), len(actualItems))
			}

			// Check each item
			for i, expectedItemInterface := range expectedItems {
				expectedItem, ok := expectedItemInterface.(map[string]interface{})
				if !ok {
					return fmt.Errorf("expected item %d is not a map", i)
				}

				actualItem, ok := actualItems[i].(map[string]interface{})
				if !ok {
					return fmt.Errorf("actual item %d is not a map", i)
				}

				// Check each expected field
				for fieldPath, expectedVal := range expectedItem {
					// Navigate nested fields
					actualVal, err := getNestedField(actualItem, fieldPath)
					if err != nil {
						return fmt.Errorf("item %d: %w", i, err)
					}

					expectedValInterpolated := ctx.Interpolate(expectedVal)
					if fmt.Sprintf("%v", actualVal) != fmt.Sprintf("%v", expectedValInterpolated) {
						return fmt.Errorf("item %d, field %s: expected %v, got %v", i, fieldPath, expectedValInterpolated, actualVal)
					}
				}
			}

			ctx.Log("All item assertions passed")
		}
	}

	return nil
}

// getNestedField retrieves a nested field value using dot notation (e.g., "data.email")
func getNestedField(m map[string]interface{}, path string) (interface{}, error) {
	parts := splitPath(path)
	var current interface{} = m

	for _, part := range parts {
		if currentMap, ok := current.(map[string]interface{}); ok {
			var found bool
			current, found = currentMap[part]
			if !found {
				return nil, fmt.Errorf("field %s not found (missing: %s)", path, part)
			}
		} else {
			return nil, fmt.Errorf("field %s: cannot navigate into %T", path, current)
		}
	}

	return current, nil
}

// splitPath splits a field path by dots
func splitPath(path string) []string {
	// Simple split - doesn't handle escaped dots
	result := []string{}
	current := ""

	for _, ch := range path {
		if ch == '.' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}
