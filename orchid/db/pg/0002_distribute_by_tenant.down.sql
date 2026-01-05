-- Undistribute tables
SELECT undistribute_table('plan_statistics');
SELECT undistribute_table('plan_contexts');
SELECT undistribute_table('plan_executions');
SELECT undistribute_table('auth_flows');
SELECT undistribute_table('plans');
SELECT undistribute_table('configs');
SELECT undistribute_table('integrations');

