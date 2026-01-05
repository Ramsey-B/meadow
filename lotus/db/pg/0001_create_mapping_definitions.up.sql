CREATE TABLE IF NOT EXISTS mapping_definitions (
  id         TEXT NOT NULL,
  tenant_id  TEXT NOT NULL,
  user_id    TEXT NOT NULL,
  version    INTEGER NOT NULL DEFAULT 1,
  key        TEXT,
  name       TEXT NOT NULL,
  description TEXT,
  tags       TEXT[],
  is_active  BOOLEAN NOT NULL DEFAULT TRUE,
  is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  updated_at TIMESTAMP NOT NULL DEFAULT now(),
  source_fields JSONB NOT NULL,
  target_fields JSONB NOT NULL,
  steps JSONB NOT NULL,
  links JSONB NOT NULL,
  PRIMARY KEY (id, tenant_id, version)
);

CREATE INDEX IF NOT EXISTS idx_mapping_definitions_tenant_id ON mapping_definitions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_mapping_definitions_user_id ON mapping_definitions (user_id);
CREATE INDEX IF NOT EXISTS idx_mapping_definitions_is_active ON mapping_definitions (is_active);
CREATE INDEX IF NOT EXISTS idx_mapping_definitions_is_deleted ON mapping_definitions (is_deleted);
CREATE INDEX IF NOT EXISTS idx_mapping_definitions_key ON mapping_definitions (key);
CREATE INDEX IF NOT EXISTS idx_mapping_definitions_tags ON mapping_definitions (tags);

