package number

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberAddRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  -1,
	},
}

type NumberAddArguments struct {
	Value float64 `json:"value" validate:"omitempty"`
}

func NewNumberAddAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberAddRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	parsedArgs, err := utils.ParseArguments[NumberAddArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberAddAction{
		key: key,
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		args:  parsedArgs,
		rules: rules,
	}, nil
}

type NumberAddAction struct {
	inputType models.ActionValueType
	key       string
	args      NumberAddArguments
	rules     models.ActionInputRules
}

func (a *NumberAddAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberAddAction) GetKey() string {
	return a.key
}

func (a *NumberAddAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberAddAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberAddAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	sum := a.args.Value // default value
	for i, input := range actionInputs["number"].Value {
		num, err := utils.AnyToType[float64](input)
		if err != nil {
			return nil, errors.NewMappingErrorf("number add action requires a number input, got %s", input).AddItemIndex(i)
		}
		sum += num
	}

	return sum, nil
}
