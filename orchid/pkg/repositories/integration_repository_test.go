package repositories_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/Gobusters/ectologger/zapadapter"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	appctx "github.com/Ramsey-B/stem/pkg/context"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/Ramsey-B/orchid/pkg/repositories"
)

func getTestLogger() ectologger.Logger {
	zapLogger, _ := zap.NewDevelopment()
	return zapadapter.NewZapEctoLogger(zapLogger, nil)
}

func getTestDB(t *testing.T) database.DB {
	// Use environment variables or defaults for test DB
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "coordinator"
	}
	dbUser := os.Getenv("DB_USER_NAME")
	if dbUser == "" {
		dbUser = "user"
	}
	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		dbPass = "password"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "orchid"
	}

	dsn := "host=" + dbHost + " user=" + dbUser + " password=" + dbPass + " dbname=" + dbName + " sslmode=disable"
	db, err := sqlx.Connect("postgres", dsn)
	require.NoError(t, err, "Failed to connect to test database")

	return database.NewDatabaseInstance(db, getTestLogger())
}

func getTestContext(tenantID uuid.UUID) context.Context {
	ctx := context.Background()
	return appctx.SetTenantID(ctx, tenantID.String())
}

// assertNotFound asserts that err is an HTTP 404 error
func assertNotFound(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	assert.True(t, httperror.IsHTTPError(err), "expected HTTP error, got: %v", err)
	assert.Equal(t, http.StatusNotFound, httperror.GetStatusCode(err), "expected 404, got: %d", httperror.GetStatusCode(err))
}

// assertUnauthorized asserts that err is an HTTP 401 error
func assertUnauthorized(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	assert.True(t, httperror.IsHTTPError(err), "expected HTTP error, got: %v", err)
	assert.Equal(t, http.StatusUnauthorized, httperror.GetStatusCode(err), "expected 401, got: %d", httperror.GetStatusCode(err))
}

func TestIntegrationRepository_CRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := getTestDB(t)
	logger := getTestLogger()
	repo := repositories.NewIntegrationRepository(db, logger)

	tenantID := uuid.New()
	ctx := getTestContext(tenantID)

	// Test Create
	integration := &models.Integration{
		Name:        "Test Integration",
		Description: strPtr("A test integration"),
	}

	err := repo.Create(ctx, integration)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, integration.ID)
	assert.Equal(t, tenantID, integration.TenantID)
	assert.False(t, integration.CreatedAt.IsZero())

	// Test GetByID
	fetched, err := repo.GetByID(ctx, integration.ID)
	require.NoError(t, err)
	assert.Equal(t, integration.ID, fetched.ID)
	assert.Equal(t, integration.Name, fetched.Name)
	assert.Equal(t, *integration.Description, *fetched.Description)

	// Test GetByName
	fetchedByName, err := repo.GetByName(ctx, "Test Integration")
	require.NoError(t, err)
	assert.Equal(t, integration.ID, fetchedByName.ID)

	// Test List
	integrations, err := repo.List(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(integrations), 1)

	// Test Update
	integration.Name = "Updated Integration"
	integration.Description = strPtr("Updated description")
	err = repo.Update(ctx, integration)
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, integration.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Integration", updated.Name)

	// Test tenant isolation - different tenant shouldn't see this integration
	otherTenantCtx := getTestContext(uuid.New())
	_, err = repo.GetByID(otherTenantCtx, integration.ID)
	assertNotFound(t, err)

	// Test Delete
	err = repo.Delete(ctx, integration.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, integration.ID)
	assertNotFound(t, err)
}

func TestIntegrationRepository_TenantRequired(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := getTestDB(t)
	logger := getTestLogger()
	repo := repositories.NewIntegrationRepository(db, logger)

	// Context without tenant ID
	ctx := context.Background()

	integration := &models.Integration{
		Name: "Should Fail",
	}

	err := repo.Create(ctx, integration)
	assertUnauthorized(t, err)
}

func strPtr(s string) *string {
	return &s
}
