-- SQLite 初始化结构。
-- 这套迁移只用于本地开发和轻量测试环境。

CREATE TABLE IF NOT EXISTS devices (
    uuid TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT 'tenant_legacy',
    name TEXT,
    hw_version TEXT,
    sw_version TEXT,
    config_version TEXT,
    sn TEXT,
    mac TEXT,
    created_at DATETIME,
    token TEXT UNIQUE,
    auth_status INTEGER
);

CREATE INDEX IF NOT EXISTS idx_devices_tenant_uuid ON devices (tenant_id, uuid);

CREATE TABLE IF NOT EXISTS metrics (
    uuid TEXT NOT NULL,
    tenant_id TEXT NOT NULL DEFAULT 'tenant_legacy',
    ts BIGINT NOT NULL,
    value REAL,
    type INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_metrics_query ON metrics (uuid, ts);
CREATE INDEX IF NOT EXISTS idx_metrics_type ON metrics (uuid, type, ts);
CREATE INDEX IF NOT EXISTS idx_metrics_tenant_uuid_ts ON metrics (tenant_id, uuid, ts);

CREATE TABLE IF NOT EXISTS logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT,
    tenant_id TEXT NOT NULL DEFAULT 'tenant_legacy',
    level TEXT,
    message TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_logs_uuid ON logs (uuid);
CREATE INDEX IF NOT EXISTS idx_logs_tenant_uuid_created ON logs (tenant_id, uuid, created_at);

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT,
    username TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    permission INTEGER NOT NULL DEFAULT 0,
    oauth2_uid TEXT,
    oauth2_provider TEXT,
    oauth2_access_token TEXT,
    oauth2_refresh_token TEXT,
    oauth2_expiry DATETIME,
    remember_token TEXT,
    recover_token TEXT,
    recover_token_expiry DATETIME,
    confirm_token TEXT,
    confirmed BOOLEAN NOT NULL DEFAULT FALSE,
    last_login DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tenants (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'active',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tenant_users (
    tenant_id TEXT NOT NULL,
    username TEXT NOT NULL,
    role TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, username),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

CREATE INDEX IF NOT EXISTS idx_tenant_users_username ON tenant_users (username);

CREATE TABLE IF NOT EXISTS device_groups (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, name),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

CREATE INDEX IF NOT EXISTS idx_device_groups_tenant ON device_groups (tenant_id);

CREATE TABLE IF NOT EXISTS group_devices (
    group_id TEXT NOT NULL,
    device_uuid TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, device_uuid),
    FOREIGN KEY (group_id) REFERENCES device_groups(id)
);

CREATE INDEX IF NOT EXISTS idx_group_devices_uuid ON group_devices (device_uuid);

CREATE TABLE IF NOT EXISTS group_users (
    group_id TEXT NOT NULL,
    username TEXT NOT NULL,
    role TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, username),
    FOREIGN KEY (group_id) REFERENCES device_groups(id)
);

CREATE INDEX IF NOT EXISTS idx_group_users_username ON group_users (username);

CREATE TABLE IF NOT EXISTS integration_external_entities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id TEXT NOT NULL DEFAULT 'tenant_legacy',
    source TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    domain TEXT NOT NULL,
    goster_uuid TEXT,
    device_id TEXT,
    model TEXT,
    name TEXT,
    room_name TEXT,
    unit TEXT,
    value_type TEXT NOT NULL DEFAULT 'string',
    device_class TEXT,
    state_class TEXT,
    attributes_json TEXT,
    last_state_text TEXT,
    last_state_num REAL,
    last_state_bool INTEGER,
    last_seen_ts BIGINT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (source, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_integration_entities_source_domain
    ON integration_external_entities (source, domain);
CREATE INDEX IF NOT EXISTS idx_integration_entities_uuid
    ON integration_external_entities (goster_uuid);
CREATE INDEX IF NOT EXISTS idx_ext_entities_tenant_source_domain
    ON integration_external_entities (tenant_id, source, domain);

CREATE TABLE IF NOT EXISTS integration_external_observations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id TEXT NOT NULL DEFAULT 'tenant_legacy',
    source TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    ts BIGINT NOT NULL,
    value_num REAL,
    value_text TEXT,
    value_bool INTEGER,
    value_json TEXT,
    unit TEXT,
    value_sig TEXT NOT NULL DEFAULT '',
    raw_event_json TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (source, entity_id, ts, value_sig)
);

CREATE INDEX IF NOT EXISTS idx_integration_observations_query
    ON integration_external_observations (source, entity_id, ts);
CREATE INDEX IF NOT EXISTS idx_ext_obs_tenant_source_entity_ts
    ON integration_external_observations (tenant_id, source, entity_id, ts);

CREATE TABLE IF NOT EXISTS integration_external_commands (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id TEXT NOT NULL DEFAULT 'tenant_legacy',
    source TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    command TEXT NOT NULL,
    payload_json TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    error_text TEXT,
    requested_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    executed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_integration_commands_status
    ON integration_external_commands (source, status, requested_at);
CREATE INDEX IF NOT EXISTS idx_ext_cmd_tenant_source_status
    ON integration_external_commands (tenant_id, source, status, requested_at);

INSERT INTO tenants (id, name, status)
VALUES ('tenant_legacy', 'legacy', 'active')
ON CONFLICT (id) DO NOTHING;
