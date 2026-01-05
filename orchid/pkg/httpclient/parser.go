package httpclient

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
)

// ParseResponse parses the response body based on content type
func ParseResponse(resp *Response) error {
	if len(resp.Body) == 0 {
		return nil
	}

	contentType := strings.ToLower(resp.ContentType)

	switch {
	case strings.Contains(contentType, "application/json"):
		return parseJSON(resp)
	case strings.Contains(contentType, "text/json"):
		return parseJSON(resp)
	case strings.Contains(contentType, "application/xml"):
		return parseXML(resp)
	case strings.Contains(contentType, "text/xml"):
		return parseXML(resp)
	case strings.Contains(contentType, "text/"):
		// Text responses - store as string
		resp.BodyJSON = string(resp.Body)
		return nil
	default:
		// Binary or unknown - base64 encode
		resp.BodyJSON = map[string]any{
			"_binary":       true,
			"_content_type": resp.ContentType,
			"_base64":       base64.StdEncoding.EncodeToString(resp.Body),
			"_size":         len(resp.Body),
		}
		return nil
	}
}

// parseJSON parses JSON response body
func parseJSON(resp *Response) error {
	var result any
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	resp.BodyJSON = result
	return nil
}

// parseXML parses XML response and converts to JSON-like structure
func parseXML(resp *Response) error {
	result, err := xmlToMap(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to parse XML: %w", err)
	}
	resp.BodyJSON = result
	return nil
}

// XMLNode represents a generic XML node for unmarshaling
type XMLNode struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Content  string     `xml:",chardata"`
	Children []XMLNode  `xml:",any"`
}

// xmlToMap converts XML to a map structure
func xmlToMap(data []byte) (map[string]any, error) {
	var node XMLNode
	if err := xml.Unmarshal(data, &node); err != nil {
		return nil, err
	}

	return nodeToMap(node), nil
}

// nodeToMap converts an XMLNode to a map
func nodeToMap(node XMLNode) map[string]any {
	result := make(map[string]any)

	// Add attributes with @ prefix
	for _, attr := range node.Attrs {
		result["@"+attr.Name.Local] = attr.Value
	}

	// If no children, just return text content
	if len(node.Children) == 0 {
		content := strings.TrimSpace(node.Content)
		if content != "" {
			if len(result) == 0 {
				// No attributes, just return the text
				return map[string]any{node.XMLName.Local: content}
			}
			result["#text"] = content
		}
		return map[string]any{node.XMLName.Local: result}
	}

	// Group children by name
	childGroups := make(map[string][]any)
	for _, child := range node.Children {
		childMap := nodeToMap(child)
		for k, v := range childMap {
			childGroups[k] = append(childGroups[k], v)
		}
	}

	// Add children - if single occurrence, don't wrap in array
	for name, values := range childGroups {
		if len(values) == 1 {
			result[name] = values[0]
		} else {
			result[name] = values
		}
	}

	return map[string]any{node.XMLName.Local: result}
}

// IsSuccessStatus returns true if the status code indicates success
func IsSuccessStatus(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

// IsRetryableStatus returns true if the status code indicates a retryable error
func IsRetryableStatus(statusCode int) bool {
	switch statusCode {
	case 408, 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}

// IsRateLimitStatus returns true if the status code indicates rate limiting
func IsRateLimitStatus(statusCode int) bool {
	return statusCode == 429
}
