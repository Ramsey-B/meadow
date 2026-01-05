package any

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
)

var IsEmptyRules = models.ActionInputRules{
	"value": {
		Type: models.ValueTypeAny,
		Min:  1,
		Max:  1,
	},
}

func NewIsEmptyAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(IsEmptyRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	return &IsEmptyAction{
		key:   key,
		rules: rules,
	}, nil
}

type IsEmptyAction struct {
	key   string
	rules models.ActionInputRules
}

func (a *IsEmptyAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *IsEmptyAction) GetKey() string {
	return a.key
}

func (a *IsEmptyAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeAny}
}

func (a *IsEmptyAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeBool}
}

func (a *IsEmptyAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	val := actionInputs["value"].Value[0]
	return isEmpty(val), nil
}

