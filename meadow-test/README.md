# 🧪 Meadow Test

**Declarative YAML-based testing framework for the Meadow data pipeline**

Meadow Test allows you to write integration and end-to-end tests using simple YAML files instead of writing Go code. Just describe what you want to test, and the framework handles the rest.

## Why Meadow Test?

**Before** (Go code):

```go
func TestUserSync(t *testing.T) {
    // 50+ lines of setup code
    integration := CreateIntegration(...)
    plan := CreatePlan(...)
    mapping := CreateMapping(...)
    // More setup...

    // Trigger execution
    execution := TriggerExecution(...)

    // Assert on Kafka messages
    msg := ConsumeKafka(...)
    assert.Equal(t, "user-123", msg.Data["user_id"])
    // More assertions...
}
```

**After** (YAML):

```yaml
name: User Sync Test
steps:
  - create_plan:
      key: sync-users
      steps:
        - type: http_request
          url: "{{mock_api_url}}/api/v1/users"

  - trigger_execution:
      plan_key: sync-users

  - assert_kafka_message:
      topic: mapped-data
      timeout: 30s
      assertions:
        - field: data.user_id
          equals: user-123
```

## Quick Start

### Prerequisites

Meadow Test requires the following services to be running:

- **Infrastructure**: PostgreSQL, Kafka, Redis, Memgraph
- **Mock API**: For simulating external APIs (port 8090)
- **Services** (as needed): Orchid (3001), Lotus (3000), Ivy (3002)

### Installation

```bash
cd meadow-test
go install ./cmd/meadow-test
```

Or use the Makefile from the meadow root:

```bash
make install-meadow-test
```

### Run Your First Test

**Option 1: Using Make (Recommended)**

```bash
# From the meadow root directory

# 1. Start infrastructure
make infra

# 2. Start mock API (in a separate terminal)
make dev-mocks

# 3. Start services as needed (in separate terminals)
make dev-orchid  # If testing Orchid
make dev-lotus   # If testing Lotus
make dev-ivy     # If testing Ivy

# 4. Run tests
make test-meadow           # Run all tests
make test-meadow-v         # Run with verbose output
make test-meadow-dry       # Validate YAML without running
```

**Option 2: Manual Setup**

```bash
# 1. Start infrastructure
cd /path/to/meadow
docker-compose up -d

# 2. Start mock API (port 8090)
cd mocks && go run ./cmd/main.go

# 3. Start services in separate terminals
cd orchid && go run ./cmd/...
cd lotus && go run ./cmd/...
cd ivy && go run ./cmd/...

# 4. Run tests
cd meadow-test
meadow-test run tests/integration/
```

**Quick Validation (No Services Required)**

To validate your YAML test files without running them:

```bash
make test-meadow-dry
# or
meadow-test run --dry-run tests/
```

### Create Your First Test

Create `my_test.yaml`:

```yaml
name: My First Test
description: Test Kafka message publishing

steps:
  - publish_kafka:
      topic: test-topic
      key: test-key
      value:
        message: Hello Meadow!
      headers:
        tenant_id: test-tenant

  - assert_kafka_message:
      topic: test-topic
      timeout: 10s
      assertions:
        - field: message
          equals: Hello Meadow!
```

Run it:

```bash
meadow-test run my_test.yaml
```

## Test Structure

### Basic Test Format

```yaml
name: Test Name
description: What this test validates

# Optional: run before all steps
setup:
  - step_type: params

# Main test steps
steps:
  - step_type: params
  - another_step: params

# Optional: run after test (always runs)
cleanup:
  - step_type: params
```

### Variable Interpolation

Use `{{variable}}` syntax to reference:

- **Saved variables** (from `save_as`):

```yaml
- create_plan:
    key: my-plan
    save_as: plan_id

- http_request:
    path: /api/v1/plans/{{plan_id}}
```

- **Built-in variables**:

  - `{{orchid_url}}` - Orchid service URL
  - `{{lotus_url}}` - Lotus service URL
  - `{{ivy_url}}` - Ivy service URL
  - `{{mock_api_url}}` - Mock API URL
  - `{{kafka_brokers}}` - Kafka broker list
  - `{{test_tenant}}` - Test tenant ID
  - `{{timestamp}}` - Current Unix timestamp
  - `{{uuid}}` - Generated UUID

- **Environment variables**:

```yaml
- assert:
    variable: MY_ENV_VAR
    equals: expected_value
```

## Available Steps

### Generic Steps

#### `wait` - Pause execution

```yaml
- wait:
    duration: 5s
    reason: Wait for processing
```

#### `assert` - Make assertions

Condition-based:

```yaml
- assert:
    condition: "{{execution_id}} != ''"
    message: Execution ID should not be empty
```

Variable-based:

```yaml
- assert:
    variable: execution_status
    equals: success
    message: Execution should succeed
```

#### `http_request` - Make HTTP requests

```yaml
- http_request:
    service: orchid # or lotus, ivy, mocks, or full URL
    method: GET
    path: /api/v1/plans/{{plan_id}}
    headers:
      X-Custom-Header: value
    body:
      key: value
    expect:
      status: 200
      body:
        key: value
    save_as: response
```

### Kafka Steps

#### `publish_kafka` - Publish message

```yaml
- publish_kafka:
    topic: api-responses
    key: test-key
    value:
      tenant_id: "{{test_tenant}}"
      data:
        user_id: user-123
    headers:
      tenant_id: "{{test_tenant}}"
```

#### `assert_kafka_message` - Consume and assert

```yaml
- assert_kafka_message:
    topic: mapped-data
    timeout: 30s
    consume_from: latest # or earliest
    assertions:
      - header: tenant_id
        equals: "{{test_tenant}}"
      - field: data.user_id
        equals: user-123
      - field: data.name
        contains: John
      - field: data.email
        not_null: true
    save_as: message
```

#### `mock_api` - Configure mock API

```yaml
- mock_api:
    method: GET
    path: /api/v1/users
    response:
      status: 200
      delay_ms: 100
      body:
        users:
          - id: user-123
            name: John Doe
```

### Orchid Steps

#### `create_integration` - Create integration

```yaml
- create_integration:
    name: Test OKTA
    type: okta
    config:
      base_url: "{{mock_api_url}}"
    save_as: integration_id
```

#### `create_plan` - Create execution plan

```yaml
- create_plan:
    key: sync-users-{{uuid}}
    name: Sync Users
    integration_id: "{{integration_id}}"
    description: Sync user data from OKTA
    plan_definition:
      type: http_request
      url: "{{mock_api_url}}/api/v1/users"
      method: GET
    enabled: true
    save_as: plan_id
```

#### `trigger_execution` - Trigger plan execution

```yaml
- trigger_execution:
    plan_key: sync-users
    wait_for_completion: true
    timeout: 60s
    parameters:
      sync_mode: full
    save_as: execution_id
```

### Lotus Steps

#### `create_mapping` - Create data mapping

```yaml
- create_mapping:
    name: User Mapping
    description: Maps OKTA users to standard format
    source_fields:
      - id: src_id
        path: users[*].id
      - id: src_name
        path: users[*].name
    target_fields:
      - id: tgt_id
        path: user_id
      - id: tgt_name
        path: name
    links:
      - source: src_id
        target: tgt_id
      - source: src_name
        target: tgt_name
    save_as: mapping_id
```

#### `create_binding` - Create mapping binding

```yaml
- create_binding:
    mapping_id: "{{mapping_id}}"
    filter:
      integration: okta
      min_status_code: 200
      max_status_code: 299
    save_as: binding_id
```

### Ivy Steps

#### `create_entity_type` - Create entity type

```yaml
- create_entity_type:
    name: user
    description: User entities from identity providers
    schema:
      email: string
      name: string
    save_as: entity_type_id
```

#### `create_match_rule` - Create matching rule

```yaml
- create_match_rule:
    entity_type: user
    name: Email-based matching
    conditions:
      - field: email
        match_type: exact
        weight: 1.0
      - field: name
        match_type: fuzzy
        weight: 0.5
    threshold: 0.8
    save_as: match_rule_id
```

#### `query_entities` - Query entities

```yaml
- query_entities:
    entity_type: user
    filters:
      source_id: user-123
    expect:
      count: 1
      items:
        - data.email: john@example.com
          data.name: John Doe
    save_as: entities
```

## CLI Usage

### Run Tests

```bash
# Run all tests in directory
meadow-test run tests/

# Run specific test file
meadow-test run tests/scenarios/user_sync.yaml

# Run multiple files
meadow-test run tests/integration/*.yaml

# Dry run (validate without executing)
meadow-test run --dry-run tests/

# Verbose output
meadow-test run -v tests/

# Custom service URLs
meadow-test run \
  --orchid-url http://localhost:8080 \
  --lotus-url http://localhost:8081 \
  --ivy-url http://localhost:8082 \
  tests/

# Custom Kafka brokers
meadow-test run --kafka-brokers localhost:9092 tests/

# Custom tenant ID
meadow-test run --tenant my-tenant tests/
```

### Configuration Flags

| Flag              | Default                 | Description                     |
| ----------------- | ----------------------- | ------------------------------- |
| `--orchid-url`    | `http://localhost:8080` | Orchid service URL              |
| `--lotus-url`     | `http://localhost:8081` | Lotus service URL               |
| `--ivy-url`       | `http://localhost:8082` | Ivy service URL                 |
| `--mocks-url`     | `http://localhost:9000` | Mock API service URL            |
| `--kafka-brokers` | `localhost:9092`        | Kafka brokers (comma-separated) |
| `--tenant`        | `test-tenant`           | Test tenant ID                  |
| `-v, --verbose`   | `false`                 | Verbose output                  |
| `--dry-run`       | `false`                 | Validate without executing      |

## Example Tests

### Integration Test

Test a single service (Orchid):

```yaml
name: Orchid Plan CRUD
description: Test creating, reading, and deleting plans

steps:
  - create_plan:
      key: test-plan-{{uuid}}
      name: Test Plan
      steps:
        - type: http_request
          url: https://api.example.com/test
      save_as: plan_id

  - http_request:
      service: orchid
      method: GET
      path: /api/v1/plans/{{plan_id}}
      expect:
        status: 200

cleanup:
  - http_request:
      service: orchid
      method: DELETE
      path: /api/v1/plans/{{plan_id}}
```

### End-to-End Test

Test the full pipeline (Orchid → Lotus → Ivy):

```yaml
name: User Sync End-to-End
description: Test complete user sync pipeline

setup:
  - mock_api:
      method: GET
      path: /api/v1/users
      response:
        status: 200
        body:
          users:
            - id: user-123
              name: John Doe
              email: john@example.com

steps:
  - create_integration:
      name: Test OKTA
      type: okta
      save_as: integration_id

  - create_plan:
      key: sync-users
      steps:
        - type: http_request
          url: "{{mock_api_url}}/api/v1/users"
      save_as: plan_id

  - create_mapping:
      name: User Mapping
      source_fields:
        - id: src_id
          path: users[*].id
      target_fields:
        - id: tgt_id
          path: user_id
      links:
        - source: src_id
          target: tgt_id
      save_as: mapping_id

  - create_binding:
      mapping_id: "{{mapping_id}}"
      save_as: binding_id

  - trigger_execution:
      plan_key: sync-users
      wait_for_completion: true

  - assert_kafka_message:
      topic: mapped-data
      timeout: 30s
      assertions:
        - field: data.user_id
          equals: user-123

  - query_entities:
      entity_type: user
      filters:
        source_id: user-123
      expect:
        count: 1
```

## Advanced Features

### Fixtures

Create reusable test data in `tests/helpers/fixtures.yaml`:

```yaml
fixtures:
  sample_users:
    - id: user-001
      name: John Doe
      email: john@example.com
    - id: user-002
      name: Jane Smith
      email: jane@example.com
```

Use in tests:

```yaml
- mock_api:
    method: GET
    path: /api/v1/users
    response:
      body:
        users: "{{fixture:sample_users}}"
```

### Templates

Create reusable step sequences in `tests/helpers/templates.yaml`:

```yaml
templates:
  create_okta_integration:
    steps:
      - create_integration:
          name: OKTA {{timestamp}}
          type: okta
          save_as: integration_id

  setup_user_sync:
    steps:
      - use_template: create_okta_integration
      - create_plan:
          key: sync-users-{{uuid}}
          steps:
            - type: http_request
              url: "{{mock_api_url}}/api/v1/users"
          save_as: plan_id
```

Use in tests:

```yaml
steps:
  - use_template: setup_user_sync
  - trigger_execution:
      plan_key: sync-users-{{uuid}}
```

## Tips & Best Practices

### 1. Use UUIDs for Unique Keys

Always use `{{uuid}}` or `{{timestamp}}` for unique identifiers:

```yaml
- create_plan:
    key: my-plan-{{uuid}} # ✅ Unique
    # NOT: key: my-plan     # ❌ Conflicts if run multiple times
```

### 2. Always Clean Up

Use the `cleanup` section to delete created resources:

```yaml
cleanup:
  - http_request:
      service: lotus
      method: DELETE
      path: /api/v1/bindings/{{binding_id}}
```

### 3. Add Descriptions

Make tests self-documenting:

```yaml
name: User Sync Test
description: |
  Tests the complete user synchronization pipeline:
  1. Orchid fetches users from OKTA
  2. Lotus transforms the data
  3. Ivy creates deduplicated entities
```

### 4. Use Verbose Mode for Debugging

```bash
meadow-test run -v tests/my_test.yaml
```

Shows:

- Variable interpolation
- HTTP requests/responses
- Kafka messages
- Step-by-step execution

### 5. Start Simple, Build Up

Begin with integration tests for individual services, then combine into end-to-end scenarios.

### 6. Write Meaningful Assertions

**Don't** just check if values exist - verify they match expected data:

```yaml
# ❌ Bad: Only checks existence
- assert:
    variable: result.email
    not_empty: true
    message: Email should exist

# ✅ Good: Verifies actual data
- assert:
    variable: result.email
    equals: "john.doe@example.com"
    message: Email should match expected value
```

**Do** verify specific values, especially for:

- Transformed data (lowercase, trimmed, concatenated)
- Mapped fields (source → target)
- Computed values (calculations, counts)
- API response bodies (user data, error messages)

```yaml
# Verify transformation results
- assert:
    variable: lowercase_result.target_raw.email
    equals: "john.doe@example.com"
    message: Email should be lowercased from JOHN.DOE@EXAMPLE.COM

# Verify array operations
- assert:
    variable: length_result.target_raw.count
    equals: 3
    message: Array length should be 3

# Verify Kafka message data
- assert_kafka_message:
    topic: api-responses
    assertions:
      - field: response_body[0].profile.firstName
        equals: "John"
      - field: response_body[0].profile.department
        equals: "Engineering"
```

### 7. Use get_kafka_offset for Reliable Kafka Tests

Always capture the offset before triggering an action:

```yaml
# Capture offset BEFORE the action
- get_kafka_offset:
    topic: api-responses
    save_as: kafka_offset_before

# Trigger the action that produces messages
- http_request:
    service: orchid
    method: POST
    path: /api/v1/plans/{{plan.key}}/trigger

# Consume from the saved offset
- assert_kafka_message:
    topic: api-responses
    from_offset: "{{kafka_offset_before}}"
    filter:
      headers:
        plan_key: "{{plan.key}}"
```

## Troubleshooting

### Test Fails: "Connection refused"

Services aren't running. Start them:

```bash
cd /path/to/meadow
docker-compose up -d
```

### Test Fails: "Timeout waiting for message"

- Check Kafka topic exists and has messages
- Increase timeout value
- Use `consume_from: earliest` instead of `latest`
- Check `tenant_id` header matches

### Variable Not Interpolating

- Make sure you use `{{variable_name}}` syntax
- Check the variable was saved with `save_as`
- Use `-v` flag to see variable values

### Assertion Fails

- Use `-v` to see actual vs expected values
- Check field paths are correct (`data.user_id` not `user_id`)
- Verify types match (string vs int)

## Development

### Project Structure

```
meadow-test/
├── cmd/
│   └── meadow-test/
│       └── main.go           # CLI entry point
├── pkg/
│   ├── runner/
│   │   ├── runner.go         # Test execution
│   │   ├── context.go        # Variable interpolation
│   │   └── steps.go          # Step dispatcher
│   └── steps/
│       ├── common.go         # Generic steps
│       ├── kafka.go          # Kafka steps
│       ├── orchid.go         # Orchid steps
│       ├── lotus.go          # Lotus steps
│       └── ivy.go            # Ivy steps
└── tests/
    ├── integration/          # Service-specific tests
    ├── scenarios/            # End-to-end tests
    └── helpers/
        ├── fixtures.yaml     # Test data
        └── templates.yaml    # Reusable steps
```

### Adding a New Step Type

1. Define the step function in the appropriate file (e.g., `pkg/steps/orchid.go`):

```go
func MyNewStep(ctx TestContext, params interface{}) error {
    paramsMap, ok := params.(map[string]interface{})
    if !ok {
        return fmt.Errorf("my_new_step params must be a map")
    }

    // Extract parameters
    name := paramsMap["name"].(string)
    name = ctx.Interpolate(name).(string)

    // Do something...
    ctx.Log("Executing my new step: %s", name)

    return nil
}
```

2. Register it in `pkg/runner/steps.go`:

```go
switch stepType {
// ... existing cases
case "my_new_step":
    return steps.MyNewStep(testCtx, params)
}
```

3. Document it in this README!

## Contributing

Contributions welcome! Please:

1. Add tests for new step types
2. Update documentation
3. Follow existing code patterns
4. Keep it simple and declarative

## License

Same as Meadow project.

---

**Happy Testing! 🧪**

For questions or issues, see the main Meadow documentation or reach out to the team.
