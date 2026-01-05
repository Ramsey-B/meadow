package text

import (
	"strconv"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextToNumberRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type TextToNumberAction struct {
	key       string
	inputType models.ActionValueType
	rules     models.ActionInputRules
}

func NewTextToNumberAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextToNumberRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextToNumberAction{
		key:       key,
		inputType: inputTypes[0],
		rules:     rules,
	}, nil
}

func (a *TextToNumberAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextToNumberAction) GetKey() string {
	return a.key
}

func (a *TextToNumberAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextToNumberAction) GetInputBType() models.ActionValueType {
	return a.inputType
}

func (a *TextToNumberAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeNumber,
	}
}

func (a *TextToNumberAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	return strconv.ParseFloat(str, 64)
}
