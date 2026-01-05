# Ivy Implementation Plan

**Ivy** is the entity resolution and graph persistence layer in the Meadow ecosystem. It consumes mapped data from Lotus, applies user-defined merge strategies for deduplication, and maintains a clean entity graph.

---

## Progress Summary

| Phase | Component                             | Status         | Progress |
| ----- | ------------------------------------- | -------------- | -------- |
| 1     | Project Setup & Core Models           | ✅ Complete    | 100%     |
| 2     | Entity Type & Relationship Management | ✅ Complete    | 100%     |
| 3     | Staging Layer                         | ✅ Complete    | 100%     |
| 4     | Match Engine                          | ✅ Complete    | 100%     |
| 5     | Merge Engine                          | ✅ Complete    | 100%     |
| 6     | Graph Persistence                     | ✅ Complete    | 100%     |
| 7     | Deletion Handling                     | ✅ Complete    | 100%     |
| 8     | Event Emission                        | ✅ Complete    | 100%     |
| 9     | REST API                              | ✅ Complete    | 100%     |
| 10    | Admin & Operations                    | 🔲 Not Started | 0%       |

**Legend:** 🔲 Not Started | 🟡 In Progress | ✅ Complete

**Overall Progress:** 9/10 phases complete

**Last Updated:** Phase 9 complete

---

## Overview

### The Meadow Pipeline

```
┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────────┐
│ Orchid  │────▶│  Lotus  │────▶│   Ivy   │────▶│  Graph DB   │
│ Extract │     │Transform│     │  Merge  │     │ (OpenCypher)│
└─────────┘     └─────────┘     └─────────┘     └─────────────┘
     │               │               │
     └───────────────┴───────────────┴────▶ Kafka Events
```

### What Ivy Does

1. **Consumes** mapped entities and relationships from Lotus (via Kafka)
2. **Stages** raw entities in a PostgreSQL staging database
3. **Matches** entities using user-defined match rules (deterministic + probabilistic)
4. **Merges** entities using configurable merge strategies
5. **Persists** clean, deduplicated entities to a graph database
6. **Emits** entity lifecycle events (created, merged, updated, deleted) to Kafka
7. **Handles** deletions based on configurable strategies

---

## Architecture

```
                              ┌────────────────────────────────────────────────────────┐
                              │                         Ivy                            │
                              │                                                        │
                              │   ┌──────────────┐      ┌──────────────────────┐      │
┌─────────┐    Kafka          │   │   Consumer   │─────▶│   Entity Processor   │      │
│  Lotus  │──────────────────▶│   └──────────────┘      └──────────┬───────────┘      │
│ Output  │                   │                                     │                  │
└─────────┘                   │                    ┌────────────────┼────────────────┐ │
                              │                    │                │                │ │
                              │                    ▼                ▼                ▼ │
                              │          ┌─────────────┐   ┌────────────┐   ┌────────┐│
                              │          │   Staging   │   │   Match    │   │ Merge  ││
                              │          │     DB      │   │   Engine   │   │ Engine ││
                              │          │ (PostgreSQL)│   │            │   │        ││
                              │          └─────────────┘   └────────────┘   └────────┘│
                              │                    │                │                │ │
                              │                    └────────────────┼────────────────┘ │
                              │                                     │                  │
                              │                                     ▼                  │
                              │                           ┌─────────────────┐          │
                              │                           │  Graph Writer   │          │
                              │                           └────────┬────────┘          │
                              │                                    │                   │
                              └────────────────────────────────────┼───────────────────┘
                                                                   │
                                          ┌────────────────────────┼────────────────────┐
                                          │                        │                    │
                                          ▼                        ▼                    ▼
                                   ┌─────────────┐          ┌─────────────┐      ┌───────────┐
                                   │  Graph DB   │          │   Kafka     │      │ REST API  │
                                   │ (Memgraph/  │          │  (Events)   │      │  (Query)  │
                                   │   Neo4j)    │          └─────────────┘      └───────────┘
                                   └─────────────┘
```

---

## Core Concepts

### 1. Entity Types

Users define **entity types** that represent the kinds of things they're tracking:

```json
{
  "id": "uuid",
  "tenant_id": "tenant-123",
  "key": "person",
  "name": "Person",
  "description": "A human individual",
  "schema": {
    "properties": {
      "first_name": { "type": "string" },
      "last_name": { "type": "string" },
      "email": { "type": "string", "format": "email" },
      "phone": { "type": "string" },
      "ssn": { "type": "string" },
      "date_of_birth": { "type": "string", "format": "date" }
    },
    "required": ["first_name", "last_name"]
  },
  "identity_fields": ["email", "ssn", "phone"],
  "display_template": "{{first_name}} {{last_name}}"
}
```

### 2. Relationship Types

Users define **relationship types** between entities:

```json
{
  "id": "uuid",
  "tenant_id": "tenant-123",
  "key": "works_at",
  "name": "Works At",
  "from_entity_type": "person",
  "to_entity_type": "company",
  "cardinality": "many_to_one",
  "properties": {
    "title": { "type": "string" },
    "start_date": { "type": "string", "format": "date" },
    "end_date": { "type": "string", "format": "date" }
  }
}
```

### 3. Match Rules

Match rules define how to identify potentially duplicate entities:

```json
{
  "id": "uuid",
  "tenant_id": "tenant-123",
  "entity_type": "person",
  "name": "Email Exact Match",
  "priority": 1,
  "rule_type": "deterministic",
  "conditions": [
    {
      "field": "email",
      "operator": "equals",
      "options": { "case_insensitive": true }
    }
  ],
  "confidence": 1.0
}
```

```json
{
  "id": "uuid",
  "tenant_id": "tenant-123",
  "entity_type": "person",
  "name": "Fuzzy Name + DOB",
  "priority": 2,
  "rule_type": "probabilistic",
  "conditions": [
    {
      "field": "first_name",
      "operator": "fuzzy_match",
      "options": { "algorithm": "jaro_winkler", "threshold": 0.85 }
    },
    {
      "field": "last_name",
      "operator": "fuzzy_match",
      "options": { "algorithm": "jaro_winkler", "threshold": 0.9 }
    },
    {
      "field": "date_of_birth",
      "operator": "equals"
    }
  ],
  "min_conditions": 2,
  "confidence": 0.85
}
```

### 4. Merge Strategies

Merge strategies define how to combine matched entities:

```json
{
  "id": "uuid",
  "tenant_id": "tenant-123",
  "entity_type": "person",
  "name": "Default Person Merge",
  "field_strategies": {
    "first_name": { "strategy": "most_recent" },
    "last_name": { "strategy": "most_recent" },
    "email": {
      "strategy": "most_trusted",
      "trust_order": ["source_a", "source_b"]
    },
    "phone": { "strategy": "collect_all" },
    "ssn": { "strategy": "first_non_null" },
    "date_of_birth": { "strategy": "most_common" }
  },
  "conflict_resolution": "manual_review",
  "auto_merge_threshold": 0.95
}
```

**Available field strategies:**

- `most_recent` - Use the value from the most recent source record
- `oldest` - Use the value from the oldest source record
- `most_trusted` - Use value from highest-trust source
- `most_common` - Use the most frequently occurring value
- `first_non_null` - Use the first non-null value found
- `collect_all` - Collect all unique values into an array
- `longest` - Use the longest string value
- `custom` - User-defined merge logic (JMESPath expression)

### 5. Deletion Strategies

Users configure how deletions are handled per entity type:

```json
{
  "id": "uuid",
  "tenant_id": "tenant-123",
  "entity_type": "person",
  "strategies": [
    {
      "type": "explicit",
      "enabled": true,
      "description": "Honor explicit delete messages from source"
    },
    {
      "type": "execution_absence",
      "enabled": true,
      "plan_ids": ["plan-123", "plan-456"],
      "grace_period_hours": 24,
      "description": "Delete if not seen in N consecutive executions"
    },
    {
      "type": "staleness",
      "enabled": false,
      "max_age_days": 365,
      "description": "Delete entities not updated in N days"
    }
  ],
  "soft_delete": true,
  "retention_days": 90
}
```

---

## Data Models

### Database Strategy

**PostgreSQL with Match Index Optimization**

```
┌─────────────────────────────────────────────────────────────────────┐
│                        PostgreSQL (Citus)                            │
│                                                                      │
│  ┌────────────────────┐         ┌──────────────────────────────┐    │
│  │  staged_entities   │────────▶│     entity_match_index       │    │
│  │  (JSONB full data) │  1:1    │  (denormalized match fields) │    │
│  └────────────────────┘         └──────────────────────────────┘    │
│           │                                   │                      │
│           │                                   │                      │
│           ▼                                   ▼                      │
│  ┌────────────────────┐         ┌──────────────────────────────┐    │
│  │  Full entity data  │         │  - Normalized text fields    │    │
│  │  Audit trail       │         │  - Phonetic encodings        │    │
│  │  Source tracking   │         │  - Trigram indexes           │    │
│  └────────────────────┘         │  - Fast exact/fuzzy matches  │    │
│                                 └──────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

**Why this approach:**

- **JSONB** for flexible entity schemas (any fields)
- **Denormalized match index** for fast field lookups
- **pg_trgm** extension for fuzzy text matching (`SIMILARITY()`)
- **fuzzystrmatch** for phonetic matching (`soundex()`, `metaphone()`)
- **Citus-compatible** for horizontal scaling (distribute by `tenant_id`)
- **Single database** - no additional infrastructure

**Performance characteristics:**
| Entity Count | Exact Match | Fuzzy Match | Notes |
|--------------|-------------|-------------|-------|
| 100K | < 10ms | < 50ms | Single index scan |
| 1M | < 20ms | < 100ms | Proper indexes critical |
| 10M+ | < 50ms | < 500ms | Consider Elasticsearch |

**Future scaling path:**
If matching becomes a bottleneck at 10M+ entities, add Elasticsearch as a dedicated match index while keeping PostgreSQL as source of truth.

### Staging Database (PostgreSQL)

**Staged Entities** - Raw entities before merging:

```sql
CREATE TABLE staged_entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    source_id TEXT NOT NULL,           -- ID from source system
    source_type TEXT NOT NULL,         -- e.g., "orchid", "api", "manual"
    source_plan_id TEXT,               -- Orchid plan that produced this
    source_execution_id TEXT,          -- Orchid execution that produced this
    data JSONB NOT NULL,               -- The entity data
    fingerprint TEXT NOT NULL,         -- Hash for change detection
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    UNIQUE(tenant_id, entity_type, source_id, source_type)
);

CREATE INDEX idx_staged_entities_tenant_type ON staged_entities(tenant_id, entity_type);
CREATE INDEX idx_staged_entities_fingerprint ON staged_entities(fingerprint);
CREATE INDEX idx_staged_entities_execution ON staged_entities(source_execution_id);
CREATE INDEX idx_staged_entities_data_gin ON staged_entities USING GIN (data);
```

**Entity Match Index** - Denormalized fields for fast matching:

```sql
-- Enables fuzzy text matching
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS fuzzystrmatch;

CREATE TABLE entity_match_index (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    staged_entity_id UUID NOT NULL REFERENCES staged_entities(id) ON DELETE CASCADE,
    entity_type TEXT NOT NULL,

    -- Denormalized match fields (populated based on entity type schema)
    -- These are normalized/lowercased for efficient matching
    field_1 TEXT,                        -- e.g., email_normalized
    field_2 TEXT,                        -- e.g., phone_normalized
    field_3 TEXT,                        -- e.g., first_name_lower
    field_4 TEXT,                        -- e.g., last_name_lower
    field_5 TEXT,                        -- e.g., ssn_normalized

    -- Phonetic encodings for fuzzy name matching
    field_3_soundex TEXT,                -- soundex(first_name)
    field_4_soundex TEXT,                -- soundex(last_name)
    field_3_metaphone TEXT,              -- metaphone(first_name)
    field_4_metaphone TEXT,              -- metaphone(last_name)

    -- Combined text for trigram similarity
    name_combined TEXT,                  -- first_name || ' ' || last_name

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(staged_entity_id)
);

-- Exact match indexes
CREATE INDEX idx_match_tenant_type ON entity_match_index(tenant_id, entity_type);
CREATE INDEX idx_match_field_1 ON entity_match_index(tenant_id, entity_type, field_1) WHERE field_1 IS NOT NULL;
CREATE INDEX idx_match_field_2 ON entity_match_index(tenant_id, entity_type, field_2) WHERE field_2 IS NOT NULL;
CREATE INDEX idx_match_field_3 ON entity_match_index(tenant_id, entity_type, field_3) WHERE field_3 IS NOT NULL;
CREATE INDEX idx_match_field_4 ON entity_match_index(tenant_id, entity_type, field_4) WHERE field_4 IS NOT NULL;
CREATE INDEX idx_match_field_5 ON entity_match_index(tenant_id, entity_type, field_5) WHERE field_5 IS NOT NULL;

-- Phonetic indexes (for fuzzy name matching)
CREATE INDEX idx_match_soundex ON entity_match_index(tenant_id, entity_type, field_3_soundex, field_4_soundex);
CREATE INDEX idx_match_metaphone ON entity_match_index(tenant_id, entity_type, field_3_metaphone, field_4_metaphone);

-- Trigram index for similarity matching
CREATE INDEX idx_match_name_trgm ON entity_match_index USING GIN (name_combined gin_trgm_ops);
```

**Match Field Mappings** - Maps entity type fields to match index columns:

```sql
CREATE TABLE match_field_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    source_path TEXT NOT NULL,           -- JSONPath: "$.email", "$.name.first", "$.contacts[*].value"
    target_column TEXT NOT NULL,         -- e.g., "field_1", "field_3"
    normalizer TEXT,                     -- e.g., "lowercase", "phone", "email"
    array_handling TEXT DEFAULT 'first', -- 'first', 'all', 'filter'
    array_filter JSONB,                  -- For 'filter': {"type": "email"}
    include_phonetic BOOLEAN DEFAULT FALSE,
    include_trigram BOOLEAN DEFAULT FALSE,

    UNIQUE(tenant_id, entity_type, source_path)
);

-- Example mappings for "person" entity type with nested fields:
--
-- Simple field:
-- INSERT INTO match_field_mappings (tenant_id, entity_type, source_path, target_column, normalizer)
-- VALUES ('tenant-1', 'person', '$.email', 'field_1', 'email');
--
-- Nested object field:
-- INSERT INTO match_field_mappings (tenant_id, entity_type, source_path, target_column, normalizer, include_phonetic, include_trigram)
-- VALUES ('tenant-1', 'person', '$.name.first', 'field_3', 'lowercase', true, true);
--
-- Array field (first value):
-- INSERT INTO match_field_mappings (tenant_id, entity_type, source_path, target_column, array_handling)
-- VALUES ('tenant-1', 'person', '$.phones[*]', 'field_2', 'first');
--
-- Array field with filter (get email from contacts array):
-- INSERT INTO match_field_mappings (tenant_id, entity_type, source_path, target_column, array_handling, array_filter, normalizer)
-- VALUES ('tenant-1', 'person', '$.contacts[*].value', 'field_1', 'filter', '{"type": "email"}', 'email');
```

**Nested Field Handling:**

| Pattern        | Example Path                           | Extracted Value                |
| -------------- | -------------------------------------- | ------------------------------ |
| Simple         | `$.email`                              | `"john@example.com"`           |
| Nested object  | `$.name.first`                         | `"John"`                       |
| Array (first)  | `$.phones[0]`                          | `"555-1234"`                   |
| Array (all)    | `$.phones[*]`                          | `"555-1234,555-5678"` (joined) |
| Array (filter) | `$.contacts[?(@.type=='email')].value` | `"john@example.com"`           |

**Array Handling Options:**

- `first` - Use first matching value
- `all` - Concatenate all values (comma-separated)
- `filter` - Apply filter condition, then use first match

**Why this approach:**

- JSONB for flexible storage, denormalized table for fast lookups
- Generic `field_N` columns work for any entity type
- **JSONPath** for nested field extraction (PostgreSQL 12+ native support)
- Mappings table makes it configurable per entity type
- pg_trgm enables `SIMILARITY()` and `%` operator for fuzzy matching
- fuzzystrmatch provides `soundex()` and `metaphone()` for phonetic matching
- All queries filter by `tenant_id` first (Citus distribution key)

**PostgreSQL JSONPath for nested extraction:**

```sql
-- Extract nested value
SELECT jsonb_path_query_first(data, '$.name.first') FROM staged_entities;

-- Extract from array with filter
SELECT jsonb_path_query_first(data, '$.contacts[*] ? (@.type == "email").value') FROM staged_entities;

-- In Go, we'll use JMESPath (same as Lotus) for consistency
import "github.com/jmespath/go-jmespath"
result, _ := jmespath.Search("name.first", entityData)
```

**Staged Relationships** - Raw relationships before merging:

```sql
CREATE TABLE staged_relationships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    relationship_type TEXT NOT NULL,
    from_staged_entity_id UUID REFERENCES staged_entities(id),
    to_staged_entity_id UUID REFERENCES staged_entities(id),
    source_type TEXT NOT NULL,
    source_plan_id TEXT,
    source_execution_id TEXT,
    data JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
```

**Merged Entities** - Canonical entities after merge:

```sql
CREATE TABLE merged_entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    canonical_data JSONB NOT NULL,     -- Merged/golden record
    confidence FLOAT,                   -- Merge confidence score
    source_count INT DEFAULT 1,         -- Number of source records
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    merged_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_merged_entities_tenant_type ON merged_entities(tenant_id, entity_type);
```

**Entity Clusters** - Links staged entities to merged entities:

```sql
CREATE TABLE entity_clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merged_entity_id UUID REFERENCES merged_entities(id),
    staged_entity_id UUID REFERENCES staged_entities(id),
    match_rule_id UUID,                 -- Rule that matched this
    match_confidence FLOAT,
    created_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(staged_entity_id)
);
```

**Match Candidates** - Potential matches pending review:

```sql
CREATE TABLE match_candidates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_a_id UUID NOT NULL,          -- Staged entity ID
    entity_b_id UUID NOT NULL,          -- Staged entity ID or merged entity ID
    match_rule_id UUID,
    confidence FLOAT NOT NULL,
    status TEXT DEFAULT 'pending',      -- pending, approved, rejected
    reviewed_by TEXT,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Graph Database (Memgraph/Neo4j)

**Node: Entity**

```cypher
(:Person {
    ivy_id: "uuid",                    -- Ivy merged entity ID
    tenant_id: "tenant-123",
    first_name: "John",
    last_name: "Doe",
    email: ["john@example.com", "jdoe@work.com"],
    phone: ["+1-555-1234"],
    source_count: 3,
    confidence: 0.95,
    created_at: datetime(),
    updated_at: datetime()
})
```

**Relationship: works_at**

```cypher
(:Person)-[:WORKS_AT {
    ivy_id: "uuid",
    title: "Software Engineer",
    start_date: date("2020-01-15"),
    created_at: datetime()
}]->(:Company)
```

---

## Implementation Phases

### Phase 1: Project Setup & Core Models ✅

**Priority: Critical** | **Status: Complete** | **Tasks: 6/6**

| Task                                      | Status | Notes                |
| ----------------------------------------- | ------ | -------------------- |
| Project scaffolding (directories, go.mod) | ✅     | cmd/, config/, pkg/  |
| Configuration (config.go)                 | ✅     | PG + Graph + Kafka   |
| Database connection (use stem)            | ✅     | Uses stem/database   |
| Dependency injection setup                | ✅     | ectoinject container |
| Main entry point                          | ✅     | cmd/main.go          |
| Add to go.work                            | ✅     | Workspace updated    |

#### 1.1 Project Scaffolding

```
ivy/
├── cmd/
│   ├── main.go
│   ├── dependency.go
│   ├── routes.go
│   └── startup.go
├── config/
│   └── config.go
├── db/
│   └── pg/
│       ├── 0001_init.up.sql
│       ├── 0002_entity_types.up.sql
│       ├── 0003_staged_entities.up.sql
│       ├── 0004_merged_entities.up.sql
│       └── 0005_match_rules.up.sql
├── internal/
│   └── repositories/
│       ├── entitytype/
│       ├── stagedentity/
│       ├── mergedentity/
│       ├── matchrule/
│       └── mergestrategy/
├── pkg/
│   ├── kafka/
│   ├── matching/
│   ├── merging/
│   ├── graph/
│   └── routes/
└── go.mod
```

#### 1.2 Configuration

```go
type Config struct {
    // Database
    DBHost     string `env:"DB_HOST" env-default:"localhost"`
    DBPort     int    `env:"DB_PORT" env-default:"5432"`
    DBName     string `env:"DB_NAME" env-default:"ivy"`

    // Graph Database
    GraphDBType     string `env:"GRAPH_DB_TYPE" env-default:"memgraph"`
    GraphDBHost     string `env:"GRAPH_DB_HOST" env-default:"localhost"`
    GraphDBPort     int    `env:"GRAPH_DB_PORT" env-default:"7687"`
    GraphDBUser     string `env:"GRAPH_DB_USER" env-default:""`
    GraphDBPassword string `env:"GRAPH_DB_PASSWORD" env-default:""`

    // Kafka
    KafkaBrokers       string `env:"KAFKA_BROKERS" env-default:"localhost:9092"`
    KafkaInputTopic    string `env:"KAFKA_INPUT_TOPIC" env-default:"lotus-output"`
    KafkaOutputTopic   string `env:"KAFKA_OUTPUT_TOPIC" env-default:"ivy-events"`
    KafkaConsumerGroup string `env:"KAFKA_CONSUMER_GROUP" env-default:"ivy-consumers"`

    // Processing
    MatchBatchSize       int `env:"MATCH_BATCH_SIZE" env-default:"100"`
    MergeWorkerCount     int `env:"MERGE_WORKER_COUNT" env-default:"4"`
    AutoMergeEnabled     bool `env:"AUTO_MERGE_ENABLED" env-default:"true"`
    ReviewQueueEnabled   bool `env:"REVIEW_QUEUE_ENABLED" env-default:"true"`
}
```

### Phase 2: Entity Type & Relationship Management ✅

**Priority: Critical** | **Status: Complete** | **Tasks: 6/6**

| Task                              | Status | Notes                           |
| --------------------------------- | ------ | ------------------------------- |
| Entity type CRUD API              | ✅     | /api/v1/entity-types            |
| Relationship type CRUD API        | ✅     | /api/v1/relationship-types      |
| Schema validation for entity data | ✅     | pkg/schema/validator.go         |
| Schema versioning support         | ✅     | Auto-increment on schema update |
| Schema export endpoint            | ✅     | GET /entity-types/:key/schema   |
| Database migrations               | ✅     | 0001-0004 complete              |

### Phase 3: Staging Layer ✅

**Priority: Critical** | **Status: Complete** | **Tasks: 11/11**

| Task                               | Status | Notes                                            |
| ---------------------------------- | ------ | ------------------------------------------------ |
| Kafka consumer for Lotus output    | ✅     | pkg/kafka/consumer.go                            |
| Message format parsing             | ✅     | pkg/kafka/message.go with LotusMessage struct    |
| Schema validation on ingest        | ✅     | Uses pkg/schema/service.go ValidationService     |
| Staged entity repository           | ✅     | internal/repositories/stagedentity/repository.go |
| Staged relationship repository     | ✅     | internal/repositories/stagedrelationship/        |
| Change detection (fingerprinting)  | ✅     | pkg/fingerprint/fingerprint.go                   |
| Upsert logic for incoming entities | ✅     | Upsert method in staged entity repo              |
| Match index repository             | ✅     | internal/repositories/matchindex/repository.go   |
| Match index sync on entity upsert  | ✅     | Processor calls syncMatchIndex after upsert      |
| Nested field extractor             | ✅     | pkg/extractor/extractor.go with dot-notation     |
| Field normalizers                  | ✅     | pkg/normalizers/normalizers.go                   |

### Phase 4: Match Engine ✅

**Priority: Critical** | **Status: Complete** | **Tasks: 12/12**

#### 4.1 Deterministic Matching

| Task                               | Status | Notes                                         |
| ---------------------------------- | ------ | --------------------------------------------- |
| Exact match on identity fields     | ✅     | pkg/matching/engine.go with ExactMatch scorer |
| Case-insensitive matching          | ✅     | Built into normalizers + scoring.go           |
| Normalized matching (phone, email) | ✅     | Uses pkg/normalizers before matching          |

#### 4.2 Probabilistic Matching

| Task                                              | Status | Notes                                        |
| ------------------------------------------------- | ------ | -------------------------------------------- |
| Fuzzy string matching (Jaro-Winkler, Levenshtein) | ✅     | pkg/matching/scoring.go with full algorithms |
| Phonetic matching (Soundex, Metaphone)            | ✅     | scoring.go: Soundex() and Metaphone()        |
| Date proximity matching                           | ✅     | scoring.go: DateProximity()                  |
| Weighted scoring                                  | ✅     | scoring.go: WeightedScore()                  |
| Match index query optimizer                       | ✅     | matchindex repo with pg_trgm SIMILARITY()    |

#### 4.3 Match Candidates

| Task                                    | Status | Notes                                           |
| --------------------------------------- | ------ | ----------------------------------------------- |
| Store match candidates with confidence  | ✅     | internal/repositories/matchcandidate/           |
| Filter by auto-merge threshold          | ✅     | Engine.config.AutoMergeThreshold                |
| Queue low-confidence matches for review | ✅     | MatchCandidateStatusPending for manual review   |
| Match candidate deduplication           | ✅     | GetByEntityPair + UNIQUE index on ordered pairs |

### Phase 5: Merge Engine ✅

**Priority: Critical** | **Status: Complete** | **Tasks: 5/5**

| Task                                 | Status | Notes                                               |
| ------------------------------------ | ------ | --------------------------------------------------- |
| Field-level merge strategies         | ✅     | 13 strategies in pkg/merging/field_merger.go        |
| Conflict detection                   | ✅     | detectConflict in field_merger.go                   |
| Golden record generation             | ✅     | MergeEntities() in pkg/merging/engine.go            |
| Cluster management (staged → merged) | ✅     | AddToCluster/RemoveFromCluster in mergedentity repo |
| Merge audit trail                    | ✅     | merge_audit_logs table + CreateAuditLog()           |

### Phase 6: Graph Persistence ✅

**Priority: High** | **Status: Complete** | **Tasks: 5/5**

| Task                                  | Status | Notes                                              |
| ------------------------------------- | ------ | -------------------------------------------------- |
| Graph database client (Bolt protocol) | ✅     | pkg/graph/client.go via neo4j-go-driver/v5         |
| Entity node creation/update           | ✅     | pkg/graph/entity.go with CreateOrUpdate, Delete    |
| Relationship creation/update          | ✅     | pkg/graph/relationship.go with CRUD                |
| Batch operations for performance      | ✅     | BatchCreateOrUpdate with UNWIND for efficient ops  |
| Transaction support                   | ✅     | ExecuteWrite/ExecuteRead with managed transactions |

**Additional Features:**

- Kafka producer for entity events (`pkg/kafka/producer.go`)
- Graph query API with OpenCypher support (`pkg/routes/graph/`)
- Shortest path and neighbors queries
- Label sanitization for safe Cypher

### Phase 7: Deletion Handling ✅

**Priority: High** | **Status: Complete** | **Tasks: 11/11**

#### 7.1 Explicit Deletions

| Task                                | Status | Notes                                          |
| ----------------------------------- | ------ | ---------------------------------------------- |
| Handle delete messages from Lotus   | ✅     | HandleDeleteMessage in pkg/deletion/handler.go |
| Soft delete in staging              | ✅     | stagedentity.Repository.Delete                 |
| Propagate to merged entities        | ✅     | RemoveFromCluster + check remaining sources    |
| Remove from graph (or mark deleted) | ✅     | graphEntity.Delete with soft delete            |

#### 7.2 Execution-Based Deletions

| Task                           | Status | Notes                                          |
| ------------------------------ | ------ | ---------------------------------------------- |
| Listen for execution.completed | ✅     | HandleExecutionCompleted in deletion handler   |
| Track entities per execution   | ✅     | execution_tracking table + last_seen_execution |
| Detect absent entities         | ✅     | processExecutionBasedDeletions compares        |
| Apply grace period             | ✅     | GracePeriodHours on DeletionStrategy           |
| Configurable per plan          | ✅     | deletion_strategies table per source/entity    |

#### 7.3 Staleness Deletions

| Task                       | Status | Notes                              |
| -------------------------- | ------ | ---------------------------------- |
| Track last update time     | ✅     | updated_at on staged_entities      |
| Background job for cleanup | ✅     | ProcessPendingDeletions can be run |
| Configurable retention     | ✅     | RetentionDays on DeletionStrategy  |

**Key Components:**

- `pkg/models/deletion.go` - DeletionStrategy, ExecutionTracking, PendingDeletion models
- `internal/repositories/deletion/` - strategy, execution, pending repositories
- `pkg/deletion/handler.go` - orchestrates all deletion logic
- `db/pg/0007_deletion_handling.up.sql` - migration for deletion tables

### Phase 8: Event Emission ✅

**Priority: High** | **Status: Complete** | **Tasks: 4/4**

| Task                          | Status | Notes                                           |
| ----------------------------- | ------ | ----------------------------------------------- |
| Kafka producer setup          | ✅     | pkg/kafka/producer.go with batch support        |
| Entity event publishing       | ✅     | created, updated, merged, deleted in emitter.go |
| Relationship event publishing | ✅     | created, deleted events                         |
| Event schema definition       | ✅     | pkg/events/schema.go with versioning            |

**Key Components:**

- `pkg/kafka/producer.go` - Enhanced with batch publishing
- `pkg/events/emitter.go` - High-level event emission API
- `pkg/events/schema.go` - Event type definitions with schema versioning

**Event Format:**

```json
{
  "event_type": "entity.merged",
  "tenant_id": "tenant-123",
  "entity_type": "person",
  "merged_entity_id": "uuid",
  "source_entity_ids": ["uuid1", "uuid2"],
  "confidence": 0.95,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**Event types:**

- `entity.created` - New merged entity
- `entity.updated` - Merged entity data changed
- `entity.merged` - Two entities merged into one
- `entity.unmerged` - Entities split apart
- `entity.deleted` - Entity removed
- `relationship.created`
- `relationship.deleted`

### Phase 9: REST API ✅

**Priority: Medium** | **Status: Complete** | **Tasks: 8/8**

| Task                                | Status | Notes                                     |
| ----------------------------------- | ------ | ----------------------------------------- |
| Entity Type CRUD endpoints          | ✅     | pkg/routes/entitytype/ (Phase 2)          |
| Relationship Type CRUD endpoints    | ✅     | pkg/routes/relationshiptype/ (Phase 2)    |
| Match Rule CRUD endpoints           | ✅     | pkg/routes/matchrule/                     |
| Merge Strategy CRUD endpoints       | ✅     | pkg/routes/mergestrategy/                 |
| Deletion Strategy CRUD endpoints    | ✅     | pkg/routes/deletionstrategy/              |
| Entity query endpoints (from graph) | ✅     | pkg/routes/entity/ with graph fallback    |
| Match candidate review endpoints    | ✅     | pkg/routes/matchcandidate/ approve/reject |
| Graph query passthrough (Cypher)    | ✅     | pkg/routes/graph/ (Phase 6)               |

**Endpoints:**

```
# Entity Types
GET/POST/PUT/DELETE  /api/v1/entity-types[/:id]

# Relationship Types
GET/POST/PUT/DELETE  /api/v1/relationship-types[/:id]

# Match Rules
GET/POST/PUT/DELETE  /api/v1/match-rules[/:id]

# Merge Strategies
GET/POST/PUT/DELETE  /api/v1/merge-strategies[/:id]

# Entities (read from graph)
GET    /api/v1/entities?type=person&limit=100
GET    /api/v1/entities/:id
GET    /api/v1/entities/:id/relationships
GET    /api/v1/entities/:id/sources        # Staged entities in cluster
GET    /api/v1/entities/:id/history        # Audit trail

# Match Review Queue
GET    /api/v1/match-candidates
POST   /api/v1/match-candidates/:id/approve
POST   /api/v1/match-candidates/:id/reject
POST   /api/v1/match-candidates/:id/defer

# Graph Query (Cypher passthrough)
POST   /api/v1/graph/query
```

### Phase 10: Admin & Operations 🔲

**Priority: Low** | **Status: Not Started** | **Tasks: 0/6**

| Task                              | Status | Notes                      |
| --------------------------------- | ------ | -------------------------- |
| Manual entity merge (API support) | 🔲     |                            |
| Manual entity split (unmerge)     | 🔲     | Nice-to-have               |
| Bulk re-match trigger             | 🔲     | After rule changes         |
| Statistics endpoints              | 🔲     | Entity counts, merge stats |
| Audit log viewer                  | 🔲     |                            |
| Health check endpoints            | 🔲     |                            |

---

## Graph Database Selection

### Option 1: Memgraph (Recommended)

**Pros:**

- Open-source, Apache 2.0 license
- Full OpenCypher support
- In-memory with disk persistence
- Excellent performance
- Kafka integration built-in (MAGE)
- Smaller footprint than Neo4j

**Cons:**

- Smaller community than Neo4j
- Fewer enterprise features

### Option 2: Neo4j Community

**Pros:**

- Most popular graph database
- Large community and ecosystem
- Excellent tooling (Neo4j Browser, Bloom)

**Cons:**

- Community edition limited (single instance)
- No clustering without Enterprise
- GPL license concerns for some

### Recommendation

Start with **Memgraph** for:

- Simpler deployment
- Better license (Apache 2.0)
- Good performance
- Full OpenCypher (users can query with standard Cypher)

---

## Message Format

### Input (from Lotus)

```json
{
  "source": {
    "type": "lotus",
    "tenant_id": "tenant-123",
    "binding_id": "uuid",
    "mapping_id": "uuid"
  },
  "origin": {
    "type": "orchid",
    "plan_id": "plan-123",
    "execution_id": "exec-456"
  },
  "entity": {
    "type": "person",
    "source_id": "person-789",
    "action": "upsert",
    "data": {
      "first_name": "John",
      "last_name": "Doe",
      "email": "john@example.com"
    }
  },
  "relationships": [
    {
      "type": "works_at",
      "to_entity_type": "company",
      "to_source_id": "company-123",
      "data": {
        "title": "Engineer"
      }
    }
  ],
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Output (events)

```json
{
  "event_type": "entity.created",
  "tenant_id": "tenant-123",
  "entity": {
    "type": "person",
    "ivy_id": "merged-uuid",
    "data": {
      "first_name": "John",
      "last_name": "Doe",
      "email": ["john@example.com"]
    }
  },
  "sources": [
    {
      "source_type": "orchid",
      "source_id": "person-789",
      "plan_id": "plan-123"
    }
  ],
  "timestamp": "2024-01-15T10:30:01Z"
}
```

---

## Dependencies

### Go Libraries

```go
// Graph database
github.com/neo4j/neo4j-go-driver/v5   // Works with Memgraph too

// Fuzzy matching
github.com/agnivade/levenshtein
github.com/antzucaro/matchr           // Jaro-Winkler, Soundex, Metaphone

// Hashing/Fingerprinting
crypto/sha256

// Existing (from stem)
github.com/Ramsey-B/stem/pkg/database
github.com/Ramsey-B/stem/pkg/tracing
```

### Infrastructure

```yaml
# docker-compose addition
services:
  memgraph:
    image: memgraph/memgraph-platform:latest
    ports:
      - "7687:7687" # Bolt
      - "7444:7444" # Lab
    volumes:
      - memgraph-data:/var/lib/memgraph
```

---

## Lotus Integration

### Schema Alignment

**Key Insight:** Lotus target fields should align with Ivy entity/relationship schemas. This creates a unified data model:

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Schema Registry                              │
│                                                                      │
│   ┌──────────────────┐           ┌──────────────────┐               │
│   │   Entity Types   │           │ Relationship     │               │
│   │   (Ivy defines)  │           │ Types (Ivy)      │               │
│   └────────┬─────────┘           └────────┬─────────┘               │
│            │                              │                          │
└────────────┼──────────────────────────────┼──────────────────────────┘
             │                              │
     ┌───────▼───────┐              ┌───────▼───────┐
     │ Lotus Target  │              │ Lotus Target  │
     │ Fields        │              │ Fields        │
     │ (person.*)    │              │ (works_at.*)  │
     └───────────────┘              └───────────────┘
```

### Implementation Approach

**Option A: Ivy as Schema Authority (Recommended)**

Ivy owns entity/relationship schemas. Lotus references them:

```json
// Lotus Mapping Definition
{
  "target_schema": {
    "type": "ivy_entity",
    "entity_type": "person",        // Reference to Ivy entity type
    "version": 1                     // Optional: pin to specific version
  },
  "target_fields": [...]             // Auto-populated from Ivy schema
}
```

**Benefits:**

- Single source of truth (Ivy)
- Schema changes propagate to Lotus
- UI can show same schema in both places

**Option B: Shared Schema Service**

Separate service that both Ivy and Lotus reference:

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Lotus     │────▶│   Schema    │◀────│    Ivy      │
│             │     │   Service   │     │             │
└─────────────┘     └─────────────┘     └─────────────┘
```

**Pros:** Complete decoupling
**Cons:** Another service to maintain

### Recommended: Option A (Ivy as Authority)

Keep it simple. Ivy defines schemas, exposes them via API, Lotus fetches them.

### API for Schema Sharing

Ivy should expose:

```
GET /api/v1/entity-types/:key/schema
GET /api/v1/relationship-types/:key/schema
```

Response format compatible with Lotus fields:

```json
{
  "entity_type": "person",
  "version": 1,
  "fields": [
    {
      "id": "first_name",
      "name": "First Name",
      "path": "first_name",
      "type": "string",
      "required": true
    },
    {
      "id": "email",
      "name": "Email",
      "path": "email",
      "type": "string",
      "required": false
    }
  ]
}
```

### Orchid Execution Lifecycle Events

**Critical for deletion strategies** - Orchid should emit execution lifecycle events:

```json
// Execution Started (optional)
{
  "event_type": "execution.started",
  "tenant_id": "tenant-123",
  "plan_id": "plan-456",
  "execution_id": "exec-789",
  "timestamp": "2024-01-15T10:00:00Z"
}

// Execution Completed (REQUIRED)
{
  "event_type": "execution.completed",
  "tenant_id": "tenant-123",
  "plan_id": "plan-456",
  "execution_id": "exec-789",
  "status": "success",  // "success", "partial", "failed"
  "stats": {
    "total_entities": 150,
    "entity_types": ["person", "company"],
    "duration_ms": 5432
  },
  "timestamp": "2024-01-15T10:05:00Z"
}
```

**Why this matters for Ivy:**

1. **Execution-based deletions**: Ivy can now detect absent entities
2. **Idempotency**: Ivy can ignore duplicate executions
3. **Audit trail**: Track data freshness per source

**Orchid Implementation (TODO):**

- Emit `execution.completed` from `PlanExecutor.ExecutePlan()` after all steps finish
- Include entity counts by type for validation
- Topic: `orchid-events` or same as `api-responses` with different `event_type`

### Lotus Output Message Format

When Lotus outputs to Ivy, it should include schema metadata:

```json
{
  "source": {
    "type": "lotus",
    "tenant_id": "tenant-123",
    "mapping_id": "uuid"
  },
  "origin": {
    "type": "orchid",
    "plan_id": "plan-123",
    "execution_id": "exec-456"
  },
  "target_schema": {
    "type": "entity",
    "entity_type": "person",
    "version": 1
  },
  "data": {
    "source_id": "person-789",
    "first_name": "John",
    "last_name": "Doe",
    "email": "john@example.com"
  },
  "relationships": [
    {
      "type": "works_at",
      "to_entity_type": "company",
      "to_source_id": "company-123",
      "data": {
        "title": "Engineer"
      }
    }
  ]
}
```

### Tasks for Integration

These will be tracked in the relevant phases:

| Task                                | Phase   | Notes                                 |
| ----------------------------------- | ------- | ------------------------------------- |
| Schema export API in Ivy            | Phase 9 | GET /entity-types/:key/schema         |
| Schema versioning support           | Phase 2 | Track schema changes                  |
| Lotus schema reference feature      | Lotus   | Import Ivy schemas as target fields   |
| Message format with schema metadata | Phase 3 | Include entity_type + version         |
| Schema validation on ingest         | Phase 3 | Validate incoming data matches schema |

---

## Design Decisions

### 1. Multi-tenancy in Graph

**Decision**: Property-based filtering with `tenant_id` on all nodes/relationships.

All Cypher queries will include `WHERE n.tenant_id = $tenant_id`. Simple, works for now.
Future: Could shard to separate Memgraph instances per tenant if needed.

### 2. Match Rule Conflicts

**Decision**: "No-merge" rules take precedence.

If any rule explicitly says "don't merge these entities", we respect that.
Priority order:

1. Explicit no-merge rules (blacklist)
2. Deterministic match rules (confidence = 1.0)
3. Probabilistic match rules (by confidence score)

### 3. Merge Undo

**Decision**: Nice-to-have, not critical.

Implementation:

- Keep audit trail of all merges with source data
- "Unmerge" operation splits merged entity back to sources
- Can be added in Phase 10 (Admin & Operations)

### 4. Graph Database: Memgraph

**Persistence:**

- WAL (Write-Ahead Logging) for durability
- Periodic snapshots to disk
- Auto-recovery on restart

**Configuration:**

```bash
--storage-snapshot-interval-sec=300
--storage-wal-enabled=true
--storage-snapshot-on-exit=true
--storage-recover-on-startup=true
```

**Capacity (in-memory, RAM-limited):**
| RAM | Approximate Nodes |
|-----|------------------|
| 8 GB | 1-5 million |
| 32 GB | 10-20 million |
| 64 GB | 30-50 million |

**Scaling path if needed:**

- Shard by tenant (separate Memgraph instances)
- Or migrate to Neo4j Enterprise (clustering)

### 5. Performance at Scale

**Decision**: Incremental matching with batch optimization.

- New entities: Match immediately against existing
- Bulk imports: Batch matching with configurable batch size
- Re-match: Background job for rule changes

---

## Implementation Order

| Priority | Phase | Component                   | Tasks | Effort | Status |
| -------- | ----- | --------------------------- | ----- | ------ | ------ |
| 1        | 1     | Project Setup & Core Models | 6     | 3 hrs  | ✅     |
| 2        | 2     | Entity/Relationship Types   | 6     | 5 hrs  | ✅     |
| 3        | 3     | Staging Layer               | 11    | 7 hrs  | 🔲     |
| 4        | 4.1   | Deterministic Matching      | 3     | 4 hrs  | 🔲     |
| 5        | 4.2   | Probabilistic Matching      | 5     | 6 hrs  | 🔲     |
| 6        | 4.3   | Match Candidates            | 4     | 3 hrs  | 🔲     |
| 7        | 5     | Merge Engine                | 5     | 6 hrs  | 🔲     |
| 8        | 6     | Graph Persistence           | 5     | 4 hrs  | 🔲     |
| 9        | 7     | Deletion Handling           | 12    | 5 hrs  | 🔲     |
| 10       | 8     | Event Emission              | 4     | 2 hrs  | 🔲     |
| 11       | 9     | REST API                    | 8     | 6 hrs  | 🔲     |
| 12       | 10    | Admin & Operations          | 6     | 4 hrs  | 🔲     |

**Total: 75 tasks | ~55 hours estimated**

**Completed: 12/75 tasks (16%)**

---

## Getting Started

1. Create project structure
2. Set up database migrations
3. Implement entity type management
4. Add staging layer with Kafka consumer
5. Build deterministic matching first
6. Add merge engine with basic strategies
7. Connect to graph database
8. Add event emission
9. Build out probabilistic matching
10. Add deletion handling
11. Complete REST API
