# Meadow

**Meadow** is a modular data integration platform for extracting, transforming, and resolving entities from external APIs into a unified graph.

---

## 🌿 System Overview

Meadow consists of three core services that form a data pipeline:

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                               Meadow Pipeline                                   │
│                                                                                 │
│   ┌───────────────┐      ┌───────────────┐      ┌───────────────┐              │
│   │    Orchid     │      │     Lotus     │      │      Ivy      │              │
│   │   (Extract)   │─────▶│  (Transform)  │─────▶│    (Merge)    │              │
│   │               │      │               │      │               │              │
│   │ • API Polling │      │ • Data Mapping│      │ • Entity Res. │              │
│   │ • Auth Mgmt   │      │ • Transforms  │      │ • Deduplication│             │
│   │ • Scheduling  │      │ • Routing     │      │ • Graph Sync  │              │
│   └───────────────┘      └───────────────┘      └───────────────┘              │
│          │                      │                      │                        │
│          │   api-responses      │    mapped-data       │    entity-events       │
│          └──────────────────────┴──────────────────────┴────────────────────────┤
│                                    Kafka                                        │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## 🌸 Orchid — API Extraction Service

**Orchid** is a horizontally scalable API polling microservice that extracts data from external APIs.

### Features

| Feature               | Description                                                     |
| --------------------- | --------------------------------------------------------------- |
| **Scheduled Polling** | Configurable cron schedules for API execution plans             |
| **Authentication**    | OAuth 2.0, API keys, bearer tokens, custom auth flows           |
| **Complex Workflows** | Multi-step plans with pagination, nested requests, conditionals |
| **Rate Limiting**     | Per-integration rate limiting with backoff                      |
| **Error Handling**    | Retries with exponential backoff, dead-letter queue             |
| **Multi-tenancy**     | Full tenant isolation for SaaS deployments                      |

### Kafka Messages

| Topic           | Direction    | Description                         |
| --------------- | ------------ | ----------------------------------- |
| `api-responses` | **Produces** | Raw API response data with metadata |
| `api-errors`    | **Produces** | Failed requests and error details   |

### Message Format (`api-responses`)

```json
{
  "tenant_id": "uuid",
  "integration": "salesforce",
  "plan_key": "contacts-sync",
  "execution_id": "uuid",
  "step_path": "fetch_contacts",
  "status_code": 200,
  "response_body": { ... },
  "timestamp": "2024-01-03T10:00:00Z"
}
```

### Key Concepts

- **Integration**: A configured connection to an external API (e.g., Salesforce, HubSpot)
- **Plan**: A sequence of API calls to execute (fetch contacts → fetch details → fetch activities)
- **Execution**: A single run of a plan, producing multiple API responses

---

## 🪷 Lotus — Data Mapping Service

**Lotus** is a Kafka-based data transformation service that maps raw API responses to normalized entity schemas.

### Features

| Feature                  | Description                                              |
| ------------------------ | -------------------------------------------------------- |
| **Declarative Mappings** | JSON-based mapping definitions with JSONPath             |
| **Binding System**       | Route messages to mappings by source, plan, or status    |
| **Transform Actions**    | String manipulation, date parsing, lookups, conditionals |
| **Batch Processing**     | Handles array responses with per-record transforms       |
| **Schema Validation**    | Validates output against JSON Schema                     |

### Kafka Messages

| Topic            | Direction    | Description                           |
| ---------------- | ------------ | ------------------------------------- |
| `api-responses`  | **Consumes** | Raw API responses from Orchid         |
| `mapped-data`    | **Produces** | Normalized entities and relationships |
| `mapping-errors` | **Produces** | Transformation failures               |

### Message Format (`mapped-data`)

```json
{
  "source": {
    "type": "orchid",
    "tenant_id": "uuid",
    "integration": "salesforce",
    "config_id": "uuid",
    "execution_id": "uuid"
  },
  "target_schema": {
    "type": "entity",
    "entity_type": "person"
  },
  "data": {
    "_entity_type": "person",
    "_source_id": "sf-contact-123",
    "first_name": "John",
    "last_name": "Doe",
    "email": "john@example.com"
  },
  "relationships": [
    {
      "_relationship_type": "works_at",
      "_from_entity_type": "person",
      "_from_source_id": "sf-contact-123",
      "_to_entity_type": "company",
      "_to_source_id": "sf-account-456"
    }
  ]
}
```

### Key Concepts

- **Mapping**: A JSONPath-based transformation from source to target schema
- **Binding**: Routes incoming messages to the appropriate mapping
- **Actions**: Transform functions (e.g., `uppercase`, `split`, `coalesce`, `lookup`)

---

## 🌱 Ivy — Entity Resolution Service

**Ivy** is the entity resolution and graph persistence layer that deduplicates and merges entities from multiple sources.

### Features

| Feature                   | Description                                                                 |
| ------------------------- | --------------------------------------------------------------------------- |
| **Entity Resolution**     | Deterministic + probabilistic matching rules                                |
| **Merge Strategies**      | Configurable per-field merge logic (most recent, most trusted, collect all) |
| **Relationship Handling** | Direct relationships and criteria-based dynamic relationships               |
| **Deletion Strategies**   | Execution-based, staleness, retention, explicit                             |
| **Graph Sync**            | Persist merged entities to Memgraph/Neo4j                                   |
| **JSONB Deep Merge**      | PostgreSQL-native incremental data merging                                  |

### Kafka Messages

| Topic           | Direction    | Description                                                 |
| --------------- | ------------ | ----------------------------------------------------------- |
| `mapped-data`   | **Consumes** | Normalized entities from Lotus                              |
| `entity-events` | **Produces** | Entity lifecycle events (created, merged, updated, deleted) |

### Entity Lifecycle

```
┌───────────────┐     ┌───────────────┐     ┌───────────────┐     ┌───────────────┐
│  Lotus Msg    │────▶│  Staged       │────▶│   Matched     │────▶│   Merged      │
│  (mapped-data)│     │  Entity       │     │  Candidates   │     │   Entity      │
└───────────────┘     └───────────────┘     └───────────────┘     └───────────────┘
                             │                     │                      │
                             ▼                     ▼                      ▼
                      ┌─────────────┐       ┌───────────┐          ┌───────────┐
                      │  PostgreSQL │       │  Review   │          │  Graph DB │
                      │  (staging)  │       │  Queue    │          │ (Memgraph)│
                      └─────────────┘       └───────────┘          └───────────┘
```

### Key Concepts

- **Staged Entity**: Raw entity from a source, before resolution
- **Merged Entity**: The "golden record" after deduplication
- **Match Rule**: Defines how to find duplicate entities (email match, fuzzy name, etc.)
- **Merge Strategy**: Defines how to combine data from duplicates
- **Entity Cluster**: Links all staged entities that resolved to the same merged entity

### Match Rule Types

| Type         | Description                         |
| ------------ | ----------------------------------- |
| `exact`      | Exact field equality                |
| `fuzzy`      | Levenshtein/Jaro-Winkler similarity |
| `phonetic`   | Soundex/Metaphone matching          |
| `numeric`    | Numeric range matching              |
| `date_range` | Date proximity matching             |

### Merge Strategy Types

| Strategy           | Description                                 |
| ------------------ | ------------------------------------------- |
| `most_recent`      | Use value from most recently updated source |
| `most_trusted`     | Use value from highest priority source      |
| `collect_all`      | Collect all unique values as array          |
| `longest_value`    | Use the longest string                      |
| `prefer_non_empty` | Use first non-null/empty value              |

### Deletion Strategies

| Strategy          | Description                                  |
| ----------------- | -------------------------------------------- |
| `execution_based` | Delete entities not seen in latest execution |
| `staleness`       | Delete entities not updated within N days    |
| `retention`       | Delete entities older than N days            |
| `explicit`        | Only delete via explicit API calls           |

---

## 🚀 Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.21+

### Start Infrastructure

```bash
make up
```

### Services

| Service      | Port | Description                |
| ------------ | ---- | -------------------------- |
| PostgreSQL   | 5432 | Citus-enabled database     |
| Redis        | 6379 | Cache and message queues   |
| Kafka        | 9092 | Message broker             |
| Kafka UI     | 8080 | Web UI for Kafka debugging |
| Memgraph     | 7687 | Graph database (Bolt)      |
| Memgraph Lab | 3000 | Graph visualization UI     |

### Databases

| Database | Service | Description                                   |
| -------- | ------- | --------------------------------------------- |
| `orchid` | Orchid  | Integration configs, plans, executions        |
| `lotus`  | Lotus   | Mappings, bindings, transform configs         |
| `ivy`    | Ivy     | Staged entities, merged entities, match rules |

---

## 📋 Commands

```bash
make up              # Start all services
make down            # Stop all services
make clean           # Stop and remove all data
make reset           # Clean and restart
make logs            # Tail all logs
make ps              # Show running services

make psql-orchid     # Connect to Orchid database
make psql-lotus      # Connect to Lotus database
make psql-ivy        # Connect to Ivy database
make redis-cli       # Connect to Redis
```

---

## 🔧 Configuration

### Orchid

```bash
DB_HOST=localhost
DB_PORT=5432
DB_NAME=orchid
DB_USER=user
DB_PASSWORD=password
REDIS_HOST=localhost
REDIS_PORT=6379
KAFKA_BROKERS=localhost:9092
KAFKA_RESPONSE_TOPIC=api-responses
KAFKA_ERROR_TOPIC=api-errors
```

### Lotus

```bash
DB_HOST=localhost
DB_PORT=5432
DB_NAME=lotus
DB_USER=user
DB_PASSWORD=password
KAFKA_BROKERS=localhost:9092
KAFKA_INPUT_TOPIC=api-responses
KAFKA_OUTPUT_TOPIC=mapped-data
KAFKA_ERROR_TOPIC=mapping-errors
KAFKA_CONSUMER_GROUP=lotus-consumer
```

### Ivy

```bash
DB_HOST=localhost
DB_PORT=5432
DB_NAME=ivy
DB_USER=user
DB_PASSWORD=password
KAFKA_BROKERS=localhost:9092
KAFKA_INPUT_TOPIC=mapped-data
KAFKA_OUTPUT_TOPIC=entity-events
KAFKA_CONSUMER_GROUP=ivy-consumer
GRAPH_DB_HOST=localhost
GRAPH_DB_PORT=7687
AUTO_MERGE_THRESHOLD=0.95
```

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              Infrastructure                                      │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  ┌──────────────────┐                                                           │
│  │    PostgreSQL    │  (Citus single-node mode)                                 │
│  │      :5432       │                                                           │
│  │                  │                                                           │
│  │  ├─ orchid       │  Integrations, plans, executions                          │
│  │  ├─ lotus        │  Mappings, bindings                                       │
│  │  └─ ivy          │  Staged entities, merged entities, match rules            │
│  └──────────────────┘                                                           │
│                                                                                  │
│  ┌──────────────────┐    ┌──────────────────────────────────────────────────┐   │
│  │      Redis       │    │                    Kafka                         │   │
│  │      :6379       │    │                    :9092                         │   │
│  │                  │    │                                                  │   │
│  │  • Rate limits   │    │  Topics:                                         │   │
│  │  • Token cache   │    │    • api-responses    (Orchid → Lotus)           │   │
│  │  • Locks         │    │    • api-errors       (Orchid → DLQ)             │   │
│  └──────────────────┘    │    • mapped-data      (Lotus → Ivy)              │   │
│                          │    • mapping-errors   (Lotus → DLQ)              │   │
│  ┌──────────────────┐    │    • entity-events    (Ivy → Consumers)          │   │
│  │    Memgraph      │    └──────────────────────────────────────────────────┘   │
│  │      :7687       │                         │                                  │
│  │                  │                 ┌───────┴───────┐                         │
│  │  Entity graph    │                 │   Kafka UI    │                         │
│  │  Relationships   │                 │     :8080     │                         │
│  └──────────────────┘                 └───────────────┘                         │
│                                                                                  │
└──────────────────────────────────────────────────────────────────────────────────┘
                    │                           │                    │
                    ▼                           ▼                    ▼
             ┌───────────┐               ┌───────────┐         ┌───────────┐
             │  Orchid   │    Kafka      │   Lotus   │  Kafka  │    Ivy    │
             │   :3001   │ ────────────▶ │   :3002   │ ──────▶ │   :3003   │
             │           │ api-responses │           │ mapped  │           │
             │  Extract  │               │ Transform │  -data  │   Merge   │
             └───────────┘               └───────────┘         └───────────┘
                    │                                                │
                    │                                                ▼
                    │                                         ┌───────────┐
                    ▼                                         │  Graph DB │
             ┌─────────────────┐                              │ (Memgraph)│
             │  External APIs  │                              └───────────┘
             │                 │
             │  • Salesforce   │
             │  • HubSpot      │
             │  • Okta         │
             │  • Custom APIs  │
             └─────────────────┘
```

---

## 📚 Additional Documentation

| Document                                                       | Description                                        |
| -------------------------------------------------------------- | -------------------------------------------------- |
| [`orchid/CONSUMER_GUIDE.md`](orchid/CONSUMER_GUIDE.md)         | Orchid message format and consumption guide        |
| [`lotus/IMPLEMENTATION_PLAN.md`](lotus/IMPLEMENTATION_PLAN.md) | Lotus architecture and implementation details      |
| [`IVY_IMPLEMENTATION_PLAN.md`](IVY_IMPLEMENTATION_PLAN.md)     | Ivy entity resolution architecture                 |
| [`meadow-test/README.md`](meadow-test/README.md)               | Declarative YAML-based testing framework           |
| [`stem/README.md`](stem/README.md)                             | Shared library documentation                       |
| [`test/http/README.md`](test/http/README.md)                   | API testing examples                               |

---

## 🧪 Testing

### Meadow Test - Declarative YAML Testing Framework

Meadow includes a declarative testing framework that allows you to write integration and end-to-end tests using simple YAML files instead of Go code.

**Quick Start:**

```bash
# Install the test runner
cd meadow-test
go install ./cmd/meadow-test

# Run example tests
meadow-test run tests/
```

**Example Test (YAML):**

```yaml
name: User Sync Pipeline
description: Test complete user sync from Orchid → Lotus → Ivy

steps:
  - create_plan:
      key: sync-users
      steps:
        - type: http_request
          url: "{{mock_api_url}}/api/v1/users"

  - trigger_execution:
      plan_key: sync-users
      wait_for_completion: true

  - assert_kafka_message:
      topic: mapped-data
      timeout: 30s
      assertions:
        - field: data.user_id
          equals: user-123
```

**See [`meadow-test/README.md`](meadow-test/README.md) for complete documentation.**

### Traditional Testing

See [`test/http/`](test/http/) for HTTP request examples to test the APIs.

```bash
# Run unit tests
make test

# Run integration tests
make test-integration
```
