package runner

import (
	"fmt"

	"github.com/Ramsey-B/meadow-test/pkg/steps"
)

// executeStep executes a single test step
func executeStep(testCtx *TestContext, step map[string]interface{}, stepLabel string) error {
	// Each step is a map with a single key (the step type)
	if len(step) == 0 {
		return fmt.Errorf("empty step")
	}

	if len(step) > 1 {
		return fmt.Errorf("step has multiple keys (expected one): %v", step)
	}

	// Get the step type and parameters
	var stepType string
	var params interface{}
	for k, v := range step {
		stepType = k
		params = v
	}

	if testCtx.Verbose {
		fmt.Printf("  [%s] %s\n", stepLabel, stepType)
	}

	// Execute the step based on type
	switch stepType {
	case "wait":
		return steps.Wait(testCtx, params)

	case "poll_until":
		return steps.PollUntil(testCtx, params)

	case "assert":
		return steps.Assert(testCtx, params)

	case "http_request":
		return steps.HTTPRequest(testCtx, params)

	case "mock_api":
		return steps.MockAPI(testCtx, params)

	case "publish_kafka":
		return steps.PublishKafka(testCtx, params)

	case "assert_kafka_message":
		return steps.AssertKafkaMessage(testCtx, params)

	case "count_kafka_messages":
		return steps.CountKafkaMessages(testCtx, params)

	case "get_kafka_offset":
		return steps.GetKafkaOffset(testCtx, params)

	case "assert_kafka_message_group":
		return steps.AssertKafkaMessageGroup(testCtx, params)

	case "create_integration":
		return steps.CreateIntegration(testCtx, params)

	case "create_plan":
		return steps.CreatePlan(testCtx, params)

	case "trigger_execution":
		return steps.TriggerExecution(testCtx, params)

	case "create_mapping":
		return steps.CreateMapping(testCtx, params)

	case "create_binding":
		return steps.CreateBinding(testCtx, params)

	case "create_entity_type":
		return steps.CreateEntityType(testCtx, params)

	case "create_match_rule":
		return steps.CreateMatchRule(testCtx, params)

	case "query_entities":
		return steps.QueryEntities(testCtx, params)

	case "use_template":
		return executeTemplate(testCtx, params)

	case "set_variable":
		return setVariable(testCtx, params)

	default:
		return fmt.Errorf("unknown step type: %s", stepType)
	}
}

// setVariable sets a variable to a value (with interpolation)
func setVariable(testCtx *TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("set_variable params must be a map")
	}

	name, ok := paramsMap["name"].(string)
	if !ok {
		return fmt.Errorf("set_variable requires 'name' parameter")
	}

	value, ok := paramsMap["value"]
	if !ok {
		return fmt.Errorf("set_variable requires 'value' parameter")
	}

	// Interpolate the value
	interpolatedValue := testCtx.Interpolate(value)

	testCtx.Set(name, interpolatedValue)

	if testCtx.Verbose {
		fmt.Printf("  [VAR] %s = %v\n", name, interpolatedValue)
	}

	return nil
}

// executeTemplate expands and executes a template
func executeTemplate(testCtx *TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("use_template params must be a map")
	}

	// Get template name (key without "with")
	var templateName string
	var templateParams map[string]interface{}

	// Handle both formats:
	// - use_template: template_name
	// - use_template:
	//     name: template_name  (or just direct key)
	//     with:
	//       param1: value1

	if name, ok := paramsMap["name"].(string); ok {
		templateName = name
		if with, ok := paramsMap["with"].(map[string]interface{}); ok {
			templateParams = with
		}
	} else {
		// Legacy format - iterate to find the template reference
		for k, v := range paramsMap {
			if k == "with" {
				if m, ok := v.(map[string]interface{}); ok {
					templateParams = m
				}
			} else {
				templateName = k
			}
		}
	}

	if templateName == "" {
		return fmt.Errorf("template name not specified")
	}

	// Get template
	tmpl, ok := testCtx.GetTemplate(templateName)
	if !ok {
		return fmt.Errorf("template not found: %s", templateName)
	}

	// Temporarily store template parameters as variables
	if templateParams != nil {
		for k, v := range templateParams {
			testCtx.Set(k, v)
		}
	}

	// Execute template steps
	templateSteps, ok := tmpl["steps"].([]interface{})
	if !ok {
		return fmt.Errorf("template %s has no steps", templateName)
	}

	for i, stepInterface := range templateSteps {
		stepMap, ok := stepInterface.(map[string]interface{})
		if !ok {
			return fmt.Errorf("template %s step %d is not a map", templateName, i)
		}

		// Interpolate the step map to replace template variables
		interpolatedStep := testCtx.Interpolate(stepMap).(map[string]interface{})

		if err := executeStep(testCtx, interpolatedStep, fmt.Sprintf("template:%s[%d]", templateName, i)); err != nil {
			return fmt.Errorf("template %s step %d failed: %w", templateName, i, err)
		}
	}

	return nil
}
