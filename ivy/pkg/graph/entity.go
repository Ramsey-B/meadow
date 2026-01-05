package graph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/Ramsey-B/ivy/pkg/models"
)

// EntityService handles entity operations in the graph database
type EntityService struct {
	client *Client
	logger ectologger.Logger
}

// NewEntityService creates a new entity service
func NewEntityService(client *Client, logger ectologger.Logger) *EntityService {
	return &EntityService{
		client: client,
		logger: logger,
	}
}

// CreateOrUpdate creates or updates an entity node in the graph
func (s *EntityService) CreateOrUpdate(ctx context.Context, entity *models.MergedEntity) error {
	ctx, span := tracing.StartSpan(ctx, "graph.EntityService.CreateOrUpdate")
	defer span.End()

	log := s.logger.WithContext(ctx).WithFields(map[string]any{
		"entity_id":   entity.ID,
		"entity_type": entity.EntityType,
		"tenant_id":   entity.TenantID,
	})

	// Parse entity data
	var data map[string]any
	if err := json.Unmarshal(entity.Data, &data); err != nil {
		return fmt.Errorf("failed to parse entity data: %w", err)
	}

	// Build properties map with metadata
	props := map[string]any{
		"id":           entity.ID,
		"tenant_id":    entity.TenantID,
		"entity_type":  entity.EntityType,
		"source_count": entity.SourceCount,
		"version":      entity.Version,
		"created_at":   entity.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":   entity.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}

	// Add entity data fields as properties
	for k, v := range data {
		props[k] = v
	}

	// Use MERGE to create or update
	// The label is the entity type (e.g., :Person, :Company)
	cypher := fmt.Sprintf(`
		MERGE (e:%s {id: $id, tenant_id: $tenant_id})
		SET e = $props
		RETURN e
	`, sanitizeLabel(entity.EntityType))

	_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, map[string]any{
			"id":        entity.ID,
			"tenant_id": entity.TenantID,
			"props":     props,
		})
		if err != nil {
			return nil, err
		}
		return result.Consume(ctx)
	})

	if err != nil {
		log.WithError(err).Error("Failed to create/update entity in graph")
		return fmt.Errorf("failed to create/update entity in graph: %w", err)
	}

	log.Debug("Created/updated entity in graph")
	return nil
}

// Delete soft-deletes an entity by adding a deleted_at property
func (s *EntityService) Delete(ctx context.Context, tenantID string, entityID string, entityType string) error {
	ctx, span := tracing.StartSpan(ctx, "graph.EntityService.Delete")
	defer span.End()

	cypher := fmt.Sprintf(`
		MATCH (e:%s {id: $id, tenant_id: $tenant_id})
		SET e.deleted_at = datetime()
		RETURN e
	`, sanitizeLabel(entityType))

	_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, map[string]any{
			"id":        entityID,
			"tenant_id": tenantID,
		})
		if err != nil {
			return nil, err
		}
		return result.Consume(ctx)
	})

	if err != nil {
		s.logger.WithContext(ctx).WithError(err).Error("Failed to delete entity in graph")
		return fmt.Errorf("failed to delete entity in graph: %w", err)
	}

	return nil
}

// Get retrieves an entity by ID
func (s *EntityService) Get(ctx context.Context, tenantID string, entityID string, entityType string) (map[string]any, error) {
	ctx, span := tracing.StartSpan(ctx, "graph.EntityService.Get")
	defer span.End()

	cypher := fmt.Sprintf(`
		MATCH (e:%s {id: $id, tenant_id: $tenant_id})
		WHERE e.deleted_at IS NULL
		RETURN e
	`, sanitizeLabel(entityType))

	result, err := s.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, map[string]any{
			"id":        entityID,
			"tenant_id": tenantID,
		})
		if err != nil {
			return nil, err
		}

		if result.Next(ctx) {
			record := result.Record()
			node, ok := record.Get("e")
			if !ok {
				return nil, nil
			}
			n := node.(neo4j.Node)
			return n.Props, nil
		}
		return nil, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get entity from graph: %w", err)
	}

	if result == nil {
		return nil, nil
	}

	return result.(map[string]any), nil
}

// BatchCreateOrUpdate creates or updates multiple entities in a single transaction
func (s *EntityService) BatchCreateOrUpdate(ctx context.Context, entities []*models.MergedEntity) error {
	ctx, span := tracing.StartSpan(ctx, "graph.EntityService.BatchCreateOrUpdate")
	defer span.End()

	if len(entities) == 0 {
		return nil
	}

	log := s.logger.WithContext(ctx).WithFields(map[string]any{
		"batch_size": len(entities),
	})

	// Group entities by type for efficient batch operations
	byType := make(map[string][]*models.MergedEntity)
	for _, e := range entities {
		byType[e.EntityType] = append(byType[e.EntityType], e)
	}

	_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		for entityType, typeEntities := range byType {
			// Prepare batch data
			batchData := make([]map[string]any, len(typeEntities))
			for i, entity := range typeEntities {
				var data map[string]any
				json.Unmarshal(entity.Data, &data)

				props := map[string]any{
					"id":           entity.ID,
					"tenant_id":    entity.TenantID,
					"entity_type":  entity.EntityType,
					"source_count": entity.SourceCount,
					"version":      entity.Version,
					"created_at":   entity.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
					"updated_at":   entity.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
				}
				for k, v := range data {
					props[k] = v
				}
				batchData[i] = props
			}

			// UNWIND for efficient batch insert
			cypher := fmt.Sprintf(`
				UNWIND $batch AS props
				MERGE (e:%s {id: props.id, tenant_id: props.tenant_id})
				SET e = props
			`, sanitizeLabel(entityType))

			_, err := tx.Run(ctx, cypher, map[string]any{"batch": batchData})
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	})

	if err != nil {
		log.WithError(err).Error("Failed to batch create/update entities in graph")
		return fmt.Errorf("failed to batch create/update entities: %w", err)
	}

	log.Debug("Batch created/updated entities in graph")
	return nil
}

// sanitizeLabel ensures the label is safe for Cypher
func sanitizeLabel(label string) string {
	// Only allow alphanumeric and underscore
	result := ""
	for _, c := range label {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			result += string(c)
		}
	}
	if result == "" {
		return "Entity"
	}
	return result
}
