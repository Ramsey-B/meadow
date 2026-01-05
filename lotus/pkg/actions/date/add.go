package date

import (
	"time"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var DateAddRules = models.ActionInputRules{
	"date": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type DateAddArguments struct {
	Years   int    `json:"years" validate:"omitempty"`
	Months  int    `json:"months" validate:"omitempty"`
	Days    int    `json:"days" validate:"omitempty"`
	Hours   int    `json:"hours" validate:"omitempty"`
	Minutes int    `json:"minutes" validate:"omitempty"`
	Seconds int    `json:"seconds" validate:"omitempty"`
	Format  string `json:"format" validate:"omitempty"` // Output format
}

func NewDateAddAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(DateAddRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[DateAddArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &DateAddAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type DateAddAction struct {
	key        string
	parsedArgs DateAddArguments
	rules      models.ActionInputRules
}

func (a *DateAddAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *DateAddAction) GetKey() string {
	return a.key
}

func (a *DateAddAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *DateAddAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *DateAddAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	dateStr, err := utils.AnyToType[string](actionInputs["date"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	parsedTime, err := parseDate(dateStr)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	// Add date components
	parsedTime = parsedTime.AddDate(a.parsedArgs.Years, a.parsedArgs.Months, a.parsedArgs.Days)
	
	// Add time components
	duration := time.Duration(a.parsedArgs.Hours)*time.Hour +
		time.Duration(a.parsedArgs.Minutes)*time.Minute +
		time.Duration(a.parsedArgs.Seconds)*time.Second
	parsedTime = parsedTime.Add(duration)

	// Format output
	outputFormat := time.RFC3339
	if a.parsedArgs.Format != "" {
		outputFormat = resolveFormat(a.parsedArgs.Format)
	}

	return parsedTime.Format(outputFormat), nil
}

