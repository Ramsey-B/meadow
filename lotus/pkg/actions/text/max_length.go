package text

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextMaxLengthRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
	"max_length": {
		Type: models.ValueTypeNumber,
		Min:  0,
		Max:  1,
	},
}

type TextMaxLengthArguments struct {
	MaxLength int `json:"max_length" validate:"omitempty,min=0"`
}

func NewTextMaxLengthAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextMaxLengthRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[TextMaxLengthArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextMaxLengthAction{
		key:       key,
		inputType: models.ActionValueType{Type: models.ValueTypeString},
		maxLength: parsedArgs.MaxLength,
		rules:     rules,
	}, nil
}

type TextMaxLengthAction struct {
	key       string
	inputType models.ActionValueType
	maxLength int
	rules     models.ActionInputRules
}

func (a *TextMaxLengthAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextMaxLengthAction) GetKey() string {
	return a.key
}

func (a *TextMaxLengthAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextMaxLengthAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeBool,
	}
}

func (a *TextMaxLengthAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	max := a.maxLength
	if len(inputs) > 1 {
		max, err = utils.AnyToType[int](inputs[1])
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(1)
		}
	}

	return len(str) <= max, nil
}
