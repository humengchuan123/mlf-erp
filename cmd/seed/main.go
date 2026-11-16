package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/mlf-erp/mlf-erp/internal/platform/config"
	"github.com/mlf-erp/mlf-erp/internal/platform/database"
	"github.com/mlf-erp/mlf-erp/internal/platform/security"
)

const (
	defaultTenantID = "00000000-0000-0000-0000-000000000001"
	defaultOrgID    = "00000000-0000-0000-0000-000000000002"
	adminRoleID     = "00000000-0000-0000-0000-000000000003"
	adminUserID     = "00000000-0000-0000-0000-000000000004"
	defaultPassword = "Admin@123456"
)

var permissions = []struct {
	Code   string
	Name   string
	Module string
	Type   string
}{
	{"system.user.read", "查看用户", "system", "api"},
	{"system.user.write", "管理用户", "system", "api"},
	{"system.role.read", "查看角色", "system", "api"},
	{"system.role.write", "管理角色", "system", "api"},
	{"system.org.read", "查看组织", "system", "api"},
	{"system.org.write", "管理组织", "system", "api"},
	{"system.audit.read", "查看审计日志", "system", "api"},
	{"purchase.read", "查看采购", "purchase", "api"},
	{"purchase.write", "管理采购", "purchase", "api"},
	{"sales.read", "查看销售", "sales", "api"},
	{"sales.write", "管理销售", "sales", "api"},
	{"inventory.read", "查看库存", "inventory", "api"},
	{"inventory.write", "管理库存", "inventory", "api"},
	{"finance.read", "查看财务", "finance", "api"},
	{"finance.write", "管理财务", "finance", "api"},
}

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	if !cfg.DB.Enabled {
		slog.Error("database is required for seed")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := database.NewPostgres(ctx, cfg.DB)
	if err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	tx, err := db.Begin(ctx)
	if err != nil {
		slog.Error("begin transaction", "error", err)
		os.Exit(1)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.tenants (id, code, name)
		VALUES ($1, 'default', 'Default Tenant')
		ON CONFLICT (id) DO NOTHING`, defaultTenantID); err != nil {
		fail("seed tenant", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.organizations (id, tenant_id, parent_id, type, code, name, path)
		VALUES ($1, $2, NULL, 'company', 'HQ', 'Default Company', '/')
		ON CONFLICT (id) DO NOTHING`, defaultOrgID, defaultTenantID); err != nil {
		fail("seed organization", err)
	}

	for _, permission := range permissions {
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.permissions (code, name, module, type)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (code) DO UPDATE
			SET name = EXCLUDED.name, module = EXCLUDED.module, type = EXCLUDED.type`,
			permission.Code, permission.Name, permission.Module, permission.Type); err != nil {
			fail("seed permission", err)
		}
	}

	var roleID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO erp.roles (id, tenant_id, code, name, is_system)
		VALUES ($1, $2, 'admin', 'Administrator', true)
		ON CONFLICT (tenant_id, code) DO UPDATE
		SET name = EXCLUDED.name, is_system = true
		RETURNING id::text`, adminRoleID, defaultTenantID).Scan(&roleID); err != nil {
		fail("seed admin role", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM erp.role_permissions WHERE tenant_id = $1 AND role_id = $2`, defaultTenantID, roleID); err != nil {
		fail("clear role permissions", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
		SELECT $1, $2, code FROM erp.permissions`, defaultTenantID, roleID); err != nil {
		fail("assign role permissions", err)
	}

	passwordHash, err := security.HashPassword(defaultPassword)
	if err != nil {
		fail("hash password", err)
	}

	var userID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO erp.users (
			id, tenant_id, primary_org_id, username, display_name, email,
			password_hash, status, is_super_admin
		) VALUES ($1, $2, $3, 'admin', 'Administrator', '', $4, 'active', true)
		ON CONFLICT (tenant_id, username) DO UPDATE
		SET primary_org_id = EXCLUDED.primary_org_id,
		    status = 'active',
		    is_super_admin = true
		RETURNING id::text`,
		adminUserID, defaultTenantID, defaultOrgID, passwordHash,
	).Scan(&userID); err != nil {
		fail("seed admin user", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.user_roles (tenant_id, user_id, role_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, user_id, role_id) DO NOTHING`, defaultTenantID, userID, roleID); err != nil {
		fail("assign admin role", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.org_memberships (tenant_id, user_id, org_id, role_id, data_scope)
		VALUES ($1, $2, $3, $4, 'all')
		ON CONFLICT (tenant_id, user_id, org_id) DO UPDATE
		SET role_id = EXCLUDED.role_id, data_scope = 'all'`,
		defaultTenantID, userID, defaultOrgID, roleID); err != nil {
		fail("assign admin organization", err)
	}

	if err := tx.Commit(ctx); err != nil {
		fail("commit seed transaction", err)
	}

	fmt.Printf("seed completed: tenant=default username=admin password=%s\n", defaultPassword)
}

func fail(message string, err error) {
	slog.Error(message, "error", err)
	os.Exit(1)
}
