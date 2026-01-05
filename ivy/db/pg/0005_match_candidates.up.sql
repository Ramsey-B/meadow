-- Match candidates table for storing potential matches
CREATE TABLE IF NOT EXISTS match_candidates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    source_entity_id UUID NOT NULL,
    candidate_entity_id UUID NOT NULL,
    match_score DECIMAL(5,4) NOT NULL DEFAULT 0,
    match_details JSONB DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending',
    matched_rules JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    resolved_by TEXT,
    
    -- Unique constraint to prevent duplicate candidates
    CONSTRAINT match_candidates_entity_pair_unique UNIQUE (tenant_id, source_entity_id, candidate_entity_id)
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_match_candidates_tenant_status 
    ON match_candidates(tenant_id, status);

CREATE INDEX IF NOT EXISTS idx_match_candidates_source_entity 
    ON match_candidates(tenant_id, source_entity_id);

CREATE INDEX IF NOT EXISTS idx_match_candidates_candidate_entity 
    ON match_candidates(tenant_id, candidate_entity_id);

CREATE INDEX IF NOT EXISTS idx_match_candidates_score 
    ON match_candidates(tenant_id, match_score DESC) 
    WHERE status = 'pending';

-- Ensure entity pairs are unique regardless of order (A-B = B-A)
CREATE UNIQUE INDEX IF NOT EXISTS idx_match_candidates_ordered_pair 
    ON match_candidates(tenant_id, LEAST(source_entity_id, candidate_entity_id), GREATEST(source_entity_id, candidate_entity_id));

