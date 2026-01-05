package graph

import (
	"context"
	"fmt"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// RelationshipService handles relationship operations in the graph database
type RelationshipService struct {
	client *Client
	logger ectologger.Logger
}

// GetByID returns the relationship properties for a relationship by (tenant_id, rel_id).
func (s *RelationshipService) GetByID(ctx context.Context, tenantID string, relID string, relType string) (map[string]any, error) {
	ctx, span := tracing.StartSpan(ctx, "graph.RelationshipService.GetByID")
	defer span.End()

	cypher := fmt.Sprintf(`
		MATCH ()-[r:%s {id: $id, tenant_id: $tenant_id}]->()
		RETURN r
		LIMIT 1
	`, sanitizeLabel(relType))

	res, err := s.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, map[string]any{
			"id":        relID,
			"tenant_id": tenantID,
		})
		if err != nil {
			return nil, err
		}
		if !result.Next(ctx) {
			return nil, nil
		}
		record := result.Record()
		relNode, _ := record.Get("r")
		r := relNode.(neo4j.Relationship)
		return r.Props, nil
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return res.(map[string]any), nil
}

// NewRelationshipService creates a new relationship service
func NewRelationshipService(client *Client, logger ectologger.Logger) *RelationshipService {
	return &RelationshipService{
		client: client,
		logger: logger,
	}
}

// RelationshipInput represents the data needed to create a relationship
type RelationshipInput struct {
	ID               string
	TenantID         string
	FromEntityID     string
	FromEntityType   string
	ToEntityID       string
	ToEntityType     string
	RelationshipType string
	Properties       map[string]any
}

// CreateOrUpdate creates or updates a relationship between two entities
func (s *RelationshipService) CreateOrUpdate(ctx context.Context, rel *RelationshipInput) error {
	ctx, span := tracing.StartSpan(ctx, "graph.RelationshipService.CreateOrUpdate")
	defer span.End()

	log := s.logger.WithContext(ctx).WithFields(map[string]any{
		"rel_id":    rel.ID,
		"from":      rel.FromEntityID,
		"to":        rel.ToEntityID,
		"rel_type":  rel.RelationshipType,
		"tenant_id": rel.TenantID,
	})

	// Build properties
	props := map[string]any{
		"id":        rel.ID,
		"tenant_id": rel.TenantID,
	}
	for k, v := range rel.Properties {
		props[k] = v
	}

	// Match source and target entities, then MERGE the relationship
	cypher := fmt.Sprintf(`
		MATCH (from:%s {id: $from_id, tenant_id: $tenant_id})
		MATCH (to:%s {id: $to_id, tenant_id: $tenant_id})
		MERGE (from)-[r:%s {id: $rel_id, tenant_id: $tenant_id}]->(to)
		SET r += $props
		RETURN r
	`, sanitizeLabel(rel.FromEntityType), sanitizeLabel(rel.ToEntityType), sanitizeLabel(rel.RelationshipType))

	_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, map[string]any{
			"from_id":   rel.FromEntityID,
			"to_id":     rel.ToEntityID,
			"rel_id":    rel.ID,
			"tenant_id": rel.TenantID,
			"props":     props,
		})
		if err != nil {
			return nil, err
		}
		return result.Consume(ctx)
	})

	if err != nil {
		log.WithError(err).Error("Failed to create/update relationship in graph")
		return fmt.Errorf("failed to create/update relationship in graph: %w", err)
	}

	log.Debug("Created/updated relationship in graph")
	return nil
}

// Delete removes a relationship by setting deleted_at
func (s *RelationshipService) Delete(ctx context.Context, tenantID string, relID string, relType string) error {
	ctx, span := tracing.StartSpan(ctx, "graph.RelationshipService.Delete")
	defer span.End()

	cypher := fmt.Sprintf(`
		MATCH ()-[r:%s {id: $id, tenant_id: $tenant_id}]->()
		SET r.deleted_at = datetime()
		RETURN r
	`, sanitizeLabel(relType))

	_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, map[string]any{
			"id":        relID,
			"tenant_id": tenantID,
		})
		if err != nil {
			return nil, err
		}
		return result.Consume(ctx)
	})

	if err != nil {
		s.logger.WithContext(ctx).WithError(err).Error("Failed to delete relationship in graph")
		return fmt.Errorf("failed to delete relationship in graph: %w", err)
	}

	return nil
}

// BatchCreateOrUpdate creates or updates multiple relationships in a single transaction
func (s *RelationshipService) BatchCreateOrUpdate(ctx context.Context, rels []*RelationshipInput) error {
	ctx, span := tracing.StartSpan(ctx, "graph.RelationshipService.BatchCreateOrUpdate")
	defer span.End()

	if len(rels) == 0 {
		return nil
	}

	log := s.logger.WithContext(ctx).WithFields(map[string]any{
		"batch_size": len(rels),
	})

	// Group by relationship type for efficient batching
	type relKey struct {
		fromType string
		toType   string
		relType  string
	}
	byType := make(map[relKey][]*RelationshipInput)
	for _, r := range rels {
		key := relKey{r.FromEntityType, r.ToEntityType, r.RelationshipType}
		byType[key] = append(byType[key], r)
	}

	_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		for key, typeRels := range byType {
			// Prepare batch data
			batchData := make([]map[string]any, len(typeRels))
			for i, rel := range typeRels {
				props := map[string]any{
					"id":        rel.ID,
					"tenant_id": rel.TenantID,
				}
				for k, v := range rel.Properties {
					props[k] = v
				}
				batchData[i] = map[string]any{
					"id":        rel.ID,
					"tenant_id": rel.TenantID,
					"from_id":   rel.FromEntityID,
					"to_id":     rel.ToEntityID,
					"props":     props,
				}
			}

			// UNWIND for efficient batch
			cypher := fmt.Sprintf(`
				UNWIND $batch AS data
				MATCH (from:%s {id: data.from_id, tenant_id: data.tenant_id})
				MATCH (to:%s {id: data.to_id, tenant_id: data.tenant_id})
				MERGE (from)-[r:%s {id: data.id, tenant_id: data.tenant_id}]->(to)
				SET r += data.props
			`, sanitizeLabel(key.fromType), sanitizeLabel(key.toType), sanitizeLabel(key.relType))

			_, err := tx.Run(ctx, cypher, map[string]any{"batch": batchData})
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	})

	if err != nil {
		log.WithError(err).Error("Failed to batch create/update relationships in graph")
		return fmt.Errorf("failed to batch create/update relationships: %w", err)
	}

	log.Debug("Batch created/updated relationships in graph")
	return nil
}

// GetRelationships gets all relationships for an entity
func (s *RelationshipService) GetRelationships(ctx context.Context, tenantID string, entityID string, entityType string, direction string) ([]map[string]any, error) {
	ctx, span := tracing.StartSpan(ctx, "graph.RelationshipService.GetRelationships")
	defer span.End()

	var cypher string
	switch direction {
	case "outgoing":
		cypher = fmt.Sprintf(`
			MATCH (e:%s {id: $id, tenant_id: $tenant_id})-[r]->(target)
			WHERE r.deleted_at IS NULL
			RETURN r, type(r) as rel_type, target
		`, sanitizeLabel(entityType))
	case "incoming":
		cypher = fmt.Sprintf(`
			MATCH (source)-[r]->(e:%s {id: $id, tenant_id: $tenant_id})
			WHERE r.deleted_at IS NULL
			RETURN r, type(r) as rel_type, source as target
		`, sanitizeLabel(entityType))
	default: // both
		cypher = fmt.Sprintf(`
			MATCH (e:%s {id: $id, tenant_id: $tenant_id})-[r]-(target)
			WHERE r.deleted_at IS NULL
			RETURN r, type(r) as rel_type, target
		`, sanitizeLabel(entityType))
	}

	result, err := s.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, map[string]any{
			"id":        entityID,
			"tenant_id": tenantID,
		})
		if err != nil {
			return nil, err
		}

		var rels []map[string]any
		for result.Next(ctx) {
			record := result.Record()
			relNode, _ := record.Get("r")
			relType, _ := record.Get("rel_type")
			targetNode, _ := record.Get("target")

			r := relNode.(neo4j.Relationship)
			t := targetNode.(neo4j.Node)

			rel := map[string]any{
				"id":          r.Props["id"],
				"type":        relType,
				"target_id":   t.Props["id"],
				"target_type": t.Props["entity_type"],
				"properties":  r.Props,
			}
			rels = append(rels, rel)
		}
		return rels, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get relationships from graph: %w", err)
	}

	return result.([]map[string]any), nil
}
