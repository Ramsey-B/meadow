# Implementation Plan Updates - Leveraging Existing Infrastructure

This document outlines the updates needed to the implementation plan to leverage existing infrastructure.

## Current Implementation Status

### Completed Components ✅

| Component               | Location                                 | Status      |
| ----------------------- | ---------------------------------------- | ----------- |
| Database Schema         | `db/pg/0003_core_schema.up.sql`          | ✅ Complete |
| Citus Distribution      | `db/pg/0004_distribute_by_tenant.up.sql` | ✅ Complete |
| Foreign Keys            | `db/pg/0005_foreign_keys.up.sql`         | ✅ Complete |
| Redis Client            | `pkg/redis/client.go`                    | ✅ Complete |
| Redis Streams           | `pkg/redis/streams.go`                   | ✅ Complete |
| Distributed Locks       | `pkg/redis/lock.go`                      | ✅ Complete |
| Rate Limiting           | `pkg/redis/ratelimit.go`                 | ✅ Complete |
| Kafka Producer          | `pkg/kafka/producer.go`                  | ✅ Complete |
| JMESPath Evaluator      | `pkg/expressions/evaluator.go`           | ✅ Complete |
| Expression Context      | `pkg/expressions/context.go`             | ✅ Complete |
| Template Processor      | `pkg/expressions/template.go`            | ✅ Complete |
| Database Models (8)     | `pkg/models/*.go`                        | ✅ Complete |
| Step Model              | `pkg/models/step.go`                     | ✅ Complete |
| Repositories (8)        | `pkg/repositories/*.go`                  | ✅ Complete |
| Repository Interfaces   | `pkg/repositories/interfaces.go`         | ✅ Complete |
| SQL Builder Integration | `pkg/database/sqlbuilder.go`             | ✅ Complete |
| HTTP Client             | `pkg/httpclient/client.go`               | ✅ Complete |
| Request Builder         | `pkg/httpclient/builder.go`              | ✅ Complete |
| Response Parser         | `pkg/httpclient/parser.go`               | ✅ Complete |
| Execution Context       | `pkg/execution/context.go`               | ✅ Complete |
| Step Executor           | `pkg/execution/executor.go`              | ✅ Complete |
| Condition Evaluator     | `pkg/execution/conditions.go`            | ✅ Complete |
| Fanout Handler          | `pkg/execution/fanout.go`                | ✅ Complete |
| DI Registration         | `cmd/startup.go`                         | ✅ Complete |

### Pending Components 🔲

| Component           | Location                         | Status         |
| ------------------- | -------------------------------- | -------------- |
| Plan Executor       | `pkg/execution/plan_executor.go` | 🔲 Not Started |
| Queue Processor     | `pkg/queue/processor.go`         | 🔲 Not Started |
| Job Scheduler       | `pkg/scheduler/scheduler.go`     | 🔲 Not Started |
| Rate Limit Manager  | `pkg/ratelimit/manager.go`       | 🔲 Not Started |
| Auth Flow Executor  | `pkg/auth/executor.go`           | 🔲 Not Started |
| Token Management    | `pkg/auth/token.go`              | 🔲 Not Started |
| REST API Handlers   | `internal/handlers/*.go`         | 🔲 Not Started |
| Statistics Tracking | `pkg/statistics/`                | 🔲 Not Started |
| Prometheus Metrics  | `pkg/metrics/`                   | 🔲 Not Started |
| Mock API Framework  | `test/mocks/`                    | 🔲 Not Started |

---

## Key Infrastructure to Leverage

### 1. Dependency Injection (`ectoinject`)

- **Location**: `cmd/dependency.go`, `cmd/startup.go`
- **Usage**: Register all services, repositories, and clients in DI container
- **Pattern**: Use `ectoinject.RegisterInstance` for all components
- **Already Registered**:
  - `database.DB`
  - `*redis.Client`
  - `*kafka.Producer`
  - `*expressions.Evaluator`
  - All 8 repository interfaces

### 2. Logging (`ectologger.Logger`)

- **Location**: Already configured in `cmd/main.go`
- **Features**:
  - Structured logging with Zap
  - Automatic trace ID, span ID, request ID injection
  - Context-aware logging
- **Usage**: Inject `ectologger.Logger` via DI everywhere, use `logger.WithContext(ctx)` for context-aware logs

### 3. Tracing (`pkg/tracing`)

- **Location**: `pkg/tracing/tracing.go`
- **Features**:
  - `StartSpan(ctx, name)` - Create new spans
  - `GetTraceID(ctx)` - Get trace ID
  - `GetSpanID(ctx)` - Get span ID
  - `GetActiveSpan(ctx)` - Get current span
- **Usage**: Create spans for all major operations (HTTP calls, plan execution, queue processing)

### 4. Environment Config (`ectoenv`)

- **Location**: `config/config.go`
- **Features**: Auto-refreshing config (updates every minute)
- **Already Added**:
  - Redis config (`RedisHost`, `RedisPort`, `RedisPassword`, `RedisDB`)
  - Kafka config (`KafkaBrokers`, `KafkaResponseTopic`)
- **Still Needed**:
  - Scheduler config (poll interval)
  - Execution limits (timeout, nesting depth)

### 5. Startup System (`pkg/startup`)

- **Location**: `pkg/startup/startup.go`, `cmd/startup.go`
- **Features**: Dependency-based startup with Fibonacci backoff
- **Already Implemented**:
  - `databaseStartup`
  - `migrateStartup`
  - `redisStartup`
  - `kafkaStartup`
  - `servicesStartup` (repositories, expression evaluator)
- **Still Needed**:
  - `queueProcessorStartup`
  - `schedulerStartup`

---

## SQL Builder Pattern

All repositories use `github.com/huandu/go-sqlbuilder` for query construction:

```go
import "github.com/Ramsey-B/orchid/pkg/database"

// Create a struct builder for the model
sb := database.NewStruct(new(models.Integration))

// Build SELECT query with tenant filter
selectBuilder := sb.SelectFrom("integrations")
selectBuilder.Where(selectBuilder.Equal("tenant_id", tenantID))
selectBuilder.Where(selectBuilder.Equal("id", id))
sql, args := selectBuilder.Build()

// Build INSERT query
insertBuilder := sb.InsertInto("integrations", model)
sql, args := insertBuilder.Build()

// Build UPDATE query
updateBuilder := sb.Update("integrations", model)
updateBuilder.Where(updateBuilder.Equal("tenant_id", tenantID))
updateBuilder.Where(updateBuilder.Equal("id", id))
sql, args := updateBuilder.Build()
```

---

## Repository Logging Pattern

All repositories log write operations:

```go
func (r *IntegrationRepository) Create(ctx context.Context, model *models.Integration) error {
    tenantID, err := r.GetTenantID(ctx)
    if err != nil {
        return err
    }
    model.TenantID = tenantID

    // ... execute query ...

    r.LogCreate(ctx, "integrations", model.ID.String())
    return nil
}

func (r *IntegrationRepository) Update(ctx context.Context, model *models.Integration) error {
    // ... execute query ...
    r.LogUpdate(ctx, "integrations", model.ID.String())
    return nil
}

func (r *IntegrationRepository) Delete(ctx context.Context, id uuid.UUID) error {
    // ... execute query ...
    r.LogDelete(ctx, "integrations", id.String())
    return nil
}
```

---

## Multi-Tenancy Implementation Status

### Database Level ✅ Complete

- All tables have `tenant_id UUID NOT NULL`
- Primary keys: `PRIMARY KEY (tenant_id, id)`
- Unique constraints include `tenant_id`
- Tables distributed by `tenant_id` using Citus
- Foreign keys include `tenant_id` in same ordinal position

### Repository Level ✅ Complete

- All repositories extract `tenant_id` from context
- All queries filter by `tenant_id`
- All inserts set `tenant_id`
- Helper: `GetTenantID(ctx)` in base repository

### API Level 🔲 Not Started

- Need tenant validation middleware
- Need to validate tenant ownership on resource access
- Need to return 403 (not 404) for cross-tenant access

### Queue/Execution Level 🟡 Partial

- `ExecutionContext` includes `TenantID` field
- Job message structure needs `tenant_id`
- Redis keys need tenant prefix pattern

---

## Config Additions Needed

Add to `config/config.go`:

```go
// Scheduler (NOT YET ADDED)
SchedulerPollInterval time.Duration `env:"SCHEDULER_POLL_INTERVAL" env-default:"30s"`

// Execution Limits (NOT YET ADDED)
DefaultExecutionTimeout time.Duration `env:"DEFAULT_EXECUTION_TIMEOUT" env-default:"5m"`
MaxRetryBackoff time.Duration `env:"MAX_RETRY_BACKOFF" env-default:"60s"`
MaxNestingDepth int `env:"MAX_NESTING_DEPTH" env-default:"5"`
MaxResponseSize int64 `env:"MAX_RESPONSE_SIZE" env-default:"10485760"` // 10MB
MaxRequestBodySize int64 `env:"MAX_REQUEST_BODY_SIZE" env-default:"5242880"` // 5MB

// Redis Streams (NOT YET ADDED)
RedisStreamsConsumerGroup string `env:"REDIS_STREAMS_CONSUMER_GROUP" env-default:"orchid-workers"`
RedisStreamsConsumerName string `env:"REDIS_STREAMS_CONSUMER_NAME" env-default:""`
```

---

## DI Container Registration Summary

Current registrations in `cmd/startup.go`:

```go
// Database
ectoinject.RegisterInstance[database.DB](container, db)

// Redis
ectoinject.RegisterInstance[*redis.Client](container, redisClient)

// Kafka
ectoinject.RegisterInstance[*kafka.Producer](container, kafkaProducer)

// Expression Evaluator
ectoinject.RegisterInstance[*expressions.Evaluator](container, evaluator)

// Repositories (all 8)
ectoinject.RegisterInstance[repositories.IntegrationRepo](container, integrationRepo)
ectoinject.RegisterInstance[repositories.ConfigSchemaRepo](container, configSchemaRepo)
ectoinject.RegisterInstance[repositories.ConfigRepo](container, configRepo)
ectoinject.RegisterInstance[repositories.AuthFlowRepo](container, authFlowRepo)
ectoinject.RegisterInstance[repositories.PlanRepo](container, planRepo)
ectoinject.RegisterInstance[repositories.PlanExecutionRepo](container, planExecutionRepo)
ectoinject.RegisterInstance[repositories.PlanContextRepo](container, planContextRepo)
ectoinject.RegisterInstance[repositories.PlanStatisticsRepo](container, planStatisticsRepo)
```

**Still Needed:**

```go
// HTTP Client
ectoinject.RegisterInstance[*httpclient.Client](container, httpClient)

// Step Executor
ectoinject.RegisterInstance[execution.StepExecutor](container, stepExecutor)

// Plan Executor (when implemented)
ectoinject.RegisterInstance[execution.PlanExecutor](container, planExecutor)

// Queue Processor (when implemented)
ectoinject.RegisterInstance[queue.Processor](container, queueProcessor)

// Scheduler (when implemented)
ectoinject.RegisterInstance[scheduler.Scheduler](container, scheduler)
```

---

## Docker Compose Services

Current services in `docker-compose.dev.yml`:

- ✅ `db` - PostgreSQL with Citus
- ✅ `redis` - Redis
- ✅ `zookeeper` - Zookeeper (for Kafka)
- ✅ `kafka` - Kafka
- ✅ `dev` - Development container

---

## Next Steps (Recommended Order)

1. **Phase 4.1: Plan Executor** (`pkg/execution/plan_executor.go`)

   - Orchestrate full plan execution
   - Handle auth flow integration
   - Handle while loops
   - Context persistence
   - Statistics tracking
   - Kafka emission

2. **Phase 4.2: Queue Job Processor** (`pkg/queue/processor.go`)

   - Redis Streams consumer
   - Job message structure with tenant_id
   - Consumer groups for horizontal scaling
   - Job acknowledgment and retry

3. **Phase 4.3: Job Scheduler** (`pkg/scheduler/scheduler.go`)

   - Poll database for due plans
   - Check wait intervals
   - Enqueue jobs to Redis Streams
   - Graceful shutdown

4. **Phase 6: Auth Flow Management** (`pkg/auth/`)

   - Token extraction and caching
   - Token refresh logic
   - Auth integration with plan executor

5. **Phase 7: API Endpoints** (`internal/handlers/`)
   - Plan CRUD API
   - Config CRUD API
   - Execution status API
   - Tenant validation middleware

---

## Summary

All new components should:

1. ✅ Use DI container for dependencies
2. ✅ Use `ectologger.Logger` (never create new loggers)
3. ✅ Use `pkg/tracing` for all tracing
4. ✅ Add config to `config.Config` with `ectoenv` tags
5. ✅ Create `StartupDependency` implementations for services that need startup
6. ✅ Register everything in DI container
7. ✅ Follow existing patterns in the codebase
8. ✅ Use SQL builder for queries (`github.com/huandu/go-sqlbuilder`)
9. ✅ Log write operations (Create, Update, Delete)
10. ✅ **Implement tenant isolation at all layers (database, repository, API, queue)**
11. ✅ **Extract tenant_id from context using `GetTenantID(ctx)`**
12. ✅ **Validate tenant ownership for all resource access operations**

---

## Future: Ivy Integration Requirements

For Ivy (entity merging service) to implement execution-based deletions, Orchid needs to emit execution lifecycle events:

### Required: Execution Completed Event

Emit from `PlanExecutor.ExecutePlan()` after all steps finish:

```json
{
  "event_type": "execution.completed",
  "tenant_id": "tenant-123",
  "plan_id": "plan-456",
  "execution_id": "exec-789",
  "status": "success",
  "stats": {
    "total_entities": 150,
    "entity_types": ["person", "company"],
    "duration_ms": 5432
  },
  "timestamp": "2024-01-15T10:05:00Z"
}
```

**Why Ivy needs this:**
- Detect absent entities (entities not in current execution = potentially deleted)
- Apply grace periods before deletion
- Track data freshness per source plan

**Implementation TODO:**
- [ ] Add `execution.completed` event emission to `PlanExecutor`
- [ ] Include entity type counts in stats
- [ ] Emit on both success and failure (with appropriate status)
