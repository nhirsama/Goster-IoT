package datastore

import (
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// TestMultiTenantFullWorkflow 测试完整的多租户工作流
func TestMultiTenantFullWorkflow(t *testing.T) {
	store := newTestStore(t)
	sqlStore := asSQLStore(t, store)

	// 1. 创建租户
	t.Run("CreateTenants", func(t *testing.T) {
		tenants := []struct {
			id     string
			name   string
			status string
		}{
			{"tenant_acme", "ACME Corporation", "active"},
			{"tenant_beta", "Beta Industries", "active"},
		}

		for _, tenant := range tenants {
			_, err := sqlStore.db.Exec(`
				INSERT INTO tenants (id, name, status, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (id) DO NOTHING
			`, tenant.id, tenant.name, tenant.status, time.Now(), time.Now())
			if err != nil {
				t.Fatalf("Failed to create tenant %s: %v", tenant.id, err)
			}
		}

		// 验证租户创建
		var count int
		err := sqlStore.db.QueryRow("SELECT COUNT(*) FROM tenants WHERE id IN ('tenant_acme', 'tenant_beta')").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count tenants: %v", err)
		}
		if count != 2 {
			t.Fatalf("Expected 2 tenants, got %d", count)
		}
	})

	// 2. 创建用户并关联租户
	t.Run("CreateUsersAndAssignToTenants", func(t *testing.T) {
		users := []struct {
			username string
			email    string
			password string
			perm     inter.PermissionType
		}{
			{"alice", "alice@acme.com", "hashed_pw_alice", inter.PermissionReadWrite},
			{"bob", "bob@beta.com", "hashed_pw_bob", inter.PermissionReadWrite},
			{"charlie", "charlie@acme.com", "hashed_pw_charlie", inter.PermissionReadOnly},
		}

		for _, user := range users {
			_, err := sqlStore.db.Exec(`
				INSERT INTO users (email, username, password, permission, confirmed, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (username) DO NOTHING
			`, user.email, user.username, user.password, user.perm, true, time.Now(), time.Now())
			if err != nil {
				t.Fatalf("Failed to create user %s: %v", user.username, err)
			}
		}

		// 关联用户到租户
		tenantAssignments := []struct {
			tenantID string
			username string
			role     string
		}{
			{"tenant_acme", "alice", "tenant_admin"},
			{"tenant_acme", "charlie", "tenant_ro"},
			{"tenant_beta", "bob", "tenant_admin"},
		}

		for _, assignment := range tenantAssignments {
			_, err := sqlStore.db.Exec(`
				INSERT INTO tenant_users (tenant_id, username, role, created_at)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (tenant_id, username) DO NOTHING
			`, assignment.tenantID, assignment.username, assignment.role, time.Now())
			if err != nil {
				t.Fatalf("Failed to assign user %s to tenant %s: %v",
					assignment.username, assignment.tenantID, err)
			}
		}

		// 验证租户用户关联
		var aliceRole string
		err := sqlStore.db.QueryRow(`
			SELECT role FROM tenant_users WHERE tenant_id = 'tenant_acme' AND username = 'alice'
		`).Scan(&aliceRole)
		if err != nil {
			t.Fatalf("Failed to query Alice's role: %v", err)
		}
		if aliceRole != "tenant_admin" {
			t.Fatalf("Expected Alice role tenant_admin, got %s", aliceRole)
		}
	})

	// 3. 创建设备并分配到租户
	t.Run("CreateDevicesForTenants", func(t *testing.T) {
		devices := []struct {
			uuid     string
			tenantID string
			name     string
		}{
			{"device-acme-001", "tenant_acme", "ACME Sensor 1"},
			{"device-acme-002", "tenant_acme", "ACME Sensor 2"},
			{"device-beta-001", "tenant_beta", "Beta Sensor 1"},
		}

		for _, device := range devices {
			meta := inter.DeviceMetadata{
				Name:               device.name,
				HWVersion:          "v1.0",
				SWVersion:          "v2.0",
				ConfigVersion:      "cfg1",
				SerialNumber:       "SN-" + device.uuid,
				MACAddress:         "AA:BB:CC:DD:EE:FF",
				CreatedAt:          time.Now(),
				Token:              "token-" + device.uuid,
				AuthenticateStatus: inter.Authenticated,
			}
			if err := store.InitDevice(device.uuid, meta); err != nil {
				t.Fatalf("Failed to create device %s: %v", device.uuid, err)
			}

			// 设置租户 ID
			_, err := sqlStore.db.Exec("UPDATE devices SET tenant_id = $1 WHERE uuid = $2",
				device.tenantID, device.uuid)
			if err != nil {
				t.Fatalf("Failed to assign device %s to tenant %s: %v",
					device.uuid, device.tenantID, err)
			}
		}

		// 验证租户设备隔离
		acmeDevices, err := store.ListDevicesByTenant("tenant_acme", nil, 1, 100)
		if err != nil {
			t.Fatalf("Failed to list ACME devices: %v", err)
		}
		if len(acmeDevices) != 2 {
			t.Fatalf("Expected 2 ACME devices, got %d", len(acmeDevices))
		}

		betaDevices, err := store.ListDevicesByTenant("tenant_beta", nil, 1, 100)
		if err != nil {
			t.Fatalf("Failed to list Beta devices: %v", err)
		}
		if len(betaDevices) != 1 {
			t.Fatalf("Expected 1 Beta device, got %d", len(betaDevices))
		}
	})

	// 4. 创建设备分组
	t.Run("CreateDeviceGroups", func(t *testing.T) {
		groups := []struct {
			id          string
			tenantID    string
			name        string
			description string
		}{
			{"group-acme-001", "tenant_acme", "Production Sensors", "生产环境传感器"},
			{"group-acme-002", "tenant_acme", "Test Sensors", "测试环境传感器"},
			{"group-beta-001", "tenant_beta", "Beta Group 1", "Beta 分组 1"},
		}

		for _, group := range groups {
			_, err := sqlStore.db.Exec(`
				INSERT INTO device_groups (id, tenant_id, name, description, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (id) DO NOTHING
			`, group.id, group.tenantID, group.name, group.description, time.Now(), time.Now())
			if err != nil {
				t.Fatalf("Failed to create group %s: %v", group.id, err)
			}
		}

		// 验证分组创建
		var groupCount int
		err := sqlStore.db.QueryRow(`
			SELECT COUNT(*) FROM device_groups WHERE tenant_id = 'tenant_acme'
		`).Scan(&groupCount)
		if err != nil {
			t.Fatalf("Failed to count ACME groups: %v", err)
		}
		if groupCount != 2 {
			t.Fatalf("Expected 2 ACME groups, got %d", groupCount)
		}
	})

	// 5. 添加设备到分组
	t.Run("AssignDevicesToGroups", func(t *testing.T) {
		assignments := []struct {
			groupID string
			uuid    string
		}{
			{"group-acme-001", "device-acme-001"},
			{"group-acme-002", "device-acme-002"},
			{"group-beta-001", "device-beta-001"},
		}

		for _, assignment := range assignments {
			_, err := sqlStore.db.Exec(`
				INSERT INTO group_devices (group_id, device_uuid, created_at)
				VALUES ($1, $2, $3)
				ON CONFLICT (group_id, device_uuid) DO NOTHING
			`, assignment.groupID, assignment.uuid, time.Now())
			if err != nil {
				t.Fatalf("Failed to assign device %s to group %s: %v",
					assignment.uuid, assignment.groupID, err)
			}
		}

		// 验证分组设备
		var deviceCount int
		err := sqlStore.db.QueryRow(`
			SELECT COUNT(*) FROM group_devices WHERE group_id = 'group-acme-001'
		`).Scan(&deviceCount)
		if err != nil {
			t.Fatalf("Failed to count devices in group: %v", err)
		}
		if deviceCount != 1 {
			t.Fatalf("Expected 1 device in group, got %d", deviceCount)
		}
	})

	// 6. 测试跨租户隔离
	t.Run("CrossTenantIsolation", func(t *testing.T) {
		// 尝试从 tenant_beta 加载 tenant_acme 的设备
		_, err := store.LoadConfigByTenant("tenant_beta", "device-acme-001")
		if err == nil {
			t.Fatal("Expected cross-tenant access to fail")
		}

		// 正确的租户应该能访问
		config, err := store.LoadConfigByTenant("tenant_acme", "device-acme-001")
		if err != nil {
			t.Fatalf("Failed to load device from correct tenant: %v", err)
		}
		if config.Name != "ACME Sensor 1" {
			t.Fatalf("Unexpected device name: %s", config.Name)
		}
	})

	// 7. 测试租户级指标隔离
	t.Run("TenantMetricsIsolation", func(t *testing.T) {
		baseTime := time.Now().UnixMilli()

		// 为 ACME 设备写入指标
		for i := 0; i < 5; i++ {
			point := inter.MetricPoint{
				Timestamp: baseTime + int64(i*1000),
				Value:     float32(20 + i),
				Type:      1,
			}
			if err := store.AppendMetric("device-acme-001", point); err != nil {
				t.Fatalf("Failed to append metric: %v", err)
			}
		}

		// 查询 ACME 租户指标
		acmeMetrics, err := store.QueryMetricsByTenant("tenant_acme", "device-acme-001",
			baseTime-1000, baseTime+10000, 0)
		if err != nil {
			t.Fatalf("Failed to query ACME metrics: %v", err)
		}
		if len(acmeMetrics) != 5 {
			t.Fatalf("Expected 5 ACME metrics, got %d", len(acmeMetrics))
		}

		// Beta 租户不应该看到 ACME 的指标
		betaMetrics, err := store.QueryMetricsByTenant("tenant_beta", "device-acme-001",
			baseTime-1000, baseTime+10000, 0)
		if err != nil {
			t.Fatalf("Failed to query Beta metrics: %v", err)
		}
		if len(betaMetrics) != 0 {
			t.Fatalf("Expected 0 Beta metrics, got %d", len(betaMetrics))
		}
	})

	// 8. 测试租户级设备指令
	t.Run("TenantDeviceCommands", func(t *testing.T) {
		// ACME 租户创建指令
		commandID, err := store.CreateDeviceCommandByTenant("tenant_acme", "device-acme-001",
			inter.CmdActionExec, "action_exec", []byte(`{"op":"reboot"}`))
		if err != nil {
			t.Fatalf("Failed to create ACME command: %v", err)
		}
		if commandID <= 0 {
			t.Fatalf("Invalid command ID: %d", commandID)
		}

		// 验证指令属于正确的租户
		var storedTenant string
		err = sqlStore.db.QueryRow(`
			SELECT tenant_id FROM integration_external_commands WHERE id = $1
		`, commandID).Scan(&storedTenant)
		if err != nil {
			t.Fatalf("Failed to query command tenant: %v", err)
		}
		if storedTenant != "tenant_acme" {
			t.Fatalf("Expected tenant_acme, got %s", storedTenant)
		}

		// Beta 租户不应该能为 ACME 设备创建指令
		_, err = store.CreateDeviceCommandByTenant("tenant_beta", "device-acme-001",
			inter.CmdActionExec, "action_exec", []byte(`{"op":"test"}`))
		if err == nil {
			t.Fatal("Expected cross-tenant command creation to fail")
		}
	})
}

// TestDeviceGroupOperations 测试设备分组操作
func TestDeviceGroupOperations(t *testing.T) {
	store := newTestStore(t)
	sqlStore := asSQLStore(t, store)

	tenantID := "test_group_tenant"
	groupID := "test_group_001"

	// 创建测试租户
	_, err := sqlStore.db.Exec(`
		INSERT INTO tenants (id, name, status, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING
	`, tenantID, "Test Group Tenant", "active", time.Now())
	if err != nil {
		t.Fatalf("Failed to create tenant: %v", err)
	}

	// 创建设备分组
	t.Run("CreateGroup", func(t *testing.T) {
		_, err := sqlStore.db.Exec(`
			INSERT INTO device_groups (id, tenant_id, name, description, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`, groupID, tenantID, "Test Group", "Test description", time.Now())
		if err != nil {
			t.Fatalf("Failed to create group: %v", err)
		}
	})

	// 创建测试设备
	devices := []string{"dev-group-001", "dev-group-002", "dev-group-003"}
	for _, uuid := range devices {
		meta := generateRandomMeta()
		meta.Name = "Device " + uuid
		if err := store.InitDevice(uuid, meta); err != nil {
			t.Fatalf("Failed to create device %s: %v", uuid, err)
		}
		_, err := sqlStore.db.Exec("UPDATE devices SET tenant_id = $1 WHERE uuid = $2", tenantID, uuid)
		if err != nil {
			t.Fatalf("Failed to assign device to tenant: %v", err)
		}
	}

	// 添加设备到分组
	t.Run("AddDevicesToGroup", func(t *testing.T) {
		for _, uuid := range devices {
			_, err := sqlStore.db.Exec(`
				INSERT INTO group_devices (group_id, device_uuid, created_at)
				VALUES ($1, $2, $3)
			`, groupID, uuid, time.Now())
			if err != nil {
				t.Fatalf("Failed to add device %s to group: %v", uuid, err)
			}
		}

		// 验证设备数量
		var count int
		err := sqlStore.db.QueryRow("SELECT COUNT(*) FROM group_devices WHERE group_id = $1", groupID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count group devices: %v", err)
		}
		if count != 3 {
			t.Fatalf("Expected 3 devices in group, got %d", count)
		}
	})

	// 从分组移除设备
	t.Run("RemoveDeviceFromGroup", func(t *testing.T) {
		_, err := sqlStore.db.Exec("DELETE FROM group_devices WHERE group_id = $1 AND device_uuid = $2",
			groupID, devices[0])
		if err != nil {
			t.Fatalf("Failed to remove device from group: %v", err)
		}

		// 验证设备数量
		var count int
		err = sqlStore.db.QueryRow("SELECT COUNT(*) FROM group_devices WHERE group_id = $1", groupID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count group devices: %v", err)
		}
		if count != 2 {
			t.Fatalf("Expected 2 devices in group after removal, got %d", count)
		}
	})

	// 删除分组
	t.Run("DeleteGroup", func(t *testing.T) {
		// 先删除分组设备关联
		_, err := sqlStore.db.Exec("DELETE FROM group_devices WHERE group_id = $1", groupID)
		if err != nil {
			t.Fatalf("Failed to delete group devices: %v", err)
		}

		// 删除分组
		_, err = sqlStore.db.Exec("DELETE FROM device_groups WHERE id = $1", groupID)
		if err != nil {
			t.Fatalf("Failed to delete group: %v", err)
		}

		// 验证分组已删除
		var exists bool
		err = sqlStore.db.QueryRow("SELECT EXISTS(SELECT 1 FROM device_groups WHERE id = $1)", groupID).Scan(&exists)
		if err != nil {
			t.Fatalf("Failed to check group existence: %v", err)
		}
		if exists {
			t.Fatal("Group should have been deleted")
		}
	})
}
