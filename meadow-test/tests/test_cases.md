# Meadow Test Cases

This document tracks all planned and implemented test cases for the Meadow data pipeline.

## Legend

| Status | Meaning                 |
| ------ | ----------------------- |
| ✅     | Implemented             |
| 🔄     | In Progress             |
| ⬚      | Planned                 |
| 💡     | Idea (needs refinement) |

## Test Summary

| Category      | Implemented | Planned | Total   |
| ------------- | ----------- | ------- | ------- |
| Smoke Tests   | 4           | 0       | 4       |
| Orchid        | 38          | 6       | 44      |
| Lotus         | 54          | 2       | 56      |
| Ivy           | 25          | 17      | 42      |
| Kafka         | 5           | 2       | 7       |
| E2E Scenarios | 6           | 7       | 13      |
| **Total**     | **132**     | **34**  | **166** |

**Current Test Suite: 76 YAML test files (75 passing, 1 needs Lotus restart)**

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

| Status | Test Case                                    | Test File                                                                                        |
| ------ | -------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| ✅     | Create/Read/Update/Delete integration        | [`integration/orchid_integration_crud.yaml`](integration/orchid_integration_crud.yaml)           |
| ✅     | Create integration with config schema        | [`integration/orchid_integration_crud.yaml`](integration/orchid_integration_crud.yaml)           |
| ✅     | Create multiple configs for same integration | [`integration/orchid_config_management.yaml`](integration/orchid_config_management.yaml)         |
| ✅     | Enable/disable configs                       | [`integration/orchid_config_enable_disable.yaml`](integration/orchid_config_enable_disable.yaml) |
| ⬚      | Config validation against schema             |                                                                                                  |

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

| Status | Test Case                                  | Test File                                                                                  |
| ------ | ------------------------------------------ | ------------------------------------------------------------------------------------------ |
| ✅     | Retry on 429 (rate limit)                  | [`integration/orchid_retry_429.yaml`](integration/orchid_retry_429.yaml)                   |
| ✅     | Retry on 5xx (server error)                | [`integration/orchid_retry_5xx.yaml`](integration/orchid_retry_5xx.yaml)                   |
| ✅     | Retry with exponential backoff             | [`integration/orchid_retry_5xx.yaml`](integration/orchid_retry_5xx.yaml)                   |
| ✅     | Abort on 401 (unauthorized)                | [`integration/orchid_abort_401.yaml`](integration/orchid_abort_401.yaml)                   |
| ✅     | Abort on 403 (forbidden)                   | [`integration/orchid_abort_403.yaml`](integration/orchid_abort_403.yaml)                   |
| ✅     | Continue on 404 (not found)                | [`integration/orchid_continue_404.yaml`](integration/orchid_continue_404.yaml)             |
| ✅     | Intermittent failures (retry succeeds)     | [`integration/orchid_retry_intermittent.yaml`](integration/orchid_retry_intermittent.yaml) |
| ✅     | Persistent failures (max retries exceeded) | [`integration/orchid_persistent_failure.yaml`](integration/orchid_persistent_failure.yaml) |
| ✅     | Timeout handling                           | [`integration/orchid_timeout.yaml`](integration/orchid_timeout.yaml)                       |

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

| Status | Test Case                              | Test File                                                                                |
| ------ | -------------------------------------- | ---------------------------------------------------------------------------------------- |
| ✅     | Create/Delete binding                  | [`integration/lotus_mapping_crud.yaml`](integration/lotus_mapping_crud.yaml)             |
| ✅     | Enable/disable binding                 | [`integration/lotus_binding_management.yaml`](integration/lotus_binding_management.yaml) |
| ✅     | Binding with filter (integration name) | [`integration/lotus_binding_management.yaml`](integration/lotus_binding_management.yaml) |
| ✅     | Binding with filter (plan keys)        | [`integration/lotus_binding_management.yaml`](integration/lotus_binding_management.yaml) |
| ✅     | Multiple bindings for same mapping     | [`integration/lotus_binding_management.yaml`](integration/lotus_binding_management.yaml) |

### Actions (Transformations)

| Status | Test Case                                                                        | Test File                                                                                  |
| ------ | -------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| ✅     | List available actions                                                           | [`integration/lotus_actions.yaml`](integration/lotus_actions.yaml)                         |
| ✅     | Get action output types                                                          | [`integration/lotus_actions.yaml`](integration/lotus_actions.yaml)                         |
| ✅     | Inline mapping test                                                              | [`integration/lotus_actions.yaml`](integration/lotus_actions.yaml)                         |
| ✅     | Text actions (to_lower, to_upper, trim)                                          | [`integration/lotus_text_actions.yaml`](integration/lotus_text_actions.yaml)               |
| ✅     | Text concat with separator                                                       | [`integration/lotus_text_actions.yaml`](integration/lotus_text_actions.yaml)               |
| ✅     | Text split to array                                                              | [`integration/lotus_text_actions.yaml`](integration/lotus_text_actions.yaml)               |
| ✅     | Number operations (add, multiply)                                                | [`integration/lotus_number_actions.yaml`](integration/lotus_number_actions.yaml)           |
| ✅     | Number operations (abs, floor, ceil, round, sqrt, square)                        | [`integration/lotus_number_advanced.yaml`](integration/lotus_number_advanced.yaml)         |
| ✅     | Number operations (modulus, sign, is_even, is_odd)                               | [`integration/lotus_number_advanced.yaml`](integration/lotus_number_advanced.yaml)         |
| ✅     | Date parsing and formatting                                                      | [`integration/lotus_date_actions.yaml`](integration/lotus_date_actions.yaml)               |
| ✅     | Coalesce (first non-null)                                                        | [`integration/lotus_coalesce_default.yaml`](integration/lotus_coalesce_default.yaml)       |
| ✅     | Default value fallback                                                           | [`integration/lotus_coalesce_default.yaml`](integration/lotus_coalesce_default.yaml)       |
| ✅     | Array operations (push, length, contains)                                        | [`integration/lotus_array_actions.yaml`](integration/lotus_array_actions.yaml)             |
| ✅     | Array operations (distinct, reverse, index_of, randomize)                        | [`integration/lotus_array_advanced.yaml`](integration/lotus_array_advanced.yaml)           |
| ✅     | Array operations (skip, take, every)                                             | [`integration/lotus_array_skip_take.yaml`](integration/lotus_array_skip_take.yaml)         |
| ✅     | Object operations (get, pick, omit, merge)                                       | [`integration/lotus_object_actions.yaml`](integration/lotus_object_actions.yaml)           |
| ✅     | Conditional (is_nil, is_empty, to_string)                                        | [`integration/lotus_conditional_actions.yaml`](integration/lotus_conditional_actions.yaml) |
| ✅     | Regex match and replace                                                          | [`integration/lotus_regex_actions.yaml`](integration/lotus_regex_actions.yaml)             |
| ✅     | Advanced text operations (contains, starts_with, ends_with, substring, index_of) | [`integration/lotus_text_advanced.yaml`](integration/lotus_text_advanced.yaml)             |

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

| Status | Test Case                        | Test File                                                                          |
| ------ | -------------------------------- | ---------------------------------------------------------------------------------- |
| ✅     | Condition passes - step executes | [`integration/lotus_condition_steps.yaml`](integration/lotus_condition_steps.yaml) |
| ✅     | Condition fails - step skipped   | [`integration/lotus_condition_steps.yaml`](integration/lotus_condition_steps.yaml) |
| ✅     | Inverted condition               | [`integration/lotus_condition_steps.yaml`](integration/lotus_condition_steps.yaml) |
| ✅     | Text condition (empty check)     | [`integration/lotus_condition_steps.yaml`](integration/lotus_condition_steps.yaml) |
| ✅     | Regex condition                  | [`integration/lotus_condition_steps.yaml`](integration/lotus_condition_steps.yaml) |
| ⬚      | Filter items from array          |                                                                                    |
| ⬚      | Multiple conditions (AND/OR)     |                                                                                    |

### Validation Steps

| Status | Test Case                        | Test File                                                                            |
| ------ | -------------------------------- | ------------------------------------------------------------------------------------ |
| ✅     | Validate mapping definition      | [`integration/lotus_actions.yaml`](integration/lotus_actions.yaml)                   |
| ✅     | Validator step passes            | [`integration/lotus_validator_steps.yaml`](integration/lotus_validator_steps.yaml)   |
| ✅     | Validator step fails (not empty) | [`integration/lotus_validator_steps.yaml`](integration/lotus_validator_steps.yaml)   |
| ✅     | Number validation (is_even)      | [`integration/lotus_validator_steps.yaml`](integration/lotus_validator_steps.yaml)   |
| ✅     | Regex pattern validation         | [`integration/lotus_validator_steps.yaml`](integration/lotus_validator_steps.yaml)   |
| ✅     | Chained validators               | [`integration/lotus_validator_steps.yaml`](integration/lotus_validator_steps.yaml)   |
| ⬚      | Required field validation        |                                                                                      |
| ⬚      | Format validation (email, url)   |                                                                                      |
| 🔄     | Range validation (min/max)       | [`integration/lotus_range_validation.yaml`](integration/lotus_range_validation.yaml) |

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

| Status | Test Case                         | Test File                                                                        |
| ------ | --------------------------------- | -------------------------------------------------------------------------------- |
| ✅     | Create entity type with schema    | [`integration/ivy_entity_types.yaml`](integration/ivy_entity_types.yaml)         |
| ✅     | List entity types                 | [`integration/ivy_entity_types.yaml`](integration/ivy_entity_types.yaml)         |
| ✅     | Get entity type by ID             | [`integration/ivy_entity_type_crud.yaml`](integration/ivy_entity_type_crud.yaml) |
| ✅     | Delete entity type                | [`integration/ivy_entity_type_crud.yaml`](integration/ivy_entity_type_crud.yaml) |
| ✅     | Update entity type                | [`integration/ivy_entity_type_crud.yaml`](integration/ivy_entity_type_crud.yaml) |
| ✅     | Entity type with merge strategies | [`integration/ivy_merge_strategies.yaml`](integration/ivy_merge_strategies.yaml) |

### Relationship Type Management

| Status | Test Case                                       | Test File                                                                            |
| ------ | ----------------------------------------------- | ------------------------------------------------------------------------------------ |
| ✅     | Create relationship type                        | [`integration/ivy_relationship_types.yaml`](integration/ivy_relationship_types.yaml) |
| ✅     | Relationship cardinality (1:1, 1:N, N:N)        | [`integration/ivy_relationship_types.yaml`](integration/ivy_relationship_types.yaml) |
| ✅     | Self-referential relationship (person → person) | [`integration/ivy_relationship_types.yaml`](integration/ivy_relationship_types.yaml) |
| ✅     | Cross-entity relationship (person → device)     | [`integration/ivy_relationship_types.yaml`](integration/ivy_relationship_types.yaml) |

### Match Rules

| Status | Test Case                                 | Test File                                                                          |
| ------ | ----------------------------------------- | ---------------------------------------------------------------------------------- |
| ✅     | Create exact match rule                   | [`integration/ivy_match_rules_crud.yaml`](integration/ivy_match_rules_crud.yaml)   |
| ✅     | List match rules by entity type           | [`integration/ivy_match_rules_crud.yaml`](integration/ivy_match_rules_crud.yaml)   |
| ✅     | Fuzzy match rule (similarity threshold)   | [`integration/ivy_match_rules_crud.yaml`](integration/ivy_match_rules_crud.yaml)   |
| ✅     | Phonetic match rule                       | [`integration/ivy_match_rules_crud.yaml`](integration/ivy_match_rules_crud.yaml)   |
| ✅     | Multi-field match rule                    | [`integration/ivy_match_rules_crud.yaml`](integration/ivy_match_rules_crud.yaml)   |
| ✅     | Match with normalizers (lowercase, phone) | [`integration/ivy_match_normalizers.yaml`](integration/ivy_match_normalizers.yaml) |
| ✅     | Match priority ordering                   | [`integration/ivy_match_rules_crud.yaml`](integration/ivy_match_rules_crud.yaml)   |
| ✅     | Blocking rules (prevent merge)            | [`integration/ivy_match_rules_crud.yaml`](integration/ivy_match_rules_crud.yaml)   |

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

| Status | Test Case                       | Test File                                                                        |
| ------ | ------------------------------- | -------------------------------------------------------------------------------- |
| ✅     | List pending match candidates   | [`integration/ivy_match_candidates.yaml`](integration/ivy_match_candidates.yaml) |
| ✅     | Filter by status                | [`integration/ivy_match_candidates.yaml`](integration/ivy_match_candidates.yaml) |
| ⬚      | Approve match candidate (merge) |                                                                                  |
| ⬚      | Reject match candidate          |                                                                                  |
| ⬚      | Defer match candidate           |                                                                                  |

### Deletion Strategies

| Status | Test Case                           | Test File                                                                              |
| ------ | ----------------------------------- | -------------------------------------------------------------------------------------- |
| ✅     | Execution-based deletion strategy   | [`integration/ivy_deletion_strategies.yaml`](integration/ivy_deletion_strategies.yaml) |
| ✅     | Explicit deletion strategy          | [`integration/ivy_deletion_strategies.yaml`](integration/ivy_deletion_strategies.yaml) |
| ✅     | Staleness-based deletion strategy   | [`integration/ivy_deletion_strategies.yaml`](integration/ivy_deletion_strategies.yaml) |
| ✅     | List and filter deletion strategies | [`integration/ivy_deletion_strategies.yaml`](integration/ivy_deletion_strategies.yaml) |
| ⬚      | Grace period before deletion        |                                                                                        |
| ⬚      | Cascade delete relationships        |                                                                                        |

### Graph Queries

| Status | Test Case                      | Test File                                                                  |
| ------ | ------------------------------ | -------------------------------------------------------------------------- |
| ⬚      | Find entity by property        |                                                                            |
| ⬚      | Find entity relationships      |                                                                            |
| ✅     | Shortest path between entities | [`integration/ivy_graph_queries.yaml`](integration/ivy_graph_queries.yaml) |
| ✅     | Neighbor traversal             | [`integration/ivy_graph_queries.yaml`](integration/ivy_graph_queries.yaml) |
| ✅     | Cypher query execution         | [`integration/ivy_graph_queries.yaml`](integration/ivy_graph_queries.yaml) |

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

| Status | Test Case                  | Test File                                                          |
| ------ | -------------------------- | ------------------------------------------------------------------ |
| ✅     | Publish message to topic   | [`integration/kafka_pubsub.yaml`](integration/kafka_pubsub.yaml)   |
| ✅     | Consume and verify message | [`integration/kafka_pubsub.yaml`](integration/kafka_pubsub.yaml)   |
| ✅     | Topic auto-creation        | [`integration/kafka_simple.yaml`](integration/kafka_simple.yaml)   |
| ✅     | Message with headers       | [`integration/kafka_headers.yaml`](integration/kafka_headers.yaml) |
| ✅     | Filter by header value     | [`integration/kafka_headers.yaml`](integration/kafka_headers.yaml) |
| ⬚      | Message key partitioning   |                                                                    |
| ⬚      | Consumer group handling    |                                                                    |

---

## End-to-End Scenarios

Full pipeline tests that exercise multiple services.

| Status | Test Case                                                                     | Test File                                                                                      |
| ------ | ----------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| ✅     | **Basic E2E**: Orchid + Lotus + Ivy integration test                          | [`scenarios/basic_user_flow.yaml`](scenarios/basic_user_flow.yaml)                             |
| ✅     | **Orchid Execution E2E**: Create plan → trigger execution → verify Kafka      | [`scenarios/orchid_execution_e2e.yaml`](scenarios/orchid_execution_e2e.yaml)                   |
| ✅     | **OKTA User Sync**: OAuth2 auth → fetch users → verify Kafka with user data   | [`integration/orchid_okta_users_sync.yaml`](integration/orchid_okta_users_sync.yaml)           |
| ✅     | **MS Graph Users E2E**: OAuth2 → fetch users → verify Kafka response          | [`integration/orchid_ms_graph_users_e2e.yaml`](integration/orchid_ms_graph_users_e2e.yaml)     |
| ✅     | **MS Graph Devices E2E**: OAuth2 → fetch devices → verify Kafka response      | [`integration/orchid_ms_graph_devices_e2e.yaml`](integration/orchid_ms_graph_devices_e2e.yaml) |
| ✅     | **OAuth2 Full E2E**: Token endpoint → protected API → verify data             | [`integration/orchid_oauth2_e2e.yaml`](integration/orchid_oauth2_e2e.yaml)                     |
| ⬚      | **Multi-Source Resolution**: OKTA + MS Graph → match by email → merged Person |                                                                                                |
| ⬚      | **Relationship Flow**: Users + Managers → reports_to relationships            |                                                                                                |
| ⬚      | **Device Ownership**: Users + Devices → owns relationships                    |                                                                                                |
| ⬚      | **Group Membership**: Users + Groups → member_of relationships                |                                                                                                |
| ⬚      | **Criteria Relationships**: Policy → has_access_to Windows devices            |                                                                                                |
| ⬚      | **Full CRUD Lifecycle**: Create → Update → Query → Delete                     |                                                                                                |
| ⬚      | **Error Recovery**: Partial failure → retry → complete                        |                                                                                                |

**Note:** E2E tests include comprehensive Kafka message assertions that verify specific field values (user profiles, device properties, etc.) rather than just checking message existence.

---

## Test Infrastructure

| Status | Test Case                                | Test File / Location                                          |
| ------ | ---------------------------------------- | ------------------------------------------------------------- |
| ✅     | Mock API dynamic configuration           | `mocks/cmd/main.go` - `/api/test/configure` endpoint          |
| ✅     | Test templates (reusable step sequences) | [`helpers/templates.yaml`](helpers/templates.yaml)            |
| ✅     | Test isolation (cleanup between tests)   | `cleanup:` section in each test + tenant isolation            |
| ✅     | Parallel test execution                  | `meadow-test run -p 4` (configurable parallelism)             |
| ✅     | Kafka offset tracking                    | `get_kafka_offset` + `from_offset` for reliable message tests |
| ⬚      | Test data fixtures/seeding               |                                                               |
| ⬚      | JUnit report generation                  |                                                               |
| ⬚      | JSON report generation                   |                                                               |

---

## Notes

- Tests in `smoke/` should be fast and run on every deploy
- Tests in `integration/` test individual service APIs
- Tests in `scenarios/` test cross-service workflows
- Benchmarks should be run separately, not in CI
- Do NOT make tests that pass just to pass
- If a test fails, investigate why - do NOT just update to pass if the test is valid

### Assertion Quality Guidelines

All tests should include **meaningful assertions** that verify actual data values, not just existence checks:

```yaml
# ❌ Bad: Only checks existence
- assert:
    variable: result.email
    not_empty: true

# ✅ Good: Verifies actual data
- assert:
    variable: result.email
    equals: "john.doe@example.com"
```

Tests with specific data assertions:

- **Lotus transformation tests**: Verify exact output values (e.g., `to_lower` produces `"hello"`)
- **Orchid execution tests**: Verify Kafka messages contain expected response data
- **Ivy validation tests**: Verify specific validation error messages

### Known Limitations

- **Object keys/values order**: Go maps have non-deterministic iteration order, so `object_keys` and `object_values` assertions check presence, not position
- **Regex extraction**: Returns full match including prefix (e.g., `@domain.com` not `domain.com`)
