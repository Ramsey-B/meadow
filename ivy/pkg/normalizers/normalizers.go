// Package normalizers provides field normalization functions for match indexing
package normalizers

import (
	"regexp"
	"strings"
	"unicode"
)

// Normalizer is a function that normalizes a string value
type Normalizer func(string) string

// registry holds all registered normalizers
var registry = make(map[string]Normalizer)

func init() {
	// Register built-in normalizers
	Register("lowercase", Lowercase)
	Register("uppercase", Uppercase)
	Register("trim", Trim)
	Register("nphone", NormalizePhone)
	Register("nemail", NormalizeEmail)
	Register("remove_whitespace", RemoveWhitespace)
	Register("remove_punctuation", RemovePunctuation)
	Register("nname", NormalizeName)
	Register("digits_only", DigitsOnly)
	Register("alphanumeric", Alphanumeric)
}

// Register adds a normalizer to the registry
func Register(name string, fn Normalizer) {
	registry[name] = fn
}

// Get retrieves a normalizer by name
func Get(name string) (Normalizer, bool) {
	fn, ok := registry[name]
	return fn, ok
}

// Apply applies a named normalizer to a value
func Apply(value, normalizer string) string {
	fn, ok := registry[normalizer]
	if !ok {
		return value
	}
	return fn(value)
}

// ApplyChain applies multiple normalizers in sequence
func ApplyChain(value string, normalizers ...string) string {
	result := value
	for _, name := range normalizers {
		result = Apply(result, name)
	}
	return result
}

// Built-in normalizers

// Lowercase converts string to lowercase
func Lowercase(s string) string {
	return strings.ToLower(s)
}

// Uppercase converts string to uppercase
func Uppercase(s string) string {
	return strings.ToUpper(s)
}

// Trim removes leading and trailing whitespace
func Trim(s string) string {
	return strings.TrimSpace(s)
}

// NormalizePhone removes all non-digit characters from a phone number
func NormalizePhone(s string) string {
	var result strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// NormalizeEmail normalizes an email address (lowercase, trim)
func NormalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// RemoveWhitespace removes all whitespace characters
func RemoveWhitespace(s string) string {
	var result strings.Builder
	for _, r := range s {
		if !unicode.IsSpace(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// RemovePunctuation removes all punctuation characters
func RemovePunctuation(s string) string {
	var result strings.Builder
	for _, r := range s {
		if !unicode.IsPunct(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// NormalizeName normalizes a person's name for matching
// - Lowercase
// - Remove extra whitespace
// - Remove common suffixes (Jr., Sr., III, etc.)
// - Remove punctuation
func NormalizeName(s string) string {
	// Lowercase
	s = strings.ToLower(s)

	// Remove common suffixes
	suffixes := []string{" jr.", " jr", " sr.", " sr", " iii", " ii", " iv", " phd", " md", " dds"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			s = s[:len(s)-len(suffix)]
		}
	}

	// Remove punctuation
	var result strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(r)
			prevSpace = false
		} else if unicode.IsSpace(r) {
			if !prevSpace {
				result.WriteRune(' ')
				prevSpace = true
			}
		}
	}

	return strings.TrimSpace(result.String())
}

// DigitsOnly keeps only digit characters
func DigitsOnly(s string) string {
	var result strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// Alphanumeric keeps only alphanumeric characters
func Alphanumeric(s string) string {
	var result strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// NormalizeSSN normalizes a social security number (US)
// Removes all non-digits and validates format
func NormalizeSSN(s string) string {
	digits := DigitsOnly(s)
	if len(digits) == 9 {
		return digits
	}
	return "" // Invalid SSN
}

// NormalizeZipCode normalizes a US zip code
func NormalizeZipCode(s string) string {
	digits := DigitsOnly(s)
	if len(digits) == 5 || len(digits) == 9 {
		return digits
	}
	return ""
}

// NormalizeAddress normalizes an address string
func NormalizeAddress(s string) string {
	s = strings.ToLower(s)

	// Common abbreviations
	replacements := map[string]string{
		" street":    " st",
		" avenue":    " ave",
		" boulevard": " blvd",
		" drive":     " dr",
		" road":      " rd",
		" lane":      " ln",
		" court":     " ct",
		" circle":    " cir",
		" place":     " pl",
		" apartment": " apt",
		" suite":     " ste",
		" north":     " n",
		" south":     " s",
		" east":      " e",
		" west":      " w",
	}

	for full, abbr := range replacements {
		s = strings.ReplaceAll(s, full, abbr)
	}

	// Remove extra whitespace
	spaceRe := regexp.MustCompile(`\s+`)
	s = spaceRe.ReplaceAllString(s, " ")

	return strings.TrimSpace(s)
}
