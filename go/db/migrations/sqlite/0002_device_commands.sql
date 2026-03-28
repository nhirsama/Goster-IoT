CREATE TABLE IF NOT EXISTS device_commands (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id TEXT NOT NULL DEFAULT 'tenant_legacy',
    uuid TEXT NOT NULL,
    cmd_id INTEGER NOT NULL,
    command TEXT NOT NULL,
    payload_json TEXT,
    status TEXT NOT NULL DEFAULT 'queued',
    error_text TEXT,
    requested_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    executed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_device_commands_uuid_status
    ON device_commands (uuid, status, requested_at);
CREATE INDEX IF NOT EXISTS idx_device_commands_tenant_uuid_status
    ON device_commands (tenant_id, uuid, status, requested_at);

INSERT INTO device_commands (id, tenant_id, uuid, cmd_id, command, payload_json, status, error_text, requested_at, executed_at)
SELECT
    id,
    COALESCE(NULLIF(TRIM(tenant_id), ''), 'tenant_legacy'),
    entity_id,
    CASE
        WHEN instr(command, ':') > 0 THEN CAST(substr(command, instr(command, ':') + 1) AS INTEGER)
        ELSE 0
    END,
    CASE
        WHEN instr(command, ':') > 0 THEN substr(command, 1, instr(command, ':') - 1)
        ELSE command
    END,
    payload_json,
    status,
    error_text,
    requested_at,
    executed_at
FROM integration_external_commands
WHERE source = 'goster_device'
  AND NOT EXISTS (
      SELECT 1
      FROM device_commands
      WHERE device_commands.id = integration_external_commands.id
  );

DELETE FROM integration_external_commands
WHERE source = 'goster_device';
