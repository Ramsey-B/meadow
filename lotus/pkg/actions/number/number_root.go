package number

import (
	"math"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberRootRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  2,
	},
}

type NumberRootArguments struct {
	Root float64 `json:"root" validate:"omitempty,min=0"`
}

func NewNumberRootAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberRootRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	parsedArgs, err := utils.ParseArguments[NumberRootArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberRootAction{
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		key:   key,
		root:  parsedArgs.Root,
		rules: rules,
	}, nil
}

type NumberRootAction struct {
	inputType models.ActionValueType
	key       string
	root      float64
	rules     models.ActionInputRules
}

func (a *NumberRootAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberRootAction) GetKey() string {
	return a.key
}

func (a *NumberRootAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberRootAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberRootAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	val, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	root := a.root

	if len(inputs) == 2 {
		root, err = utils.AnyToType[float64](inputs[1])
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(1)
		}
	}

	if root == 0 {
		return nil, errors.NewMappingErrorf("action '%s' root value must be greater than 0, got %f", a.GetKey(), root)
	}

	result := math.Pow(val, 1/root)

	if math.IsNaN(result) {
		return nil, errors.NewMappingErrorf("action '%s' root value is not a number, got %f", a.GetKey(), result)
	}

	if math.IsInf(result, 0) {
		return nil, errors.NewMappingErrorf("action '%s' root value is infinite, got %f", a.GetKey(), result)
	}

	return result, nil
}
