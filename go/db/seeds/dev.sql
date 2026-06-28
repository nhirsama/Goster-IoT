-- 开发环境基础种子数据。

INSERT INTO tenants (id, name, status)
VALUES ('tenant_legacy', 'legacy', 'active')
ON CONFLICT (id) DO NOTHING;
