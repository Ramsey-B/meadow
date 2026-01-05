CREATE TABLE IF NOT EXISTS bindings (
  id           TEXT NOT NULL,
  tenant_id    TEXT NOT NULL,
  name         TEXT NOT NULL,
  description  TEXT,
  mapping_id   TEXT NOT NULL,
  is_enabled   BOOLEAN NOT NULL DEFAULT TRUE,
  output_topic TEXT,
  filter       JSONB NOT NULL DEFAULT '{}',
  created_at   TIMESTAMP NOT NULL DEFAULT now(),
  updated_at   TIMESTAMP NOT NULL DEFAULT now(),
  PRIMARY KEY (id, tenant_id)
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_bindings_tenant_id ON bindings (tenant_id);
CREATE INDEX IF NOT EXISTS idx_bindings_mapping_id ON bindings (mapping_id);
CREATE INDEX IF NOT EXISTS idx_bindings_is_enabled ON bindings (is_enabled);
CREATE INDEX IF NOT EXISTS idx_bindings_tenant_enabled ON bindings (tenant_id, is_enabled);

-- Comment on table
COMMENT ON TABLE bindings IS 'Bindings connect incoming Kafka messages to mapping definitions';
COMMENT ON COLUMN bindings.filter IS 'JSON filter criteria: integration, plan_ids, status_codes, step_path_prefix, min/max_status_code';

