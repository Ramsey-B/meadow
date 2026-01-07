# 🌱 Stem

**Shared Go library providing foundational packages for the Meadow microservices ecosystem**

Stem is a production-ready framework that abstracts common concerns across Meadow services (Orchid, Lotus, Ivy), providing database operations, distributed tracing, HTTP middleware, context management, and service orchestration.

## Table of Contents

- [Architecture](#architecture)
- [Packages](#packages)
  - [database](#database---database-utilities)
  - [tracing](#tracing---opentelemetry-integration)
  - [middleware](#middleware---echo-http-middleware)
  - [context](#context---request-context-management)
  - [startup](#startup---service-orchestration)
- [Usage](#usage)
- [Installation](#installation)
- [Dependencies](#dependencies)

## Architecture

```
┌───────────────────────────────────────────────────────────────────┐
│                        Applications                                │
├──────────────────┬──────────────────┬─────────────────────────────┤
│    Orchid 🌸     │     Lotus 🪷     │         Ivy 🌿              │
│  (API Polling)   │  (Data Mapping)  │ (Entity/Relationship Merge) │
└────────┬─────────┴──────────┬───────┴─────────────┬───────────────┘
         │                    │                     │
         └────────────────────┼─────────────────────┘
                              ▼
         ┌──────────────────────────────────────────────┐
         │                 Stem 🌱                      │
         ├──────────────────────────────────────────────┤
         │  database │ tracing │ middleware │ context   │
         │                    startup                   │
         └──────────────────────────────────────────────┘
                              │
         ┌────────────────────┼────────────────────┐
         ▼                    ▼                    ▼
    PostgreSQL          OpenTelemetry         Echo HTTP
    (sqlx)              (OTLP/Console)        (Framework)
```

**Design Philosophy**:
- **Interface-driven**: All abstractions use interfaces for mockability and testing
- **Context-aware**: Request context flows through all operations
- **Observability-first**: Tracing and structured logging built-in
- **Multi-tenant ready**: Request context includes tenant/user isolation
- **Resilient**: Fibonacci backoff retry logic for service startup
- **Production-ready**: Used in production by Orchid, Lotus, and Ivy

## Packages

### database - Database Utilities

**Location**: `pkg/database/`

Provides a comprehensive database abstraction layer with transaction management, migrations, SQL builders, and JSONB support.

#### Key Features

- **DB Interface**: Wraps `sqlx.DB` with interface for mockability
- **Transaction Management**: Context-aware transaction handling
- **Migrations**: golang-migrate integration with auto-rollback and retry
- **SQL Builders**: PostgreSQL-optimized query builders
- **Generic JSONB**: Type-safe JSON column support

#### DB Interface

```go
type DB interface {
    // Connection management
    Unsafe() *sqlx.DB
    Ping() error
    Close() error
    SetMaxOpenConns(n int)
    SetMaxIdleConns(n int)
    SetConnMaxLifetime(d time.Duration)

    // Transaction management
    BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error)
    BeginTxx(ctx context.Context, opts *sql.TxOptions) (Tx, error)
    GetTx(ctx context.Context, opts *sql.TxOptions) (context.Context, Tx, error)

    // Query operations
    Get(dest interface{}, query string, args ...interface{}) error
    GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
    Select(dest interface{}, query string, args ...interface{}) error
    SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error

    // Execution
    Exec(query string, args ...interface{}) (sql.Result, error)
    ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
    NamedExec(query string, arg interface{}) (sql.Result, error)
    NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)

    // Prepared statements
    Preparex(query string) (*sqlx.Stmt, error)
    PreparexContext(ctx context.Context, query string) (*sqlx.Stmt, error)
    PrepareNamed(query string) (*sqlx.NamedStmt, error)
    PrepareNamedContext(ctx context.Context, query string) (*sqlx.NamedStmt, error)

    // Utilities
    BindNamed(query string, arg interface{}) (string, []interface{}, error)
    NamedQuery(query string, arg interface{}) (*sqlx.Rows, error)
    NamedQueryContext(ctx context.Context, query string, arg interface{}) (*sqlx.Rows, error)
    Rebind(query string) string
    DriverName() string
}
```

#### Transaction Management

Context-aware transaction handling with automatic cleanup:

```go
// Get or create transaction from context
ctx, tx, err := db.GetTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback() // Safe even if committed

// Use transaction
err = tx.ExecContext(ctx, "INSERT INTO users (id, name) VALUES ($1, $2)", userID, name)
if err != nil {
    return err // Auto-rollback via defer
}

return tx.Commit()
```

**Nested Transaction Support**:
```go
func outerFunc(ctx context.Context, db database.DB) error {
    ctx, tx, err := db.GetTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Inner function reuses the same transaction from context
    err = innerFunc(ctx, db)
    if err != nil {
        return err
    }

    return tx.Commit()
}

func innerFunc(ctx context.Context, db database.DB) error {
    ctx, tx, err := db.GetTx(ctx, nil) // Reuses existing transaction
    // No commit/rollback needed - handled by outer function
    return tx.ExecContext(ctx, "UPDATE users SET updated_at = NOW()")
}
```

#### Database Migrations

Wraps golang-migrate with Fibonacci retry and auto-rollback:

```go
import "github.com/Ramsey-B/stem/pkg/database"

// Run migrations with automatic retry
err := database.RunMigrations(
    db,
    "file:///app/db/migrations",
    logger,
    3, // maxAttempts
)
```

**Features**:
- Auto-rollback on migration failure
- Fibonacci backoff (1s, 1s, 2s, 3s, 5s...)
- Dirty state detection and recovery
- Progress logging

#### SQL Builder Wrappers

PostgreSQL-optimized SQL builders with type safety:

```go
import (
    "github.com/Ramsey-B/stem/pkg/database"
    "github.com/huandu/go-sqlbuilder"
)

// Insert with ON CONFLICT
ib := database.PostgreSQLInsertBuilder()
query, args := ib.
    InsertInto("users").
    Cols("id", "name", "email").
    Values(userID, name, email).
    SQL(database.OnConflict("id").DoUpdate(
        ib.Assign("name", database.Excluded("name")),
        ib.Assign("email", database.Excluded("email")),
    )).
    Build()

_, err := db.ExecContext(ctx, query, args...)

// Update with WHERE
ub := database.PostgreSQLUpdateBuilder()
query, args := ub.
    Update("users").
    Set(
        ub.Assign("name", newName),
        ub.Assign("updated_at", sqlbuilder.Raw("NOW()")),
    ).
    Where(ub.Equal("id", userID)).
    Build()

_, err := db.ExecContext(ctx, query, args...)

// Select with JOIN
sb := database.PostgreSQLSelectBuilder()
query, args := sb.
    Select("u.id", "u.name", "p.title").
    From("users u").
    Join("posts p", "u.id = p.user_id").
    Where(sb.Equal("u.tenant_id", tenantID)).
    OrderBy("p.created_at DESC").
    Limit(10).
    Build()

var results []UserPost
err := db.SelectContext(ctx, &results, query, args...)
```

#### Generic JSONB Type

Type-safe JSONB column support:

```go
import "github.com/Ramsey-B/stem/pkg/database"

type UserMetadata struct {
    Preferences map[string]string `json:"preferences"`
    Tags        []string          `json:"tags"`
}

type User struct {
    ID       string                       `db:"id"`
    Name     string                       `db:"name"`
    Metadata database.JSONB[UserMetadata] `db:"metadata"`
}

// Insert
user := User{
    ID:   "user-123",
    Name: "Alice",
    Metadata: database.JSONB[UserMetadata]{
        Data: UserMetadata{
            Preferences: map[string]string{"theme": "dark"},
            Tags:        []string{"admin", "verified"},
        },
    },
}

_, err := db.NamedExecContext(ctx, `
    INSERT INTO users (id, name, metadata)
    VALUES (:id, :name, :metadata)
`, user)

// Query
var user User
err := db.GetContext(ctx, &user, "SELECT * FROM users WHERE id = $1", "user-123")

// Access typed data
theme := user.Metadata.Data.Preferences["theme"] // "dark"
tags := user.Metadata.Data.Tags                  // ["admin", "verified"]
```

---

### tracing - OpenTelemetry Integration

**Location**: `pkg/tracing/`

Provides OpenTelemetry tracing integration with W3C Trace Context support and multiple exporters.

#### Key Features

- **Global Tracer Management**: Set/get global tracer instance
- **Span Helpers**: Simplified span creation and management
- **Context Extraction**: Extract trace ID, span ID, traceparent, tracestate
- **OTLP Exporter**: Production-ready OpenTelemetry Protocol exporter (gRPC/HTTP)
- **Console Exporter**: Development/testing exporter

#### Basic Usage

```go
import (
    "github.com/Ramsey-B/stem/pkg/tracing"
    "github.com/Ramsey-B/stem/pkg/tracing/exporters"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// Initialize tracer with OTLP exporter
func initTracing(serviceName string) error {
    exporter, err := exporters.NewOTLPExporter(exporters.OTLPConfig{
        Endpoint: "localhost:4317",
        Protocol: "grpc",
        Insecure: true,
    })
    if err != nil {
        return err
    }

    resource, err := resource.New(context.Background(),
        resource.WithAttributes(
            semconv.ServiceNameKey.String(serviceName),
        ),
    )
    if err != nil {
        return err
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithSampler(sdktrace.AlwaysSample()),
        sdktrace.WithResource(resource),
        sdktrace.WithBatcher(exporter),
    )

    otel.SetTracerProvider(tp)
    tracing.SetTracer(tp.Tracer(serviceName))

    return nil
}

// Create spans
func processRequest(ctx context.Context, userID string) error {
    ctx, span := tracing.StartSpan(ctx, "processRequest")
    defer span.End()

    // Span is automatically linked to parent trace
    span.SetAttributes(attribute.String("user.id", userID))

    // Child operations inherit trace context
    err := saveToDatabase(ctx, userID)
    if err != nil {
        return err
    }

    return nil
}

func saveToDatabase(ctx context.Context, userID string) error {
    ctx, span := tracing.StartSpan(ctx, "database.save")
    defer span.End()

    // Database operations...
    return nil
}
```

#### W3C Trace Context Support

Extract trace context for propagation across service boundaries:

```go
import "github.com/Ramsey-B/stem/pkg/tracing"

// Extract for Kafka headers
traceparent := tracing.GetTraceParent(ctx) // "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
tracestate := tracing.GetTraceState(ctx)   // "vendor1=value1,vendor2=value2"

headers := []kafka.Header{
    {Key: "traceparent", Value: []byte(traceparent)},
    {Key: "tracestate", Value: []byte(tracestate)},
}

// Extract for logging
traceID := tracing.GetTraceID(ctx) // "4bf92f3577b34da6a3ce929d0e0e4736"
spanID := tracing.GetSpanID(ctx)   // "00f067aa0ba902b7"

logger.WithFields(map[string]interface{}{
    "trace_id": traceID,
    "span_id": spanID,
}).Info("Request processed")
```

#### OTLP Exporter Configuration

```go
import "github.com/Ramsey-B/stem/pkg/tracing/exporters"

// Production configuration (gRPC with TLS)
config := exporters.OTLPConfig{
    Endpoint: "otel-collector.prod.example.com:4317",
    Protocol: "grpc",
    Insecure: false,
    Timeout:  30 * time.Second,
    Headers: map[string]string{
        "authorization": "Bearer " + apiKey,
    },
}

exporter, err := exporters.NewOTLPExporter(config)

// Development configuration (HTTP, insecure)
config := exporters.OTLPConfig{
    Endpoint: "localhost:4318",
    Protocol: "http",
    Insecure: true,
}

exporter, err := exporters.NewOTLPExporter(config)
```

**Default Configuration**:
- Endpoint: `localhost:4317`
- Protocol: `grpc`
- Insecure: `true`
- Timeout: `30s`

---

### middleware - Echo HTTP Middleware

**Location**: `pkg/middleware/`

Provides Echo framework middleware for context enrichment, authentication, error handling, and logging.

#### Available Middleware

| Middleware | Purpose |
|------------|---------|
| `Context()` | Extract request metadata (request ID, tenant ID, user ID, HTTP info) |
| `Authentication()` | OIDC/JWT authentication with token verification |
| `TestAuth()` | Development-only auth bypass using headers |
| `Logger()` | Structured request/response logging |
| `Error()` | Global error handler with trace context |

#### Context Middleware

Extracts request headers and enriches context:

```go
import (
    "github.com/Ramsey-B/stem/pkg/middleware"
    "github.com/labstack/echo/v4"
)

e := echo.New()
e.Use(middleware.Context())

// Handler can access context values
func handler(c echo.Context) error {
    ctx := c.Request().Context()

    requestID := ctxmiddleware.GetRequestID(ctx)  // From X-Request-ID or generated UUID
    tenantID := ctxmiddleware.GetTenantID(ctx)    // From X-Tenant-ID
    userID := ctxmiddleware.GetUserID(ctx)        // From X-User-ID

    // HTTP metadata
    method := ctxmiddleware.GetMethod(ctx)        // GET, POST, etc.
    route := ctxmiddleware.GetRoute(ctx)          // /api/v1/users/:id
    remoteIP := ctxmiddleware.GetRemoteIP(ctx)    // 192.168.1.1
    referer := ctxmiddleware.GetReferer(ctx)      // https://example.com

    return c.JSON(200, map[string]string{
        "request_id": requestID,
        "tenant_id": tenantID,
    })
}
```

**Extracted Headers**:
- `X-Request-ID` → Auto-generated UUID if missing
- `X-Tenant-ID` → Tenant identifier for multi-tenancy
- `X-User-ID` → User identifier
- HTTP metadata: Method, Route, Remote IP, Referer

#### Authentication Middleware

OIDC/JWT authentication with token verification:

```go
import (
    "github.com/Ramsey-B/stem/pkg/middleware"
    "github.com/labstack/echo/v4"
)

e := echo.New()

// Production: OIDC authentication
e.Use(middleware.Authentication(
    logger,
    "https://auth.example.com/realms/production", // issuerURL
    "my-client-id",                                 // clientID
))

// Handlers receive authenticated context
func handler(c echo.Context) error {
    ctx := c.Request().Context()

    userID := ctxmiddleware.GetUserID(ctx)     // JWT subject claim
    tenantID := ctxmiddleware.GetTenantID(ctx) // First role from realm_access

    // User is authenticated, proceed with business logic
    return c.JSON(200, map[string]string{"user_id": userID})
}
```

**How it works**:
1. Extracts `Authorization: Bearer <token>` header
2. Verifies token against OIDC provider (issuerURL)
3. Validates audience matches clientID
4. Extracts claims:
   - `sub` → User ID
   - `email` → Email address
   - `realm_access.roles[0]` → Tenant ID (first role)
5. Enriches request context with user/tenant info
6. Creates trace span for auth verification

**Test Authentication (Development Only)**:

```go
// Development: Bypass auth, use headers
e.Use(middleware.TestAuth())

// Headers required:
// X-Tenant-ID: tenant-123
// X-User-ID: user-456
```

#### Logger Middleware

Structured request/response logging:

```go
import (
    "github.com/Ramsey-B/stem/pkg/middleware"
    "github.com/labstack/echo/v4"
)

e := echo.New()
e.Use(middleware.Logger(logger))

// Logs every request with:
// - method, uri, status, route
// - remote_ip, user_agent
// - response_time (duration)
// - response_size (bytes)
// - request_id, tenant_id, user_id
// - trace_id, span_id
```

**Example Log Output**:
```json
{
  "level": "info",
  "method": "POST",
  "uri": "/api/v1/entities",
  "status": 201,
  "route": "/api/v1/entities",
  "remote_ip": "192.168.1.100",
  "user_agent": "curl/7.81.0",
  "response_time": "45ms",
  "response_size": 1024,
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "tenant_id": "acme-corp",
  "user_id": "user-123",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "message": "request completed"
}
```

#### Error Middleware

Global error handler with trace context:

```go
import (
    "github.com/Ramsey-B/stem/pkg/middleware"
    "github.com/labstack/echo/v4"
)

e := echo.New()
e.HTTPErrorHandler = middleware.Error(logger)

// Error response format:
type ErrorResponse struct {
    RequestID string                 `json:"request_id"`
    TraceID   string                 `json:"trace_id"`
    Error     string                 `json:"error"`
    Code      string                 `json:"code,omitempty"`
    Details   map[string]interface{} `json:"details,omitempty"`
}
```

**Example Error Response**:
```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "error": "Entity not found",
  "code": "ENTITY_NOT_FOUND",
  "details": {
    "entity_id": "entity-123",
    "entity_type": "user"
  }
}
```

#### Complete Middleware Stack Example

```go
import (
    "github.com/Ramsey-B/stem/pkg/middleware"
    stemmiddleware "github.com/Ramsey-B/stem/pkg/middleware"
    "github.com/labstack/echo/v4"
    echomiddleware "github.com/labstack/echo/v4/middleware"
    "go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

e := echo.New()

// Error handling
e.HTTPErrorHandler = stemmiddleware.Error(logger)

// Standard middleware
e.Use(echomiddleware.Recover())
e.Use(otelecho.Middleware("my-service"))
e.Use(echomiddleware.RequestID())
e.Use(stemmiddleware.Context())
e.Use(stemmiddleware.Logger(logger))
e.Use(echomiddleware.Gzip())

// Authentication (choose one)
if cfg.AuthEnabled {
    e.Use(stemmiddleware.Authentication(logger, cfg.AuthIssuerURL, cfg.AuthClientID))
} else {
    e.Use(stemmiddleware.TestAuth()) // Development only
}

// CORS
e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
    AllowOrigins: []string{"https://app.example.com"},
    AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
}))

e.Use(echomiddleware.Secure())
```

---

### context - Request Context Management

**Location**: `pkg/context/`

Provides type-safe context key/value management for request metadata.

#### Available Context Values

| Key | Type | Purpose |
|-----|------|---------|
| `RequestID` | `string` | Unique request tracking ID (UUID) |
| `TenantID` | `string` | Multi-tenant isolation identifier |
| `UserID` | `string` | Authenticated user identifier |
| `Method` | `string` | HTTP method (GET, POST, etc.) |
| `Route` | `string` | HTTP route pattern |
| `RemoteIP` | `string` | Client IP address |
| `Referer` | `string` | HTTP Referer header |

#### Usage

```go
import (
    ctxpkg "github.com/Ramsey-B/stem/pkg/context"
)

// Set values (typically done by middleware)
ctx = ctxpkg.WithRequestID(ctx, "550e8400-e29b-41d4-a716-446655440000")
ctx = ctxpkg.WithTenantID(ctx, "acme-corp")
ctx = ctxpkg.WithUserID(ctx, "user-123")

// Get values (in handlers, services, repositories)
requestID := ctxpkg.GetRequestID(ctx)  // Returns "" if not set
tenantID := ctxpkg.GetTenantID(ctx)    // Returns "" if not set
userID := ctxpkg.GetUserID(ctx)        // Returns "" if not set

// HTTP metadata
method := ctxpkg.GetMethod(ctx)        // GET, POST, etc.
route := ctxpkg.GetRoute(ctx)          // /api/v1/users/:id
remoteIP := ctxpkg.GetRemoteIP(ctx)    // 192.168.1.1
referer := ctxpkg.GetReferer(ctx)      // https://example.com
```

**Type Safety**: All getters return empty strings if values are not set, preventing nil pointer panics.

---

### startup - Service Orchestration

**Location**: `pkg/startup/`

Manages service startup with dependency ordering, Fibonacci backoff retry, and graceful shutdown.

#### Key Features

- **Dependency Management**: Topological ordering based on dependencies
- **Resilient Startup**: Fibonacci backoff retry (1s, 1s, 2s, 3s, 5s, 8s...)
- **Status Tracking**: Per-service status (Pending, Started, Stopped, Failed)
- **Graceful Shutdown**: Reverse-order shutdown respecting dependencies
- **Context Cancellation**: Cancellable startup with context
- **Structured Logging**: Detailed logging of each startup attempt

#### StartupDependency Interface

```go
type StartupDependency interface {
    GetName() string
    DependsOn() []string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

#### Basic Usage

```go
import "github.com/Ramsey-B/stem/pkg/startup"

// Create startup manager
mgr := startup.NewManager(logger, 5) // 5 max retry attempts

// Define dependencies
type DatabaseStartup struct {
    db *sqlx.DB
}

func (d *DatabaseStartup) GetName() string { return "database" }
func (d *DatabaseStartup) DependsOn() []string { return nil }
func (d *DatabaseStartup) Start(ctx context.Context) error {
    return d.db.Ping()
}
func (d *DatabaseStartup) Stop(ctx context.Context) error {
    return d.db.Close()
}

type MigrationStartup struct {
    db database.DB
}

func (m *MigrationStartup) GetName() string { return "migrations" }
func (m *MigrationStartup) DependsOn() []string { return []string{"database"} }
func (m *MigrationStartup) Start(ctx context.Context) error {
    return database.RunMigrations(m.db, "file:///app/db/migrations", logger, 3)
}
func (m *MigrationStartup) Stop(ctx context.Context) error { return nil }

// Register dependencies
mgr.AddDependency(&DatabaseStartup{db: db})
mgr.AddDependency(&MigrationStartup{db: db})

// Start all (automatically orders by dependencies)
ctx := context.Background()
err := mgr.Start(ctx)
if err != nil {
    log.Fatalf("Startup failed: %v", err)
}

// Graceful shutdown (reverse order)
defer mgr.Stop(ctx)
```

#### Complex Dependency Example

```go
// Orchid service startup with multiple dependencies

type RedisStartup struct {
    client *redis.Client
}

func (r *RedisStartup) GetName() string { return "redis" }
func (r *RedisStartup) DependsOn() []string { return nil }
func (r *RedisStartup) Start(ctx context.Context) error {
    return r.client.Ping(ctx).Err()
}
func (r *RedisStartup) Stop(ctx context.Context) error {
    return r.client.Close()
}

type KafkaStartup struct {
    producer *kafka.Producer
}

func (k *KafkaStartup) GetName() string { return "kafka" }
func (k *KafkaStartup) DependsOn() []string { return nil }
func (k *KafkaStartup) Start(ctx context.Context) error {
    return k.producer.Ping()
}
func (k *KafkaStartup) Stop(ctx context.Context) error {
    return k.producer.Close()
}

type ServicesStartup struct {
    scheduler *scheduler.Scheduler
}

func (s *ServicesStartup) GetName() string { return "services" }
func (s *ServicesStartup) DependsOn() []string {
    return []string{"database", "migrations", "redis", "kafka"}
}
func (s *ServicesStartup) Start(ctx context.Context) error {
    return s.scheduler.Start(ctx)
}
func (s *ServicesStartup) Stop(ctx context.Context) error {
    return s.scheduler.Stop(ctx)
}

// Register in any order - startup manager handles dependency ordering
mgr := startup.NewManager(logger, 5)
mgr.AddDependency(&DatabaseStartup{db: db})
mgr.AddDependency(&ServicesStartup{scheduler: sched})
mgr.AddDependency(&KafkaStartup{producer: producer})
mgr.AddDependency(&MigrationStartup{db: db})
mgr.AddDependency(&RedisStartup{client: redisClient})

// Actual startup order (automatically determined):
// 1. database, redis, kafka (parallel - no dependencies)
// 2. migrations (depends on database)
// 3. services (depends on all others)

err := mgr.Start(ctx)
```

#### Fibonacci Backoff Retry

The startup manager uses Fibonacci backoff for retries:

```
Attempt 1: Immediate
Attempt 2: Wait 1s
Attempt 3: Wait 1s
Attempt 4: Wait 2s
Attempt 5: Wait 3s
Attempt 6: Wait 5s
Attempt 7: Wait 8s
```

**Example Logs**:
```
INFO  Starting dependency: database (attempt 1/5)
ERROR Failed to start database: connection refused (attempt 1/5)
INFO  Retrying in 1s...
INFO  Starting dependency: database (attempt 2/5)
INFO  Successfully started: database
```

#### Graceful Shutdown

Shutdown happens in reverse dependency order:

```go
// Startup order:
// database → migrations → redis → kafka → services

// Shutdown order (reverse):
// services → kafka → redis → migrations → database

defer mgr.Stop(context.Background())
```

---

## Usage

### In Other Projects (Orchid, Lotus, Ivy)

```go
import (
    "github.com/Ramsey-B/stem/pkg/database"
    "github.com/Ramsey-B/stem/pkg/tracing"
    "github.com/Ramsey-B/stem/pkg/middleware"
    ctxpkg "github.com/Ramsey-B/stem/pkg/context"
    "github.com/Ramsey-B/stem/pkg/startup"
)
```

### Local Development with Go Workspaces

Create a `go.work` file in the parent directory:

```go
go 1.24

use (
    ./stem
    ./orchid
    ./lotus
    ./ivy
)
```

This allows you to develop all projects together without publishing stem.

**Benefits**:
- Edit stem code and see changes immediately in all services
- No need to publish stem versions during development
- Easier debugging across service boundaries
- Simplified local testing

### Production Usage

In production, services import stem as a versioned Go module:

```go
// go.mod
module github.com/Ramsey-B/orchid

go 1.24

require (
    github.com/Ramsey-B/stem v1.24.1
)
```

---

## Installation

```bash
go get github.com/Ramsey-B/stem
```

---

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/Gobusters/ectoerror` | v1.0.0 | Structured error handling |
| `github.com/Gobusters/ectologger` | v0.0.1 | Structured logging interface |
| `github.com/coreos/go-oidc/v3` | v3.14.1 | OpenID Connect authentication |
| `github.com/golang-migrate/migrate/v4` | v4.18.2 | Database migrations |
| `github.com/google/uuid` | v1.6.0 | UUID generation |
| `github.com/huandu/go-sqlbuilder` | v1.35.0 | SQL query builder |
| `github.com/jmoiron/sqlx` | v1.4.0 | SQL extensions |
| `github.com/labstack/echo/v4` | v4.13.3 | HTTP web framework |
| `github.com/pkg/errors` | v0.9.1 | Error wrapping |
| `go.opentelemetry.io/otel` | v1.39.0 | OpenTelemetry tracing core |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace` | v1.39.0 | OTLP trace exporter |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | v1.39.0 | OTLP gRPC exporter |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` | v1.39.0 | OTLP HTTP exporter |
| `go.opentelemetry.io/otel/sdk` | v1.39.0 | OpenTelemetry SDK |
| `google.golang.org/grpc` | v1.77.0 | gRPC framework |

---

## Examples

### Complete Service Setup

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/Ramsey-B/stem/pkg/database"
    "github.com/Ramsey-B/stem/pkg/middleware"
    ctxpkg "github.com/Ramsey-B/stem/pkg/context"
    "github.com/Ramsey-B/stem/pkg/startup"
    "github.com/Ramsey-B/stem/pkg/tracing"
    "github.com/Ramsey-B/stem/pkg/tracing/exporters"
    "github.com/labstack/echo/v4"
    echomiddleware "github.com/labstack/echo/v4/middleware"
    "go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

func main() {
    // Initialize logger (using your preferred logging library)
    logger := ... // ectologger implementation

    // Initialize tracing
    exporter, _ := exporters.NewOTLPExporter(exporters.OTLPConfig{
        Endpoint: "localhost:4317",
        Protocol: "grpc",
        Insecure: true,
    })
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
    )
    otel.SetTracerProvider(tp)
    tracing.SetTracer(tp.Tracer("my-service"))

    // Initialize database
    db, _ := database.New("postgres://user:pass@localhost/db")

    // Setup startup manager
    mgr := startup.NewManager(logger, 5)
    mgr.AddDependency(&DatabaseStartup{db: db})
    mgr.AddDependency(&MigrationStartup{db: db})

    ctx := context.Background()
    err := mgr.Start(ctx)
    if err != nil {
        log.Fatalf("Startup failed: %v", err)
    }
    defer mgr.Stop(ctx)

    // Setup Echo server
    e := echo.New()
    e.HTTPErrorHandler = middleware.Error(logger)
    e.Use(echomiddleware.Recover())
    e.Use(otelecho.Middleware("my-service"))
    e.Use(echomiddleware.RequestID())
    e.Use(middleware.Context())
    e.Use(middleware.Logger(logger))
    e.Use(middleware.Authentication(logger, "https://auth.example.com", "client-id"))

    // Register routes
    e.GET("/api/v1/entities/:id", getEntity)

    // Start server
    e.Start(":8080")
}

func getEntity(c echo.Context) error {
    ctx := c.Request().Context()

    // Access context values
    tenantID := ctxpkg.GetTenantID(ctx)
    userID := ctxpkg.GetUserID(ctx)

    // Create trace span
    ctx, span := tracing.StartSpan(ctx, "getEntity")
    defer span.End()

    // Get database from DI
    db := ... // from dependency injection

    // Use transaction
    ctx, tx, err := db.GetTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Query
    var entity Entity
    err = tx.GetContext(ctx, &entity,
        "SELECT * FROM entities WHERE id = $1 AND tenant_id = $2",
        c.Param("id"), tenantID,
    )
    if err != nil {
        return echo.NewHTTPError(http.StatusNotFound, "Entity not found")
    }

    tx.Commit()

    return c.JSON(http.StatusOK, entity)
}
```

---

## License

[Your License Here]

## Contributing

[Your Contributing Guidelines Here]
