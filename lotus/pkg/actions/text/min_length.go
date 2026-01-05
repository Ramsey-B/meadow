package text

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextMinLengthRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
	"min_length": {
		Type: models.ValueTypeNumber,
		Min:  0,
		Max:  1,
	},
}

type TextMinLengthArguments struct {
	MinLength int `json:"min_length" validate:"omitempty,min=0"`
}

func NewTextMinLengthAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextMinLengthRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[TextMinLengthArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextMinLengthAction{
		key:       key,
		inputType: models.ActionValueType{Type: models.ValueTypeString},
		minLength: parsedArgs.MinLength,
		rules:     rules,
	}, nil
}

type TextMinLengthAction struct {
	key       string
	inputType models.ActionValueType
	minLength int
	rules     models.ActionInputRules
}

func (a *TextMinLengthAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextMinLengthAction) GetKey() string {
	return a.key
}

func (a *TextMinLengthAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextMinLengthAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeBool,
	}
}

func (a *TextMinLengthAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	min := a.minLength

	if len(inputs) > 1 {
		min, err = utils.AnyToType[int](inputs[1])
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(1)
		}
	}

	return len(str) >= min, nil
}
