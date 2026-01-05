package text

import (
	"strings"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextToLowerRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type TextToLowerAction struct {
	key       string
	inputType models.ActionValueType
	rules     models.ActionInputRules
}

func NewTextToLowerAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextToLowerRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextToLowerAction{
		key:       key,
		inputType: inputTypes[0],
		rules:     rules,
	}, nil
}

func (a *TextToLowerAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextToLowerAction) GetKey() string {
	return a.key
}

func (a *TextToLowerAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextToLowerAction) GetInputBType() models.ActionValueType {
	return a.inputType
}

func (a *TextToLowerAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeString,
	}
}

func (a *TextToLowerAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	return strings.ToLower(str), nil
}
