-- Add foreign keys after distribution (Citus supports FK between co-located tables)
-- For Citus, foreign keys must have the partition column (tenant_id) in the same ordinal position
-- All tables are co-located by tenant_id, so foreign keys are supported
-- Using DO block to execute multiple statements as a single command
DO $$
BEGIN
    EXECUTE 'ALTER TABLE configs ADD CONSTRAINT configs_integration_id_fkey FOREIGN KEY (tenant_id, integration_id) REFERENCES integrations(tenant_id, id) ON DELETE CASCADE';
    EXECUTE 'ALTER TABLE auth_flows ADD CONSTRAINT auth_flows_integration_id_fkey FOREIGN KEY (tenant_id, integration_id) REFERENCES integrations(tenant_id, id) ON DELETE CASCADE';
    EXECUTE 'ALTER TABLE plans ADD CONSTRAINT plans_integration_id_fkey FOREIGN KEY (tenant_id, integration_id) REFERENCES integrations(tenant_id, id) ON DELETE CASCADE';
    EXECUTE 'ALTER TABLE plan_executions ADD CONSTRAINT plan_executions_plan_key_fkey FOREIGN KEY (tenant_id, plan_key) REFERENCES plans(tenant_id, key) ON DELETE CASCADE';
    EXECUTE 'ALTER TABLE plan_executions ADD CONSTRAINT plan_executions_config_id_fkey FOREIGN KEY (tenant_id, config_id) REFERENCES configs(tenant_id, id) ON DELETE CASCADE';
    EXECUTE 'ALTER TABLE plan_executions ADD CONSTRAINT plan_executions_parent_execution_id_fkey FOREIGN KEY (tenant_id, parent_execution_id) REFERENCES plan_executions(tenant_id, id) ON DELETE CASCADE';
    EXECUTE 'ALTER TABLE plan_contexts ADD CONSTRAINT plan_contexts_plan_key_fkey FOREIGN KEY (tenant_id, plan_key) REFERENCES plans(tenant_id, key) ON DELETE CASCADE';
    EXECUTE 'ALTER TABLE plan_contexts ADD CONSTRAINT plan_contexts_config_id_fkey FOREIGN KEY (tenant_id, config_id) REFERENCES configs(tenant_id, id) ON DELETE CASCADE';
    EXECUTE 'ALTER TABLE plan_statistics ADD CONSTRAINT plan_statistics_plan_key_fkey FOREIGN KEY (tenant_id, plan_key) REFERENCES plans(tenant_id, key) ON DELETE CASCADE';
    EXECUTE 'ALTER TABLE plan_statistics ADD CONSTRAINT plan_statistics_config_id_fkey FOREIGN KEY (tenant_id, config_id) REFERENCES configs(tenant_id, id) ON DELETE CASCADE';
END $$;

