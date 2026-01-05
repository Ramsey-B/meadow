# Orchid Implementation Plan

This document outlines the implementation plan for building Orchid, a horizontally scalable API polling microservice.

**⚠️ IMPORTANT: See `IMPLEMENTATION_PLAN_UPDATES.md` for details on leveraging existing infrastructure:**

- Dependency Injection (`ectoinject`)
- Logging (`ectologger.Logger`)
- Tracing (`pkg/tracing`)
- Environment Config (`ectoenv`)
- Startup System (`pkg/startup`)

All new components must use these existing systems rather than creating new ones.

## Overview

**Architecture Decisions:**

- Expression Language: JMESPath everywhere
- Queue System: Redis Streams (job queue), Kafka (data emission)
- Database: PostgreSQL (Citus) for persistent state, Redis for ephemeral state
- Language: Go

**Key Components:**

1. Database models and migrations
2. JMESPath expression evaluator
3. Plan/step execution engine
4. Queue system (Redis Streams)
5. Rate limiting system
6. Auth flow management
7. Kafka producer
8. REST API endpoints
9. Observability (metrics, logging, tracing)
10. **Multi-tenancy support** - Tenant isolation and data segregation

---

## Implementation Status Summary

| Phase | Status | Notes |
|-------|--------|-------|
| Phase 1: Foundation & Infrastructure | ✅ Complete | DB, Redis, Kafka, JMESPath |
| Phase 2: Core Models & Repositories | ✅ Complete | All models + repos with SQL builder |
| Phase 3: Execution Engine Core | ✅ Complete | HTTP client, executor, conditions, fanout |
| Phase 4: Plan Execution Orchestration | 🔲 Not Started | Plan executor, queue processor, scheduler |
| Phase 5: Rate Limiting | 🔲 Not Started | Base Redis client done |
| Phase 6: Auth Flow Management | 🔲 Not Started | |
| Phase 7: API Endpoints | 🔲 Not Started | |
| Phase 7.5: Multi-Tenancy Implementation | 🟡 Partial | DB done, repos done, API pending |
| Phase 8: Observability & Operations | 🔲 Not Started | Base tracing/logging exists |
| Phase 9: Testing & Documentation | 🔲 Not Started | One integration test exists |
| Phase 10: Performance & Scale Testing | 🔲 Not Started | |

---

## Phase 1: Foundation & Infrastructure ✅ COMPLETE

### 1.1 Database Schema & Migrations ✅

**Priority: Critical** | **Status: Complete**

**Tasks:**

- [x] Create database migrations for core tables:
  - `integrations` - Integration definitions
  - `config_schemas` - Config schema definitions
  - `configs` - Config instances with values
  - `plans` - Plan definitions (JSONB for plan structure)
  - `plan_executions` - Execution history and status
  - `plan_contexts` - Context storage per plan/config
  - `plan_statistics` - Aggregated statistics
  - `auth_flows` - Auth flow definitions
- [x] **Add tenant_id column to all tenant-scoped tables** (composite PKs: `PRIMARY KEY (tenant_id, id)`)
- [x] Add indexes for tenant_id + common queries
- [x] Add JSONB columns for flexible plan/step definitions
- [x] **Create migration for Citus distribution by tenant_id** (`0004_distribute_by_tenant.up.sql`)
- [x] **Add foreign key constraints** (`0005_foreign_keys.up.sql` - added after distribution)
- [ ] Add Row Level Security (RLS) policies for tenant isolation (optional, deferred)

**Deliverables:**

- ✅ Migration files in `db/pg/` (0001-0005)
- ✅ Database models in `pkg/models/`

### 1.2 Redis Integration ✅

**Priority: Critical** | **Status: Complete**

**Tasks:**

- [x] Set up Redis client library (`go-redis/v9`)
- [x] Create Redis connection management (`pkg/redis/client.go`)
- [x] Implement Redis Streams consumer/producer (`pkg/redis/streams.go`)
- [x] Create Redis helpers for:
  - Locks (distributed locking) - `pkg/redis/lock.go`
  - Rate limit counters - `pkg/redis/ratelimit.go`
- [x] Add Redis to config and dependency injection

**Deliverables:**

- ✅ `pkg/redis/` package with Streams, locks, rate limiting helpers
- ✅ Redis config in `config/config.go`

### 1.3 Kafka Producer ✅

**Priority: Critical** | **Status: Complete**

**Tasks:**

- [x] Set up Kafka producer client (`segmentio/kafka-go`)
- [x] Create Kafka connection management
- [x] Implement message producer for raw API responses
- [x] Add Kafka config to `config/config.go`
- [x] Implement message serialization (JSON)
- [x] Add error handling and retry logic

**Deliverables:**

- ✅ `pkg/kafka/producer.go`
- ✅ Kafka config in `config/config.go`

### 1.4 JMESPath Expression Evaluator ✅

**Priority: Critical** | **Status: Complete**

**Tasks:**

- [x] Add JMESPath library (`github.com/jmespath/go-jmespath`)
- [x] Create expression evaluator wrapper (`pkg/expressions/evaluator.go`)
- [x] Implement context builder (`pkg/expressions/context.go`)
- [x] Add string template support (`pkg/expressions/template.go`)
- [x] Add compiled expression caching
- [x] Add error handling for invalid expressions

**Deliverables:**

- ✅ `pkg/expressions/` package with JMESPath evaluator
- ✅ Expression context builder
- ✅ Template processor for string interpolation

---

## Phase 2: Core Models & Repositories ✅ COMPLETE

### 2.1 Database Models ✅

**Priority: Critical** | **Status: Complete**

**Tasks:**

- [x] Create model structs:
  - `Integration` - `pkg/models/integration.go`
  - `ConfigSchema` - `pkg/models/config_schema.go`
  - `Config` - `pkg/models/config.go`
  - `Plan` (with nested Step structure) - `pkg/models/plan.go`
  - `PlanExecution` - `pkg/models/plan_execution.go`
  - `PlanContext` - `pkg/models/plan_context.go`
  - `PlanStatistics` - `pkg/models/plan_statistics.go`
  - `AuthFlow` - `pkg/models/auth_flow.go`
  - `Step` - `pkg/models/step.go`
- [x] Add JSON tags for serialization
- [x] Add `db` tags for SQLX mapping
- [x] Add `TableName()` methods

**Deliverables:**

- ✅ Models in `pkg/models/`

### 2.2 Repository Layer ✅

**Priority: Critical** | **Status: Complete**

**Tasks:**

- [x] Create repository interfaces (`pkg/repositories/interfaces.go`):
  - `IntegrationRepo`
  - `ConfigSchemaRepo`
  - `ConfigRepo`
  - `PlanRepo`
  - `PlanExecutionRepo`
  - `PlanContextRepo`
  - `PlanStatisticsRepo`
  - `AuthFlowRepo`
- [x] Implement PostgreSQL repositories (all 8)
- [x] **Use SQL builder** (`github.com/huandu/go-sqlbuilder`)
- [x] **Add logging to write operations** (Create, Update, Delete)
- [x] **Add tenant context to all repository methods:**
  - All queries filter by `tenant_id` from context
  - Use `GetTenantID(ctx)` helper
  - Validate tenant_id is present before executing queries
- [x] Implement CRUD operations (all tenant-scoped)
- [x] Add query methods (FindByIntegrationID, etc.)

**Deliverables:**

- ✅ Repository interfaces in `pkg/repositories/interfaces.go`
- ✅ PostgreSQL implementations in `pkg/repositories/*_repository.go`
- ✅ Base repository with tenant helpers in `pkg/repositories/repository.go`

### 2.3 Plan/Step Structure Definition ✅

**Priority: Critical** | **Status: Complete**

**Tasks:**

- [x] Define JSON schema for plans/steps:
  - URL schema
  - Method
  - Headers (static + JMESPath)
  - Params (static + JMESPath)
  - Request body (static + JMESPath)
  - Conditions (while, abort_when, retry_when, break_when)
  - Sub-steps (with iterate_over)
  - Retry configuration
  - Timeout configuration
- [x] Create Go structs matching schema (`pkg/models/step.go`)

**Deliverables:**

- ✅ Plan/step structs in `pkg/models/plan.go` and `pkg/models/step.go`

---

## Phase 3: Execution Engine Core ✅ COMPLETE

### 3.1 HTTP Client & Request Builder ✅

**Priority: Critical** | **Status: Complete**

**Tasks:**

- [x] Create HTTP client wrapper (`pkg/httpclient/client.go`)
- [x] Implement request builder (`pkg/httpclient/builder.go`):
  - URL construction with JMESPath templating
  - Header injection (static + JMESPath)
  - Body construction (static + JMESPath)
  - Method handling
- [x] Add timeout support
- [x] Add request/response logging
- [x] Implement response parsing (`pkg/httpclient/parser.go`)
- [x] Handle JSON responses

**Deliverables:**

- ✅ `pkg/httpclient/` package
- ✅ Request builder with JMESPath support

### 3.2 Expression Context Builder ✅

**Priority: Critical** | **Status: Complete**

**Tasks:**

- [x] Create execution context structure (`pkg/execution/context.go`)
- [x] Implement context builder that assembles:
  - `Response` (status_code, headers, body)
  - `Prev` (previous response in loop)
  - `Parent` (parent step response)
  - `ContextData` (plan context)
  - `Config` (config values)
  - `Auth` (auth result)
  - `Item` (current item when iterating)
  - `TenantID`
- [x] Add context conversion to map for JMESPath

**Deliverables:**

- ✅ `pkg/execution/context.go`
- ✅ Context builder and ToMap converter

### 3.3 Step Executor ✅

**Priority: Critical** | **Status: Complete**

**Tasks:**

- [x] Create step executor interface
- [x] Implement single step execution (`pkg/execution/executor.go`):
  - Build request (URL, headers, body)
  - Execute HTTP request
  - Parse response
  - Evaluate conditions
  - Handle retries (Fibonacci backoff)
  - Handle timeouts
- [x] Add error handling and classification
- [x] Implement response size limits

**Deliverables:**

- ✅ `pkg/execution/executor.go`
- ✅ Step execution logic with retry handling

### 3.4 Condition Evaluator ✅

**Priority: Critical** | **Status: Complete**

**Tasks:**

- [x] Implement condition evaluation (`pkg/execution/conditions.go`):
  - `while` - continue loop if true
  - `abort_when` - stop execution if true
  - `retry_when` - retry if true
  - `break_when` - exit while loop if true
- [x] Use JMESPath for all conditions
- [x] Add condition result tracking
- [x] Handle condition evaluation errors

**Deliverables:**

- ✅ Condition evaluator in `pkg/execution/conditions.go`

### 3.5 Sub-Step Fanout Handler ✅

**Priority: Critical** | **Status: Complete**

**Tasks:**

- [x] Implement `iterate_over` evaluation (JMESPath)
- [x] Create sub-step fanout executor (`pkg/execution/fanout.go`):
  - Extract array from parent response
  - Create execution context for each item
  - Execute sub-steps concurrently (configurable concurrency)
  - Handle concurrency limits
  - Collect results
- [x] Add error handling (continue-on-error mode)
- [x] Add nesting depth limit

**Deliverables:**

- ✅ `pkg/execution/fanout.go`
- ✅ Fanout executor with concurrency control

---

## Phase 4: Plan Execution Orchestration 🔲 NOT STARTED

### 4.1 Plan Executor

**Priority: Critical** | **Status: Not Started**

**Tasks:**

- [ ] Create plan executor that orchestrates:
  - Auth flow execution (if needed)
  - Main plan execution
  - Sub-step execution
  - While loop handling
  - Context persistence
  - Statistics tracking
- [ ] Implement execution flow
- [ ] Add execution state management
- [ ] Handle execution timeouts

**Deliverables:**

- `pkg/execution/plan_executor.go`

### 4.2 Queue Job Processor

**Priority: Critical** | **Status: Not Started**

**Tasks:**

- [ ] Create Redis Streams consumer
- [ ] Implement job message structure
- [ ] Create job processor
- [ ] Implement consumer groups for horizontal scaling
- [ ] Add job acknowledgment logic

**Deliverables:**

- `pkg/queue/processor.go`

### 4.3 Job Scheduler

**Priority: Critical** | **Status: Not Started**

**Tasks:**

- [ ] Create job scheduler
- [ ] Implement scheduling logic
- [ ] Add scheduler configuration
- [ ] Implement graceful shutdown

**Deliverables:**

- `pkg/scheduler/` package

### 4.4 Context Management

**Priority: High** | **Status: Not Started**

**Tasks:**

- [ ] Implement context persistence
- [ ] Implement `set_context` with JMESPath
- [ ] Add context retrieval for expressions
- [ ] Implement context cleanup

**Deliverables:**

- Context management in `pkg/execution/context.go`

---

## Phase 5: Rate Limiting 🔲 NOT STARTED

**Note:** Base Redis rate limiting infrastructure exists in `pkg/redis/ratelimit.go`

### 5.1 Rate Limit Manager

**Priority: High** | **Status: Not Started**

**Tasks:**

- [ ] Create rate limit manager interface
- [ ] Implement rate limit buckets
- [ ] Implement algorithms (sliding window exists)
- [ ] Implement priority queue for rate-limited requests
- [ ] Add rate limit configuration parsing

### 5.2 Dynamic Rate Limiting

**Priority: High** | **Status: Not Started**

### 5.3 Rate Limit Integration

**Priority: High** | **Status: Not Started**

---

## Phase 6: Auth Flow Management 🔲 NOT STARTED

### 6.1 Auth Flow Executor

**Priority: High** | **Status: Not Started**

### 6.2 Auth Token Management

**Priority: High** | **Status: Not Started**

### 6.3 Auth Integration

**Priority: High** | **Status: Not Started**

---

## Phase 7: API Endpoints 🔲 NOT STARTED

### 7.1 Plan Management API

**Priority: High** | **Status: Not Started**

### 7.2 Config Management API

**Priority: High** | **Status: Not Started**

### 7.3 Execution Status API

**Priority: High** | **Status: Not Started**

### 7.4 Integration Management API

**Priority: Medium** | **Status: Not Started**

---

## Phase 7.5: Multi-Tenancy Implementation 🟡 PARTIAL

**Priority: Critical**

**Overview:**
Multi-tenancy is a core requirement for Orchid. All data must be isolated by tenant, and all operations must be tenant-scoped.

### 7.5.1 Database Schema Updates ✅

**Status: Complete**

**Completed:**
- [x] `tenant_id UUID NOT NULL` on all tenant-scoped tables
- [x] Composite primary keys: `PRIMARY KEY (tenant_id, id)`
- [x] Composite unique constraints including `tenant_id`
- [x] Tables distributed by `tenant_id` using Citus
- [x] Foreign keys with `tenant_id` in same ordinal position

**Migrations:**
- `0003_core_schema.up.sql` - Schema with tenant_id
- `0004_distribute_by_tenant.up.sql` - Citus distribution
- `0005_foreign_keys.up.sql` - FK constraints

### 7.5.2 Repository Tenant Isolation ✅

**Status: Complete**

**Completed:**
- [x] All repositories use `GetTenantID(ctx)` helper
- [x] All queries include `tenant_id` in WHERE clauses
- [x] All INSERTs include `tenant_id`
- [x] Tenant validation before all operations

### 7.5.3 API Tenant Validation 🔲

**Status: Not Started**

**Tasks:**
- [ ] Create tenant validation middleware
- [ ] Apply middleware to all API routes
- [ ] Update all API handlers
- [ ] Add tenant ownership validation helpers

### 7.5.4 Queue & Execution Tenant Context 🔲

**Status: Not Started** (partially addressed in execution context)

### 7.5.5 Testing Tenant Isolation 🔲

**Status: Not Started**

---

## Phase 8: Observability & Operations 🔲 NOT STARTED

**Note:** Base tracing (`pkg/tracing/`) and logging (`ectologger`) already exist.

### 8.1 Statistics Tracking

**Priority: High** | **Status: Not Started**

### 8.2 Metrics & Monitoring

**Priority: High** | **Status: Not Started**

### 8.3 Logging & Tracing

**Priority: Medium** | **Status: Partial** (base infrastructure exists)

### 8.4 Error Handling & Dead Letter Queue

**Priority: Medium** | **Status: Not Started**

---

## Phase 9: Testing & Documentation 🔲 NOT STARTED

**Note:** One integration test exists: `pkg/repositories/integration_repository_test.go`

### 9.0 Mock API Framework

**Priority: High** | **Status: Not Started**

### 9.1 Unit Tests

**Priority: High** | **Status: Not Started**

### 9.2 Integration Tests

**Priority: High** | **Status: Partial** (1 test exists)

### 9.3 API Documentation

**Priority: Medium** | **Status: Not Started**

### 9.4 User Documentation

**Priority: Medium** | **Status: Not Started**

---

## Phase 10: Performance & Scale Testing 🔲 NOT STARTED

### 10.1 Load Testing

**Priority: Medium** | **Status: Not Started**

### 10.2 Scale Testing

**Priority: Medium** | **Status: Not Started**

---

## Dependencies & Prerequisites

### External Dependencies

- PostgreSQL (Citus) - Database ✅ Configured
- Redis - Queue, locks, rate limits ✅ Configured
- Kafka - Data emission ✅ Configured
- Go 1.21+ - Language ✅

### Go Libraries (All Added)

- ✅ `github.com/jmespath/go-jmespath` - Expression evaluation
- ✅ `github.com/redis/go-redis/v9` - Redis client
- ✅ `github.com/segmentio/kafka-go` - Kafka producer
- ✅ `github.com/huandu/go-sqlbuilder` - SQL query builder
- ✅ `github.com/jmoiron/sqlx` - SQL
- ✅ `github.com/labstack/echo/v4` - HTTP framework
- ✅ `go.opentelemetry.io/otel` - Tracing

### Existing Infrastructure (In Use)

- ✅ `github.com/Gobusters/ectoinject` - Dependency injection
- ✅ `github.com/Gobusters/ectologger` - Logging
- ✅ `github.com/Gobusters/ectoenv` - Environment config
- ✅ `github.com/Ramsey-B/orchid/pkg/tracing` - OpenTelemetry tracing
- ✅ `github.com/Ramsey-B/orchid/pkg/startup` - Startup dependency management

---

## Next Steps

Based on current progress, the recommended next steps are:

1. **Phase 4.1: Plan Executor** - Orchestrate full plan execution with auth, sub-steps, and while loops
2. **Phase 4.2: Queue Job Processor** - Redis Streams consumer for job processing
3. **Phase 4.3: Job Scheduler** - Poll database and enqueue jobs
4. **Phase 6: Auth Flow Management** - Token management and caching
5. **Phase 7: API Endpoints** - REST API for plan/config management

---

## Timeline Summary

- **Week 1-2**: Foundation (DB, Redis, Kafka, JMESPath) ✅ COMPLETE
- **Week 2-3**: Models & Repositories ✅ COMPLETE
- **Week 3-5**: Execution Engine Core ✅ COMPLETE
- **Week 5-6**: Orchestration & Queue ← **CURRENT FOCUS**
- **Week 6-7**: Rate Limiting
- **Week 7-8**: Auth Flows
- **Week 8-9**: API Endpoints
- **Week 9-10**: Observability
- **Week 10-11**: Testing & Documentation
- **Week 11-12**: Performance & Scale Testing

**Total: ~12 weeks for full implementation**
