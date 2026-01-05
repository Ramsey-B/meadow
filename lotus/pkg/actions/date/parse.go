package date

import (
	"time"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var DateParseRules = models.ActionInputRules{
	"date": {
		Type: models.ValueTypeString,
		Min:  1,
		Max:  1,
	},
}

type DateParseArguments struct {
	InputFormat  string `json:"input_format" validate:"omitempty"`  // Expected input format
	OutputFormat string `json:"output_format" validate:"omitempty"` // Desired output format
	Timezone     string `json:"timezone" validate:"omitempty"`      // Convert to timezone
}

// Common date format aliases
var formatAliases = map[string]string{
	"iso8601":   time.RFC3339,
	"rfc3339":   time.RFC3339,
	"rfc822":    time.RFC822,
	"rfc850":    time.RFC850,
	"rfc1123":   time.RFC1123,
	"unix":      time.UnixDate,
	"date":      "2006-01-02",
	"datetime":  "2006-01-02 15:04:05",
	"time":      "15:04:05",
	"timestamp": "2006-01-02T15:04:05Z07:00",
}

func resolveFormat(format string) string {
	if alias, ok := formatAliases[format]; ok {
		return alias
	}
	return format
}

func NewDateParseAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(DateParseRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[DateParseArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &DateParseAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type DateParseAction struct {
	key        string
	parsedArgs DateParseArguments
	rules      models.ActionInputRules
}

func (a *DateParseAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *DateParseAction) GetKey() string {
	return a.key
}

func (a *DateParseAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *DateParseAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeString}
}

func (a *DateParseAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	dateStr, err := utils.AnyToType[string](actionInputs["date"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	// Try to parse with input format or common formats
	var parsedTime time.Time
	if a.parsedArgs.InputFormat != "" {
		format := resolveFormat(a.parsedArgs.InputFormat)
		parsedTime, err = time.Parse(format, dateStr)
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key)
		}
	} else {
		// Try common formats
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
			"2006-01-02",
			time.RFC1123,
			time.RFC822,
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
	outputFormat := time.RFC3339
	if a.parsedArgs.OutputFormat != "" {
		outputFormat = resolveFormat(a.parsedArgs.OutputFormat)
	}

	return parsedTime.Format(outputFormat), nil
}

