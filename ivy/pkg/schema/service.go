package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/ivy/pkg/models"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

// EntityTypeGetter interface for fetching entity type schemas
type EntityTypeGetter interface {
	GetByKey(ctx context.Context, tenantID string, key string) (*models.EntityType, error)
}

// ValidationService provides schema validation for entity data
type ValidationService struct {
	entityTypeGetter EntityTypeGetter
	logger           ectologger.Logger
	cache            sync.Map // map[tenantID:entityType:version]*Validator
}

// NewValidationService creates a new validation service
func NewValidationService(getter EntityTypeGetter, logger ectologger.Logger) *ValidationService {
	return &ValidationService{
		entityTypeGetter: getter,
		logger:           logger,
	}
}

// ValidateEntityData validates entity data against its entity type schema
func (s *ValidationService) ValidateEntityData(ctx context.Context, tenantID, entityType string, data map[string]any) (ValidationResult, error) {
	ctx, span := tracing.StartSpan(ctx, "ValidationService.ValidateEntityData")
	defer span.End()

	// Get the entity type
	et, err := s.entityTypeGetter.GetByKey(ctx, tenantID, entityType)
	if err != nil {
		return ValidationResult{}, fmt.Errorf("failed to get entity type: %w", err)
	}
	if et == nil {
		return ValidationResult{}, fmt.Errorf("entity type %q not found", entityType)
	}

	// Get or create validator (cached by version)
	validator, err := s.getValidator(tenantID, entityType, et.Version, et.Schema)
	if err != nil {
		return ValidationResult{}, fmt.Errorf("failed to create validator: %w", err)
	}

	result := validator.Validate(data)

	if !result.Valid {
		s.logger.WithContext(ctx).WithFields(map[string]any{
			"tenant_id":   tenantID,
			"entity_type": entityType,
			"errors":      len(result.Errors),
		}).Debug("entity data validation failed")
	}

	return result, nil
}

// getValidator returns a cached validator or creates a new one
func (s *ValidationService) getValidator(tenantID, entityType string, version int, schemaJSON json.RawMessage) (*Validator, error) {
	cacheKey := fmt.Sprintf("%s:%s:%d", tenantID, entityType, version)

	if cached, ok := s.cache.Load(cacheKey); ok {
		return cached.(*Validator), nil
	}

	validator, err := NewValidator(schemaJSON)
	if err != nil {
		return nil, err
	}

	s.cache.Store(cacheKey, validator)
	return validator, nil
}

// InvalidateCache invalidates the cache for a specific entity type
func (s *ValidationService) InvalidateCache(tenantID, entityType string) {
	// Delete all versions for this entity type
	s.cache.Range(func(key, value any) bool {
		keyStr := key.(string)
		prefix := fmt.Sprintf("%s:%s:", tenantID, entityType)
		if len(keyStr) >= len(prefix) && keyStr[:len(prefix)] == prefix {
			s.cache.Delete(key)
		}
		return true
	})
}

// GetFingerprintExclusions returns the set of field paths that should be excluded
// from fingerprint calculation for the given entity type.
// Returns nil if the entity type doesn't exist or has no exclusions defined.
func (s *ValidationService) GetFingerprintExclusions(ctx context.Context, tenantID, entityType string) (map[string]bool, error) {
	ctx, span := tracing.StartSpan(ctx, "ValidationService.GetFingerprintExclusions")
	defer span.End()

	et, err := s.entityTypeGetter.GetByKey(ctx, tenantID, entityType)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity type: %w", err)
	}
	if et == nil {
		// Entity type not defined yet - no exclusions
		return nil, nil
	}

	var schema models.EntityTypeSchema
	if err := json.Unmarshal(et.Schema, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse entity type schema: %w", err)
	}

	exclusions := schema.GetFingerprintExclusions()
	if len(exclusions) == 0 {
		return nil, nil
	}

	return exclusions, nil
}

// Service is an alias for ValidationService for use by the processor
type Service = ValidationService

// NewService creates a new validation service (alias for NewValidationService)
func NewService(getter EntityTypeGetter, logger ectologger.Logger) *Service {
	return NewValidationService(getter, logger)
}

