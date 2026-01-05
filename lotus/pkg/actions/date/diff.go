package date

import (
	"time"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var DateDiffRules = models.ActionInputRules{
	"date": {
		Type: models.ValueTypeString,
		Min:  2,
		Max:  2,
	},
}

type DateDiffArguments struct {
	Unit string `json:"unit" validate:"omitempty"` // seconds, minutes, hours, days (default: seconds)
}

func NewDateDiffAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(DateDiffRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[DateDiffArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &DateDiffAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type DateDiffAction struct {
	key        string
	parsedArgs DateDiffArguments
	rules      models.ActionInputRules
}

func (a *DateDiffAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *DateDiffAction) GetKey() string {
	return a.key
}

func (a *DateDiffAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *DateDiffAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeNumber}
}

func (a *DateDiffAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	dateVals := actionInputs["date"].Value
	
	date1Str, err := utils.AnyToType[string](dateVals[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}
	
	date2Str, err := utils.AnyToType[string](dateVals[1])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	// Parse both dates
	date1, err := parseDate(date1Str)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}
	
	date2, err := parseDate(date2Str)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	// Calculate difference
	diff := date2.Sub(date1)

	// Convert to requested unit
	unit := a.parsedArgs.Unit
	if unit == "" {
		unit = "seconds"
	}

	var result float64
	switch unit {
	case "seconds":
		result = diff.Seconds()
	case "minutes":
		result = diff.Minutes()
	case "hours":
		result = diff.Hours()
	case "days":
		result = diff.Hours() / 24
	case "milliseconds":
		result = float64(diff.Milliseconds())
	default:
		result = diff.Seconds()
	}

	return result, nil
}

func parseDate(dateStr string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	
	var parsedTime time.Time
	var err error
	for _, format := range formats {
		parsedTime, err = time.Parse(format, dateStr)
		if err == nil {
			return parsedTime, nil
		}
	}
	return time.Time{}, errors.NewMappingError("unable to parse date: " + dateStr)
}

