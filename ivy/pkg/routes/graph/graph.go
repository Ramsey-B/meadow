package graph

import (
	"net/http"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectoinject"
	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/context"
	"github.com/labstack/echo/v4"

	graphpkg "github.com/Ramsey-B/ivy/pkg/graph"
)

// Handler handles graph query API endpoints
type Handler struct {
	queryService *graphpkg.QueryService
	logger       ectologger.Logger
}

// NewHandler creates a new graph handler
func NewHandler(queryService *graphpkg.QueryService, logger ectologger.Logger) *Handler {
	return &Handler{
		queryService: queryService,
		logger:       logger,
	}
}

// Register registers the graph routes
func (h *Handler) Register(g *echo.Group) {
	g.POST("/query", h.ExecuteQuery)
	g.GET("/path", h.FindShortestPath)
	g.GET("/neighbors/:entityType/:entityId", h.FindNeighbors)
}

func (h *Handler) requireQueryService(c echo.Context) (*graphpkg.QueryService, error) {
	// Prefer explicitly provided service (useful for tests), but fall back to DI-from-context,
	// which is the standard pattern used elsewhere (see Lotus routes).
	if h != nil && h.queryService != nil {
		return h.queryService, nil
	}

	ctx := c.Request().Context()
	_, svc, err := ectoinject.GetContext[*graphpkg.QueryService](ctx)
	if err != nil || svc == nil {
		// 503 because this is an optional dependency (graph DB can be disabled).
		return nil, httperror.NewHTTPError(http.StatusServiceUnavailable, "graph query service unavailable")
	}
	return svc, nil
}

// QueryRequest is the request body for executing a Cypher query
type QueryRequest struct {
	Query  string         `json:"query" validate:"required"`
	Params map[string]any `json:"params,omitempty"`
}

// ExecuteQuery executes a read-only Cypher query
// @Summary Execute a Cypher query
// @Description Run a read-only OpenCypher query against the graph database
// @Tags Graph
// @Accept json
// @Produce json
// @Param body body QueryRequest true "Query request"
// @Success 200 {object} graphpkg.QueryResult
// @Failure 400 {object} httperror.HTTPError
// @Failure 500 {object} httperror.HTTPError
// @Router /api/v1/graph/query [post]
func (h *Handler) ExecuteQuery(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	qs, err := h.requireQueryService(c)
	if err != nil {
		return err
	}

	var req QueryRequest
	if err := c.Bind(&req); err != nil {
		return httperror.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Query == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "query is required")
	}

	// Execute the query
	result, err := qs.ExecuteQuery(ctx, tenantID, req.Query, req.Params)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, result)
}

// FindShortestPath finds the shortest path between two entities
// @Summary Find shortest path
// @Description Find the shortest path between two entities in the graph
// @Tags Graph
// @Produce json
// @Param from query string true "From entity ID"
// @Param to query string true "To entity ID"
// @Param max_hops query int false "Maximum hops (default 10)"
// @Success 200 {object} graphpkg.QueryResult
// @Failure 400 {object} httperror.HTTPError
// @Failure 500 {object} httperror.HTTPError
// @Router /api/v1/graph/path [get]
func (h *Handler) FindShortestPath(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	qs, err := h.requireQueryService(c)
	if err != nil {
		return err
	}

	fromID := c.QueryParam("from")
	toID := c.QueryParam("to")

	if fromID == "" || toID == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "from and to parameters are required")
	}

	maxHops := 10
	// Parse max_hops if provided
	if hopsStr := c.QueryParam("max_hops"); hopsStr != "" {
		var parsed int
		if err := echo.QueryParamsBinder(c).Int("max_hops", &parsed).BindError(); err == nil && parsed > 0 {
			maxHops = parsed
		}
	}

	result, err := qs.FindShortestPath(ctx, tenantID, fromID, toID, maxHops)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, result)
}

// FindNeighbors finds all entities connected to a given entity
// @Summary Find neighbors
// @Description Find all entities connected to a given entity within N hops
// @Tags Graph
// @Produce json
// @Param entityType path string true "Entity type"
// @Param entityId path string true "Entity ID"
// @Param hops query int false "Number of hops (default 1)"
// @Success 200 {object} graphpkg.QueryResult
// @Failure 400 {object} httperror.HTTPError
// @Failure 500 {object} httperror.HTTPError
// @Router /api/v1/graph/neighbors/{entityType}/{entityId} [get]
func (h *Handler) FindNeighbors(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	qs, err := h.requireQueryService(c)
	if err != nil {
		return err
	}

	entityType := c.Param("entityType")
	entityID := c.Param("entityId")

	if entityType == "" || entityID == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "entity type and ID are required")
	}

	hops := 1
	if hopsStr := c.QueryParam("hops"); hopsStr != "" {
		var parsed int
		if err := echo.QueryParamsBinder(c).Int("hops", &parsed).BindError(); err == nil && parsed > 0 {
			hops = parsed
		}
	}

	result, err := qs.FindNeighbors(ctx, tenantID, entityID, entityType, hops)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, result)
}

