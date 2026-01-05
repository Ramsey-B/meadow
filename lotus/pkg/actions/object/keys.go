package object

import (
	"sort"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ObjectKeysRules = models.ActionInputRules{
	"object": {
		Type: models.ValueTypeObject,
		Min:  1,
		Max:  1,
	},
}

type ObjectKeysArguments struct {
	Sorted bool `json:"sorted" validate:"omitempty"` // Sort keys alphabetically
}

func NewObjectKeysAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ObjectKeysRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[ObjectKeysArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &ObjectKeysAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type ObjectKeysAction struct {
	key        string
	parsedArgs ObjectKeysArguments
	rules      models.ActionInputRules
}

func (a *ObjectKeysAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ObjectKeysAction) GetKey() string {
	return a.key
}

func (a *ObjectKeysAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeObject}
}

func (a *ObjectKeysAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeArray}
}

func (a *ObjectKeysAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	obj, err := utils.AnyToType[map[string]any](actionInputs["object"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}

	if a.parsedArgs.Sorted {
		sort.Strings(keys)
	}

	// Convert to []any for consistency
	result := make([]any, len(keys))
	for i, k := range keys {
		result[i] = k
	}

	return result, nil
}

