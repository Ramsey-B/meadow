-- Staged Entities - Raw entities before merging
CREATE TABLE IF NOT EXISTS staged_entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    config_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    integration TEXT NOT NULL,
    source_key TEXT,
    source_execution_id TEXT,
    execution_id TEXT,
    last_seen_execution TEXT,
    data JSONB NOT NULL,
    fingerprint TEXT NOT NULL,
    previous_fingerprint TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    
    UNIQUE(tenant_id, entity_type, source_id, integration, source_key, config_id)
);

CREATE INDEX IF NOT EXISTS idx_staged_entities_tenant_type ON staged_entities(tenant_id, entity_type);
CREATE INDEX IF NOT EXISTS idx_staged_entities_fingerprint ON staged_entities(fingerprint);
CREATE INDEX IF NOT EXISTS idx_staged_entities_execution ON staged_entities(source_execution_id);
CREATE INDEX IF NOT EXISTS idx_staged_entities_data_gin ON staged_entities USING GIN (data);

-- Entity Match Index - Denormalized fields for fast matching
CREATE TABLE IF NOT EXISTS staged_entity_match_fields (
  tenant_id   text NOT NULL,
  entity_type text NOT NULL,
  staged_entity_id   uuid NOT NULL REFERENCES staged_entities(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW(),

  field       text NOT NULL,     -- e.g. 'first_name', 'phone'
  normalizer  text NOT NULL,     -- e.g. 'lower', 'phone_e164', 'raw'
  match_type  text NOT NULL,     -- 'exact' | 'fuzzy' | 'phonetic' | 'numeric' | 'date_range'

  value_text  text,
  value_num   numeric,
  value_ts    timestamptz,

  -- optional: precomputed phonetic token, or “blocking key”
  token       text,

  PRIMARY KEY (tenant_id, staged_entity_id, field, normalizer, match_type)
);

-- Exact
CREATE INDEX IF NOT EXISTS semf_exact
  ON staged_entity_match_fields (tenant_id, entity_type, field, normalizer, value_text)
  WHERE match_type = 'exact' AND value_text IS NOT NULL;

-- Numeric
CREATE INDEX IF NOT EXISTS semf_num
  ON staged_entity_match_fields (tenant_id, entity_type, field, normalizer, value_num)
  WHERE match_type = 'numeric' AND value_num IS NOT NULL;

-- Date/range (basic)
CREATE INDEX IF NOT EXISTS semf_ts
  ON staged_entity_match_fields (tenant_id, entity_type, field, normalizer, value_ts)
  WHERE match_type = 'date_range' AND value_ts IS NOT NULL;

-- Fuzzy via pg_trgm (GIN)
CREATE INDEX IF NOT EXISTS semf_fuzzy_trgm
  ON staged_entity_match_fields
  USING gin (value_text gin_trgm_ops)
  WHERE match_type = 'fuzzy' AND value_text IS NOT NULL;

-- Phonetic token exact lookup
CREATE INDEX IF NOT EXISTS semf_phonetic
  ON staged_entity_match_fields (tenant_id, entity_type, field, normalizer, token)
  WHERE match_type = 'phonetic' AND token IS NOT NULL;

-- Staged Relationships - Raw relationships before merging (direct and criteria-materialized)
-- Supports out-of-order message processing: stores source IDs immediately,
-- entity ID references are resolved when entities exist (may be NULL initially)
-- All relationships flow through the same merge pipeline to create merged_relationships
CREATE TABLE IF NOT EXISTS staged_relationships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    config_id TEXT NOT NULL,
    integration TEXT NOT NULL,  -- Source integration of this relationship
    source_key TEXT,
    
    relationship_type TEXT NOT NULL,
    
    -- From side (always by source_id)
    from_entity_type TEXT NOT NULL,
    from_source_id TEXT NOT NULL,
    from_integration TEXT NOT NULL,
    from_staged_entity_id UUID REFERENCES staged_entities(id) ON DELETE SET NULL,
    
    -- To side (by source_id)
    to_entity_type TEXT NOT NULL,
    to_source_id TEXT NOT NULL,
    to_integration TEXT NOT NULL,
    to_staged_entity_id UUID REFERENCES staged_entities(id) ON DELETE SET NULL,
    
    -- Optional: criteria that created this relationship (NULL for direct relationships)
    -- When set, this relationship was materialized from a criteria match
    criteria_id UUID,  -- FK added by 0012_criteria_relationships.up.sql
    
    -- Execution tracking for deletion strategies
    source_execution_id TEXT,
    execution_id TEXT,
    last_seen_execution TEXT,
    
    data JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    
    -- Unique on: tenant + relationship type + from (type, source_id, integration) + to (type, source_id, integration) + config
    UNIQUE(tenant_id, relationship_type, 
           from_entity_type, from_source_id, from_integration,
           to_entity_type, to_source_id, to_integration, 
           config_id)
);

-- Indexes for staged_relationships
CREATE INDEX IF NOT EXISTS idx_sr_tenant_type ON staged_relationships(tenant_id, relationship_type);
CREATE INDEX IF NOT EXISTS idx_sr_from_entity ON staged_relationships(from_staged_entity_id) WHERE from_staged_entity_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_sr_to_entity ON staged_relationships(to_staged_entity_id) WHERE to_staged_entity_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_sr_unresolved ON staged_relationships(tenant_id) 
    WHERE from_staged_entity_id IS NULL OR to_staged_entity_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_sr_from_source ON staged_relationships(tenant_id, from_entity_type, from_source_id, from_integration);
CREATE INDEX IF NOT EXISTS idx_sr_to_source ON staged_relationships(tenant_id, to_entity_type, to_source_id, to_integration);
CREATE INDEX IF NOT EXISTS idx_sr_execution ON staged_relationships(tenant_id, source_execution_id) WHERE source_execution_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_sr_last_seen ON staged_relationships(tenant_id, last_seen_execution) WHERE last_seen_execution IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_sr_criteria ON staged_relationships(criteria_id) WHERE criteria_id IS NOT NULL;

COMMENT ON TABLE staged_relationships IS 'Direct and criteria-materialized relationships (all flow through merge pipeline)';
COMMENT ON COLUMN staged_relationships.criteria_id IS 'NULL for direct relationships, set for criteria-materialized relationships';
