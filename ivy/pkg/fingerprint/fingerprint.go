package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// Generate creates a deterministic fingerprint for entity data
// The fingerprint is a SHA256 hash of the canonicalized JSON
func Generate(data map[string]any) string {
	return GenerateWithExclusions(data, nil)
}

// GenerateWithExclusions creates a fingerprint excluding specified fields.
// The excludeFields set contains dot-notation paths to exclude (e.g., "last_synced_at", "metadata.version").
// Top-level fields are matched directly; nested paths are matched hierarchically.
func GenerateWithExclusions(data map[string]any, excludeFields map[string]bool) string {
	// Create a canonical JSON representation, excluding specified fields
	canonical := canonicalizeWithExclusions(data, excludeFields, "")

	// Hash it
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// GenerateFromJSON creates a fingerprint from raw JSON
func GenerateFromJSON(data json.RawMessage) (string, error) {
	return GenerateFromJSONWithExclusions(data, nil)
}

// GenerateFromJSONWithExclusions creates a fingerprint from raw JSON, excluding specified fields.
func GenerateFromJSONWithExclusions(data json.RawMessage, excludeFields map[string]bool) (string, error) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return "", err
	}
	return GenerateWithExclusions(m, excludeFields), nil
}

// canonicalize creates a deterministic string representation of a map
// by sorting keys and recursively processing nested structures
func canonicalize(data any) string {
	return canonicalizeWithExclusions(data, nil, "")
}

// canonicalizeWithExclusions creates a deterministic string with field exclusions.
// currentPath tracks the dot-notation path for nested field matching.
func canonicalizeWithExclusions(data any, excludeFields map[string]bool, currentPath string) string {
	switch v := data.(type) {
	case map[string]any:
		return canonicalizeMapWithExclusions(v, excludeFields, currentPath)
	case []any:
		return canonicalizeArrayWithExclusions(v, excludeFields, currentPath)
	default:
		// For primitives, use JSON encoding
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func canonicalizeMapWithExclusions(m map[string]any, excludeFields map[string]bool, currentPath string) string {
	// Get sorted keys
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build canonical string
	result := "{"
	first := true
	for _, k := range keys {
		// Build the full path for this key
		fieldPath := k
		if currentPath != "" {
			fieldPath = currentPath + "." + k
		}

		// Check if this field should be excluded
		if shouldExcludeField(fieldPath, excludeFields) {
			continue
		}

		if !first {
			result += ","
		}
		first = false
		keyJSON, _ := json.Marshal(k)
		result += string(keyJSON) + ":" + canonicalizeWithExclusions(m[k], excludeFields, fieldPath)
	}
	result += "}"
	return result
}

func canonicalizeArrayWithExclusions(arr []any, excludeFields map[string]bool, currentPath string) string {
	result := "["
	for i, v := range arr {
		if i > 0 {
			result += ","
		}
		// For array elements, we use the same path (can't exclude individual indices)
		result += canonicalizeWithExclusions(v, excludeFields, currentPath)
	}
	result += "]"
	return result
}

// shouldExcludeField checks if a field path should be excluded.
// Supports exact matches and prefix matches for nested objects.
func shouldExcludeField(fieldPath string, excludeFields map[string]bool) bool {
	if excludeFields == nil {
		return false
	}

	// Exact match
	if excludeFields[fieldPath] {
		return true
	}

	// Check if any exclusion is a prefix (exclude parent object)
	for excluded := range excludeFields {
		if strings.HasPrefix(fieldPath, excluded+".") {
			return true
		}
	}

	return false
}

// HasChanged compares two fingerprints to detect changes
func HasChanged(oldFingerprint, newFingerprint string) bool {
	return oldFingerprint != newFingerprint
}

