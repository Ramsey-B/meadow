package steps

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// CreateIntegration implements the create_integration step
func CreateIntegration(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("create_integration params must be a map")
	}

	name, ok := paramsMap["name"].(string)
	if !ok {
		return fmt.Errorf("create_integration requires 'name'")
	}

	integType, ok := paramsMap["type"].(string)
	if !ok {
		return fmt.Errorf("create_integration requires 'type'")
	}

	// Interpolate values
	name = ctx.Interpolate(name).(string)
	integType = ctx.Interpolate(integType).(string)

	ctx.Log("Creating integration: %s (type: %s)", name, integType)

	// Build request body
	body := map[string]interface{}{
		"name": name,
		"type": integType,
	}

	// Add optional fields
	if config, ok := paramsMap["config"].(map[string]interface{}); ok {
		body["config"] = ctx.Interpolate(config)
	}

	// Make request
	resp, err := ctx.HTTPRequest("POST", "orchid", "/api/v1/integrations", nil, body)
	if err != nil {
		return fmt.Errorf("failed to create integration: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("create integration failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Save integration ID if requested
	if saveAs, ok := paramsMap["save_as"].(string); ok {
		if id, ok := result["id"]; ok {
			ctx.Set(saveAs, id)
			ctx.Log("Integration ID saved as: %s = %v", saveAs, id)
		}
	}

	return nil
}

// CreatePlan implements the create_plan step
func CreatePlan(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("create_plan params must be a map")
	}

	key, ok := paramsMap["key"].(string)
	if !ok {
		return fmt.Errorf("create_plan requires 'key'")
	}

	name, ok := paramsMap["name"].(string)
	if !ok {
		return fmt.Errorf("create_plan requires 'name'")
	}

	integrationID, ok := paramsMap["integration_id"].(string)
	if !ok {
		return fmt.Errorf("create_plan requires 'integration_id'")
	}

	planDefinition, ok := paramsMap["plan_definition"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("create_plan requires 'plan_definition'")
	}

	// Interpolate values
	key = ctx.Interpolate(key).(string)
	name = ctx.Interpolate(name).(string)
	integrationID = ctx.Interpolate(integrationID).(string)
	planDefinition = ctx.Interpolate(planDefinition).(map[string]interface{})

	ctx.Log("Creating plan: %s (key: %s)", name, key)

	// Build request body
	body := map[string]interface{}{
		"key":             key,
		"name":            name,
		"integration_id":  integrationID,
		"plan_definition": planDefinition,
	}

	// Add optional fields
	if description, ok := paramsMap["description"].(string); ok {
		body["description"] = ctx.Interpolate(description)
	}

	if enabled, ok := paramsMap["enabled"].(bool); ok {
		body["enabled"] = enabled
	}

	// Make request
	resp, err := ctx.HTTPRequest("POST", "orchid", "/api/v1/plans", nil, body)
	if err != nil {
		return fmt.Errorf("failed to create plan: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("create plan failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Save plan ID if requested
	if saveAs, ok := paramsMap["save_as"].(string); ok {
		if id, ok := result["id"]; ok {
			ctx.Set(saveAs, id)
			ctx.Log("Plan ID saved as: %s = %v", saveAs, id)
		}
	}

	return nil
}

// TriggerExecution implements the trigger_execution step
func TriggerExecution(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("trigger_execution params must be a map")
	}

	planKey, ok := paramsMap["plan_key"].(string)
	if !ok {
		return fmt.Errorf("trigger_execution requires 'plan_key'")
	}

	// Interpolate plan key
	planKey = ctx.Interpolate(planKey).(string)

	ctx.Log("Triggering execution for plan: %s", planKey)

	// Build request body
	body := map[string]interface{}{
		"plan_key": planKey,
	}

	// Add optional parameters
	if runParams, ok := paramsMap["parameters"].(map[string]interface{}); ok {
		body["parameters"] = ctx.Interpolate(runParams)
	}

	// Make request
	resp, err := ctx.HTTPRequest("POST", "orchid", "/api/v1/executions", nil, body)
	if err != nil {
		return fmt.Errorf("failed to trigger execution: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("trigger execution failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Save execution ID
	var executionID interface{}
	if id, ok := result["id"]; ok {
		executionID = id
	} else if id, ok := result["execution_id"]; ok {
		executionID = id
	}

	if saveAs, ok := paramsMap["save_as"].(string); ok {
		if executionID != nil {
			ctx.Set(saveAs, executionID)
			ctx.Log("Execution ID saved as: %s = %v", saveAs, executionID)
		}
	}

	// Wait for completion if requested
	waitForCompletion := false
	if wait, ok := paramsMap["wait_for_completion"].(bool); ok {
		waitForCompletion = wait
	}

	if waitForCompletion && executionID != nil {
		timeoutStr := "60s"
		if t, ok := paramsMap["timeout"].(string); ok {
			timeoutStr = t
		}

		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return fmt.Errorf("invalid timeout: %w", err)
		}

		ctx.Log("Waiting for execution to complete (timeout: %s)", timeout)

		if err := waitForExecutionCompletion(ctx, fmt.Sprintf("%v", executionID), timeout); err != nil {
			return fmt.Errorf("execution did not complete: %w", err)
		}

		ctx.Log("Execution completed successfully")
	}

	return nil
}

// waitForExecutionCompletion polls the execution status until complete or timeout
func waitForExecutionCompletion(ctx TestContext, executionID string, timeout time.Duration) error {
	startTime := time.Now()
	pollInterval := 2 * time.Second

	for {
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timeout waiting for execution to complete")
		}

		// Poll execution status
		resp, err := ctx.HTTPRequest("GET", "orchid", fmt.Sprintf("/api/v1/executions/%s", executionID), nil, nil)
		if err != nil {
			return fmt.Errorf("failed to get execution status: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode >= 300 {
			return fmt.Errorf("get execution failed with status %d: %s", resp.StatusCode, string(respBody))
		}

		var result map[string]interface{}
		if err := json.Unmarshal(respBody, &result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		// Check status
		status, ok := result["status"].(string)
		if !ok {
			return fmt.Errorf("no status in execution response")
		}

		switch status {
		case "completed", "success":
			// Save final status
			ctx.Set("execution_status", status)
			return nil
		case "failed", "error":
			ctx.Set("execution_status", status)
			return fmt.Errorf("execution failed with status: %s", status)
		case "running", "pending", "in_progress":
			// Continue polling
			ctx.Log("Execution status: %s", status)
			time.Sleep(pollInterval)
		default:
			return fmt.Errorf("unknown execution status: %s", status)
		}
	}
}
