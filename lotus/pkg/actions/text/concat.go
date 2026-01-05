package text

import (
	"strings"

	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextConcatRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  -1,
	},
}

type TextConcatArguments struct {
	Value     string `json:"value" validate:"omitempty"`
	Separator string `json:"separator" validate:"omitempty"`
}

func NewTextConcatAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextConcatRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[TextConcatArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextConcatAction{
		key: key,
		inputType: models.ActionValueType{
			Type: models.ValueTypeString,
		},
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type TextConcatAction struct {
	key        string
	inputType  models.ActionValueType
	parsedArgs TextConcatArguments
	rules      models.ActionInputRules
}

func (a *TextConcatAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextConcatAction) GetKey() string {
	return a.key
}

func (a *TextConcatAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextConcatAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *TextConcatAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	items := ectolinq.Map(actionInputs["text"].Value, func(input any) string {
		str, err := utils.AnyToType[string](input)
		if err != nil {
			return ""
		}
		return str
	})

	if a.parsedArgs.Value != "" {
		items = append(items, a.parsedArgs.Value)
	}

	return strings.Join(items, a.parsedArgs.Separator), nil
}
