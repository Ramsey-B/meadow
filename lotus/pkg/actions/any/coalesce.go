package any

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var CoalesceRules = models.ActionInputRules{
	"value": {
		Type: models.ValueTypeAny,
		Min:  1,
		Max:  -1, // Unlimited
	},
}

type CoalesceArguments struct {
	Default any `json:"default" validate:"omitempty"` // Default value if all inputs are nil/empty
}

func NewCoalesceAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(CoalesceRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[CoalesceArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &CoalesceAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type CoalesceAction struct {
	key        string
	parsedArgs CoalesceArguments
	rules      models.ActionInputRules
}

func (a *CoalesceAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *CoalesceAction) GetKey() string {
	return a.key
}

func (a *CoalesceAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeAny}
}

func (a *CoalesceAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeAny}
}

func (a *CoalesceAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	values := actionInputs["value"].Value
	
	for _, val := range values {
		if !isEmpty(val) {
			return val, nil
		}
	}

	// All values were nil/empty, return default
	return a.parsedArgs.Default, nil
}

func isEmpty(val any) bool {
	if val == nil {
		return true
	}
	
	switch v := val.(type) {
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	}
	
	return false
}

