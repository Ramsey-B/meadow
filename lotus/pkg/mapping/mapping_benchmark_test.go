package mapping

import (
	"fmt"
	"testing"

	"github.com/Ramsey-B/lotus/pkg/actions"
	"github.com/Ramsey-B/lotus/pkg/actions/registry"
	"github.com/Ramsey-B/lotus/pkg/fields"
	"github.com/Ramsey-B/lotus/pkg/links"
	"github.com/Ramsey-B/lotus/pkg/models"
)

func init() {
	// Register all actions for benchmarks
	for _, action := range actions.ActionDefinitions {
		registry.Actions[action.Key] = action.Factory
	}
}

// createSimpleMapping creates a mapping with N direct field-to-field links (no transforms)
func createSimpleMapping(numFields int) (*MappingDefinition, map[string]any) {
	sourceFields := make(fields.Fields, numFields)
	targetFields := make(fields.Fields, numFields)
	linkList := make(links.Links, numFields)
	sourceData := make(map[string]any)

	for i := 0; i < numFields; i++ {
		sourceFieldID := fmt.Sprintf("source_field_%d", i)
		targetFieldID := fmt.Sprintf("target_field_%d", i)
		sourcePath := fmt.Sprintf("field_%d", i)
		
		sourceFields[i] = fields.Field{
			ID:   sourceFieldID,
			Name: fmt.Sprintf("Source Field %d", i),
			Path: sourcePath,
			Type: models.ValueTypeString,
		}
		// ID matches the desired output key
		targetFields[i] = fields.Field{
			ID:   targetFieldID,
			Name: fmt.Sprintf("Target Field %d", i),
			Path: targetFieldID,
			Type: models.ValueTypeString,
		}
		linkList[i] = links.Link{
			Priority: i,
			Source:   links.LinkDirection{FieldID: sourceFieldID},
			Target:   links.LinkDirection{FieldID: targetFieldID},
		}
		sourceData[sourcePath] = fmt.Sprintf("value_%d", i)
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "simple-benchmark"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	return mapping, sourceData
}

// createMappingWithTransforms creates a mapping with N fields and transforms
func createMappingWithTransforms(numFields int) (*MappingDefinition, map[string]any) {
	sourceFields := make(fields.Fields, numFields)
	targetFields := make(fields.Fields, numFields)
	stepDefs := make([]models.StepDefinition, numFields)
	linkList := make(links.Links, 0, numFields*2)
	sourceData := make(map[string]any)

	for i := 0; i < numFields; i++ {
		sourceFieldID := fmt.Sprintf("source_field_%d", i)
		targetFieldID := fmt.Sprintf("target_field_%d", i)
		sourcePath := fmt.Sprintf("field_%d", i)
		stepID := fmt.Sprintf("step_%d", i)

		sourceFields[i] = fields.Field{
			ID:   sourceFieldID,
			Name: fmt.Sprintf("Source Field %d", i),
			Path: sourcePath,
			Type: models.ValueTypeString,
		}
		targetFields[i] = fields.Field{
			ID:   targetFieldID,
			Name: fmt.Sprintf("Target Field %d", i),
			Path: targetFieldID,
			Type: models.ValueTypeString,
		}

		// Add a transform step (to_upper)
		stepDefs[i] = models.StepDefinition{
			ID:   stepID,
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "text_to_upper",
			},
		}

		// Link source -> step
		linkList = append(linkList, links.Link{
			Priority: i * 2,
			Source:   links.LinkDirection{FieldID: sourceFieldID},
			Target:   links.LinkDirection{StepID: stepID},
		})

		// Link step -> target
		linkList = append(linkList, links.Link{
			Priority: i*2 + 1,
			Source:   links.LinkDirection{StepID: stepID},
			Target:   links.LinkDirection{FieldID: targetFieldID},
		})

		sourceData[sourcePath] = fmt.Sprintf("value_%d", i)
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "transform-benchmark"},
		sourceFields,
		targetFields,
		stepDefs,
		linkList,
	)

	return mapping, sourceData
}

// createNestedMapping creates a mapping with nested objects
func createNestedMapping(depth int) (*MappingDefinition, map[string]any) {
	// Create a nested object structure
	sourceData := make(map[string]any)
	current := sourceData

	// Build nested source data
	for i := 0; i < depth-1; i++ {
		nested := make(map[string]any)
		current[fmt.Sprintf("level_%d", i)] = nested
		current = nested
	}
	current["value"] = "deep_value"

	// Build field path
	path := ""
	for i := 0; i < depth-1; i++ {
		if path != "" {
			path += "."
		}
		path += fmt.Sprintf("level_%d", i)
	}
	if path != "" {
		path += ".value"
	} else {
		path = "value"
	}

	sourceFields := fields.Fields{
		{
			ID:   "source_nested",
			Name: "Nested Source",
			Path: path,
			Type: models.ValueTypeString,
		},
	}

	targetFields := fields.Fields{
		{
			ID:   "output",
			Name: "Nested Target",
			Path: "output",
			Type: models.ValueTypeString,
		},
	}

	linkList := links.Links{
		{
			Priority: 0,
			Source:   links.LinkDirection{FieldID: "source_nested"},
			Target:   links.LinkDirection{FieldID: "output"},
		},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "nested-benchmark"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	return mapping, sourceData
}

// createArrayMapping creates a mapping with array data
func createArrayMapping(arraySize int) (*MappingDefinition, map[string]any) {
	// Create array data - must be []any for type validation
	items := make([]any, arraySize)
	for i := 0; i < arraySize; i++ {
		items[i] = fmt.Sprintf("item_%d", i)
	}

	sourceData := map[string]any{
		"items": items,
	}

	sourceFields := fields.Fields{
		{
			ID:   "source_array",
			Name: "Source Array",
			Path: "items",
			Type: models.ValueTypeArray,
			Items: &fields.Field{
				ID:   "source_array_item",
				Name: "Source Array Item",
				Path: "",
				Type: models.ValueTypeString,
			},
		},
	}

	targetFields := fields.Fields{
		{
			ID:   "output",
			Name: "Target Array",
			Path: "output",
			Type: models.ValueTypeArray,
			Items: &fields.Field{
				ID:   "output_item",
				Name: "Target Array Item",
				Path: "",
				Type: models.ValueTypeString,
			},
		},
	}

	linkList := links.Links{
		{
			Priority: 0,
			Source:   links.LinkDirection{FieldID: "source_array"},
			Target:   links.LinkDirection{FieldID: "output"},
		},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "array-benchmark"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	return mapping, sourceData
}

// createOrchidLikeMapping creates a mapping similar to what we'd see from Orchid
func createOrchidLikeMapping() (*MappingDefinition, map[string]any) {
	sourceData := map[string]any{
		"users": []any{
			map[string]any{
				"id":    1,
				"name":  "Alice Johnson",
				"email": "alice@example.com",
				"department": map[string]any{
					"id":   101,
					"name": "Engineering",
				},
				"active": true,
			},
			map[string]any{
				"id":    2,
				"name":  "Bob Smith",
				"email": "bob@example.com",
				"department": map[string]any{
					"id":   102,
					"name": "Marketing",
				},
				"active": true,
			},
		},
		"total":     2,
		"page":      1,
		"page_size": 10,
	}

	sourceFields := fields.Fields{
		{
			ID:   "total",
			Name: "Total",
			Path: "total",
			Type: models.ValueTypeNumber,
		},
		{
			ID:   "page",
			Name: "Page",
			Path: "page",
			Type: models.ValueTypeNumber,
		},
	}

	targetFields := fields.Fields{
		{
			ID:   "record_count",
			Name: "Record Count",
			Path: "record_count",
			Type: models.ValueTypeNumber,
		},
		{
			ID:   "current_page",
			Name: "Current Page",
			Path: "current_page",
			Type: models.ValueTypeNumber,
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{FieldID: "total"}, Target: links.LinkDirection{FieldID: "record_count"}},
		{Priority: 1, Source: links.LinkDirection{FieldID: "page"}, Target: links.LinkDirection{FieldID: "current_page"}},
	}

	mapping := NewMappingDefinition(
		MappingDefinitionFields{ID: "orchid-like-benchmark"},
		sourceFields,
		targetFields,
		nil,
		linkList,
	)

	return mapping, sourceData
}

// Benchmark: Simple mapping with 5 fields (direct field-to-field)
func BenchmarkSimpleMapping5Fields(b *testing.B) {
	mapping, sourceData := createSimpleMapping(5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mapping.ExecuteMapping(sourceData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Simple mapping with 20 fields
func BenchmarkSimpleMapping20Fields(b *testing.B) {
	mapping, sourceData := createSimpleMapping(20)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mapping.ExecuteMapping(sourceData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Simple mapping with 50 fields
func BenchmarkSimpleMapping50Fields(b *testing.B) {
	mapping, sourceData := createSimpleMapping(50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mapping.ExecuteMapping(sourceData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Mapping with transforms (5 fields, each with a to_upper step)
func BenchmarkMappingWithTransforms5(b *testing.B) {
	mapping, sourceData := createMappingWithTransforms(5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mapping.ExecuteMapping(sourceData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Mapping with transforms (20 fields)
func BenchmarkMappingWithTransforms20(b *testing.B) {
	mapping, sourceData := createMappingWithTransforms(20)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mapping.ExecuteMapping(sourceData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Nested object access (depth 5)
func BenchmarkNestedMapping5(b *testing.B) {
	mapping, sourceData := createNestedMapping(5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mapping.ExecuteMapping(sourceData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Nested object access (depth 10)
func BenchmarkNestedMapping10(b *testing.B) {
	mapping, sourceData := createNestedMapping(10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mapping.ExecuteMapping(sourceData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Array mapping (100 items)
func BenchmarkArrayMapping100(b *testing.B) {
	mapping, sourceData := createArrayMapping(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mapping.ExecuteMapping(sourceData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Array mapping (1000 items)
func BenchmarkArrayMapping1000(b *testing.B) {
	mapping, sourceData := createArrayMapping(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mapping.ExecuteMapping(sourceData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Orchid-like API response mapping
func BenchmarkOrchidLikeMapping(b *testing.B) {
	mapping, sourceData := createOrchidLikeMapping()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mapping.ExecuteMapping(sourceData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Plan generation only (to isolate compilation cost)
func BenchmarkPlanGeneration(b *testing.B) {
	mapping, _ := createMappingWithTransforms(20)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset steps to force regeneration
		mapping.Steps = nil
		err := mapping.GenerateMappingPlan()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Parallel execution
func BenchmarkParallelMapping(b *testing.B) {
	mapping, sourceData := createSimpleMapping(10)

	// Pre-generate plan
	if err := mapping.GenerateMappingPlan(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := mapping.ExecuteMapping(sourceData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Benchmark: Memory allocations
func BenchmarkMemoryAllocations(b *testing.B) {
	mapping, sourceData := createSimpleMapping(10)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mapping.ExecuteMapping(sourceData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Pre-compiled mapping (key optimization)
func BenchmarkPreCompiledMapping5Fields(b *testing.B) {
	mapping, sourceData := createSimpleMapping(5)

	// Pre-compile the mapping once
	if err := mapping.Compile(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mapping.ExecuteMapping(sourceData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Pre-compiled mapping with 20 fields
func BenchmarkPreCompiledMapping20Fields(b *testing.B) {
	mapping, sourceData := createSimpleMapping(20)

	// Pre-compile the mapping once
	if err := mapping.Compile(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mapping.ExecuteMapping(sourceData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Pre-compiled mapping with transforms
func BenchmarkPreCompiledWithTransforms20(b *testing.B) {
	mapping, sourceData := createMappingWithTransforms(20)

	// Pre-compile the mapping once
	if err := mapping.Compile(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mapping.ExecuteMapping(sourceData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Pre-compiled Orchid-like mapping
func BenchmarkPreCompiledOrchidLike(b *testing.B) {
	mapping, sourceData := createOrchidLikeMapping()

	// Pre-compile the mapping once
	if err := mapping.Compile(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mapping.ExecuteMapping(sourceData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: Pre-compiled parallel execution
func BenchmarkPreCompiledParallel(b *testing.B) {
	mapping, sourceData := createSimpleMapping(10)

	// Pre-compile the mapping once
	if err := mapping.Compile(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := mapping.ExecuteMapping(sourceData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// ============================================
// Pooled Execution Benchmarks
// ============================================

// Benchmark: Pooled mapping with 5 fields
func BenchmarkPooledMapping5Fields(b *testing.B) {
	mapping, sourceData := createSimpleMapping(5)

	if err := mapping.Compile(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := mapping.ExecuteMappingPooled(sourceData)
		if err != nil {
			b.Fatal(err)
		}
		ReleaseMapping(result)
	}
}

// Benchmark: Pooled mapping with 20 fields
func BenchmarkPooledMapping20Fields(b *testing.B) {
	mapping, sourceData := createSimpleMapping(20)

	if err := mapping.Compile(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := mapping.ExecuteMappingPooled(sourceData)
		if err != nil {
			b.Fatal(err)
		}
		ReleaseMapping(result)
	}
}

// Benchmark: Pooled mapping with transforms
func BenchmarkPooledWithTransforms20(b *testing.B) {
	mapping, sourceData := createMappingWithTransforms(20)

	if err := mapping.Compile(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := mapping.ExecuteMappingPooled(sourceData)
		if err != nil {
			b.Fatal(err)
		}
		ReleaseMapping(result)
	}
}

// Benchmark: Pooled Orchid-like mapping
func BenchmarkPooledOrchidLike(b *testing.B) {
	mapping, sourceData := createOrchidLikeMapping()

	if err := mapping.Compile(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := mapping.ExecuteMappingPooled(sourceData)
		if err != nil {
			b.Fatal(err)
		}
		ReleaseMapping(result)
	}
}

// Benchmark: Pooled parallel execution
func BenchmarkPooledParallel(b *testing.B) {
	mapping, sourceData := createSimpleMapping(10)

	if err := mapping.Compile(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result, err := mapping.ExecuteMappingPooled(sourceData)
			if err != nil {
				b.Fatal(err)
			}
			ReleaseMapping(result)
		}
	})
}

// Benchmark: Pooled memory allocations
func BenchmarkPooledMemoryAllocations(b *testing.B) {
	mapping, sourceData := createSimpleMapping(10)

	if err := mapping.Compile(); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := mapping.ExecuteMappingPooled(sourceData)
		if err != nil {
			b.Fatal(err)
		}
		ReleaseMapping(result)
	}
}

