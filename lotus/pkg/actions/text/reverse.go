package text

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextReverseRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

func NewTextReverseAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextReverseRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextReverseAction{
		key:       key,
		inputType: inputTypes[0],
		rules:     rules,
	}, nil
}

type TextReverseAction struct {
	key       string
	inputType models.ActionValueType
	rules     models.ActionInputRules
}

func (a *TextReverseAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextReverseAction) GetKey() string {
	return a.key
}

func (a *TextReverseAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextReverseAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *TextReverseAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	runes := []rune(str)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes), nil
}
