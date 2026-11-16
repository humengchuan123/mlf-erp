package app

import (
	"context"
	"errors"
	"testing"

	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
)

func testActorContext() context.Context {
	return auth.WithActor(context.Background(), auth.Actor{
		UserID:      "u1",
		UserName:    "Manager",
		TenantID:    "t1",
		Permissions: []string{"system.user.write", "system.role.write", "purchase.read"},
	})
}

func newAuthorizationFixtures() (*fakeUserRepo, *fakeRoleRepo, *fakeRefreshTokenRepo) {
	users := newFakeUserRepo()
	users.users[testKey("t1", "u1")] = &domain.User{
		ID: "u1", TenantID: "t1", Username: "manager", DisplayName: "Manager", Status: domain.UserStatusActive,
	}
	users.users[testKey("t1", "u2")] = &domain.User{
		ID: "u2", TenantID: "t1", Username: "target", DisplayName: "Target", Status: domain.UserStatusActive,
	}
	users.users[testKey("t1", "u3")] = &domain.User{
		ID: "u3", TenantID: "t1", Username: "root", DisplayName: "Root", Status: domain.UserStatusActive, IsSuperAdmin: true,
	}

	roles := newFakeRoleRepo()
	roles.roles["r-admin"] = &domain.Role{ID: "r-admin", TenantID: "t1", Code: "admin", Name: "Admin", IsSystem: true}
	roles.rolePermissions["r-admin"] = []string{"system.user.write", "system.role.write", "purchase.read", "finance.read"}
	roles.roles["r-purchase"] = &domain.Role{ID: "r-purchase", TenantID: "t1", Code: "purchase", Name: "Purchase"}
	roles.rolePermissions["r-purchase"] = []string{"purchase.read"}

	return users, roles, newFakeRefreshTokenRepo()
}

func TestUserServiceBlocksSuperAdminEscalation(t *testing.T) {
	users, roles, tokens := newAuthorizationFixtures()
	service := NewUserService(users, roles, tokens)

	_, err := service.Create(testActorContext(), CreateUserInput{
		TenantID:     "t1",
		Username:     "new-admin",
		DisplayName:  "New Admin",
		Password:     "Admin@123456",
		IsSuperAdmin: true,
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Create() error = %v, want ErrForbidden", err)
	}

	_, err = service.Update(testActorContext(), "t1", "u2", UpdateUserInput{
		DisplayName:  "Target",
		IsSuperAdmin: true,
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Update() error = %v, want ErrForbidden", err)
	}
}

func TestUserServiceBlocksPrivilegedRoleAssignment(t *testing.T) {
	users, roles, tokens := newAuthorizationFixtures()
	service := NewUserService(users, roles, tokens)

	if err := service.SetRoles(testActorContext(), "t1", "u1", []string{"r-purchase"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("SetRoles(self) error = %v, want ErrForbidden", err)
	}

	if err := service.SetRoles(testActorContext(), "t1", "u2", []string{"r-admin"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("SetRoles(system role) error = %v, want ErrForbidden", err)
	}

	if err := service.SetRoles(testActorContext(), "t1", "u2", []string{"r-purchase"}); err != nil {
		t.Fatalf("SetRoles(allowed role) error = %v", err)
	}
}

func TestRoleServiceBlocksPermissionEscalation(t *testing.T) {
	_, roles, _ := newAuthorizationFixtures()
	service := NewRoleService(roles)

	if err := service.SetPermissions(testActorContext(), "t1", "r-admin", []string{"purchase.read"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("SetPermissions(system role) error = %v, want ErrForbidden", err)
	}

	if err := service.SetPermissions(testActorContext(), "t1", "r-purchase", []string{"finance.read"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("SetPermissions(unheld permission) error = %v, want ErrForbidden", err)
	}

	if err := service.SetPermissions(testActorContext(), "t1", "r-purchase", []string{"purchase.read"}); err != nil {
		t.Fatalf("SetPermissions(allowed) error = %v", err)
	}
}

func TestUserServiceProtectsLastSuperAdmin(t *testing.T) {
	users, roles, tokens := newAuthorizationFixtures()
	service := NewUserService(users, roles, tokens)

	superActor := auth.WithActor(context.Background(), auth.Actor{
		UserID:       "u3",
		UserName:     "Root",
		TenantID:     "t1",
		Permissions:  []string{"system.user.write"},
		IsSuperAdmin: true,
	})

	err := service.Disable(superActor, "t1", "u3")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Disable(self) error = %v, want ErrForbidden", err)
	}
}
