package date

import (
	"time"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var DateFormatRules = models.ActionInputRules{
	"date": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type DateFormatArguments struct {
	Format   string `json:"format" validate:"required"`    // Output format
	Timezone string `json:"timezone" validate:"omitempty"` // Convert to timezone
}

func NewDateFormatAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(DateFormatRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[DateFormatArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &DateFormatAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type DateFormatAction struct {
	key        string
	parsedArgs DateFormatArguments
	rules      models.ActionInputRules
}

func (a *DateFormatAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *DateFormatAction) GetKey() string {
	return a.key
}

func (a *DateFormatAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *DateFormatAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *DateFormatAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	dateStr, err := utils.AnyToType[string](actionInputs["date"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	// Parse the date (try common formats)
	var parsedTime time.Time
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	
	for _, format := range formats {
		parsedTime, err = time.Parse(format, dateStr)
		if err == nil {
			break
		}
	}
	if parsedTime.IsZero() {
		return nil, errors.NewMappingError("unable to parse date: " + dateStr)
	}

	// Apply timezone if specified
	if a.parsedArgs.Timezone != "" {
		loc, err := time.LoadLocation(a.parsedArgs.Timezone)
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key)
		}
		parsedTime = parsedTime.In(loc)
	}

	// Format output
	outputFormat := resolveFormat(a.parsedArgs.Format)
	return parsedTime.Format(outputFormat), nil
}

