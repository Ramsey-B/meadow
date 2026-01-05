package matching

import (
	"math"
	"strings"
	"time"
	"unicode"
)

// Scorer provides various string and value comparison algorithms
type Scorer struct{}

// NewScorer creates a new Scorer
func NewScorer() *Scorer {
	return &Scorer{}
}

// ExactMatch returns 1.0 for exact match, 0.0 otherwise
func (s *Scorer) ExactMatch(a, b string, caseSensitive bool) float64 {
	if !caseSensitive {
		a = strings.ToLower(a)
		b = strings.ToLower(b)
	}
	if a == b {
		return 1.0
	}
	return 0.0
}

// JaroWinkler calculates the Jaro-Winkler similarity between two strings
// Returns a value between 0.0 (no similarity) and 1.0 (exact match)
func (s *Scorer) JaroWinkler(a, b string) float64 {
	if a == b {
		return 1.0
	}

	jaro := s.Jaro(a, b)
	
	// Winkler modification: boost for common prefix
	prefixLen := 0
	maxPrefix := 4
	for i := 0; i < len(a) && i < len(b) && i < maxPrefix; i++ {
		if a[i] == b[i] {
			prefixLen++
		} else {
			break
		}
	}

	// Winkler scaling factor is typically 0.1
	scalingFactor := 0.1
	return jaro + float64(prefixLen)*scalingFactor*(1.0-jaro)
}

// Jaro calculates the Jaro similarity between two strings
func (s *Scorer) Jaro(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	// Maximum distance for character matching
	matchDist := max(len(a), len(b))/2 - 1
	if matchDist < 0 {
		matchDist = 0
	}

	aMatches := make([]bool, len(a))
	bMatches := make([]bool, len(b))

	matches := 0
	transpositions := 0

	// Find matches
	for i := 0; i < len(a); i++ {
		start := max(0, i-matchDist)
		end := min(len(b), i+matchDist+1)

		for j := start; j < end; j++ {
			if bMatches[j] || a[i] != b[j] {
				continue
			}
			aMatches[i] = true
			bMatches[j] = true
			matches++
			break
		}
	}

	if matches == 0 {
		return 0.0
	}

	// Count transpositions
	k := 0
	for i := 0; i < len(a); i++ {
		if !aMatches[i] {
			continue
		}
		for !bMatches[k] {
			k++
		}
		if a[i] != b[k] {
			transpositions++
		}
		k++
	}

	m := float64(matches)
	t := float64(transpositions) / 2

	return (m/float64(len(a)) + m/float64(len(b)) + (m-t)/m) / 3
}

// Levenshtein calculates the Levenshtein distance between two strings
// Returns a similarity score between 0.0 and 1.0
func (s *Scorer) Levenshtein(a, b string) float64 {
	distance := s.LevenshteinDistance(a, b)
	maxLen := max(len(a), len(b))
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - float64(distance)/float64(maxLen)
}

// LevenshteinDistance calculates the edit distance between two strings
func (s *Scorer) LevenshteinDistance(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Create two rows for dynamic programming
	row := make([]int, len(b)+1)
	prevRow := make([]int, len(b)+1)

	for j := 0; j <= len(b); j++ {
		prevRow[j] = j
	}

	for i := 1; i <= len(a); i++ {
		row[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			row[j] = min(min(row[j-1]+1, prevRow[j]+1), prevRow[j-1]+cost)
		}
		row, prevRow = prevRow, row
	}

	return prevRow[len(b)]
}

// Soundex calculates the Soundex encoding of a string
func (s *Scorer) Soundex(str string) string {
	if len(str) == 0 {
		return ""
	}

	// Convert to uppercase
	str = strings.ToUpper(str)

	// Keep the first letter
	result := string(str[0])
	prevCode := soundexCode(rune(str[0]))

	// Process remaining characters
	for i := 1; i < len(str) && len(result) < 4; i++ {
		char := rune(str[i])
		if !unicode.IsLetter(char) {
			continue
		}

		code := soundexCode(char)
		if code != "0" && code != prevCode {
			result += code
		}
		prevCode = code
	}

	// Pad with zeros
	for len(result) < 4 {
		result += "0"
	}

	return result
}

// SoundexMatch returns 1.0 if Soundex codes match, 0.0 otherwise
func (s *Scorer) SoundexMatch(a, b string) float64 {
	if s.Soundex(a) == s.Soundex(b) {
		return 1.0
	}
	return 0.0
}

// soundexCode returns the Soundex code for a character
func soundexCode(char rune) string {
	switch char {
	case 'B', 'F', 'P', 'V':
		return "1"
	case 'C', 'G', 'J', 'K', 'Q', 'S', 'X', 'Z':
		return "2"
	case 'D', 'T':
		return "3"
	case 'L':
		return "4"
	case 'M', 'N':
		return "5"
	case 'R':
		return "6"
	default:
		return "0"
	}
}

// Metaphone calculates a simplified Metaphone encoding
func (s *Scorer) Metaphone(str string) string {
	if len(str) == 0 {
		return ""
	}

	// Convert to uppercase
	str = strings.ToUpper(str)
	
	// Remove non-alphabetic characters
	var result strings.Builder
	for _, char := range str {
		if unicode.IsLetter(char) {
			result.WriteRune(char)
		}
	}
	str = result.String()

	if len(str) == 0 {
		return ""
	}

	// Simplified Metaphone - just using first few consonants
	var metaphone strings.Builder
	prevCode := byte(0)
	
	for i := 0; i < len(str) && metaphone.Len() < 6; i++ {
		char := str[i]
		code := metaphoneCode(char, i, str)
		
		if code != 0 && code != prevCode {
			metaphone.WriteByte(code)
			prevCode = code
		}
	}

	return metaphone.String()
}

// metaphoneCode returns the Metaphone code for a character
func metaphoneCode(char byte, pos int, word string) byte {
	switch char {
	case 'A', 'E', 'I', 'O', 'U':
		if pos == 0 {
			return char
		}
		return 0
	case 'B':
		return 'B'
	case 'C':
		if pos+1 < len(word) && (word[pos+1] == 'I' || word[pos+1] == 'E' || word[pos+1] == 'Y') {
			return 'S'
		}
		return 'K'
	case 'D':
		return 'T'
	case 'F':
		return 'F'
	case 'G':
		return 'J'
	case 'H':
		return 0 // Usually silent
	case 'J':
		return 'J'
	case 'K':
		return 'K'
	case 'L':
		return 'L'
	case 'M':
		return 'M'
	case 'N':
		return 'N'
	case 'P':
		if pos+1 < len(word) && word[pos+1] == 'H' {
			return 'F'
		}
		return 'P'
	case 'Q':
		return 'K'
	case 'R':
		return 'R'
	case 'S':
		return 'S'
	case 'T':
		return 'T'
	case 'V':
		return 'F'
	case 'W':
		return 0
	case 'X':
		return 'S'
	case 'Y':
		return 0
	case 'Z':
		return 'S'
	default:
		return 0
	}
}

// MetaphoneMatch returns 1.0 if Metaphone codes match, 0.0 otherwise
func (s *Scorer) MetaphoneMatch(a, b string) float64 {
	if s.Metaphone(a) == s.Metaphone(b) {
		return 1.0
	}
	return 0.0
}

// DateProximity calculates a proximity score for two dates
// Returns 1.0 for exact match, decreasing to 0.0 beyond maxDaysDiff
func (s *Scorer) DateProximity(a, b time.Time, maxDaysDiff int) float64 {
	if a.IsZero() || b.IsZero() {
		return 0.0
	}

	daysDiff := math.Abs(a.Sub(b).Hours() / 24)
	
	if daysDiff == 0 {
		return 1.0
	}
	if int(daysDiff) >= maxDaysDiff {
		return 0.0
	}

	// Linear decay
	return 1.0 - (daysDiff / float64(maxDaysDiff))
}

// NumericProximity calculates a proximity score for two numbers
// Returns 1.0 for exact match, decreasing based on relative difference
func (s *Scorer) NumericProximity(a, b, maxDiff float64) float64 {
	if a == b {
		return 1.0
	}

	diff := math.Abs(a - b)
	if diff >= maxDiff {
		return 0.0
	}

	return 1.0 - (diff / maxDiff)
}

// WeightedScore calculates a weighted average of scores
func (s *Scorer) WeightedScore(scores map[string]float64, weights map[string]float64) float64 {
	if len(scores) == 0 {
		return 0.0
	}

	var totalWeight float64
	var weightedSum float64

	for field, score := range scores {
		weight := 1.0 // Default weight
		if w, ok := weights[field]; ok {
			weight = w
		}
		weightedSum += score * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 0.0
	}

	return weightedSum / totalWeight
}

