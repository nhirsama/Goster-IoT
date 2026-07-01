package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"path/filepath"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/cli"
)

type testClient struct {
	baseURL    string
	httpClient *http.Client
	csrfToken  string
	tenantID   string
}

func newTestClient(baseURL string) *testClient {
	jar, _ := cookiejar.New(nil)
	return &testClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Jar: jar,
		},
	}
}

func (c *testClient) post(path string, body interface{}) (*http.Response, error) {
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if c.csrfToken != "" {
		req.Header.Set("X-CSRF-Token", c.csrfToken)
	}
	if c.tenantID != "" {
		req.Header.Set("X-Tenant-Id", c.tenantID)
	}
	return c.httpClient.Do(req)
}

func (c *testClient) get(path string) (*http.Response, error) {
	req, _ := http.NewRequest("GET", c.baseURL+path, nil)
	if c.tenantID != "" {
		req.Header.Set("X-Tenant-Id", c.tenantID)
	}
	return c.httpClient.Do(req)
}

func (c *testClient) register(username, password string) error {
	resp, err := c.post("/api/v1/auth/register", map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("register failed: status %d", resp.StatusCode)
	}
	return nil
}

func (c *testClient) login(username, password string) error {
	resp, err := c.post("/api/v1/auth/login", map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed: status %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			CSRFToken string `json:"csrf_token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	c.csrfToken = result.Data.CSRFToken
	return nil
}

func TestTenantInvitationE2E(t *testing.T) {
	webAddr := reserveTCPAddr(t)

	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "invitation_test.db"))
	t.Setenv("WEB_HTTP_ADDR", webAddr)
	t.Setenv("AUTHBOSS_ROOT_URL", "http://"+webAddr)

	// 初始化数据库
	if err := cli.RunWithArgs(context.Background(), []string{"db", "init"}); err != nil {
		t.Fatalf("db init failed: %v", err)
	}

	// 启动服务器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- cli.RunWithArgs(ctx, []string{"serve"})
	}()

	baseURL := "http://" + webAddr
	waitForHTTPServer(t, baseURL+"/health")
	t.Logf("Test server running at %s", baseURL)

	// 步骤 1: 创建管理员和普通用户
	admin := newTestClient(baseURL)
	if err := admin.register("admin_user", "admin12345"); err != nil {
		t.Fatalf("failed to register admin: %v", err)
	}

	user := newTestClient(baseURL)
	if err := user.register("test_user", "test12345"); err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// 步骤 2: 管理员登录并创建租户
	if err := admin.login("admin_user", "admin12345"); err != nil {
		t.Fatalf("failed to login admin: %v", err)
	}

	resp, err := admin.post("/api/v1/tenants", map[string]string{
		"name":   "测试租户",
		"status": "active",
	})
	if err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("create tenant failed: status %d", resp.StatusCode)
	}

	var createResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&createResult); err != nil {
		t.Fatalf("failed to decode create tenant response: %v", err)
	}
	tenantID := createResult.Data.ID
	t.Logf("Created tenant: %s", tenantID)

	// 步骤 2.5: 管理员设置租户上下文
	admin.tenantID = tenantID

	// 步骤 3: 管理员邀请用户到租户
	resp, err = admin.post(fmt.Sprintf("/api/v1/tenants/%s/users", tenantID), map[string]string{
		"username": "test_user",
		"role":     "tenant_rw",
	})
	if err != nil {
		t.Fatalf("failed to invite user: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		t.Fatalf("invite user failed: status %d, body: %s", resp.StatusCode, body.String())
	}

	var inviteResult struct {
		Data struct {
			Invitation struct {
				ID        string    `json:"id"`
				TenantID  string    `json:"tenant_id"`
				Username  string    `json:"username"`
				Role      string    `json:"role"`
				InvitedBy string    `json:"invited_by"`
				Status    string    `json:"status"`
				ExpiresAt time.Time `json:"expires_at"`
			} `json:"invitation"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&inviteResult); err != nil {
		t.Fatalf("failed to decode invite response: %v", err)
	}
	invitationID := inviteResult.Data.Invitation.ID
	t.Logf("Created invitation: %s", invitationID)

	// 验证邀请详情
	invitation := inviteResult.Data.Invitation
	if invitation.TenantID != tenantID {
		t.Errorf("expected tenant_id %s, got %s", tenantID, invitation.TenantID)
	}
	if invitation.Username != "test_user" {
		t.Errorf("expected username test_user, got %s", invitation.Username)
	}
	if invitation.Role != "tenant_rw" {
		t.Errorf("expected role tenant_rw, got %s", invitation.Role)
	}
	if invitation.Status != "pending" {
		t.Errorf("expected status pending, got %s", invitation.Status)
	}

	// 步骤 4: 用户登录并查看邀请
	if err := user.login("test_user", "test12345"); err != nil {
		t.Fatalf("failed to login user: %v", err)
	}

	resp, err = user.get("/api/v1/invitations")
	if err != nil {
		t.Fatalf("failed to get invitations: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get invitations failed: status %d", resp.StatusCode)
	}

	var listResult struct {
		Data struct {
			Items []struct {
				ID       string `json:"id"`
				TenantID string `json:"tenant_id"`
				Status   string `json:"status"`
			} `json:"items"`
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResult); err != nil {
		t.Fatalf("failed to decode invitations list: %v", err)
	}

	if listResult.Data.Total != 1 {
		t.Errorf("expected 1 invitation, got %d", listResult.Data.Total)
	}

	found := false
	for _, inv := range listResult.Data.Items {
		if inv.ID == invitationID {
			found = true
			if inv.Status != "pending" {
				t.Errorf("expected status pending, got %s", inv.Status)
			}
		}
	}
	if !found {
		t.Errorf("invitation %s not found in list", invitationID)
	}

	// 步骤 5: 用户接受邀请
	resp, err = user.post(fmt.Sprintf("/api/v1/invitations/%s/accept", invitationID), nil)
	if err != nil {
		t.Fatalf("failed to accept invitation: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		t.Fatalf("accept invitation failed: status %d, body: %s", resp.StatusCode, body.String())
	}

	// 步骤 6: 验证用户已加入租户
	resp, err = user.get("/api/v1/tenants")
	if err != nil {
		t.Fatalf("failed to get user tenants: %v", err)
	}
	defer resp.Body.Close()

	var tenantsResult struct {
		Data struct {
			Items []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tenantsResult); err != nil {
		t.Fatalf("failed to decode tenants list: %v", err)
	}

	foundTenant := false
	for _, t := range tenantsResult.Data.Items {
		if t.ID == tenantID {
			foundTenant = true
			break
		}
	}
	if !foundTenant {
		t.Errorf("user not found in tenant %s after accepting invitation", tenantID)
	}

	// 步骤 7: 验证邀请列表已清空
	resp, err = user.get("/api/v1/invitations")
	if err != nil {
		t.Fatalf("failed to get invitations after accept: %v", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&listResult); err != nil {
		t.Fatalf("failed to decode invitations list: %v", err)
	}

	if listResult.Data.Total != 0 {
		t.Errorf("expected 0 pending invitations after accept, got %d", listResult.Data.Total)
	}

	t.Log("✅ All tests passed!")
}

func TestTenantInvitationReject(t *testing.T) {
	webAddr := reserveTCPAddr(t)

	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "invitation_reject_test.db"))
	t.Setenv("WEB_HTTP_ADDR", webAddr)
	t.Setenv("AUTHBOSS_ROOT_URL", "http://"+webAddr)

	if err := cli.RunWithArgs(context.Background(), []string{"db", "init"}); err != nil {
		t.Fatalf("db init failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		cli.RunWithArgs(ctx, []string{"serve"})
	}()

	baseURL := "http://" + webAddr
	waitForHTTPServer(t, baseURL+"/health")

	// 创建用户
	admin := newTestClient(baseURL)
	if err := admin.register("admin_user", "admin12345"); err != nil {
		t.Fatalf("failed to register admin: %v", err)
	}
	if err := admin.login("admin_user", "admin12345"); err != nil {
		t.Fatalf("failed to login admin: %v", err)
	}

	user := newTestClient(baseURL)
	if err := user.register("test_user", "test12345"); err != nil {
		t.Fatalf("failed to register user: %v", err)
	}
	if err := user.login("test_user", "test12345"); err != nil {
		t.Fatalf("failed to login user: %v", err)
	}

	// 创建租户并邀请
	resp, _ := admin.post("/api/v1/tenants", map[string]string{"name": "测试租户", "status": "active"})
	var createResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&createResult)
	resp.Body.Close()
	tenantID := createResult.Data.ID

	// 设置租户上下文
	admin.tenantID = tenantID

	resp, _ = admin.post(fmt.Sprintf("/api/v1/tenants/%s/users", tenantID), map[string]string{
		"username": "test_user",
		"role":     "tenant_ro",
	})
	var inviteResult struct {
		Data struct {
			Invitation struct {
				ID string `json:"id"`
			} `json:"invitation"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&inviteResult)
	resp.Body.Close()
	invitationID := inviteResult.Data.Invitation.ID

	// 用户拒绝邀请
	resp, err := user.post(fmt.Sprintf("/api/v1/invitations/%s/reject", invitationID), nil)
	if err != nil {
		t.Fatalf("failed to reject invitation: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reject invitation failed: status %d", resp.StatusCode)
	}

	// 验证用户未加入租户
	resp, _ = user.get("/api/v1/tenants")
	var tenantsResult struct {
		Data struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&tenantsResult)
	resp.Body.Close()

	for _, tenant := range tenantsResult.Data.Items {
		if tenant.ID == tenantID {
			t.Errorf("user should not be in tenant %s after rejecting invitation", tenantID)
		}
	}

	// 验证邀请已从待处理列表中移除
	resp, _ = user.get("/api/v1/invitations")
	var listResult struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&listResult)
	resp.Body.Close()

	if listResult.Data.Total != 0 {
		t.Errorf("expected 0 pending invitations after reject, got %d", listResult.Data.Total)
	}

	t.Log("✅ Reject test passed!")
}

func TestCannotAcceptInvitationTwice(t *testing.T) {
	webAddr := reserveTCPAddr(t)

	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "invitation_twice_test.db"))
	t.Setenv("WEB_HTTP_ADDR", webAddr)
	t.Setenv("AUTHBOSS_ROOT_URL", "http://"+webAddr)

	if err := cli.RunWithArgs(context.Background(), []string{"db", "init"}); err != nil {
		t.Fatalf("db init failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		cli.RunWithArgs(ctx, []string{"serve"})
	}()

	baseURL := "http://" + webAddr
	waitForHTTPServer(t, baseURL+"/health")

	admin := newTestClient(baseURL)
	admin.register("admin_user", "admin12345")
	admin.login("admin_user", "admin12345")

	user := newTestClient(baseURL)
	user.register("test_user", "test12345")
	user.login("test_user", "test12345")

	// 创建租户并邀请
	resp, _ := admin.post("/api/v1/tenants", map[string]string{"name": "测试租户", "status": "active"})
	var createResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&createResult)
	resp.Body.Close()

	// 设置租户上下文
	admin.tenantID = createResult.Data.ID

	resp, _ = admin.post(fmt.Sprintf("/api/v1/tenants/%s/users", createResult.Data.ID), map[string]string{
		"username": "test_user",
		"role":     "tenant_rw",
	})
	var inviteResult struct {
		Data struct {
			Invitation struct {
				ID string `json:"id"`
			} `json:"invitation"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&inviteResult)
	resp.Body.Close()
	invitationID := inviteResult.Data.Invitation.ID

	// 第一次接受
	resp, _ = user.post(fmt.Sprintf("/api/v1/invitations/%s/accept", invitationID), nil)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first accept failed: status %d", resp.StatusCode)
	}

	// 第二次接受应该失败
	resp, _ = user.post(fmt.Sprintf("/api/v1/invitations/%s/accept", invitationID), nil)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Errorf("second accept should fail but got status %d", resp.StatusCode)
	}

	t.Log("✅ Cannot accept twice test passed!")
}
