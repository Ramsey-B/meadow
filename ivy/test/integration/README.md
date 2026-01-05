# Ivy Integration Tests

This directory contains integration tests for Ivy's core components.

## Running Tests

```bash
# From the meadow root
make test-ivy-integration

# Or directly with go
go test ./ivy/test/integration/...

# Verbose output
go test -v ./ivy/test/integration/...
```

## Test Coverage

### Match Engine Tests (`match_engine_test.go`)
- Match rule type validation
- Match condition serialization/deserialization
- Match rule model validation
- Match candidate status transitions
- Match result structure validation
- Weighted scoring logic

### Merge Engine Tests (`merge_engine_test.go`)
- Merge strategy type validation
- Field merge strategy serialization
- Merge strategy model validation
- Source priority handling
- Merged entity model validation
- Entity cluster management
- Merge conflict detection
- Merge audit log creation

### Deletion Tests (`deletion_test.go`)
- Deletion strategy type validation
- Deletion strategy model for all types (explicit, execution-based, staleness, none)
- Pending deletion lifecycle
- Execution tracking
- Lotus delete message format
- Execution completed message format

### API Tests (`api_test.go`)
- Entity type API validation
- Match rule API validation
- Merge strategy API validation
- Deletion strategy API validation
- Match candidate API validation
- Graph query API validation
- Health endpoint validation
- Error response validation
- Performance benchmarks

## Notes

- These tests do not require a running database
- They test model serialization, validation, and business logic
- For full E2E testing, see `test/e2e/` in the workspace root

