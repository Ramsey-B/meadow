-- Drop foreign keys
ALTER TABLE plan_statistics DROP CONSTRAINT IF EXISTS plan_statistics_config_id_fkey;
ALTER TABLE plan_statistics DROP CONSTRAINT IF EXISTS plan_statistics_plan_key_fkey;
ALTER TABLE plan_contexts DROP CONSTRAINT IF EXISTS plan_contexts_config_id_fkey;
ALTER TABLE plan_contexts DROP CONSTRAINT IF EXISTS plan_contexts_plan_key_fkey;
ALTER TABLE plan_executions DROP CONSTRAINT IF EXISTS plan_executions_parent_execution_id_fkey;
ALTER TABLE plan_executions DROP CONSTRAINT IF EXISTS plan_executions_config_id_fkey;
ALTER TABLE plan_executions DROP CONSTRAINT IF EXISTS plan_executions_plan_key_fkey;
ALTER TABLE plans DROP CONSTRAINT IF EXISTS plans_integration_id_fkey;
ALTER TABLE auth_flows DROP CONSTRAINT IF EXISTS auth_flows_integration_id_fkey;
ALTER TABLE configs DROP CONSTRAINT IF EXISTS configs_integration_id_fkey;


