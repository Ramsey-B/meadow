package text

import (
	"regexp"
	"strings"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextReplaceRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  3,
	},
}

type TextReplaceArguments struct {
	Old string `json:"old" validate:"omitempty"`
	New string `json:"new" validate:"omitempty"`
}

func NewTextReplaceAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextReplaceRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[TextReplaceArguments](args)
	if err != nil {
		return nil, err
	}

	return &TextReplaceAction{
		key:       key,
		inputType: models.ActionValueType{Type: models.ValueTypeString},
		old:       parsedArgs.Old,
		new:       parsedArgs.New,
		rules:     rules,
	}, nil
}

type TextReplaceAction struct {
	key       string
	inputType models.ActionValueType
	old       string
	new       string
	rules     models.ActionInputRules
}

func (a *TextReplaceAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextReplaceAction) GetKey() string {
	return a.key
}

func (a *TextReplaceAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextReplaceAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *TextReplaceAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	old := a.old
	new := a.new
	if len(inputs) == 2 {
		old, err = utils.AnyToType[string](actionInputs["text"].Value[1])
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(1)
		}
	}

	if len(inputs) == 3 {
		new, err = utils.AnyToType[string](actionInputs["text"].Value[2])
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(2)
		}
	}

	re, err := regexp.Compile(old)
	if err == nil {
		// if old is regex, use regex.ReplaceAllString()
		return re.ReplaceAllString(str, a.new), nil
	}

	// if old is not regex, use strings.ReplaceAll()
	return strings.ReplaceAll(str, old, new), nil
}
