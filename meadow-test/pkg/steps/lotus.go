package steps

import (
	"encoding/json"
	"fmt"
	"io"
)

// CreateMapping implements the create_mapping step
func CreateMapping(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("create_mapping params must be a map")
	}

	name, ok := paramsMap["name"].(string)
	if !ok {
		return fmt.Errorf("create_mapping requires 'name'")
	}

	// Interpolate name
	name = ctx.Interpolate(name).(string)

	ctx.Log("Creating Lotus mapping: %s", name)

	// Build request body with interpolated values
	body := map[string]interface{}{
		"name": name,
	}

	// Add source_fields if provided
	if sourceFields, ok := paramsMap["source_fields"]; ok {
		body["source_fields"] = ctx.Interpolate(sourceFields)
	}

	// Add target_fields if provided
	if targetFields, ok := paramsMap["target_fields"]; ok {
		body["target_fields"] = ctx.Interpolate(targetFields)
	}

	// Add links if provided
	if links, ok := paramsMap["links"]; ok {
		body["links"] = ctx.Interpolate(links)
	}

	// Add transformation steps if provided
	if steps, ok := paramsMap["steps"]; ok {
		body["steps"] = ctx.Interpolate(steps)
	}

	// Add description if provided
	if description, ok := paramsMap["description"].(string); ok {
		body["description"] = ctx.Interpolate(description)
	}

	// Make request
	resp, err := ctx.HTTPRequest("POST", "lotus", "/api/v1/mappings/definitions", nil, body)
	if err != nil {
		return fmt.Errorf("failed to create mapping: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("create mapping failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Save mapping ID if requested
	if saveAs, ok := paramsMap["save_as"].(string); ok {
		if id, ok := result["id"]; ok {
			ctx.Set(saveAs, id)
			ctx.Log("Mapping ID saved as: %s = %v", saveAs, id)
		}
	}

	return nil
}

// CreateBinding implements the create_binding step
func CreateBinding(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("create_binding params must be a map")
	}

	mappingID, ok := paramsMap["mapping_id"]
	if !ok {
		return fmt.Errorf("create_binding requires 'mapping_id'")
	}

	// Interpolate mapping ID
	mappingID = ctx.Interpolate(mappingID)

	ctx.Log("Creating Lotus binding for mapping: %v", mappingID)

	// Build request body
	body := map[string]interface{}{
		"mapping_id": mappingID,
	}

	// Add filter if provided
	if filter, ok := paramsMap["filter"].(map[string]interface{}); ok {
		body["filter"] = ctx.Interpolate(filter)
	}

	// Add name if provided
	if name, ok := paramsMap["name"].(string); ok {
		body["name"] = ctx.Interpolate(name)
	}

	// Add enabled flag if provided
	if enabled, ok := paramsMap["enabled"].(bool); ok {
		body["enabled"] = enabled
	}

	// Make request
	resp, err := ctx.HTTPRequest("POST", "lotus", "/api/v1/bindings", nil, body)
	if err != nil {
		return fmt.Errorf("failed to create binding: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("create binding failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Save binding ID if requested
	if saveAs, ok := paramsMap["save_as"].(string); ok {
		if id, ok := result["id"]; ok {
			ctx.Set(saveAs, id)
			ctx.Log("Binding ID saved as: %s = %v", saveAs, id)
		}
	}

	return nil
}
