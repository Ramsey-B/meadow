package mapping

import (
	"testing"

	"github.com/Ramsey-B/lotus/pkg/fields"
	"github.com/Ramsey-B/lotus/pkg/links"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestMapping_ArrayOfObjectsOutput_Relationships(t *testing.T) {
	def := NewMappingDefinition(
		MappingDefinitionFields{ID: "test", TenantID: "t1", UserID: "u1", Name: "test"},
		fields.Fields{
			{ID: "src_owner_id", Path: "response_body.id", Type: models.ValueTypeString},
			{
				ID:   "src_devices",
				Path: "response_body.owned_devices.value",
				Type: models.ValueTypeArray,
				Items: &fields.Field{
					ID:   "src_device",
					Path: "",
					Type: models.ValueTypeObject,
					Fields: fields.Fields{
						{ID: "src_device_id", Path: "id", Type: models.ValueTypeString},
					},
				},
			},
		},
		fields.Fields{
			{
				ID:   "relationships",
				Path: "relationships",
				Type: models.ValueTypeArray,
				Items: &fields.Field{
					ID:   "relationship",
					Path: "",
					Type: models.ValueTypeObject,
					Fields: fields.Fields{
						{ID: "_relationship_type", Path: "_relationship_type", Type: models.ValueTypeString},
						{ID: "_from_entity_type", Path: "_from_entity_type", Type: models.ValueTypeString},
						{ID: "_from_source_id", Path: "_from_source_id", Type: models.ValueTypeString},
						{ID: "_to_entity_type", Path: "_to_entity_type", Type: models.ValueTypeString},
						{ID: "_to_source_id", Path: "_to_source_id", Type: models.ValueTypeString},
					},
				},
			},
		},
		nil,
		links.Links{
			{Source: links.LinkDirection{Constant: "owns"}, Target: links.LinkDirection{FieldID: "_relationship_type"}},
			{Source: links.LinkDirection{Constant: "person"}, Target: links.LinkDirection{FieldID: "_from_entity_type"}},
			{Source: links.LinkDirection{FieldID: "src_owner_id"}, Target: links.LinkDirection{FieldID: "_from_source_id"}},
			{Source: links.LinkDirection{Constant: "device"}, Target: links.LinkDirection{FieldID: "_to_entity_type"}},
			{Source: links.LinkDirection{FieldID: "src_device_id"}, Target: links.LinkDirection{FieldID: "_to_source_id"}},
		},
	)

	res, err := def.ExecuteMapping(map[string]any{
		"response_body": map[string]any{
			"id": "u1",
			"owned_devices": map[string]any{
				"value": []any{
					map[string]any{"id": "d1"},
					map[string]any{"id": "d2"},
				},
			},
		},
	})
	assert.NoError(t, err)

	relsAny, ok := res.TargetRaw["relationships"].([]any)
	if !assert.True(t, ok) {
		return
	}
	if !assert.Len(t, relsAny, 2) {
		return
	}

	rel0 := relsAny[0].(map[string]any)
	rel1 := relsAny[1].(map[string]any)

	assert.Equal(t, "owns", rel0["_relationship_type"])
	assert.Equal(t, "person", rel0["_from_entity_type"])
	assert.Equal(t, "u1", rel0["_from_source_id"])
	assert.Equal(t, "device", rel0["_to_entity_type"])
	assert.Equal(t, "d1", rel0["_to_source_id"])

	assert.Equal(t, "d2", rel1["_to_source_id"])
}


