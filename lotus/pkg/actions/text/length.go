package text

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextLengthRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

func NewTextLengthAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextLengthRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	return &TextLengthAction{
		key:       key,
		inputType: models.ActionValueType{Type: models.ValueTypeString},
		rules:     rules,
	}, nil
}

type TextLengthAction struct {
	key       string
	inputType models.ActionValueType
	rules     models.ActionInputRules
}

func (a *TextLengthAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextLengthAction) GetKey() string {
	return a.key
}

func (a *TextLengthAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextLengthAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeNumber,
	}
}

func (a *TextLengthAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	return len(str), nil
}
