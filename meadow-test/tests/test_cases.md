# Meadow Test Cases

This document tracks all planned and implemented test cases for the Meadow data pipeline.

## Legend

| Status | Meaning                 |
| ------ | ----------------------- |
| ✅     | Implemented             |
| 🔄     | In Progress             |
| ⬚      | Planned                 |
| 💡     | Idea (needs refinement) |

---

## Smoke Tests

Quick validation that services are running and reachable.

| Status | Test Case                   | Test File                                                  |
| ------ | --------------------------- | ---------------------------------------------------------- |
| ✅     | All services health check   | [`smoke/services_health.yaml`](smoke/services_health.yaml) |
| ⬚      | Database connectivity check |                                                            |
| ⬚      | Kafka connectivity check    |                                                            |
| ⬚      | Redis connectivity check    |                                                            |

---

## Orchid (Data Extraction)

### Integration & Config Management

| Status | Test Case                                    | Test File                                                                              |
| ------ | -------------------------------------------- | -------------------------------------------------------------------------------------- |
| ✅     | Create/Read/Update/Delete integration        | [`integration/orchid_integration_crud.yaml`](integration/orchid_integration_crud.yaml) |
| ⬚      | Create integration with config schema        |                                                                                        |
| ⬚      | Create multiple configs for same integration |                                                                                        |
| ⬚      | Enable/disable configs                       |                                                                                        |
| ⬚      | Config validation against schema             |                                                                                        |

### Authentication Flows

| Status | Test Case                            | Test File |
| ------ | ------------------------------------ | --------- |
| ⬚      | No authentication (public API)       |           |
| ⬚      | OAuth2 Client Credentials flow       |           |
| ⬚      | OAuth2 with token refresh            |           |
| ⬚      | Basic authentication                 |           |
| ⬚      | API key authentication (header)      |           |
| ⬚      | API key authentication (query param) |           |
| ⬚      | Custom authentication flow           |           |
| ⬚      | Auth token caching and reuse         |           |
| ⬚      | Auth token invalidation on 401       |           |

### Plan Execution

| Status | Test Case                         | Test File |
| ------ | --------------------------------- | --------- |
| ⬚      | Simple single-step plan execution |           |
| ⬚      | Multi-step sequential execution   |           |
| ⬚      | Plan with context variables       |           |
| ⬚      | Plan execution publishes to Kafka |           |

### Pagination

| Status | Test Case                              | Test File |
| ------ | -------------------------------------- | --------- |
| ⬚      | Cursor-based pagination (after/limit)  |           |
| ⬚      | Page-based pagination (page/per_page)  |           |
| ⬚      | Offset-based pagination (offset/limit) |           |
| ⬚      | Link header pagination (next URL)      |           |
| ⬚      | OData pagination (@odata.nextLink)     |           |
| ⬚      | Break on empty page                    |           |
| ⬚      | Break on partial page                  |           |

### Fanout (Nested Requests)

| Status | Test Case                               | Test File |
| ------ | --------------------------------------- | --------- |
| ⬚      | Get list → fetch details for each item  |           |
| ⬚      | Multiple sub-steps (details + settings) |           |
| ⬚      | Nested fanout (3+ levels deep)          |           |
| ⬚      | Fanout with concurrency limit           |           |
| ⬚      | Fanout with rate limiting               |           |

### Error Handling

| Status | Test Case                                  | Test File |
| ------ | ------------------------------------------ | --------- |
| ⬚      | Retry on 429 (rate limit)                  |           |
| ⬚      | Retry on 5xx (server error)                |           |
| ⬚      | Retry with exponential backoff             |           |
| ⬚      | Abort on 401 (unauthorized)                |           |
| ⬚      | Abort on 403 (forbidden)                   |           |
| ⬚      | Continue on 404 (not found)                |           |
| ⬚      | Intermittent failures (retry succeeds)     |           |
| ⬚      | Persistent failures (max retries exceeded) |           |
| ⬚      | Timeout handling                           |           |

### Rate Limiting

| Status | Test Case                           | Test File |
| ------ | ----------------------------------- | --------- |
| ⬚      | Static rate limit (requests/window) |           |
| ⬚      | Dynamic rate limit from headers     |           |
| ⬚      | Per-endpoint rate limits            |           |
| ⬚      | Global rate limits                  |           |
| ⬚      | Respect Retry-After header          |           |

### Scheduling & Triggers

| Status | Test Case                              | Test File |
| ------ | -------------------------------------- | --------- |
| ⬚      | Manual plan trigger                    |           |
| ⬚      | Scheduled plan execution               |           |
| ⬚      | Repeat execution (repeat_count)        |           |
| ⬚      | Wait between executions (wait_seconds) |           |

### Benchmarks

| Status | Test Case                        | Test File |
| ------ | -------------------------------- | --------- |
| 💡     | Execution speed (simple plan)    |           |
| 💡     | Execution speed (complex fanout) |           |
| 💡     | Memory usage under load          |           |
| 💡     | Concurrent plan executions       |           |

---

## Lotus (Data Transformation)

### Mapping Definition Management

| Status | Test Case                        | Test File                                                                    |
| ------ | -------------------------------- | ---------------------------------------------------------------------------- |
| ✅     | Create/Read mapping definition   | [`integration/lotus_mapping_crud.yaml`](integration/lotus_mapping_crud.yaml) |
| ✅     | Execute mapping with sample data | [`integration/lotus_mapping_crud.yaml`](integration/lotus_mapping_crud.yaml) |
| ⬚      | Update mapping definition        |                                                                              |
| ⬚      | Delete/deactivate mapping        |                                                                              |
| ⬚      | Mapping versioning               |                                                                              |

### Binding Management

| Status | Test Case                              | Test File                                                                    |
| ------ | -------------------------------------- | ---------------------------------------------------------------------------- |
| ✅     | Create/Delete binding                  | [`integration/lotus_mapping_crud.yaml`](integration/lotus_mapping_crud.yaml) |
| ⬚      | Enable/disable binding                 |                                                                              |
| ⬚      | Binding with filter (integration name) |                                                                              |
| ⬚      | Binding with filter (plan keys)        |                                                                              |
| ⬚      | Multiple bindings for same mapping     |                                                                              |

### Actions (Transformations)

| Status | Test Case                                  | Test File                                                          |
| ------ | ------------------------------------------ | ------------------------------------------------------------------ |
| ✅     | List available actions                     | [`integration/lotus_actions.yaml`](integration/lotus_actions.yaml) |
| ✅     | Get action output types                    | [`integration/lotus_actions.yaml`](integration/lotus_actions.yaml) |
| ✅     | Inline mapping test                        | [`integration/lotus_actions.yaml`](integration/lotus_actions.yaml) |
| ⬚      | Text actions (to_lower, to_upper, trim)    |                                                                    |
| ⬚      | Text concat with separator                 |                                                                    |
| ⬚      | Text split to array                        |                                                                    |
| ⬚      | Number operations (add, multiply)          |                                                                    |
| ⬚      | Date parsing and formatting                |                                                                    |
| ⬚      | Coalesce (first non-null)                  |                                                                    |
| ⬚      | Default value fallback                     |                                                                    |
| ⬚      | Array operations (push, length, contains)  |                                                                    |
| ⬚      | Object operations (get, pick, omit, merge) |                                                                    |
| ⬚      | Conditional (if-else)                      |                                                                    |
| ⬚      | Regex match and replace                    |                                                                    |

### Type Matching

| Status | Test Case                        | Test File |
| ------ | -------------------------------- | --------- |
| ⬚      | String field to string target    |           |
| ⬚      | Number field to number target    |           |
| ⬚      | Boolean field to boolean target  |           |
| ⬚      | Array field to array target      |           |
| ⬚      | Object field to object target    |           |
| ⬚      | Type coercion (string → number)  |           |
| ⬚      | Type coercion (string → boolean) |           |
| ⬚      | Nested field extraction (a.b.c)  |           |

### Simple Mappings

| Status | Test Case                           | Test File |
| ------ | ----------------------------------- | --------- |
| ⬚      | Direct field-to-field mapping       |           |
| ⬚      | Constant value mapping              |           |
| ⬚      | Multiple fields to multiple targets |           |
| ⬚      | Nested source to flat target        |           |
| ⬚      | Flat source to nested target        |           |

### Transform Mappings

| Status | Test Case                           | Test File |
| ------ | ----------------------------------- | --------- |
| ⬚      | Single transformation step          |           |
| ⬚      | Chained transformations (A → B → C) |           |
| ⬚      | Multiple inputs to one step         |           |
| ⬚      | One input to multiple steps         |           |
| ⬚      | Aggregate step (collect into array) |           |
| ⬚      | Aggregate step (join strings)       |           |

### Conditional Mappings

| Status | Test Case                           | Test File |
| ------ | ----------------------------------- | --------- |
| ⬚      | Skip step if condition false        |           |
| ⬚      | Filter items from array             |           |
| ⬚      | Conditional output (if-else result) |           |
| ⬚      | Conditional to aggregate step       |           |
| ⬚      | Multiple conditions (AND/OR)        |           |

### Validation Steps

| Status | Test Case                        | Test File                                                          |
| ------ | -------------------------------- | ------------------------------------------------------------------ |
| ✅     | Validate mapping definition      | [`integration/lotus_actions.yaml`](integration/lotus_actions.yaml) |
| ⬚      | Required field validation        |                                                                    |
| ⬚      | Format validation (email, url)   |                                                                    |
| ⬚      | Range validation (min/max)       |                                                                    |
| ⬚      | Regex pattern validation         |                                                                    |
| ⬚      | Validation failure stops mapping |                                                                    |

### Array/Relationship Mappings

| Status | Test Case                             | Test File |
| ------ | ------------------------------------- | --------- |
| ⬚      | Source array iteration                |           |
| ⬚      | Map array of objects to relationships |           |
| ⬚      | Nested array extraction               |           |
| ⬚      | One-to-many relationship output       |           |

### Benchmarks

| Status | Test Case                     | Test File |
| ------ | ----------------------------- | --------- |
| 💡     | Simple mapping throughput     |           |
| 💡     | Complex mapping throughput    |           |
| 💡     | Large payload transformation  |           |
| 💡     | High-volume Kafka consumption |           |

---

## Ivy (Entity Resolution)

### Entity Type Management

| Status | Test Case                         | Test File                                                                |
| ------ | --------------------------------- | ------------------------------------------------------------------------ |
| ✅     | Create entity type with schema    | [`integration/ivy_entity_types.yaml`](integration/ivy_entity_types.yaml) |
| ✅     | List entity types                 | [`integration/ivy_entity_types.yaml`](integration/ivy_entity_types.yaml) |
| ⬚      | Update entity type                |                                                                          |
| ⬚      | Delete entity type                |                                                                          |
| ⬚      | Entity type with merge strategies |                                                                          |

### Relationship Type Management

| Status | Test Case                                       | Test File |
| ------ | ----------------------------------------------- | --------- |
| ⬚      | Create relationship type                        |           |
| ⬚      | Relationship cardinality (1:1, 1:N, N:N)        |           |
| ⬚      | Self-referential relationship (person → person) |           |
| ⬚      | Cross-entity relationship (person → device)     |           |

### Match Rules

| Status | Test Case                                 | Test File                                                                |
| ------ | ----------------------------------------- | ------------------------------------------------------------------------ |
| ✅     | Create exact match rule                   | [`integration/ivy_entity_types.yaml`](integration/ivy_entity_types.yaml) |
| ✅     | List match rules by entity type           | [`integration/ivy_entity_types.yaml`](integration/ivy_entity_types.yaml) |
| ⬚      | Fuzzy match rule (similarity threshold)   |                                                                          |
| ⬚      | Multi-field match rule                    |                                                                          |
| ⬚      | Match with normalizers (lowercase, phone) |                                                                          |
| ⬚      | Match priority ordering                   |                                                                          |
| ⬚      | Blocking rules (prevent merge)            |                                                                          |

### Entity Matching

| Status | Test Case                                      | Test File |
| ------ | ---------------------------------------------- | --------- |
| ⬚      | Exact email match                              |           |
| ⬚      | Fuzzy name match                               |           |
| ⬚      | Multi-field composite match                    |           |
| ⬚      | Cross-integration match (OKTA + MS Graph)      |           |
| ⬚      | Same integration, different source keys        |           |
| ⬚      | No match (new entity created)                  |           |
| ⬚      | Match candidate below threshold (review queue) |           |

### Entity Merging

| Status | Test Case                        | Test File |
| ------ | -------------------------------- | --------- |
| ⬚      | Auto-merge on high confidence    |           |
| ⬚      | Merge strategy: most_recent      |           |
| ⬚      | Merge strategy: prefer_non_empty |           |
| ⬚      | Merge strategy: prefer_source    |           |
| ⬚      | Merge updates relationships      |           |
| ⬚      | Merge multiple source entities   |           |

### Relationship Handling

| Status | Test Case                                   | Test File |
| ------ | ------------------------------------------- | --------- |
| ⬚      | Direct relationship (source_id → source_id) |           |
| ⬚      | Relationship before source entity           |           |
| ⬚      | Relationship before target entity           |           |
| ⬚      | Relationship before both entities           |           |
| ⬚      | Criteria-based relationship                 |           |
| ⬚      | Criteria matches existing entities          |           |
| ⬚      | Criteria matches future entities            |           |
| ⬚      | Relationship with properties                |           |

### Match Candidates (Review Queue)

| Status | Test Case                       | Test File |
| ------ | ------------------------------- | --------- |
| ⬚      | List pending match candidates   |           |
| ⬚      | Approve match candidate (merge) |           |
| ⬚      | Reject match candidate          |           |
| ⬚      | Defer match candidate           |           |

### Deletion Strategies

| Status | Test Case                      | Test File |
| ------ | ------------------------------ | --------- |
| ⬚      | Execution-based deletion       |           |
| ⬚      | Grace period before deletion   |           |
| ⬚      | Retention period (soft delete) |           |
| ⬚      | Cascade delete relationships   |           |

### Graph Queries

| Status | Test Case                      | Test File |
| ------ | ------------------------------ | --------- |
| ⬚      | Find entity by property        |           |
| ⬚      | Find entity relationships      |           |
| ⬚      | Shortest path between entities |           |
| ⬚      | Neighbor traversal             |           |
| ⬚      | Complex Cypher query           |           |

### Validation

| Status | Test Case                           | Test File                                                                |
| ------ | ----------------------------------- | ------------------------------------------------------------------------ |
| ✅     | Validate entity data against schema | [`integration/ivy_entity_types.yaml`](integration/ivy_entity_types.yaml) |
| ⬚      | Invalid data rejected               |                                                                          |
| ⬚      | Required field validation           |                                                                          |

### Benchmarks

| Status | Test Case                         | Test File |
| ------ | --------------------------------- | --------- |
| 💡     | Entity resolution throughput      |           |
| 💡     | Match rule evaluation performance |           |
| 💡     | Graph query performance           |           |
| 💡     | High-volume entity ingestion      |           |

---

## Kafka Integration

| Status | Test Case                  | Test File                                                        |
| ------ | -------------------------- | ---------------------------------------------------------------- |
| ✅     | Publish message to topic   | [`integration/kafka_pubsub.yaml`](integration/kafka_pubsub.yaml) |
| ✅     | Consume and verify message | [`integration/kafka_pubsub.yaml`](integration/kafka_pubsub.yaml) |
| ✅     | Topic auto-creation        | [`integration/kafka_simple.yaml`](integration/kafka_simple.yaml) |
| ⬚      | Message with headers       |                                                                  |
| ⬚      | Message key partitioning   |                                                                  |
| ⬚      | Consumer group handling    |                                                                  |

---

## End-to-End Scenarios

Full pipeline tests that exercise multiple services.

| Status | Test Case                                                                     | Test File |
| ------ | ----------------------------------------------------------------------------- | --------- |
| ⬚      | **Basic E2E**: Orchid pulls data → Kafka → Lotus transforms → Ivy resolves    |           |
| ⬚      | **OKTA User Sync**: Auth → fetch users → transform → create Person entities   |           |
| ⬚      | **MS Graph Sync**: Auth → fetch users/devices → transform → resolve entities  |           |
| ⬚      | **Multi-Source Resolution**: OKTA + MS Graph → match by email → merged Person |           |
| ⬚      | **Relationship Flow**: Users + Managers → reports_to relationships            |           |
| ⬚      | **Device Ownership**: Users + Devices → owns relationships                    |           |
| ⬚      | **Group Membership**: Users + Groups → member_of relationships                |           |
| ⬚      | **Criteria Relationships**: Policy → has_access_to Windows devices            |           |
| ⬚      | **Full CRUD Lifecycle**: Create → Update → Query → Delete                     |           |
| ⬚      | **Error Recovery**: Partial failure → retry → complete                        |           |

---

## Test Infrastructure

| Status | Test Case                              | Test File |
| ------ | -------------------------------------- | --------- |
| ⬚      | Mock API dynamic configuration         |           |
| ⬚      | Test data fixtures/seeding             |           |
| ⬚      | Test isolation (cleanup between tests) |           |
| ⬚      | Parallel test execution                |           |
| ⬚      | JUnit report generation                |           |
| ⬚      | JSON report generation                 |           |

---

## Notes

- Tests in `smoke/` should be fast and run on every deploy
- Tests in `integration/` test individual service APIs
- Tests in `scenarios/` test cross-service workflows
- Benchmarks should be run separately, not in CI
- Do NOT make tests that pass just to pass
- If a test fails, ensure investigate why, do NOT just update to pass if the test is valid.
