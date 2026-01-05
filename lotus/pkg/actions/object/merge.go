package object

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ObjectMergeRules = models.ActionInputRules{
	"object": {
		Type: models.ValueTypeObject,
		Min:  1,
		Max:  -1, // Unlimited - merge multiple objects
	},
}

type ObjectMergeArguments struct {
	Deep bool `json:"deep" validate:"omitempty"` // Deep merge nested objects
}

func NewObjectMergeAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ObjectMergeRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[ObjectMergeArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &ObjectMergeAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type ObjectMergeAction struct {
	key        string
	parsedArgs ObjectMergeArguments
	rules      models.ActionInputRules
}

func (a *ObjectMergeAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ObjectMergeAction) GetKey() string {
	return a.key
}

func (a *ObjectMergeAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeObject}
}

func (a *ObjectMergeAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeObject}
}

func (a *ObjectMergeAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	result := make(map[string]any)
	
	for _, input := range actionInputs["object"].Value {
		obj, err := utils.AnyToType[map[string]any](input)
		if err != nil {
			continue // Skip non-object inputs
		}

		if a.parsedArgs.Deep {
			deepMerge(result, obj)
		} else {
			shallowMerge(result, obj)
		}
	}

	return result, nil
}

func shallowMerge(target, source map[string]any) {
	for key, val := range source {
		target[key] = val
	}
}

func deepMerge(target, source map[string]any) {
	for key, sourceVal := range source {
		if targetVal, exists := target[key]; exists {
			// Both are maps - merge recursively
			targetMap, targetIsMap := targetVal.(map[string]any)
			sourceMap, sourceIsMap := sourceVal.(map[string]any)
			
			if targetIsMap && sourceIsMap {
				deepMerge(targetMap, sourceMap)
				continue
			}
		}
		// Otherwise just overwrite
		target[key] = sourceVal
	}
}

