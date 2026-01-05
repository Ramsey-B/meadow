package number

import (
	"strconv"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberParseRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type NumberParseArguments struct {
	Default float64 `json:"default" validate:"omitempty"` // Default value if parsing fails
}

func NewNumberParseAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberParseRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[NumberParseArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberParseAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type NumberParseAction struct {
	key        string
	parsedArgs NumberParseArguments
	rules      models.ActionInputRules
}

func (a *NumberParseAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberParseAction) GetKey() string {
	return a.key
}

func (a *NumberParseAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *NumberParseAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeNumber}
}

func (a *NumberParseAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	text, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return a.parsedArgs.Default, nil
	}

	result, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return a.parsedArgs.Default, nil
	}

	return result, nil
}

