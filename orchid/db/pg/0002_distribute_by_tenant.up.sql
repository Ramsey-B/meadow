-- Distribute tables by tenant_id for horizontal scaling
-- All tables are co-located by using the same distribution column (tenant_id)
SELECT create_distributed_table('integrations', 'tenant_id');
SELECT create_distributed_table('configs', 'tenant_id', colocate_with => 'integrations');
SELECT create_distributed_table('plans', 'tenant_id', colocate_with => 'integrations');
SELECT create_distributed_table('auth_flows', 'tenant_id', colocate_with => 'integrations');
SELECT create_distributed_table('plan_executions', 'tenant_id', colocate_with => 'integrations');
SELECT create_distributed_table('plan_contexts', 'tenant_id', colocate_with => 'integrations');
SELECT create_distributed_table('plan_statistics', 'tenant_id', colocate_with => 'integrations');


