package object

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ObjectOmitRules = models.ActionInputRules{
	"object": {
		Type: models.ValueTypeObject,
		Min:  1,
		Max:  1,
	},
}

type ObjectOmitArguments struct {
	Keys []string `json:"keys" validate:"required"` // Keys to omit from object
}

func NewObjectOmitAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ObjectOmitRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[ObjectOmitArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &ObjectOmitAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type ObjectOmitAction struct {
	key        string
	parsedArgs ObjectOmitArguments
	rules      models.ActionInputRules
}

func (a *ObjectOmitAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ObjectOmitAction) GetKey() string {
	return a.key
}

func (a *ObjectOmitAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeObject}
}

func (a *ObjectOmitAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeObject}
}

func (a *ObjectOmitAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	obj, err := utils.AnyToType[map[string]any](actionInputs["object"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	// Build set of keys to omit
	omitSet := make(map[string]bool)
	for _, key := range a.parsedArgs.Keys {
		omitSet[key] = true
	}

	result := make(map[string]any)
	for key, val := range obj {
		if !omitSet[key] {
			result[key] = val
		}
	}

	return result, nil
}

