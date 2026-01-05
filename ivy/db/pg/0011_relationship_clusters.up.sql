-- Relationship Clusters - Track which staged relationships contribute to merged relationships
-- Similar to entity_clusters but for relationships

CREATE TABLE IF NOT EXISTS relationship_clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    merged_relationship_id UUID NOT NULL REFERENCES merged_relationships(id) ON DELETE CASCADE,
    staged_relationship_id UUID NOT NULL REFERENCES staged_relationships(id) ON DELETE CASCADE,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    added_at TIMESTAMPTZ DEFAULT NOW(),
    removed_at TIMESTAMPTZ,

    UNIQUE(tenant_id, staged_relationship_id)
);

CREATE INDEX IF NOT EXISTS idx_relationship_clusters_merged
    ON relationship_clusters(tenant_id, merged_relationship_id, removed_at);

CREATE INDEX IF NOT EXISTS idx_relationship_clusters_staged
    ON relationship_clusters(tenant_id, staged_relationship_id, removed_at);

CREATE INDEX IF NOT EXISTS idx_relationship_clusters_active
    ON relationship_clusters(tenant_id, merged_relationship_id)
    WHERE removed_at IS NULL;

COMMENT ON TABLE relationship_clusters IS 'Tracks which staged relationships contribute to each merged relationship';
COMMENT ON COLUMN relationship_clusters.is_primary IS 'The primary source is used for property priority in merging';
COMMENT ON COLUMN relationship_clusters.removed_at IS 'NULL means active member, non-NULL means removed from cluster';
