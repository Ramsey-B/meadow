package number

import (
	"math"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberSquareRootRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  1,
	},
}

func NewNumberSquareRootAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberSquareRootRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberSquareRootAction{
		key: key,
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		rules: rules,
	}, nil
}

type NumberSquareRootAction struct {
	key       string
	inputType models.ActionValueType
	rules     models.ActionInputRules
}

func (a *NumberSquareRootAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberSquareRootAction) GetKey() string {
	return a.key
}

func (a *NumberSquareRootAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberSquareRootAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberSquareRootAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	num, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	if num < 0 {
		return nil, errors.NewMappingErrorf("number square root action requires a number greater than or equal to 0, got %f", num)
	}

	return math.Sqrt(num), nil
}
