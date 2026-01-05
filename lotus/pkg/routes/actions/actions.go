package actions

import (
	"net/http"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/actions"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/Ramsey-B/lotus/pkg/utils"
	"github.com/labstack/echo/v4"
)

type GetActionsRequest struct {
	Key string `json:"key" validate:"required"`
}

func GetActions(c echo.Context) error {
	_, span := tracing.StartSpan(c.Request().Context(), "mapping.TestMapping")
	defer span.End()

	result := ectolinq.Values(actions.ActionDefinitions)

	return c.JSON(http.StatusOK, result)
}

type GetOutputTypesRequest struct {
	Key        string                   `json:"key" validate:"required"`
	Arguments  any                      `json:"arguments" validate:"omitempty"`
	InputTypes []models.ActionValueType `json:"input_types" validate:"required"`
}

func GetOutputTypes(c echo.Context) error {
	_, span := tracing.StartSpan(c.Request().Context(), "mapping.TestMapping")
	defer span.End()

	req, err := utils.BindRequest[GetOutputTypesRequest](c)
	if err != nil {
		return err
	}

	// look up the action definition
	actionDefinition, ok := actions.ActionDefinitions[req.Key]
	if !ok {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "action '%s' not found", req.Key)
	}

	// validate the input types
	action, err := actionDefinition.Factory(req.Key, req.Arguments, req.InputTypes...)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]any{
		"output_type": action.GetOutputType(),
	})
}
