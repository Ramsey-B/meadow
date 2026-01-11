package any

import (
	"fmt"
	"strconv"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
)

var ToStringRules = models.ActionInputRules{
	"value": {
		Type: models.ValueTypeAny,
		Min:  1,
		Max:  1,
	},
}

type ToStringAction struct {
	key   string
	rules models.ActionInputRules
}

func NewToStringAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ToStringRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &ToStringAction{
		key:   key,
		rules: rules,
	}, nil
}

func (a *ToStringAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ToStringAction) GetKey() string {
	return a.key
}

func (a *ToStringAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeAny}
}

func (a *ToStringAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *ToStringAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	value := actionInputs["value"].Value[0]
	
	// Handle nil
	if value == nil {
		return "", nil
	}

	// Convert any type to string
	switch v := value.(type) {
	case string:
		return v, nil
	case bool:
		return strconv.FormatBool(v), nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		// Format without trailing zeros for whole numbers
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10), nil
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case float32:
		f64 := float64(v)
		if f64 == float64(int64(f64)) {
			return strconv.FormatInt(int64(f64), 10), nil
		}
		return strconv.FormatFloat(f64, 'f', -1, 32), nil
	default:
		// Fallback to fmt.Sprintf for complex types
		return fmt.Sprintf("%v", v), nil
	}
}


