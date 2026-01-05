package mapping

import (
	"net/http"

	"github.com/Gobusters/ectoinject"
	"github.com/Ramsey-B/lotus/internal/services/mappingdefinition"
	"github.com/Ramsey-B/stem/pkg/context"
	maperr "github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/fields"
	"github.com/Ramsey-B/lotus/pkg/links"
	"github.com/Ramsey-B/lotus/pkg/mapping"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/Ramsey-B/lotus/pkg/utils"
	"github.com/labstack/echo/v4"
)

type MappingValidateRequest struct {
	mapping.MappingDefinitionFields
	SourceFields fields.Fields           `json:"source_fields" validate:"required"`
	TargetFields fields.Fields           `json:"target_fields" validate:"required"`
	Steps        []models.StepDefinition `json:"steps" validate:"required"`
	Links        links.Links             `json:"links" validate:"required"`
}

type MappingExecuteRequest struct {
	ID        string `json:"id" param:"id" validate:"required"`
	SourceRaw any    `json:"source_raw" validate:"required"`
}

func ExecuteMapping(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "mapping.ExecuteMapping")
	defer span.End()

	req, err := utils.BindRequest[MappingExecuteRequest](c)
	if err != nil {
		return err
	}

	ctx, service, err := ectoinject.GetContext[mappingdefinition.MappingDefinitionRepository](ctx)
	if err != nil {
		return err
	}

	tenantID := context.GetTenantID(ctx)

	mapDef, err := service.GetActiveMappingDefinition(ctx, tenantID, req.ID)
	if err != nil {
		return err
	}

	result, err := mapDef.ExecuteMapping(req.SourceRaw)
	if err != nil {
		// check if the error is a mapping error
		if mappingErr, ok := err.(*maperr.MappingError); ok {
			return mappingErr.ToHTTPError()
		}

		return err
	}

	return c.JSON(http.StatusOK, result)
}

type TestMappingRequest struct {
	SourceRaw    any                     `json:"source_raw" validate:"required"`
	SourceFields fields.Fields           `json:"source_fields" validate:"required"`
	TargetFields fields.Fields           `json:"target_fields" validate:"required"`
	Steps        []models.StepDefinition `json:"steps" validate:"required"`
	Links        links.Links             `json:"links" validate:"required"`
}

func TestMapping(c echo.Context) error {
	_, span := tracing.StartSpan(c.Request().Context(), "mapping.TestMapping")
	defer span.End()

	req, err := utils.BindRequest[TestMappingRequest](c)
	if err != nil {
		return err
	}

	mapDef := mapping.NewMappingDefinition(mapping.MappingDefinitionFields{}, req.SourceFields, req.TargetFields, req.Steps, req.Links)

	result, err := mapDef.ExecuteMapping(req.SourceRaw)
	if err != nil {
		// check if the error is a mapping error
		if mappingErr, ok := err.(*maperr.MappingError); ok {
			return mappingErr.ToHTTPError()
		}

		return err
	}

	return c.JSON(http.StatusOK, result)
}
