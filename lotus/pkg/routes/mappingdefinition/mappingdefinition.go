package mappingdefinition

import (
	"net/http"

	"github.com/Gobusters/ectoinject"
	"github.com/Ramsey-B/lotus/internal/services/mappingdefinition"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/fields"
	"github.com/Ramsey-B/lotus/pkg/links"
	"github.com/Ramsey-B/lotus/pkg/mapping"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
	"github.com/Ramsey-B/stem/pkg/context"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/labstack/echo/v4"
)

type MappingValidateRequest struct {
	mapping.MappingDefinitionFields
	SourceFields fields.Fields           `json:"source_fields" validate:"required"`
	TargetFields fields.Fields           `json:"target_fields" validate:"required"`
	Steps        []models.StepDefinition `json:"steps" validate:"required"`
	Links        links.Links             `json:"links" validate:"required"`
}

func ValidateMapping(c echo.Context) error {
	req, err := utils.BindRequest[MappingValidateRequest](c)
	if err != nil {
		return err
	}

	mapDef := mapping.NewMappingDefinition(req.MappingDefinitionFields, req.SourceFields, req.TargetFields, req.Steps, req.Links)

	err = mapDef.GenerateMappingPlan()
	if errors.IsMappingError(err) {
		return c.JSON(http.StatusBadRequest, err)
	}
	if err != nil {
		return err
	}

	// Return extracted params
	return c.JSON(http.StatusAccepted, req)
}

func CreateMapping(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "mappingdefinition.CreateMapping")
	defer span.End()

	req, err := utils.BindRequest[MappingValidateRequest](c)
	if err != nil {
		return err
	}

	req.TenantID = context.GetTenantID(ctx)
	req.UserID = context.GetUserID(ctx)

	mapDef := mapping.NewMappingDefinition(req.MappingDefinitionFields, req.SourceFields, req.TargetFields, req.Steps, req.Links)

	ctx, service, err := ectoinject.GetContext[mappingdefinition.MappingDefinitionRepository](ctx)
	if err != nil {
		return err
	}

	result, err := service.Create(ctx, *mapDef)
	if err != nil {
		if errors.IsMappingError(err) {
			mappingErr := err.(*errors.MappingError)
			return mappingErr.ToHTTPError()
		}
		return err
	}

	return c.JSON(http.StatusAccepted, result)
}

func UpdateMapping(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "mappingdefinition.UpdateMapping")
	defer span.End()

	req, err := utils.BindRequest[MappingValidateRequest](c)
	if err != nil {
		return err
	}

	req.TenantID = context.GetTenantID(ctx)
	req.UserID = context.GetUserID(ctx)

	mapDef := mapping.NewMappingDefinition(req.MappingDefinitionFields, req.SourceFields, req.TargetFields, req.Steps, req.Links)

	ctx, service, err := ectoinject.GetContext[mappingdefinition.MappingDefinitionRepository](ctx)
	if err != nil {
		return err
	}

	result, err := service.Update(ctx, *mapDef)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusAccepted, result)
}

func GetActiveMappingDefinition(c echo.Context) error {
	ctx, span := tracing.StartSpan(c.Request().Context(), "mappingdefinition.GetActiveMappingDefinition")
	defer span.End()

	tenantID := context.GetTenantID(ctx)
	id := c.Param("id")

	ctx, service, err := ectoinject.GetContext[mappingdefinition.MappingDefinitionRepository](ctx)
	if err != nil {
		return err
	}

	result, err := service.GetActiveMappingDefinition(ctx, tenantID, id)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, result)
}
