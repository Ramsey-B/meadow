package number

import (
	"math"

	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberModulusRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  2,
	},
}

type NumberModulusArguments struct {
	Denominator float64 `json:"denominator" validate:"omitempty"`
}

func NewNumberModulusAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberModulusRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	parsedArgs, err := utils.ParseArguments[NumberModulusArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberModulusAction{
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		key:         key,
		denominator: parsedArgs.Denominator,
		rules:       rules,
	}, nil
}

type NumberModulusAction struct {
	inputType   models.ActionValueType
	key         string
	denominator float64
	rules       models.ActionInputRules
}

func (a *NumberModulusAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberModulusAction) GetKey() string {
	return a.key
}

func (a *NumberModulusAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberModulusAction) GetInputBType() models.ActionValueType {
	return a.inputType
}

func (a *NumberModulusAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberModulusAction) Execute(inputs ...any) (any, error) {
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
	
	// Use argument denominator if provided, otherwise use second input
	denominator := a.denominator
	if len(numbers) > 1 {
		denominator = numbers[1]
	}

	if denominator == 0 {
		return nil, errors.NewMappingErrorf("cannot divide by zero, got %f", denominator)
	}

	return math.Mod(numerator, denominator), nil
}
