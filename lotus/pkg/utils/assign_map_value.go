package utils

import "strings"

func AssignMapValue(targetRaw map[string]any, path string, value any) map[string]any {
	if path == "" {
		return targetRaw
	}

	paths := strings.Split(path, ".")

	if len(paths) == 1 {
		targetRaw[paths[0]] = value
		return targetRaw
	}

	existingValue, ok := targetRaw[paths[0]].(map[string]any)
	if !ok {
		existingValue = make(map[string]any)
	}

	result := AssignMapValue(existingValue, strings.Join(paths[1:], "."), value)

	targetRaw[paths[0]] = result

	return targetRaw
}
