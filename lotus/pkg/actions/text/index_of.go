package text

import (
	"strings"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextIndexOfRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  2,
	},
}

type TextIndexOfArguments struct {
	Substring       string `json:"substring" validate:"required"`
	CaseInsensitive bool   `json:"case_insensitive" validate:"omitempty"`
}

func NewTextIndexOfAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextIndexOfRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[TextIndexOfArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextIndexOfAction{
		key:             key,
		inputType:       models.ActionValueType{Type: models.ValueTypeString},
		substring:       parsedArgs.Substring,
		caseInsensitive: parsedArgs.CaseInsensitive,
		rules:           rules,
	}, nil
}

type TextIndexOfAction struct {
	key             string
	inputType       models.ActionValueType
	substring       string
	caseInsensitive bool
	rules           models.ActionInputRules
}

func (a *TextIndexOfAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextIndexOfAction) GetKey() string {
	return a.key
}

func (a *TextIndexOfAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextIndexOfAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeNumber,
	}
}

func (a *TextIndexOfAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddItemIndex(0).AddAction(a.key)
	}

	substring := a.substring
	if len(inputs) == 2 {
		var err error
		substring, err = utils.AnyToType[string](actionInputs["text"].Value[1])
		if err != nil {
			return nil, errors.WrapMappingError(err).AddItemIndex(1).AddAction(a.key)
		}
	}

	if a.caseInsensitive {
		return strings.Index(strings.ToLower(str), strings.ToLower(substring)), nil
	}

	return strings.Index(str, substring), nil
}
