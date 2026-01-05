-- Criteria-based Relationship Definitions
-- These are "subscriptions" that match multiple target entities based on criteria
-- When a match is found, a staged_relationship is created which flows through the normal merge pipeline

CREATE TABLE IF NOT EXISTS staged_relationship_criteria (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    config_id TEXT NOT NULL,
    integration TEXT NOT NULL,  -- Source integration of this criteria definition
    source_key TEXT,
    
    relationship_type TEXT NOT NULL,
    
    -- From side (always by source_id)
    from_entity_type TEXT NOT NULL,
    from_source_id TEXT NOT NULL,
    from_integration TEXT NOT NULL,
    from_staged_entity_id UUID REFERENCES staged_entities(id) ON DELETE SET NULL,
    
    -- To side (criteria-based, ALWAYS scoped by type + integration)
    to_entity_type TEXT NOT NULL,
    to_integration TEXT NOT NULL,  -- REQUIRED - limits scope of criteria evaluation
    criteria JSONB NOT NULL,       -- {"field": value, "field2": {"$contains": value}, ...}
    criteria_hash TEXT NOT NULL,   -- SHA256 of canonical JSON for dedup
    
    -- Execution tracking for deletion strategies
    source_execution_id TEXT,
    execution_id TEXT,
    last_seen_execution TEXT,
    
    data JSONB,  -- Additional relationship properties applied to all matches
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    
    -- Unique: one criteria per from-entity + to-type + to-integration + criteria
    UNIQUE(tenant_id, relationship_type, 
           from_entity_type, from_source_id, from_integration,
           to_entity_type, to_integration, criteria_hash, 
           config_id)
);

-- Indexes for criteria lookup
CREATE INDEX IF NOT EXISTS idx_src_tenant_type ON staged_relationship_criteria(tenant_id, relationship_type);
CREATE INDEX IF NOT EXISTS idx_src_from_entity ON staged_relationship_criteria(from_staged_entity_id) 
    WHERE from_staged_entity_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_src_from_source ON staged_relationship_criteria(tenant_id, from_entity_type, from_source_id, from_integration);

-- Critical index for backfill: find criteria by target entity_type + integration
CREATE INDEX IF NOT EXISTS idx_src_target ON staged_relationship_criteria(tenant_id, to_entity_type, to_integration) 
    WHERE deleted_at IS NULL;

-- Execution tracking indexes for deletion strategies
CREATE INDEX IF NOT EXISTS idx_src_execution ON staged_relationship_criteria(tenant_id, source_execution_id) 
    WHERE source_execution_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_src_last_seen ON staged_relationship_criteria(tenant_id, last_seen_execution) 
    WHERE last_seen_execution IS NOT NULL;

-- Materialized matches from criteria evaluation
-- Each row represents a match between a criteria definition and a target entity
-- When a match is created, a staged_relationship is also created (referenced by staged_relationship_id)
CREATE TABLE IF NOT EXISTS staged_relationship_criteria_matches (
    criteria_id UUID NOT NULL REFERENCES staged_relationship_criteria(id) ON DELETE CASCADE,
    to_staged_entity_id UUID NOT NULL REFERENCES staged_entities(id) ON DELETE CASCADE,
    
    -- The staged_relationship created for this match (flows through normal merge pipeline)
    staged_relationship_id UUID NOT NULL REFERENCES staged_relationships(id) ON DELETE CASCADE,
    
    -- Denormalized for efficient queries
    tenant_id TEXT NOT NULL,
    relationship_type TEXT NOT NULL,
    
    -- Track when this match was created/verified
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_verified_at TIMESTAMPTZ DEFAULT NOW(),
    
    PRIMARY KEY (criteria_id, to_staged_entity_id)
);

-- Index for finding matches by entity (for cleanup when entity changes/deleted)
CREATE INDEX IF NOT EXISTS idx_srcm_entity ON staged_relationship_criteria_matches(to_staged_entity_id);
CREATE INDEX IF NOT EXISTS idx_srcm_tenant_type ON staged_relationship_criteria_matches(tenant_id, relationship_type);
CREATE INDEX IF NOT EXISTS idx_srcm_staged_rel ON staged_relationship_criteria_matches(staged_relationship_id);

-- Add FK from staged_relationships.criteria_id to criteria table
ALTER TABLE staged_relationships 
    ADD CONSTRAINT fk_staged_rel_criteria 
    FOREIGN KEY (criteria_id) REFERENCES staged_relationship_criteria(id) ON DELETE SET NULL;

-- Comments
COMMENT ON TABLE staged_relationship_criteria IS 'Criteria-based relationship definitions (subscriptions)';
COMMENT ON TABLE staged_relationship_criteria_matches IS 'Materialized matches - each creates a staged_relationship';
COMMENT ON COLUMN staged_relationship_criteria_matches.staged_relationship_id IS 'The staged_relationship created for this match';
COMMENT ON COLUMN staged_relationship_criteria.to_integration IS 'REQUIRED - all criteria matches are scoped by entity_type + integration';
COMMENT ON COLUMN staged_relationship_criteria.criteria IS 'JSON criteria: {"field": "value"} for equality, {"field": {"$contains": "value"}} for operators';
COMMENT ON COLUMN staged_relationship_criteria.criteria_hash IS 'SHA256 hash of canonicalized criteria JSON for deduplication';
