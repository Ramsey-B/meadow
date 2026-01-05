# Source Key Design

## Problem Statement

A single source integration (e.g., "orchid") can have multiple plans that provide different aspects of the same entity. Without proper differentiation, these plans would conflict and overwrite each other's data instead of contributing complementary information.

### Example Scenario

**Orchid Integration** with two plans:
- **contacts-plan**: Provides person entities with contact information (email, phone)
- **employment-plan**: Provides person entities with employment data (company, role)

Both plans:
- Use `source_type = "orchid"`
- Reference the same person: `source_id = "person-123"`
- But provide **different data subsets**

**Old Constraint**: `UNIQUE (tenant_id, entity_type, source_id, source_type)`
- ❌ Second plan **overwrites** first plan's data
- ❌ Cannot have multiple plans contributing to same entity
- ❌ Loss of complementary data

**New Constraint**: `UNIQUE (tenant_id, entity_type, source_id, source_type, source_key)`
- ✅ Each plan has unique `source_key`
- ✅ Multiple plans can contribute to same entity
- ✅ Data is **deep merged** across plans

---

## Solution: `source_key` Field

### What is `source_key`?

`source_key` identifies the **specific plan or integration** within a source that provides the data.

### Structure

```
source_type  = "orchid"           (the integration type)
source_key   = "contacts-plan"    (specific plan within orchid)
source_id    = "person-123"       (the entity identifier)
```

### Unique Constraint

**Staged Entities**:
```sql
UNIQUE (tenant_id, entity_type, source_id, source_type, source_key)
```

**Staged Relationships**:
```sql
UNIQUE (tenant_id, relationship_type, from_entity_type, from_source_id,
        to_entity_type, to_source_id, source_key)
```

---

## Data Flow Example

### Setup

Two Orchid plans for the same person:

**Plan: `contacts-plan`**
```json
{
  "source_type": "orchid",
  "source_key": "contacts-plan",
  "source_id": "person-123",
  "entity_type": "person",
  "data": {
    "email": "john@example.com",
    "phone": "+1-555-1234",
    "contact": {
      "preferred_method": "email"
    }
  }
}
```

**Plan: `employment-plan`**
```json
{
  "source_type": "orchid",
  "source_key": "employment-plan",
  "source_id": "person-123",
  "entity_type": "person",
  "data": {
    "company": "Acme Corp",
    "role": "Engineer",
    "employment": {
      "start_date": "2020-01-01",
      "department": "Engineering"
    }
  }
}
```

### Result: Two Staged Entities

These create **two separate rows** in `staged_entities`:

| id | source_type | source_key | source_id | data (summary) |
|----|-------------|------------|-----------|----------------|
| 1  | orchid | contacts-plan | person-123 | email, phone, contact |
| 2  | orchid | employment-plan | person-123 | company, role, employment |

### Merging Process

During entity resolution/merging:
1. **Match** both staged entities to the same person (via `source_id = "person-123"`)
2. **Merge** both data objects using deep merge
3. **Create** merged entity with combined data:

```json
{
  "merged_entity_id": "merged-person-123",
  "data": {
    "email": "john@example.com",
    "phone": "+1-555-1234",
    "company": "Acme Corp",
    "role": "Engineer",
    "contact": {
      "preferred_method": "email"
    },
    "employment": {
      "start_date": "2020-01-01",
      "department": "Engineering"
    }
  }
}
```

---

## Deletion Strategies with `source_key`

Deletion strategies can now be scoped to specific plans within a source.

### Hierarchy (most specific to least specific)

1. **Plan-specific**: `source_type = "orchid"`, `source_key = "contacts-plan"`, `entity_type = "person"`
2. **Source-specific**: `source_type = "orchid"`, `source_key = NULL`, `entity_type = "person"`
3. **Entity-level**: `source_type = NULL`, `source_key = NULL`, `entity_type = "person"`

### Example: Different Deletion Strategies per Plan

```sql
-- contacts-plan: execution-based deletion (full sync)
INSERT INTO deletion_strategies (
  tenant_id, entity_type, source_type, source_key, strategy_type, priority
) VALUES (
  'tenant-1', 'person', 'orchid', 'contacts-plan', 'execution_based', 100
);

-- employment-plan: staleness-based deletion (event stream)
INSERT INTO deletion_strategies (
  tenant_id, entity_type, source_type, source_key, strategy_type, priority
) VALUES (
  'tenant-1', 'person', 'orchid', 'employment-plan', 'staleness', 90
);

-- All other orchid plans: explicit deletion only
INSERT INTO deletion_strategies (
  tenant_id, entity_type, source_type, source_key, strategy_type, priority
) VALUES (
  'tenant-1', 'person', 'orchid', NULL, 'explicit', 50
);
```

### Strategy Resolution

When determining deletion strategy for an entity:
1. Look for `(source_type, source_key)` match (highest priority)
2. Fall back to `(source_type, NULL)` match
3. Fall back to `(NULL, NULL)` match (global default)

---

## Matching and Merging with `source_key`

### Match Rules

Match rules can be scoped to specific plans:

```sql
-- Match rule applies only to contacts-plan data
{
  "tenant_id": "tenant-1",
  "entity_type": "person",
  "source_type": "orchid",
  "source_key": "contacts-plan",  -- Optional: restrict to this plan
  "rules": [...]
}
```

### Merge Strategies

Merge strategies can prioritize data from specific plans:

```json
{
  "field_strategies": [
    {
      "field": "email",
      "strategy": "source_priority",
      "source_priorities": [
        {"source_type": "orchid", "source_key": "contacts-plan", "priority": 100},
        {"source_type": "orchid", "source_key": "employment-plan", "priority": 50},
        {"source_type": "manual", "source_key": "default", "priority": 150}
      ]
    }
  ]
}
```

**Result**: When merging `email` field, prefer:
1. manual > contacts-plan > employment-plan

---

## Migration Strategy

### Phase 1: Add `source_key` Column (Migration 0013)

```sql
-- Add column (nullable)
ALTER TABLE staged_entities ADD COLUMN source_key TEXT;

-- Backfill from source_plan_id
UPDATE staged_entities
SET source_key = COALESCE(source_plan_id, 'default')
WHERE source_key IS NULL;

-- Make NOT NULL
ALTER TABLE staged_entities ALTER COLUMN source_key SET NOT NULL;

-- Update unique constraint
ALTER TABLE staged_entities
DROP CONSTRAINT staged_entities_tenant_id_entity_type_source_id_source_type_key;

ALTER TABLE staged_entities
ADD CONSTRAINT staged_entities_unique_with_source_key
UNIQUE (tenant_id, entity_type, source_id, source_type, source_key);
```

### Phase 2: Update Application Code

1. **Models**: Add `source_key` field ✅
2. **Repositories**: Include `source_key` in queries ⏳
3. **Processor**: Extract `source_key` from messages ⏳
4. **Deletion**: Filter by `source_key` when applicable ⏳

### Backward Compatibility

- Existing data: Backfilled with `source_plan_id` or `"default"`
- New data: Must provide explicit `source_key`

---

## API Changes

### Create Entity Request

**Before**:
```json
{
  "entity_type": "person",
  "source_id": "person-123",
  "source_type": "orchid",
  "data": {...}
}
```

**After**:
```json
{
  "entity_type": "person",
  "source_id": "person-123",
  "source_type": "orchid",
  "source_key": "contacts-plan",  // NEW: Required
  "data": {...}
}
```

### Query Entities

```http
GET /api/v1/entities?source_type=orchid&source_key=contacts-plan
```

Returns only entities from the `contacts-plan`.

---

## Benefits

### ✅ Multiple Plans per Source
- Same source can have many plans contributing to same entity
- No data loss or overwrites

### ✅ Plan-Specific Configuration
- Different deletion strategies per plan
- Different matching rules per plan
- Different merge priorities per plan

### ✅ Better Observability
- Track which plan provided which data
- Monitor per-plan metrics
- Debug data issues by plan

### ✅ Flexible Architecture
- Add new plans without affecting existing ones
- Deprecate plans independently
- A/B test different mapping strategies

---

## Common Patterns

### Pattern 1: Event Stream + Snapshot

**Event Stream Plan** (`source_key = "events"`):
- Real-time updates
- Staleness-based deletion (delete if no event in 30 days)

**Snapshot Plan** (`source_key = "snapshot"`):
- Daily full sync
- Execution-based deletion (delete if not in latest sync)

### Pattern 2: Multi-System Integration

**CRM Plan** (`source_key = "crm"`):
- Contact information
- Execution-based deletion

**HRIS Plan** (`source_key = "hris"`):
- Employment information
- Explicit deletion only (authoritative)

### Pattern 3: Incremental Enrichment

**Base Plan** (`source_key = "base"`):
- Core entity data
- Execution-based deletion

**Enrichment Plan** (`source_key = "enrichment"`):
- Additional attributes
- Staleness-based deletion

---

## FAQ

### Q: What if I only have one plan per source?

Use a simple source_key like `"default"` or the integration name.

### Q: Can different plans have different source_ids for the same entity?

Yes! The matching engine will identify that they represent the same entity and merge them.

### Q: How does this affect performance?

- Minimal: One additional column in unique constraint
- Indexes added for common query patterns
- Deep merge already optimized in PostgreSQL

### Q: What about relationships?

Relationships also have `source_key` - same concept applies.

---

## Implementation Checklist

- [x] Database migration (0013_add_source_key.up.sql)
- [x] Model updates (StagedEntity, StagedRelationship, DeletionStrategy)
- [ ] Repository updates
  - [ ] stagedentity.Repository
  - [ ] stagedrelationship.Repository
  - [ ] deletionstrategy.Repository
- [ ] Processor updates
  - [ ] Extract source_key from Lotus messages
  - [ ] Pass source_key to repository
- [ ] Deletion strategy resolution
  - [ ] Support source_key in lookup
  - [ ] Test hierarchy (plan > source > global)
- [ ] Tests
  - [ ] Multi-plan scenario tests
  - [ ] Deletion strategy scoping tests
  - [ ] Deep merge across plans tests
- [ ] Documentation
  - [x] Design document
  - [ ] API documentation updates
  - [ ] Integration guide
