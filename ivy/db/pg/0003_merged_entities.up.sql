-- Merged Entities - Canonical entities after merge
CREATE TABLE IF NOT EXISTS merged_entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    data JSONB NOT NULL,
    source_count INT NOT NULL DEFAULT 1,
    primary_source_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX idx_merged_entities_tenant_type ON merged_entities(tenant_id, entity_type);
CREATE INDEX idx_merged_entities_tenant_type_active ON merged_entities(tenant_id, entity_type) WHERE deleted_at IS NULL;

-- Entity Clusters - Links staged entities to merged entities
CREATE TABLE IF NOT EXISTS entity_clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    merged_entity_id UUID NOT NULL REFERENCES merged_entities(id),
    staged_entity_id UUID NOT NULL REFERENCES staged_entities(id),
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    removed_at TIMESTAMPTZ,

    UNIQUE(tenant_id, merged_entity_id, staged_entity_id)
);

CREATE INDEX idx_entity_clusters_merged ON entity_clusters(merged_entity_id);
CREATE INDEX idx_entity_clusters_staged ON entity_clusters(staged_entity_id);
CREATE INDEX idx_entity_clusters_tenant_merged ON entity_clusters(tenant_id, merged_entity_id);
CREATE UNIQUE INDEX entity_clusters_unique_active_staged ON entity_clusters(tenant_id, staged_entity_id) WHERE removed_at IS NULL;
