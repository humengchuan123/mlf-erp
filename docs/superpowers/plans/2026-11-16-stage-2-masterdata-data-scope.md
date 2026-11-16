# Stage 2 主数据、编码规则与数据权限 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Stage 1 平台底座上实现主数据（客商、物料、仓库、计量单位、币种、税率）、编码规则和数据权限过滤，并完成前端主数据管理页面、接口测试和代码审核。

**Architecture:** 新增 `internal/modules/masterdata` 模块，按 `domain / app / infra / http` 分层；编码规则放在 `system` 模块；数据权限通过 `auth.Actor` 的 `DataScope/OrgPath/CompanyPath` 与 `internal/platform/scope` 过滤器落到 SQL；前端使用一个可复用的 `MasterDataCrud` 组件承载六类主数据页面。

**Tech Stack:** Go 1.25 + Gin + pgx + PostgreSQL + shopspring/decimal；Vue 3 + TypeScript + Element Plus。

---

## 阶段范围

### 主数据

- 客商 `partners`：客户/供应商/两者，编码、名称、分类、联系人、电话、邮箱、地址、税号、状态。
- 物料 `materials`：编码、名称、规格、分类、计量单位、税率、条码、状态。
- 仓库 `warehouses`：编码、名称、类型、地址、状态。
- 计量单位 `units`：编码、名称、精度、状态。
- 币种 `currencies`：编码、名称、符号、小数位、状态。
- 税率 `tax_rates`：编码、名称、税率、类型、状态。

### 编码规则

- `code_rules`：实体、前缀、日期模式、分隔符、流水号长度、当前值、重置策略、状态。
- 生成器：事务 + `FOR UPDATE` 保证并发安全。
- 默认规则：客商、物料、仓库、采购订单。

### 数据权限

- `all`：全租户。
- `company`：公司子树。
- `dept`：部门子树。
- `self`：仅本人创建。
- 应用到客商、物料、仓库的列表、详情、更新、删除。
- 单位、币种、税率是租户级参考数据，只做租户隔离。

### 前端

- 主数据菜单：客商、物料、仓库、计量单位、币种、税率。
- 系统菜单增加编码规则。
- 复用 `MasterDataCrud` 组件完成列表、搜索、分页、新建、编辑、禁用。
- 物料页面联动单位/税率选项。

---

## File Structure

```text
db/migrations/
├─ 000003_masterdata_code_rules.up.sql
└─ 000003_masterdata_code_rules.down.sql

internal/platform/scope/
└─ scope.go

internal/platform/auth/context.go          # 增加 DataScope/OrgPath/CompanyPath
internal/platform/authmw/middleware.go     # 加载数据范围与组织路径

internal/modules/system/
├─ domain/code_rule.go
├─ domain/repositories.go                  # 增加 CodeRuleRepository / 数据范围方法
├─ infra/postgres/code_rule_repository.go
├─ app/code_rule_service.go
└─ http/code_rule_handler.go

internal/modules/masterdata/
├─ domain/
│  ├─ partner.go
│  ├─ material.go
│  ├─ warehouse.go
│  ├─ unit.go
│  ├─ currency.go
│  ├─ tax_rate.go
│  └─ repositories.go
├─ app/
│  ├─ partner_service.go
│  ├─ material_service.go
│  ├─ warehouse_service.go
│  ├─ unit_service.go
│  ├─ currency_service.go
│  ├─ tax_rate_service.go
│  └─ code_generator.go
├─ infra/postgres/
│  ├─ partner_repository.go
│  ├─ material_repository.go
│  ├─ warehouse_repository.go
│  ├─ unit_repository.go
│  ├─ currency_repository.go
│  └─ tax_rate_repository.go
└─ http/
   ├─ partner_handler.go
   ├─ material_handler.go
   ├─ warehouse_handler.go
   ├─ unit_handler.go
   ├─ currency_handler.go
   ├─ tax_rate_handler.go
   └─ dto.go

apps/web/src/
├─ components/MasterDataCrud.vue
├─ views/masterdata/PartnerListView.vue
├─ views/masterdata/MaterialListView.vue
├─ views/masterdata/WarehouseListView.vue
├─ views/masterdata/UnitListView.vue
├─ views/masterdata/CurrencyListView.vue
├─ views/masterdata/TaxRateListView.vue
├─ views/system/CodeRuleView.vue
├─ router/index.ts
└─ layouts/DefaultLayout.vue

packages/sdk/src/
├─ masterdata.ts
├─ system.ts                               # 增加 codeRules
└─ index.ts
```

---

## Task 1: 数据库迁移 000003

**Files:**
- Create: `db/migrations/000003_masterdata_code_rules.up.sql`
- Create: `db/migrations/000003_masterdata_code_rules.down.sql`

- [ ] **Step 1: 建表**

```sql
CREATE TABLE IF NOT EXISTS erp.units (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    code text NOT NULL,
    name text NOT NULL,
    precision integer NOT NULL DEFAULT 0,
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT units_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT units_tenant_id_key UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.currencies (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    code text NOT NULL,
    name text NOT NULL,
    symbol text NOT NULL DEFAULT '',
    decimal_places integer NOT NULL DEFAULT 2,
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT currencies_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT currencies_tenant_id_key UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.tax_rates (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    code text NOT NULL,
    name text NOT NULL,
    rate numeric(9,4) NOT NULL DEFAULT 0,
    type text NOT NULL DEFAULT 'vat_output',
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tax_rates_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT tax_rates_tenant_id_key UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.partners (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    org_id uuid NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    type text NOT NULL DEFAULT 'both',
    category text NOT NULL DEFAULT '',
    contact text NOT NULL DEFAULT '',
    phone text NOT NULL DEFAULT '',
    email text NOT NULL DEFAULT '',
    address text NOT NULL DEFAULT '',
    tax_no text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    created_by uuid,
    updated_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT partners_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT partners_org_fk FOREIGN KEY (tenant_id, org_id)
        REFERENCES erp.organizations (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.materials (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    org_id uuid NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    spec text NOT NULL DEFAULT '',
    category text NOT NULL DEFAULT '',
    unit_id uuid,
    tax_rate_id uuid,
    barcode text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    created_by uuid,
    updated_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT materials_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT materials_org_fk FOREIGN KEY (tenant_id, org_id)
        REFERENCES erp.organizations (tenant_id, id),
    CONSTRAINT materials_unit_fk FOREIGN KEY (tenant_id, unit_id)
        REFERENCES erp.units (tenant_id, id),
    CONSTRAINT materials_tax_rate_fk FOREIGN KEY (tenant_id, tax_rate_id)
        REFERENCES erp.tax_rates (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.warehouses (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    org_id uuid NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    type text NOT NULL DEFAULT 'normal',
    address text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    created_by uuid,
    updated_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT warehouses_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT warehouses_org_fk FOREIGN KEY (tenant_id, org_id)
        REFERENCES erp.organizations (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS erp.code_rules (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    entity text NOT NULL,
    prefix text NOT NULL,
    date_pattern text NOT NULL DEFAULT 'none',
    separator text NOT NULL DEFAULT '-',
    sequence_length integer NOT NULL DEFAULT 4,
    current_value bigint NOT NULL DEFAULT 0,
    reset_policy text NOT NULL DEFAULT 'never',
    last_reset_date date,
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT code_rules_tenant_entity_key UNIQUE (tenant_id, entity)
);

CREATE INDEX IF NOT EXISTS partners_scope_idx ON erp.partners (tenant_id, org_id, created_by, status);
CREATE INDEX IF NOT EXISTS materials_scope_idx ON erp.materials (tenant_id, org_id, created_by, status);
CREATE INDEX IF NOT EXISTS warehouses_scope_idx ON erp.warehouses (tenant_id, org_id, created_by, status);
```

新增权限：

```sql
INSERT INTO erp.permissions (code, name, module, type) VALUES
('masterdata.partner.read', '查看客商', 'masterdata', 'api'),
('masterdata.partner.write', '管理客商', 'masterdata', 'api'),
('masterdata.material.read', '查看物料', 'masterdata', 'api'),
('masterdata.material.write', '管理物料', 'masterdata', 'api'),
('masterdata.warehouse.read', '查看仓库', 'masterdata', 'api'),
('masterdata.warehouse.write', '管理仓库', 'masterdata', 'api'),
('masterdata.unit.read', '查看计量单位', 'masterdata', 'api'),
('masterdata.unit.write', '管理计量单位', 'masterdata', 'api'),
('masterdata.currency.read', '查看币种', 'masterdata', 'api'),
('masterdata.currency.write', '管理币种', 'masterdata', 'api'),
('masterdata.tax.read', '查看税率', 'masterdata', 'api'),
('masterdata.tax.write', '管理税率', 'masterdata', 'api'),
('system.code_rule.read', '查看编码规则', 'system', 'api'),
('system.code_rule.write', '管理编码规则', 'system', 'api')
ON CONFLICT (code) DO NOTHING;
```

- [ ] **Step 2: 回滚**

Down 删除新增表、索引和权限。

- [ ] **Step 3: 迁移验证**

```bash
go run ./cmd/migrate -direction up
go run ./cmd/migrate -direction down -steps 1
go run ./cmd/migrate -direction up
```

---

## Task 2: 数据权限基础

**Files:**
- Modify: `internal/platform/auth/context.go`
- Modify: `internal/modules/system/domain/repositories.go`
- Modify: `internal/modules/system/infra/postgres/user_repository.go`
- Modify: `internal/modules/system/infra/postgres/org_repository.go`
- Modify: `internal/platform/authmw/middleware.go`
- Create: `internal/platform/scope/scope.go`
- Modify: `cmd/api/main.go`
- Test: `internal/modules/system/app/fakes_test.go`

- [ ] **Step 1: Actor 扩展**

```go
type Actor struct {
    UserID       string
    UserName     string
    TenantID     string
    OrgID        string
    OrgPath      string
    CompanyPath  string
    DataScope    string
    Roles        []string
    Permissions  []string
    IsSuperAdmin bool
}
```

`DataScope` 取值：`all`、`company`、`dept`、`self`。

- [ ] **Step 2: 仓储方法**

`UserRepository` 增加：

```go
DataScopeByUserID(ctx context.Context, tenantID, userID, orgID string) (string, error)
```

SQL：

```sql
SELECT COALESCE((
    SELECT data_scope FROM erp.org_memberships
    WHERE tenant_id = $1 AND user_id = $2 AND org_id = $3
    LIMIT 1
), 'self')
```

`OrgRepository` 增加：

```go
AncestorByType(ctx context.Context, tenantID, orgID, orgType string) (*Org, error)
```

使用递归 CTE 查找最近的指定类型祖先。

- [ ] **Step 3: 认证中间件加载范围**

`authmw.Auth` 增加 `orgs domain.OrgRepository` 参数。登录用户为超级管理员时 `DataScope=all`；否则取 `org_memberships.data_scope`，默认 `self`。同时加载用户组织路径和公司祖先路径。

- [ ] **Step 4: ScopeFilter**

```go
package scope

type Filter struct {
    TenantID    string
    DataScope   string
    UserID      string
    OrgPath     string
    CompanyPath string
}

func FromActor(ctx context.Context, tenantID string) Filter
func (f Filter) AllowsOrgPath(orgPath string) bool
```

SQL 条件约定（主数据列表/详情/更新/删除共用）：

```sql
AND (
    $5 = 'all'
    OR ($5 = 'self' AND p.created_by = $6)
    OR ($5 = 'dept' AND EXISTS (
        SELECT 1 FROM erp.organizations o
        WHERE o.tenant_id = p.tenant_id AND o.id = p.org_id
          AND (o.path = $7 OR o.path LIKE $7 || '/%')
    ))
    OR ($5 = 'company' AND EXISTS (
        SELECT 1 FROM erp.organizations o
        WHERE o.tenant_id = p.tenant_id AND o.id = p.org_id
          AND (o.path = $8 OR o.path LIKE $8 || '/%')
    ))
)
```

- [ ] **Step 5: 测试**

覆盖：all/self/dept/company 的 SQL 参数与 `AllowsOrgPath` 逻辑；authmw 加载超级管理员 `all`。

---

## Task 3: 编码规则

**Files:**
- Create: `internal/modules/system/domain/code_rule.go`
- Modify: `internal/modules/system/domain/repositories.go`
- Create: `internal/modules/system/infra/postgres/code_rule_repository.go`
- Create: `internal/modules/system/app/code_rule_service.go`
- Create: `internal/modules/system/http/code_rule_handler.go`
- Test: `internal/modules/system/app/code_rule_service_test.go`

- [ ] **Step 1: 领域模型**

```go
type CodeRule struct {
    ID             string
    TenantID       string
    Entity         string
    Prefix         string
    DatePattern    string
    Separator      string
    SequenceLength int
    CurrentValue   int64
    ResetPolicy    string
    LastResetDate  *time.Time
    Status         string
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

- [ ] **Step 2: 仓储接口**

```go
type CodeRuleRepository interface {
    List(ctx context.Context, tenantID string) ([]*CodeRule, error)
    Create(ctx context.Context, rule *CodeRule) error
    Update(ctx context.Context, rule *CodeRule) error
    Next(ctx context.Context, tenantID, entity string) (string, error)
}
```

`Next` 在事务中 `SELECT ... FOR UPDATE`，按 `reset_policy` 重置流水号，生成编码并更新当前值。

- [ ] **Step 3: 服务与 Handler**

```go
type CodeRuleService struct { repo domain.CodeRuleRepository }
func (s *CodeRuleService) List(...)
func (s *CodeRuleService) Create(...)
func (s *CodeRuleService) Update(...)
func (s *CodeRuleService) Next(ctx context.Context, tenantID, entity string) (string, error)
```

API：

- `GET /api/v1/system/code-rules` → `system.code_rule.read`
- `POST /api/v1/system/code-rules` → `system.code_rule.write`
- `PUT /api/v1/system/code-rules/:id` → `system.code_rule.write`

- [ ] **Step 4: 测试**

- 日期模式 `yyyyMMdd` 生成正确。
- 流水号补零。
- 日重置策略在跨日时归零。
- 并发调用不产生重复编码（使用真实 PostgreSQL 测试，或至少验证 `FOR UPDATE` SQL）。
- 不存在规则返回 `ErrNotFound`。

---

## Task 4: 主数据领域与仓储

**Files:**
- Create: `internal/modules/masterdata/domain/*.go`
- Create: `internal/modules/masterdata/infra/postgres/*.go`

- [ ] **Step 1: 实体**

六个实体统一包含：ID、TenantID、OrgID（参考数据除外）、Code、Name、Status、CreatedBy、UpdatedBy、CreatedAt、UpdatedAt。

- Partner：Type、Category、Contact、Phone、Email、Address、TaxNo。
- Material：Spec、Category、UnitID、TaxRateID、Barcode。
- Warehouse：Type、Address。
- Unit：Precision。
- Currency：Symbol、DecimalPlaces。
- TaxRate：Rate（decimal）、Type。

- [ ] **Step 2: 仓储接口**

```go
type PartnerRepository interface {
    Create(ctx context.Context, partner *Partner) error
    Update(ctx context.Context, filter scope.Filter, partner *Partner) error
    Disable(ctx context.Context, filter scope.Filter, tenantID, id, userID string) error
    FindByID(ctx context.Context, filter scope.Filter, tenantID, id string) (*Partner, error)
    List(ctx context.Context, filter scope.Filter, keyword string, offset, limit int) ([]*Partner, int, error)
}
```

Material、Warehouse 同构。Unit、Currency、TaxRate 只做租户隔离。

- [ ] **Step 3: PostgreSQL 实现**

- 所有写操作带 `tenant_id`。
- 主数据查询带数据权限条件。
- 唯一键冲突映射 `domain.ErrConflict`。
- 禁用使用 `status='disabled'`，不物理删除。

- [ ] **Step 4: 测试**

使用真实 PostgreSQL 或 SQL 参数测试，覆盖列表 scope、详情/更新/删除越权返回 `ErrNotFound`、唯一键冲突。

---

## Task 5: 主数据服务

**Files:**
- Create: `internal/modules/masterdata/app/*.go`
- Test: `internal/modules/masterdata/app/*_test.go`

- [ ] **Step 1: 通用规则**

- `TenantID` 从 context 获取，禁止从请求体传入。
- `OrgID` 默认取当前用户主组织。
- `CreatedBy/UpdatedBy` 取当前 Actor。
- `Status` 默认 `active`。
- `Code` 为空时调用编码规则生成器。
- 写操作前检查数据权限。

- [ ] **Step 2: 服务**

每个实体一个服务，提供 `List/Create/Update/Disable/Get`。物料创建/更新时校验单位和税率属于当前租户。

- [ ] **Step 3: 测试**

- 编码自动生成。
- 跨租户不可见。
- self/dept/company/all 四种范围。
- 物料引用不存在单位返回冲突/错误。
- 禁用后列表可过滤。

---

## Task 6: 主数据 HTTP API

**Files:**
- Create: `internal/modules/masterdata/http/*.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: 路由**

| 路径 | 权限 |
|---|---|
| `/api/v1/masterdata/partners` | `masterdata.partner.read/write` |
| `/api/v1/masterdata/materials` | `masterdata.material.read/write` |
| `/api/v1/masterdata/warehouses` | `masterdata.warehouse.read/write` |
| `/api/v1/masterdata/units` | `masterdata.unit.read/write` |
| `/api/v1/masterdata/currencies` | `masterdata.currency.read/write` |
| `/api/v1/masterdata/tax-rates` | `masterdata.tax.read/write` |

- [ ] **Step 2: DTO**

统一分页响应 `{ items, total }`，金额/税率使用字符串，状态只有 `active/disabled`。

- [ ] **Step 3: 装配**

`cmd/api/main.go` 创建仓储、服务、Handler，并在受保护路由组注册。数据权限由 `authmw` 注入的 Actor 提供。

---

## Task 7: Seed 更新

**Files:**
- Modify: `cmd/seed/main.go`

- [ ] **Step 1: 增加权限**

把 Stage 2 的 14 个权限加入 seed 列表，并重新分配给 admin 角色。

- [ ] **Step 2: 默认编码规则**

```text
partner        CUS  yyyyMMdd 4 day
material       MAT  yyyyMMdd 4 day
warehouse      WH   yyyyMMdd 3 day
purchase_order PO   yyyyMMdd 4 day
```

- [ ] **Step 3: 验证**

```bash
go run ./cmd/seed
```

重复执行不报错，不重置已有编码流水。

---

## Task 8: 前端 SDK 与通用 CRUD 组件

**Files:**
- Create: `packages/sdk/src/masterdata.ts`
- Modify: `packages/sdk/src/system.ts`
- Modify: `packages/sdk/src/index.ts`
- Create: `apps/web/src/components/MasterDataCrud.vue`

- [ ] **Step 1: SDK**

类型：`Partner`、`Material`、`Warehouse`、`Unit`、`Currency`、`TaxRate`、`CodeRule`。

API：

```ts
masterDataApi.partners.list/create/update/remove
masterDataApi.materials...
masterDataApi.warehouses...
masterDataApi.units...
masterDataApi.currencies...
masterDataApi.taxRates...
systemApi.codeRules.list/create/update
```

- [ ] **Step 2: 通用组件**

`MasterDataCrud.vue` 支持：

- 标题、描述、权限前缀。
- 搜索、分页。
- 动态列、动态表单字段。
- `input/textarea/select/number/switch` 字段类型。
- 新建、编辑、禁用。
- 只读权限隐藏写操作。

- [ ] **Step 3: 测试**

```bash
pnpm typecheck:web
pnpm build:web
```

---

## Task 9: 前端页面与路由

**Files:**
- Create: `apps/web/src/views/masterdata/*.vue`
- Create: `apps/web/src/views/system/CodeRuleView.vue`
- Modify: `apps/web/src/router/index.ts`
- Modify: `apps/web/src/layouts/DefaultLayout.vue`

- [ ] **Step 1: 六个主数据页面**

使用 `MasterDataCrud`，字段和列按实体配置。物料页面加载单位/税率选项。

- [ ] **Step 2: 编码规则页面**

列表、创建、编辑、启用/禁用；展示当前流水号和重置策略。

- [ ] **Step 3: 菜单与路由**

- 主数据菜单：客商、物料、仓库、计量单位、币种、税率。
- 系统菜单：编码规则。
- 路由 meta 权限与后端一致。

---

## Task 10: 测试、审核与报告

**Files:**
- Create: `docs/reports/2026-11-16-stage-2-masterdata-data-scope.md`

- [ ] **Step 1: 静态检查**

```bash
gofmt -l ./cmd ./internal ./db
go vet ./...
go test ./...
pnpm typecheck:web
pnpm build:web
```

- [ ] **Step 2: 数据库迁移与 seed**

```bash
go run ./cmd/migrate -direction up
go run ./cmd/seed
```

- [ ] **Step 3: API 测试**

- 登录 admin。
- 创建/查询/更新/禁用客商、物料、仓库、单位、币种、税率。
- 自动编码生成。
- 数据权限：self/dept/company/all 四种范围。
- 无权限 403、无 token 401。
- 跨租户不可见（使用第二个租户测试或 SQL 验证）。

- [ ] **Step 4: 代码审核**

独立审核 Stage 2 提交范围，重点检查租户隔离、数据权限、编码并发、物料引用校验、前端权限。

- [ ] **Step 5: 汇总报告**

报告包含范围、API、测试结果、审核发现、未完成项、Stage 3 建议。

---

## Acceptance Criteria

- [ ] 六个主数据 CRUD 可用。
- [ ] 编码规则可配置，主数据编码可自动生成。
- [ ] 并发生成编码不重复。
- [ ] 数据权限 all/company/dept/self 生效。
- [ ] 跨租户不可见、不可改、不可删。
- [ ] 物料单位/税率引用校验生效。
- [ ] 前端主数据页面和编码规则页面可用。
- [ ] 权限菜单按 `masterdata.*` / `system.code_rule.*` 显示。
- [ ] `go test ./...`、`go vet ./...`、`pnpm typecheck:web`、`pnpm build:web` 通过。
- [ ] 真实 PostgreSQL API 冒烟测试通过。
- [ ] 代码审核完成并修复 Critical/Important。
- [ ] 汇总报告完成，等待用户审阅。
