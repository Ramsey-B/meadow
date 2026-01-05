-- Rollback core Orchid schema

DROP TRIGGER IF EXISTS update_plan_statistics_updated_at ON plan_statistics;
DROP TRIGGER IF EXISTS update_plan_contexts_updated_at ON plan_contexts;
DROP TRIGGER IF EXISTS update_plan_executions_updated_at ON plan_executions;
DROP TRIGGER IF EXISTS update_plans_updated_at ON plans;
DROP TRIGGER IF EXISTS update_auth_flows_updated_at ON auth_flows;
DROP TRIGGER IF EXISTS update_configs_updated_at ON configs;
DROP TRIGGER IF EXISTS update_integrations_updated_at ON integrations;

DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tenant-scoped indexes
DROP INDEX IF EXISTS idx_plan_statistics_tenant_id_plan_config;
DROP INDEX IF EXISTS idx_plan_statistics_tenant_id;
DROP INDEX IF EXISTS idx_plan_contexts_tenant_id_plan_config;
DROP INDEX IF EXISTS idx_plan_contexts_tenant_id;
DROP INDEX IF EXISTS idx_plan_executions_tenant_id_started_at;
DROP INDEX IF EXISTS idx_plan_executions_tenant_id_status;
DROP INDEX IF EXISTS idx_plan_executions_tenant_id_plan_key;
DROP INDEX IF EXISTS idx_plan_executions_tenant_id;
DROP INDEX IF EXISTS idx_plans_tenant_id_enabled;
DROP INDEX IF EXISTS idx_plans_tenant_id_integration_id;
DROP INDEX IF EXISTS idx_plans_tenant_id;
DROP INDEX IF EXISTS idx_auth_flows_tenant_id_integration_id;
DROP INDEX IF EXISTS idx_auth_flows_tenant_id;
DROP INDEX IF EXISTS idx_configs_tenant_id_integration_id;
DROP INDEX IF EXISTS idx_configs_tenant_id_enabled;
DROP INDEX IF EXISTS idx_configs_tenant_id;
DROP INDEX IF EXISTS idx_integrations_tenant_id;

-- Drop legacy indexes
DROP INDEX IF EXISTS idx_auth_flows_integration_id;
DROP INDEX IF EXISTS idx_plan_statistics_plan_config;
DROP INDEX IF EXISTS idx_plan_contexts_plan_config;
DROP INDEX IF EXISTS idx_plan_executions_parent_id;
DROP INDEX IF EXISTS idx_plan_executions_started_at;
DROP INDEX IF EXISTS idx_plan_executions_status;
DROP INDEX IF EXISTS idx_plan_executions_config_id;
DROP INDEX IF EXISTS idx_plan_executions_plan_key;
DROP INDEX IF EXISTS idx_configs_enabled;
DROP INDEX IF EXISTS idx_configs_integration_id;
DROP INDEX IF EXISTS idx_plans_enabled;
DROP INDEX IF EXISTS idx_plans_integration_id;

DROP TABLE IF EXISTS plan_statistics;
DROP TABLE IF EXISTS plan_contexts;
DROP TABLE IF EXISTS plan_executions;
DROP TABLE IF EXISTS plans;
DROP TABLE IF EXISTS auth_flows;
DROP TABLE IF EXISTS configs;
DROP TABLE IF EXISTS integrations;








