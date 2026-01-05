package object

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ObjectValuesRules = models.ActionInputRules{
	"object": {
		Type: models.ValueTypeObject,
		Min:  1,
		Max:  1,
	},
}

func NewObjectValuesAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ObjectValuesRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	return &ObjectValuesAction{
		key:   key,
		rules: rules,
	}, nil
}

type ObjectValuesAction struct {
	key   string
	rules models.ActionInputRules
}

func (a *ObjectValuesAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ObjectValuesAction) GetKey() string {
	return a.key
}

func (a *ObjectValuesAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeObject}
}

func (a *ObjectValuesAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeArray}
}

func (a *ObjectValuesAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	obj, err := utils.AnyToType[map[string]any](actionInputs["object"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	values := make([]any, 0, len(obj))
	for _, val := range obj {
		values = append(values, val)
	}

	return values, nil
}

