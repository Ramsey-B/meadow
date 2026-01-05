package text

import (
	"strconv"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextToBoolRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type TextToBoolAction struct {
	key       string
	inputType models.ActionValueType
	rules     models.ActionInputRules
}

func NewTextToBoolAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextToBoolRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	return &TextToBoolAction{
		key:       key,
		inputType: inputTypes[0],
		rules:     rules,
	}, nil
}

func (a *TextToBoolAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextToBoolAction) GetKey() string {
	return a.key
}

func (a *TextToBoolAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextToBoolAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeBool,
	}
}

func (a *TextToBoolAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	return strconv.ParseBool(str)
}
