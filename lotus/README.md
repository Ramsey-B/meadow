# Lotus

**Data Transformation and Mapping Service**

Lotus is the second stage of the Meadow data integration pipeline. It consumes raw API responses from Orchid, applies user-defined data mappings to transform and normalize the data, and produces structured entity and relationship records for downstream consumption by Ivy.

## Overview

```
Orchid (Extract) → Lotus (Transform) → Ivy (Merge & Deduplicate)
```

### Core Responsibilities

1. **Consume** - Ingest raw API response messages from Orchid via Kafka
2. **Route** - Match incoming messages to appropriate mappings using binding filters
3. **Transform** - Apply declarative JSON-based mapping transformations to normalize data
4. **Batch Process** - Handle array responses by splitting into individual entity messages
5. **Produce** - Publish transformed entity/relationship records to output Kafka topics
6. **Forward** - Pass through execution lifecycle events without transformation

### Key Features

- **Declarative Mappings**: JSON-based mapping definitions with source fields, target fields, transformation steps, and data flow links
- **Binding System**: Route messages to mappings based on integration, plan key, status code, URL pattern, and step path filters
- **81+ Transformation Actions**: Text, number, array, object, date, and conditional operations
- **Chained Transformations**: Multi-stage data transformations with priority-based execution
- **Array Processing**: Automatic splitting of array responses into individual entity messages
- **High-Performance**: 300K+ messages/second throughput with compiled mapping cache and object pooling
- **Multi-Tenancy**: Complete tenant isolation with configurable routing
- **Tracing Integration**: OpenTelemetry trace context propagation from Orchid through output
- **Dynamic Configuration**: Hot-reload of bindings and mappings without restart

## Architecture

### Technology Stack

| Layer | Technology |
|-------|-----------|
| **Web Framework** | Echo v4 (HTTP), Ectoinject (DI) |
| **Database** | PostgreSQL with migrations |
| **Message Queue** | Apache Kafka (segmentio/kafka-go) |
| **Caching** | LRU cache for compiled mappings (in-memory) |
| **Observability** | OpenTelemetry, Zap structured logging |
| **Configuration** | Environment variables (Ectoinject) |
| **Performance** | Object pooling, mapping compilation, worker concurrency |

### System Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              Kafka Topics                                │
│  api-responses (from Orchid) ─────────────────────────────────┐         │
└───────────────────────────────────────────────────────────────┼─────────┘
                                                                │
                                                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         Lotus Service (Go)                               │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                      Kafka Consumer                               │  │
│  │                 (api-responses topic)                             │  │
│  └──────────────────────┬───────────────────────────────────────────┘  │
│                         │                                                │
│                         ▼                                                │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                   Message Processor                               │  │
│  │  • Parse OrchidMessage                                            │  │
│  │  • Extract trace context                                          │  │
│  │  • Handle batch arrays                                            │  │
│  │  • Forward lifecycle events                                       │  │
│  └──────────────────────┬───────────────────────────────────────────┘  │
│                         │                                                │
│                         ▼                                                │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                   Binding Matcher                                 │  │
│  │  • Match by integration name                                      │  │
│  │  • Filter by plan keys                                            │  │
│  │  • Filter by status codes                                         │  │
│  │  • Match URL patterns                                             │  │
│  │  • Match step path prefix                                         │  │
│  │  • Score and select best binding                                  │  │
│  └──────────────────────┬───────────────────────────────────────────┘  │
│                         │                                                │
│                         ▼                                                │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                  Mapping Cache (LRU)                              │  │
│  │  • Load compiled mapping (TTL: 5min)                              │  │
│  │  • Max size: 1000 mappings                                        │  │
│  │  • Hot-reload from database                                       │  │
│  └──────────────────────┬───────────────────────────────────────────┘  │
│                         │                                                │
│                         ▼                                                │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                  Mapping Engine                                   │  │
│  │  1. Extract source field values (JSONPath)                        │  │
│  │  2. Execute transformation steps                                  │  │
│  │     • Apply actions (81+ transformations)                         │  │
│  │     • Chain steps via links                                       │  │
│  │     • Handle conditionals and validation                          │  │
│  │  3. Generate target field values                                  │  │
│  │  4. Build output structure (nested objects/arrays)                │  │
│  └──────────────────────┬───────────────────────────────────────────┘  │
│                         │                                                │
│                         ▼                                                │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │              Batch Record Extraction                              │  │
│  │  • Detect _entity_type or _relationship_type arrays               │  │
│  │  • Split into per-record messages                                 │  │
│  │  • Preserve trace context                                         │  │
│  └──────────────────────┬───────────────────────────────────────────┘  │
│                         │                                                │
│                         ▼                                                │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                   Kafka Producer                                  │  │
│  │  • Route to binding-specific output topic                         │  │
│  │  • Add headers (tenant_id, binding_id, trace context)             │  │
│  │  • Batch messages (100/batch, 100ms timeout)                      │  │
│  │  • Compression (snappy)                                           │  │
│  └──────────────────────┬───────────────────────────────────────────┘  │
└─────────────────────────┼────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                              Kafka Topics                                │
│  mapped-data (to Ivy) ───────────────────────────────────────────┐     │
│  mapping-errors (DLQ) ───────────────────────────────────────────┤     │
│  mapped-data (passthrough for execution.* events) ───────────────┘     │
└─────────────────────────────────────────────────────────────────────────┘
                          │
                          ▼
              ┌───────────────────────┐
              │  PostgreSQL Database  │
              │  • Mapping Definitions│
              │  • Bindings           │
              └───────────────────────┘
```

### Core Components

| Component | Location | Responsibility |
|-----------|----------|----------------|
| **Mapping Engine** | [pkg/mapping/mapping.go](pkg/mapping/mapping.go) | Core transformation engine with field extraction, step execution, and output generation |
| **Binding Matcher** | [pkg/binding/matcher.go](pkg/binding/matcher.go) | Routes messages to mappings based on filter criteria with scoring |
| **Message Processor** | [pkg/processor/processor.go](pkg/processor/processor.go) | Orchestrates pipeline: match → load → transform → publish |
| **Kafka Consumer** | [pkg/kafka/consumer.go](pkg/kafka/consumer.go) | Consumes messages from `api-responses` topic |
| **Kafka Producer** | [pkg/kafka/producer.go](pkg/kafka/producer.go) | Publishes transformed data to output topics |
| **Binding Loader** | [pkg/processor/binding_loader.go](pkg/processor/binding_loader.go) | Dynamically loads bindings from database with periodic refresh (60s) |
| **Mapping Cache** | [pkg/processor/mapping_cache.go](pkg/processor/mapping_cache.go) | LRU cache for compiled mappings with TTL (5min) |
| **Actions Registry** | [pkg/actions/registry.go](pkg/actions/registry.go) | Library of 81+ transformation functions |
| **Fields System** | [pkg/fields/field.go](pkg/fields/field.go) | Field definitions with types, paths, and validation |
| **Steps System** | [pkg/steps/step.go](pkg/steps/step.go) | Transformation step execution (transformer, validator, condition) |
| **Links System** | [pkg/links/link.go](pkg/links/link.go) | Data flow connections between sources, steps, and targets |

## HTTP API Endpoints

All endpoints are served under `/api/v1` with tenant-based authentication.

### Mapping Definition Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/api/v1/mappings/definitions/validate` | Validate mapping definition without persisting |
| POST | `/api/v1/mappings/definitions` | Create new mapping definition |
| PUT | `/api/v1/mappings/definitions/:id` | Update existing mapping definition |
| GET | `/api/v1/mappings/definitions/:id` | Get active mapping definition by ID |

### Mapping Execution Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/api/v1/mappings/execute` | Execute stored mapping against source data |
| POST | `/api/v1/mappings/test` | Test mapping without storing (ad-hoc execution) |

### Actions Discovery Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/actions` | List all available transformation actions (81+) |
| POST | `/api/v1/actions/output-types` | Infer output type for action given inputs |

### Bindings Management Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/bindings` | List all bindings for current tenant |
| POST | `/api/v1/bindings` | Create new binding (message-to-mapping routing) |
| GET | `/api/v1/bindings/:id` | Get specific binding |
| PUT | `/api/v1/bindings/:id` | Update binding with partial updates |
| DELETE | `/api/v1/bindings/:id` | Delete binding |

### Health Check Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/health` | Detailed health check with component statuses |
| GET | `/api/v1/health/live` | Kubernetes liveness probe (is process running?) |
| GET | `/api/v1/health/ready` | Kubernetes readiness probe (is service ready for traffic?) |

### Authentication

**Development Mode** (`AUTH_ENABLED=false`):
- Uses `TestAuthMiddleware` to extract tenant/user from headers
- Required headers: `X-Tenant-ID`, `X-User-ID`

**Production Mode** (`AUTH_ENABLED=true`):
- OAuth2/OIDC authentication via configured issuer
- Environment variables: `AUTH_ISSUER_URL`, `AUTH_CLIENT_ID`

All endpoints enforce tenant isolation. Operations are scoped to the authenticated tenant.

## Kafka Integration

### Messages Consumed

#### `api-responses` Topic (from Orchid)

**Purpose**: Ingest raw API response messages from Orchid

**Message Format**:
```json
{
  "tenant_id": "tenant-123",
  "integration": "salesforce",
  "plan_key": "leads-plan-uuid",
  "config_id": "config-789",
  "execution_id": "exec-456",
  "step_path": "api.leads.list",
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123...",
  "span_id": "def456...",
  "request_url": "https://api.salesforce.com/leads",
  "request_method": "GET",
  "request_headers": {
    "Authorization": "Bearer ..."
  },
  "status_code": 200,
  "response_body": {
    "leads": [
      {
        "id": "00Q5e000001xyz",
        "first_name": "John",
        "last_name": "Doe",
        "email": "john.doe@example.com"
      }
    ]
  },
  "response_headers": {
    "Content-Type": "application/json"
  },
  "response_size": 1024,
  "duration_ms": 250,
  "extracted_data": {}
}
```

**Special Handling**:
- **Lifecycle Events**: Messages with `type` field starting with `execution.` (e.g., `execution.completed`) are forwarded to passthrough topic without transformation
- **Batch Arrays**: If `response_body` is a JSON array, Lotus splits it into per-item messages for individual processing

### Messages Produced

#### `mapped-data` Topic (to Ivy and passthrough)

**Purpose**: Publish transformed entity/relationship records for downstream consumption

**Message Format**:
```json
{
  "source": {
    "type": "orchid",
    "tenant_id": "tenant-123",
    "integration": "salesforce",
    "key": "leads-plan-uuid",
    "config_id": "config-789",
    "execution_id": "exec-456"
  },
  "binding_id": "binding-abc",
  "mapping_id": "mapping-def",
  "mapping_version": 1,
  "timestamp": "2024-01-15T10:31:00Z",
  "trace_id": "abc123...",
  "span_id": "ghi789...",
  "data": {
    "_entity_type": "person",
    "_source_id": "00Q5e000001xyz",
    "first_name": "John",
    "last_name": "Doe",
    "email": "john.doe@example.com"
  }
}
```

**Kafka Headers**:
- `tenant_id`: Tenant identifier for filtering
- `integration`: Integration name
- `plan_key`: Orchid plan key
- `execution_id`: Orchid execution ID
- `binding_id`: Lotus binding ID
- `traceparent`: W3C Trace Context format (`00-{trace_id}-{span_id}-01`)
- `tracestate`: W3C Trace State

**Batch Record Extraction**:
If mapping produces arrays with `_entity_type` or `_relationship_type` fields, Lotus emits one Kafka message per record:
```json
{
  "data": [
    {"_entity_type": "person", "_source_id": "1", ...},
    {"_entity_type": "person", "_source_id": "2", ...}
  ]
}
```
Becomes two separate messages with individual records.

#### `mapping-errors` Topic (Dead Letter Queue)

**Purpose**: Capture mapping/processing failures for monitoring and debugging

**Message Format**:
```json
{
  "tenant_id": "tenant-123",
  "execution_id": "exec-456",
  "stage": "mapping_execution",
  "error": "field 'email' not found in source data",
  "input_data": {...},
  "step_path": "api.leads.list",
  "timestamp": "2024-01-15T10:31:00Z"
}
```

### Kafka Configuration

**Environment Variables**:

```bash
# Broker Configuration
KAFKA_BROKERS=localhost:9092

# Consumer
KAFKA_INPUT_TOPIC=api-responses
KAFKA_CONSUMER_GROUP=lotus-consumer
KAFKA_CONSUMER_ENABLED=true

# Producer
KAFKA_OUTPUT_TOPIC=mapped-data
KAFKA_ERROR_TOPIC=mapping-errors
KAFKA_BATCH_SIZE=100
KAFKA_BATCH_TIMEOUT_MS=100
KAFKA_REQUIRED_ACKS=1
KAFKA_COMPRESSION=snappy  # Options: gzip, lz4, zstd, snappy, none

# Processing
PROCESSOR_WORKER_COUNT=4
PROCESSOR_TIMEOUT_SECONDS=30
```

**Consumer Configuration**:
- Consumer group: `lotus-consumer` (configurable)
- Start offset: `LastOffset` (process new messages only)
- Commit interval: 1 second
- Session timeout: 30 seconds
- Rebalance timeout: 30 seconds

**Producer Configuration**:
- Partition key: `{tenantID}:{bindingID}` for consistent ordering
- Batch size: 100 messages (configurable)
- Batch timeout: 100ms (configurable)
- Required acks: 1 (leader acknowledgement)
- Compression: Snappy (configurable)

## Mapping Engine

### Core Concepts

#### Mapping Definition

A mapping definition is a declarative blueprint for data transformation containing:

- **Source Fields**: Fields to extract from input data with JSONPath-style paths
- **Target Fields**: Fields to produce in output data with nested structure support
- **Steps**: Optional transformation steps (transformers, validators, conditions)
- **Links**: Data flow connections between sources, steps, and targets with priority-based execution

#### Data Flow with Links

Links define how data flows through the mapping:

```
Field → Field   : Direct mapping (no transformation)
Field → Step    : Input to a transformation
Step → Step     : Chained transformations
Step → Field    : Output of a transformation
```

Each link has a priority (lower = earlier execution) to control the order of operations.

#### Step Types

1. **Transformer**: Transforms input data (e.g., `text_to_upper`, `number_add`)
2. **Validator**: Validates data and fails if condition is false
3. **Condition**: Conditional branching that breaks the chain if false (allows filtering)

#### Example Mapping

**Simple uppercase transformation**:
```json
{
  "source_fields": [
    {"id": "name", "path": "name", "type": "string"}
  ],
  "target_fields": [
    {"id": "upper_name", "path": "upper_name", "type": "string"}
  ],
  "steps": [
    {
      "id": "uppercase",
      "type": "transformer",
      "action": {"key": "text_to_upper"}
    }
  ],
  "links": [
    {"priority": 0, "source": {"field_id": "name"}, "target": {"step_id": "uppercase"}},
    {"priority": 1, "source": {"step_id": "uppercase"}, "target": {"field_id": "upper_name"}}
  ]
}
```

**Input**: `{"name": "hello"}`
**Output**: `{"upper_name": "HELLO"}`

### Transformation Actions

Lotus provides 81+ built-in transformation actions organized by category:

#### Text Actions (20+)
`text_to_upper`, `text_to_lower`, `text_length`, `text_contains`, `text_starts_with`, `text_ends_with`, `text_replace`, `text_trim`, `text_concat`, `text_substring`, `text_split`, `text_to_array`, `text_to_number`, `text_to_bool`, `text_index_of`, `text_reverse`, `text_min_length`, `text_max_length`, `text_regex_match`, `text_regex_extract`, `text_regex_replace`, `text_pad`

#### Number Actions (20+)
`number_add`, `number_subtract`, `number_multiply`, `number_divide`, `number_modulus`, `number_abs`, `number_ceiling`, `number_floor`, `number_round`, `number_square`, `number_cube`, `number_square_root`, `number_cube_root`, `number_min`, `number_max`, `number_equals`, `number_exponent`, `number_root`, `number_factorial`, `number_clamp`, `number_sign`, `number_parse`, `number_to_string`, `number_is_even`, `number_is_odd`, `number_to_positive`, `number_to_negative`

#### Array Actions (10+)
`array_length`, `array_contains`, `array_index_of`, `array_push`, `array_reverse`, `array_distinct`, `array_take`, `array_skip`, `array_randomize`

#### Object Actions (6)
`object_get`, `object_pick`, `object_omit`, `object_merge`, `object_keys`, `object_values`

#### Date Actions (5)
`date_now`, `date_parse`, `date_format`, `date_diff`, `date_add`

#### Any/Conditional Actions (6)
`any_coalesce`, `any_default`, `any_if_else`, `any_is_nil`, `any_is_empty`, `any_to_string`

Use the `GET /api/v1/actions` endpoint to retrieve complete action metadata including parameters, types, and descriptions.

### Array Processing

When a source field is an array with `items` defined, each array element is extracted separately:

```json
{
  "source_fields": [
    {
      "id": "numbers",
      "type": "array",
      "path": "numbers",
      "items": {"id": "num", "type": "number"}
    }
  ],
  "steps": [
    {"id": "sum", "action": {"key": "number_add"}}
  ],
  "links": [
    {"source": {"field_id": "num"}, "target": {"step_id": "sum"}},
    {"source": {"step_id": "sum"}, "target": {"field_id": "total"}}
  ]
}
```

**Input**: `{"numbers": [1, 2, 3, 4, 5]}`
**Processing**: `sum` step receives all array elements `(1, 2, 3, 4, 5)`
**Output**: `{"total": 15}`

### Conditional Logic

Condition steps can filter data flow. When a condition breaks (returns false), the chain stops but other inputs still flow:

```
value_a → is_even (condition) → add_step → result
value_b ──────────────────────→ add_step
```

- If `value_a` is odd: `is_even` breaks, only `value_b` reaches `add_step`
- If `value_a` is even: both values reach `add_step` and are added

## Binding System

Bindings route incoming messages to appropriate mappings based on filter criteria.

### Binding Filter Criteria

```json
{
  "name": "Salesforce Leads Mapping",
  "mapping_id": "mapping-uuid",
  "is_enabled": true,
  "output_topic": "mapped-data",
  "filter": {
    "integration": "salesforce",
    "keys": ["leads-plan-uuid"],
    "status_codes": [200],
    "min_status_code": 200,
    "max_status_code": 299,
    "step_path_prefix": "api.leads",
    "request_url_contains": "/leads"
  }
}
```

### Filter Matching

Messages are matched against bindings using the following criteria:

1. **Integration Match**: `filter.integration` matches `message.integration`
2. **Plan Key Match**: `message.plan_key` in `filter.keys` (if keys specified)
3. **Status Code**: `message.status_code` in `filter.status_codes` OR within `min_status_code` to `max_status_code` range
4. **Step Path**: `message.step_path` starts with `filter.step_path_prefix`
5. **URL Pattern**: `message.request_url` contains `filter.request_url_contains`

### Binding Scoring

When multiple bindings match, Lotus selects the highest-scoring binding:

- Base score: 1 (for integration match)
- +10: Plan key match
- +5: Status code match
- +5: Step path prefix match
- +5: URL pattern match

The binding with the highest score is selected for mapping execution.

### Dynamic Binding Reload

Bindings are automatically reloaded from the database every 60 seconds (configurable via `BINDING_REFRESH_INTERVAL_MS`). This allows changes to take effect without restarting the service.

## Processing Flow

### Message Processing Pipeline

```
1. Kafka Consumer receives OrchidMessage
   ↓
2. Check if lifecycle event (execution.*)
   ├─ Yes → Forward to passthrough topic (no transformation)
   └─ No → Continue
   ↓
3. Parse message and extract trace context
   ↓
4. Check if response_body is array
   ├─ Yes → Split into per-item messages
   └─ No → Continue
   ↓
5. Match message to binding (BindingMatcher)
   ├─ No match → Log and skip
   └─ Match found → Continue
   ↓
6. Load compiled mapping from cache (or database)
   ├─ Cache hit → Use cached mapping
   └─ Cache miss → Load from DB, compile, cache
   ↓
7. Execute mapping transformation
   • Extract source field values
   • Execute transformation steps
   • Generate target field values
   • Build output structure
   ↓
8. Check for batch record extraction
   ├─ _entity_type or _relationship_type arrays found
   │  └─ Split into per-record messages
   └─ No arrays → Single message
   ↓
9. Publish to output topic (from binding configuration)
   • Add Kafka headers (tenant_id, binding_id, trace context)
   • Partition key: {tenant_id}:{binding_id}
   ↓
10. Commit Kafka offset
```

### Error Handling

When errors occur during processing:

1. **Binding Match Failure**: Log warning, skip message (no DLQ)
2. **Mapping Load Failure**: Publish to `mapping-errors` topic with error details
3. **Mapping Execution Failure**: Publish to `mapping-errors` topic with input data and error
4. **Output Topic Missing**: Publish to `mapping-errors` topic
5. **Kafka Publish Failure**: Log error, message may be reprocessed on restart

All errors include:
- Stage where error occurred
- Error message
- Input data (for debugging)
- Trace context (for correlation)

## Performance Optimizations

### Benchmark Results

From internal benchmarks ([IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md)):

- **Throughput**: 308,000 messages/second
- **Latency**: ~2μs per message (simple mapping)
- **Memory**: 58% reduction with object pooling

### Optimization Strategies

1. **Mapping Compilation**: Mappings are compiled once and reused (validation, step creation, link sorting)
2. **Object Pooling**: Reusable object pools for `SourceFieldValue` and `TargetFieldValue` reduce allocations
3. **LRU Cache**: Compiled mappings cached with TTL (5 minutes, max 1000 entries)
4. **Worker Concurrency**: Configurable worker count (default: 4) for parallel message processing
5. **Batch Publishing**: Kafka producer batches messages (100 per batch, 100ms timeout)
6. **Binding Pre-load**: Bindings loaded into memory at startup and refreshed periodically

### Configuration for Performance

```bash
# Increase worker count for higher throughput
PROCESSOR_WORKER_COUNT=8

# Increase mapping cache size for large mapping sets
MAPPING_CACHE_MAX_SIZE=5000
MAPPING_CACHE_TTL_SECONDS=600

# Tune Kafka producer batching
KAFKA_BATCH_SIZE=500
KAFKA_BATCH_TIMEOUT_MS=50
```

## Database Schema

### Mapping Definitions Table

Stores mapping definitions with source fields, target fields, steps, and links:

```sql
CREATE TABLE mapping_definitions (
    id UUID PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255),
    name VARCHAR(255) NOT NULL,
    key VARCHAR(255),
    description TEXT,
    tags TEXT[],
    version INT NOT NULL DEFAULT 1,
    is_active BOOLEAN NOT NULL DEFAULT true,
    source_fields JSONB NOT NULL,
    target_fields JSONB NOT NULL,
    steps JSONB,
    links JSONB NOT NULL,
    created_ts TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_ts TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, key)
);
```

### Bindings Table

Stores message-to-mapping routing rules:

```sql
CREATE TABLE bindings (
    id UUID PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    mapping_id UUID NOT NULL REFERENCES mapping_definitions(id),
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    output_topic VARCHAR(255) NOT NULL,
    filter JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Filter JSONB Structure**:
```json
{
  "integration": "string",
  "keys": ["uuid1", "uuid2"],
  "status_codes": [200, 201],
  "min_status_code": 200,
  "max_status_code": 299,
  "step_path_prefix": "string",
  "request_url_contains": "string"
}
```

## Getting Started

### Prerequisites

- Go 1.21+
- PostgreSQL 15+
- Apache Kafka 3.0+

### Environment Configuration

Create a `.env` file:

```bash
# Application
APP_NAME=lotus
PORT=8080
LOG_LEVEL=info
PRETTY_LOGS=true

# Database
DATABASE_URL=postgres://user:password@localhost:5432/lotus?sslmode=disable
RUN_MIGRATIONS=true

# Kafka
KAFKA_BROKERS=localhost:9092
KAFKA_INPUT_TOPIC=api-responses
KAFKA_OUTPUT_TOPIC=mapped-data
KAFKA_ERROR_TOPIC=mapping-errors
KAFKA_CONSUMER_GROUP=lotus-consumer
KAFKA_CONSUMER_ENABLED=true
KAFKA_BATCH_SIZE=100
KAFKA_BATCH_TIMEOUT_MS=100
KAFKA_REQUIRED_ACKS=1
KAFKA_COMPRESSION=snappy

# Processing
PROCESSOR_WORKER_COUNT=4
PROCESSOR_TIMEOUT_SECONDS=30

# Caching
BINDING_REFRESH_INTERVAL_MS=60000
MAPPING_CACHE_MAX_SIZE=1000
MAPPING_CACHE_TTL_SECONDS=300

# Authentication
AUTH_ENABLED=false
AUTH_ISSUER_URL=
AUTH_CLIENT_ID=

# CORS
ALLOW_ORIGINS=http://localhost:3000
ALLOW_METHODS=GET,POST,PUT,DELETE,OPTIONS
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
   curl http://localhost:8080/api/v1/health
   ```

### Docker Setup

```bash
docker-compose -f docker-compose.dev.yml up -d
```

Includes: PostgreSQL, Kafka, Zookeeper

## Development

### Project Structure

```
lotus/
├── cmd/                        # Application entry point
│   ├── main.go                # HTTP server setup
│   ├── startup.go             # Database, Kafka, binding loader initialization
│   ├── dependency.go          # DI container registration
│   ├── routes.go              # Route registration
│   └── middleware.go          # HTTP middleware
│
├── pkg/                       # Core business logic
│   ├── mapping/              # Transformation engine
│   │   ├── mapping.go        # Core mapping execution
│   │   ├── pool.go           # Object pooling
│   │   └── *_test.go         # Unit tests and benchmarks
│   │
│   ├── kafka/                # Kafka integration
│   │   ├── consumer.go       # Message consumer
│   │   ├── producer.go       # Message producer
│   │   ├── config.go         # Kafka configuration
│   │   └── message.go        # Message types
│   │
│   ├── processor/            # Pipeline orchestration
│   │   ├── processor.go      # Main message processor
│   │   ├── binding_loader.go # Dynamic binding loading
│   │   └── mapping_cache.go  # Compiled mapping cache
│   │
│   ├── binding/              # Message routing
│   │   └── matcher.go        # Binding matcher with scoring
│   │
│   ├── actions/              # Transformation functions
│   │   ├── text_actions.go   # String operations
│   │   ├── number_actions.go # Numeric operations
│   │   ├── array_actions.go  # Array operations
│   │   └── registry.go       # Action registry
│   │
│   ├── fields/               # Field definitions
│   ├── steps/                # Transformation steps
│   ├── links/                # Data flow connections
│   ├── models/               # Domain models
│   └── routes/               # HTTP handlers
│
├── internal/                 # Private packages
│   ├── repositories/         # Data access layer
│   └── services/            # Business logic services
│
├── config/                   # Configuration
├── db/pg/                    # Database migrations
└── test/integration/         # Integration tests
```

### Testing

```bash
# Unit tests
go test ./...

# With coverage
go test -cover ./...

# Benchmarks
go test -bench=. ./pkg/mapping/
```

### Adding New Transformation Actions

1. Implement action function in appropriate file (e.g., `pkg/actions/text_actions.go`)
2. Register action in `pkg/actions/registry.go`
3. Define action metadata (name, parameters, input/output types)
4. Add unit tests

Example:
```go
// pkg/actions/text_actions.go
func TextReverse(input string) string {
    runes := []rune(input)
    for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
        runes[i], runes[j] = runes[j], runes[i]
    }
    return string(runes)
}

// pkg/actions/registry.go
registry.Register("text_reverse", TextReverse)
```

## Architecture Decisions

1. **Declarative Mappings**: JSON-based definitions enable non-developer configuration and version control
2. **Link-Based Data Flow**: Explicit links provide clear data lineage and enable complex transformations
3. **Compiled Mappings**: Compilation validates mappings once and enables fast execution
4. **Object Pooling**: Reduces GC pressure in high-throughput scenarios
5. **LRU Cache**: Balances memory usage with database load for frequently used mappings
6. **Binding Scoring**: Enables flexible message routing with conflict resolution
7. **Batch Record Extraction**: Automatically handles array responses without manual splitting
8. **Trace Propagation**: OpenTelemetry context flows through entire pipeline for observability

## Performance Characteristics

- **Latency**: Sub-millisecond transformation for simple mappings
- **Throughput**: 300K+ messages/second on modern hardware
- **Memory**: Low allocation rate with object pooling
- **Scalability**: Horizontal scaling via Kafka consumer groups
- **Cache Hit Rate**: >95% for stable mapping configurations

## License

Proprietary - Meadow Platform
