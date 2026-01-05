package errors

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Gobusters/ectoerror/httperror"
)

type MappingError struct {
	Link      string
	Field     string
	Step      string
	Action    string
	itemIndex *int
	Message   string
}

func NewMappingError(msg string) *MappingError {
	return &MappingError{
		Message: msg,
		Step:    "",
		Action:  "",
		Field:   "",
		Link:    "",
	}
}

func WrapMappingError(e error) *MappingError {
	if e == nil {
		return nil
	}

	if mappingError, ok := e.(*MappingError); ok {
		return mappingError
	}

	return &MappingError{
		Message: e.Error(),
		Step:    "",
		Action:  "",
		Field:   "",
	}
}

// NewMappingErrorf creates a new MappingError with a formatted message
func NewMappingErrorf(format string, args ...any) *MappingError {
	// Handle error wrapping directive %w
	// If one of the args is an error and format contains %w,
	// extract the error message and replace %w with %v
	for i, arg := range args {
		if err, ok := arg.(error); ok && strings.Contains(format, "%w") {
			// Replace %w with %v and use error message
			format = strings.Replace(format, "%w", "%v", 1)
			args[i] = err.Error()
		}
	}

	return &MappingError{
		Message: fmt.Sprintf(format, args...),
		Step:    "",
		Action:  "",
		Field:   "",
	}
}

func (e *MappingError) Error() string {
	path := []string{}
	if e.Field != "" {
		path = append(path, fmt.Sprintf("field '%s'", e.Field))
	}
	if e.Step != "" {
		path = append(path, fmt.Sprintf("step '%s'", e.Step))
	}
	if e.Action != "" {
		path = append(path, fmt.Sprintf("action '%s'", e.Action))
	}

	if len(path) == 0 {
		return e.Message
	}

	return strings.Join(path, " -> ") + ": " + e.Message
}

func (e *MappingError) AddField(fieldID string) *MappingError {
	e.Field = fieldID
	return e
}

func (e *MappingError) AddStep(stepID string) *MappingError {
	e.Step = stepID
	return e
}

func (e *MappingError) AddAction(actionKey string) *MappingError {
	e.Action = actionKey
	return e
}

func (e *MappingError) AddItemIndex(itemIndex int) *MappingError {
	e.itemIndex = &itemIndex
	return e
}

func (e *MappingError) AddLink(linkID string) *MappingError {
	e.Link = linkID
	return e
}

func (e *MappingError) ToHTTPError() *httperror.HTTPError {
	return httperror.NewHTTPError(http.StatusBadRequest, e.Error()).AddMetaValue("link_id", e.Link).AddMetaValue("field_id", e.Field).AddMetaValue("step_id", e.Step).AddMetaValue("action_key", e.Action)
}

func IsMappingError(err error) bool {
	_, ok := err.(*MappingError)
	return ok
}
