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

| Status | Test Case                   | Test File                                                              |
| ------ | --------------------------- | ---------------------------------------------------------------------- |
| ✅     | All services health check   | [`smoke/services_health.yaml`](smoke/services_health.yaml)             |
| ✅     | Database connectivity check | [`smoke/database_connectivity.yaml`](smoke/database_connectivity.yaml) |
| ✅     | Kafka connectivity check    | [`smoke/kafka_connectivity.yaml`](smoke/kafka_connectivity.yaml)       |
| ✅     | Redis connectivity check    | [`smoke/redis_connectivity.yaml`](smoke/redis_connectivity.yaml)       |

---

## Orchid (Data Extraction)

### Integration & Config Management

| Status | Test Case                                    | Test File                                                                                |
| ------ | -------------------------------------------- | ---------------------------------------------------------------------------------------- |
| ✅     | Create/Read/Update/Delete integration        | [`integration/orchid_integration_crud.yaml`](integration/orchid_integration_crud.yaml)   |
| ✅     | Create integration with config schema        | [`integration/orchid_integration_crud.yaml`](integration/orchid_integration_crud.yaml)   |
| ✅     | Create multiple configs for same integration | [`integration/orchid_config_management.yaml`](integration/orchid_config_management.yaml) |
| ⬚      | Enable/disable configs                       |                                                                                          |
| ⬚      | Config validation against schema             |                                                                                          |

### Authentication Flows

| Status | Test Case                            | Test File                                                                                            |
| ------ | ------------------------------------ | ---------------------------------------------------------------------------------------------------- |
| ✅     | No authentication (public API)       | [`integration/orchid_auth_none.yaml`](integration/orchid_auth_none.yaml)                             |
| ✅     | OAuth2 Client Credentials flow       | [`integration/orchid_auth_oauth2.yaml`](integration/orchid_auth_oauth2.yaml)                         |
| ✅     | OAuth2 with token refresh            | [`integration/orchid_auth_oauth2_refresh.yaml`](integration/orchid_auth_oauth2_refresh.yaml)         |
| ✅     | Basic authentication                 | [`integration/orchid_auth_basic.yaml`](integration/orchid_auth_basic.yaml)                           |
| ✅     | API key authentication (header)      | [`integration/orchid_auth_api_key.yaml`](integration/orchid_auth_api_key.yaml)                       |
| ✅     | API key authentication (query param) | [`integration/orchid_auth_api_key.yaml`](integration/orchid_auth_api_key.yaml)                       |
| ✅     | Custom authentication flow           | [`integration/orchid_auth_custom.yaml`](integration/orchid_auth_custom.yaml)                         |
| ✅     | Auth token caching and reuse         | [`integration/orchid_auth_token_caching.yaml`](integration/orchid_auth_token_caching.yaml)           |
| ✅     | Auth token invalidation on 401       | [`integration/orchid_auth_token_invalidation.yaml`](integration/orchid_auth_token_invalidation.yaml) |

### Plan Execution

| Status | Test Case                         | Test File                                                                                    |
| ------ | --------------------------------- | -------------------------------------------------------------------------------------------- |
| ✅     | Simple single-step plan creation  | [`integration/orchid_plan_simple.yaml`](integration/orchid_plan_simple.yaml)                 |
| ✅     | Multi-step sequential execution   | [`integration/orchid_multistep_plan.yaml`](integration/orchid_multistep_plan.yaml)           |
| ✅     | Plan with context variables       | [`integration/orchid_multistep_plan.yaml`](integration/orchid_multistep_plan.yaml)           |
| ✅     | Plan execution publishes to Kafka | [`integration/orchid_kafka_integration.yaml`](integration/orchid_kafka_integration.yaml)     |
| ✅     | Trigger plan execution via API    | [`integration/orchid_plan_execution_test.yaml`](integration/orchid_plan_execution_test.yaml) |

**Note:** All plan tests now include execution verification using the `/api/v1/plans/{key}/trigger` endpoint, verifying that plans are queued successfully.

### Pagination

| Status | Test Case                              | Test File                                                                                    |
| ------ | -------------------------------------- | -------------------------------------------------------------------------------------------- |
| ✅     | Cursor-based pagination (after/limit)  | [`integration/orchid_pagination.yaml`](integration/orchid_pagination.yaml)                   |
| ✅     | Page-based pagination (page/per_page)  | [`integration/orchid_pagination.yaml`](integration/orchid_pagination.yaml)                   |
| ✅     | Offset-based pagination (offset/limit) | [`integration/orchid_pagination.yaml`](integration/orchid_pagination.yaml)                   |
| ✅     | Link header pagination (next URL)      | [`integration/orchid_pagination.yaml`](integration/orchid_pagination.yaml)                   |
| ✅     | OData pagination (@odata.nextLink)     | [`integration/orchid_pagination_advanced.yaml`](integration/orchid_pagination_advanced.yaml) |
| ✅     | Break on empty page                    | [`integration/orchid_pagination_advanced.yaml`](integration/orchid_pagination_advanced.yaml) |
| ✅     | Break on partial page                  | [`integration/orchid_pagination_advanced.yaml`](integration/orchid_pagination_advanced.yaml) |

### Fanout (Nested Requests)

| Status | Test Case                               | Test File                                                                                  |
| ------ | --------------------------------------- | ------------------------------------------------------------------------------------------ |
| ✅     | Get list → fetch details for each item  | [`integration/orchid_fanout_basic.yaml`](integration/orchid_fanout_basic.yaml)             |
| ✅     | Multiple sub-steps (details + settings) | [`integration/orchid_fanout_multistep.yaml`](integration/orchid_fanout_multistep.yaml)     |
| ✅     | Nested fanout (3+ levels deep)          | [`integration/orchid_fanout_nested.yaml`](integration/orchid_fanout_nested.yaml)           |
| ✅     | Fanout with concurrency limit           | [`integration/orchid_fanout_concurrency.yaml`](integration/orchid_fanout_concurrency.yaml) |
| ✅     | Fanout with rate limiting               | [`integration/orchid_fanout_rate_limit.yaml`](integration/orchid_fanout_rate_limit.yaml)   |

**Note:** All fanout tests now include execution verification to confirm the fanout pattern works when triggered.

### Error Handling

| Status | Test Case                                  | Test File                                                                      |
| ------ | ------------------------------------------ | ------------------------------------------------------------------------------ |
| ✅     | Retry on 429 (rate limit)                  | [`integration/orchid_retry_429.yaml`](integration/orchid_retry_429.yaml)       |
| ✅     | Retry on 5xx (server error)                | [`integration/orchid_retry_5xx.yaml`](integration/orchid_retry_5xx.yaml)       |
| ✅     | Retry with exponential backoff             | [`integration/orchid_retry_5xx.yaml`](integration/orchid_retry_5xx.yaml)       |
| ✅     | Abort on 401 (unauthorized)                | [`integration/orchid_abort_401.yaml`](integration/orchid_abort_401.yaml)       |
| ✅     | Abort on 403 (forbidden)                   | [`integration/orchid_abort_403.yaml`](integration/orchid_abort_403.yaml)       |
| ✅     | Continue on 404 (not found)                | [`integration/orchid_continue_404.yaml`](integration/orchid_continue_404.yaml) |
| ⬚      | Intermittent failures (retry succeeds)     |                                                                                |
| ⬚      | Persistent failures (max retries exceeded) |                                                                                |
| ✅     | Timeout handling                           | [`integration/orchid_timeout.yaml`](integration/orchid_timeout.yaml)           |

### Rate Limiting

| Status | Test Case                           | Test File                                                                                          |
| ------ | ----------------------------------- | -------------------------------------------------------------------------------------------------- |
| ✅     | Static rate limit (requests/window) | [`integration/orchid_rate_limit_static.yaml`](integration/orchid_rate_limit_static.yaml)           |
| ✅     | Dynamic rate limit from headers     | [`integration/orchid_rate_limit_dynamic.yaml`](integration/orchid_rate_limit_dynamic.yaml)         |
| ⬚      | Per-endpoint rate limits            |                                                                                                    |
| ⬚      | Global rate limits                  |                                                                                                    |
| ✅     | Respect Retry-After header          | [`integration/orchid_rate_limit_retry_after.yaml`](integration/orchid_rate_limit_retry_after.yaml) |

### Scheduling & Triggers

| Status | Test Case                              | Test File                                                                                    |
| ------ | -------------------------------------- | -------------------------------------------------------------------------------------------- |
| ✅     | Manual plan trigger                    | [`integration/orchid_manual_trigger.yaml`](integration/orchid_manual_trigger.yaml)           |
| ✅     | Scheduled plan execution               | [`integration/orchid_scheduled_execution.yaml`](integration/orchid_scheduled_execution.yaml) |
| ✅     | Repeat execution (repeat_count)        | [`integration/orchid_repeat_execution.yaml`](integration/orchid_repeat_execution.yaml)       |
| ✅     | Wait between executions (wait_seconds) | [`integration/orchid_wait_between.yaml`](integration/orchid_wait_between.yaml)               |

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

| Status | Test Case                        | Test File                                                                          |
| ------ | -------------------------------- | ---------------------------------------------------------------------------------- |
| ✅     | Create/Read mapping definition   | [`integration/lotus_mapping_crud.yaml`](integration/lotus_mapping_crud.yaml)       |
| ✅     | Execute mapping with sample data | [`integration/lotus_mapping_crud.yaml`](integration/lotus_mapping_crud.yaml)       |
| ✅     | Update mapping definition        | [`integration/lotus_mapping_updates.yaml`](integration/lotus_mapping_updates.yaml) |
| ⬚      | Delete/deactivate mapping        |                                                                                    |
| ✅     | Mapping versioning               | [`integration/lotus_mapping_updates.yaml`](integration/lotus_mapping_updates.yaml) |

### Binding Management

| Status | Test Case                              | Test File                                                                    |
| ------ | -------------------------------------- | ---------------------------------------------------------------------------- |
| ✅     | Create/Delete binding                  | [`integration/lotus_mapping_crud.yaml`](integration/lotus_mapping_crud.yaml) |
| ⬚      | Enable/disable binding                 |                                                                              |
| ⬚      | Binding with filter (integration name) |                                                                              |
| ⬚      | Binding with filter (plan keys)        |                                                                              |
| ⬚      | Multiple bindings for same mapping     |                                                                              |

### Actions (Transformations)

| Status | Test Case                                                                        | Test File                                                                            |
| ------ | -------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| ✅     | List available actions                                                           | [`integration/lotus_actions.yaml`](integration/lotus_actions.yaml)                   |
| ✅     | Get action output types                                                          | [`integration/lotus_actions.yaml`](integration/lotus_actions.yaml)                   |
| ✅     | Inline mapping test                                                              | [`integration/lotus_actions.yaml`](integration/lotus_actions.yaml)                   |
| ✅     | Text actions (to_lower, to_upper, trim)                                          | [`integration/lotus_text_actions.yaml`](integration/lotus_text_actions.yaml)         |
| ✅     | Text concat with separator                                                       | [`integration/lotus_text_actions.yaml`](integration/lotus_text_actions.yaml)         |
| ✅     | Text split to array                                                              | [`integration/lotus_text_actions.yaml`](integration/lotus_text_actions.yaml)         |
| ✅     | Number operations (add, multiply)                                                | [`integration/lotus_number_actions.yaml`](integration/lotus_number_actions.yaml)     |
| ⬚      | Date parsing and formatting                                                      |                                                                                      |
| ✅     | Coalesce (first non-null)                                                        | [`integration/lotus_coalesce_default.yaml`](integration/lotus_coalesce_default.yaml) |
| ✅     | Default value fallback                                                           | [`integration/lotus_coalesce_default.yaml`](integration/lotus_coalesce_default.yaml) |
| ✅     | Array operations (push, length, contains)                                        | [`integration/lotus_array_actions.yaml`](integration/lotus_array_actions.yaml)       |
| ✅     | Object operations (get, pick, omit, merge)                                       | [`integration/lotus_object_actions.yaml`](integration/lotus_object_actions.yaml)     |
| ⬚      | Conditional (if-else)                                                            |                                                                                      |
| ✅     | Regex match and replace                                                          | [`integration/lotus_regex_actions.yaml`](integration/lotus_regex_actions.yaml)       |
| ✅     | Advanced text operations (contains, starts_with, ends_with, substring, index_of) | [`integration/lotus_text_advanced.yaml`](integration/lotus_text_advanced.yaml)       |

### Type Matching

| Status | Test Case                        | Test File                                                                      |
| ------ | -------------------------------- | ------------------------------------------------------------------------------ |
| ✅     | String field to string target    | [`integration/lotus_type_matching.yaml`](integration/lotus_type_matching.yaml) |
| ✅     | Number field to number target    | [`integration/lotus_type_matching.yaml`](integration/lotus_type_matching.yaml) |
| ✅     | Boolean field to boolean target  | [`integration/lotus_type_matching.yaml`](integration/lotus_type_matching.yaml) |
| ✅     | Array field to array target      | [`integration/lotus_type_matching.yaml`](integration/lotus_type_matching.yaml) |
| ✅     | Object field to object target    | [`integration/lotus_type_matching.yaml`](integration/lotus_type_matching.yaml) |
| ✅     | Type coercion (string → number)  | [`integration/lotus_type_matching.yaml`](integration/lotus_type_matching.yaml) |
| ✅     | Type coercion (string → boolean) | [`integration/lotus_type_matching.yaml`](integration/lotus_type_matching.yaml) |
| ✅     | Nested field extraction (a.b.c)  | [`integration/lotus_type_matching.yaml`](integration/lotus_type_matching.yaml) |

### Simple Mappings

| Status | Test Case                           | Test File                                                                          |
| ------ | ----------------------------------- | ---------------------------------------------------------------------------------- |
| ✅     | Direct field-to-field mapping       | [`integration/lotus_simple_mappings.yaml`](integration/lotus_simple_mappings.yaml) |
| ✅     | Constant value mapping              | [`integration/lotus_simple_mappings.yaml`](integration/lotus_simple_mappings.yaml) |
| ✅     | Multiple fields to multiple targets | [`integration/lotus_simple_mappings.yaml`](integration/lotus_simple_mappings.yaml) |
| ✅     | Nested source to flat target        | [`integration/lotus_simple_mappings.yaml`](integration/lotus_simple_mappings.yaml) |
| ✅     | Flat source to nested target        | [`integration/lotus_simple_mappings.yaml`](integration/lotus_simple_mappings.yaml) |

### Transform Mappings

| Status | Test Case                           | Test File                                                                                |
| ------ | ----------------------------------- | ---------------------------------------------------------------------------------------- |
| ✅     | Single transformation step          | [`integration/lotus_text_actions.yaml`](integration/lotus_text_actions.yaml)             |
| ✅     | Chained transformations (A → B → C) | [`integration/lotus_chained_transforms.yaml`](integration/lotus_chained_transforms.yaml) |
| ✅     | Multiple inputs to one step         | [`integration/lotus_chained_transforms.yaml`](integration/lotus_chained_transforms.yaml) |
| ✅     | One input to multiple steps         | [`integration/lotus_chained_transforms.yaml`](integration/lotus_chained_transforms.yaml) |
| ⬚      | Aggregate step (collect into array) |                                                                                          |
| ⬚      | Aggregate step (join strings)       |                                                                                          |

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

| Status | Test Case                           | Test File                                                            |
| ------ | ----------------------------------- | -------------------------------------------------------------------- |
| ✅     | Validate entity data against schema | [`integration/ivy_validation.yaml`](integration/ivy_validation.yaml) |
| ✅     | Invalid data rejected               | [`integration/ivy_validation.yaml`](integration/ivy_validation.yaml) |
| ✅     | Required field validation           | [`integration/ivy_validation.yaml`](integration/ivy_validation.yaml) |
| ✅     | Email format validation             | [`integration/ivy_validation.yaml`](integration/ivy_validation.yaml) |

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

| Status | Test Case                                                                     | Test File                                                                    |
| ------ | ----------------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| ✅     | **Basic E2E**: Orchid + Lotus + Ivy integration test                          | [`scenarios/basic_user_flow.yaml`](scenarios/basic_user_flow.yaml)           |
| ✅     | **Orchid Execution E2E**: Create plan → trigger execution → verify queued     | [`scenarios/orchid_execution_e2e.yaml`](scenarios/orchid_execution_e2e.yaml) |
| ⬚      | **OKTA User Sync**: Auth → fetch users → transform → create Person entities   |                                                                              |
| ⬚      | **MS Graph Sync**: Auth → fetch users/devices → transform → resolve entities  |                                                                              |
| ⬚      | **Multi-Source Resolution**: OKTA + MS Graph → match by email → merged Person |                                                                              |
| ⬚      | **Relationship Flow**: Users + Managers → reports_to relationships            |                                                                              |
| ⬚      | **Device Ownership**: Users + Devices → owns relationships                    |                                                                              |
| ⬚      | **Group Membership**: Users + Groups → member_of relationships                |                                                                              |
| ⬚      | **Criteria Relationships**: Policy → has_access_to Windows devices            |                                                                              |
| ⬚      | **Full CRUD Lifecycle**: Create → Update → Query → Delete                     |                                                                              |
| ⬚      | **Error Recovery**: Partial failure → retry → complete                        |                                                                              |

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
