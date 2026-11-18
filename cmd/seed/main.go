package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	{"masterdata.partner.read", "查看客商", "masterdata", "api"},
	{"masterdata.partner.write", "管理客商", "masterdata", "api"},
	{"masterdata.material.read", "查看物料", "masterdata", "api"},
	{"masterdata.material.write", "管理物料", "masterdata", "api"},
	{"masterdata.warehouse.read", "查看仓库", "masterdata", "api"},
	{"masterdata.warehouse.write", "管理仓库", "masterdata", "api"},
	{"masterdata.unit.read", "查看计量单位", "masterdata", "api"},
	{"masterdata.unit.write", "管理计量单位", "masterdata", "api"},
	{"masterdata.currency.read", "查看币种", "masterdata", "api"},
	{"masterdata.currency.write", "管理币种", "masterdata", "api"},
	{"masterdata.tax.read", "查看税率", "masterdata", "api"},
	{"masterdata.tax.write", "管理税率", "masterdata", "api"},
	{"system.code_rule.read", "查看编码规则", "system", "api"},
	{"system.code_rule.write", "管理编码规则", "system", "api"},
	{"purchase.order.read", "查看采购订单", "purchase", "api"},
	{"purchase.order.write", "管理采购订单", "purchase", "api"},
	{"purchase.order.approve", "审批采购订单", "purchase", "api"},
	{"purchase.receipt.read", "查看采购收货", "purchase", "api"},
	{"purchase.return.read", "查看采购退货", "purchase", "api"},
	{"purchase.return.write", "管理采购退货", "purchase", "api"},
	{"inventory.balance.read", "查看库存余额", "inventory", "api"},
	{"inventory.transaction.read", "查看库存流水", "inventory", "api"},
	{"sales.order.read", "查看销售订单", "sales", "api"},
	{"sales.order.write", "管理销售订单", "sales", "api"},
	{"sales.order.approve", "审批销售订单", "sales", "api"},
	{"sales.shipment.read", "查看销售发货", "sales", "api"},
	{"sales.return.read", "查看销售退货", "sales", "api"},
	{"sales.return.write", "管理销售退货", "sales", "api"},
	{"finance.invoice.read", "查看应收应付", "finance", "api"},
	{"finance.invoice.write", "开具/作废发票", "finance", "api"},
	{"finance.payment.read", "查看收付款", "finance", "api"},
	{"finance.payment.write", "管理收付款", "finance", "api"},
	{"finance.account.read", "查看会计科目", "finance", "api"},
	{"finance.account.write", "管理会计科目", "finance", "api"},
	{"finance.period.read", "查看会计期间", "finance", "api"},
	{"finance.period.close", "关账/反关账", "finance", "api"},
	{"finance.voucher.read", "查看凭证", "finance", "api"},
	{"finance.voucher.write", "管理凭证", "finance", "api"},
	{"finance.voucher.post", "过账凭证", "finance", "api"},
	{"finance.voucher.reverse", "反过账凭证", "finance", "api"},
	{"finance.report.read", "查看财务报表", "finance", "api"},
	{"finance.exchange_rate.read", "查看汇率", "finance", "api"},
	{"finance.exchange_rate.write", "管理汇率", "finance", "api"},
}

var defaultCodeRules = []struct {
	Entity         string
	Prefix         string
	DatePattern    string
	Separator      string
	SequenceLength int
	ResetPolicy    string
}{
	{"partner", "CUS", "yyyyMMdd", "-", 4, "day"},
	{"material", "MAT", "yyyyMMdd", "-", 4, "day"},
	{"warehouse", "WH", "yyyyMMdd", "-", 3, "day"},
	{"purchase_order", "PO", "yyyyMMdd", "-", 4, "day"},
	{"purchase_receipt", "GRN", "yyyyMMdd", "-", 4, "day"},
	{"purchase_return", "PR", "yyyyMMdd", "-", 4, "day"},
	{"sales_order", "SO", "yyyyMMdd", "-", 4, "day"},
	{"sales_shipment", "DN", "yyyyMMdd", "-", 4, "day"},
	{"sales_return", "SR", "yyyyMMdd", "-", 4, "day"},
	{"finance_credit_note", "CN", "yyyyMMdd", "-", 4, "day"},
	{"sales_receivable", "AR", "yyyyMMdd", "-", 4, "day"},
	{"purchase_payable", "AP", "yyyyMMdd", "-", 4, "day"},
	{"finance_receipt", "RCPT", "yyyyMMdd", "-", 4, "day"},
	{"finance_payment", "PAY", "yyyyMMdd", "-", 4, "day"},
	{"gl_voucher", "GL", "yyyyMM", "-", 4, "month"},
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
		VALUES ($1, $2, NULL, 'company', 'HQ', 'Default Company', '/' || ($1::uuid)::text)
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

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
		SELECT $1, $2, code FROM erp.permissions
		ON CONFLICT DO NOTHING`, defaultTenantID, roleID); err != nil {
		fail("assign role permissions", err)
	}

	seedPassword := os.Getenv("SEED_ADMIN_PASSWORD")
	if seedPassword == "" {
		seedPassword = defaultPassword
	}
	if len(seedPassword) < 8 {
		fail("seed admin password must be at least 8 characters", nil)
	}

	var existingUserID string
	lookupErr := tx.QueryRow(ctx, `
		SELECT id::text FROM erp.users
		WHERE tenant_id = $1 AND username = 'admin'`, defaultTenantID).Scan(&existingUserID)
	userCreated := errors.Is(lookupErr, pgx.ErrNoRows)
	if lookupErr != nil && !userCreated {
		fail("check admin user", lookupErr)
	}

	passwordHash, err := security.HashPassword(seedPassword)
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

	for _, rule := range defaultCodeRules {
		if _, err := tx.Exec(ctx, `
			INSERT INTO erp.code_rules (
				id, tenant_id, entity, prefix, date_pattern, separator,
				sequence_length, reset_policy, status
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'active')
			ON CONFLICT (tenant_id, entity) DO NOTHING`,
			uuid.NewString(), defaultTenantID, rule.Entity, rule.Prefix, rule.DatePattern,
			rule.Separator, rule.SequenceLength, rule.ResetPolicy,
		); err != nil {
			fail("seed code rule", err)
		}
	}

	if err := seedGL(ctx, tx, defaultTenantID); err != nil {
		fail("seed general ledger", err)
	}

	if err := tx.Commit(ctx); err != nil {
		fail("commit seed transaction", err)
	}

	if userCreated {
		fmt.Printf("seed completed: tenant=default username=admin password=%s\n", seedPassword)
		fmt.Println("warning: change the default password before production use")
	} else {
		fmt.Println("seed completed: tenant=default username=admin password=unchanged")
	}
}

func seedGL(ctx context.Context, tx pgx.Tx, tenantID string) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.gl_accounts (
			id, tenant_id, code, name, type, direction, level, is_leaf, status
		)
		SELECT gen_random_uuid(), $1, seed.code, seed.name, seed.type, seed.direction,
		       seed.level, seed.is_leaf, 'active'
		FROM (
			VALUES
				('1000', '资产', 'asset', 'debit', 1, false),
				('1001', '库存现金', 'asset', 'debit', 2, true),
				('1002', '银行存款', 'asset', 'debit', 2, true),
				('1122', '应收账款', 'asset', 'debit', 2, true),
				('1405', '库存商品', 'asset', 'debit', 2, true),
				('1601', '固定资产', 'asset', 'debit', 2, true),
				('2000', '负债', 'liability', 'credit', 1, false),
				('2202', '应付账款', 'liability', 'credit', 2, true),
				('2211', '应付职工薪酬', 'liability', 'credit', 2, true),
				('2221', '应交税费', 'liability', 'credit', 2, false),
				('222101', '应交增值税-进项税额', 'liability', 'debit', 3, true),
				('222102', '应交增值税-销项税额', 'liability', 'credit', 3, true),
				('4000', '所有者权益', 'equity', 'credit', 1, false),
				('4001', '实收资本', 'equity', 'credit', 2, true),
				('4103', '本年利润', 'equity', 'credit', 2, true),
				('4104', '利润分配', 'equity', 'credit', 2, true),
				('6000', '收入', 'revenue', 'credit', 1, false),
				('6001', '主营业务收入', 'revenue', 'credit', 2, true),
				('6051', '其他业务收入', 'revenue', 'credit', 2, true),
				('6400', '成本费用', 'expense', 'debit', 1, false),
				('6401', '主营业务成本', 'expense', 'debit', 2, true),
				('6601', '销售费用', 'expense', 'debit', 2, true),
				('6602', '管理费用', 'expense', 'debit', 2, true),
				('6603', '财务费用', 'expense', 'debit', 2, false),
				('660301', '汇兑损失', 'expense', 'debit', 3, true),
				('660302', '汇兑收益', 'expense', 'credit', 3, true),
				('6801', '所得税费用', 'expense', 'debit', 2, true)
		) AS seed(code, name, type, direction, level, is_leaf)
		ON CONFLICT (tenant_id, code) DO NOTHING`, tenantID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE erp.gl_accounts child
		SET parent_id = parent.id
		FROM erp.gl_accounts parent
		WHERE child.tenant_id = parent.tenant_id
		  AND (
		      (parent.code = '1000' AND child.code IN ('1001', '1002', '1122', '1405', '1601')) OR
		      (parent.code = '2000' AND child.code IN ('2202', '2211', '2221')) OR
		      (parent.code = '2221' AND child.code IN ('222101', '222102')) OR
		      (parent.code = '4000' AND child.code IN ('4001', '4103', '4104')) OR
		      (parent.code = '6000' AND child.code IN ('6001', '6051')) OR
		      (parent.code = '6400' AND child.code IN ('6401', '6601', '6602', '6603', '6801')) OR
		      (parent.code = '6603' AND child.code IN ('660301', '660302'))
		  )
		  AND child.parent_id IS NULL
		  AND child.tenant_id = $1`, tenantID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.gl_account_mappings (tenant_id, key, account_code)
		SELECT $1, mapping.key, mapping.account_code
		FROM (
			VALUES
				('ar', '1122'),
				('ap', '2202'),
				('revenue', '6001'),
				('inventory', '1405'),
				('cogs', '6401'),
				('bank', '1002'),
				('cash', '1001'),
				('profit', '4103'),
				('retained_earnings', '4104'),
				('input_vat', '222101'),
				('output_vat', '222102'),
				('fx_loss', '660301'),
				('fx_gain', '660302')
		) AS mapping(key, account_code)
		ON CONFLICT DO NOTHING`, tenantID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO erp.gl_periods (id, tenant_id, year, month, start_date, end_date, status)
		SELECT gen_random_uuid(), $1, y.year, m.month,
		       make_date(y.year, m.month, 1),
		       (make_date(y.year, m.month, 1) + interval '1 month - 1 day')::date,
		       'open'
		FROM (SELECT extract(year FROM current_date)::int AS year) y
		CROSS JOIN generate_series(1, 12) AS m(month)
		ON CONFLICT (tenant_id, year, month) DO NOTHING`, tenantID); err != nil {
		return err
	}
	return nil
}

func fail(message string, err error) {
	if err != nil {
		slog.Error(message, "error", err)
	} else {
		slog.Error(message)
	}
	os.Exit(1)
}
