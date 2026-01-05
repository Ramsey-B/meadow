-- Merged Relationships - Canonical "golden" relationships between merged entities
--
-- This table is the Postgres source of truth for relationships after endpoint resolution.
-- It enables:
-- - fast relationship property merges without round-tripping to Memgraph
-- - CDC/Kafka-connector based graph sync (optional future direction)
-- - deletion-by-execution and explicit deletes at the golden-edge level

CREATE TABLE IF NOT EXISTS merged_relationships (
    id UUID PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    relationship_type TEXT NOT NULL,
    from_entity_type TEXT NOT NULL,
    from_merged_entity_id UUID NOT NULL REFERENCES merged_entities(id),
    to_entity_type TEXT NOT NULL,
    to_merged_entity_id UUID NOT NULL REFERENCES merged_entities(id),
    data JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    UNIQUE(tenant_id, relationship_type, from_merged_entity_id, to_merged_entity_id)
);

CREATE INDEX IF NOT EXISTS idx_merged_relationships_tenant_type
  ON merged_relationships(tenant_id, relationship_type)
  WHERE deleted_at IS NULL;


