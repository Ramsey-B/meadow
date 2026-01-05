package text

import (
	"strings"

	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextEqualsRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  -1,
	},
}

type TextEqualsArguments struct {
	// Value is the explicit value to compare against. If omitted, the first input becomes the expected value.
	Value *string `json:"value" validate:"omitempty"`
	// CompareTo is a legacy/alternate key used by older mappings (e.g. {"compare_to":"ACTIVE"}).
	CompareTo *string `json:"compare_to" validate:"omitempty"`
	CaseInsensitive bool    `json:"case_insensitive" validate:"omitempty"`
}

func NewTextEqualsAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextEqualsRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[TextEqualsArguments](args)
	if err != nil {
		return nil, err
	}

	expected := parsedArgs.Value
	if expected == nil {
		expected = parsedArgs.CompareTo
	}

	return &TextEqualsAction{
		key:             key,
		inputType:       models.ActionValueType{Type: models.ValueTypeString},
		str:             expected,
		caseInsensitive: parsedArgs.CaseInsensitive,
		rules:           rules,
	}, nil
}

type TextEqualsAction struct {
	key             string
	inputType       models.ActionValueType
	str             *string
	caseInsensitive bool
	rules           models.ActionInputRules
}

func (a *TextEqualsAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextEqualsAction) GetKey() string {
	return a.key
}

func (a *TextEqualsAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextEqualsAction) GetInputBType() models.ActionValueType {
	return a.inputType
}

func (a *TextEqualsAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeBool,
	}
}

func (a *TextEqualsAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	strs := ectolinq.Map(actionInputs["text"].Value, func(input any) string {
		str, err := utils.AnyToType[string](input)
		if err != nil {
			return ""
		}
		return str
	})

	if a.str != nil {
		strs = append([]string{*a.str}, strs...)
	}

	expected := ectolinq.First(strs)
	for _, str := range strs {
		if a.caseInsensitive {
			if strings.EqualFold(str, expected) {
				return true, nil
			}
		} else {
			if str == expected {
				return true, nil
			}
		}
	}

	return false, nil
}
