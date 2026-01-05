package text

import (
	"strings"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextTrimRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type TextTrimArguments struct {
	Chars string `json:"chars" validate:"omitempty"` // Characters to trim (default: whitespace)
	Side  string `json:"side" validate:"omitempty"`  // "left", "right", "both" (default: both)
}

func NewTextTrimAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextTrimRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[TextTrimArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextTrimAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type TextTrimAction struct {
	key        string
	parsedArgs TextTrimArguments
	rules      models.ActionInputRules
}

func (a *TextTrimAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextTrimAction) GetKey() string {
	return a.key
}

func (a *TextTrimAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *TextTrimAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *TextTrimAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	text, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	side := a.parsedArgs.Side
	if side == "" {
		side = "both"
	}

	if a.parsedArgs.Chars == "" {
		// Trim whitespace
		switch side {
		case "left":
			return strings.TrimLeft(text, " \t\n\r"), nil
		case "right":
			return strings.TrimRight(text, " \t\n\r"), nil
		default:
			return strings.TrimSpace(text), nil
		}
	}

	// Trim specific characters
	switch side {
	case "left":
		return strings.TrimLeft(text, a.parsedArgs.Chars), nil
	case "right":
		return strings.TrimRight(text, a.parsedArgs.Chars), nil
	default:
		return strings.Trim(text, a.parsedArgs.Chars), nil
	}
}

