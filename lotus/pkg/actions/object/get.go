package object

import (
	"strings"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ObjectGetRules = models.ActionInputRules{
	"object": {
		Type: models.ValueTypeObject,
		Min:  1,
		Max:  1,
	},
}

type ObjectGetArguments struct {
	Path    string `json:"path" validate:"required"`    // Dot-notation path (e.g., "user.name")
	Default any    `json:"default" validate:"omitempty"` // Default if path not found
}

func NewObjectGetAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ObjectGetRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[ObjectGetArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &ObjectGetAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type ObjectGetAction struct {
	key        string
	parsedArgs ObjectGetArguments
	rules      models.ActionInputRules
}

func (a *ObjectGetAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ObjectGetAction) GetKey() string {
	return a.key
}

func (a *ObjectGetAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeObject}
}

func (a *ObjectGetAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeAny}
}

func (a *ObjectGetAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	obj, err := utils.AnyToType[map[string]any](actionInputs["object"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	result := getByPath(obj, a.parsedArgs.Path)
	if result == nil {
		return a.parsedArgs.Default, nil
	}

	return result, nil
}

func getByPath(obj map[string]any, path string) any {
	parts := strings.Split(path, ".")
	current := any(obj)

	for _, part := range parts {
		if current == nil {
			return nil
		}

		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		default:
			return nil
		}
	}

	return current
}

