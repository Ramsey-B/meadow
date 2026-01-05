package date

import (
	"time"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var DateNowRules = models.ActionInputRules{}

type DateNowArguments struct {
	Format   string `json:"format" validate:"omitempty"`   // Output format (Go time format)
	Timezone string `json:"timezone" validate:"omitempty"` // Timezone (e.g., "America/New_York")
}

func NewDateNowAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(DateNowRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[DateNowArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &DateNowAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type DateNowAction struct {
	key        string
	parsedArgs DateNowArguments
	rules      models.ActionInputRules
}

func (a *DateNowAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *DateNowAction) GetKey() string {
	return a.key
}

func (a *DateNowAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeAny}
}

func (a *DateNowAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *DateNowAction) Execute(inputs ...any) (any, error) {
	now := time.Now()

	// Apply timezone if specified
	if a.parsedArgs.Timezone != "" {
		loc, err := time.LoadLocation(a.parsedArgs.Timezone)
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key)
		}
		now = now.In(loc)
	}

	// Format output
	format := time.RFC3339
	if a.parsedArgs.Format != "" {
		format = a.parsedArgs.Format
	}

	return now.Format(format), nil
}

