package any

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ToStringRules = models.ActionInputRules{
	"value": {
		Type: models.ValueTypeAny,
		Min:  1,
		Max:  1,
	},
}

type ToStringAction struct {
	key   string
	rules models.ActionInputRules
}

func NewToStringAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ToStringRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &ToStringAction{
		key:   key,
		rules: rules,
	}, nil
}

func (a *ToStringAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ToStringAction) GetKey() string {
	return a.key
}

func (a *ToStringAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeAny}
}

func (a *ToStringAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *ToStringAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["value"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}
	return str, nil
}


