package text

import (
	"strings"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextToUpperRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type TextToUpperAction struct {
	key       string
	inputType models.ActionValueType
	rules     models.ActionInputRules
}

func NewTextToUpperAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextToUpperRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	return &TextToUpperAction{
		key:       key,
		inputType: inputTypes[0],
		rules:     rules,
	}, nil
}

func (a *TextToUpperAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextToUpperAction) GetKey() string {
	return a.key
}

func (a *TextToUpperAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextToUpperAction) GetInputBType() models.ActionValueType {
	return a.inputType
}

func (a *TextToUpperAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeString,
	}
}

func (a *TextToUpperAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	return strings.ToUpper(str), nil
}
