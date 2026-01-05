# Lotus Implementation Plan

This document outlines the implementation plan for building Lotus into a Kafka-based data mapping service that consumes messages from Orchid and other sources, applies user-defined mappings, and outputs transformed data.

---

## Overview

**What Lotus Does:**

- Consumes API response messages from Kafka (primarily from Orchid)
- Matches messages to user-defined mapping configurations ("bindings")
- Applies data transformations using the mapping engine
- Outputs mapped data to downstream Kafka topics

**Architecture:**

```
                         ┌─────────────────────────────────────────────┐
                         │                   Lotus                     │
                         │                                             │
┌──────────┐   consume   │  ┌──────────┐    ┌────────────┐            │   produce   ┌────────────┐
│  Orchid  │────────────▶│  │ Consumer │───▶│ Processor  │            │────────────▶│  Output    │
│  Kafka   │             │  └──────────┘    └─────┬──────┘            │             │  Kafka     │
│  Topic   │             │                        │                    │             │  Topic     │
└──────────┘             │                        ▼                    │             └────────────┘
                         │               ┌────────────────┐            │
                         │               │ Binding Matcher│            │
                         │               └───────┬────────┘            │
                         │                       │                     │
                         │                       ▼                     │
                         │           ┌───────────────────┐             │
                         │           │ Mapping Executor  │             │
                         │           │  (existing core)  │             │
                         │           └───────────────────┘             │
                         │                       │                     │
                         │                       ▼                     │
                         │              ┌───────────────┐              │
                         │              │   PostgreSQL  │              │
                         │              │  (mappings,   │              │
                         │              │   bindings)   │              │
                         │              └───────────────┘              │
                         └─────────────────────────────────────────────┘
```

---

## Current State

### ✅ Already Implemented

| Component           | Location                                   | Description                                        |
| ------------------- | ------------------------------------------ | -------------------------------------------------- |
| Mapping Engine      | `pkg/mapping/`                             | Core mapping execution with source/target fields   |
| Fields System       | `pkg/fields/`                              | Field definitions with types, paths, validation    |
| Steps System        | `pkg/steps/`                               | Step execution (transformer, validator, condition) |
| Links System        | `pkg/links/`                               | Connect fields to steps and steps to fields        |
| Actions (POC)       | `pkg/actions/`                             | Transformation functions (see note below)          |
| Mapping Definitions | `internal/repositories/mappingdefinition/` | CRUD for mapping definitions                       |
| REST API            | `pkg/routes/`                              | API for mappings and actions                       |
| Database            | `pkg/database/`                            | PostgreSQL with migrations                         |
| Multi-tenancy       | Built-in                                   | `tenant_id` on all resources                       |

### ⚠️ Actions - Current Limitations

The current actions are **proof-of-concept** implementations with limited functionality:

**Text Actions:**

- `concat`, `contains`, `ends_with`, `equals`, `index_of`, `length`
- `replace`, `reverse`, `split`, `starts_with`
- `to_array`, `to_bool`, `to_lower`, `to_upper`, `to_number`
- `min_length`, `max_length`, `allowed_char_count`, `required_char_count`

**Number Actions:**

- `add`, `subtract`, `multiply`, `divide`, `modulus`
- `min`, `max`, `round`, `floor`, `ceiling`
- `square`, `cube`, `square_root`, `cube_root`, `root`, `exponent`
- `factorial`, `to_positive`, `to_negative`, `to_string`, `equals`

**Array Actions:**

- `length`, `contains`, `distinct`, `every`, `index_of`
- `push`, `randomize`, `reverse`, `skip`, `take`

**TODO - Actions to Add:**

- [ ] Date/time parsing and formatting
- [ ] JSON path extraction
- [ ] Regex matching and extraction
- [ ] Conditional/ternary operations
- [ ] Null coalescing
- [ ] Type coercion improvements
- [ ] Object manipulation (pick, omit, merge)
- [ ] Mathematical functions (abs, sign, clamp)
- [ ] String formatting (pad, trim, truncate)
- [ ] Hash/encoding (md5, sha256, base64)

---

## Implementation Phases

### Phase 1: Performance & Core Optimization ✅ COMPLETED

**Priority: Critical** - Ensure the mapping engine is production-ready before adding Kafka

#### 1.1 Mapping Compilation & Caching ✅

**Problem:** `GenerateMappingPlan()` was called for every message, rebuilding steps each time.

**Solution:** Pre-compile mapping definitions and cache them.

**Files created/modified:**

- `pkg/mapping/mapping.go` - Added `Compile()` method and cached state
- `pkg/mapping/pool.go` - Object pooling for Mapping results

**Completed:**

- [x] Added `Compile()` method to pre-build mapping plan
- [x] Pre-compute source links (filtered once, not every execution)
- [x] Pre-compute target field paths
- [x] `IsCompiled()` check to avoid redundant compilation
- [x] Auto-compile on first ExecuteMapping (backwards compatible)

**Results:**

- Transforms: **71-77% faster** after compilation
- Simple mappings: **26-45% faster**

#### 1.2 Memory Optimization ✅

**Problem:** New allocations for every message execution.

**Completed:**

- [x] `sync.Pool` for `Mapping` result objects (`pool.go`)
- [x] `AcquireMapping()` / `ReleaseMapping()` pool management
- [x] `Reset()` method to clear state for reuse
- [x] `ExecuteMappingPooled()` for automatic pooling
- [x] Pre-allocated maps/slices with capacity hints

**Results:**

- Orchid-like mapping: **58% less memory** (4,288 → 1,792 bytes)
- Allocations: **36% fewer** (39 → 25)

#### 1.3 Benchmarks ✅

**File:** `pkg/mapping/mapping_benchmark_test.go`

**Benchmarks added:**

- [x] `BenchmarkSimpleMapping5/20/50Fields` - Direct field mappings
- [x] `BenchmarkMappingWithTransforms5/20` - Field → Step → Field
- [x] `BenchmarkNestedMapping5/10` - Nested object access
- [x] `BenchmarkArrayMapping100/1000` - Array data
- [x] `BenchmarkOrchidLikeMapping` - Realistic Orchid message
- [x] `BenchmarkPlanGeneration` - Isolated compilation cost
- [x] `BenchmarkParallelMapping` - Concurrent execution
- [x] `BenchmarkPreCompiled*` - Compiled mapping versions
- [x] `BenchmarkPooled*` - Pooled execution versions
- [x] `BenchmarkOrchidFullPipeline` - Full 6-field pipeline

**Achieved metrics:**

| Benchmark            | Result              | Target       |
| -------------------- | ------------------- | ------------ |
| Orchid-like          | 2,080 ns (~2μs)     | ✅ <100μs    |
| Simple 50 fields     | 135,867 ns (~136μs) | ✅ <1ms      |
| Parallel (10 fields) | 11,993 ns (~12μs)   | ✅ Excellent |
| Throughput           | 308K msgs/sec       | ✅ Excellent |

#### 1.4 Integration Tests ✅

**File:** `pkg/mapping/orchid_integration_test.go`

**Tests added:**

- [x] `TestOrchidMetadataExtraction` - Extract tenant_id, plan_id, etc.
- [x] `TestOrchidResponseBodyExtraction` - Nested response_body fields
- [x] `TestOrchidNestedDetailsMapping` - Map user detail responses
- [x] `TestOrchidErrorResponse` - Handle 429 error responses
- [x] `TestOrchidBatchProcessing` - Multiple messages with same mapping
- [x] `TestOrchidJSONSerialization` - Raw JSON from Kafka
- [x] `TestOrchidHighVolume` - 100K messages with throughput assertion
- [x] `BenchmarkOrchidFullPipeline` - Realistic pipeline benchmark

**Results:**

- All tests passing
- 100K messages processed in ~325ms
- **Throughput: 308K messages/second**

---

### Phase 2: Kafka Infrastructure ✅ COMPLETED

**Priority: Critical** (after Phase 1)

#### 2.1 Kafka Consumer ✅

**Files created:**

- `pkg/kafka/config.go` - Consumer and Producer configuration
- `pkg/kafka/consumer.go` - Consumer group with partition handling
- `pkg/kafka/message.go` - Orchid message types and parsing
- `pkg/kafka/message_test.go` - Unit tests

**Completed:**

- [x] Added `segmentio/kafka-go` dependency
- [x] Created `ConsumerConfig` and `ProducerConfig` with defaults
- [x] Implemented consumer group with auto-commit
- [x] Handle partition rebalancing
- [x] Parse Orchid message format (`OrchidMessage` struct)
- [x] Extract trace context from headers (`MessageHeaders`)
- [x] `ToMap()` method for mapping execution

#### 2.2 Kafka Producer ✅

**Files created:**

- `pkg/kafka/producer.go` - Producer for mapped output

**Completed:**

- [x] Create producer with configurable batching
- [x] Support per-binding output topics (`PublishToTopic`)
- [x] Propagate trace context to output
- [x] Handle publish failures with retries
- [x] Batch publishing support (`PublishBatch`)
- [x] `MappedMessage` struct for output format

**Configuration added to `config/config.go`:**

- `KAFKA_BROKERS`, `KAFKA_INPUT_TOPIC`, `KAFKA_CONSUMER_GROUP`
- `KAFKA_OUTPUT_TOPIC`, `KAFKA_CONSUMER_ENABLED`
- `KAFKA_BATCH_SIZE`, `KAFKA_BATCH_TIMEOUT_MS`, `KAFKA_COMPRESSION`

---

### Phase 3: Binding System ✅ COMPLETED

**Priority: Critical**

#### 3.1 Binding Model ✅

**Files created:**

- `pkg/models/binding.go` - Binding model with filter criteria
- `pkg/binding/matcher.go` - Binding matcher for routing messages
- `pkg/binding/matcher_test.go` - Comprehensive unit tests

**Completed:**

- [x] `Binding` model with `BindingFilter` for matching
- [x] Filter by: source type, plan IDs, status codes, step path prefix
- [x] Status code range filtering (min/max)
- [x] `Matcher` with thread-safe binding loading
- [x] Scored matching for priority resolution
- [x] Tests for all matching scenarios

---

### Phase 4: Processing Pipeline ✅ COMPLETED

**Files created:**

- `pkg/processor/processor.go` - Message processing pipeline
- `pkg/processor/mapping_cache.go` - LRU cache for compiled mappings

**Completed:**

- [x] `Processor` that ties everything together
- [x] `MappingCache` with TTL and LRU eviction
- [x] `ProcessMessage()` - full pipeline: match → load → map → publish
- [x] `MessageHandler()` for Kafka consumer integration
- [x] Statistics tracking (processed, matched, failed)
- [x] Cache hit/miss tracking

---

### Phase 5: Binding Repository & API ✅ COMPLETED

A "binding" connects incoming messages to mapping definitions.

**Completed:**

- [x] Database migration (`db/pg/0004_create_bindings.up.sql`)
- [x] DAO layer (`internal/repositories/binding/dao.go`)
- [x] Repository implementation (`internal/repositories/binding/repository.go`)
- [x] API handlers (`pkg/routes/binding/binding.go`)
- [x] Route registration (`cmd/routes.go`)
- [x] DI registration (`cmd/dependency.go`)

**API Endpoints:**

| Method | Endpoint                 | Description       |
| ------ | ------------------------ | ----------------- |
| GET    | `/api/v1.0/bindings`     | List all bindings |
| POST   | `/api/v1.0/bindings`     | Create a binding  |
| GET    | `/api/v1.0/bindings/:id` | Get a binding     |
| PUT    | `/api/v1.0/bindings/:id` | Update a binding  |
| DELETE | `/api/v1.0/bindings/:id` | Delete a binding  |

---

### Phase 6: Startup Integration ✅ COMPLETED

**Priority: Critical**

**Files created/modified:**

- `cmd/startup.go` - Added Kafka startup dependency
- `pkg/processor/binding_loader.go` - Dynamic binding loading from database
- `pkg/processor/mapping_adapter.go` - Adapts mapping repository for cache
- `pkg/routes/health/health.go` - Health check endpoints
- `config/config.go` - Added processor configuration

**Completed:**

- [x] Kafka consumer startup with configurable options
- [x] Kafka producer startup with batching
- [x] Processor pipeline wiring (matcher → cache → executor → producer)
- [x] Dynamic tenant binding loading (on-demand when messages arrive)
- [x] Binding loader with background refresh
- [x] Mapping cache with TTL and LRU eviction
- [x] Graceful shutdown (consumer → loader → producer)
- [x] Health check endpoints (`/health`, `/health/live`, `/health/ready`, `/health/stats`)
- [x] DI registration for runtime components

**Configuration added:**

```go
// Processor
ProcessorWorkerCount     int `env:"PROCESSOR_WORKER_COUNT" env-default:"4"`
ProcessorTimeoutSeconds  int `env:"PROCESSOR_TIMEOUT_SECONDS" env-default:"30"`
BindingRefreshIntervalMs int `env:"BINDING_REFRESH_INTERVAL_MS" env-default:"60000"`
MappingCacheMaxSize      int `env:"MAPPING_CACHE_MAX_SIZE" env-default:"1000"`
MappingCacheTTLSeconds   int `env:"MAPPING_CACHE_TTL_SECONDS" env-default:"300"`
```

**To enable Kafka processing:**

Set `KAFKA_CONSUMER_ENABLED=true` in your environment.

---

### Phase 7: Observability 🔲 NOT STARTED

**Priority: Medium**

#### 7.1 Prometheus Metrics

**File:** `pkg/metrics/metrics.go`

**Metrics to expose:**

- `lotus_messages_consumed_total` - Messages consumed (by tenant, source_type)
- `lotus_messages_processed_total` - Messages processed (by tenant, binding, status)
- `lotus_messages_produced_total` - Messages produced (by tenant, topic)
- `lotus_mapping_duration_seconds` - Mapping execution time
- `lotus_consumer_lag` - Consumer lag per partition
- `lotus_bindings_matched_total` - Bindings matched per message
- `lotus_cache_hits_total` - Mapping cache hits/misses

#### 7.2 Health Checks ✅ COMPLETED (in Phase 6)

**File:** `pkg/routes/health/health.go`

**Endpoints:**

- `GET /health` - Overall health status
- `GET /health/live` - Liveness probe
- `GET /health/ready` - Readiness probe
- `GET /health/stats` - Processor and binding statistics

#### 7.3 Tracing

**Tasks:**

- [x] Extract trace context from Orchid messages
- [x] Propagate trace to output messages
- [ ] Add OTLP exporter (same as Orchid)

---

### Phase 8: API Enhancements 🔲 PARTIAL

**Priority: High**

#### 8.1 Binding CRUD API ✅ COMPLETED (in Phase 5)

**Completed endpoints:**

```
POST   /api/v1.0/bindings           - Create binding
GET    /api/v1.0/bindings           - List bindings
GET    /api/v1.0/bindings/:id       - Get binding by ID
PUT    /api/v1.0/bindings/:id       - Update binding
DELETE /api/v1.0/bindings/:id       - Delete binding
```

#### 8.2 Stats & Monitoring API 🔲 NOT STARTED

**File:** `pkg/routes/stats/stats.go`

**Endpoints to add:**

```
GET /api/v1/stats/processing   - Processing statistics
GET /api/v1/stats/consumer     - Consumer status and lag
GET /api/v1/stats/bindings     - Per-binding statistics
```

---

### Phase 9: End-to-End Testing 🔲 NOT STARTED

**Priority: Medium**

#### 8.1 Kafka Integration Tests

- [ ] Kafka consumer/producer integration
- [ ] End-to-end message processing
- [ ] Binding matching logic
- [ ] Error handling scenarios

#### 8.2 Full Pipeline Tests

- [ ] Expand action unit tests
- [ ] Complex mapping scenarios
- [ ] Performance benchmarks

---

## Implementation Order

| Priority | Phase | Component                        | Estimated Effort |
| -------- | ----- | -------------------------------- | ---------------- |
| 1        | 1.1   | Mapping Compilation & Caching    | 3-4 hours        |
| 2        | 1.2   | Memory Optimization              | 1-2 hours        |
| 3        | 1.3   | Benchmarks                       | 2 hours          |
| 4        | 1.4   | Integration Tests (mapping core) | 2-3 hours        |
| 5        | 2.1   | Kafka Consumer                   | 2-3 hours        |
| 6        | 2.2   | Kafka Producer                   | 1-2 hours        |
| 7        | 3.1   | Binding Model & DB               | 2 hours          |
| 8        | 3.2   | Binding Matcher                  | 2 hours          |
| 9        | 4.1   | Message Processor                | 3-4 hours        |
| 10       | 4.2   | Worker Pool                      | 2 hours          |
| 11       | 5.1   | Config Updates                   | 30 mins          |
| 12       | 7.1   | Binding API                      | 2 hours          |
| 13       | 6.1   | Metrics                          | 1-2 hours        |
| 14       | 6.2   | Health Checks                    | 1 hour           |
| 15       | 6.3   | Tracing                          | 1 hour           |
| 16       | 7.2   | Stats API                        | 1 hour           |
| 17       | 8.x   | End-to-End Testing               | 3-4 hours        |

**Total Estimated: ~28-35 hours**

---

## Dependencies

### Go Libraries to Add

```go
// Kafka
github.com/segmentio/kafka-go

// Metrics
github.com/prometheus/client_golang

// Caching
github.com/hashicorp/golang-lru/v2

// OTLP Tracing (optional)
go.opentelemetry.io/otel/exporters/otlp/otlptrace
```

### External Services

- **Kafka** - Message broker (shared with Orchid or separate)
- **PostgreSQL** - Mapping and binding storage (existing)
- **Jaeger/OTLP** - Trace collection (optional)

---

## Environment Variables

```bash
# Kafka Consumer
KAFKA_CONSUMER_BROKERS=localhost:9092
KAFKA_CONSUMER_TOPICS=api-responses
KAFKA_CONSUMER_GROUP=lotus-consumers
KAFKA_CONSUMER_WORKERS=4

# Kafka Producer
KAFKA_PRODUCER_BROKERS=localhost:9092
KAFKA_PRODUCER_DEFAULT_TOPIC=mapped-data

# Processing
PROCESSOR_ENABLED=true
PROCESSOR_CACHE_SIZE=1000

# Database (existing)
DB_HOST=localhost
DB_PORT=5432
DB_NAME=lotus

# Observability
OTLP_ENABLED=false
OTLP_ENDPOINT=localhost:4317
```

---

## Future Considerations

### Action Expansion

See "Actions - Current Limitations" section above. The actions library needs significant expansion for production use cases.

### Multi-Source Support

The binding system is designed to support sources beyond Orchid:

- Custom Kafka topics
- Webhook ingestion
- File uploads (batch processing)

### Schema Registry

Consider integrating Confluent Schema Registry for:

- Input message validation
- Output schema enforcement
- Schema evolution

### Dead Letter Queue

Add DLQ support for:

- Failed mappings
- Invalid messages
- Binding errors

---

## Getting Started

1. **Optimize mapping core** (Phase 1.1-1.2) - Compilation, caching, memory
2. **Add benchmarks** (Phase 1.3) - Establish performance baselines
3. **Integration test mapping** (Phase 1.4) - Ensure core is solid with Orchid message format
4. **Add Kafka infrastructure** (Phase 2) - Consumer and producer
5. **Build binding system** (Phase 3) - Route messages to mappings
6. **Implement processor** (Phase 4) - Tie it all together
7. **Add observability** (Phase 6) - Metrics, health, tracing
8. **Build APIs** (Phase 7) - Binding management, stats
