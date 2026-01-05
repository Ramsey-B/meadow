package text

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextSubstringRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type TextSubstringArguments struct {
	Start int `json:"start" validate:"omitempty"` // Start index (0-based, can be negative)
	End   int `json:"end" validate:"omitempty"`   // End index (exclusive, can be negative or 0 for end)
}

func NewTextSubstringAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextSubstringRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[TextSubstringArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextSubstringAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type TextSubstringAction struct {
	key        string
	parsedArgs TextSubstringArguments
	rules      models.ActionInputRules
}

func (a *TextSubstringAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextSubstringAction) GetKey() string {
	return a.key
}

func (a *TextSubstringAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *TextSubstringAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *TextSubstringAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	text, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	length := len(text)
	if length == 0 {
		return "", nil
	}

	// Handle negative indices (from end)
	start := a.parsedArgs.Start
	if start < 0 {
		start = length + start
	}
	if start < 0 {
		start = 0
	}
	if start >= length {
		return "", nil
	}

	end := a.parsedArgs.End
	if end == 0 {
		end = length
	} else if end < 0 {
		end = length + end
	}
	if end > length {
		end = length
	}
	if end <= start {
		return "", nil
	}

	return text[start:end], nil
}

