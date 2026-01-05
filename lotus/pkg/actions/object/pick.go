package object

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ObjectPickRules = models.ActionInputRules{
	"object": {
		Type: models.ValueTypeObject,
		Min:  1,
		Max:  1,
	},
}

type ObjectPickArguments struct {
	Keys []string `json:"keys" validate:"required"` // Keys to pick from object
}

func NewObjectPickAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ObjectPickRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[ObjectPickArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &ObjectPickAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type ObjectPickAction struct {
	key        string
	parsedArgs ObjectPickArguments
	rules      models.ActionInputRules
}

func (a *ObjectPickAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ObjectPickAction) GetKey() string {
	return a.key
}

func (a *ObjectPickAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeObject}
}

func (a *ObjectPickAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeObject}
}

func (a *ObjectPickAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	obj, err := utils.AnyToType[map[string]any](actionInputs["object"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	result := make(map[string]any)
	for _, key := range a.parsedArgs.Keys {
		if val, ok := obj[key]; ok {
			result[key] = val
		}
	}

	return result, nil
}

