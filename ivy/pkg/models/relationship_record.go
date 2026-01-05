package models

// RelationshipRecord is the expected shape for a relationship message produced by Lotus mappings.
// Supports two modes:
// 1. Direct: from source_id to a specific source_id (ToSourceID is set)
// 2. Criteria: from source_id to entities matching criteria (ToCriteria is set)
type RelationshipRecord struct {
	RelationshipType string `json:"_relationship_type"`

	// From side (always by source_id)
	FromEntityType  string `json:"_from_entity_type"`
	FromSourceID    string `json:"_from_source_id"`
	FromIntegration string `json:"_from_integration,omitempty"` // defaults to message integration

	// To side - EITHER source_id OR criteria (mutually exclusive)
	ToEntityType  string `json:"_to_entity_type"`
	ToSourceID    string `json:"_to_source_id,omitempty"`   // For direct relationships
	ToIntegration string `json:"_to_integration,omitempty"` // Required for criteria, optional for direct

	// Criteria-based targeting (if present, ToSourceID is ignored)
	// Format: {"field": "value"} for equality, {"field": {"$contains": "value"}} for operators
	// Supported operators: $contains (array contains), $in (value in list), $gte, $gt, $lte, $lt
	ToCriteria map[string]any `json:"_to_criteria,omitempty"`
}

// IsCriteriaBased returns true if this is a criteria-based relationship
func (r *RelationshipRecord) IsCriteriaBased() bool {
	return len(r.ToCriteria) > 0
}

// Normalize fills in default values from the message context
func (r *RelationshipRecord) Normalize(messageIntegration string) {
	if r.FromIntegration == "" {
		r.FromIntegration = messageIntegration
	}
	if r.ToIntegration == "" {
		r.ToIntegration = messageIntegration
	}
}

// IsValid returns true if the relationship record has all required fields
func (r *RelationshipRecord) IsValid() bool {
	if r.RelationshipType == "" || r.FromEntityType == "" || r.FromSourceID == "" || r.ToEntityType == "" {
		return false
	}

	// Must have either ToSourceID (direct) or ToCriteria (criteria-based)
	if r.ToSourceID == "" && len(r.ToCriteria) == 0 {
		return false
	}

	return true
}
