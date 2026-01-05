package any

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var IfElseRules = models.ActionInputRules{
	"condition": {
		Type: models.ValueTypeBool,
		Min:  1,
		Max:  1,
	},
	"then": {
		Type: models.ValueTypeAny,
		Min:  1,
		Max:  1,
	},
}

type IfElseArguments struct {
	Else any `json:"else" validate:"omitempty"` // Value if condition is false
}

func NewIfElseAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(IfElseRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[IfElseArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &IfElseAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type IfElseAction struct {
	key        string
	parsedArgs IfElseArguments
	rules      models.ActionInputRules
}

func (a *IfElseAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *IfElseAction) GetKey() string {
	return a.key
}

func (a *IfElseAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeAny}
}

func (a *IfElseAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeAny}
}

func (a *IfElseAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	condition, err := utils.AnyToType[bool](actionInputs["condition"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	thenValue := actionInputs["then"].Value[0]

	if condition {
		return thenValue, nil
	}

	return a.parsedArgs.Else, nil
}

