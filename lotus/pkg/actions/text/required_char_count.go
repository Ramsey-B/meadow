package text

import (
	"unicode"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var TextRequiredCharCountRules = models.ActionInputRules{
	"text": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type TextRequiredCharCountArguments struct {
	Special int            `json:"special" validate:"omitempty"`
	Upper   int            `json:"upper" validate:"omitempty"`
	Lower   int            `json:"lower" validate:"omitempty"`
	Number  int            `json:"number" validate:"omitempty"`
	Alpha   int            `json:"alpha" validate:"omitempty"`
	Chars   map[string]int `json:"chars" validate:"omitempty"`
}

func NewTextRequiredCharCountAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(TextRequiredCharCountRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	parsedArgs, err := utils.ParseArguments[TextRequiredCharCountArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &TextRequiredCharCountAction{
		key:        key,
		inputType:  models.ActionValueType{Type: models.ValueTypeString},
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type TextRequiredCharCountAction struct {
	key        string
	inputType  models.ActionValueType
	parsedArgs TextRequiredCharCountArguments
	rules      models.ActionInputRules
}

func (a *TextRequiredCharCountAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *TextRequiredCharCountAction) GetKey() string {
	return a.key
}

func (a *TextRequiredCharCountAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *TextRequiredCharCountAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *TextRequiredCharCountAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	str, err := utils.AnyToType[string](actionInputs["text"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	specialCount := 0
	upperCount := 0
	lowerCount := 0
	numberCount := 0
	alphaCount := 0
	punctuationCount := 0
	charCount := map[string]int{}

	for _, char := range str {
		if unicode.IsUpper(char) {
			upperCount++
		} else if unicode.IsLower(char) {
			lowerCount++
		} else if unicode.IsNumber(char) {
			numberCount++
		} else if unicode.IsSymbol(char) {
			specialCount++
		} else if unicode.IsPunct(char) {
			punctuationCount++
		} else if unicode.IsLetter(char) {
			alphaCount++
		} else {
			charCount[string(char)]++
		}
	}

	hasSpecial := specialCount >= a.parsedArgs.Special
	hasUpper := upperCount >= a.parsedArgs.Upper
	hasLower := lowerCount >= a.parsedArgs.Lower
	hasNumber := numberCount >= a.parsedArgs.Number
	hasAlpha := alphaCount >= a.parsedArgs.Alpha

	if !hasSpecial || !hasUpper || !hasLower || !hasNumber || !hasAlpha {
		return false, nil
	}

	for char, count := range charCount {
		parsedCount, ok := a.parsedArgs.Chars[char]
		if !ok {
			parsedCount = 0
		}

		if count < parsedCount {
			return false, nil
		}
	}

	return true, nil
}
