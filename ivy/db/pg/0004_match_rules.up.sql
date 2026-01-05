-- Match Rules - Defines how to match entities
CREATE TABLE IF NOT EXISTS match_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    priority INT NOT NULL DEFAULT 0,
    -- New match rule schema (current Ivy API)
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    conditions JSONB NOT NULL,
    score_weight DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    
    UNIQUE(tenant_id, entity_type, name)
);

CREATE INDEX idx_match_rules_tenant_type ON match_rules(tenant_id, entity_type);
CREATE INDEX idx_match_rules_priority ON match_rules(tenant_id, entity_type, priority DESC);
