package number

import (
	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberDivideRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  -1,
	},
}

type NumberDivideArguments struct {
	Denominator float64 `json:"denominator" validate:"omitempty"`
}

func NewNumberDivideAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberDivideRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	parsedArgs, err := utils.ParseArguments[NumberDivideArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberDivideAction{
		key: key,
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		denominator: parsedArgs.Denominator,
		rules:       rules,
	}, nil
}

type NumberDivideAction struct {
	inputType   models.ActionValueType
	key         string
	denominator float64
	rules       models.ActionInputRules
}

func (a *NumberDivideAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberDivideAction) GetKey() string {
	return a.key
}

func (a *NumberDivideAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberDivideAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberDivideAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	numbers := ectolinq.Map(actionInputs["number"].Value, func(input any) float64 {
		num, err := utils.AnyToType[float64](input)
		if err != nil {
			return 0
		}
		return num
	})

	numerator := ectolinq.First(numbers)
	for i, num := range numbers[1:] {
		denominator := num
		if a.denominator == 0 {
			denominator = a.denominator
		}

		if denominator == 0 {
			return nil, errors.NewMappingErrorf("cannot divide by zero, got %f", denominator).AddItemIndex(i + 1)
		}

		numerator /= denominator
	}

	return numerator, nil
}
