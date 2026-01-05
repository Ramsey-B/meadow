// Package links defines the data flow connections in a mapping.
//
// # Overview
//
// Links connect sources (fields or steps) to targets (fields or steps),
// defining how data flows through a mapping. They are the "wires" that
// connect the components of a mapping together.
//
// # Link Types
//
// Links can connect different combinations of sources and targets:
//
// ## Field → Field (Direct Mapping)
//
// The simplest case - copies a value directly from source to target.
//
//	{Source: {FieldID: "name"}, Target: {FieldID: "full_name"}}
//
// ## Field → Step (Transformation Input)
//
// Feeds a field value into a transformation step.
//
//	{Source: {FieldID: "name"}, Target: {StepID: "uppercase"}}
//
// ## Constant → Field / Step (Literal Injection)
//
// Injects a literal value into the mapping. Common uses:
//
//   - emitting a fixed `_integration`
//
//   - emitting a fixed `_integration`
//
//   - wiring in booleans like `is_primary=false`
//
//     {Source: {Constant: "msgraph"}, Target: {FieldID: "integration"}}
//     {Source: {Constant: false}, Target: {FieldID: "is_primary"}}
//
// ## Step → Field (Transformation Output)
//
// Writes a step's output to a target field.
//
//	{Source: {StepID: "uppercase"}, Target: {FieldID: "upper_name"}}
//
// ## Step → Step (Chained Transformation)
//
// Pipes one step's output into another step's input.
//
//	{Source: {StepID: "trim"}, Target: {StepID: "uppercase"}}
//
// # Priority
//
// Links are executed in priority order (lower = earlier). This is important
// for multi-input steps where inputs must arrive in a specific order.
//
// Example: For number_add(a, b), you need priority to ensure 'a' arrives first:
//
//	{Priority: 0, Source: {FieldID: "value_a"}, Target: {StepID: "add"}}
//	{Priority: 1, Source: {FieldID: "value_b"}, Target: {StepID: "add"}}
//
// # Execution
//
// The mapping engine processes links in two phases:
//  1. Source links (links with field sources) - executed first
//  2. Child links (links with step sources) - executed after their source step
//
// When a step receives all its expected inputs, it executes and its child
// links are then processed.
package links

import (
	"fmt"
	"sort"

	"github.com/Gobusters/ectolinq"
)

// LinkDirection specifies a link endpoint.
//
// For Sources, exactly one of:
//   - FieldID (read from extracted source fields)
//   - StepID (read from a previous step output)
//   - Constant (inject a literal value)
//
// should be set.
//
// For Targets, exactly one of StepID or FieldID should be set.
type LinkDirection struct {
	StepID  string `json:"step_id" validate:"omitempty"`  // ID of a step (for step sources/targets)
	FieldID string `json:"field_id" validate:"omitempty"` // ID of a field (for field sources/targets)
	// Constant is a literal value injected into the mapping.
	// Example JSON:
	//
	//	{"source": {"constant": "msgraph"}, "target": {"field_id": "tgt_integration"}}
	//
	// Note: this is supported for Source directions only.
	Constant any `json:"constant" validate:"omitempty"`
}

// Link represents a connection from a source to a target.
// Data flows from Source to Target during mapping execution.
type Link struct {
	Priority int           `json:"priority" validate:"omitempty"` // Execution order (lower = earlier)
	Source   LinkDirection `json:"source" validate:"omitempty"`   // Where data comes from
	Target   LinkDirection `json:"target" validate:"omitempty"`   // Where data goes to
}

// Links is a collection of Link objects with utility methods.
type Links []Link

func (l Links) ToPriority() {
	sort.Slice(l, func(i, j int) bool {
		return l[i].Priority < l[j].Priority
	})
}

func (l Links) Filter(predicate func(Link) bool) Links {
	return ectolinq.Filter(l, predicate)
}

func (l *Link) GetLinkID() string {
	to := l.Target.FieldID
	if to == "" {
		to = l.Target.StepID
	}

	from := l.Source.FieldID
	if from == "" {
		from = l.Source.StepID
	}
	if from == "" && l.Source.Constant != nil {
		// Keep this short-ish: it's used in errors/logs.
		from = "const"
	}

	return fmt.Sprintf("%s -> %s", from, to)
}
