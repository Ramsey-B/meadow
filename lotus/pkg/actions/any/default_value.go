package any

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var DefaultValueRules = models.ActionInputRules{
	"value": {
		Type: models.ValueTypeAny,
		Min:  1,
		Max:  1,
	},
}

type DefaultValueArguments struct {
	Default any `json:"default" validate:"required"` // Default value if input is nil/empty
}

func NewDefaultValueAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(DefaultValueRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[DefaultValueArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &DefaultValueAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type DefaultValueAction struct {
	key        string
	parsedArgs DefaultValueArguments
	rules      models.ActionInputRules
}

func (a *DefaultValueAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *DefaultValueAction) GetKey() string {
	return a.key
}

func (a *DefaultValueAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeAny}
}

func (a *DefaultValueAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeAny}
}

func (a *DefaultValueAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	val := actionInputs["value"].Value[0]
	
	if isEmpty(val) {
		return a.parsedArgs.Default, nil
	}

	return val, nil
}

