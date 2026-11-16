package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	masterapp "github.com/mlf-erp/mlf-erp/internal/modules/masterdata/app"
	masterdomain "github.com/mlf-erp/mlf-erp/internal/modules/masterdata/domain"
	masterpostgres "github.com/mlf-erp/mlf-erp/internal/modules/masterdata/infra/postgres"
	purchaseapp "github.com/mlf-erp/mlf-erp/internal/modules/purchase/app"
	purchasepostgres "github.com/mlf-erp/mlf-erp/internal/modules/purchase/infra/postgres"
	systemapp "github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	systemdomain "github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
	systempostgres "github.com/mlf-erp/mlf-erp/internal/modules/system/infra/postgres"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/config"
	"github.com/mlf-erp/mlf-erp/internal/platform/database"
	"github.com/mlf-erp/mlf-erp/internal/platform/logger"
	"github.com/mlf-erp/mlf-erp/internal/platform/tenant"
	"github.com/shopspring/decimal"
)

const (
	defaultOrgID   = "00000000-0000-0000-0000-000000000002"
	adminUserID    = "00000000-0000-0000-0000-000000000004"
	demoPassword   = "Demo@123456"
	demoRoleCode   = "demo_viewer"
	demoRoleName   = "演示只读角色"
	companyOrgCode = "DEPT-EAST"
	southOrgCode   = "DEPT-SOUTH"
)

var demoUnits = []struct {
	Code      string
	Name      string
	Precision int
}{
	{"PCS", "件", 0},
	{"KG", "千克", 3},
	{"M", "米", 2},
	{"SET", "套", 0},
}

var demoCurrencies = []struct {
	Code          string
	Name          string
	Symbol        string
	DecimalPlaces int
}{
	{"CNY", "人民币", "¥", 2},
	{"USD", "美元", "$", 2},
}

var demoTaxRates = []struct {
	Code string
	Name string
	Rate string
	Type string
}{
	{"VAT13", "增值税13%", "0.13", "vat_output"},
	{"VAT9", "增值税9%", "0.09", "vat_output"},
	{"VAT6", "增值税6%", "0.06", "vat_output"},
	{"VAT0", "免税", "0", "vat_output"},
}

var demoWarehouses = []struct {
	Code    string
	Name    string
	Type    string
	OrgCode string
}{
	{"WH-SH", "上海成品仓", "normal", "HQ"},
	{"WH-BJ", "北京原料仓", "normal", "HQ"},
	{"WH-EAST", "华东周转仓", "normal", companyOrgCode},
	{"WH-CONSIGN", "寄售仓", "consignment", "HQ"},
}

var demoPartners = []struct {
	Code    string
	Name    string
	Type    string
	OrgCode string
	Contact string
	Phone   string
	Email   string
}{
	{"CUS-DEMO-001", "上海客户甲", "customer", "HQ", "张经理", "13800000001", "customer1@example.com"},
	{"CUS-DEMO-002", "上海客户乙", "customer", "HQ", "李经理", "13800000002", "customer2@example.com"},
	{"CUS-DEMO-003", "上海客户丙", "customer", "HQ", "王经理", "13800000003", "customer3@example.com"},
	{"SUP-DEMO-001", "苏州供应商甲", "supplier", "HQ", "赵经理", "13900000001", "supplier1@example.com"},
	{"SUP-DEMO-002", "苏州供应商乙", "supplier", "HQ", "钱经理", "13900000002", "supplier2@example.com"},
	{"SUP-DEMO-003", "无锡供应商丙", "supplier", "HQ", "孙经理", "13900000003", "supplier3@example.com"},
	{"BOTH-DEMO-001", "综合客商甲", "both", "HQ", "周经理", "13700000001", "both1@example.com"},
	{"BOTH-DEMO-002", "综合客商乙", "both", "HQ", "吴经理", "13700000002", "both2@example.com"},
	{"CUS-EAST-001", "杭州客户甲", "customer", companyOrgCode, "郑经理", "13600000001", "east-customer1@example.com"},
	{"CUS-EAST-002", "南京客户乙", "customer", companyOrgCode, "冯经理", "13600000002", "east-customer2@example.com"},
	{"SUP-SOUTH-001", "广州供应商甲", "supplier", southOrgCode, "陈经理", "13500000001", "south-supplier1@example.com"},
	{"SUP-SOUTH-002", "深圳供应商乙", "supplier", southOrgCode, "褚经理", "13500000002", "south-supplier2@example.com"},
}

var demoMaterials = []struct {
	Code     string
	Name     string
	Spec     string
	Category string
	OrgCode  string
	UnitCode string
	TaxCode  string
}{
	{"MAT-DEMO-001", "原材料 A", "规格 A", "raw", "HQ", "KG", "VAT13"},
	{"MAT-DEMO-002", "原材料 B", "规格 B", "raw", "HQ", "KG", "VAT13"},
	{"MAT-DEMO-003", "原材料 C", "规格 C", "raw", "HQ", "KG", "VAT13"},
	{"MAT-DEMO-004", "原材料 D", "规格 D", "raw", "HQ", "KG", "VAT13"},
	{"MAT-DEMO-005", "原材料 E", "规格 E", "raw", "HQ", "KG", "VAT13"},
	{"MAT-DEMO-006", "半成品 A", "规格 A", "semi", "HQ", "PCS", "VAT13"},
	{"MAT-DEMO-007", "半成品 B", "规格 B", "semi", "HQ", "PCS", "VAT13"},
	{"MAT-DEMO-008", "半成品 C", "规格 C", "semi", "HQ", "PCS", "VAT13"},
	{"MAT-DEMO-009", "半成品 D", "规格 D", "semi", "HQ", "PCS", "VAT13"},
	{"MAT-DEMO-010", "半成品 E", "规格 E", "semi", "HQ", "PCS", "VAT13"},
	{"MAT-DEMO-011", "成品 A", "规格 A", "finished", companyOrgCode, "PCS", "VAT13"},
	{"MAT-DEMO-012", "成品 B", "规格 B", "finished", companyOrgCode, "PCS", "VAT13"},
	{"MAT-DEMO-013", "成品 C", "规格 C", "finished", companyOrgCode, "PCS", "VAT13"},
	{"MAT-DEMO-014", "成品 D", "规格 D", "finished", companyOrgCode, "PCS", "VAT13"},
	{"MAT-DEMO-015", "成品 E", "规格 E", "finished", companyOrgCode, "PCS", "VAT13"},
	{"MAT-DEMO-016", "包装材料 A", "规格 A", "packaging", southOrgCode, "SET", "VAT9"},
	{"MAT-DEMO-017", "包装材料 B", "规格 B", "packaging", southOrgCode, "SET", "VAT9"},
	{"MAT-DEMO-018", "包装材料 C", "规格 C", "packaging", southOrgCode, "SET", "VAT9"},
	{"MAT-DEMO-019", "包装材料 D", "规格 D", "packaging", southOrgCode, "SET", "VAT9"},
	{"MAT-DEMO-020", "包装材料 E", "规格 E", "packaging", southOrgCode, "SET", "VAT9"},
}

var demoViewerPermissions = []string{
	"masterdata.partner.read",
	"masterdata.material.read",
	"masterdata.warehouse.read",
	"masterdata.unit.read",
	"masterdata.currency.read",
	"masterdata.tax.read",
	"purchase.order.read",
	"purchase.read",
	"inventory.balance.read",
	"inventory.transaction.read",
}

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	if cfg.App.Env != "dev" && cfg.App.Env != "test" && os.Getenv("ALLOW_DEMO_SEED") != "true" {
		slog.Error("demo seed is only allowed in dev/test; set ALLOW_DEMO_SEED=true to override")
		os.Exit(1)
	}
	if !cfg.DB.Enabled {
		slog.Error("database is required; set DB_ENABLED=true")
		os.Exit(1)
	}

	log := logger.New(cfg.Log.Level)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	db, err := database.NewPostgres(ctx, cfg.DB)
	if err != nil {
		log.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := seedDemo(ctx, cfg, db, log); err != nil {
		log.Error("seed demo data", "error", err)
		os.Exit(1)
	}
}

func seedDemo(ctx context.Context, cfg *config.Config, db *pgxpool.Pool, log *slog.Logger) error {
	tenantID := cfg.App.DefaultTenantID
	baseCtx := tenant.With(ctx, tenant.Info{ID: tenantID, OrgID: defaultOrgID})
	baseCtx = auth.WithActor(baseCtx, auth.Actor{
		UserID:       adminUserID,
		UserName:     "admin",
		TenantID:     tenantID,
		OrgID:        defaultOrgID,
		DataScope:    "all",
		IsSuperAdmin: true,
	})

	userRepository := systempostgres.NewUserRepository(db)
	roleRepository := systempostgres.NewRoleRepository(db)
	orgRepository := systempostgres.NewOrgRepository(db)
	refreshTokenRepository := systempostgres.NewRefreshTokenRepository(db)
	codeRuleRepository := systempostgres.NewCodeRuleRepository(db)

	userService := systemapp.NewUserService(userRepository, roleRepository, refreshTokenRepository)
	roleService := systemapp.NewRoleService(roleRepository)
	orgService := systemapp.NewOrgService(orgRepository)
	codeRuleService := systemapp.NewCodeRuleService(codeRuleRepository)

	partnerRepository := masterpostgres.NewRepository[masterdomain.Partner](db)
	materialRepository := masterpostgres.NewRepository[masterdomain.Material](db)
	warehouseRepository := masterpostgres.NewRepository[masterdomain.Warehouse](db)
	unitRepository := masterpostgres.NewRepository[masterdomain.Unit](db)
	currencyRepository := masterpostgres.NewRepository[masterdomain.Currency](db)
	taxRateRepository := masterpostgres.NewRepository[masterdomain.TaxRate](db)

	partnerService := masterapp.NewPartnerService(partnerRepository, codeRuleService)
	materialService := masterapp.NewMaterialService(materialRepository, unitRepository, taxRateRepository, codeRuleService)
	warehouseService := masterapp.NewWarehouseService(warehouseRepository, codeRuleService)
	unitService := masterapp.NewUnitService(unitRepository)
	currencyService := masterapp.NewCurrencyService(currencyRepository)
	taxRateService := masterapp.NewTaxRateService(taxRateRepository)

	purchaseRepository := purchasepostgres.NewRepository(db)
	orderService := purchaseapp.NewOrderService(
		purchaseRepository,
		codeRuleService,
		partnerRepository,
		materialRepository,
		warehouseRepository,
	)
	receiptService := purchaseapp.NewReceiptService(purchaseRepository, codeRuleService)

	orgIDs := map[string]string{"HQ": defaultOrgID}

	east, err := ensureOrg(baseCtx, orgService, tenantID, defaultOrgID, companyOrgCode, "华东事业部")
	if err != nil {
		return err
	}
	orgIDs[companyOrgCode] = east.ID

	south, err := ensureOrg(baseCtx, orgService, tenantID, defaultOrgID, southOrgCode, "华南事业部")
	if err != nil {
		return err
	}
	orgIDs[southOrgCode] = south.ID

	unitIDs := make(map[string]string, len(demoUnits))
	for _, spec := range demoUnits {
		unit, err := ensureUnit(baseCtx, unitService, spec.Code, spec.Name, spec.Precision)
		if err != nil {
			return err
		}
		unitIDs[spec.Code] = unit.ID
	}

	for _, spec := range demoCurrencies {
		if _, err := ensureCurrency(baseCtx, currencyService, spec.Code, spec.Name, spec.Symbol, spec.DecimalPlaces); err != nil {
			return err
		}
	}

	taxRateIDs := make(map[string]string, len(demoTaxRates))
	for _, spec := range demoTaxRates {
		taxRate, err := ensureTaxRate(baseCtx, taxRateService, spec.Code, spec.Name, spec.Rate, spec.Type)
		if err != nil {
			return err
		}
		taxRateIDs[spec.Code] = taxRate.ID
	}

	for _, spec := range demoWarehouses {
		orgCtx := withOrg(baseCtx, tenantID, orgIDs[spec.OrgCode])
		if _, err := ensureWarehouse(orgCtx, warehouseService, spec.Code, spec.Name, spec.Type); err != nil {
			return err
		}
	}

	for _, spec := range demoPartners {
		orgCtx := withOrg(baseCtx, tenantID, orgIDs[spec.OrgCode])
		if _, err := ensurePartner(orgCtx, partnerService, spec.Code, spec.Name, spec.Type, spec.Contact, spec.Phone, spec.Email); err != nil {
			return err
		}
	}

	for _, spec := range demoMaterials {
		orgCtx := withOrg(baseCtx, tenantID, orgIDs[spec.OrgCode])
		if _, err := ensureMaterial(
			orgCtx,
			materialService,
			spec.Code,
			spec.Name,
			spec.Spec,
			spec.Category,
			unitIDs[spec.UnitCode],
			taxRateIDs[spec.TaxCode],
		); err != nil {
			return err
		}
	}

	demoRole, err := ensureDemoRole(baseCtx, roleService, tenantID)
	if err != nil {
		return err
	}
	adminRole, err := findRoleByCode(baseCtx, roleService, tenantID, "admin")
	if err != nil {
		return err
	}
	if adminRole == nil {
		return fmt.Errorf("admin role not found; run cmd/seed first")
	}

	demoUsers := []struct {
		Username    string
		DisplayName string
		Email       string
		OrgCode     string
		RoleID      string
		DataScope   string
		SuperAdmin  bool
	}{
		{"demo_admin", "演示管理员", "demo-admin@example.com", "HQ", adminRole.ID, "all", true},
		{"demo_company", "演示公司用户", "demo-company@example.com", companyOrgCode, demoRole.ID, "company", false},
		{"demo_dept", "演示部门用户", "demo-dept@example.com", companyOrgCode, demoRole.ID, "dept", false},
	}

	for _, spec := range demoUsers {
		if _, err := ensureDemoUser(
			baseCtx,
			db,
			userService,
			tenantID,
			spec.Username,
			spec.DisplayName,
			spec.Email,
			orgIDs[spec.OrgCode],
			spec.RoleID,
			spec.DataScope,
			spec.SuperAdmin,
		); err != nil {
			return err
		}
	}

	if err := ensureDemoPurchases(baseCtx, db, tenantID, orderService, receiptService, partnerService, materialService, warehouseService); err != nil {
		return err
	}

	log.Info("demo data ready",
		"units", len(demoUnits),
		"currencies", len(demoCurrencies),
		"tax_rates", len(demoTaxRates),
		"warehouses", len(demoWarehouses),
		"partners", len(demoPartners),
		"materials", len(demoMaterials),
		"users", len(demoUsers),
	)

	fmt.Println("demo seed completed")
	fmt.Println("demo accounts (dev/test only):")
	fmt.Printf("  demo_admin   / %s  data_scope=all\n", demoPassword)
	fmt.Printf("  demo_company / %s  data_scope=company\n", demoPassword)
	fmt.Printf("  demo_dept    / %s  data_scope=dept\n", demoPassword)
	return nil
}

func withOrg(ctx context.Context, tenantID, orgID string) context.Context {
	return tenant.With(ctx, tenant.Info{ID: tenantID, OrgID: orgID})
}

func ensureOrg(ctx context.Context, service *systemapp.OrgService, tenantID, parentID, code, name string) (*systemdomain.Org, error) {
	tree, err := service.ListTree(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if existing := findOrgByCode(tree, code); existing != nil {
		return existing, nil
	}
	org, err := service.Create(ctx, systemapp.CreateOrgInput{
		TenantID: tenantID,
		ParentID: parentID,
		Type:     systemdomain.OrgTypeDept,
		Code:     code,
		Name:     name,
	})
	if err != nil {
		return nil, err
	}
	return org, nil
}

func findOrgByCode(orgs []*systemdomain.Org, code string) *systemdomain.Org {
	for _, org := range orgs {
		if org.Code == code {
			return org
		}
		if found := findOrgByCode(org.Children, code); found != nil {
			return found
		}
	}
	return nil
}

func ensureUnit(ctx context.Context, service *masterapp.UnitService, code, name string, precision int) (*masterdomain.Unit, error) {
	items, _, err := service.List(ctx, code, 1, 50)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Code == code {
			return &items[i], nil
		}
	}
	unit, err := service.Create(ctx, masterapp.UnitInput{Code: code, Name: name, Precision: precision})
	if err != nil {
		return nil, err
	}
	return &unit, nil
}

func ensureCurrency(ctx context.Context, service *masterapp.CurrencyService, code, name, symbol string, decimalPlaces int) (*masterdomain.Currency, error) {
	items, _, err := service.List(ctx, code, 1, 50)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Code == code {
			return &items[i], nil
		}
	}
	currency, err := service.Create(ctx, masterapp.CurrencyInput{
		Code: code, Name: name, Symbol: symbol, DecimalPlaces: decimalPlaces,
	})
	if err != nil {
		return nil, err
	}
	return &currency, nil
}

func ensureTaxRate(ctx context.Context, service *masterapp.TaxRateService, code, name, rate, taxType string) (*masterdomain.TaxRate, error) {
	items, _, err := service.List(ctx, code, 1, 50)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Code == code {
			return &items[i], nil
		}
	}
	taxRate, err := service.Create(ctx, masterapp.TaxRateInput{
		Code: code, Name: name, Rate: rate, Type: taxType,
	})
	if err != nil {
		return nil, err
	}
	return &taxRate, nil
}

func ensureWarehouse(ctx context.Context, service *masterapp.WarehouseService, code, name, warehouseType string) (*masterdomain.Warehouse, error) {
	items, _, err := service.List(ctx, code, 1, 50)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Code == code {
			return &items[i], nil
		}
	}
	warehouse, err := service.Create(ctx, masterapp.WarehouseInput{Code: code, Name: name, Type: warehouseType})
	if err != nil {
		return nil, err
	}
	return &warehouse, nil
}

func ensurePartner(ctx context.Context, service *masterapp.PartnerService, code, name, partnerType, contact, phone, email string) (*masterdomain.Partner, error) {
	items, _, err := service.List(ctx, code, 1, 50)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Code == code {
			return &items[i], nil
		}
	}
	partner, err := service.Create(ctx, masterapp.PartnerInput{
		Code: code, Name: name, Type: partnerType, Contact: contact, Phone: phone, Email: email,
	})
	if err != nil {
		return nil, err
	}
	return &partner, nil
}

func ensureMaterial(ctx context.Context, service *masterapp.MaterialService, code, name, spec, category, unitID, taxRateID string) (*masterdomain.Material, error) {
	items, _, err := service.List(ctx, code, 1, 50)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Code == code {
			return &items[i], nil
		}
	}
	material, err := service.Create(ctx, masterapp.MaterialInput{
		Code: code, Name: name, Spec: spec, Category: category, UnitID: unitID, TaxRateID: taxRateID,
	})
	if err != nil {
		return nil, err
	}
	return &material, nil
}

func ensureDemoRole(ctx context.Context, service *systemapp.RoleService, tenantID string) (*systemdomain.Role, error) {
	role, err := findRoleByCode(ctx, service, tenantID, demoRoleCode)
	if err != nil {
		return nil, err
	}
	if role == nil {
		created, createErr := service.Create(ctx, systemapp.CreateRoleInput{
			TenantID: tenantID,
			Code:     demoRoleCode,
			Name:     demoRoleName,
		})
		if createErr != nil {
			return nil, createErr
		}
		role = created
	}
	if err := service.SetPermissions(ctx, tenantID, role.ID, demoViewerPermissions); err != nil {
		return nil, err
	}
	role.Permissions = append([]string(nil), demoViewerPermissions...)
	return role, nil
}

func findRoleByCode(ctx context.Context, service *systemapp.RoleService, tenantID, code string) (*systemdomain.Role, error) {
	roles, err := service.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		if role.Code == code {
			return role, nil
		}
	}
	return nil, nil
}

func ensureDemoPurchases(
	ctx context.Context,
	db *pgxpool.Pool,
	tenantID string,
	orderService *purchaseapp.OrderService,
	receiptService *purchaseapp.ReceiptService,
	partnerService *masterapp.PartnerService,
	materialService *masterapp.MaterialService,
	warehouseService *masterapp.WarehouseService,
) error {
	supplier, err := findPartnerByCode(ctx, partnerService, "SUP-DEMO-001")
	if err != nil {
		return err
	}
	warehouse, err := findWarehouseByCode(ctx, warehouseService, "WH-SH")
	if err != nil {
		return err
	}
	material1, err := findMaterialByCode(ctx, materialService, "MAT-DEMO-001")
	if err != nil {
		return err
	}
	material2, err := findMaterialByCode(ctx, materialService, "MAT-DEMO-002")
	if err != nil {
		return err
	}

	if err := ensureDemoOrder(ctx, db, tenantID, orderService, receiptService,
		"demo-seed-order-1", supplier.ID, warehouse.ID, material1.ID,
		"10", "100.00", "4",
	); err != nil {
		return err
	}
	return ensureDemoOrder(ctx, db, tenantID, orderService, receiptService,
		"demo-seed-order-2", supplier.ID, warehouse.ID, material2.ID,
		"5", "50.00", "5",
	)
}

func ensureDemoOrder(
	ctx context.Context,
	db *pgxpool.Pool,
	tenantID string,
	orderService *purchaseapp.OrderService,
	receiptService *purchaseapp.ReceiptService,
	remark, supplierID, warehouseID, materialID, quantity, unitPrice, receiveQuantity string,
) error {
	var count int
	if err := db.QueryRow(ctx, `
		SELECT count(*) FROM erp.purchase_orders
		WHERE tenant_id = $1 AND remark = $2`, tenantID, remark).Scan(&count); err != nil {
		return fmt.Errorf("check demo order: %w", err)
	}
	if count > 0 {
		return nil
	}

	order, err := orderService.Create(ctx, purchaseapp.OrderInput{
		SupplierID:  supplierID,
		WarehouseID: warehouseID,
		OrderDate:   time.Now(),
		Currency:    "CNY",
		Remark:      remark,
		Lines: []purchaseapp.OrderLineInput{{
			MaterialID: materialID,
			Quantity:   decimal.RequireFromString(quantity),
			UnitPrice:  decimal.RequireFromString(unitPrice),
		}},
	})
	if err != nil {
		return err
	}
	if err := orderService.Submit(ctx, order.ID); err != nil {
		return err
	}
	if err := orderService.Approve(ctx, order.ID); err != nil {
		return err
	}
	if _, err := receiptService.Receive(ctx, order.ID, purchaseapp.ReceiveInput{
		ReceiptDate: time.Now(),
		Remark:      remark + "-receipt",
		Lines: []purchaseapp.ReceiveLineInput{{
			OrderLineID: order.Lines[0].ID,
			Quantity:    decimal.RequireFromString(receiveQuantity),
		}},
	}); err != nil {
		return err
	}
	return nil
}

func findPartnerByCode(ctx context.Context, service *masterapp.PartnerService, code string) (*masterdomain.Partner, error) {
	items, _, err := service.List(ctx, code, 1, 50)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Code == code {
			return &items[i], nil
		}
	}
	return nil, fmt.Errorf("demo partner %s not found", code)
}

func findMaterialByCode(ctx context.Context, service *masterapp.MaterialService, code string) (*masterdomain.Material, error) {
	items, _, err := service.List(ctx, code, 1, 50)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Code == code {
			return &items[i], nil
		}
	}
	return nil, fmt.Errorf("demo material %s not found", code)
}

func findWarehouseByCode(ctx context.Context, service *masterapp.WarehouseService, code string) (*masterdomain.Warehouse, error) {
	items, _, err := service.List(ctx, code, 1, 50)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Code == code {
			return &items[i], nil
		}
	}
	return nil, fmt.Errorf("demo warehouse %s not found", code)
}

func ensureDemoUser(
	ctx context.Context,
	db *pgxpool.Pool,
	service *systemapp.UserService,
	tenantID, username, displayName, email, orgID, roleID, dataScope string,
	superAdmin bool,
) (*systemdomain.User, error) {
	users, _, err := service.List(ctx, tenantID, username, 1, 50)
	if err != nil {
		return nil, err
	}

	var user *systemdomain.User
	for i := range users {
		if users[i].Username == username {
			user = users[i]
			break
		}
	}

	if user == nil {
		created, createErr := service.Create(ctx, systemapp.CreateUserInput{
			TenantID:     tenantID,
			OrgID:        orgID,
			Username:     username,
			DisplayName:  displayName,
			Email:        email,
			Password:     demoPassword,
			IsSuperAdmin: superAdmin,
			RoleIDs:      []string{roleID},
		})
		if createErr != nil {
			return nil, createErr
		}
		user = created
	} else {
		updated, updateErr := service.Update(ctx, tenantID, user.ID, systemapp.UpdateUserInput{
			OrgID:        orgID,
			DisplayName:  displayName,
			Email:        email,
			Status:       systemdomain.UserStatusActive,
			IsSuperAdmin: superAdmin,
			Password:     demoPassword,
		})
		if updateErr != nil {
			return nil, updateErr
		}
		user = updated
		if err := service.SetRoles(ctx, tenantID, user.ID, []string{roleID}); err != nil {
			return nil, err
		}
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO erp.org_memberships (tenant_id, user_id, org_id, role_id, data_scope)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, user_id, org_id) DO UPDATE
		SET role_id = EXCLUDED.role_id, data_scope = EXCLUDED.data_scope`,
		tenantID, user.ID, orgID, roleID, dataScope,
	); err != nil {
		return nil, fmt.Errorf("set demo user membership: %w", err)
	}

	return user, nil
}
