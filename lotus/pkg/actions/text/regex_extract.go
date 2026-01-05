package text

import (
	"regexp"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextRegexExtractRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type TextRegexExtractArguments struct {
	Pattern string `json:"pattern" validate:"required"` // Regular expression pattern with capture groups
	Group   int    `json:"group" validate:"omitempty"`  // Which group to extract (0 = full match, 1+ = capture groups)
	All     bool   `json:"all" validate:"omitempty"`    // Return all matches
}

func NewTextRegexExtractAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextRegexExtractRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[TextRegexExtractArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	// Pre-compile the regex
	_, err = regexp.Compile(parsedArgs.Pattern)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextRegexExtractAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type TextRegexExtractAction struct {
	key        string
	parsedArgs TextRegexExtractArguments
	rules      models.ActionInputRules
}

func (a *TextRegexExtractAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextRegexExtractAction) GetKey() string {
	return a.key
}

func (a *TextRegexExtractAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *TextRegexExtractAction) GetOutputType() models.ActionValueType {
	if a.parsedArgs.All {
		return models.ActionValueType{Type: models.ValueTypeArray}
	}
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *TextRegexExtractAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	text, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	re, _ := regexp.Compile(a.parsedArgs.Pattern) // Already validated

	if a.parsedArgs.All {
		// Return all matches
		matches := re.FindAllStringSubmatch(text, -1)
		result := make([]any, 0, len(matches))
		for _, match := range matches {
			if a.parsedArgs.Group < len(match) {
				result = append(result, match[a.parsedArgs.Group])
			}
		}
		return result, nil
	}

	// Return first match
	match := re.FindStringSubmatch(text)
	if match == nil {
		return "", nil
	}

	if a.parsedArgs.Group < len(match) {
		return match[a.parsedArgs.Group], nil
	}

	return "", nil
}

