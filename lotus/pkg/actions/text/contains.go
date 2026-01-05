package text

import (
	"strings"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextContainsRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  2,
	},
}

type TextContainsArguments struct {
	Substring       string `json:"substring" validate:"omitempty"`
	CaseInsensitive bool   `json:"case_insensitive" validate:"omitempty"`
}

func NewTextContainsAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextContainsRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[TextContainsArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextContainsAction{
		key: key,
		inputType: models.ActionValueType{
			Type: models.ValueTypeString,
		},
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type TextContainsAction struct {
	key        string
	inputType  models.ActionValueType
	parsedArgs TextContainsArguments
	rules      models.ActionInputRules
}

func (a *TextContainsAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextContainsAction) GetKey() string {
	return a.key
}

func (a *TextContainsAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextContainsAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeBool,
	}
}

func (a *TextContainsAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	substring := a.parsedArgs.Substring
	if len(inputs) > 1 {
		substring, err = utils.AnyToType[string](inputs[1])
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(1)
		}
	}

	if a.parsedArgs.CaseInsensitive {
		substring = strings.ToLower(substring)
		str = strings.ToLower(str)
	}

	return strings.Contains(str, substring), nil
}
