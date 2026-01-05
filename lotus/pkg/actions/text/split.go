package text

import (
	"strings"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextSplitRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  2,
	},
}

type TextSplitArguments struct {
	Separator string `json:"separator" validate:"omitempty"`
}

func NewTextSplitAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextSplitRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	parsedArgs, err := utils.ParseArguments[TextSplitArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextSplitAction{
		key:       key,
		inputType: inputTypes[0],
		separator: parsedArgs.Separator,
		rules:     rules,
	}, nil
}

type TextSplitAction struct {
	key       string
	inputType models.ActionValueType
	separator string
	rules     models.ActionInputRules
}

func (a *TextSplitAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextSplitAction) GetKey() string {
	return a.key
}

func (a *TextSplitAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextSplitAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type:  models.ValueTypeArray,
		Items: models.ValueTypeString,
	}
}

func (a *TextSplitAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	separator := a.separator
	if len(actionInputs["text"].Value) == 2 {
		separator, err = utils.AnyToType[string](actionInputs["text"].Value[1])
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(1)
		}
	}

	// Convert []string to []any for consistent handling
	parts := strings.Split(str, separator)
	result := make([]any, len(parts))
	for i, p := range parts {
		result[i] = p
	}
	return result, nil
}
