package mapping

import (
	"sync"

	"github.com/Ramsey-B/lotus/pkg/fields"
	"github.com/Ramsey-B/lotus/pkg/models"
)

// mappingPool provides object pooling for Mapping structs to reduce allocations.
var mappingPool = sync.Pool{
	New: func() any {
		return &Mapping{
			TargetRaw:         make(map[string]any, 8),
			SourceFieldValues: make([]SourceFieldValue, 0, 16),
			TargetFieldValues: make(map[string]TargetFieldValue, 8),
			StepResults:       make(map[string]models.StepOutput, 4),
			StepInputs:        make(map[string][]any, 4),
			StepPendingInputs: make(map[string]int, 4),
			targetArrayIndices: make(map[string]map[int]struct{}, 2),
			pendingBroadcasts:  make(map[string][]pendingBroadcast, 2),
		}
	},
}

// AcquireMapping gets a Mapping from the pool.
// The returned mapping should be released with ReleaseMapping after use.
func AcquireMapping() *Mapping {
	return mappingPool.Get().(*Mapping)
}

// ReleaseMapping returns a Mapping to the pool for reuse.
// The mapping should not be used after calling this.
func ReleaseMapping(m *Mapping) {
	if m == nil {
		return
	}
	m.Reset()
	mappingPool.Put(m)
}

// Reset clears the mapping state for reuse.
func (m *Mapping) Reset() {
	// Clear the definition reference
	m.MappingDefinition = MappingDefinition{}

	// Clear source data
	m.SourceRaw = nil

	// Clear target raw map (reuse the map, just clear entries)
	for k := range m.TargetRaw {
		delete(m.TargetRaw, k)
	}

	// Clear source field values (reuse slice capacity)
	m.SourceFieldValues = m.SourceFieldValues[:0]

	// Clear target field values map
	for k := range m.TargetFieldValues {
		delete(m.TargetFieldValues, k)
	}

	// Clear step results map
	for k := range m.StepResults {
		delete(m.StepResults, k)
	}

	// Clear step inputs map (per-execution state)
	for k := range m.StepInputs {
		delete(m.StepInputs, k)
	}

	// Clear step pending inputs map
	for k := range m.StepPendingInputs {
		delete(m.StepPendingInputs, k)
	}

	// Clear indexed-array bookkeeping
	if m.targetArrayIndices != nil {
		for k := range m.targetArrayIndices {
			delete(m.targetArrayIndices, k)
		}
	}
	if m.pendingBroadcasts != nil {
		for k := range m.pendingBroadcasts {
			delete(m.pendingBroadcasts, k)
		}
	}

	// Clear path
	m.PathToTargetFields = nil
}

// ExecuteMappingPooled executes the mapping using a pooled Mapping struct.
// This reduces allocations by reusing Mapping objects.
// The returned Mapping is borrowed from the pool - call ReleaseMapping when done.
//
// Usage:
//
//	result, err := mapping.ExecuteMappingPooled(sourceData)
//	if err != nil { return err }
//	defer ReleaseMapping(result)
//	// use result...
func (m *MappingDefinition) ExecuteMappingPooled(sourceRaw any) (*Mapping, error) {
	// Auto-compile if not already done
	if !m.compiled {
		if err := m.Compile(); err != nil {
			return nil, err
		}
	}

	// Get a mapping from the pool
	mappingResult := AcquireMapping()

	// Initialize with this definition
	mappingResult.MappingDefinition = *m
	mappingResult.PathToTargetFields = m.pathToTargetFields

	// Ensure maps are initialized (pool may have nil maps on first use)
	if mappingResult.TargetRaw == nil {
		mappingResult.TargetRaw = make(map[string]any, 8)
	}
	if mappingResult.SourceFieldValues == nil {
		mappingResult.SourceFieldValues = make([]SourceFieldValue, 0, 16)
	}
	if mappingResult.TargetFieldValues == nil {
		mappingResult.TargetFieldValues = make(map[string]TargetFieldValue, 8)
	}
	if mappingResult.StepResults == nil {
		mappingResult.StepResults = make(map[string]models.StepOutput, 4)
	}
	if mappingResult.StepInputs == nil {
		mappingResult.StepInputs = make(map[string][]any, 4)
	}
	if mappingResult.StepPendingInputs == nil {
		mappingResult.StepPendingInputs = make(map[string]int, 4)
	}
	if mappingResult.targetArrayIndices == nil {
		mappingResult.targetArrayIndices = make(map[string]map[int]struct{}, 2)
	}
	if mappingResult.pendingBroadcasts == nil {
		mappingResult.pendingBroadcasts = make(map[string][]pendingBroadcast, 2)
	}

	// Count how many links target each step (for tracking when all inputs have arrived)
	for _, link := range m.Links {
		if link.Target.StepID != "" {
			mappingResult.StepPendingInputs[link.Target.StepID]++
		}
	}

	err := mappingResult.AddSourceRaw(sourceRaw)
	if err != nil {
		ReleaseMapping(mappingResult)
		return nil, err
	}

	// Use pre-computed source links
	for _, link := range m.sourceLinks {
		err := mappingResult.ExecuteLink(link)
		if err != nil {
			ReleaseMapping(mappingResult)
			return nil, err
		}
	}

	raw, err := mappingResult.generateTargetRaw()
	if err != nil {
		ReleaseMapping(mappingResult)
		return nil, err
	}

	mappingResult.TargetRaw = raw

	return mappingResult, nil
}

// sourceFieldValuePool provides pooling for SourceFieldValue slices.
var sourceFieldValuePool = sync.Pool{
	New: func() any {
		slice := make([]SourceFieldValue, 0, 16)
		return &slice
	},
}

// AcquireSourceFieldValues gets a SourceFieldValue slice from the pool.
func AcquireSourceFieldValues() *[]SourceFieldValue {
	return sourceFieldValuePool.Get().(*[]SourceFieldValue)
}

// ReleaseSourceFieldValues returns a SourceFieldValue slice to the pool.
func ReleaseSourceFieldValues(s *[]SourceFieldValue) {
	if s == nil {
		return
	}
	*s = (*s)[:0]
	sourceFieldValuePool.Put(s)
}

// targetFieldValuePool provides pooling for TargetFieldValue maps.
var targetFieldValueMapPool = sync.Pool{
	New: func() any {
		m := make(map[string]TargetFieldValue, 8)
		return &m
	},
}

// fieldPathsPool provides pooling for FieldPaths slices.
var fieldPathsPool = sync.Pool{
	New: func() any {
		slice := make(fields.FieldPaths, 0, 16)
		return &slice
	},
}

