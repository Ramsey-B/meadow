// Package graph provides Memgraph/Neo4j graph database client using Bolt protocol
package graph

import (
	"context"
	"fmt"
	"sync"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Client wraps the Neo4j driver for Memgraph compatibility
type Client struct {
	driver neo4j.DriverWithContext
	logger ectologger.Logger
	mu     sync.RWMutex
}

// Config holds graph database configuration
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
}

// NewClient creates a new graph database client
func NewClient(cfg Config, logger ectologger.Logger) (*Client, error) {
	uri := fmt.Sprintf("bolt://%s:%d", cfg.Host, cfg.Port)

	auth := neo4j.NoAuth()
	if cfg.Username != "" {
		auth = neo4j.BasicAuth(cfg.Username, cfg.Password, "")
	}

	driver, err := neo4j.NewDriverWithContext(uri, auth)
	if err != nil {
		return nil, fmt.Errorf("failed to create graph driver: %w", err)
	}

	return &Client{
		driver: driver,
		logger: logger,
	}, nil
}

// Close closes the driver connection
func (c *Client) Close(ctx context.Context) error {
	return c.driver.Close(ctx)
}

// VerifyConnectivity checks if the database is reachable
func (c *Client) VerifyConnectivity(ctx context.Context) error {
	return c.driver.VerifyConnectivity(ctx)
}

// Session creates a new session with the given access mode
func (c *Client) Session(ctx context.Context, accessMode neo4j.AccessMode) neo4j.SessionWithContext {
	return c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: accessMode,
	})
}

// ExecuteWrite runs a write transaction
func (c *Client) ExecuteWrite(ctx context.Context, work func(tx neo4j.ManagedTransaction) (any, error)) (any, error) {
	ctx, span := tracing.StartSpan(ctx, "graph.Client.ExecuteWrite")
	defer span.End()

	session := c.Session(ctx, neo4j.AccessModeWrite)
	defer session.Close(ctx)

	return session.ExecuteWrite(ctx, work)
}

// ExecuteRead runs a read transaction
func (c *Client) ExecuteRead(ctx context.Context, work func(tx neo4j.ManagedTransaction) (any, error)) (any, error) {
	ctx, span := tracing.StartSpan(ctx, "graph.Client.ExecuteRead")
	defer span.End()

	session := c.Session(ctx, neo4j.AccessModeRead)
	defer session.Close(ctx)

	return session.ExecuteRead(ctx, work)
}

// Run executes a single query in auto-commit mode
func (c *Client) Run(ctx context.Context, cypher string, params map[string]any) (neo4j.ResultWithContext, error) {
	ctx, span := tracing.StartSpan(ctx, "graph.Client.Run")
	defer span.End()

	session := c.Session(ctx, neo4j.AccessModeWrite)
	defer session.Close(ctx)

	return session.Run(ctx, cypher, params)
}

