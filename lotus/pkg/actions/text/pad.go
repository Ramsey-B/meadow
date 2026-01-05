package text

import (
	"strings"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextPadRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type TextPadArguments struct {
	Length int    `json:"length" validate:"required"` // Target length
	Char   string `json:"char" validate:"omitempty"`  // Padding character (default: space)
	Side   string `json:"side" validate:"omitempty"`  // "left", "right", "both" (default: right)
}

func NewTextPadAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextPadRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[TextPadArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextPadAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type TextPadAction struct {
	key        string
	parsedArgs TextPadArguments
	rules      models.ActionInputRules
}

func (a *TextPadAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextPadAction) GetKey() string {
	return a.key
}

func (a *TextPadAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *TextPadAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *TextPadAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	text, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	if len(text) >= a.parsedArgs.Length {
		return text, nil
	}

	padChar := " "
	if a.parsedArgs.Char != "" {
		padChar = a.parsedArgs.Char
	}

	side := a.parsedArgs.Side
	if side == "" {
		side = "right"
	}

	padLen := a.parsedArgs.Length - len(text)
	padding := strings.Repeat(padChar, padLen)

	switch side {
	case "left":
		return padding + text, nil
	case "both":
		leftPad := padLen / 2
		rightPad := padLen - leftPad
		return strings.Repeat(padChar, leftPad) + text + strings.Repeat(padChar, rightPad), nil
	default: // right
		return text + padding, nil
	}
}

