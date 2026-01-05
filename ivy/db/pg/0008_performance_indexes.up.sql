-- Execution-based deletion / plan scoping on relationships
CREATE INDEX IF NOT EXISTS idx_staged_relationships_tenant_plan_exec
  ON staged_relationships (tenant_id, source_execution_id, source_key)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_staged_relationships_tenant_plan
  ON staged_relationships (tenant_id, source_key)
  WHERE deleted_at IS NULL;


