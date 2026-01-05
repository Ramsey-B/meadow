package repositories

import (
	"context"

	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/google/uuid"
)

// IntegrationRepo defines the interface for integration repository operations
type IntegrationRepo interface {
	Create(ctx context.Context, integration *models.Integration) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Integration, error)
	GetByName(ctx context.Context, name string) (*models.Integration, error)
	List(ctx context.Context) ([]models.Integration, error)
	Update(ctx context.Context, integration *models.Integration) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// ConfigRepo defines the interface for config repository operations
type ConfigRepo interface {
	Create(ctx context.Context, config *models.Config) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Config, error)
	ListByIntegration(ctx context.Context, integrationID uuid.UUID) ([]models.Config, error)
	ListEnabled(ctx context.Context) ([]models.Config, error)
	Update(ctx context.Context, config *models.Config) error
	SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// AuthFlowRepo defines the interface for auth flow repository operations
type AuthFlowRepo interface {
	Create(ctx context.Context, authFlow *models.AuthFlow) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AuthFlow, error)
	ListByIntegration(ctx context.Context, integrationID uuid.UUID) ([]models.AuthFlow, error)
	Update(ctx context.Context, authFlow *models.AuthFlow) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// PlanRepo defines the interface for plan repository operations
type PlanRepo interface {
	Create(ctx context.Context, plan *models.Plan) error
	GetByKey(ctx context.Context, key string) (*models.Plan, error)
	ListByIntegration(ctx context.Context, integrationID uuid.UUID) ([]models.Plan, error)
	ListEnabled(ctx context.Context) ([]models.Plan, error)
	Update(ctx context.Context, plan *models.Plan) error
	SetEnabled(ctx context.Context, key string, enabled bool) error
	Delete(ctx context.Context, key string) error
}

// PlanExecutionRepo defines the interface for plan execution repository operations
type PlanExecutionRepo interface {
	Create(ctx context.Context, execution *models.PlanExecution) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.PlanExecution, error)
	ListByPlan(ctx context.Context, planKey string, limit int) ([]models.PlanExecution, error)
	ListByStatus(ctx context.Context, status models.ExecutionStatus, limit int) ([]models.PlanExecution, error)
	ListChildren(ctx context.Context, parentID uuid.UUID) ([]models.PlanExecution, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.ExecutionStatus) error
	MarkStarted(ctx context.Context, id uuid.UUID) error
	MarkCompleted(ctx context.Context, id uuid.UUID, status models.ExecutionStatus, errorMsg *string, errorType *models.ErrorType) error
	IncrementRetry(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// PlanContextRepo defines the interface for plan context repository operations
type PlanContextRepo interface {
	Upsert(ctx context.Context, planContext *models.PlanContext) error
	GetByPlanAndConfig(ctx context.Context, planKey string, configID uuid.UUID) (*models.PlanContext, error)
	ListByPlan(ctx context.Context, planKey string) ([]models.PlanContext, error)
	Delete(ctx context.Context, planKey string, configID uuid.UUID) error
}

// PlanStatisticsRepo defines the interface for plan statistics repository operations
type PlanStatisticsRepo interface {
	GetOrCreate(ctx context.Context, planKey string, configID uuid.UUID) (*models.PlanStatistics, error)
	GetByPlanAndConfig(ctx context.Context, planKey string, configID uuid.UUID) (*models.PlanStatistics, error)
	RecordExecution(ctx context.Context, planKey string, configID uuid.UUID, success bool, executionTimeMs int) error
	IncrementAPICalls(ctx context.Context, planKey string, configID uuid.UUID, count int) error
	ListByPlan(ctx context.Context, planKey string) ([]models.PlanStatistics, error)
	Delete(ctx context.Context, planKey string, configID uuid.UUID) error
}
