// Package mapping provides the core data transformation engine for Lotus.
//
// # Overview
//
// The mapping package transforms source data into target data based on defined
// field mappings and transformation steps. It supports:
//   - Direct field-to-field mappings
//   - Chained transformation steps (e.g., trim -> uppercase -> concat)
//   - Multi-input steps (e.g., number_add that takes multiple numbers)
//   - Conditional logic and validation
//   - High-performance pooled execution
//
// # Key Concepts
//
// ## MappingDefinition
//
// A MappingDefinition is the blueprint for a mapping. It contains:
//   - SourceFields: Fields to extract from input data
//   - TargetFields: Fields to produce in output data
//   - StepDefinitions: Transformation steps (optional)
//   - Links: Connections between sources, steps, and targets
//
// ## Links
//
// Links define the data flow. Each link has:
//   - Source: Where data comes from (a field or a step's output)
//   - Target: Where data goes to (a field or a step's input)
//   - Priority: Execution order (lower = earlier)
//
// Link types:
//   - Field → Field: Direct mapping (no transformation)
//   - Field → Step: Input to a transformation
//   - Step → Step: Chained transformations
//   - Step → Field: Output of a transformation
//
// ## Steps
//
// Steps apply transformations using Actions. A step can:
//   - Have multiple inputs (accumulated from multiple links)
//   - Chain to other steps
//   - Be a Transformer, Validator, or Condition
//
// ## Arrays
//
// When a source field is an array with Items defined, each array element is
// extracted as a separate SourceFieldValue. A single link from the item field
// to a step will pass ALL array elements as inputs:
//
//	Field{ID: "numbers", Type: Array, Items: &Field{ID: "num"}}
//	Link: num → add_step
//
//	Source: [1, 2, 3, 4, 5] → add_step receives (1, 2, 3, 4, 5) → sum = 15
//
// ## Conditional Steps
//
// When a Condition step breaks (returns false), the chain stops but other
// inputs to downstream steps still flow. This allows filtering:
//
//	value_a → is_even (condition) → add_step → result
//	value_b ----------------------→ add_step
//
//	If value_a is odd: is_even breaks, only value_b reaches add_step
//	If value_a is even: both values reach add_step and are added
//
// ## Execution Flow
//
//  1. Compile() - Validates links and creates Step instances
//  2. ExecuteMapping() - Processes source data:
//     a. Extract source field values
//     b. Execute source links (links with field sources)
//     c. For each link, accumulate step inputs
//     d. When a step has all inputs, execute it
//     e. Execute child links (links sourced from that step)
//     f. Generate target output from TargetFieldValues
//
// # Example
//
//	// Define a mapping that uppercases a name
//	mapping := NewMappingDefinition(
//	    MappingDefinitionFields{ID: "example"},
//	    fields.Fields{{ID: "name", Path: "name", Type: models.ValueTypeString}},
//	    fields.Fields{{ID: "upper_name", Path: "upper_name", Type: models.ValueTypeString}},
//	    []models.StepDefinition{{
//	        ID: "uppercase",
//	        Type: models.StepTypeTransformer,
//	        Action: models.ActionDefinition{Key: "text_to_upper"},
//	    }},
//	    links.Links{
//	        {Priority: 0, Source: links.LinkDirection{FieldID: "name"}, Target: links.LinkDirection{StepID: "uppercase"}},
//	        {Priority: 1, Source: links.LinkDirection{StepID: "uppercase"}, Target: links.LinkDirection{FieldID: "upper_name"}},
//	    },
//	)
//
//	result, _ := mapping.ExecuteMapping(map[string]any{"name": "hello"})
//	// result.TargetRaw["upper_name"] == "HELLO"
package mapping

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/fields"
	"github.com/Ramsey-B/lotus/pkg/links"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/steps"
	"github.com/Ramsey-B/lotus/pkg/utils"
	"github.com/google/uuid"
)

// SourceFieldValue holds the extracted value from a source field.
// It tracks the field ID, a trace ID for debugging, and the actual value.
type SourceFieldValue struct {
	FieldID string `json:"field_id"` // ID of the source field
	TraceID string `json:"trace_id"` // Unique ID for tracing this extraction
	Index   int    `json:"index"`    // Index for array items (0 for non-arrays)
	Value   any    `json:"value"`    // The extracted value
	Error   error  `json:"error"`    // Any error during extraction
}

// TargetFieldValue holds the value to be set on a target field.
// Supports nested objects (Fields) and arrays (Items).
type TargetFieldValue struct {
	FieldID string             `json:"field_id"` // ID of the target field
	TraceID string             `json:"trace_id"` // Unique ID for tracing
	Index   int                `json:"index"`    // Index for array items
	Value   any                `json:"value"`    // The value to set (for leaf fields)
	Items   []TargetFieldValue `json:"items"`    // Array items (for array fields)
	Fields  []TargetFieldValue `json:"fields"`   // Nested fields (for object fields)
	Error   error              `json:"error"`    // Any error during setting
}

// MappingDefinitionFields contains metadata for a mapping definition.
// These are persisted to the database and used for management/querying.
type MappingDefinitionFields struct {
	ID          string    `json:"id" param:"id" validate:"omitempty"`
	TenantID    string    `json:"tenant_id" validate:"omitempty"`
	UserID      string    `json:"user_id" validate:"omitempty"`
	Name        string    `json:"name" validate:"omitempty"`
	Key         string    `json:"key" validate:"omitempty"`
	Description string    `json:"description" validate:"omitempty"`
	Tags        []string  `json:"tags" validate:"omitempty"`
	Version     int       `json:"version" validate:"omitempty,min=1"`
	CreatedTS   time.Time `json:"created_ts" validate:"omitempty"`
	UpdatedTS   time.Time `json:"updated_ts" validate:"omitempty"`
	IsActive    bool      `json:"is_active" validate:"omitempty"`
	IsDeleted   bool      `json:"is_deleted" validate:"omitempty"`
}

// MappingDefinition is the blueprint for transforming source data to target data.
//
// It defines:
//   - SourceFields: Schema of expected input data
//   - TargetFields: Schema of output data
//   - StepDefinitions: Transformation steps (keyed by step ID)
//   - Links: Data flow connections
//
// Call Compile() before ExecuteMapping() to validate and prepare the mapping.
type MappingDefinition struct {
	MappingDefinitionFields
	SourceFields    fields.Fields                    `json:"source_fields" validate:"required"`
	TargetFields    fields.Fields                    `json:"target_fields" validate:"required"`
	StepDefinitions map[string]models.StepDefinition `json:"steps" validate:"omitempty"`
	Links           links.Links                      `json:"links" validate:"required"`
	Steps           map[string]*steps.Step           `json:"-" validate:"omitempty" db:"-"` // Compiled Step instances

	// Compiled state - pre-computed for performance
	compiled           bool              `json:"-"` // true if Compile() has been called
	sourceLinks        links.Links       `json:"-"` // pre-filtered links with field sources
	pathToTargetFields fields.FieldPaths `json:"-"` // target field ID -> path lookup
}

// Mapping is an instance of a MappingDefinition executing on specific source data.
//
// It holds:
//   - SourceRaw: The input data being transformed
//   - TargetRaw: The final output (after GenerateTargetRaw)
//   - SourceFieldValues: Values extracted from source data
//   - TargetFieldValues: Values assigned to target fields
//   - StepResults: Output from executed transformation steps
//   - StepInputs: Accumulated inputs for each step (per-execution)
//   - StepPendingInputs: Count of links still pending for each step
//
// Use sync.Pool (via ExecuteMappingPooled/ReleaseMapping) for high-throughput scenarios.
type Mapping struct {
	MappingDefinition
	SourceRaw          any                          `json:"source_raw" validate:"omitempty"` // Input data
	TargetRaw          map[string]any               `json:"target_raw" validate:"omitempty"` // Final output
	SourceFieldValues  []SourceFieldValue           `json:"-" validate:"omitempty"`          // Extracted source values
	TargetFieldValues  map[string]TargetFieldValue  `json:"-" validate:"omitempty"`          // Mapped target values
	StepResults        map[string]models.StepOutput `json:"-" validate:"omitempty"`          // Step execution results
	StepInputs         map[string][]any             `json:"-" validate:"omitempty"`          // Per-execution input accumulation
	StepPendingInputs  map[string]int               `json:"-" validate:"omitempty"`          // Links pending per step
	PathToTargetFields fields.FieldPaths            `json:"-"`                               // Target field path lookup

	// Indexed array-object support:
	// When mapping array item fields into target fields under an array, we preserve per-item indices and
	// merge target writes into a coherent array of objects.
	targetArrayIndices map[string]map[int]struct{}   // array_root_field_id -> set(indices)
	pendingBroadcasts  map[string][]pendingBroadcast // array_root_field_id -> broadcasts
}

type pendingBroadcast struct {
	FieldID string
	Value   any
}

func NewMappingDefinition(
	fields MappingDefinitionFields,
	sourceFields fields.Fields,
	targetFields fields.Fields,
	steps []models.StepDefinition,
	links links.Links,
) *MappingDefinition {
	ctx := &MappingDefinition{
		MappingDefinitionFields: fields,
		SourceFields:            sourceFields,
		TargetFields:            targetFields,
		StepDefinitions:         make(map[string]models.StepDefinition),
	}

	links.ToPriority()

	ctx.Links = links

	for _, step := range steps {
		ctx.StepDefinitions[step.ID] = step
	}

	return ctx
}

// Compile pre-computes the mapping plan and caches derived state for performance.
// Call this once after creating the mapping definition, then reuse for multiple ExecuteMapping calls.
// Returns an error if the mapping definition is invalid.
func (m *MappingDefinition) Compile() error {
	if m.compiled {
		return nil // Already compiled
	}

	// Generate the mapping plan (steps, validations)
	err := m.GenerateMappingPlan()
	if err != nil {
		return err
	}

	// Pre-compute source links (links that start from a field)
	m.sourceLinks = m.Links.Filter(func(link links.Link) bool {
		// Source links are executed first (before any step outputs exist).
		// These are links that start from a source field or a literal constant.
		return link.Source.FieldID != "" || link.Source.Constant != nil
	})

	// Pre-compute target field paths
	m.pathToTargetFields = m.TargetFields.GetFieldPaths()

	m.compiled = true
	return nil
}

// IsCompiled returns true if Compile() has been called successfully.
func (m *MappingDefinition) IsCompiled() bool {
	return m.compiled
}

// ExecuteMapping executes the mapping against the provided source data.
// If the mapping hasn't been compiled, it will be compiled automatically (slower).
// For best performance, call Compile() once and reuse the mapping definition.
func (m *MappingDefinition) ExecuteMapping(sourceRaw any) (*Mapping, error) {
	// Auto-compile if not already done (backwards compatible, but slower)
	if !m.compiled {
		if err := m.Compile(); err != nil {
			return nil, err
		}
	}

	mappingResult := &Mapping{
		MappingDefinition:  *m,
		TargetRaw:          make(map[string]any),
		SourceFieldValues:  make([]SourceFieldValue, 0),
		TargetFieldValues:  make(map[string]TargetFieldValue),
		StepResults:        make(map[string]models.StepOutput),
		StepInputs:         make(map[string][]any),
		StepPendingInputs:  make(map[string]int),
		targetArrayIndices: make(map[string]map[int]struct{}),
		pendingBroadcasts:  make(map[string][]pendingBroadcast),
	}

	// Use pre-computed path (from compiled state)
	mappingResult.PathToTargetFields = m.pathToTargetFields

	// Count how many links target each step (for tracking when all inputs have arrived)
	for _, link := range m.Links {
		if link.Target.StepID != "" {
			mappingResult.StepPendingInputs[link.Target.StepID]++
		}
	}

	err := mappingResult.AddSourceRaw(sourceRaw)
	if err != nil {
		return nil, err
	}

	// Use pre-computed source links (from compiled state)
	for _, link := range m.sourceLinks {
		err := mappingResult.ExecuteLink(link)
		if err != nil {
			return nil, err
		}
	}

	raw, err := mappingResult.generateTargetRaw()
	if err != nil {
		return nil, err
	}

	mappingResult.TargetRaw = raw

	return mappingResult, nil
}

func (m *Mapping) getArrayRootIDForTargetField(fieldID string) (string, bool, error) {
	path, err := m.PathToTargetFields.GetPathToField(fieldID)
	if err != nil {
		return "", false, err
	}
	for _, p := range path {
		if p.IsItem {
			// The item path stores the parent array field ID in ParentID.
			if p.ParentID != "" {
				return p.ParentID, true, nil
			}
		}
	}
	return "", false, nil
}

func (m *Mapping) recordArrayIndex(rootID string, index int) {
	if rootID == "" || index < 0 {
		return
	}
	if m.targetArrayIndices == nil {
		m.targetArrayIndices = make(map[string]map[int]struct{}, 2)
	}
	set, ok := m.targetArrayIndices[rootID]
	if !ok {
		set = make(map[int]struct{}, 4)
		m.targetArrayIndices[rootID] = set
	}
	set[index] = struct{}{}
}

func (m *Mapping) addBroadcast(rootID, fieldID string, value any) {
	if rootID == "" {
		return
	}
	if m.pendingBroadcasts == nil {
		m.pendingBroadcasts = make(map[string][]pendingBroadcast, 2)
	}
	m.pendingBroadcasts[rootID] = append(m.pendingBroadcasts[rootID], pendingBroadcast{FieldID: fieldID, Value: value})
}

func (m *Mapping) applyBroadcasts() error {
	if m == nil || len(m.pendingBroadcasts) == 0 {
		return nil
	}
	for rootID, bcasts := range m.pendingBroadcasts {
		indices := m.targetArrayIndices[rootID]
		if len(indices) == 0 {
			continue
		}
		for idx := range indices {
			for _, b := range bcasts {
				if err := m.setTargetFieldValueWithIndex(b.FieldID, b.Value, idx); err != nil {
					return err
				}
			}
		}
	}
	// Clear after application
	for k := range m.pendingBroadcasts {
		delete(m.pendingBroadcasts, k)
	}
	return nil
}

func (m *Mapping) ExecuteLink(link links.Link) error {
	inputs := m.getLinkInputs(link)

	// For field targets, we need inputs - return early if none
	if link.Target.FieldID != "" {
		if len(inputs) == 0 {
			return nil // no inputs, nothing to set
		}
		// Preserve array item indexes when mapping array items -> target fields.
		// If the source is an array item field, there will be multiple SourceFieldValues with Index >= 0.
		if link.Source.FieldID != "" {
			srcField, srcErr := m.SourceFields.GetField(link.Source.FieldID)
			if srcErr == nil && !srcField.IsItem {
				// Scalar source -> scalar target, or scalar broadcast into an array-of-objects target.
				return m.setTargetFieldValueWithIndex(link.Target.FieldID, inputs[0], -1)
			}

			fieldValues := ectolinq.Filter(m.SourceFieldValues, func(fieldValue SourceFieldValue) bool {
				return fieldValue.FieldID == link.Source.FieldID
			})
			for _, fv := range fieldValues {
				if err := m.setTargetFieldValueWithIndex(link.Target.FieldID, fv.Value, fv.Index); err != nil {
					return err
				}
			}
		} else {
			// Constant or step output: write as scalar (index = -1).
			if err := m.setTargetFieldValueWithIndex(link.Target.FieldID, inputs[0], -1); err != nil {
				return err
			}
		}
		return nil
	}

	// For step targets, we always process the link (even if inputs are empty due to conditional break)
	if link.Target.StepID != "" {
		step, ok := m.Steps[link.Target.StepID]
		if !ok {
			return errors.NewMappingError("step not defined").AddStep(link.Target.StepID)
		}

		// Initialize step inputs map if needed
		if m.StepInputs == nil {
			m.StepInputs = make(map[string][]any)
		}

		// Accumulate inputs for this step (if any - may be empty if conditional broke)
		if len(inputs) > 0 {
			stepInputs := m.StepInputs[link.Target.StepID]
			stepInputs = append(stepInputs, inputs...)
			m.StepInputs[link.Target.StepID] = stepInputs
		}

		// Decrement pending count - this link has been processed (whether it had inputs or not)
		m.StepPendingInputs[link.Target.StepID]--

		// Only execute when ALL links to this step have been processed
		if m.StepPendingInputs[link.Target.StepID] > 0 {
			return nil // still waiting for other links
		}

		// All links processed - execute with whatever inputs we collected
		stepInputs := m.StepInputs[link.Target.StepID]

		// If no inputs at all (all conditionals broke), don't execute
		if len(stepInputs) == 0 {
			return nil
		}

		output, err := step.Execute(stepInputs...)
		if err != nil {
			return errors.WrapMappingError(err).AddLink(link.GetLinkID())
		}

		m.StepResults[link.Target.StepID] = output

		// Always process child links, even if Break=true.
		// Child links will get empty inputs (getLinkInputs checks Break),
		// which properly decrements pending counts for downstream steps.
		for _, childLink := range m.Links.Filter(func(l links.Link) bool {
			return l.Source.StepID == link.Target.StepID
		}) {
			err := m.ExecuteLink(childLink)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// setTargetFieldValueWithIndex sets a target field value, optionally scoped to an array item index.
// index == -1 means "scalar write"; if the target field is nested under an array item, this becomes
// a broadcast write applied later to all discovered indices for that array.
func (m *Mapping) setTargetFieldValueWithIndex(fieldID string, value any, index int) error {
	// Determine whether this target field is nested under an array item.
	rootID, underArray, err := m.getArrayRootIDForTargetField(fieldID)
	if err != nil {
		return err
	}

	// Broadcast behavior: scalar sources targeting array-item fields should apply to all indices
	// discovered for that array root.
	if underArray && index < 0 {
		m.addBroadcast(rootID, fieldID, value)
		return nil
	}
	if underArray {
		m.recordArrayIndex(rootID, index)
	}

	path, err := m.PathToTargetFields.GetPathToField(fieldID)
	if err != nil {
		return err
	}

	rootPath := ectolinq.At(path, 0)
	fieldValue, ok := m.TargetFieldValues[rootPath.FieldID]
	if !ok {
		fieldValue = TargetFieldValue{
			FieldID: rootPath.FieldID,
			Index:   0,
		}
	}

	var setErr error
	defer func() {
		fieldValue.Error = setErr
		m.TargetFieldValues[fieldValue.FieldID] = fieldValue
	}()

	if len(path) == 1 {
		fieldValue, setErr = m.addValueToTargetField(fieldValue, value)
		return setErr
	}

	fieldValue, setErr = m.setChildTargetFieldValueWithIndex(path[1:], fieldValue, value, index)
	return setErr
}

func (m *Mapping) getLinkInputs(link links.Link) []any {
	inputs := make([]any, 0)

	if link.Source.FieldID != "" {
		fieldValues := ectolinq.Filter(m.SourceFieldValues, func(fieldValue SourceFieldValue) bool {
			return fieldValue.FieldID == link.Source.FieldID
		})

		for _, fieldValue := range fieldValues {
			inputs = append(inputs, fieldValue.Value)
		}
	}

	if link.Source.StepID != "" {
		stepResult, ok := m.StepResults[link.Source.StepID]
		if ok && !stepResult.Break && stepResult.Output != nil {
			// Only use the step output if it has actually executed and produced output
			inputs = append(inputs, stepResult.Output)
		}
	}

	if link.Source.Constant != nil {
		inputs = append(inputs, link.Source.Constant)
	}

	return inputs
}

func (m *Mapping) AddSourceRaw(sourceRaw any) error {
	m.SourceRaw = sourceRaw

	for _, field := range m.SourceFields {
		err := m.setSourceFieldValue(field, m.SourceRaw, 0)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Mapping) setSourceFieldValue(field fields.Field, raw any, index int) error {
	var err error
	var input any
	fieldResult := &SourceFieldValue{
		FieldID: field.ID,
		TraceID: uuid.New().String(),
		Index:   index,
	}

	defer func() {
		fieldResult.Error = err
		fieldResult.Value = input
		m.SourceFieldValues = append(m.SourceFieldValues, *fieldResult)
	}()

	// get value
	fieldValue, err := utils.GetFieldByPathI(raw, field.Path)
	if err != nil {
		if strings.Contains(err.Error(), "unable to find the key") {
			if field.Required {
				err = errors.NewMappingError("field is required but missing value").AddField(field.ID)
				return err
			}

			if ectolinq.IsEmpty(field.Default) {
				input = models.GetDefault(field.GetType())
			} else {
				input = field.Default
			}
		} else {
			err = errors.WrapMappingError(err).AddField(field.ID)
			return err
		}
	} else {
		// Check if the reflect.Value is valid before calling Interface()
		// A zero reflect.Value (from nil) would panic
		if fieldValue.IsValid() {
			input = fieldValue.Interface()
		} else {
			input = nil
		}
	}

	err = models.IsActionValueType(input, field.GetType())
	if err != nil {
		err = errors.WrapMappingError(err).AddField(field.ID)
		return err
	}

	// set field values for nested fields
	for _, f := range field.Fields {
		// Preserve the current index when traversing nested fields so array item fields
		// (e.g. items[].id) retain their per-item index.
		err := m.setSourceFieldValue(f, input, index)
		if err != nil {
			return err
		}
	}

	if field.Items != nil {
		arr, err := toArray(input)
		if err != nil {
			err = errors.WrapMappingError(err).AddField(field.ID)
			return err
		}

		// set field items
		err = m.setSourceFieldItems(*field.Items, arr)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Mapping) setSourceFieldItems(item fields.Field, values []any) error {
	for i, value := range values {
		err := m.setSourceFieldValue(item, value, i)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Mapping) setChildTargetFieldValueWithIndex(path fields.FieldPaths, parentValue TargetFieldValue, value any, index int) (TargetFieldValue, error) {
	next := ectolinq.At(path, 0)

	if len(path) == 1 {
		leaf := TargetFieldValue{
			FieldID: next.FieldID,
			Index:   0,
		}
		// For array items, the index is the array index.
		if next.IsItem {
			if index < 0 {
				return parentValue, errors.NewMappingError("cannot set array item field without an index").AddField(next.FieldID)
			}
			leaf.Index = index
		}

		leaf, err := m.addValueToTargetField(leaf, value)
		if err != nil {
			return parentValue, err
		}

		if next.IsItem {
			// Upsert item by (FieldID, Index)
			replaced := false
			for i := range parentValue.Items {
				if parentValue.Items[i].FieldID == leaf.FieldID && parentValue.Items[i].Index == leaf.Index {
					parentValue.Items[i] = leaf
					replaced = true
					break
				}
			}
			if !replaced {
				parentValue.Items = append(parentValue.Items, leaf)
			}
			return parentValue, nil
		}

		if next.IsField {
			// Upsert field by FieldID
			replaced := false
			for i := range parentValue.Fields {
				if parentValue.Fields[i].FieldID == leaf.FieldID {
					parentValue.Fields[i] = leaf
					replaced = true
					break
				}
			}
			if !replaced {
				parentValue.Fields = append(parentValue.Fields, leaf)
			}
			return parentValue, nil
		}

		// Fallback: treat as a direct child field
		parentValue.Fields = append(parentValue.Fields, leaf)
		return parentValue, nil
	}

	if next.IsItem {
		if index < 0 {
			return parentValue, errors.NewMappingError("cannot set array item field without an index").AddField(next.FieldID)
		}
		// Upsert the array item container by (FieldID, Index)
		var item *TargetFieldValue
		for i := range parentValue.Items {
			if parentValue.Items[i].FieldID == next.FieldID && parentValue.Items[i].Index == index {
				item = &parentValue.Items[i]
				break
			}
		}
		if item == nil {
			parentValue.Items = append(parentValue.Items, TargetFieldValue{FieldID: next.FieldID, Index: index})
			item = &parentValue.Items[len(parentValue.Items)-1]
		}

		child, err := m.setChildTargetFieldValueWithIndex(path[1:], *item, value, index)
		if err != nil {
			return parentValue, err
		}
		*item = child
	}

	if next.IsField {
		// Upsert the nested object container by FieldID
		var field *TargetFieldValue
		for i := range parentValue.Fields {
			if parentValue.Fields[i].FieldID == next.FieldID {
				field = &parentValue.Fields[i]
				break
			}
		}
		if field == nil {
			parentValue.Fields = append(parentValue.Fields, TargetFieldValue{FieldID: next.FieldID, Index: 0})
			field = &parentValue.Fields[len(parentValue.Fields)-1]
		}

		child, err := m.setChildTargetFieldValueWithIndex(path[1:], *field, value, index)
		if err != nil {
			return parentValue, err
		}
		*field = child
	}

	return parentValue, nil
}

func (m *Mapping) addValueToTargetField(fieldValue TargetFieldValue, value any) (TargetFieldValue, error) {
	var targetField fields.Field
	targetField, err := m.TargetFields.GetField(fieldValue.FieldID)
	if err != nil {
		return fieldValue, err
	}

	if ectolinq.IsEmpty(value) {
		if targetField.Required {
			return fieldValue, errors.NewMappingError("field is required but missing value").AddField(fieldValue.FieldID)
		}

		if ectolinq.IsEmpty(targetField.Default) {
			value = models.GetDefault(targetField.GetType())
		} else {
			value = targetField.Default
		}
	}

	err = fieldValue.SetValue(value, targetField.GetType())
	if err != nil {
		return fieldValue, err
	}

	return fieldValue, nil
}

func toArray(val any) ([]any, error) {
	v := reflect.ValueOf(val)
	if v.Kind() != reflect.Slice {
		return nil, fmt.Errorf("expected array but got: %v", val)
	}

	// Create a slice to hold the results.
	result := make([]any, v.Len())

	// Iterate over the slice and assign each element.
	for i := 0; i < v.Len(); i++ {
		result[i] = v.Index(i).Interface()
	}

	return result, nil
}

func (f *TargetFieldValue) SetValue(value any, expectedType models.ActionValueType) error {
	f.Value = value

	err := models.IsActionValueType(value, expectedType)
	if err != nil {
		err = errors.WrapMappingError(err).AddField(f.FieldID)
		return err
	}

	return nil
}

func (f *TargetFieldValue) AddItem(item TargetFieldValue) {
	f.Items = append(f.Items, item)
}

func (f *TargetFieldValue) AddField(field TargetFieldValue) {
	f.Fields = append(f.Fields, field)
}

func (m *Mapping) generateTargetRaw() (map[string]any, error) {
	// Apply any deferred broadcasts (scalar values targeting array-item fields).
	if err := m.applyBroadcasts(); err != nil {
		return nil, err
	}

	// Use capacity, not length - otherwise we get empty elements at the start
	fields := make([]TargetFieldValue, 0, len(m.TargetFieldValues))
	for _, field := range m.TargetFieldValues {
		fields = append(fields, field)
	}

	raw, err := m.getTargetRawObject(fields)
	if err != nil {
		return nil, err
	}

	return raw, nil
}

func (m *Mapping) getTargetRawArrays(values []TargetFieldValue) ([]any, error) {
	// Ensure stable ordering for arrays when values carry an Index.
	sort.SliceStable(values, func(i, j int) bool {
		return values[i].Index < values[j].Index
	})

	items := make([]any, 0)

	for _, value := range values {
		if len(value.Fields) > 0 {
			obj, err := m.getTargetRawObject(value.Fields)
			if err != nil {
				return nil, err
			}

			items = append(items, obj)
		} else if len(value.Items) > 0 {
			arr, err := m.getTargetRawArrays(value.Items)
			if err != nil {
				return nil, err
			}

			items = append(items, arr)
		} else {
			items = append(items, value.Value)
		}
	}

	return items, nil
}

func (m *Mapping) getTargetRawObject(values []TargetFieldValue) (map[string]any, error) {
	fields := make(map[string]any)

	for _, value := range values {
		// Get the target field to use its Path (not just FieldID)
		targetField, err := m.TargetFields.GetField(value.FieldID)
		if err != nil {
			// If field not found, fall back to using FieldID
			targetField.Path = value.FieldID
		}

		// Use the field's Path for the output key
		outputPath := targetField.Path
		if outputPath == "" {
			outputPath = value.FieldID
		}

		if len(value.Fields) > 0 {
			obj, err := m.getTargetRawObject(value.Fields)
			if err != nil {
				return nil, err
			}

			fields = utils.AssignMapValue(fields, outputPath, obj)
		} else if len(value.Items) > 0 {
			arr, err := m.getTargetRawArrays(value.Items)
			if err != nil {
				return nil, err
			}

			fields = utils.AssignMapValue(fields, outputPath, arr)
		} else {
			fields = utils.AssignMapValue(fields, outputPath, value.Value)
		}
	}

	return fields, nil
}
