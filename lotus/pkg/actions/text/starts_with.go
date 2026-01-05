package text

import (
	"strings"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextStartsWithRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  2,
	},
}

type TextStartsWithArguments struct {
	Substring       string `json:"substring" validate:"omitempty"`
	CaseInsensitive bool   `json:"case_insensitive" validate:"omitempty"`
}

func NewTextStartsWithAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextStartsWithRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	parsedArgs, err := utils.ParseArguments[TextStartsWithArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextStartsWithAction{
		key:             key,
		inputType:       inputTypes[0],
		caseInsensitive: parsedArgs.CaseInsensitive,
		substring:       parsedArgs.Substring,
		rules:           rules,
	}, nil
}

type TextStartsWithAction struct {
	key             string
	inputType       models.ActionValueType
	caseInsensitive bool
	substring       string
	rules           models.ActionInputRules
}

func (a *TextStartsWithAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextStartsWithAction) GetKey() string {
	return a.key
}

func (a *TextStartsWithAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextStartsWithAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeBool,
	}
}

func (a *TextStartsWithAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	substring := a.substring
	if len(inputs) == 2 {
		substring, err = utils.AnyToType[string](actionInputs["text"].Value[1])
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(1)
		}
	}

	if a.caseInsensitive {
		return strings.HasPrefix(strings.ToLower(str), strings.ToLower(substring)), nil
	}

	return strings.HasPrefix(str, substring), nil
}
