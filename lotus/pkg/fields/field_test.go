package fields

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetField(t *testing.T) {
	fields := Fields{
		{
			ID: "1",
			Fields: Fields{
				{
					ID: "2",
				},
			},
		},
		{
			ID: "3",
			Fields: Fields{
				{
					ID: "4",
					Fields: Fields{
						{
							ID: "5",
						},
					},
				},
			},
		},
	}

	field, err := fields.GetField("1")
	assert.NoError(t, err)
	assert.Equal(t, "1", field.ID)

	field, err = fields.GetField("2")
	assert.NoError(t, err)
	assert.Equal(t, "2", field.ID)

	field, err = fields.GetField("3")
	assert.NoError(t, err)
	assert.Equal(t, "3", field.ID)

	field, err = fields.GetField("4")
	assert.NoError(t, err)
	assert.Equal(t, "4", field.ID)

	field, err = fields.GetField("5")
	assert.NoError(t, err)
	assert.Equal(t, "5", field.ID)

	// should return an error if the field is not found
	_, err = fields.GetField("6")
	assert.Error(t, err)
	assert.Equal(t, "field '6': field not found", err.Error())
}

func TestGetFieldPath(t *testing.T) {
	fields := Fields{
		{
			ID:   "1",
			Path: "foo",
			Fields: Fields{
				{
					ID:   "2",
					Path: "bar",
				},
			},
		},
	}

	path, err := fields.GetFieldPath("1")
	assert.NoError(t, err)
	assert.Equal(t, "foo", path)

	path, err = fields.GetFieldPath("2")
	assert.NoError(t, err)
	assert.Equal(t, "foo.bar", path)

	// should return an error if the field is not found
	_, err = fields.GetFieldPath("3")
	assert.Error(t, err)
	assert.Equal(t, "field '3': field not found", err.Error())
}
