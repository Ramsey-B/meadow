package mappingdefinition

import (
	"context"
	"net/http"
	"time"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/lotus/internal/repositories/mappingdefinition"
	"github.com/Ramsey-B/lotus/pkg/mapping"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/google/uuid"
)

type MappingDefinitionRepository interface {
	Create(ctx context.Context, definition mapping.MappingDefinition) (mapping.MappingDefinition, error)
	Update(ctx context.Context, definition mapping.MappingDefinition) (mapping.MappingDefinition, error)
	GetActiveMappingDefinition(ctx context.Context, tenantID, id string) (mapping.MappingDefinition, error)
}

type Service struct {
	logger ectologger.Logger
	repo   mappingdefinition.MappingDefinitionRepository
}

func (s *Service) Create(ctx context.Context, definition mapping.MappingDefinition) (mapping.MappingDefinition, error) {
	ctx, span := tracing.StartSpan(ctx, "mappingdefinition.Create")
	defer span.End()

	definition.ID = uuid.New().String()
	definition.CreatedTS = time.Now().UTC()
	definition.UpdatedTS = time.Now().UTC()
	definition.IsActive = true
	definition.Version = 1

	if definition.TenantID == "" {
		return mapping.MappingDefinition{}, httperror.NewHTTPError(http.StatusBadRequest, "tenant_id is required")
	}

	if definition.UserID == "" {
		return mapping.MappingDefinition{}, httperror.NewHTTPError(http.StatusBadRequest, "user_id is required")
	}

	if definition.Key == "" {
		definition.Key = definition.ID
	}

	if definition.Name == "" {
		return mapping.MappingDefinition{}, httperror.NewHTTPError(http.StatusBadRequest, "name is required")
	}

	err := definition.GenerateMappingPlan()
	if err != nil {
		return mapping.MappingDefinition{}, err
	}

	s.logger.WithContext(ctx).WithFields(map[string]interface{}{
		"id":        definition.ID,
		"name":      definition.Name,
		"version":   definition.Version,
		"tenant_id": definition.TenantID,
		"user_id":   definition.UserID,
	}).Info("creating mapping definition")
	return definition, s.repo.Upsert(ctx, definition)
}

func (s *Service) Update(ctx context.Context, definition mapping.MappingDefinition) (mapping.MappingDefinition, error) {
	ctx, span := tracing.StartSpan(ctx, "mappingdefinition.Update")
	defer span.End()

	if definition.ID == "" {
		return mapping.MappingDefinition{}, httperror.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	if definition.TenantID == "" {
		return mapping.MappingDefinition{}, httperror.NewHTTPError(http.StatusBadRequest, "tenant_id is required")
	}

	if definition.UserID == "" {
		return mapping.MappingDefinition{}, httperror.NewHTTPError(http.StatusBadRequest, "user_id is required")
	}

	if definition.Version < 1 {
		return mapping.MappingDefinition{}, httperror.NewHTTPError(http.StatusBadRequest, "version is required")
	}

	definition.UpdatedTS = time.Now().UTC()

	err := definition.GenerateMappingPlan()
	if err != nil {
		return mapping.MappingDefinition{}, err
	}

	s.logger.WithContext(ctx).WithFields(map[string]interface{}{
		"id":        definition.ID,
		"name":      definition.Name,
		"version":   definition.Version,
		"tenant_id": definition.TenantID,
		"user_id":   definition.UserID,
	}).Info("updating mapping definition")
	return definition, s.repo.Upsert(ctx, definition)
}

func (s *Service) GetActiveMappingDefinition(ctx context.Context, tenantID, id string) (mapping.MappingDefinition, error) {
	ctx, span := tracing.StartSpan(ctx, "mappingdefinition.GetActiveMappingDefinition")
	defer span.End()

	if tenantID == "" {
		return mapping.MappingDefinition{}, httperror.NewHTTPError(http.StatusBadRequest, "tenant_id is required")
	}

	if id == "" {
		return mapping.MappingDefinition{}, httperror.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	definition, err := s.repo.GetActiveMappingDefinition(ctx, tenantID, id)
	if err != nil {
		return mapping.MappingDefinition{}, err
	}

	err = definition.GenerateMappingPlan()
	if err != nil {
		return mapping.MappingDefinition{}, err
	}

	for key, step := range definition.Steps {
		stepDef := definition.StepDefinitions[key]
		stepDef.OutputType = step.GetOutputType()
		definition.StepDefinitions[key] = stepDef
	}

	return definition, nil
}
