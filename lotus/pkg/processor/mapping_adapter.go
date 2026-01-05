package processor

import (
	"context"

	"github.com/Ramsey-B/lotus/pkg/mapping"
)

// MappingDefinitionRepository defines the interface for loading mapping definitions
type MappingDefinitionRepository interface {
	GetActiveMappingDefinition(ctx context.Context, tenantID, id string) (mapping.MappingDefinition, error)
}

// MappingRepositoryAdapter adapts the MappingDefinitionRepository to the MappingRepository interface
type MappingRepositoryAdapter struct {
	repo MappingDefinitionRepository
}

// NewMappingRepositoryAdapter creates a new adapter
func NewMappingRepositoryAdapter(repo MappingDefinitionRepository) *MappingRepositoryAdapter {
	return &MappingRepositoryAdapter{repo: repo}
}

// GetByID implements MappingRepository interface
func (a *MappingRepositoryAdapter) GetByID(ctx context.Context, tenantID, mappingID string) (*mapping.MappingDefinition, error) {
	def, err := a.repo.GetActiveMappingDefinition(ctx, tenantID, mappingID)
	if err != nil {
		return nil, err
	}
	return &def, nil
}

