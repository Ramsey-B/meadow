package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

// GetKafkaOffset gets the current latest offset for a topic (use before triggering to avoid reading old messages)
func GetKafkaOffset(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("get_kafka_offset params must be a map")
	}

	topic, ok := paramsMap["topic"].(string)
	if !ok {
		return fmt.Errorf("get_kafka_offset requires 'topic'")
	}

	saveAs, ok := paramsMap["save_as"].(string)
	if !ok {
		return fmt.Errorf("get_kafka_offset requires 'save_as'")
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

	// Get the latest offset for partition 0
	conn, err := kafka.DialLeader(context.Background(), "tcp", brokersSlice[0], topic, 0)
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	defer conn.Close()

	offset, err := conn.ReadLastOffset()
	if err != nil {
		return fmt.Errorf("failed to get offset: %w", err)
	}

	ctx.Log("Got current offset for topic %s: %d", topic, offset)
	ctx.Set(saveAs, offset)
	return nil
}

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

	// Check for from_offset - allows starting from a specific offset (e.g., saved before triggering)
	var fromOffset int64 = -1
	if fo, ok := paramsMap["from_offset"]; ok {
		// Interpolate to get the saved offset value
		interpolated := ctx.Interpolate(fo)
		switch v := interpolated.(type) {
		case int64:
			fromOffset = v
		case int:
			fromOffset = int64(v)
		case float64:
			fromOffset = int64(v)
		case string:
			// Try to parse the string as a number (interpolation returns strings)
			if parsed, err := parseInt64(v); err == nil {
				fromOffset = parsed
			} else {
				ctx.Log("Warning: from_offset '%s' could not be parsed as number: %v", v, err)
			}
		}
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

	// If from_offset is specified, use direct partition reader instead of consumer group
	if fromOffset >= 0 {
		ctx.Log("Consuming from Kafka topic: %s from offset %d (timeout: %s)", topic, fromOffset, timeout)
		return assertKafkaMessageFromOffset(ctx, paramsMap, brokersSlice, topic, fromOffset, timeout)
	}

	ctx.Log("Consuming from Kafka topic: %s (timeout: %s)", topic, timeout)

	// Create reader
	readerConfig := kafka.ReaderConfig{
		Brokers:  brokersSlice,
		Topic:    topic,
		GroupID:  fmt.Sprintf("test-%d", time.Now().UnixNano()),
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

	// Parse filter criteria if specified
	var filterHeader, filterEquals string
	var filterField string
	var filterHasField string
	if filter, ok := paramsMap["filter"].(map[string]interface{}); ok {
		if h, ok := filter["header"].(string); ok {
			filterHeader = h
		}
		if f, ok := filter["field"].(string); ok {
			filterField = f
		}
		if eq, ok := filter["equals"]; ok {
			filterEquals = ctx.Interpolate(eq).(string)
		}
		if hf, ok := filter["has_field"].(string); ok {
			filterHasField = hf
		}
	}

	// Read messages with timeout, filtering for matching message
	timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var msg kafka.Message
	var messageValue map[string]interface{}
	messagesRead := 0

	for {
		var err error
		msg, err = reader.ReadMessage(timeoutCtx)
		if err != nil {
			if messagesRead == 0 {
				return fmt.Errorf("failed to read any message within %s: %w", timeout, err)
			}
			return fmt.Errorf("no matching message found after reading %d messages: %w", messagesRead, err)
		}
		messagesRead++

		// Parse message value
		if err := json.Unmarshal(msg.Value, &messageValue); err != nil {
			ctx.Log("Warning: could not parse message %d as JSON: %v", msg.Offset, err)
			messageValue = map[string]interface{}{
				"_raw": string(msg.Value),
			}
		}

		// Check if message matches filter
		if filterHeader != "" && filterEquals != "" {
			var headerValue string
			for _, h := range msg.Headers {
				if h.Key == filterHeader {
					headerValue = string(h.Value)
					break
				}
			}
			if headerValue != filterEquals {
				ctx.Log("Skipping message %d: header %s=%s (want %s)", msg.Offset, filterHeader, headerValue, filterEquals)
				continue
			}
		}

		if filterField != "" && filterEquals != "" {
			fieldValue, _ := getNestedField(messageValue, filterField)
			if fmt.Sprintf("%v", fieldValue) != filterEquals {
				ctx.Log("Skipping message %d: field %s=%v (want %s)", msg.Offset, filterField, fieldValue, filterEquals)
				continue
			}
		}

		// Check has_field filter (message must have this field)
		if filterHasField != "" {
			if _, ok := messageValue[filterHasField]; !ok {
				ctx.Log("Skipping message %d: missing required field %s", msg.Offset, filterHasField)
				continue
			}
		}

		// Message matches filter (or no filter specified)
		break
	}

	ctx.Log("Received matching message (offset: %d, after reading %d messages)", msg.Offset, messagesRead)

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

// assertKafkaMessageFromOffset reads messages starting from a specific offset (more efficient for tests)
func assertKafkaMessageFromOffset(ctx TestContext, paramsMap map[string]interface{}, brokers []string, topic string, startOffset int64, timeout time.Duration) error {
	// Parse filter criteria
	var filterHeader, filterEquals string
	var filterField string
	var filterHasField string
	if filter, ok := paramsMap["filter"].(map[string]interface{}); ok {
		if h, ok := filter["header"].(string); ok {
			filterHeader = h
		}
		if f, ok := filter["field"].(string); ok {
			filterField = f
		}
		if eq, ok := filter["equals"]; ok {
			filterEquals = ctx.Interpolate(eq).(string)
		}
		if hf, ok := filter["has_field"].(string); ok {
			filterHasField = hf
		}
	}

	// Use kafka.Reader with explicit partition and offset
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   brokers,
		Topic:     topic,
		Partition: 0,
		MinBytes:  1,
		MaxBytes:  10e6,
	})
	defer reader.Close()

	// Set the offset to start reading from
	reader.SetOffset(startOffset)

	// Read messages with timeout
	timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var msg kafka.Message
	var messageValue map[string]interface{}
	messagesRead := 0

	for {
		m, err := reader.ReadMessage(timeoutCtx)
		if err != nil {
			if messagesRead == 0 {
				return fmt.Errorf("no messages found starting from offset %d within timeout: %w", startOffset, err)
			}
			return fmt.Errorf("no matching message found after reading %d messages from offset %d: %w", messagesRead, startOffset, err)
		}
		messagesRead++

		// Parse message
		if parseErr := json.Unmarshal(m.Value, &messageValue); parseErr != nil {
			ctx.Log("Skipping message %d: failed to parse JSON", m.Offset)
			continue
		}

		// Check filter
		if filterHeader != "" && filterEquals != "" {
			var headerValue string
			for _, h := range m.Headers {
				if h.Key == filterHeader {
					headerValue = string(h.Value)
					break
				}
			}
			if headerValue != filterEquals {
				ctx.Log("Skipping message %d: header %s=%s (want %s)", m.Offset, filterHeader, headerValue, filterEquals)
				continue
			}
		}

		if filterField != "" && filterEquals != "" {
			fieldValue, _ := getNestedField(messageValue, filterField)
			if fmt.Sprintf("%v", fieldValue) != filterEquals {
				ctx.Log("Skipping message %d: field %s=%v (want %s)", m.Offset, filterField, fieldValue, filterEquals)
				continue
			}
		}

		// Check has_field filter (message must have this field)
		if filterHasField != "" {
			if _, ok := messageValue[filterHasField]; !ok {
				ctx.Log("Skipping message %d: missing required field %s", m.Offset, filterHasField)
				continue
			}
		}

		// Found matching message!
		msg = m
		ctx.Log("Found matching message at offset %d (read %d messages from offset %d)", m.Offset, messagesRead, startOffset)
		break
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
		// Navigate to nested field (e.g., "data.user_id" or "response_body[0].profile.email")
		currentValue, err := navigateFieldPath(messageValue, fieldPath)
		if err != nil {
			return fmt.Errorf("field %s: %w", fieldPath, err)
		}

		// Check equals
		if equals, ok := assertion["equals"]; ok {
			expectedVal := ctx.Interpolate(equals)
			if !compareValues(currentValue, expectedVal) {
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

		// Check not_empty
		if notEmpty, ok := assertion["not_empty"].(bool); ok && notEmpty {
			if currentValue == nil {
				return fmt.Errorf("field %s: expected not empty but was nil", fieldPath)
			}
			// Check for empty string
			if str, ok := currentValue.(string); ok && str == "" {
				return fmt.Errorf("field %s: expected not empty but was empty string", fieldPath)
			}
			// Check for empty array
			if arr, ok := currentValue.([]interface{}); ok && len(arr) == 0 {
				return fmt.Errorf("field %s: expected not empty but was empty array", fieldPath)
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

// navigateFieldPath navigates to a nested field in a value using dot notation and array indexing.
// Examples: "data.user_id", "response_body[0].profile.email", "items[2].name"
func navigateFieldPath(value interface{}, path string) (interface{}, error) {
	// Parse the path into segments, handling both dots and array indices
	// e.g., "response_body[0].profile.email" -> ["response_body", "[0]", "profile", "email"]
	segments := parseFieldPath(path)

	current := value
	for _, segment := range segments {
		if segment == "" {
			continue
		}

		// Check if this is an array index segment like "[0]"
		if strings.HasPrefix(segment, "[") && strings.HasSuffix(segment, "]") {
			indexStr := segment[1 : len(segment)-1]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return nil, fmt.Errorf("invalid array index: %s", segment)
			}

			// Navigate into array
			arr, ok := current.([]interface{})
			if !ok {
				return nil, fmt.Errorf("expected array at %s, got %T", segment, current)
			}
			if index < 0 || index >= len(arr) {
				return nil, fmt.Errorf("array index %d out of bounds (length: %d)", index, len(arr))
			}
			current = arr[index]
		} else {
			// Navigate into map
			m, ok := current.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("expected object at %s, got %T", segment, current)
			}
			var found bool
			current, found = m[segment]
			if !found {
				return nil, fmt.Errorf("field '%s' not found", segment)
			}
		}
	}

	return current, nil
}

// parseFieldPath parses a field path into segments.
// "response_body[0].profile.email" -> ["response_body", "[0]", "profile", "email"]
func parseFieldPath(path string) []string {
	var segments []string
	var current strings.Builder

	for i := 0; i < len(path); i++ {
		ch := path[i]
		switch ch {
		case '.':
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
		case '[':
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
			// Find the closing bracket
			j := i + 1
			for j < len(path) && path[j] != ']' {
				j++
			}
			if j < len(path) {
				segments = append(segments, path[i:j+1]) // Include brackets
				i = j                                    // Skip past the ]
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		segments = append(segments, current.String())
	}

	return segments
}

// compareValues compares two values, handling numeric type differences
// (YAML parses numbers as int, JSON as float64)
func compareValues(actual, expected interface{}) bool {
	// Try numeric comparison first
	actualNum, actualIsNum := toFloat64(actual)
	expectedNum, expectedIsNum := toFloat64(expected)

	if actualIsNum && expectedIsNum {
		return actualNum == expectedNum
	}

	// Fall back to string comparison for mixed types
	if actualIsNum != expectedIsNum {
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	}

	// Use reflect.DeepEqual for non-numeric types
	return reflect.DeepEqual(actual, expected)
}

// toFloat64 converts numeric types to float64 for comparison
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	default:
		return 0, false
	}
}

// parseInt64 parses a string to int64
func parseInt64(s string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

// CountKafkaMessages counts messages matching a filter and validates minimum count
func CountKafkaMessages(ctx TestContext, params interface{}) error {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("count_kafka_messages params must be a map")
	}

	topic, ok := paramsMap["topic"].(string)
	if !ok {
		return fmt.Errorf("count_kafka_messages requires 'topic'")
	}

	// Get minimum expected count
	minCount := 1
	if mc, ok := paramsMap["min_count"].(int); ok {
		minCount = mc
	} else if mc, ok := paramsMap["min_count"].(float64); ok {
		minCount = int(mc)
	}

	timeoutStr := "30s"
	if t, ok := paramsMap["timeout"].(string); ok {
		timeoutStr = t
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	// Check for from_offset
	var fromOffset int64 = -1
	if fo, ok := paramsMap["from_offset"]; ok {
		interpolated := ctx.Interpolate(fo)
		switch v := interpolated.(type) {
		case int64:
			fromOffset = v
		case int:
			fromOffset = int64(v)
		case float64:
			fromOffset = int64(v)
		case string:
			if parsed, err := parseInt64(v); err == nil {
				fromOffset = parsed
			}
		}
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

	// Create reader - use explicit partition if from_offset specified
	var reader *kafka.Reader
	if fromOffset >= 0 {
		ctx.Log("Counting messages from Kafka topic: %s from offset %d (min: %d, timeout: %s)", topic, fromOffset, minCount, timeout)
		reader = kafka.NewReader(kafka.ReaderConfig{
			Brokers:   brokersSlice,
			Topic:     topic,
			Partition: 0,
			MinBytes:  1,
			MaxBytes:  10e6,
		})
		reader.SetOffset(fromOffset)
	} else {
		ctx.Log("Counting messages from Kafka topic: %s (min: %d, timeout: %s)", topic, minCount, timeout)
		reader = kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokersSlice,
			Topic:       topic,
			GroupID:     fmt.Sprintf("count-%d", time.Now().UnixNano()),
			MinBytes:    1,
			MaxBytes:    10e6,
			StartOffset: kafka.FirstOffset,
		})
	}
	defer reader.Close()

	// Parse filter criteria
	var filterHeader, filterEquals string
	var filterHasField string
	if filter, ok := paramsMap["filter"].(map[string]interface{}); ok {
		if h, ok := filter["header"].(string); ok {
			filterHeader = h
		}
		if eq, ok := filter["equals"]; ok {
			filterEquals = ctx.Interpolate(eq).(string)
		}
		if hf, ok := filter["has_field"].(string); ok {
			filterHasField = hf
		}
	}

	// Read messages and count matching ones
	timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	matchingMessages := make([]map[string]interface{}, 0)
	messagesRead := 0

	for {
		msg, err := reader.ReadMessage(timeoutCtx)
		if err != nil {
			// Context deadline exceeded or other error
			break
		}
		messagesRead++

		// Parse message
		var messageValue map[string]interface{}
		if parseErr := json.Unmarshal(msg.Value, &messageValue); parseErr != nil {
			continue
		}

		// Check if message matches filter
		if filterHeader != "" && filterEquals != "" {
			var headerValue string
			for _, h := range msg.Headers {
				if h.Key == filterHeader {
					headerValue = string(h.Value)
					break
				}
			}
			if headerValue != filterEquals {
				continue
			}
		}

		// Check has_field filter
		if filterHasField != "" {
			if _, ok := messageValue[filterHasField]; !ok {
				continue
			}
		}

		// Message matches!
		matchingMessages = append(matchingMessages, messageValue)
		ctx.Log("Found matching message %d (offset: %d)", len(matchingMessages), msg.Offset)

		// If we have enough messages, we can stop early
		if len(matchingMessages) >= minCount {
			// But let's try to read a few more in case there are more
			// Give it 1 more second to find additional messages
			shortCtx, shortCancel := context.WithTimeout(context.Background(), 1*time.Second)
			for {
				msg, err := reader.ReadMessage(shortCtx)
				if err != nil {
					break
				}
				var mv map[string]interface{}
				if parseErr := json.Unmarshal(msg.Value, &mv); parseErr != nil {
					continue
				}
				if filterHeader != "" && filterEquals != "" {
					var hv string
					for _, h := range msg.Headers {
						if h.Key == filterHeader {
							hv = string(h.Value)
							break
						}
					}
					if hv != filterEquals {
						continue
					}
				}
				if filterHasField != "" {
					if _, ok := mv[filterHasField]; !ok {
						continue
					}
				}
				matchingMessages = append(matchingMessages, mv)
				ctx.Log("Found additional matching message %d (offset: %d)", len(matchingMessages), msg.Offset)
			}
			shortCancel()
			break
		}
	}

	ctx.Log("Read %d messages, found %d matching (need %d)", messagesRead, len(matchingMessages), minCount)

	if len(matchingMessages) < minCount {
		return fmt.Errorf("expected at least %d matching messages, found %d", minCount, len(matchingMessages))
	}

	// Save count and messages if requested
	if saveAs, ok := paramsMap["save_as"].(string); ok {
		ctx.Set(saveAs, map[string]interface{}{
			"count":    len(matchingMessages),
			"messages": matchingMessages,
		})
	}

	ctx.Log("SUCCESS: Found %d matching messages (min required: %d)", len(matchingMessages), minCount)
	return nil
}
