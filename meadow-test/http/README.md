# Manual Testing with REST Client

This directory contains `.http` files for manual testing using the [REST Client](https://marketplace.visualstudio.com/items?itemName=humao.rest-client) VS Code extension.

## Prerequisites

1. Install the REST Client extension in VS Code
2. Start the infrastructure: `make infra`
3. Start the services you need (in separate terminals)

## Quick Start

### 1. Start Mock API Server
```bash
make dev-mocks
```
This starts mock OKTA and MS Graph APIs on `http://localhost:8090`

### 2. Start Services
```bash
# Terminal 1
make dev-orchid

# Terminal 2
make dev-lotus

# Terminal 3
make dev-ivy
```

### 3. Open .http Files

Open the `.http` files in VS Code and click "Send Request" above each request.

## File Descriptions

| File | Purpose | Port |
|------|---------|------|
| `mock-apis.http` | Test mock OKTA & MS Graph endpoints | 8090 |
| `orchid.http` | Create integrations, plans, trigger executions | 3001 |
| `lotus.http` | Create mappings, bindings, test transformations (includes `source.constant`) | 3000 |
| `ivy.http` | Define entities, match rules, merge strategies, query graph | 3002 |

## Workflow Example

### Setting Up the Full Pipeline

1. **Mock APIs** (`mock-apis.http`)
   - Get OKTA token
   - Test user/group endpoints
   - Get MS Graph token
   - Test user/device/group endpoints

2. **Ivy** (`ivy.http`)
   - Create Entity Types (person, device, group)
   - Create Relationship Types (owns, member_of, reports_to)
   - Create Match Rules
   - Create Merge Strategies

3. **Lotus** (`lotus.http`)
   - Create Mappings (OKTA user → person, MS Graph user → person, etc.)
   - Create Bindings to connect Orchid sources to mappings

4. **Orchid** (`orchid.http`)
   - Create Integrations (OKTA Mock, MS Graph Mock)
   - Create Plans (user sync, device sync)
   - Trigger Executions

5. **Verify** (`ivy.http`)
   - Check Match Candidates
   - Review/Approve matches
   - Query the graph

## Criteria-Based Relationships

Ivy supports **criteria-based relationships** that connect one entity to multiple entities matching a criteria. This is useful for policies like "User X has access to all Windows devices".

### Direct vs Criteria-Based

**Direct Relationship** (1-to-1):
```json
{
  "_relationship_type": "owns",
  "_from_entity_type": "person",
  "_from_source_id": "user-001",
  "_to_entity_type": "device",
  "_to_source_id": "device-001"
}
```

**Criteria-Based Relationship** (1-to-many matching criteria):
```json
{
  "_relationship_type": "has_access_to",
  "_from_entity_type": "person",
  "_from_source_id": "user-001",
  "_from_integration": "okta",
  "_to_entity_type": "device",
  "_to_integration": "msgraph",
  "_to_criteria": {
    "operating_system": "Windows",
    "is_compliant": true
  }
}
```

### Supported Operators

| Operator | Example | Description |
|----------|---------|-------------|
| equality | `{"field": "value"}` | Field equals value |
| `$contains` | `{"tags": {"$contains": "FISMA"}}` | Array contains value |
| `$in` | `{"platform": {"$in": ["Windows", "macOS"]}}` | Value in list |
| `$gte/$gt/$lte/$lt` | `{"risk_score": {"$gte": 80}}` | Numeric comparisons |
| `$exists` | `{"serial_number": {"$exists": true}}` | Field exists |
| `$ne` | `{"status": {"$ne": "retired"}}` | Not equal |

### How It Works

1. When a criteria relationship arrives, Ivy stores the criteria definition
2. Evaluates all existing entities matching `_to_entity_type` + `_to_integration`
3. For each match, creates a `staged_relationship` (flows through normal merge)
4. When new entities arrive, checks against existing criteria definitions
5. Deletion strategies work the same as direct relationships

See `lotus.http` for mapping examples and `ivy.http` for message format details.

## Mock Data

The mock server includes:

### OKTA Mock
- 8 users across departments (Engineering, Marketing, Sales, HR, Finance, Product)
- 4 groups (Engineering, Marketing, Admins, All Employees)
- Group memberships

### MS Graph Mock
- 8 users (same people as OKTA, different source IDs)
- 6 devices (MacBooks, iPhones, Windows laptops)
- User-device ownership relationships
- User-manager relationships
- 4 groups with memberships

## Variables

Each `.http` file uses variables at the top:
```
@orchidUrl = http://localhost:3001
@tenantId = manual-test-tenant
@userId = manual-test-user
```

You can modify these as needed.

## Named Requests

Some requests are named (e.g., `# @name createOktaIntegration`). These capture the response so you can reference it in later requests:
```
@oktaIntegrationId = {{createOktaIntegration.response.body.id}}
```

## Tips

- Execute requests in order within each file (earlier requests create resources used by later ones)
- The mock server tokens expire after 1 hour - re-run the token request if you get 401s
- Use `Ctrl+Alt+R` (or `Cmd+Alt+R` on Mac) to quickly re-run the last request

