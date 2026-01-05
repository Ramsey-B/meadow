# Comprehensive Integration Tests

This document describes the comprehensive integration tests that validate complex scenarios involving entity merging, relationships, and deletion strategies.

## Test Coverage

### 1. Deep Merge Multiple Updates (`TestDeepMergeMultipleUpdates`)

**Scenario**: A person entity receives incremental updates from multiple sources over time.

**Flow**:

1. **CRM Source**: Provides basic contact info (email, name with first/last)
2. **Web Form**: Adds phone number and middle name, plus preferences
3. **API Enrichment**: Adds company information and additional preferences

**Validates**:

- ✅ Deep merging preserves all nested fields across updates
- ✅ New fields at any nesting level are added correctly
- ✅ Existing fields are not overwritten unless explicitly provided
- ✅ Fingerprint change detection works after merges
- ✅ Multiple updates to the same entity maintain data integrity

**Expected Result**: Final entity contains:

```json
{
  "email": "john.doe@example.com",
  "phone": "+1-555-1234",
  "name": {
    "first": "John",
    "middle": "Q",
    "last": "Doe"
  },
  "preferences": {
    "newsletter": true,
    "sms": false,
    "language": "en"
  },
  "company": {
    "name": "Acme Corp",
    "role": "Engineer"
  }
}
```

---

### 2. Concurrent Upserts (`TestConcurrentUpserts`)

**Scenario**: Multiple goroutines attempt to upsert the same entity simultaneously.

**Flow**:

- 10 goroutines concurrently upsert the same entity with different data
- Each uses `INSERT...ON CONFLICT` for atomic race handling

**Validates**:

- ✅ PostgreSQL's `INSERT...ON CONFLICT` handles races correctly
- ✅ Exactly one insert creates the entity, others update it
- ✅ No deadlocks or database errors occur
- ✅ All concurrent updates eventually succeed
- ✅ Final entity data reflects merged state

**Expected Result**:

- 1 result marked as `IsNew = true`
- 9 results marked as `IsNew = false`
- Final entity exists with valid merged data

---

### 3. Entity Relationship Integrity (`TestEntityRelationshipIntegrity`)

**Scenario**: Tests out-of-order message handling for relationships.

**Flow**:

1. **Relationship arrives first** (person -> company)
2. **Person entity arrives** second
3. **Company entity arrives** third

**Validates**:

- ✅ Relationships can be created before entities exist
- ✅ Entity IDs are `NULL` when entities don't exist yet
- ✅ Entities can be created in any order
- ✅ Relationship data (role, since) is preserved
- ✅ Background resolution process can later resolve entity IDs

**Expected Result**:

- Relationship created with `from_staged_entity_id = NULL`, `to_staged_entity_id = NULL`
- Both entities created successfully
- Data integrity maintained for future resolution

---

### 4. Execution-Based Deletion (`TestExecutionBasedDeletion`)

**Scenario**: Full-sync execution where entities not seen should be marked deleted.

**Flow**:

1. **Execution 1**: Creates 3 product entities (product-1, product-2, product-3)
2. **Execution 2**: Only sends 2 entities (product-1, product-2)
3. **Deletion Logic**: Marks product-3 as deleted since it wasn't in execution 2

**Validates**:

- ✅ `MarkDeletedExceptExecution()` correctly identifies missing entities
- ✅ Entities in the current execution are preserved
- ✅ Entities not in the current execution are soft-deleted
- ✅ Deletion count is accurate
- ✅ `deleted_at` timestamp is set correctly

**Expected Result**:

- product-1, product-2: Active with updated data
- product-3: Soft-deleted (`deleted_at != NULL`)
- Deletion count: 1

---

### 5. Partial Data Merge (`TestPartialDataMerge`)

**Scenario**: Different sources provide different subsets of fields for the same entity.

**Flow**:

1. **Profile Service**: Provides `profile` (bio, avatar) and `contact` (email)
2. **Analytics Service**: Adds `activity` (lastLogin, loginCount) and extends `contact` with phone

**Validates**:

- ✅ Partial updates don't lose existing data
- ✅ Nested objects are merged, not replaced
- ✅ Different sources can contribute to the same nested structure
- ✅ Final entity is the union of all partial updates

**Expected Result**: Entity contains all fields from both sources:

```json
{
  "profile": {
    "bio": "Software Engineer",
    "avatar": "https://example.com/avatar.jpg"
  },
  "activity": {
    "lastLogin": "2024-01-03T10:00:00Z",
    "loginCount": 42
  },
  "contact": {
    "email": "user@example.com",
    "phone": "+1-555-0000"
  }
}
```

---

### 6. Array and Primitive Overwrite (`TestArrayAndPrimitiveOverwrite`)

**Scenario**: Validates that arrays and primitive values are replaced, not merged.

**Flow**:

1. **Initial**: Creates entity with `tags: ["tag1", "tag2"]`, `status: "active"`
2. **Update**: Sends `tags: ["tag3", "tag4", "tag5"]`, `status: "inactive"`

**Validates**:

- ✅ Arrays are replaced entirely, not concatenated
- ✅ Primitive values (strings, numbers, booleans) are overwritten
- ✅ Nested arrays within objects are also replaced
- ✅ Object fields themselves are still deep merged

**Expected Result**:

```json
{
  "tags": ["tag3", "tag4", "tag5"], // Replaced, not ["tag1", "tag2", "tag3", "tag4", "tag5"]
  "status": "inactive", // Replaced, not merged
  "nested": {
    "items": ["item3"] // Replaced, not ["item1", "item2", "item3"]
  }
}
```

---

## Running the Tests

### Prerequisites

1. **PostgreSQL Database**: Tests require a PostgreSQL database with the deep merge function installed
2. **Migration Applied**: Run migration `0012_jsonb_deep_merge.up.sql`
3. **Test Database**: Configure a separate test database

### Running All Integration Tests

```bash
# Run all integration tests
go test ./ivy/test/integration -v

# Run comprehensive scenarios only
go test ./ivy/test/integration -v -run TestComprehensiveScenarios

# Run a specific scenario
go test ./ivy/test/integration -v -run TestComprehensiveScenarios/TestDeepMergeMultipleUpdates
```

### Running with Database

```bash
# Set test database connection
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5432
export TEST_DB_NAME=ivy_test
export TEST_DB_USER=postgres
export TEST_DB_PASSWORD=postgres

go test ./ivy/test/integration -v
```

### Skip Integration Tests (Short Mode)

```bash
# Skip integration tests that require database
go test ./ivy/test/integration -short
```

---

## Test Database Setup

### Create Test Database

```sql
CREATE DATABASE ivy_test;
\c ivy_test;

-- Run all migrations
-- (migrations are applied automatically by test suite setup)
```

### Required Migrations

The tests require these migrations to be applied:

- `0002_staged_entities.up.sql` - Core tables
- `0012_jsonb_deep_merge.up.sql` - Deep merge function

---

## Adding New Tests

When adding new comprehensive tests, follow this pattern:

```go
func (s *ComprehensiveTestSuite) TestYourScenario() {
    if s.db == nil {
        s.T().Skip("Database not configured")
    }

    // 1. Setup: Create initial state
    // 2. Action: Perform operations
    // 3. Assert: Verify expected outcomes
}
```

### Best Practices

1. **Use unique IDs**: Add `uuid.New().String()` to source IDs to avoid conflicts
2. **Test realistic scenarios**: Mirror real-world data flow patterns
3. **Verify both IsNew and IsChanged**: Ensure proper upsert behavior
4. **Check data integrity**: Unmarshal and verify JSON structure
5. **Test edge cases**: Null values, empty objects, deep nesting
6. **Document the scenario**: Add comments explaining the business case

---

## Performance Benchmarks

Run benchmarks to measure performance under load:

```bash
go test ./ivy/test/integration -bench=. -benchmem
```

---

## Continuous Integration

These tests should run in CI/CD pipeline with:

- Clean test database per run
- Parallel test execution (with proper isolation)
- Coverage reporting
- Performance regression detection

Example GitHub Actions workflow:

```yaml
name: Integration Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_DB: ivy_test
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: "1.21"
      - name: Run migrations
        run: make migrate-test
      - name: Run integration tests
        run: go test ./ivy/test/integration -v
```

---

## Troubleshooting

### Tests Skip with "Database not configured"

**Solution**: Ensure `SetupSuite()` properly initializes database connection

### "function jsonb_deep_merge does not exist"

**Solution**: Run migration `0012_jsonb_deep_merge.up.sql`

### Race condition errors

**Solution**: Check that unique constraint exists on `(tenant_id, entity_type, source_id, integration)`

### Tests hang or timeout

**Solution**:

- Check database connection pooling
- Verify no deadlocks in concurrent tests
- Increase test timeout if needed
