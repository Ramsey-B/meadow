# Orchid

**API Polling and Data Extraction Service**

Orchid is the first stage of the Meadow data integration pipeline. It polls external APIs on configurable schedules, handles authentication flows, manages rate limiting, and emits raw API responses to Kafka for downstream transformation by Lotus and entity resolution by Ivy.

## Overview

```
Orchid (Extract) → Lotus (Transform) → Ivy (Merge & Deduplicate)
```

### Core Responsibilities

1. **Poll** - Execute API calls to external systems on configurable schedules
2. **Authenticate** - Manage OAuth flows, token caching, and credential rotation
3. **Extract** - Retrieve raw data from 3rd party APIs with pagination support
4. **Rate Limit** - Respect API rate limits with dynamic header-based adjustments
5. **Fanout** - Execute nested sub-steps with configurable concurrency
6. **Emit** - Publish raw API responses to Kafka for downstream processing

### Key Features

- **Declarative Plan Definitions**: JSON-based workflow definitions with conditional logic
- **Complex Workflows**: Pagination (while loops), nested sub-steps (fanout), retries, conditionals
- **OAuth & Token Management**: Configurable auth flows with Redis caching and automatic refresh
- **Dynamic Rate Limiting**: Extract rate limits from API response headers
- **Horizontal Scalability**: Redis Streams job queue for distributed execution
- **Expression Templating**: JMESPath for dynamic URL, header, and body generation
- **Multi-Tenancy**: Complete tenant isolation with per-tenant configurations
- **Observability**: OpenTelemetry tracing, Prometheus metrics, structured logging
- **Dead Letter Queue**: Failed job recovery with manual retry capability

## Architecture

### Technology Stack

| Layer | Technology |
|-------|-----------|
| **Web Framework** | Echo v4 (HTTP), Ectoinject (DI) |
| **Database** | PostgreSQL with Citus extension (multi-tenant ready) |
| **Message Queue** | Apache Kafka (segmentio/kafka-go) |
| **Job Queue** | Redis Streams (distributed work distribution) |
| **Caching** | Redis (auth tokens, rate limits, locks) |
| **Scheduling** | Cron-based polling (every 30s) with distributed locks |
| **Observability** | OpenTelemetry (OTLP), Prometheus, Zap logging |
| **Configuration** | Environment variables (Ectoinject) |

### System Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Orchid Service (Go)                              │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                      Scheduler                                    │  │
│  │  • Polls every 30s for due plans                                 │  │
│  │  • Distributed locking via Redis                                 │  │
│  │  • Creates jobs in Redis Streams queue                           │  │
│  └──────────────────────┬───────────────────────────────────────────┘  │
│                         │                                                │
│                         ▼                                                │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                   Redis Streams Queue                             │  │
│  │  Stream: orchid:jobs                                              │  │
│  │  Consumer Group: orchid-workers                                   │  │
│  │  Job Type: plan_execution                                         │  │
│  └──────────────────────┬───────────────────────────────────────────┘  │
│                         │                                                │
│                         ▼                                                │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                   Queue Processor                                 │  │
│  │  • Consumes jobs from Redis Streams                               │  │
│  │  • Claims stale jobs                                              │  │
│  │  • Invokes PlanExecutor                                           │  │
│  │  • Moves failed jobs to DLQ                                       │  │
│  └──────────────────────┬───────────────────────────────────────────┘  │
│                         │                                                │
│                         ▼                                                │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                    Plan Executor                                  │  │
│  │  1. Load plan definition & config                                │  │
│  │  2. Execute auth flow (if configured)                            │  │
│  │     └─> AuthManager (Redis-cached tokens)                        │  │
│  │  3. Execute steps sequentially                                   │  │
│  │     ├─> StepExecutor (HTTP requests)                             │  │
│  │     │   └─> Rate Limiter (Redis-backed)                          │  │
│  │     ├─> While loops (pagination)                                 │  │
│  │     ├─> FanoutExecutor (parallel sub-steps)                      │  │
│  │     │   └─> Configurable concurrency (default: 50)               │  │
│  │     └─> Conditional logic (abort_when, retry_when, break_when)  │  │
│  │  4. Emit responses to Kafka                                      │  │
│  │  5. Update execution status in PostgreSQL                        │  │
│  └──────────────────────┬───────────────────────────────────────────┘  │
└─────────────────────────┼────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                           External APIs                                  │
│  • OAuth providers, REST APIs, webhooks                                 │
│  • Rate limited, paginated responses                                    │
└─────────────────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         Kafka Topics                                     │
│  api-responses ─────────────────────────────────────────────────────┐   │
│  api-errors (DLQ for failed responses) ─────────────────────────────┤   │
└─────────────────────────────────────────────────────────────────────┼───┘
                                                                      │
                                                                      ▼
                                              ┌───────────────────────────┐
                                              │  Lotus (Transform)        │
                                              │  Consumes api-responses   │
                                              └───────────────────────────┘
                                                                      │
                                                                      ▼
                                              ┌───────────────────────────┐
                                              │  PostgreSQL Database      │
                                              │  • Plans, Configs         │
                                              │  • Integrations           │
                                              │  • Executions             │
                                              │  • Statistics             │
                                              └───────────────────────────┘
```

### Core Components

| Component | Location | Responsibility |
|-----------|----------|----------------|
| **Scheduler** | [pkg/scheduler/scheduler.go](pkg/scheduler/scheduler.go) | Polls enabled plans every 30s, enqueues jobs in Redis Streams |
| **Queue Processor** | [pkg/queue/processor.go](pkg/queue/processor.go) | Consumes jobs from Redis Streams, invokes PlanExecutor |
| **Plan Executor** | [pkg/execution/plan_executor.go](pkg/execution/plan_executor.go) | Orchestrates plan execution with auth, rate limiting, step execution |
| **Step Executor** | [pkg/execution/executor.go](pkg/execution/executor.go) | Executes individual HTTP requests with retries and conditionals |
| **Fanout Executor** | [pkg/execution/fanout.go](pkg/execution/fanout.go) | Executes parallel sub-steps with configurable concurrency |
| **Auth Manager** | [pkg/auth/manager.go](pkg/auth/manager.go) | Manages OAuth tokens with Redis caching and automatic refresh |
| **Rate Limiter** | [pkg/ratelimit/limiter.go](pkg/ratelimit/limiter.go) | Redis-backed rate limiting with dynamic header extraction |
| **HTTP Client** | [pkg/httpclient/client.go](pkg/httpclient/client.go) | HTTP wrapper with template substitution and response parsing |
| **Kafka Producer** | [pkg/kafka/producer.go](pkg/kafka/producer.go) | Publishes API responses and execution events to Kafka |
| **Expression Evaluator** | [pkg/expressions/expressions.go](pkg/expressions/expressions.go) | JMESPath template evaluation for URLs, headers, bodies |

## HTTP API Endpoints

All endpoints are served under `/api/v1` with tenant-based authentication.

### Integration Management

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/integrations` | List all integrations |
| POST | `/api/v1/integrations` | Create integration with config schema |
| GET | `/api/v1/integrations/:id` | Get integration by ID |
| PUT | `/api/v1/integrations/:id` | Update integration |
| DELETE | `/api/v1/integrations/:id` | Delete integration |

**Integration**: Defines an external API system (e.g., Salesforce, HubSpot) with optional config schema for credentials.

### Configuration Management

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/configs` | List configs (requires `integration_id` query param) |
| POST | `/api/v1/configs` | Create config with credentials |
| GET | `/api/v1/configs/:id` | Get config by ID |
| PUT | `/api/v1/configs/:id` | Update config values |
| DELETE | `/api/v1/configs/:id` | Delete config |
| POST | `/api/v1/configs/:id/enable` | Enable configuration |
| POST | `/api/v1/configs/:id/disable` | Disable configuration |

**Config**: Tenant-specific credentials and settings for an integration (API keys, base URLs, etc.).

### Authentication Flow Management

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/auth-flows` | List auth flows (requires `integration_id` query param) |
| POST | `/api/v1/auth-flows` | Create OAuth/token auth flow |
| GET | `/api/v1/auth-flows/:id` | Get auth flow by ID |
| PUT | `/api/v1/auth-flows/:id` | Update auth flow definition |
| DELETE | `/api/v1/auth-flows/:id` | Delete auth flow |

**Auth Flow**: Defines how to obtain and refresh authentication tokens (OAuth 2.0, API keys, custom flows).

### Plan Management

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/plans` | List plans (supports `integration_id`, `enabled` query params) |
| POST | `/api/v1/plans` | Create plan with workflow definition |
| GET | `/api/v1/plans/:key` | Get plan by key (string identifier) |
| PUT | `/api/v1/plans/:key` | Update plan definition |
| DELETE | `/api/v1/plans/:key` | Delete plan |
| PATCH | `/api/v1/plans/:key/enabled` | Enable/disable plan |
| POST | `/api/v1/plans/:key/trigger` | Manually trigger plan execution |

**Plan**: Declarative workflow definition specifying how to extract data from an API.

### Execution Management

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/executions` | List executions (supports `plan_key`, `status` query params) |
| GET | `/api/v1/executions/:id` | Get execution details by ID |
| GET | `/api/v1/executions/:id/children` | List child executions (sub-steps) |

**Execution**: Tracks plan execution history, status, and timing.

### Statistics

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/statistics` | List statistics (requires `plan_key` query param) |
| GET | `/api/v1/statistics/:plan_key/:config_id` | Get stats for specific plan/config |

**Statistics**: Aggregated metrics (last_run, success_count, failure_count, API call count).

### Dead Letter Queue (DLQ)

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/dlq` | List failed jobs (supports `count` query param) |
| GET | `/api/v1/dlq/stats` | Get DLQ statistics |
| GET | `/api/v1/dlq/:id` | Get specific DLQ entry |
| POST | `/api/v1/dlq/:id/retry` | Re-enqueue failed job |
| DELETE | `/api/v1/dlq/:id` | Remove DLQ entry |

**DLQ**: Failed execution jobs that can be inspected and manually retried.

### Health & Metrics

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/health` | Health check (database, Redis connectivity) |
| GET | `/healthz` | Kubernetes readiness probe |
| GET | `/live` | Kubernetes liveness probe |
| GET | `/metrics` | Prometheus metrics |

### Authentication

**Development Mode** (`AUTH_ENABLED=false`):
- Uses `TestAuthMiddleware` to extract tenant/user from headers
- Required headers: `X-Tenant-ID`, `X-User-ID`

**Production Mode** (`AUTH_ENABLED=true`):
- OAuth2/OIDC authentication via configured issuer
- Environment variables: `AUTH_ISSUER_URL`, `AUTH_CLIENT_ID`

All endpoints enforce tenant isolation.

## Kafka Integration

### Messages Produced

#### `api-responses` Topic

**Purpose**: Publish raw API response data for downstream consumption by Lotus

**Message Format**:
```json
{
  "tenant_id": "tenant-123",
  "integration": "salesforce",
  "plan_key": "contacts-sync",
  "config_id": "config-789",
  "execution_id": "exec-456",
  "step_path": "fetch_contacts.get_details",
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123...",
  "span_id": "def456...",
  "request_url": "https://api.salesforce.com/contacts/00Q123",
  "request_method": "GET",
  "request_headers": {
    "Authorization": "Bearer token123",
    "Content-Type": "application/json"
  },
  "status_code": 200,
  "response_body": {
    "id": "00Q123",
    "first_name": "John",
    "last_name": "Doe",
    "email": "john.doe@example.com"
  },
  "response_headers": {
    "Content-Type": "application/json",
    "X-RateLimit-Remaining": "4999"
  },
  "response_size": 1024,
  "duration_ms": 250,
  "extracted_data": {
    "user_id": "00Q123"
  }
}
```

**Kafka Headers**:
- `tenant_id`: Tenant identifier
- `integration`: Integration name
- `plan_key`: Plan identifier
- `execution_id`: Execution identifier
- `traceparent`: W3C Trace Context format
- `tracestate`: W3C Trace State

**Message Key**: `{tenant_id}:{execution_id}` (ensures partition affinity)

**Special Handling**:
- When using fanout (`iterate_over`), `response_body` is always a JSON array
- Each array item is enriched with sub-step outputs as additional fields

#### `api-errors` Topic

**Purpose**: Capture failed API responses based on step policies

**When Published**:
- HTTP status codes match `abort_on` or `ignore_on` step configuration
- Step execution fails after max retries
- Rate limit exceeded and cannot wait

**Message Format**: Same as `api-responses` but routed to error topic

#### Execution Event Messages

**Purpose**: Lifecycle events for plan executions (used by Ivy for execution-based deletion)

**Event Types**:
- `execution.started` - Plan execution begins
- `execution.completed` - Plan execution ends (status: success, failed, aborted)

**Message Format**:
```json
{
  "type": "execution.completed",
  "tenant_id": "tenant-123",
  "integration": "salesforce",
  "plan_key": "contacts-sync",
  "config_id": "config-789",
  "execution_id": "exec-456",
  "status": "success",
  "timestamp": "2024-01-15T10:35:00Z"
}
```

### Kafka Configuration

**Environment Variables**:

```bash
# Broker Configuration
KAFKA_BROKERS=localhost:9092

# Topics
KAFKA_RESPONSE_TOPIC=api-responses
KAFKA_ERROR_TOPIC=api-errors

# Producer Settings (hardcoded in code)
# - Batch size: 100 messages
# - Batch timeout: 10ms
# - Required acks: 1 (leader acknowledgement)
# - Balancer: LeastBytes
# - Auto topic creation: enabled
```

## Plan Definition Structure

Plans are JSON documents that define the extraction workflow.

### Basic Plan Example

```json
{
  "key": "salesforce-contacts",
  "integration": "salesforce",
  "wait_seconds": 3600,
  "enabled": true,
  "step": {
    "url": "{config.base_url}/api/contacts",
    "method": "GET",
    "headers": {
      "Authorization": "Bearer {auth.token}",
      "Content-Type": "application/json"
    },
    "params": {
      "limit": "100",
      "offset": "{context.next_offset || '0'}"
    },
    "while": "response.body.has_more == true",
    "context_updates": {
      "next_offset": "response.body.next_offset"
    }
  }
}
```

### Plan with Fanout (Nested Sub-Steps)

```json
{
  "key": "contacts-with-details",
  "step": {
    "url": "{config.base_url}/api/contacts",
    "method": "GET",
    "sub_steps": [
      {
        "iterate_over": "response.body.contacts[*]",
        "concurrency": 50,
        "url": "{config.base_url}/api/contacts/{item.id}/details",
        "method": "GET",
        "emit_to_kafka": true
      }
    ]
  },
  "rate_limits": [
    {
      "requests": 100,
      "window_secs": 60,
      "scope": "per_config"
    }
  ]
}
```

### Step Configuration Options

**Core Fields**:
- `url`: URL template with variable substitution
- `method`: HTTP method (GET, POST, PUT, DELETE, PATCH)
- `headers`: Key-value map with template support
- `params`: Query/path parameters with template support
- `body`: Request body (JSON, form data, or template string)

**Control Flow**:
- `while`: JMESPath condition for loop continuation (pagination)
- `abort_when`: JMESPath condition to abort execution
- `retry_when`: JMESPath condition to retry request
- `break_when`: JMESPath condition to exit current loop
- `emit_to_kafka`: Boolean to control message emission (default: true)

**Error Handling**:
- `abort_on`: HTTP status codes that abort execution (e.g., `[401, 403]`)
- `ignore_on`: HTTP status codes to route to error topic and continue (e.g., `[404]`)
- `retry`: Retry configuration with max attempts and backoff

**Sub-Steps (Fanout)**:
- `sub_steps`: Array of nested step definitions
- `iterate_over`: JMESPath expression for array iteration
- `concurrency`: Max parallel executions (default: 50)
- `fanout_emit_mode`: "record" (one message per item) or "page" (one message per batch)

**Rate Limiting**:
- `rate_limits`: Array of rate limit configurations
  - `requests`: Number of requests allowed
  - `window_secs`: Time window in seconds
  - `scope`: "per_config", "per_endpoint", or "global"
  - `priority`: Priority level (higher = more important)

**Context Management**:
- `context_updates`: Map of context field updates using JMESPath expressions
- `extract`: Extract specific fields from response for downstream use

### Template Variables

Plans can reference dynamic values using `{variable.path}` syntax:

- `{config.*}` - Configuration values (API keys, base URLs)
- `{auth.*}` - Authentication tokens and headers
- `{context.*}` - Persistent context fields
- `{response.*}` - Current step response (body, headers, status_code)
- `{prev.*}` - Previous iteration response (in while loops)
- `{parent.*}` - Parent step response (in sub-steps)
- `{item.*}` - Current iteration item (in fanout sub-steps)

**Example**:
```
URL: "{config.base_url}/users/{item.id}"
Header: "Authorization: Bearer {auth.access_token}"
Param: "since={context.last_sync_time}"
```

## Execution Flow

### Plan Execution Lifecycle

```
1. Scheduler Polls (every 30s)
   ↓
2. Check if plan+config is due
   ↓
3. Enqueue job in Redis Streams (orchid:jobs)
   ↓
4. Queue Processor claims job
   ↓
5. PlanExecutor.Execute()
   ├─> Load plan definition & config from PostgreSQL
   ├─> Execute auth flow (if configured)
   │   └─> AuthManager checks Redis cache
   │       └─> If expired, execute auth plan & cache token
   ├─> For each step in plan:
   │   ├─> Apply rate limiting (check Redis counters)
   │   ├─> Build request (template substitution)
   │   ├─> StepExecutor.Execute()
   │   │   ├─> Make HTTP request
   │   │   ├─> Handle retries (Fibonacci backoff)
   │   │   ├─> Evaluate conditionals (abort_when, retry_when, break_when)
   │   │   └─> Return response
   │   ├─> Check while condition (pagination)
   │   ├─> Execute sub-steps (fanout)
   │   │   └─> FanoutExecutor (parallel execution, concurrency control)
   │   ├─> Emit to Kafka (api-responses or api-errors)
   │   └─> Update context
   ├─> Update execution status in PostgreSQL
   └─> Publish execution.completed event
```

### Authentication Flow

```
1. PlanExecutor checks if auth_flow_id is configured
   ↓
2. AuthManager.GetAuth(config_id, auth_flow_id)
   ├─> Check Redis cache for token
   │   └─> Key: auth:{tenant_id}:{config_id}:{auth_flow_id}
   ├─> If cached and not expired:
   │   └─> Return cached token
   └─> If missing or expired:
       ├─> Execute auth flow plan (separate plan execution)
       ├─> Extract token from response using token_path JMESPath
       ├─> Cache in Redis with TTL (accounting for skew)
       └─> Return token
   ↓
3. Token available as {auth.token} or {auth.*} in step templates
```

### Rate Limiting Flow

```
1. StepExecutor prepares to make HTTP request
   ↓
2. RateLimiter.Wait(endpoint, config)
   ├─> Load rate limit rules for endpoint/scope
   ├─> Check Redis counters for current window
   │   └─> Key pattern: ratelimit:{scope}:{identifier}:{window}
   ├─> If limit exceeded:
   │   ├─> Calculate wait time until next window
   │   ├─> If wait time > max_wait (60s):
   │   │   └─> Return error (route to api-errors topic)
   │   └─> Sleep until window resets
   └─> Increment counter in Redis
   ↓
3. HTTP request proceeds
   ↓
4. Extract dynamic rate limit from response headers (if configured)
   ├─> X-RateLimit-Remaining
   ├─> X-RateLimit-Reset
   ├─> Retry-After
   └─> Update Redis counters based on header values
```

### Retry Logic

```
1. StepExecutor makes HTTP request
   ↓
2. Check response status and conditions
   ├─> If status in abort_on → abort execution
   ├─> If status in ignore_on → route to error topic, continue
   ├─> If retry_when condition is true:
   │   ├─> Check retry count < max_retries
   │   ├─> Calculate Fibonacci backoff with jitter
   │   │   └─> Sequence: 1s, 1s, 2s, 3s, 5s, 8s, 13s, 21s...
   │   │   └─> Max backoff: 60s (configurable)
   │   ├─> Sleep for backoff duration
   │   └─> Retry request
   └─> If all retries exhausted → route to error topic
```

## Configuration

### Database

```bash
DB_HOST=localhost
DB_PORT=5432
DB_USER_NAME=postgres
DB_PASSWORD=postgres
DB_NAME=orchid
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_MIGRATION_FOLDER_PATH=db/pg
```

### Redis

```bash
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
```

### Kafka

```bash
KAFKA_BROKERS=localhost:9092
KAFKA_RESPONSE_TOPIC=api-responses
KAFKA_ERROR_TOPIC=api-errors
```

### Scheduling

```bash
SCHEDULER_ENABLED=true
SCHEDULER_POLL_INTERVAL=30s
REDIS_STREAMS_JOB_QUEUE=orchid:jobs
REDIS_STREAMS_CONSUMER_GROUP=orchid-workers
```

### Execution Limits

```bash
MAX_EXECUTION_TIME=5m
MAX_LOOPS=1000
MAX_NESTING_DEPTH=5
DEFAULT_CONCURRENCY=50
```

### Observability

```bash
# Tracing
OTLP_ENABLED=false
OTLP_ENDPOINT=localhost:4317
OTLP_PROTOCOL=grpc  # or http

# Logging
LOG_LEVEL=info
PRETTY_LOGS=true
```

### Authentication

```bash
AUTH_ENABLED=false
AUTH_ISSUER_URL=
AUTH_CLIENT_ID=
```

### HTTP Server

```bash
PORT=8080
HTTP_SERVER_READ_TIMEOUT_SECONDS=30
HTTP_SERVER_WRITE_TIMEOUT_SECONDS=30
HTTP_SERVER_IDLE_TIMEOUT_SECONDS=60
```

## Operational Limits and Constraints

### Performance Limits

| Limit | Default | Configurable | Purpose |
|-------|---------|--------------|---------|
| **Response Size** | 10MB | No | Maximum API response body size (Kafka limit) |
| **Request Body Size** | 5MB | Per-step | Maximum request body size |
| **Execution Timeout** | 5 minutes | `MAX_EXECUTION_TIME` | Per-plan timeout |
| **Sub-Step Concurrency** | 50 | Per sub-step | Parallel fanout executions |
| **Max Loops** | 1000 | `MAX_LOOPS` | While loop iterations |
| **Max Nesting Depth** | 5 | `MAX_NESTING_DEPTH` | Sub-step nesting levels |
| **Context Size** | 64KB per field, 1MB total | No | Persistent context limits |
| **Retry Backoff Max** | 60 seconds | Configurable | Maximum retry wait time |

### Retry Strategy

**Fibonacci Backoff**: Grows more slowly than exponential, less aggressive on APIs
- Sequence: 1s, 1s, 2s, 3s, 5s, 8s, 13s, 21s, 34s, 55s, 60s (max)
- Jitter applied to prevent thundering herd
- Configurable max backoff (default: 60s)

### Supported Response Formats

- **JSON**: Primary format, native parsing
- **XML**: Converted to JSON for processing
- **CSV**: Handled as needed
- **Binary**: Base64 encoded for Kafka emission

### Horizontal Scaling

- **Job Distribution**: Redis Streams ensures work is distributed across instances
- **No Per-Instance Limits**: Natural scaling through Go concurrency
- **Shared State**: Redis for locks, rate limits, auth cache
- **Database**: PostgreSQL with Citus for multi-tenant horizontal scaling

## Getting Started

### Prerequisites

- Go 1.21+
- PostgreSQL 15+
- Redis 6.0+
- Apache Kafka 3.0+

### Environment Setup

Create a `.env` file:

```bash
# Application
APP_NAME=orchid
PORT=8080
LOG_LEVEL=info

# Database
DATABASE_URL=postgres://user:password@localhost:5432/orchid?sslmode=disable

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# Kafka
KAFKA_BROKERS=localhost:9092
KAFKA_RESPONSE_TOPIC=api-responses
KAFKA_ERROR_TOPIC=api-errors

# Scheduler
SCHEDULER_ENABLED=true
SCHEDULER_POLL_INTERVAL=30s

# Auth
AUTH_ENABLED=false
```

### Running Locally

1. **Install Dependencies**:
   ```bash
   go mod download
   ```

2. **Run Database Migrations**:
   ```bash
   migrate -path db/pg -database "$DATABASE_URL" up
   ```

3. **Start the Service**:
   ```bash
   go run cmd/main.go
   ```

4. **Verify Health**:
   ```bash
   curl http://localhost:8080/health
   ```

### Docker Setup

```bash
docker-compose up -d
```

Includes: PostgreSQL, Redis, Kafka, Zookeeper

## Development

### Project Structure

```
orchid/
├── cmd/                    # Application entry point, DI, routing
├── config/                 # Configuration management (30+ env vars)
├── db/pg/                  # PostgreSQL migrations
├── internal/handlers/      # HTTP endpoint handlers
├── pkg/
│   ├── auth/              # OAuth token caching & flow execution
│   ├── execution/         # Plan/step/fanout execution engines
│   ├── expressions/       # JMESPath templating
│   ├── httpclient/        # HTTP request builder
│   ├── kafka/             # Kafka producer
│   ├── queue/             # Redis Streams job processor
│   ├── ratelimit/         # Redis-backed rate limiting
│   ├── redis/             # Redis client, locks, DLQ
│   ├── repositories/      # Database CRUD
│   └── scheduler/         # Cron polling
└── CONSUMER_GUIDE.md      # Kafka message documentation
```

### Testing

```bash
# Unit tests
go test ./...

# With coverage
go test -cover ./...

# Integration tests
go test ./test/integration/...
```

## Architecture Decisions

1. **Redis Streams for Job Queue**: Distributed, fault-tolerant work distribution across instances
2. **Declarative Plans**: JSON-based definitions enable version control and UI generation
3. **JMESPath Templating**: Industry-standard expression language for dynamic values
4. **Fibonacci Backoff**: Less aggressive than exponential, better for API relationships
5. **Token Caching**: Redis caching reduces auth overhead and API calls
6. **Dynamic Rate Limiting**: Adapts to API-provided rate limit information
7. **Fanout with Concurrency Control**: Maximizes throughput while respecting limits
8. **Multi-Tenancy**: Composite primary keys enable horizontal scaling with Citus

## License

Proprietary - Meadow Platform
