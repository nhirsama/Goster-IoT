package v1_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

func TestTenantRoleControlsDeviceWritePermissions(t *testing.T) {
	env := newTestAPI(t)
	uuid := strings.Repeat("7", 64)
	seedDevice(t, env.dataStore, uuid, inter.Authenticated)

	readonlyReq := httptest.NewRequest(http.MethodPost, "/api/v1/devices/"+uuid+"/commands",
		bytes.NewBufferString(`{"command":"action_exec","payload":{"op":"reboot"}}`))
	readonlyReq = withTenantPerm(readonlyReq, "tenant_legacy", inter.TenantRoleRO)
	readonlyRec := httptest.NewRecorder()
	env.api.DeviceByUUIDHandler(readonlyRec, readonlyReq)
	if readonlyRec.Code != http.StatusForbidden {
		t.Fatalf("tenant_ro command should return 403, got %d body=%s", readonlyRec.Code, readonlyRec.Body.String())
	}

	readwriteReq := httptest.NewRequest(http.MethodPost, "/api/v1/devices/"+uuid+"/commands",
		bytes.NewBufferString(`{"command":"action_exec","payload":{"op":"reboot"}}`))
	readwriteReq = withTenantPerm(readwriteReq, "tenant_legacy", inter.TenantRoleRW)
	readwriteRec := httptest.NewRecorder()
	env.api.DeviceByUUIDHandler(readwriteRec, readwriteReq)
	if readwriteRec.Code != http.StatusOK {
		t.Fatalf("tenant_rw command should return 200, got %d body=%s", readwriteRec.Code, readwriteRec.Body.String())
	}
}

func TestOnlyTenantAdminCanManageTenantMembers(t *testing.T) {
	env := newTestAPI(t)
	seedUser(t, env.dataStore, "member_user")

	rwReq := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/tenant_legacy/users",
		bytes.NewBufferString(`{"username":"member_user","role":"tenant_ro"}`))
	rwReq = withTenantPerm(rwReq, "tenant_legacy", inter.TenantRoleRW)
	rwRec := httptest.NewRecorder()
	env.api.TenantByIDHandler(rwRec, rwReq)
	if rwRec.Code != http.StatusForbidden {
		t.Fatalf("tenant_rw member management should return 403, got %d body=%s", rwRec.Code, rwRec.Body.String())
	}

	adminReq := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/tenant_legacy/users",
		bytes.NewBufferString(`{"username":"member_user","role":"tenant_ro"}`))
	adminReq = withTenantPerm(adminReq, "tenant_legacy", inter.TenantRoleAdmin)
	adminRec := httptest.NewRecorder()
	env.api.TenantByIDHandler(adminRec, adminReq)
	if adminRec.Code != http.StatusCreated {
		t.Fatalf("tenant_admin member management should return 201, got %d body=%s", adminRec.Code, adminRec.Body.String())
	}
}
