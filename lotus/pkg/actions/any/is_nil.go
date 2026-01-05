package any

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
)

var IsNilRules = models.ActionInputRules{
	"value": {
		Type: models.ValueTypeAny,
		Min:  1,
		Max:  1,
	},
}

func NewIsNilAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(IsNilRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	return &IsNilAction{
		key:   key,
		rules: rules,
	}, nil
}

type IsNilAction struct {
	key   string
	rules models.ActionInputRules
}

func (a *IsNilAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *IsNilAction) GetKey() string {
	return a.key
}

func (a *IsNilAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeAny}
}

func (a *IsNilAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeBool}
}

func (a *IsNilAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	val := actionInputs["value"].Value[0]
	return val == nil, nil
}

