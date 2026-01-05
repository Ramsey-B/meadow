package text

import (
	"regexp"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextRegexMatchRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type TextRegexMatchArguments struct {
	Pattern string `json:"pattern" validate:"required"` // Regular expression pattern
}

func NewTextRegexMatchAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextRegexMatchRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[TextRegexMatchArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	// Pre-compile the regex
	_, err = regexp.Compile(parsedArgs.Pattern)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextRegexMatchAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type TextRegexMatchAction struct {
	key        string
	parsedArgs TextRegexMatchArguments
	rules      models.ActionInputRules
}

func (a *TextRegexMatchAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextRegexMatchAction) GetKey() string {
	return a.key
}

func (a *TextRegexMatchAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *TextRegexMatchAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeBool}
}

func (a *TextRegexMatchAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	text, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	re, _ := regexp.Compile(a.parsedArgs.Pattern) // Already validated
	return re.MatchString(text), nil
}

