package tenant_test

import (
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/storage/internal/testhelper"
	"github.com/nhirsama/Goster-IoT/src/storage/tenant"
)

func TestCreateTenantInvitation(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "invitation_create.db")
	defer base.Close()

	repo := tenant.NewRepository(base.DB)

	// 创建测试租户
	createdTenant, err := repo.CreateTenant(inter.Tenant{
		Name:   "Test Tenant",
		Status: inter.TenantStatusActive,
	})
	if err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	// 测试创建邀请
	invitation, err := repo.CreateTenantInvitation(inter.TenantInvitation{
		TenantID:  createdTenant.ID,
		Username:  "testuser",
		Role:      inter.TenantRoleRW,
		InvitedBy: "admin",
	})

	if err != nil {
		t.Fatalf("failed to create invitation: %v", err)
	}

	if invitation.ID == "" {
		t.Error("invitation ID should not be empty")
	}

	if invitation.Status != "pending" {
		t.Errorf("expected status 'pending', got '%s'", invitation.Status)
	}

	if invitation.Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", invitation.Username)
	}

	// 验证过期时间大约是7天后
	expectedExpiry := time.Now().UTC().Add(7 * 24 * time.Hour)
	diff := invitation.ExpiresAt.Sub(expectedExpiry)
	if diff < -time.Minute || diff > time.Minute {
		t.Errorf("expiry time should be around 7 days from now, got %v", invitation.ExpiresAt)
	}
}

func TestListPendingInvitations(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "invitation_list.db")
	defer base.Close()

	repo := tenant.NewRepository(base.DB)

	// 创建测试租户
	createdTenant, _ := repo.CreateTenant(inter.Tenant{
		Name:   "Test Tenant",
		Status: inter.TenantStatusActive,
	})

	// 创建多个邀请
	repo.CreateTenantInvitation(inter.TenantInvitation{
		TenantID:  createdTenant.ID,
		Username:  "user1",
		Role:      inter.TenantRoleRW,
		InvitedBy: "admin",
	})

	repo.CreateTenantInvitation(inter.TenantInvitation{
		TenantID:  createdTenant.ID,
		Username:  "user1",
		Role:      inter.TenantRoleRO,
		InvitedBy: "admin",
	})

	repo.CreateTenantInvitation(inter.TenantInvitation{
		TenantID:  createdTenant.ID,
		Username:  "user2",
		Role:      inter.TenantRoleRW,
		InvitedBy: "admin",
	})

	// 列出 user1 的邀请
	invitations, err := repo.ListPendingInvitations("user1")
	if err != nil {
		t.Fatalf("failed to list invitations: %v", err)
	}

	if len(invitations) != 2 {
		t.Errorf("expected 2 invitations for user1, got %d", len(invitations))
	}

	// 列出 user2 的邀请
	invitations, err = repo.ListPendingInvitations("user2")
	if err != nil {
		t.Fatalf("failed to list invitations: %v", err)
	}

	if len(invitations) != 1 {
		t.Errorf("expected 1 invitation for user2, got %d", len(invitations))
	}
}

func TestAcceptInvitation(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "invitation_accept.db")
	defer base.Close()

	repo := tenant.NewRepository(base.DB)

	// 创建测试租户
	createdTenant, _ := repo.CreateTenant(inter.Tenant{
		Name:   "Test Tenant",
		Status: inter.TenantStatusActive,
	})

	// 创建邀请
	invitation, _ := repo.CreateTenantInvitation(inter.TenantInvitation{
		TenantID:  createdTenant.ID,
		Username:  "testuser",
		Role:      inter.TenantRoleRW,
		InvitedBy: "admin",
	})

	// 接受邀请
	err := repo.AcceptInvitation(invitation.ID)
	if err != nil {
		t.Fatalf("failed to accept invitation: %v", err)
	}

	// 验证用户已添加到租户
	members, err := repo.ListTenantUsers(createdTenant.ID)
	if err != nil {
		t.Fatalf("failed to list tenant users: %v", err)
	}

	if len(members) != 1 {
		t.Errorf("expected 1 member, got %d", len(members))
	}

	if members[0].Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", members[0].Username)
	}

	if members[0].Role != inter.TenantRoleRW {
		t.Errorf("expected role 'tenant_rw', got '%s'", members[0].Role)
	}

	// 验证邀请状态已更新
	updatedInvitation, err := repo.GetInvitation(invitation.ID)
	if err != nil {
		t.Fatalf("failed to get invitation: %v", err)
	}

	if updatedInvitation.Status != "accepted" {
		t.Errorf("expected status 'accepted', got '%s'", updatedInvitation.Status)
	}
}

func TestRejectInvitation(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "invitation_reject.db")
	defer base.Close()

	repo := tenant.NewRepository(base.DB)

	// 创建测试租户
	createdTenant, _ := repo.CreateTenant(inter.Tenant{
		Name:   "Test Tenant",
		Status: inter.TenantStatusActive,
	})

	// 创建邀请
	invitation, _ := repo.CreateTenantInvitation(inter.TenantInvitation{
		TenantID:  createdTenant.ID,
		Username:  "testuser",
		Role:      inter.TenantRoleRW,
		InvitedBy: "admin",
	})

	// 拒绝邀请
	err := repo.RejectInvitation(invitation.ID)
	if err != nil {
		t.Fatalf("failed to reject invitation: %v", err)
	}

	// 验证用户未添加到租户
	members, err := repo.ListTenantUsers(createdTenant.ID)
	if err != nil {
		t.Fatalf("failed to list tenant users: %v", err)
	}

	if len(members) != 0 {
		t.Errorf("expected 0 members, got %d", len(members))
	}

	// 验证邀请状态已更新
	updatedInvitation, err := repo.GetInvitation(invitation.ID)
	if err != nil {
		t.Fatalf("failed to get invitation: %v", err)
	}

	if updatedInvitation.Status != "rejected" {
		t.Errorf("expected status 'rejected', got '%s'", updatedInvitation.Status)
	}
}

func TestInvitationNotFound(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "invitation_notfound.db")
	defer base.Close()

	repo := tenant.NewRepository(base.DB)

	_, err := repo.GetInvitation("nonexistent")
	if err != inter.ErrInvitationNotFound {
		t.Errorf("expected ErrInvitationNotFound, got %v", err)
	}
}

func TestCannotAcceptTwice(t *testing.T) {
	base, _ := testhelper.OpenSQLiteStore(t, "invitation_twice.db")
	defer base.Close()

	repo := tenant.NewRepository(base.DB)

	// 创建测试租户
	createdTenant, _ := repo.CreateTenant(inter.Tenant{
		Name:   "Test Tenant",
		Status: inter.TenantStatusActive,
	})

	// 创建邀请
	invitation, _ := repo.CreateTenantInvitation(inter.TenantInvitation{
		TenantID:  createdTenant.ID,
		Username:  "testuser",
		Role:      inter.TenantRoleRW,
		InvitedBy: "admin",
	})

	// 第一次接受
	repo.AcceptInvitation(invitation.ID)

	// 第二次接受应该失败
	err := repo.AcceptInvitation(invitation.ID)
	if err != inter.ErrInvitationAccepted {
		t.Errorf("expected ErrInvitationAccepted, got %v", err)
	}
}
