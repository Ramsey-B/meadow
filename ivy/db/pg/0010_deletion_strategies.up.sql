-- Deletion Strategies - Configurable deletion policies for entities and relationships
-- Allows per-entity-type, per-source deletion rules

-- Drop old deletion tables from migration 0007 if they exist
DROP TABLE IF EXISTS pending_deletions;
DROP TABLE IF EXISTS execution_tracking;

-- Drop old deletion_strategies table (has different schema than new one)
DROP TABLE IF EXISTS deletion_strategies;

CREATE TABLE deletion_strategies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,

    -- Target: either entity_type or relationship_type (one must be set)
    entity_type TEXT,
    relationship_type TEXT,

    -- Source filter (NULL means applies to all sources for this entity/relationship type)
    integration TEXT,

    -- Strategy configuration
    -- Valid values: 'execution_based', 'explicit', 'staleness', 'retention', 'composite'
    -- Validation enforced in application code
    strategy_type TEXT NOT NULL,
    source_key TEXT,
    config JSONB NOT NULL DEFAULT '{}',

    -- Priority for conflict resolution (higher = takes precedence)
    -- When multiple strategies match, highest priority wins
    priority INT NOT NULL DEFAULT 0,

    -- Enabled flag for easy toggle
    enabled BOOLEAN NOT NULL DEFAULT true,

    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    -- Exactly one of entity_type or relationship_type must be set
    CONSTRAINT check_exactly_one_type CHECK (
        (entity_type IS NOT NULL AND relationship_type IS NULL) OR
        (entity_type IS NULL AND relationship_type IS NOT NULL)
    ),

    -- Unique constraint: one strategy per (tenant, entity_type/relationship_type, integration)
    UNIQUE(tenant_id, entity_type, relationship_type, integration, source_key)
);

CREATE INDEX IF NOT EXISTS idx_deletion_strategies_tenant_entity ON deletion_strategies(tenant_id, entity_type) WHERE entity_type IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_deletion_strategies_tenant_relationship ON deletion_strategies(tenant_id, relationship_type) WHERE relationship_type IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_deletion_strategies_source ON deletion_strategies(tenant_id, integration) WHERE integration IS NOT NULL;

-- Config JSON schema examples:
-- execution_based: {}  (no additional config needed)
-- explicit: {}  (no automatic deletion)
-- staleness: {"max_age_days": 30, "check_field": "updated_at"}
-- retention: {"retention_days": 90, "check_field": "created_at"}
-- composite: {
--   "strategies": [
--     {"type": "staleness", "max_age_days": 30},
--     {"type": "retention", "retention_days": 90}
--   ],
--   "operator": "AND"  -- or "OR"
-- }

COMMENT ON TABLE deletion_strategies IS 'Configurable deletion policies for entities and relationships by type and source';
COMMENT ON COLUMN deletion_strategies.integration IS 'NULL means this strategy applies to all sources for the entity/relationship type';
COMMENT ON COLUMN deletion_strategies.priority IS 'Higher priority wins when multiple strategies match (specific source strategies should have higher priority than defaults)';
