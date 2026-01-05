package text

import (
	"strings"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextToArrayRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type TextToArrayAction struct {
	key       string
	inputType models.ActionValueType
	rules     models.ActionInputRules
}

func NewTextToArrayAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextToArrayRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextToArrayAction{
		key:       key,
		inputType: inputTypes[0],
		rules:     rules,
	}, nil
}

func (a *TextToArrayAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextToArrayAction) GetKey() string {
	return a.key
}

func (a *TextToArrayAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextToArrayAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type:  models.ValueTypeArray,
		Items: models.ValueTypeString,
	}
}

func (a *TextToArrayAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	return strings.Split(str, ""), nil
}
