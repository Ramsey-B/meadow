package handlers

import (
	"net/http"

	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/Ramsey-B/orchid/pkg/repositories"
)

// TenantHandler handles tenant-level operations
type TenantHandler struct {
	integrationRepo repositories.IntegrationRepo
	configRepo      repositories.ConfigRepo
	authFlowRepo    repositories.AuthFlowRepo
	planRepo        repositories.PlanRepo
	executionRepo   repositories.PlanExecutionRepo
	statisticsRepo  repositories.PlanStatisticsRepo
	logger          ectologger.Logger
}

// NewTenantHandler creates a new tenant handler
func NewTenantHandler(
	integrationRepo repositories.IntegrationRepo,
	configRepo repositories.ConfigRepo,
	authFlowRepo repositories.AuthFlowRepo,
	planRepo repositories.PlanRepo,
	executionRepo repositories.PlanExecutionRepo,
	statisticsRepo repositories.PlanStatisticsRepo,
	logger ectologger.Logger,
) *TenantHandler {
	return &TenantHandler{
		integrationRepo: integrationRepo,
		configRepo:      configRepo,
		authFlowRepo:    authFlowRepo,
		planRepo:        planRepo,
		executionRepo:   executionRepo,
		statisticsRepo:  statisticsRepo,
		logger:          logger,
	}
}

// RegisterRoutes registers tenant routes
func (h *TenantHandler) RegisterRoutes(g *echo.Group) {
	g.DELETE("/tenant/:tenant_id", h.DeleteTenantData)
}

// DeleteTenantData deletes all data for a specific tenant
// This is intended for testing purposes to clean up test data
func (h *TenantHandler) DeleteTenantData(c echo.Context) error {
	ctx := c.Request().Context()

	tenantIDStr := c.Param("tenant_id")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid tenant_id format",
		})
	}

	h.logger.WithContext(ctx).WithFields(map[string]any{"tenant_id": tenantID}).Info("Deleting all data for tenant")

	// Delete in order respecting foreign key constraints
	// 1. Delete executions (references plans)
	execCount, err := h.executionRepo.DeleteByTenantID(ctx, tenantID)
	if err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to delete executions")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to delete executions",
		})
	}

	// 2. Delete statistics (references plans)
	statsCount, err := h.statisticsRepo.DeleteByTenantID(ctx, tenantID)
	if err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to delete statistics")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to delete statistics",
		})
	}

	// 3. Delete plans (references integrations)
	planCount, err := h.planRepo.DeleteByTenantID(ctx, tenantID)
	if err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to delete plans")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to delete plans",
		})
	}

	// 4. Delete auth flows (references integrations)
	authFlowCount, err := h.authFlowRepo.DeleteByTenantID(ctx, tenantID)
	if err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to delete auth flows")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to delete auth flows",
		})
	}

	// 5. Delete configs (references integrations)
	configCount, err := h.configRepo.DeleteByTenantID(ctx, tenantID)
	if err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to delete configs")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to delete configs",
		})
	}

	// 6. Delete integrations (base table)
	integrationCount, err := h.integrationRepo.DeleteByTenantID(ctx, tenantID)
	if err != nil {
		h.logger.WithContext(ctx).WithError(err).Error("Failed to delete integrations")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to delete integrations",
		})
	}

	h.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":    tenantID,
		"integrations": integrationCount,
		"configs":      configCount,
		"auth_flows":   authFlowCount,
		"plans":        planCount,
		"executions":   execCount,
		"statistics":   statsCount,
	}).Info("Tenant data deleted")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":      "tenant data deleted",
		"tenant_id":    tenantID,
		"integrations": integrationCount,
		"configs":      configCount,
		"auth_flows":   authFlowCount,
		"plans":        planCount,
		"executions":   execCount,
		"statistics":   statsCount,
	})
}

