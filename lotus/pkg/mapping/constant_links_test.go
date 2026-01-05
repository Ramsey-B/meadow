package mapping

import (
	"testing"

	"github.com/Ramsey-B/lotus/pkg/fields"
	"github.com/Ramsey-B/lotus/pkg/links"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConstantToField(t *testing.T) {
	targetFields := fields.Fields{
		{
			ID:   "flag",
			Name: "Flag",
			Path: "flag",
			Type: models.ValueTypeBool,
		},
		{
			ID:   "integration",
			Name: "Integration",
			Path: "integration",
			Type: models.ValueTypeString,
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{Constant: false}, Target: links.LinkDirection{FieldID: "flag"}},
		{Priority: 1, Source: links.LinkDirection{Constant: "msgraph"}, Target: links.LinkDirection{FieldID: "integration"}},
	}

	m := NewMappingDefinition(
		MappingDefinitionFields{ID: "test-constant-field"},
		nil, // no source fields required
		targetFields,
		nil,
		linkList,
	)

	result, err := m.ExecuteMapping(map[string]any{})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, false, result.TargetRaw["flag"])
	assert.Equal(t, "msgraph", result.TargetRaw["integration"])
}

func TestConstantToStep(t *testing.T) {
	targetFields := fields.Fields{
		{
			ID:   "full_name",
			Name: "Full Name",
			Path: "full_name",
			Type: models.ValueTypeString,
		},
	}

	stepDefs := []models.StepDefinition{
		{
			ID:   "concat",
			Type: models.StepTypeTransformer,
			Action: models.ActionDefinition{
				Key: "text_concat",
				Arguments: map[string]any{
					"separator": " ",
				},
			},
		},
	}

	linkList := links.Links{
		{Priority: 0, Source: links.LinkDirection{Constant: "John"}, Target: links.LinkDirection{StepID: "concat"}},
		{Priority: 1, Source: links.LinkDirection{Constant: "Doe"}, Target: links.LinkDirection{StepID: "concat"}},
		{Priority: 2, Source: links.LinkDirection{StepID: "concat"}, Target: links.LinkDirection{FieldID: "full_name"}},
	}

	m := NewMappingDefinition(
		MappingDefinitionFields{ID: "test-constant-step"},
		nil,
		targetFields,
		stepDefs,
		linkList,
	)

	result, err := m.ExecuteMapping(map[string]any{})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "John Doe", result.TargetRaw["full_name"])
}
