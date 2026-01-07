# Ivy

**Entity Resolution and Graph Persistence Layer**

Ivy is the third and final stage of the Meadow data integration pipeline. It consumes normalized entity data from Lotus, performs entity resolution (deduplication and matching), merges matched entities into canonical "golden records," and persists them to a graph database while emitting lifecycle events for downstream consumers.

## Overview

```
Orchid (Extract) → Lotus (Transform) → Ivy (Merge & Deduplicate) → Graph Database + Event Stream
```

### Core Responsibilities

1. **Consume** - Ingest normalized entity and relationship data from Lotus via Kafka
2. **Stage** - Store raw entities in PostgreSQL with fingerprint-based change detection
3. **Match** - Find duplicate entities using configurable deterministic and probabilistic rules
4. **Merge** - Combine matched entities into canonical records using field-level merge strategies
5. **Persist** - Write merged entities and relationships to Memgraph/Neo4j graph database
6. **Emit** - Publish entity lifecycle events to Kafka for downstream consumers

### Key Features

- **Multi-Strategy Matching**: Deterministic (exact match) and probabilistic (fuzzy, phonetic, similarity-based) entity matching
- **Field-Level Merge Strategies**: 13 configurable strategies (most_recent, most_trusted, collect_all, etc.)
- **Criteria-Based Relationships**: Dynamic relationship creation based on entity attributes
- **Manual Review Queue**: Match candidates below auto-merge threshold require human approval
- **Deletion Strategies**: Explicit deletes, execution-based, staleness-based, and composite strategies
- **Multi-Tenancy**: Complete tenant isolation with Citus-ready schema design
- **Change Detection**: SHA256 fingerprinting prevents duplicate merge work
- **Graph Database Integration**: Cypher query passthrough and graph traversal APIs

## Architecture

### Technology Stack

| Layer | Technology |
|-------|-----------|
| **Web Framework** | Echo v4 (HTTP), Ectoinject (DI) |
| **Database** | PostgreSQL with Citus extension (single-node mode) |
| **Extensions** | pg_trgm (fuzzy matching), fuzzystrmatch (phonetic matching) |
| **Graph Database** | Memgraph / Neo4j (Bolt protocol) |
| **Message Queue** | Apache Kafka (segmentio/kafka-go) |
| **Matching Algorithms** | Jaro-Winkler, Levenshtein, Soundex, Metaphone, Trigram |
| **Observability** | OpenTelemetry, Zap structured logging |
| **Configuration** | Environment variables (Ectoinject) |

### System Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              Kafka Topics                                │
│  mapped-data ─────────────────────────────┐                             │
│  ivy.public.staged_entities (CDC) ────┐   │                             │
│  ivy.public.staged_relationships (CDC)│   │                             │
└───────────────────────────────────────┼───┼─────────────────────────────┘
                                        │   │
                                        ▼   ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                           Ivy Service (Go)                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────────┐  │
│  │  Ingestion   │  │    Merge     │  │    Relationship              │  │
│  │  Consumer    │  │  Consumer    │  │    Consumer                  │  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────────────────────┘  │
│         │                 │                 │                            │
│         ▼                 ▼                 ▼                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────────┐  │
│  │  Processor   │  │MergeProcessor│  │ RelationshipProcessor        │  │
│  │  (Staging)   │  │  (Matching & │  │ (Relationship Merging)       │  │
│  │              │  │   Merging)   │  │                              │  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────────────────────┘  │
│         │                 │                 │                            │
└─────────┼─────────────────┼─────────────────┼────────────────────────────┘
          │                 │                 │
          ▼                 ▼                 ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      PostgreSQL (Citus)                                  │
│  ┌─────────────────┐  ┌──────────────────┐  ┌────────────────────────┐ │
│  │ staged_entities │  │ merged_entities  │  │ staged_relationships   │ │
│  │ staged_entity_  │  │ entity_clusters  │  │ merged_relationships   │ │
│  │ match_fields    │  │ match_candidates │  │ relationship_clusters  │ │
│  └─────────────────┘  └──────────────────┘  └────────────────────────┘ │
│         │                       │                       │                │
│         └───────────Debezium CDC│───────────────────────┘                │
└───────────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
                    ┌──────────────────────────┐
                    │  Memgraph / Neo4j Graph  │
                    │  Database (Bolt)         │
                    └──────────────────────────┘
                                  │
                                  ▼
                    ┌──────────────────────────┐
                    │  Kafka: entity-events    │
                    │  (Downstream Consumers)  │
                    └──────────────────────────┘
```

### Core Components

| Component | Location | Responsibility |
|-----------|----------|----------------|
| **Ingestion Consumer** | [pkg/kafka/consumer.go](pkg/kafka/consumer.go) | Consumes `mapped-data` from Lotus |
| **Processor** | [pkg/processor/processor.go](pkg/processor/processor.go) | Stages entities, validates schemas, handles deletions |
| **Merge Consumer** | [pkg/processor/merge_processor.go](pkg/processor/merge_processor.go) | Processes CDC events, orchestrates entity merging |
| **Relationship Consumer** | [pkg/processor/relationship_processor.go](pkg/processor/relationship_processor.go) | Processes relationship CDC events, merges relationships |
| **Match Engine** | [pkg/matching/service.go](pkg/matching/service.go) | Finds duplicate entities using match rules and index |
| **Merge Engine** | [pkg/merging/engine.go](pkg/merging/engine.go) | Applies field-level merge strategies to create golden records |
| **Graph Writer** | [pkg/graph/entity.go](pkg/graph/entity.go), [pkg/graph/relationship.go](pkg/graph/relationship.go) | Persists merged data to graph database |
| **Event Emitter** | [pkg/events/emitter.go](pkg/events/emitter.go) | Publishes entity/relationship events to `entity-events` topic |
| **Deletion Engine** | [pkg/deletion/engine.go](pkg/deletion/engine.go) | Executes deletion strategies (explicit, execution-based, staleness) |
| **Schema Validator** | [pkg/schema/validator.go](pkg/schema/validator.go) | Validates entity data against JSON schemas |
| **Criteria Evaluator** | [pkg/processor/processor.go](pkg/processor/processor.go) | Creates relationships based on entity criteria matching |

## HTTP API Endpoints

All endpoints are served under `/api/v1` with tenant-based authentication.

### Configuration Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/entity-types` | List all entity types (paginated) |
| POST | `/api/v1/entity-types` | Create new entity type with JSON schema |
| GET | `/api/v1/entity-types/:id` | Get single entity type by ID |
| PUT | `/api/v1/entity-types/:id` | Update entity type |
| DELETE | `/api/v1/entity-types/:id` | Soft delete entity type |
| GET | `/api/v1/entity-types/:key/schema` | Export entity type schema (Lotus format) |
| GET | `/api/v1/relationship-types` | List all relationship types |
| POST | `/api/v1/relationship-types` | Create new relationship type |
| GET | `/api/v1/relationship-types/:id` | Get single relationship type |
| PUT | `/api/v1/relationship-types/:id` | Update relationship type |
| DELETE | `/api/v1/relationship-types/:id` | Soft delete relationship type |
| GET | `/api/v1/match-rules` | List match rules (requires `entity_type` query param) |
| POST | `/api/v1/match-rules` | Create match rule |
| PUT | `/api/v1/match-rules/:id` | Update match rule |
| DELETE | `/api/v1/match-rules/:id` | Delete match rule |
| GET | `/api/v1/deletion-strategies` | List deletion strategies (filter by entity/relationship type) |
| POST | `/api/v1/deletion-strategies` | Create deletion strategy |
| PUT | `/api/v1/deletion-strategies/:id` | Update deletion strategy |
| DELETE | `/api/v1/deletion-strategies/:id` | Delete deletion strategy |

### Data Query Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/entities/:entityType/:id` | Get merged entity (tries graph DB first, falls back to PostgreSQL) |
| GET | `/api/v1/entities/:entityType/:id/sources` | Get source entities that were merged into this entity |
| GET | `/api/v1/entities/:entityType/:id/relationships` | Get all relationships for entity (supports direction filter) |

### Review Queue Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/match-candidates` | List match candidates (filter by status, entity_id) |
| GET | `/api/v1/match-candidates/:id` | Get specific match candidate |
| POST | `/api/v1/match-candidates/:id/approve` | Approve match for merging |
| POST | `/api/v1/match-candidates/:id/reject` | Reject match candidate |
| POST | `/api/v1/match-candidates/:id/defer` | Defer match for later review |

### Graph Query Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/api/v1/graph/query` | Execute read-only OpenCypher query with parameters |
| GET | `/api/v1/graph/path` | Find shortest path between two entities |
| GET | `/api/v1/graph/neighbors/:entityType/:entityId` | Find all connected entities (configurable hop depth) |

### Validation & Health Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/api/v1/validate/validate` | Validate entity data against schema |
| GET | `/api/v1/health` | Overall health with database/redis checks |
| GET | `/api/v1/health/live` | Liveness probe |
| GET | `/api/v1/health/ready` | Readiness probe |
| GET | `/api/v1/status` | Simple status check |

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

#### 1. `mapped-data` Topic (from Lotus)

**Consumer Group**: `ivy-consumer`
**Purpose**: Ingest normalized entity and relationship data from Lotus mappings

**Message Format**:
```json
{
  "source": {
    "integration": "salesforce",
    "source_id": "00Q5e000001xyz",
    "source_key": "leads_plan"
  },
  "tenant_id": "tenant-123",
  "execution_id": "exec-456",
  "source_key": "leads_plan",
  "config_id": "config-789",
  "integration": "salesforce",
  "binding_id": "binding-abc",
  "mapping_id": "mapping-def",
  "mapping_version": 1,
  "timestamp": "2024-01-15T10:30:00Z",
  "target_schema": {
    "type": "entity",
    "key": "person"
  },
  "data": {
    "first_name": "John",
    "last_name": "Doe",
    "email": "john.doe@example.com"
  },
  "relationships": [
    {
      "type": "works_at",
      "to_entity_type": "company",
      "to_source_id": "001xyz",
      "data": {
        "title": "Software Engineer"
      }
    }
  ]
}
```

#### 2. `ivy.public.staged_entities` Topic (Debezium CDC)

**Consumer Group**: `ivy-merge-consumer`
**Purpose**: React to staged entity changes to trigger entity matching and merging

**Message Format**:
```json
{
  "schema": {...},
  "payload": {
    "before": null,
    "after": {
      "id": "staged-entity-123",
      "tenant_id": "tenant-123",
      "entity_type": "person",
      "source_id": "00Q5e000001xyz",
      "integration": "salesforce",
      "source_key": "leads_plan",
      "config_id": "config-789",
      "data": "{\"first_name\":\"John\",\"last_name\":\"Doe\"}",
      "fingerprint": "abc123...",
      "created_at": "2024-01-15T10:30:00Z",
      "updated_at": "2024-01-15T10:30:00Z"
    },
    "source": {...},
    "op": "c",
    "ts_ms": 1705315800000
  }
}
```

**Operations**: `c` (create), `u` (update), `d` (delete), `r` (read/snapshot)

#### 3. `ivy.public.staged_relationships` Topic (Debezium CDC)

**Consumer Group**: `ivy-relationship-consumer`
**Purpose**: Process relationship changes to merge and resolve relationships

**Message Format**: Similar to staged_entities CDC format, with relationship-specific fields

### Messages Produced

#### `entity-events` Topic

**Purpose**: Publish entity and relationship lifecycle events for downstream consumers

**Entity Event Types**:
- `entity.created` - New merged entity created
- `entity.updated` - Existing merged entity updated
- `entity.deleted` - Merged entity deleted
- `entity.merged` - Multiple source entities merged into one

**Relationship Event Types**:
- `relationship.created` - New merged relationship created
- `relationship.updated` - Merged relationship updated
- `relationship.deleted` - Merged relationship deleted

**Entity Event Format**:
```json
{
  "event_type": "entity.merged",
  "tenant_id": "tenant-123",
  "entity_id": "merged-entity-456",
  "entity_type": "person",
  "data": {
    "first_name": "John",
    "last_name": "Doe",
    "email": "john.doe@example.com"
  },
  "source_entities": ["staged-entity-123", "staged-entity-124"],
  "version": 2,
  "timestamp": "2024-01-15T10:31:00Z"
}
```

**Message Headers**: `event_type`, `tenant_id`, `entity_type`, `schema_version`

### Kafka Configuration

**Environment Variables**:

```bash
# Broker Configuration
KAFKA_BROKERS=localhost:9092

# Ingestion Consumer
KAFKA_INPUT_TOPIC=mapped-data
KAFKA_CONSUMER_GROUP=ivy-consumer
KAFKA_CONSUMER_ENABLED=true

# Merge Consumers (CDC)
KAFKA_STAGED_ENTITIES_TOPIC=ivy.public.staged_entities
KAFKA_STAGED_RELATIONSHIPS_TOPIC=ivy.public.staged_relationships
KAFKA_MERGE_CONSUMER_GROUP=ivy-merge-consumer
KAFKA_RELATIONSHIP_CONSUMER_GROUP=ivy-relationship-consumer

# Event Producer
KAFKA_OUTPUT_TOPIC=entity-events
KAFKA_BATCH_SIZE=100
KAFKA_BATCH_TIMEOUT_MS=100
KAFKA_REQUIRED_ACKS=1
KAFKA_COMPRESSION=snappy  # Options: gzip, lz4, zstd, snappy, none
```

## Processing Flow

### 1. Ingestion Pipeline

```
Lotus Message (mapped-data)
    ↓
[Processor.ProcessMessage]
    ↓
Schema Validation
    ↓
Fingerprint Calculation (SHA256)
    ↓
Upsert staged_entities (if fingerprint changed)
    ↓
Extract & Normalize Match Fields → staged_entity_match_fields
    ↓
Create Staged Relationships (direct + criteria-based)
    ↓
Evaluate Criteria → Backfill Relationships
    ↓
[PostgreSQL Write Complete]
    ↓
Debezium CDC Detects Change
    ↓
[Merge Consumer Triggered]
```

### 2. Match Engine Workflow

```
[MatchEngine.IndexEntity]
    ↓
Load Active Match Rules for Entity Type
    ↓
Extract Fields from Entity JSON (dot notation)
    ↓
Normalize Values:
  • exact: lowercase, trimmed
  • fuzzy: trigrams for pg_trgm
  • phonetic: soundex, metaphone
  • numeric: parsed numbers
  • date_range: date parsing
    ↓
Upsert staged_entity_match_fields
    ↓
[Match Index Ready]

[MatchEngine.FindMatches]
    ↓
Load Match Rules (sorted by priority)
    ↓
For Each Rule:
  • Query match index with anchor condition
  • Score candidates using Jaro-Winkler, Levenshtein, etc.
  • Filter by min_match_score threshold
    ↓
Aggregate Scores (weighted by rule priority)
    ↓
Return Matches Above Threshold
```

### 3. Merge Engine Workflow

```
[MergeEngine.MergeWithMatches]
    ↓
Receive Staged Entity + Match Results
    ↓
Load Entity Type Schema (merge strategies)
    ↓
Filter Blocked Matches (no-merge rules)
    ↓
If No Matches:
  • Create Single Merged Entity
  • Link to Entity Cluster
Else:
  • Load All Source Entities
  • Apply Field Merge Strategies:
    - most_recent
    - most_trusted
    - collect_all
    - longest_string
    - highest_number
    - custom_priority
    - etc. (13 total strategies)
  • Resolve Conflicts
  • Create/Update Merged Entity
  • Update Entity Clusters
    ↓
Persist to Graph Database (nodes + properties)
    ↓
Emit Entity Event (entity.created / entity.merged / entity.updated)
    ↓
[Event Published to entity-events Topic]
```

### 4. Deletion Processing

```
Deletion Message / Staleness Check / Execution Absence
    ↓
[DeletionEngine.Execute]
    ↓
Apply Deletion Strategy:
  • Explicit: Direct delete message from Lotus
  • ExecutionBased: Entity not in latest execution
  • Staleness: Entity not updated in N days
  • Composite: Combination of above
    ↓
Soft Delete Staged Entity (set deleted_at)
    ↓
Soft Delete Staged Relationships (cascade)
    ↓
Debezium CDC Detects Deletion
    ↓
Remove from Merged Entities
    ↓
Delete from Graph Database
    ↓
Emit entity.deleted Event
```

## Database Schema

### Key Tables

#### Entity Management

- **entity_types**: Schema definitions with JSON schemas, versioning, identity field declarations
- **staged_entities**: Raw entities before merging with fingerprint-based change detection
  - UNIQUE constraint: `(tenant_id, entity_type, source_id, integration, source_key, config_id)`
- **staged_entity_match_fields**: Denormalized match index with normalized values (exact, fuzzy, phonetic, numeric, date_range)
- **merged_entities**: Canonical "golden records" after merge strategies applied
- **entity_clusters**: Links staged entities to merged entities for lineage tracking

#### Relationship Management

- **relationship_types**: Schema definitions for relationships between entity types
- **staged_relationships**: Raw relationships (direct or criteria-materialized)
- **staged_relationship_criteria**: Criteria definitions for dynamic relationship creation
- **merged_relationships**: Deduplicated canonical relationships
- **relationship_clusters**: Links staged relationships to merged relationships

#### Matching & Review

- **match_rules**: User-defined matching rules with conditions, operators, and scoring weights
- **match_candidates**: Pending matches below auto_merge_threshold awaiting manual review

#### Deletion

- **deletion_strategies**: Configure how entity/relationship deletions are handled

### Performance Optimizations

- **Denormalized Match Index**: `staged_entity_match_fields` table with specialized indexes (GIN for fuzzy, B-tree for exact)
- **Tenant Isolation**: All queries filter by `tenant_id` first (Citus distribution key ready)
- **pg_trgm Extension**: Fast trigram-based fuzzy matching
- **Fingerprint Deduplication**: SHA256 hashing prevents redundant merge operations
- **Partial Indexes**: On `deleted_at IS NULL` for active record queries

## Getting Started

### Prerequisites

- Go 1.21+
- PostgreSQL 15+ with extensions: `pg_trgm`, `fuzzystrmatch`, `citus` (optional)
- Memgraph or Neo4j (graph database)
- Apache Kafka 3.0+
- Debezium PostgreSQL Connector (for CDC)

### Environment Configuration

Create a `.env` file:

```bash
# Application
APP_NAME=ivy
PORT=8080
LOG_LEVEL=info
PRETTY_LOGS=true

# Database
DATABASE_URL=postgres://user:password@localhost:5432/ivy?sslmode=disable

# Graph Database
GRAPH_DB_URI=bolt://localhost:7687
GRAPH_DB_USER=
GRAPH_DB_PASSWORD=

# Kafka
KAFKA_BROKERS=localhost:9092
KAFKA_INPUT_TOPIC=mapped-data
KAFKA_CONSUMER_GROUP=ivy-consumer
KAFKA_CONSUMER_ENABLED=true
KAFKA_STAGED_ENTITIES_TOPIC=ivy.public.staged_entities
KAFKA_STAGED_RELATIONSHIPS_TOPIC=ivy.public.staged_relationships
KAFKA_MERGE_CONSUMER_GROUP=ivy-merge-consumer
KAFKA_RELATIONSHIP_CONSUMER_GROUP=ivy-relationship-consumer
KAFKA_OUTPUT_TOPIC=entity-events

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

3. **Install PostgreSQL Extensions**:
   ```sql
   CREATE EXTENSION IF NOT EXISTS pg_trgm;
   CREATE EXTENSION IF NOT EXISTS fuzzystrmatch;
   ```

4. **Start the Service**:
   ```bash
   go run cmd/main.go
   ```

5. **Verify Health**:
   ```bash
   curl http://localhost:8080/api/v1/health
   ```

### Docker Setup

```bash
docker-compose up -d
```

Includes: PostgreSQL, Memgraph, Kafka, Zookeeper, Debezium Connect

## Development

### Project Structure

```
ivy/
├── cmd/                    # Application entry point, startup, routing, middleware
├── config/                 # Configuration management
├── db/pg/                  # PostgreSQL migrations
├── internal/repositories/  # Data access layer (10+ repositories)
├── pkg/
│   ├── kafka/             # Kafka consumer/producer, message parsing
│   ├── matching/          # Match engine, scoring algorithms
│   ├── merging/           # Merge engine, field merge strategies
│   ├── graph/             # Graph database client (Bolt protocol)
│   ├── processor/         # Message processors (ingestion, merge, relationships)
│   ├── events/            # Event emission
│   ├── deletion/          # Deletion strategy execution
│   ├── schema/            # JSON schema validation
│   ├── criteria/          # Criteria-based relationship evaluation
│   ├── normalizers/       # Field normalization (phone, email, dates)
│   ├── extractor/         # Nested field extraction (dot notation)
│   ├── fingerprint/       # Change detection hashing
│   ├── models/            # Domain models
│   └── routes/            # HTTP handlers
└── test/integration/       # Integration tests
```

### Key Packages

- **[pkg/processor](pkg/processor)**: Ingestion layer, writes to staged tables
- **[pkg/matching](pkg/matching)**: Entity matching with rules and index maintenance
- **[pkg/merging](pkg/merging)**: Field-level merge strategies and conflict resolution
- **[pkg/graph](pkg/graph)**: Graph database operations (Memgraph/Neo4j)
- **[pkg/kafka](pkg/kafka)**: Kafka consumer/producer abstractions
- **[internal/repositories](internal/repositories)**: Database access for all domain models

### Testing

```bash
# Unit tests
go test ./...

# Integration tests
go test ./test/integration/...

# With coverage
go test -cover ./...
```

## Architecture Decisions

1. **Staging Layer First**: All ingestion writes to PostgreSQL staging tables before merging. This enables:
   - Audit trail of all source data
   - Out-of-order message handling
   - Reprocessing without re-ingestion

2. **CDC-Driven Merging**: Separate Debezium consumers react to staged table changes. This decouples:
   - Data ingestion (fast write path)
   - Entity resolution (compute-intensive)
   - Relationship merging (graph operations)

3. **Denormalized Match Index**: Precomputed `staged_entity_match_fields` table enables fast matching without JSON queries

4. **Multi-Tenancy Ready**: All tables include `tenant_id` with proper indexing for Citus horizontal scaling

5. **Fingerprinting**: SHA256 hashing of entity data prevents redundant merge operations when data hasn't changed

6. **Soft Deletes**: Maintains complete audit trail with `deleted_at` timestamps

7. **Field-Level Merge Control**: Fine-grained merge strategies per entity field enable flexible conflict resolution

## License

Proprietary - Meadow Platform
