package text

import (
	"regexp"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextRegexReplaceRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type TextRegexReplaceArguments struct {
	Pattern     string `json:"pattern" validate:"required"`     // Regular expression pattern
	Replacement string `json:"replacement" validate:"required"` // Replacement string (can use $1, $2 for groups)
}

func NewTextRegexReplaceAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextRegexReplaceRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[TextRegexReplaceArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	// Pre-compile the regex
	_, err = regexp.Compile(parsedArgs.Pattern)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextRegexReplaceAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type TextRegexReplaceAction struct {
	key        string
	parsedArgs TextRegexReplaceArguments
	rules      models.ActionInputRules
}

func (a *TextRegexReplaceAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextRegexReplaceAction) GetKey() string {
	return a.key
}

func (a *TextRegexReplaceAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *TextRegexReplaceAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *TextRegexReplaceAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	text, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	re, _ := regexp.Compile(a.parsedArgs.Pattern) // Already validated
	return re.ReplaceAllString(text, a.parsedArgs.Replacement), nil
}

