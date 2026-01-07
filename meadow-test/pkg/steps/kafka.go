package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

// PublishKafka implements the publish_kafka step
func PublishKafka(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("publish_kafka params must be a map")
	}

	topic, ok := paramsMap["topic"].(string)
	if !ok {
		return fmt.Errorf("publish_kafka requires 'topic'")
	}

	key := ""
	if k, ok := paramsMap["key"].(string); ok {
		key = ctx.Interpolate(k).(string)
	}

	value := paramsMap["value"]
	if value == nil {
		return fmt.Errorf("publish_kafka requires 'value'")
	}

	// Interpolate value
	value = ctx.Interpolate(value)

	// Convert value to JSON bytes
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	// Get headers
	headers := make([]kafka.Header, 0)
	if h, ok := paramsMap["headers"].(map[string]interface{}); ok {
		for hKey, hVal := range h {
			hValStr := fmt.Sprintf("%v", ctx.Interpolate(hVal))
			headers = append(headers, kafka.Header{
				Key:   hKey,
				Value: []byte(hValStr),
			})
		}
	}

	// Get Kafka brokers from context
	brokers, ok := ctx.Get("kafka_brokers")
	if !ok {
		// Try getting from built-in variable
		brokersStr := ctx.Interpolate("{{kafka_brokers}}").(string)
		brokers = strings.Split(brokersStr, ",")
	}

	brokersSlice, ok := brokers.([]string)
	if !ok {
		// Try converting
		if str, ok := brokers.(string); ok {
			brokersSlice = strings.Split(str, ",")
		} else {
			return fmt.Errorf("invalid kafka_brokers type: %T", brokers)
		}
	}

	ctx.Log("Publishing to Kafka topic: %s (key: %s)", topic, key)

	// Ensure topic exists by creating it if needed
	conn, err := kafka.Dial("tcp", brokersSlice[0])
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("failed to get Kafka controller: %w", err)
	}

	controllerConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka controller: %w", err)
	}
	defer controllerConn.Close()

	// Try to create topic (will fail silently if already exists)
	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
	}
	_ = controllerConn.CreateTopics(topicConfigs...)

	// Create writer with AllowAutoTopicCreation enabled
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  brokersSlice,
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	})
	defer writer.Close()

	// Publish message
	err = writer.WriteMessages(context.Background(), kafka.Message{
		Key:     []byte(key),
		Value:   valueBytes,
		Headers: headers,
	})

	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	ctx.Log("Message published successfully")
	return nil
}

// AssertKafkaMessage implements the assert_kafka_message step
func AssertKafkaMessage(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("assert_kafka_message params must be a map")
	}

	topic, ok := paramsMap["topic"].(string)
	if !ok {
		return fmt.Errorf("assert_kafka_message requires 'topic'")
	}

	timeoutStr := "30s"
	if t, ok := paramsMap["timeout"].(string); ok {
		timeoutStr = t
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	consumeFrom := "latest"
	if cf, ok := paramsMap["consume_from"].(string); ok {
		consumeFrom = cf
	}

	// Get Kafka brokers
	brokers, ok := ctx.Get("kafka_brokers")
	if !ok {
		brokersStr := ctx.Interpolate("{{kafka_brokers}}").(string)
		brokers = strings.Split(brokersStr, ",")
	}

	brokersSlice, ok := brokers.([]string)
	if !ok {
		if str, ok := brokers.(string); ok {
			brokersSlice = strings.Split(str, ",")
		} else {
			return fmt.Errorf("invalid kafka_brokers type: %T", brokers)
		}
	}

	ctx.Log("Consuming from Kafka topic: %s (timeout: %s)", topic, timeout)

	// Create reader
	readerConfig := kafka.ReaderConfig{
		Brokers:  brokersSlice,
		Topic:    topic,
		GroupID:  fmt.Sprintf("test-%d", time.Now().Unix()),
		MinBytes: 1,
		MaxBytes: 10e6,
	}

	// Set start offset
	switch consumeFrom {
	case "earliest":
		readerConfig.StartOffset = kafka.FirstOffset
	case "latest":
		readerConfig.StartOffset = kafka.LastOffset
	default:
		// For any other value, default to latest
		// Note: kafka-go doesn't support TimeOffset directly
		readerConfig.StartOffset = kafka.LastOffset
	}

	reader := kafka.NewReader(readerConfig)
	defer reader.Close()

	// Read messages with timeout
	timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	msg, err := reader.ReadMessage(timeoutCtx)
	if err != nil {
		return fmt.Errorf("failed to read message within %s: %w", timeout, err)
	}

	ctx.Log("Received message (offset: %d)", msg.Offset)

	// Parse message value
	var messageValue map[string]interface{}
	if err := json.Unmarshal(msg.Value, &messageValue); err != nil {
		ctx.Log("Warning: could not parse message as JSON: %v", err)
		messageValue = map[string]interface{}{
			"_raw": string(msg.Value),
		}
	}

	// Save message if requested
	if saveAs, ok := paramsMap["save_as"].(string); ok {
		ctx.Set(saveAs, messageValue)
	}

	// Run assertions
	if assertions, ok := paramsMap["assertions"].([]interface{}); ok {
		for i, assertionInterface := range assertions {
			assertion, ok := assertionInterface.(map[string]interface{})
			if !ok {
				return fmt.Errorf("assertion %d is not a map", i)
			}

			if err := runKafkaAssertion(ctx, msg, messageValue, assertion); err != nil {
				return fmt.Errorf("assertion %d failed: %w", i, err)
			}
		}
	}

	ctx.Log("All assertions passed")
	return nil
}

// runKafkaAssertion runs a single Kafka message assertion
func runKafkaAssertion(ctx TestContext, msg kafka.Message, messageValue map[string]interface{}, assertion map[string]interface{}) error {
	// Check header assertion
	if headerKey, ok := assertion["header"].(string); ok {
		var headerValue string
		for _, h := range msg.Headers {
			if h.Key == headerKey {
				headerValue = string(h.Value)
				break
			}
		}

		if equals, ok := assertion["equals"]; ok {
			expectedVal := ctx.Interpolate(equals)
			expectedStr := fmt.Sprintf("%v", expectedVal)
			if headerValue != expectedStr {
				return fmt.Errorf("header %s: expected %s, got %s", headerKey, expectedStr, headerValue)
			}
		}

		if contains, ok := assertion["contains"].(string); ok {
			expectedContains := ctx.Interpolate(contains).(string)
			if !strings.Contains(headerValue, expectedContains) {
				return fmt.Errorf("header %s: expected to contain %s, got %s", headerKey, expectedContains, headerValue)
			}
		}

		return nil
	}

	// Check field assertion
	if fieldPath, ok := assertion["field"].(string); ok {
		// Navigate to nested field (e.g., "data.user_id")
		parts := strings.Split(fieldPath, ".")
		var currentValue interface{} = messageValue

		for _, part := range parts {
			if m, ok := currentValue.(map[string]interface{}); ok {
				var found bool
				currentValue, found = m[part]
				if !found {
					return fmt.Errorf("field %s not found (missing: %s)", fieldPath, part)
				}
			} else {
				return fmt.Errorf("field %s: cannot navigate into %T", fieldPath, currentValue)
			}
		}

		// Check equals
		if equals, ok := assertion["equals"]; ok {
			expectedVal := ctx.Interpolate(equals)
			if !reflect.DeepEqual(currentValue, expectedVal) {
				return fmt.Errorf("field %s: expected %v, got %v", fieldPath, expectedVal, currentValue)
			}
		}

		// Check contains (for strings)
		if contains, ok := assertion["contains"].(string); ok {
			currentStr, ok := currentValue.(string)
			if !ok {
				return fmt.Errorf("field %s: contains only works on strings, got %T", fieldPath, currentValue)
			}

			expectedContains := ctx.Interpolate(contains).(string)
			if !strings.Contains(currentStr, expectedContains) {
				return fmt.Errorf("field %s: expected to contain %s, got %s", fieldPath, expectedContains, currentStr)
			}
		}

		// Check not_null
		if notNull, ok := assertion["not_null"].(bool); ok && notNull {
			if currentValue == nil {
				return fmt.Errorf("field %s: expected not null", fieldPath)
			}
		}

		return nil
	}

	return fmt.Errorf("assertion must have either 'header' or 'field'")
}

// MockAPI implements the mock_api step (configures the mock API service)
func MockAPI(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("mock_api params must be a map")
	}

	method, ok := paramsMap["method"].(string)
	if !ok {
		return fmt.Errorf("mock_api requires 'method'")
	}

	path, ok := paramsMap["path"].(string)
	if !ok {
		return fmt.Errorf("mock_api requires 'path'")
	}

	response, ok := paramsMap["response"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("mock_api requires 'response'")
	}

	// Build configuration request
	config := map[string]interface{}{
		"method":   strings.ToUpper(method),
		"path":     path,
		"response": ctx.Interpolate(response),
	}

	ctx.Log("Configuring mock API: %s %s", method, path)

	// Call mock API configuration endpoint
	// Note: This assumes the mock API has a configuration endpoint
	// You may need to implement this in the mocks service
	resp, err := ctx.HTTPRequest("POST", "mocks", "/api/test/configure", nil, config)
	if err != nil {
		return fmt.Errorf("failed to configure mock API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("mock API configuration failed with status: %d", resp.StatusCode)
	}

	ctx.Log("Mock API configured successfully")
	return nil
}
