# Meadow End-to-End Tests

This directory contains end-to-end tests for the complete Meadow data pipeline:

- **Orchid → Lotus**: API polling through data mapping
- **Orchid → Lotus → Ivy**: Full pipeline including entity resolution

## Prerequisites

1. **Shared infrastructure running:**

   ```bash
   make infra
   # or: cd meadow && make up
   ```

2. **All services running:**

   ```bash
   # From meadow root, in separate terminals:
   make dev-orchid  # Terminal 1 - Port 3001
   make dev-lotus   # Terminal 2 - Port 3000
   make dev-ivy     # Terminal 3 - Port 3002
   ```

3. **Wait for services to be healthy:**
   ```bash
   curl http://localhost:3000/api/v1/health  # Lotus
   curl http://localhost:3001/api/v1/health  # Orchid
   curl http://localhost:3002/api/v1/health  # Ivy
   ```

## Running Tests

```bash
# From meadow root

# Run all E2E tests
make test-e2e

# Run Ivy-specific E2E tests
make test-e2e-ivy

# Run with verbose output
go test ./test/e2e/... -v -count=1

# Run specific test
go test ./test/e2e/... -v -run TestKafkaPipelineIntegration

# Skip tests if services aren't available
go test ./test/e2e/... -short
```

## Test Scenarios

### Orchid → Lotus Pipeline (`pipeline_test.go`)

#### `TestKafkaPipelineIntegration`

Tests the complete Orchid → Kafka → Lotus pipeline:

1. Creates a mapping definition in Lotus
2. Creates a binding for the mapping
3. Simulates an Orchid message to Kafka
4. Verifies Lotus processes and outputs the mapped data

#### `TestBindingFiltering`

Tests that bindings correctly filter messages based on status codes.

#### `TestHealthEndpoints`

Verifies health endpoints are working for Orchid and Lotus.

### Full Pipeline with Ivy (`ivy_pipeline_test.go`)

#### `TestFullPipelineOrchidToIvy`

Tests the complete Orchid → Lotus → Ivy pipeline:

1. Sets up entity types in Ivy
2. Configures match rules and merge strategies
3. Creates Lotus mapping for Ivy consumption
4. Sends multiple user records (including potential duplicates)
5. Verifies Lotus outputs mapped data
6. Checks Ivy for match candidates

#### `TestIvyMatchCandidateReview`

Tests the match candidate review workflow:

- List pending candidates
- Approve, reject, or defer matches

#### `TestIvyDeletionStrategies`

Tests deletion strategy CRUD operations:

- Explicit, execution-based, staleness, and none strategies

#### `TestIvyGraphQueries`

Tests the graph query API for Cypher queries.

#### `TestExecutionCompletedHandling`

Tests Ivy's handling of `execution.completed` events from Orchid.

#### `TestIvyHealthCheck`

Verifies all Ivy health endpoints.

## Configuration

Tests can be configured via environment variables:

| Variable              | Default                 | Description                 |
| --------------------- | ----------------------- | --------------------------- |
| `ORCHID_URL`          | `http://localhost:3001` | Orchid service URL          |
| `LOTUS_URL`           | `http://localhost:3000` | Lotus service URL           |
| `IVY_URL`             | `http://localhost:3002` | Ivy service URL             |
| `KAFKA_BROKERS`       | `localhost:9092`        | Kafka broker addresses      |
| `ORCHID_INPUT_TOPIC`  | `api-responses`         | Topic Orchid writes to      |
| `LOTUS_OUTPUT_TOPIC`  | `mapped-data`           | Topic Lotus writes to       |
| `ORCHID_EVENTS_TOPIC` | `orchid-events`         | Topic for execution events  |
| `IVY_EVENTS_TOPIC`    | `ivy-events`            | Topic for Ivy entity events |
| `TEST_TENANT_ID`      | `test-tenant-e2e`       | Tenant ID for tests         |

## Troubleshooting

### Services not running

If tests skip with "service not available", ensure:

- Infrastructure is running (`make infra`)
- All services are running and healthy

### Old data interference

Tests automatically clean up old bindings and filter messages by timestamp.
If you still see issues, restart the services or clear Kafka topics:

```bash
make infra-clean && make infra
```

### Cached test results

Tests use `-count=1` to disable caching. If you see `(cached)` in output:

```bash
go test -count=1 ./test/e2e/...
```
