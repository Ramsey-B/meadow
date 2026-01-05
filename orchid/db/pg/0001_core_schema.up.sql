-- Core Orchid schema
-- This migration creates the foundational tables for plans, configs, integrations, and executions
-- Includes tenant_id for multi-tenancy support from the start

-- Integrations table
-- Represents a third-party API integration
CREATE TABLE IF NOT EXISTS integrations (
    id UUID DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    -- Config schema is owned by the integration (no separate config_schema_id)
    -- Shape is app-defined, typically: { "name": "...", "schema": { ...JSON Schema... } }
    config_schema JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE(tenant_id, name)
);

-- Configs table
-- Actual configuration instances with values
CREATE TABLE IF NOT EXISTS configs (
    id UUID DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    integration_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    values JSONB NOT NULL, -- Encrypted/secure config values
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE(tenant_id, integration_id, name)
);

-- Auth flows table
-- Defines authentication flows for integrations
CREATE TABLE IF NOT EXISTS auth_flows (
    id UUID DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    integration_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    plan_definition JSONB NOT NULL, -- Plan definition for auth flow
    token_path VARCHAR(255) NOT NULL, -- JMESPath to extract token
    header_name VARCHAR(255) NOT NULL, -- Header name for token
    header_format VARCHAR(255), -- Format string (e.g., "Bearer {token}")
    refresh_path VARCHAR(255), -- JMESPath for refresh token
    expires_in_path VARCHAR(255), -- JMESPath for expiration
    ttl_seconds INTEGER, -- Cache TTL
    skew_seconds INTEGER DEFAULT 60, -- Refresh before expiry
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE(tenant_id, integration_id, name)
);

-- Plans table
-- Defines API polling plans
CREATE TABLE IF NOT EXISTS plans (
    key TEXT NOT NULL,
    tenant_id UUID NOT NULL,
    integration_id UUID NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    plan_definition JSONB NOT NULL, -- Full plan structure (URL, method, headers, params, sub-steps, etc.)
    enabled BOOLEAN NOT NULL DEFAULT false,
    wait_seconds INTEGER, -- Wait time between executions (NULL = use default)
    repeat_count INTEGER, -- Number of times to execute (NULL = infinite)
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, key)
);

-- Plan executions table
-- Tracks individual plan/step executions
CREATE TABLE IF NOT EXISTS plan_executions (
    id UUID DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    plan_key TEXT NOT NULL,
    config_id UUID NOT NULL,
    parent_execution_id UUID,
    parent_execution_tenant_id UUID,
    status VARCHAR(50) NOT NULL, -- pending, running, success, failed, aborted
    step_path TEXT, -- Path to step in plan (e.g., "main", "main.sub_steps[0]")
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    error_type VARCHAR(100), -- transient, permanent, rate_limit, etc.
    retry_count INTEGER NOT NULL DEFAULT 0,
    request_url TEXT,
    request_method VARCHAR(10),
    response_status_code INTEGER,
    response_size_bytes BIGINT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id)
);

-- Plan contexts table
-- Stores context data for plans/configs
CREATE TABLE IF NOT EXISTS plan_contexts (
    id UUID DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    plan_key TEXT NOT NULL,
    config_id UUID NOT NULL,
    context_data JSONB NOT NULL, -- Context fields (last_user_id, last_page_token, etc.)
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE(tenant_id, plan_key, config_id)
);

-- Plan statistics table
-- Aggregated statistics for plans
CREATE TABLE IF NOT EXISTS plan_statistics (
    id UUID DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    plan_key TEXT NOT NULL,
    config_id UUID NOT NULL,
    last_execution_at TIMESTAMP WITH TIME ZONE,
    last_success_at TIMESTAMP WITH TIME ZONE,
    last_failure_at TIMESTAMP WITH TIME ZONE,
    total_executions BIGINT NOT NULL DEFAULT 0,
    total_successes BIGINT NOT NULL DEFAULT 0,
    total_failures BIGINT NOT NULL DEFAULT 0,
    total_api_calls BIGINT NOT NULL DEFAULT 0,
    average_execution_time_ms INTEGER,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE(tenant_id, plan_key, config_id)
);

-- Indexes for common queries (including tenant-scoped indexes)
-- Tenant-scoped indexes
CREATE INDEX IF NOT EXISTS idx_integrations_tenant_id ON integrations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_configs_tenant_id ON configs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_configs_tenant_id_enabled ON configs(tenant_id, enabled);
CREATE INDEX IF NOT EXISTS idx_configs_tenant_id_integration_id ON configs(tenant_id, integration_id);
CREATE INDEX IF NOT EXISTS idx_auth_flows_tenant_id ON auth_flows(tenant_id);
CREATE INDEX IF NOT EXISTS idx_auth_flows_tenant_id_integration_id ON auth_flows(tenant_id, integration_id);
CREATE INDEX IF NOT EXISTS idx_plans_tenant_id ON plans(tenant_id);
CREATE INDEX IF NOT EXISTS idx_plans_tenant_id_integration_id ON plans(tenant_id, integration_id);
CREATE INDEX IF NOT EXISTS idx_plans_tenant_id_enabled ON plans(tenant_id, enabled);
CREATE INDEX IF NOT EXISTS idx_plan_executions_tenant_id ON plan_executions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_plan_executions_tenant_id_plan_key ON plan_executions(tenant_id, plan_key);
CREATE INDEX IF NOT EXISTS idx_plan_executions_tenant_id_status ON plan_executions(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_plan_executions_tenant_id_started_at ON plan_executions(tenant_id, started_at);
CREATE INDEX IF NOT EXISTS idx_plan_contexts_tenant_id ON plan_contexts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_plan_contexts_tenant_id_plan_config ON plan_contexts(tenant_id, plan_key, config_id);
CREATE INDEX IF NOT EXISTS idx_plan_statistics_tenant_id ON plan_statistics(tenant_id);
CREATE INDEX IF NOT EXISTS idx_plan_statistics_tenant_id_plan_config ON plan_statistics(tenant_id, plan_key, config_id);

-- Legacy indexes (still useful for queries that don't filter by tenant)
CREATE INDEX IF NOT EXISTS idx_plans_integration_id ON plans(integration_id);
CREATE INDEX IF NOT EXISTS idx_plans_enabled ON plans(enabled);
CREATE INDEX IF NOT EXISTS idx_configs_integration_id ON configs(integration_id);
CREATE INDEX IF NOT EXISTS idx_configs_enabled ON configs(enabled);
CREATE INDEX IF NOT EXISTS idx_plan_executions_plan_key ON plan_executions(plan_key);
CREATE INDEX IF NOT EXISTS idx_plan_executions_config_id ON plan_executions(config_id);
CREATE INDEX IF NOT EXISTS idx_plan_executions_status ON plan_executions(status);
CREATE INDEX IF NOT EXISTS idx_plan_executions_started_at ON plan_executions(started_at);
CREATE INDEX IF NOT EXISTS idx_plan_executions_parent_id ON plan_executions(parent_execution_id);
CREATE INDEX IF NOT EXISTS idx_plan_contexts_plan_config ON plan_contexts(plan_key, config_id);
CREATE INDEX IF NOT EXISTS idx_plan_statistics_plan_config ON plan_statistics(plan_key, config_id);
CREATE INDEX IF NOT EXISTS idx_auth_flows_integration_id ON auth_flows(integration_id);








