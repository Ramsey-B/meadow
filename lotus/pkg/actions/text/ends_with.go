package text

import (
	"strings"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextEndsWithRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  2,
	},
}

type TextEndsWithArguments struct {
	Substring       string `json:"substring" validate:"omitempty"`
	CaseInsensitive bool   `json:"case_insensitive" validate:"omitempty"`
}

func NewTextEndsWithAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextEndsWithRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[TextEndsWithArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextEndsWithAction{
		key: key,
		inputType: models.ActionValueType{
			Type: models.ValueTypeString,
		},
		substring:     parsedArgs.Substring,
		caseSensitive: parsedArgs.CaseInsensitive,
		rules:         rules,
	}, nil
}

type TextEndsWithAction struct {
	key           string
	inputType     models.ActionValueType
	substring     string
	caseSensitive bool
	rules         models.ActionInputRules
}

func (a *TextEndsWithAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextEndsWithAction) GetKey() string {
	return a.key
}

func (a *TextEndsWithAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextEndsWithAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeBool,
	}
}

func (a *TextEndsWithAction) Execute(inputs ...any) (any, error) {
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
		var err error
		substring, err = utils.AnyToType[string](inputs[1])
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(1)
		}
	}

	if a.caseSensitive {
		substring = strings.ToLower(substring)
		str = strings.ToLower(str)
	}

	return strings.HasSuffix(str, substring), nil
}
