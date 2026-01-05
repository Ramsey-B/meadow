package matchcandidate

import (
	"net/http"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectoinject"
	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/context"
	"github.com/labstack/echo/v4"

	"github.com/Ramsey-B/ivy/internal/repositories/matchcandidate"
	"github.com/Ramsey-B/ivy/pkg/models"
)

// Register registers match candidate routes
func Register(g *echo.Group) {
	g.GET("", ListMatchCandidates)
	g.GET("/:id", GetMatchCandidate)
	g.POST("/:id/approve", ApproveMatchCandidate)
	g.POST("/:id/reject", RejectMatchCandidate)
	g.POST("/:id/defer", DeferMatchCandidate)
}

// ListMatchCandidates lists match candidates with optional filters
func ListMatchCandidates(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	status := c.QueryParam("status")
	entityID := c.QueryParam("entity_id")

	ctx, repo, err := ectoinject.GetContext[*matchcandidate.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	var candidates []models.MatchCandidate

	if entityID != "" {
		candidates, err = repo.ListByEntity(ctx, tenantID, entityID, status)
		if err != nil {
			return err
		}
	} else {
		// List pending candidates for review
		candidates, err = repo.ListPending(ctx, tenantID, 100)
		if err != nil {
			return err
		}
	}

	return c.JSON(http.StatusOK, candidates)
}

// GetMatchCandidate gets a match candidate by entity pair
func GetMatchCandidate(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	// Parse composite ID (entityA_entityB)
	entityAID := c.QueryParam("entity_a_id")
	entityBID := c.QueryParam("entity_b_id")

	if entityAID == "" || entityBID == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "entity_a_id and entity_b_id query parameters are required")
	}

	ctx, repo, err := ectoinject.GetContext[*matchcandidate.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	candidate, err := repo.GetByEntityPair(ctx, tenantID, entityAID, entityBID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, candidate)
}

// ApproveMatchCandidate approves a match candidate for merging
func ApproveMatchCandidate(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	entityAID := c.QueryParam("entity_a_id")
	entityBID := c.QueryParam("entity_b_id")

	if entityAID == "" || entityBID == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "entity_a_id and entity_b_id query parameters are required")
	}

	ctx, repo, err := ectoinject.GetContext[*matchcandidate.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	if err := repo.UpdateStatus(ctx, tenantID, entityAID, entityBID, models.MatchCandidateStatusApproved); err != nil {
		return err
	}

	ctx, logger, _ := ectoinject.GetContext[ectologger.Logger](ctx)
	if logger != nil {
		logger.WithContext(ctx).WithFields(map[string]any{
			"entity_a_id": entityAID,
			"entity_b_id": entityBID,
		}).Info("Approved match candidate")
	}

	// TODO: Trigger merge process after approval

	return c.JSON(http.StatusOK, map[string]string{"status": "approved"})
}

// RejectMatchCandidate rejects a match candidate
func RejectMatchCandidate(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	entityAID := c.QueryParam("entity_a_id")
	entityBID := c.QueryParam("entity_b_id")

	if entityAID == "" || entityBID == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "entity_a_id and entity_b_id query parameters are required")
	}

	ctx, repo, err := ectoinject.GetContext[*matchcandidate.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	if err := repo.UpdateStatus(ctx, tenantID, entityAID, entityBID, models.MatchCandidateStatusRejected); err != nil {
		return err
	}

	ctx, logger, _ := ectoinject.GetContext[ectologger.Logger](ctx)
	if logger != nil {
		logger.WithContext(ctx).WithFields(map[string]any{
			"entity_a_id": entityAID,
			"entity_b_id": entityBID,
		}).Info("Rejected match candidate")
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "rejected"})
}

// DeferMatchCandidate defers a match candidate for later review
func DeferMatchCandidate(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := context.GetTenantID(ctx)

	entityAID := c.QueryParam("entity_a_id")
	entityBID := c.QueryParam("entity_b_id")

	if entityAID == "" || entityBID == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, "entity_a_id and entity_b_id query parameters are required")
	}

	ctx, repo, err := ectoinject.GetContext[*matchcandidate.Repository](ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "service unavailable")
	}

	if err := repo.UpdateStatus(ctx, tenantID, entityAID, entityBID, models.MatchCandidateStatusDeferred); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "deferred"})
}
