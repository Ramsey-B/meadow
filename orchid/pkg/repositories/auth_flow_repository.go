package repositories

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"
	"github.com/huandu/go-sqlbuilder"

	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

const authFlowsTable = "auth_flows"

var authFlowStruct = database.NewStruct(new(models.AuthFlow))

// AuthFlowRepository handles database operations for auth flows
type AuthFlowRepository struct {
	*Repository
}

// NewAuthFlowRepository creates a new auth flow repository
func NewAuthFlowRepository(db database.DB, logger ectologger.Logger) *AuthFlowRepository {
	return &AuthFlowRepository{
		Repository: NewRepository(db, logger),
	}
}

// Create creates a new auth flow
func (r *AuthFlowRepository) Create(ctx context.Context, authFlow *models.AuthFlow) error {
	ctx, span := tracing.StartSpan(ctx, "AuthFlowRepository.Create")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}
	authFlow.TenantID = tenantID

	if authFlow.ID == uuid.Nil {
		authFlow.ID = uuid.New()
	}

	ib := database.NewInsertBuilder()
	ib.InsertInto(authFlowsTable).
		Cols("id", "tenant_id", "integration_id", "name", "plan_definition",
			"token_path", "header_name", "header_format", "refresh_path",
			"expires_in_path", "ttl_seconds", "skew_seconds", "created_at", "updated_at").
		Values(authFlow.ID, authFlow.TenantID, authFlow.IntegrationID, authFlow.Name, authFlow.PlanDefinition,
			authFlow.TokenPath, authFlow.HeaderName, authFlow.HeaderFormat, authFlow.RefreshPath,
			authFlow.ExpiresInPath, authFlow.TTLSeconds, authFlow.SkewSeconds,
			sqlbuilder.Raw("NOW()"), sqlbuilder.Raw("NOW()")).
		Returning("created_at", "updated_at")

	query, args := ib.Build()
	err = r.DB().QueryRowContext(ctx, query, args...).Scan(&authFlow.CreatedAt, &authFlow.UpdatedAt)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"auth_flow_id": authFlow.ID,
		}).Error("failed to create auth flow")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to create auth flow")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"auth_flow_id": authFlow.ID,
	}).Debugf("Created %s", authFlowsTable)
	return nil
}

// GetByID retrieves an auth flow by ID (tenant-scoped)
func (r *AuthFlowRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AuthFlow, error) {
	ctx, span := tracing.StartSpan(ctx, "AuthFlowRepository.GetByID")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	sb := authFlowStruct.SelectFrom(authFlowsTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("id", id))

	query, args := sb.Build()
	var authFlow models.AuthFlow
	err = r.DB().GetContext(ctx, &authFlow, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, httperror.NewHTTPErrorf(http.StatusNotFound, "auth flow %s does not exist", id)
	}
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"auth_flow_id": id,
		}).Error("failed to get auth flow")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get auth flow")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"auth_flow_id": id,
	}).Debugf("Retrieved %s", authFlowsTable)
	return &authFlow, nil
}

// ListByIntegration retrieves all auth flows for an integration
func (r *AuthFlowRepository) ListByIntegration(ctx context.Context, integrationID uuid.UUID) ([]models.AuthFlow, error) {
	ctx, span := tracing.StartSpan(ctx, "AuthFlowRepository.ListByIntegration")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	sb := authFlowStruct.SelectFrom(authFlowsTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("integration_id", integrationID))
	sb.OrderBy("name")

	query, args := sb.Build()
	var authFlows []models.AuthFlow
	err = r.DB().SelectContext(ctx, &authFlows, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"auth_flow_count": len(authFlows),
		}).Error("failed to list auth flows")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list auth flows")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"auth_flow_count": len(authFlows),
	}).Debugf("Listed %s", authFlowsTable)
	return authFlows, nil
}

// Update updates an existing auth flow
func (r *AuthFlowRepository) Update(ctx context.Context, authFlow *models.AuthFlow) error {
	ctx, span := tracing.StartSpan(ctx, "AuthFlowRepository.Update")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"auth_flow_id": authFlow.ID,
		}).Error("failed to update auth flow")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to update auth flow")
	}

	ub := database.NewUpdateBuilder()
	ub.Update(authFlowsTable).
		Set(
			ub.Assign("name", authFlow.Name),
			ub.Assign("plan_definition", authFlow.PlanDefinition),
			ub.Assign("token_path", authFlow.TokenPath),
			ub.Assign("header_name", authFlow.HeaderName),
			ub.Assign("header_format", authFlow.HeaderFormat),
			ub.Assign("refresh_path", authFlow.RefreshPath),
			ub.Assign("expires_in_path", authFlow.ExpiresInPath),
			ub.Assign("ttl_seconds", authFlow.TTLSeconds),
			ub.Assign("skew_seconds", authFlow.SkewSeconds),
			ub.Assign("updated_at", sqlbuilder.Raw("NOW()")),
		).
		Where(ub.Equal("tenant_id", tenantID), ub.Equal("id", authFlow.ID))
	ub.SQL("RETURNING updated_at")

	query, args := ub.Build()
	err = r.DB().QueryRowContext(ctx, query, args...).Scan(&authFlow.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "auth flow %s does not exist", authFlow.ID)
	}
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"auth_flow_id": authFlow.ID,
		}).Error("failed to update auth flow")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to update auth flow")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"auth_flow_id": authFlow.ID,
	}).Debugf("Updated %s", authFlowsTable)
	return nil
}

// Delete deletes an auth flow by ID
func (r *AuthFlowRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := tracing.StartSpan(ctx, "AuthFlowRepository.Delete")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}

	db := database.NewDeleteBuilder()
	db.DeleteFrom(authFlowsTable).
		Where(db.Equal("tenant_id", tenantID), db.Equal("id", id))

	query, args := db.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"auth_flow_id": id,
		}).Error("failed to delete auth flow")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete auth flow")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"auth_flow_id": id,
		}).Error("failed to delete auth flow")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete auth flow")
	}
	if rows == 0 {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "auth flow %s does not exist", id)
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"auth_flow_id": id,
	}).Debugf("Deleted %s", authFlowsTable)
	return nil
}
