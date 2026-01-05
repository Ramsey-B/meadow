package graph

import (
	"context"
	"fmt"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// QueryService handles graph queries (OpenCypher)
type QueryService struct {
	client *Client
	logger ectologger.Logger
}

// NewQueryService creates a new query service
func NewQueryService(client *Client, logger ectologger.Logger) *QueryService {
	return &QueryService{
		client: client,
		logger: logger,
	}
}

// QueryResult represents the result of a graph query
type QueryResult struct {
	Nodes         []NodeResult `json:"nodes,omitempty"`
	Relationships []RelResult  `json:"relationships,omitempty"`
	Rows          []any        `json:"rows,omitempty"`
}

// NodeResult represents a node from query results
type NodeResult struct {
	ID         string         `json:"id"`
	Labels     []string       `json:"labels"`
	Properties map[string]any `json:"properties"`
}

// RelResult represents a relationship from query results
type RelResult struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	StartNode  string         `json:"start_node"`
	EndNode    string         `json:"end_node"`
	Properties map[string]any `json:"properties"`
}

// ExecuteQuery runs a read-only Cypher query with tenant isolation
func (s *QueryService) ExecuteQuery(ctx context.Context, tenantID string, cypher string, params map[string]any) (*QueryResult, error) {
	ctx, span := tracing.StartSpan(ctx, "graph.QueryService.ExecuteQuery")
	defer span.End()

	log := s.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id": tenantID,
		"query_len": len(cypher),
	})

	// Add tenant_id to params for use in query
	if params == nil {
		params = make(map[string]any)
	}
	params["_tenant_id"] = tenantID

	result, err := s.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}

		qr := &QueryResult{
			Nodes:         make([]NodeResult, 0),
			Relationships: make([]RelResult, 0),
			Rows:          make([]any, 0),
		}

		seenNodes := make(map[string]bool)
		seenRels := make(map[string]bool)

		for result.Next(ctx) {
			record := result.Record()
			row := make(map[string]any)

			for _, key := range record.Keys {
				val, _ := record.Get(key)
				row[key] = extractValue(val, qr, seenNodes, seenRels)
			}

			qr.Rows = append(qr.Rows, row)
		}

		return qr, nil
	})

	if err != nil {
		log.WithError(err).Error("Failed to execute graph query")
		return nil, fmt.Errorf("failed to execute graph query: %w", err)
	}

	return result.(*QueryResult), nil
}

// FindShortestPath finds the shortest path between two entities
func (s *QueryService) FindShortestPath(ctx context.Context, tenantID string, fromID, toID string, maxHops int) (*QueryResult, error) {
	ctx, span := tracing.StartSpan(ctx, "graph.QueryService.FindShortestPath")
	defer span.End()

	if maxHops <= 0 {
		maxHops = 10
	}

	cypher := fmt.Sprintf(`
		MATCH (start {id: $from_id, tenant_id: $tenant_id})
		MATCH (end {id: $to_id, tenant_id: $tenant_id})
		MATCH p = shortestPath((start)-[*..%d]-(end))
		WHERE ALL(r IN relationships(p) WHERE r.deleted_at IS NULL)
		RETURN p
	`, maxHops)

	return s.ExecuteQuery(ctx, tenantID, cypher, map[string]any{
		"from_id": fromID,
		"to_id":   toID,
	})
}

// FindNeighbors finds all entities connected within N hops
func (s *QueryService) FindNeighbors(ctx context.Context, tenantID string, entityID string, entityType string, hops int) (*QueryResult, error) {
	ctx, span := tracing.StartSpan(ctx, "graph.QueryService.FindNeighbors")
	defer span.End()

	if hops <= 0 {
		hops = 1
	}

	cypher := fmt.Sprintf(`
		MATCH (start:%s {id: $id, tenant_id: $tenant_id})
		MATCH (start)-[r*1..%d]-(neighbor)
		WHERE neighbor.deleted_at IS NULL
		AND ALL(rel IN r WHERE rel.deleted_at IS NULL)
		RETURN DISTINCT neighbor
	`, sanitizeLabel(entityType), hops)

	return s.ExecuteQuery(ctx, tenantID, cypher, map[string]any{
		"id": entityID,
	})
}

// extractValue converts neo4j types to standard Go types
func extractValue(val any, qr *QueryResult, seenNodes, seenRels map[string]bool) any {
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case neo4j.Node:
		id := fmt.Sprintf("%v", v.Props["id"])
		if !seenNodes[id] {
			seenNodes[id] = true
			qr.Nodes = append(qr.Nodes, NodeResult{
				ID:         id,
				Labels:     v.Labels,
				Properties: v.Props,
			})
		}
		return id

	case neo4j.Relationship:
		id := fmt.Sprintf("%v", v.Props["id"])
		if !seenRels[id] {
			seenRels[id] = true
			qr.Relationships = append(qr.Relationships, RelResult{
				ID:         id,
				Type:       v.Type,
				Properties: v.Props,
			})
		}
		return id

	case neo4j.Path:
		// Extract nodes and relationships from path
		for _, node := range v.Nodes {
			extractValue(node, qr, seenNodes, seenRels)
		}
		for _, rel := range v.Relationships {
			extractValue(rel, qr, seenNodes, seenRels)
		}
		return map[string]any{
			"node_count": len(v.Nodes),
			"rel_count":  len(v.Relationships),
		}

	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = extractValue(item, qr, seenNodes, seenRels)
		}
		return result

	default:
		return v
	}
}

