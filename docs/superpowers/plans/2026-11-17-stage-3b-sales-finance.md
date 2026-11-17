# Stage 3B 销售、发货出库与应收应付 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Stage 3A 采购/库存闭环之上，实现贸易/分销的第二条业务闭环：销售订单 → 审批 → 发货出库 → 库存扣减 → 应收发票 → 收款核销；同时把采购收货接入应付发票与付款核销，并补齐自动化接口测试和前端完整度检查。

**Architecture:** 新增 `internal/modules/sales` 负责销售订单和发货单；新增 `internal/modules/finance` 负责统一应收/应付发票与收付款核销。发货在单个 PostgreSQL 事务内完成订单行更新、库存扣减、库存流水、应收发票；采购收货在既有事务内追加应付发票。库存余额增加移动平均成本 `avg_cost`，发货按当前平均成本记库存出库成本。

**Tech Stack:** Go 1.25 + Gin + pgx + PostgreSQL + shopspring/decimal；Vue 3 + TypeScript + Element Plus；Go integration test + PowerShell 编排脚本。

---

## 1. 范围

### 1.1 销售

- 销售订单头/行：客户、仓库、订单日期、币种、金额、备注。
- 行：物料、数量、单价、金额、已发数量。
- 状态机：`draft → submitted → approved → partially_shipped → shipped`；`cancelled` 可从无发货的 draft/submitted/approved 进入。
- 操作：创建、编辑草稿、提交、审批、取消、发货。
- 发货单头/行：发货单号、销售订单、仓库、发货日期、备注、行数量、成本。
- 部分发货、全部发货、超发校验、重复发货行校验、库存不足校验。

### 1.2 库存

- 库存余额增加 `avg_cost`（移动加权平均成本）。
- 采购收货按采购单价更新移动平均成本。
- 销售发货扣减 `on_hand`、`available`，写 `direction='out'`、`biz_type='sales_shipment'` 的库存流水，单位成本取当前 `avg_cost`。
- 数据库增加 `on_hand >= 0`、`reserved >= 0`、`available >= 0` 检查约束。

### 1.3 财务

统一发票模型：

- `finance_invoices`：`invoice_type` 为 `receivable` / `payable`，来源 `sales_shipment` / `purchase_receipt`，记录往来单位、日期、到期日、币种、总额、已核销、状态。
- 销售发货自动生成应收发票。
- 采购收货自动生成应付发票。
- 发票状态：`open → partially_paid → paid`。

统一收付款：

- `finance_payments`：`payment_type` 为 `receipt`（收款）/ `payment`（付款）。
- `finance_payment_allocations`：付款/收款与发票的核销明细。
- 支持部分核销、多次核销、重复核销校验、类型/往来单位/币种匹配校验。
- 状态：`unallocated → partially_allocated → allocated`。

### 1.4 权限

- `sales.order.read`
- `sales.order.write`
- `sales.order.approve`
- `sales.shipment.read`
- `finance.invoice.read`
- `finance.payment.read`
- `finance.payment.write`

### 1.5 编码规则

新增默认规则：

| entity | prefix | 示例 |
|---|---|---|
| `sales_order` | `SO` | `SO-20261117-0001` |
| `sales_shipment` | `DN` | `DN-20261117-0001` |
| `sales_receivable` | `AR` | `AR-20261117-0001` |
| `purchase_payable` | `AP` | `AP-20261117-0001` |
| `finance_receipt` | `RCPT` | `RCPT-20261117-0001` |
| `finance_payment` | `PAY` | `PAY-20261117-0001` |

### 1.6 前端

- 销售订单列表、新建/编辑、详情、提交、审批、取消、发货。
- 发货单列表。
- 应收/应付发票列表、详情、账龄/到期提示。
- 收款/付款列表、新建、核销。
- 菜单、路由、权限、SDK。

### 1.7 测试

- Go 领域单元测试：销售状态机、金额、剩余可发、重复行聚合、付款核销金额。
- Go 集成接口测试：`tests/api/stage3b_test.go`，通过真实 HTTP API 验证销售、发货、库存、应收、应付、收付款、数据权限、401/403、错误码。
- PowerShell 编排脚本：`scripts/test/stage3b-api.ps1`，启动 Docker/Postgres、迁移、seed、demo、API，再跑集成测试。
- 前端完整度检查：`scripts/test/check-frontend.mjs` 校验路由、菜单、页面文件、SDK 方法；同时执行 `pnpm typecheck:web` 和 `pnpm build:web`。

---

## 2. 数据库迁移 000008

### 2.1 文件

- Create: `db/migrations/000008_sales_finance.up.sql`
- Create: `db/migrations/000008_sales_finance.down.sql`

### 2.2 表结构

```sql
ALTER TABLE erp.inventory_balances
    ADD COLUMN IF NOT EXISTS avg_cost numeric(19,6) NOT NULL DEFAULT 0;

ALTER TABLE erp.inventory_balances
    ADD CONSTRAINT inventory_balances_non_negative
    CHECK (on_hand >= 0 AND reserved >= 0 AND available >= 0);

CREATE TABLE erp.sales_orders (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    org_id uuid NOT NULL,
    code text NOT NULL,
    customer_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    order_date date NOT NULL,
    currency text NOT NULL DEFAULT 'CNY',
    status text NOT NULL DEFAULT 'draft',
    total_amount numeric(19,4) NOT NULL DEFAULT 0,
    remark text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sales_orders_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT sales_orders_tenant_id_key UNIQUE (tenant_id, id)
);

CREATE TABLE erp.sales_order_lines (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    order_id uuid NOT NULL,
    line_no integer NOT NULL,
    material_id uuid NOT NULL,
    quantity numeric(19,6) NOT NULL,
    unit_price numeric(19,6) NOT NULL,
    amount numeric(19,4) NOT NULL,
    shipped_qty numeric(19,6) NOT NULL DEFAULT 0,
    CONSTRAINT sales_order_lines_order_fk FOREIGN KEY (tenant_id, order_id)
        REFERENCES erp.sales_orders (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT sales_order_lines_order_line_key UNIQUE (order_id, line_no),
    CONSTRAINT sales_order_lines_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT sales_order_lines_shipped_check CHECK (shipped_qty >= 0 AND shipped_qty <= quantity)
);

CREATE TABLE erp.sales_shipments (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    code text NOT NULL,
    order_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    shipment_date date NOT NULL,
    status text NOT NULL DEFAULT 'posted',
    remark text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sales_shipments_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT sales_shipments_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT sales_shipments_order_fk FOREIGN KEY (tenant_id, order_id)
        REFERENCES erp.sales_orders (tenant_id, id)
);

CREATE TABLE erp.sales_shipment_lines (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    order_line_id uuid NOT NULL,
    material_id uuid NOT NULL,
    quantity numeric(19,6) NOT NULL,
    unit_price numeric(19,6) NOT NULL,
    amount numeric(19,4) NOT NULL,
    unit_cost numeric(19,6) NOT NULL,
    cost_amount numeric(19,4) NOT NULL,
    CONSTRAINT sales_shipment_lines_shipment_fk FOREIGN KEY (tenant_id, shipment_id)
        REFERENCES erp.sales_shipments (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT sales_shipment_lines_order_line_fk FOREIGN KEY (tenant_id, order_line_id)
        REFERENCES erp.sales_order_lines (tenant_id, id),
    CONSTRAINT sales_shipment_lines_shipment_line_key UNIQUE (shipment_id, order_line_id)
);

CREATE TABLE erp.finance_invoices (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    code text NOT NULL,
    invoice_type text NOT NULL,
    source_type text NOT NULL,
    source_id uuid NOT NULL,
    source_code text NOT NULL DEFAULT '',
    partner_id uuid NOT NULL,
    invoice_date date NOT NULL,
    due_date date NOT NULL,
    currency text NOT NULL DEFAULT 'CNY',
    status text NOT NULL DEFAULT 'open',
    total_amount numeric(19,4) NOT NULL,
    paid_amount numeric(19,4) NOT NULL DEFAULT 0,
    remark text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_invoices_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT finance_invoices_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT finance_invoices_source_key UNIQUE (tenant_id, invoice_type, source_type, source_id),
    CONSTRAINT finance_invoices_amount_check CHECK (total_amount >= 0 AND paid_amount >= 0 AND paid_amount <= total_amount)
);

CREATE TABLE erp.finance_payments (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    code text NOT NULL,
    payment_type text NOT NULL,
    partner_id uuid NOT NULL,
    payment_date date NOT NULL,
    currency text NOT NULL DEFAULT 'CNY',
    amount numeric(19,4) NOT NULL,
    allocated_amount numeric(19,4) NOT NULL DEFAULT 0,
    status text NOT NULL DEFAULT 'unallocated',
    method text NOT NULL DEFAULT 'bank',
    reference text NOT NULL DEFAULT '',
    remark text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_payments_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT finance_payments_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT finance_payments_amount_check CHECK (amount > 0 AND allocated_amount >= 0 AND allocated_amount <= amount)
);

CREATE TABLE erp.finance_payment_allocations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    payment_id uuid NOT NULL,
    invoice_id uuid NOT NULL,
    amount numeric(19,4) NOT NULL,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_payment_allocations_payment_fk FOREIGN KEY (tenant_id, payment_id)
        REFERENCES erp.finance_payments (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT finance_payment_allocations_invoice_fk FOREIGN KEY (tenant_id, invoice_id)
        REFERENCES erp.finance_invoices (tenant_id, id),
    CONSTRAINT finance_payment_allocations_payment_invoice_key UNIQUE (payment_id, invoice_id),
    CONSTRAINT finance_payment_allocations_amount_check CHECK (amount > 0)
);
```

索引：

```sql
CREATE INDEX sales_orders_list_idx ON erp.sales_orders (tenant_id, org_id, status, order_date DESC);
CREATE INDEX sales_order_lines_order_idx ON erp.sales_order_lines (tenant_id, order_id);
CREATE INDEX sales_shipments_order_idx ON erp.sales_shipments (tenant_id, order_id, shipment_date DESC);
CREATE INDEX sales_shipment_lines_shipment_idx ON erp.sales_shipment_lines (tenant_id, shipment_id);
CREATE INDEX finance_invoices_list_idx ON erp.finance_invoices (tenant_id, org_id, invoice_type, status, invoice_date DESC);
CREATE INDEX finance_invoices_partner_idx ON erp.finance_invoices (tenant_id, partner_id, invoice_type);
CREATE INDEX finance_payments_list_idx ON erp.finance_payments (tenant_id, org_id, payment_type, payment_date DESC);
CREATE INDEX finance_payment_allocations_invoice_idx ON erp.finance_payment_allocations (tenant_id, invoice_id);
```

权限、角色授权、编码规则与迁移 000006 相同模式。

### 2.3 平均成本回填

```sql
UPDATE erp.inventory_balances b
SET avg_cost = COALESCE((
    SELECT SUM(rl.amount) / NULLIF(SUM(rl.quantity), 0)
    FROM erp.purchase_receipt_lines rl
    JOIN erp.purchase_receipts r
      ON r.tenant_id = rl.tenant_id AND r.id = rl.receipt_id
    WHERE r.tenant_id = b.tenant_id
      AND r.warehouse_id = b.warehouse_id
      AND rl.material_id = b.material_id
), 0)
WHERE b.avg_cost = 0;
```

---

## 3. 文件结构

```text
internal/modules/sales/
├─ domain/
│  ├─ order.go
│  ├─ shipment.go
│  ├─ status.go
│  ├─ errors.go
│  └─ repository.go
├─ app/
│  ├─ common.go
│  ├─ order_service.go
│  └─ shipment_service.go
├─ infra/postgres/
│  └─ repository.go
└─ http/
   └─ handler.go

internal/modules/finance/
├─ domain/
│  ├─ invoice.go
│  ├─ payment.go
│  ├─ status.go
│  ├─ errors.go
│  └─ repository.go
├─ app/
│  ├─ common.go
│  ├─ invoice_service.go
│  └─ payment_service.go
├─ infra/postgres/
│  ├─ invoice_repository.go
│  └─ payment_repository.go
└─ http/
   └─ handler.go

tests/api/
└─ stage3b_test.go

scripts/test/
├─ stage3b-api.ps1
└─ check-frontend.mjs
```

---

## 4. Task 1：迁移、权限、编码规则

**Files:**
- Create: `db/migrations/000008_sales_finance.up.sql`
- Create: `db/migrations/000008_sales_finance.down.sql`
- Modify: `cmd/seed/main.go`
- Modify: `db/migrations/README.md`

- [ ] 写迁移 SQL（见第 2 节）。
- [ ] `cmd/seed` 增加 7 个权限和 6 个编码规则。
- [ ] 迁移 README 增加 000008 说明。
- [ ] 运行 `go run ./cmd/migrate -direction up`。
- [ ] 验证 `\d erp.sales_orders`、`\d erp.finance_invoices`、`avg_cost`、约束存在。
- [ ] 提交。

## 5. Task 2：财务领域与仓储

**Files:**
- Create: `internal/modules/finance/domain/invoice.go`
- Create: `internal/modules/finance/domain/payment.go`
- Create: `internal/modules/finance/domain/status.go`
- Create: `internal/modules/finance/domain/errors.go`
- Create: `internal/modules/finance/domain/repository.go`
- Create: `internal/modules/finance/infra/postgres/invoice_repository.go`
- Create: `internal/modules/finance/infra/postgres/payment_repository.go`

核心类型：

```go
type Invoice struct {
    ID, TenantID, OrgID, Code string
    InvoiceType string // receivable | payable
    SourceType, SourceID, SourceCode string
    PartnerID, PartnerName string
    InvoiceDate, DueDate time.Time
    Currency, Status string
    TotalAmount, PaidAmount decimal.Decimal
    Remark string
    CreatedAt, UpdatedAt time.Time
}

func (i *Invoice) Outstanding() decimal.Decimal {
    return i.TotalAmount.Sub(i.PaidAmount)
}

type Payment struct {
    ID, TenantID, OrgID, Code string
    PaymentType string // receipt | payment
    PartnerID, PartnerName string
    PaymentDate time.Time
    Currency, Status, Method, Reference, Remark string
    Amount, AllocatedAmount decimal.Decimal
    CreatedAt time.Time
}

func (p *Payment) Unallocated() decimal.Decimal {
    return p.Amount.Sub(p.AllocatedAmount)
}

type AllocationInput struct {
    InvoiceID string
    Amount decimal.Decimal
}
```

仓储接口：

```go
type InvoiceRepository interface {
    List(ctx context.Context, filter scope.Filter, tenantID, invoiceType, status, partnerID, keyword string, offset, limit int) ([]*Invoice, int, error)
    Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*Invoice, error)
    ListOpenByPartner(ctx context.Context, filter scope.Filter, tenantID, partnerID, invoiceType string) ([]*Invoice, error)
}

type PaymentRepository interface {
    Create(ctx context.Context, payment *Payment) error
    Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*Payment, error)
    List(ctx context.Context, filter scope.Filter, tenantID, paymentType, status, partnerID, keyword string, offset, limit int) ([]*Payment, int, error)
    Allocate(ctx context.Context, filter scope.Filter, paymentID string, allocations []*Allocation) error
}
```

`Allocate` 事务：

1. `SELECT ... FOR UPDATE` 锁定 payment，校验数据权限、状态、剩余可核销金额。
2. 按 invoice_id 聚合核销金额，拒绝重复、非正数。
3. 按 ID 排序 `SELECT ... FOR UPDATE` 锁定发票。
4. 校验发票 tenant、类型匹配（receipt→receivable，payment→payable）、partner、currency、状态、未核销金额。
5. 写 allocation 行。
6. 更新 invoice `paid_amount`，重算状态。
7. 更新 payment `allocated_amount`，重算状态。
8. 提交。

## 6. Task 3：财务应用服务与 HTTP

**Files:**
- Create: `internal/modules/finance/app/common.go`
- Create: `internal/modules/finance/app/invoice_service.go`
- Create: `internal/modules/finance/app/payment_service.go`
- Create: `internal/modules/finance/http/handler.go`
- Modify: `cmd/api/main.go`

API：

| 方法 | 路径 | 权限 |
|---|---|---|
| GET | `/api/v1/finance/invoices` | `finance.invoice.read` |
| GET | `/api/v1/finance/invoices/:id` | `finance.invoice.read` |
| GET | `/api/v1/finance/payments` | `finance.payment.read` |
| GET | `/api/v1/finance/payments/:id` | `finance.payment.read` |
| POST | `/api/v1/finance/payments` | `finance.payment.write` |
| POST | `/api/v1/finance/payments/:id/allocate` | `finance.payment.write` |

错误码：

- 400：参数非法、金额非正、类型不匹配、往来单位/币种不匹配、核销超过发票未付金额或付款未核销金额。
- 403：无权限。
- 404：发票/付款不存在或越权。
- 409：重复核销、状态冲突。

## 7. Task 4：销售领域

**Files:**
- Create: `internal/modules/sales/domain/order.go`
- Create: `internal/modules/sales/domain/shipment.go`
- Create: `internal/modules/sales/domain/status.go`
- Create: `internal/modules/sales/domain/errors.go`
- Create: `internal/modules/sales/domain/repository.go`
- Test: `internal/modules/sales/domain/order_test.go`

状态机：

```go
const (
    StatusDraft            Status = "draft"
    StatusSubmitted        Status = "submitted"
    StatusApproved         Status = "approved"
    StatusPartiallyShipped Status = "partially_shipped"
    StatusShipped          Status = "shipped"
    StatusCancelled        Status = "cancelled"
)

func (s Status) CanEdit() bool { return s == StatusDraft }
func (s Status) CanSubmit() bool { return s == StatusDraft }
func (s Status) CanApprove() bool { return s == StatusSubmitted }
func (s Status) CanCancel() bool {
    return s == StatusDraft || s == StatusSubmitted || s == StatusApproved
}
func (s Status) CanShip() bool {
    return s == StatusApproved || s == StatusPartiallyShipped
}
```

订单行：

```go
type OrderLine struct {
    ID, TenantID, OrderID string
    LineNo int
    MaterialID, MaterialName string
    Quantity, UnitPrice, Amount, ShippedQty decimal.Decimal
}

func (l *OrderLine) Remaining() decimal.Decimal {
    return l.Quantity.Sub(l.ShippedQty)
}
```

领域测试：

- 金额计算 `quantity * unit_price`，总金额四舍五入 4 位。
- `Remaining()`。
- 状态迁移允许/拒绝矩阵。
- 取消时若存在已发数量必须拒绝。

## 8. Task 5：销售仓储与发货事务

**Files:**
- Create: `internal/modules/sales/infra/postgres/repository.go`

`Repository` 实现：

```go
type Repository struct { db *pgxpool.Pool }

func (r *Repository) Create(ctx context.Context, order *domain.Order) error
func (r *Repository) UpdateDraft(ctx context.Context, filter scope.Filter, order *domain.Order) error
func (r *Repository) Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*domain.Order, error)
func (r *Repository) List(ctx context.Context, filter scope.Filter, tenantID, status, keyword string, offset, limit int) ([]*domain.Order, int, error)
func (r *Repository) UpdateStatus(ctx context.Context, tenantID, id string, from, to domain.Status, userID string) (bool, error)
func (r *Repository) Ship(ctx context.Context, filter scope.Filter, shipment *domain.Shipment) error
func (r *Repository) ListShipments(ctx context.Context, filter scope.Filter, tenantID, orderID string, offset, limit int) ([]*domain.Shipment, int, error)
```

`Ship` 事务步骤：

1. 锁定销售订单 `FOR UPDATE OF o`，校验状态 `CanShip`。
2. 锁定订单行 `FOR UPDATE OF l`。
3. 拒绝重复 `order_line_id`，校验数量和剩余可发数量。
4. 按 `material_id + order_line_id` 稳定排序。
5. 逐个物料锁定库存余额：

```sql
SELECT on_hand, reserved, available, avg_cost
FROM erp.inventory_balances
WHERE tenant_id = $1 AND warehouse_id = $2 AND material_id = $3
FOR UPDATE
```

6. 校验 `available >= quantity`，否则 `ErrInsufficientStock`。
7. 插入 shipment 头和行。
8. 条件更新订单行：

```sql
UPDATE erp.sales_order_lines
SET shipped_qty = shipped_qty + $3
WHERE tenant_id = $1 AND id = $2 AND shipped_qty + $3 <= quantity
```

9. 条件扣减库存：

```sql
UPDATE erp.inventory_balances
SET on_hand = on_hand - $4,
    available = available - $4,
    updated_at = now()
WHERE tenant_id = $1 AND warehouse_id = $2 AND material_id = $3
  AND on_hand >= $4 AND available >= $4
```

10. 插入库存流水：`direction='out'`、`biz_type='sales_shipment'`、`unit_cost=avg_cost`、`amount=quantity*avg_cost`。
11. 插入应收发票：`invoice_type='receivable'`、`source_type='sales_shipment'`、`partner_id=客户`、`total_amount=发货金额`、`due_date=发货日期+30天`。
12. 重算订单状态 `partially_shipped` / `shipped`。
13. 提交。

## 9. Task 6：销售应用服务与 HTTP

**Files:**
- Create: `internal/modules/sales/app/common.go`
- Create: `internal/modules/sales/app/order_service.go`
- Create: `internal/modules/sales/app/shipment_service.go`
- Create: `internal/modules/sales/http/handler.go`
- Modify: `cmd/api/main.go`

API：

| 方法 | 路径 | 权限 |
|---|---|---|
| GET/POST | `/api/v1/sales/orders` | `sales.order.read/write` |
| GET/PUT | `/api/v1/sales/orders/:id` | `sales.order.read/write` |
| POST | `/api/v1/sales/orders/:id/submit` | `sales.order.write` |
| POST | `/api/v1/sales/orders/:id/approve` | `sales.order.approve` |
| POST | `/api/v1/sales/orders/:id/cancel` | `sales.order.write` |
| POST | `/api/v1/sales/orders/:id/ship` | `sales.order.write` |
| GET | `/api/v1/sales/shipments` | `sales.shipment.read` |

创建销售订单校验：

- 客户存在、active、类型为 `customer` 或 `both`。
- 仓库存在、active。
- 物料存在、active。
- 至少一行，数量 > 0，单价 >= 0。
- 订单组织取仓库组织。
- 编码 `sales_order`。
- 金额服务端计算，不接受客户端总金额。

发货校验：

- 订单存在且数据权限可见。
- 状态允许发货。
- 至少一行，数量 > 0。
- 重复行聚合后不得超过剩余可发数量。
- 库存不足返回 409 `INSUFFICIENT_STOCK`。
- 超发返回 409 `OVER_SHIPMENT`。
- 生成发货单编码 `sales_shipment` 和应收发票编码 `sales_receivable`。

## 10. Task 7：采购收货接入应付与平均成本

**Files:**
- Modify: `internal/modules/purchase/domain/receipt.go`
- Modify: `internal/modules/purchase/app/receipt_service.go`
- Modify: `internal/modules/purchase/infra/postgres/order_repository.go`
- Modify: `cmd/api/main.go`

变更：

1. `domain.Receipt` 增加：

```go
PayableID      string
PayableCode    string
PayableDueDate time.Time
```

2. `ReceiptService.Receive` 生成 `purchase_payable` 编码，到期日 = 收货日期 + 30 天。
3. `Repository.Receive` 在事务内插入 `finance_invoices`：

```sql
INSERT INTO erp.finance_invoices (
    id, tenant_id, org_id, code, invoice_type, source_type, source_id, source_code,
    partner_id, invoice_date, due_date, currency, status, total_amount, paid_amount,
    remark, created_by, updated_by
) VALUES ($1,$2,$3,$4,'payable','purchase_receipt',$5,$6,$7,$8,$9,$10,'open',$11,0,'',$12,$12)
```

4. 库存 upsert 增加 `avg_cost` 移动平均：

```sql
INSERT INTO erp.inventory_balances (
    tenant_id, org_id, warehouse_id, material_id, on_hand, reserved, available, avg_cost, updated_at
) VALUES ($1,$2,$3,$4,$5,0,$5,$6,now())
ON CONFLICT (tenant_id, warehouse_id, material_id) DO UPDATE
SET avg_cost = CASE
        WHEN erp.inventory_balances.on_hand + EXCLUDED.on_hand = 0 THEN 0
        ELSE (erp.inventory_balances.on_hand * erp.inventory_balances.avg_cost
              + EXCLUDED.on_hand * EXCLUDED.avg_cost)
             / (erp.inventory_balances.on_hand + EXCLUDED.on_hand)
    END,
    on_hand = erp.inventory_balances.on_hand + EXCLUDED.on_hand,
    available = erp.inventory_balances.available + EXCLUDED.available,
    updated_at = now()
```

5. 已有 Stage 3A 收货单不自动回填应付发票；由 demo 数据补建 `AP-DEMO-*`，并在报告中说明。

## 11. Task 8：SDK 与前端

**Files:**
- Create: `packages/sdk/src/sales.ts`
- Create: `packages/sdk/src/finance.ts`
- Modify: `packages/sdk/src/index.ts`
- Create: `apps/web/src/views/sales/OrderListView.vue`
- Create: `apps/web/src/views/sales/ShipmentListView.vue`
- Create: `apps/web/src/views/finance/InvoiceListView.vue`
- Create: `apps/web/src/views/finance/PaymentListView.vue`
- Modify: `apps/web/src/router/index.ts`
- Modify: `apps/web/src/layouts/DefaultLayout.vue`

前端页面要求：

### 销售订单

- 列表：单号、客户、仓库、日期、金额、状态。
- 新建/编辑：客户、仓库、日期、币种、备注、明细（物料、数量、单价）。
- 详情：明细、已发数量、未发数量。
- 操作：提交、审批、取消、发货。
- 发货弹窗：默认填入剩余数量，允许部分发货，显示库存可用量提示。

### 发货单

- 列表：单号、销售订单、客户、仓库、日期、金额、成本、状态。

### 应收/应付

- Tab 或筛选：应收、应付。
- 列表：单号、类型、来源单号、往来单位、日期、到期日、币种、总额、已核销、未核销、状态。
- 到期日小于今天且未付清时显示“逾期”标签。
- 详情抽屉：发票头信息、来源单据、核销记录（可选）。

### 收付款

- 列表：单号、类型、往来单位、日期、币种、金额、已核销、未核销、状态、方式。
- 新建：类型、往来单位、日期、币种、金额、方式、参考号、备注。
- 核销弹窗：加载同往来单位、同币种、同类型的未付清发票；填写本次核销金额；支持多张发票；显示合计与剩余。

路由与菜单：

```text
/sales/orders       sales.order.read
/sales/shipments    sales.shipment.read
/finance/invoices   finance.invoice.read
/finance/payments   finance.payment.read
```

## 12. Task 9：Demo 数据

**Files:**
- Modify: `cmd/demo/main.go`

新增数据：

1. 销售订单 1：CUS-DEMO-001，WH-SH，MAT-DEMO-001 × 2，单价 150，审批并发货 2，生成应收 300；创建收款 300 并核销 → 已收款。
2. 销售订单 2：CUS-DEMO-002，WH-SH，MAT-DEMO-002 × 3，单价 80，审批并部分发货 1，生成应收 80；剩余 2 未发。
3. 采购订单 3：SUP-DEMO-001，WH-SH，MAT-DEMO-003 × 2，单价 100，审批并收货 2，生成应付 200；创建付款 200 并核销 → 已付款。
4. 为 Stage 3A 已有 demo 收货单补建应付发票：编码 `AP-DEMO-0001`、`AP-DEMO-0002`，金额取收货行合计，状态 open。

幂等：

- 销售订单按 `remark` 查询是否存在，存在则跳过。
- 应收/应付按 `(tenant_id, invoice_type, source_type, source_id)` 查询。
- 收付款按 `reference` 或固定 remark 查询。
- 重复执行不重复插入、不重复扣库存。

## 13. Task 10：Go 集成接口测试

**Files:**
- Create: `tests/api/stage3b_test.go`

构建标签：

```go
//go:build integration

package api_test
```

环境变量：

- `TEST_API_BASE_URL`（默认 `http://localhost:18080`）
- `TEST_ADMIN_USERNAME`（默认 `admin`）
- `TEST_ADMIN_PASSWORD`（默认 `Admin@123456`）
- `TEST_DEMO_PASSWORD`（默认 `Demo@123456`）

测试辅助：

```go
type client struct {
    baseURL string
    token   string
    t       *testing.T
}

func (c *client) do(method, path string, body any, wantStatus int) map[string]any
func (c *client) get(path string, wantStatus int) map[string]any
func (c *client) post(path string, body any, wantStatus int) map[string]any
func (c *client) put(path string, body any, wantStatus int) map[string]any
```

测试用例：

1. **认证**
   - 无 token 访问 `/sales/orders` → 401。
   - 错误 token → 401。
2. **销售订单生命周期**
   - 创建 10 × 120 → 201，`status=draft`，`total_amount=1200.0000`。
   - 提交 → 200。
   - 审批 → 200。
   - 发货 4 → 201，订单 `partially_shipped`，`shipped_qty=4`，`remaining_qty=6`。
   - 库存 `on_hand` 减少 4。
   - 应收发票 1 张，金额 480，状态 open。
   - 再发货 6 → 201，订单 `shipped`，库存再减少 6。
   - 应收发票 2 张，合计 1200。
3. **超发与重复行**
   - 新订单 10，发货 11 → 409 `OVER_SHIPMENT`。
   - 同一请求两次 `order_line_id`，合计 11 → 409，库存不变。
   - 同一请求重复行合计 10 → 409 `BAD_REQUEST`（拒绝重复行）或 409。
4. **库存不足**
   - 对无库存物料创建订单并审批，发货 → 409 `INSUFFICIENT_STOCK`。
5. **非法状态**
   - 草稿订单发货 → 409 `INVALID_STATUS`。
   - 已取消订单发货 → 409。
   - 审批后再提交 → 409。
6. **取消**
   - 审批后未发货订单取消 → 200。
   - 部分发货订单取消 → 409。
7. **应收与收款**
   - 查询应收发票列表，按客户过滤。
   - 创建收款 500，类型 receipt，客户匹配。
   - 核销 480 到第一张发票 → 发票 paid，付款 partially_allocated。
   - 核销 20 到第二张发票 → 付款 allocated，发票 partially_paid。
   - 超额核销 → 400。
   - 类型不匹配（receipt 核销 payable）→ 400。
   - 重复核销同一发票 → 409/400。
8. **应付与付款**
   - 创建采购订单 5 × 100，提交/审批/收货 → 201。
   - 应付发票自动生成，金额 500，状态 open。
   - 创建付款 500，类型 payment，供应商匹配。
   - 核销 → 发票 paid。
9. **数据权限**
   - `demo_dept` 查看 HQ 销售订单/应收发票 → 0。
   - `demo_dept` 创建销售订单 → 403。
   - `demo_company` 能查看华东仓库相关数据（若 demo 数据存在）。
   - demo 角色只读，POST 收付款 → 403。
10. **分页与筛选**
    - `page_size=1` 返回 1 条，`total >= 1`。
    - `status=shipped` 只返回 shipped。
    - `invoice_type=receivable` 只返回应收。
11. **金额精度**
    - 0.1 × 3 = 0.3，服务端金额字符串为 `0.3000`。
    - 负数/零数量 → 400。

## 14. Task 11：测试编排与前端完整度检查

**Files:**
- Create: `scripts/test/stage3b-api.ps1`
- Create: `scripts/test/check-frontend.mjs`
- Modify: `package.json`

`stage3b-api.ps1` 步骤：

1. 启动 Docker Desktop（如未运行）。
2. `docker compose -f deploy/compose/docker-compose.dev.yml up -d postgres`。
3. 等待 PostgreSQL healthy。
4. `go run ./cmd/migrate -direction up`。
5. `go run ./cmd/seed`。
6. `go run ./cmd/demo`。
7. `go build -o dist/api.exe ./cmd/api`。
8. 使用 `HTTP_ADDR=:18080` 启动 API。
9. 等待 `/readyz`。
10. `go test -tags=integration ./tests/api -run TestStage3B -v`。
11. 停止 API，保留测试数据或按需清理。
12. 失败时输出 API 日志并以非 0 退出。

`check-frontend.mjs` 检查：

- 路由文件包含 `/sales/orders`、`/sales/shipments`、`/finance/invoices`、`/finance/payments` 及对应权限。
- 菜单包含销售订单、发货单、应收应付、收付款。
- 四个页面文件存在且包含 `PageHeader`、`el-table`、`el-dialog` 或 `el-drawer`。
- SDK 导出 `salesApi`、`financeApi`，并包含 `list/create/submit/approve/cancel/ship`、`invoices/payments/allocate` 方法。
- 失败时退出 1。

`package.json` 增加：

```json
"test:api:stage3b": "powershell -ExecutionPolicy Bypass -File ./scripts/test/stage3b-api.ps1",
"test:frontend": "node ./scripts/test/check-frontend.mjs"
```

## 15. Task 12：文档与报告

**Files:**
- Modify: `README.md`
- Create: `internal/modules/sales/README.md`
- Create: `internal/modules/finance/README.md`
- Create: `docs/reports/2026-11-17-stage-3b-sales-finance.md`

报告内容：

- 完成范围、数据库迁移、API 清单。
- 自动化接口测试结果（逐条用例）。
- 前端完整度检查结果和构建结果。
- 代码审核结论、修复项、遗留项。
- Demo 数据说明。
- 用户审阅清单。

---

## 16. Acceptance Criteria

- [ ] 销售订单创建/编辑/提交/审批/取消可用。
- [ ] 部分发货、全部发货、超发、重复行、库存不足校验可用。
- [ ] 发货事务内扣减库存并写库存流水，库存不足不产生脏数据。
- [ ] 销售发货自动生成应收发票。
- [ ] 采购收货自动生成应付发票。
- [ ] 收付款创建、部分核销、全部核销、重复/超额/类型不匹配校验可用。
- [ ] 销售/发货/应收应付/收付款前端页面可用，路由和菜单权限正确。
- [ ] Demo 数据包含销售、发货、应收、应付、收付款。
- [ ] `tests/api/stage3b_test.go` 自动化接口测试全部通过。
- [ ] 前端完整度检查、`pnpm typecheck:web`、`pnpm build:web` 通过。
- [ ] `go test ./...`、`go vet ./...`、`gofmt` 通过。
- [ ] 代码审核完成，Critical/Important 修复并回归。
- [ ] 汇总报告写入 `docs/reports/`，等待用户审阅。
