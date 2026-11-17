# Stage 3E 税务/发票与价税分离 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为销售、采购、应收应付发票增加价税分离、进项/销项税入账、开票状态和增值税汇总表。

**Architecture:** 在单据头和行保存净额、税额、含税金额及税率快照；新增共享 `taxcalc` 包统一计算；销售发货和采购收货自动凭证拆分收入/库存与税额；扩展现有 `finance_invoices` 支持开票/作废；GL 增加增值税汇总查询。

**Tech Stack:** Go 1.25 + Gin + pgx + PostgreSQL + shopspring/decimal；Vue 3 + TypeScript + Element Plus。

**Design:** `docs/superpowers/specs/2026-11-17-stage-3e-tax-invoice-design.md`

---

## Task 1: 迁移、科目、映射和权限

**Files:**
- Create: `db/migrations/000017_tax_invoice.up.sql`
- Create: `db/migrations/000017_tax_invoice.down.sql`
- Modify: `db/migrations/README.md`
- Modify: `cmd/seed/main.go`

- [ ] **Step 1: 写迁移 up**

```sql
ALTER TABLE erp.sales_orders
    ADD COLUMN IF NOT EXISTS price_includes_tax boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.sales_order_lines
    ADD COLUMN IF NOT EXISTS tax_rate_id uuid,
    ADD COLUMN IF NOT EXISTS tax_rate numeric(9,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS gross_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.purchase_orders
    ADD COLUMN IF NOT EXISTS price_includes_tax boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.purchase_order_lines
    ADD COLUMN IF NOT EXISTS tax_rate_id uuid,
    ADD COLUMN IF NOT EXISTS tax_rate numeric(9,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS gross_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.sales_shipments
    ADD COLUMN IF NOT EXISTS price_includes_tax boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.sales_shipment_lines
    ADD COLUMN IF NOT EXISTS tax_rate_id uuid,
    ADD COLUMN IF NOT EXISTS tax_rate numeric(9,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS gross_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.purchase_receipts
    ADD COLUMN IF NOT EXISTS price_includes_tax boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.purchase_receipt_lines
    ADD COLUMN IF NOT EXISTS tax_rate_id uuid,
    ADD COLUMN IF NOT EXISTS tax_rate numeric(9,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS gross_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.finance_invoices
    ADD COLUMN IF NOT EXISTS net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS vat_invoice_no text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS vat_invoice_type text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS vat_invoice_date date,
    ADD COLUMN IF NOT EXISTS vat_invoice_status text NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS buyer_tax_no text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS seller_tax_no text NOT NULL DEFAULT '';

UPDATE erp.sales_orders SET net_amount = total_amount WHERE net_amount = 0;
UPDATE erp.sales_order_lines SET gross_amount = amount WHERE gross_amount = 0;
-- 其余单据同样回填
UPDATE erp.finance_invoices SET net_amount = total_amount WHERE net_amount = 0;

ALTER TABLE erp.finance_invoices
    ADD CONSTRAINT finance_invoices_vat_status_check
        CHECK (vat_invoice_status IN ('pending', 'issued', 'void')),
    ADD CONSTRAINT finance_invoices_vat_type_check
        CHECK (vat_invoice_type IN ('', 'special', 'general'));

CREATE UNIQUE INDEX IF NOT EXISTS finance_invoices_vat_no_key
    ON erp.finance_invoices (tenant_id, vat_invoice_no)
    WHERE vat_invoice_no <> '';

INSERT INTO erp.gl_accounts (id, tenant_id, code, name, type, direction, level, is_leaf, status)
SELECT gen_random_uuid(), t.id, a.code, a.name, 'liability', a.direction, 3, true, 'active'
FROM erp.tenants t
CROSS JOIN (VALUES
    ('222101', '应交增值税-进项税额', 'debit'),
    ('222102', '应交增值税-销项税额', 'credit')
) AS a(code, name, direction)
ON CONFLICT (tenant_id, code) DO NOTHING;

UPDATE erp.gl_accounts SET is_leaf = false WHERE code = '2221';

UPDATE erp.gl_accounts child
SET parent_id = parent.id
FROM erp.gl_accounts parent
WHERE child.tenant_id = parent.tenant_id
  AND parent.code = '2221'
  AND child.code IN ('222101', '222102');

INSERT INTO erp.gl_account_mappings (tenant_id, key, account_code)
SELECT t.id, m.key, m.account_code
FROM erp.tenants t
CROSS JOIN (VALUES ('input_vat', '222101'), ('output_vat', '222102')) AS m(key, account_code)
ON CONFLICT DO NOTHING;

INSERT INTO erp.permissions (code, name, module, type)
VALUES ('finance.invoice.write', '开具/作废发票', 'finance', 'api')
ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, module = EXCLUDED.module, type = EXCLUDED.type;

INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, 'finance.invoice.write'
FROM erp.roles r
WHERE r.code = 'admin'
ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: 写迁移 down**

删除新增字段、索引、约束、权限、映射和子科目；`2221` 恢复 `is_leaf=true`。仅删除 `222101/222102` 无发生额时执行，避免破坏已生成凭证。

- [ ] **Step 3: 更新 seed**

在 `cmd/seed/main.go` 标准科目列表加入 `222101`/`222102`，`2221` 改为 `is_leaf=false`；父级映射加入 `(parent.code='2221' AND child.code IN ('222101','222102'))`；映射加入 `input_vat`/`output_vat`；权限列表加入 `finance.invoice.write`。

- [ ] **Step 4: 迁移和 seed 验证**

Run: `go run ./cmd/migrate -direction up && go run ./cmd/seed`
Expected: `migration completed direction=up`，`seed completed`，无 SQL 错误。

- [ ] **Step 5: Commit**

```bash
git add db/migrations cmd/seed/main.go
git commit -m "feat(stage-3e): add tax and invoice schema"
```

---

## Task 2: taxcalc 共享计算包

**Files:**
- Create: `internal/platform/taxcalc/taxcalc.go`
- Create: `internal/platform/taxcalc/taxcalc_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestCalculateInclusive(t *testing.T) {
    got, err := taxcalc.Calculate(decimal.NewFromInt(1), decimal.NewFromInt(100), decimal.RequireFromString("0.13"), true)
    if err != nil { t.Fatal(err) }
    if !got.Net.Equal(decimal.RequireFromString("88.4956")) { t.Fatalf("net=%s", got.Net) }
    if !got.Tax.Equal(decimal.RequireFromString("11.5044")) { t.Fatalf("tax=%s", got.Tax) }
    if !got.Gross.Equal(decimal.NewFromInt(100)) { t.Fatalf("gross=%s", got.Gross) }
}

func TestCalculateExclusive(t *testing.T) {
    got, _ := taxcalc.Calculate(decimal.NewFromInt(2), decimal.RequireFromString("50"), decimal.RequireFromString("0.13"), false)
    // net=100.0000 tax=13.0000 gross=113.0000
}
```

覆盖零税率、非法税率（<0、>=1）、负数量、负单价。

- [ ] **Step 2: 运行并确认失败**

Run: `go test ./internal/platform/taxcalc -run TestCalculate -v`
Expected: FAIL（包/函数不存在）。

- [ ] **Step 3: 实现**

```go
package taxcalc

import (
    "github.com/shopspring/decimal"
)

type Result struct {
    Net   decimal.Decimal
    Tax   decimal.Decimal
    Gross decimal.Decimal
}

func Calculate(quantity, unitPrice, rate decimal.Decimal, priceIncludesTax bool) (Result, error) {
    if quantity.IsNegative() || unitPrice.IsNegative() || rate.IsNegative() || rate.GreaterThanOrEqual(decimal.NewFromInt(1)) {
        return Result{}, ErrInvalidInput
    }
    entered := quantity.Mul(unitPrice).Round(4)
    if priceIncludesTax {
        net := entered.Div(decimal.NewFromInt(1).Add(rate)).Round(4)
        return Result{Net: net, Tax: entered.Sub(net).Round(4), Gross: entered}, nil
    }
    tax := entered.Mul(rate).Round(4)
    return Result{Net: entered, Tax: tax, Gross: entered.Add(tax).Round(4)}, nil
}
```

- [ ] **Step 4: 测试通过并提交**

Run: `go test ./internal/platform/taxcalc -v`
Expected: PASS。

```bash
git add internal/platform/taxcalc
git commit -m "feat(stage-3e): add shared tax calculator"
```

---

## Task 3: 销售域模型和税率解析

**Files:**
- Modify: `internal/modules/sales/domain/order.go`
- Modify: `internal/modules/sales/domain/shipment.go`
- Modify: `internal/modules/sales/domain/order_test.go`
- Modify: `internal/modules/sales/app/common.go`
- Modify: `internal/modules/sales/app/order_service.go`
- Modify: `internal/modules/sales/app/shipment_service.go`
- Modify: `cmd/api/main.go`
- Modify: `cmd/demo/main.go`

- [ ] **Step 1: 域模型**

`Order` 增加 `PriceIncludesTax bool`、`NetAmount`、`TaxAmount`；`OrderLine` 增加 `TaxRateID`、`TaxRate`、`TaxAmount`、`GrossAmount`。`Recalculate()` 使用 `taxcalc.Calculate`：

```go
func (o *Order) Recalculate() {
    netTotal, taxTotal, grossTotal := decimal.Zero, decimal.Zero, decimal.Zero
    for _, line := range o.Lines {
        result, err := taxcalc.Calculate(line.Quantity, line.UnitPrice, line.TaxRate, o.PriceIncludesTax)
        if err != nil { continue }
        line.Amount = result.Net
        line.TaxAmount = result.Tax
        line.GrossAmount = result.Gross
        netTotal = netTotal.Add(result.Net)
        taxTotal = taxTotal.Add(result.Tax)
        grossTotal = grossTotal.Add(result.Gross)
    }
    o.NetAmount = netTotal.Round(4)
    o.TaxAmount = taxTotal.Round(4)
    o.TotalAmount = grossTotal.Round(4)
}
```

`Shipment` 同样增加头字段；`ShipmentLine` 增加税率/净额/税额/含税金额；`TotalAmount()` 返回 `Σ gross_amount`，新增 `TotalNet()`、`TotalTax()`。

- [ ] **Step 2: 税率读取接口**

`internal/modules/sales/app/common.go` 增加：

```go
type TaxRateReader interface {
    FindByID(ctx context.Context, filter scope.Filter, tenantID, id string) (masterdomain.TaxRate, error)
}
```

- [ ] **Step 3: 订单服务和税率解析**

`OrderService` 增加 `taxRates TaxRateReader`，构造函数增加参数。`OrderLineInput` 增加 `TaxRateID string`，`OrderInput` 增加 `PriceIncludesTax bool`。`buildLines` 改为返回 error，并解析：

```go
rateID := strings.TrimSpace(input.TaxRateID)
if rateID == "" { rateID = material.TaxRateID }
rate := decimal.Zero
if rateID != "" {
    taxRate, err := s.taxRates.FindByID(ctx, filter, tenantID, rateID)
    if err != nil || taxRate.Status != masterdomain.StatusActive { return domain.ErrInvalidInput }
    rate = decimal.RequireFromString(taxRate.Rate)
}
```

把 `tax_rate_id`、`tax_rate`、计算结果写入行。

- [ ] **Step 4: 发货复制税率并计算**

`ShipmentLine` 从 `orderLine` 复制 `TaxRateID`、`TaxRate`；用 `taxcalc.Calculate(quantity, orderLine.UnitPrice, orderLine.TaxRate, order.PriceIncludesTax)` 设置 `Amount`/`TaxAmount`/`GrossAmount`；表头设置 `PriceIncludesTax=order.PriceIncludesTax`。

- [ ] **Step 5: 单元测试**

扩展 `order_test.go`：含税订单 2 行（13%、0%）的税额/净额/总额；不含税订单；税率覆盖。

Run: `go test ./internal/modules/sales/...`
Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/modules/sales cmd/api/main.go cmd/demo/main.go
git commit -m "feat(stage-3e): add sales tax amounts"
```

---

## Task 4: 销售仓储、HTTP DTO 和自动凭证

**Files:**
- Modify: `internal/modules/sales/infra/postgres/repository.go`
- Modify: `internal/modules/sales/http/handler.go`
- Modify: `internal/modules/gl/infra/postgres/posting.go`
- Modify: `internal/modules/gl/infra/postgres/backfill.go`

- [ ] **Step 1: 仓储 SQL**

`INSERT/UPDATE sales_orders` 增加 `price_includes_tax, net_amount, tax_amount`；`insertLines` 增加税率和金额列；`loadOrderLines`/`loadShipmentLines` 和扫描函数增加字段。

`Ship`：

- 插入 `sales_shipments` 增加 `price_includes_tax, net_amount, tax_amount`。
- 插入发货行增加税率/净额/税额/含税金额。
- `invoiceTotal` 改为 `Σ gross_amount`；新增 `netTotal`/`taxTotal`。
- 插入 `finance_invoices` 增加 `net_amount, tax_amount, vat_invoice_status`（默认 pending）。
- 调用 GL 传入 `NetAmount`、`TaxAmount`、`GrossAmount`。

- [ ] **Step 2: HTTP DTO**

`orderLineRequest` 增加 `tax_rate_id`；`orderRequest` 增加 `price_includes_tax *bool`，解析时缺省 true；响应增加 `price_includes_tax`、`net_amount`、`tax_amount`、行 `tax_rate_id`、`tax_rate`、`tax_amount`、`gross_amount`。发货响应同样增加。

- [ ] **Step 3: GL 销售凭证**

`SalesShipmentPosting` 改为 `NetAmount/TaxAmount/GrossAmount/CostAmount`：

```go
if input.GrossAmount.IsPositive() {
    lines = append(lines AS AR debit Gross, revenue credit Net)
    if input.TaxAmount.IsPositive() {
        lines = append(lines, postingLine{AccountKey: "output_vat", Summary: "销项税额", Credit: input.TaxAmount, PartnerID: input.CustomerID})
    }
}
```

- [ ] **Step 4: 回填兼容**

`backfill.go` 销售查询改为 `SUM(amount), SUM(tax_amount), SUM(gross_amount), SUM(cost_amount)`，传新字段。

- [ ] **Step 5: 测试和提交**

Run: `go build ./... && go test ./internal/modules/sales/...`
Expected: PASS。

```bash
git add internal/modules/sales internal/modules/gl/infra/postgres
git commit -m "feat(stage-3e): post sales output tax"
```

---

## Task 5: 采购域、仓储、HTTP 和自动凭证

**Files:**
- Modify: `internal/modules/purchase/domain/order.go`
- Modify: `internal/modules/purchase/domain/receipt.go`
- Modify: `internal/modules/purchase/domain/order_test.go`
- Modify: `internal/modules/purchase/app/common.go`
- Modify: `internal/modules/purchase/app/order_service.go`
- Modify: `internal/modules/purchase/app/receipt_service.go`
- Modify: `internal/modules/purchase/infra/postgres/order_repository.go`
- Modify: `internal/modules/purchase/http/handler.go`
- Modify: `internal/modules/gl/infra/postgres/posting.go`
- Modify: `internal/modules/gl/infra/postgres/backfill.go`
- Modify: `cmd/api/main.go`
- Modify: `cmd/demo/main.go`

- [ ] **Step 1: 采购域模型**

采购 `Order` 增加 `PriceIncludesTax bool`、`NetAmount`、`TaxAmount`；`OrderLine` 增加 `TaxRateID`、`TaxRate`、`TaxAmount`、`GrossAmount`。`Recalculate()` 与销售一致，使用 `taxcalc.Calculate` 并汇总净额、税额、含税总额。

采购 `Receipt` 增加 `PriceIncludesTax bool`、`NetAmount`、`TaxAmount`；`ReceiptLine` 增加 `TaxRateID`、`TaxRate`、`TaxAmount`、`GrossAmount`。新增 `TotalNet()`、`TotalTax()`、`TotalAmount()`。

`purchase/app/common.go` 增加 `TaxRateReader` 接口（与销售相同签名）。`OrderService` 构造函数增加 `taxRates`；`OrderLineInput` 增加 `TaxRateID`，`OrderInput` 增加 `PriceIncludesTax`；`buildLines` 返回 error，并按“物料默认税率 → 行覆盖税率 → 快照 `tax_rate`”解析。

`Receive` 从订单行复制 `TaxRateID`、`TaxRate`，用 `taxcalc.Calculate(quantity, orderLine.UnitPrice, orderLine.TaxRate, order.PriceIncludesTax)` 计算收货行净额、税额、含税金额；表头设置 `PriceIncludesTax=order.PriceIncludesTax`。

- [ ] **Step 2: 仓储和发票**

`Receive` 插入头/行新字段；`invoiceTotal = Σ gross_amount`；`finance_invoices` 写入 `net_amount`、`tax_amount`、`vat_invoice_status='pending'`；库存余额和库存流水使用不含税 `amount`。

- [ ] **Step 3: GL 采购凭证**

`PurchaseReceiptPosting` 改为 `NetAmount/TaxAmount/GrossAmount`：

```go
lines := []postingLine{
    {AccountKey: "inventory", Summary: "采购入库", Debit: input.NetAmount},
}
if input.TaxAmount.IsPositive() {
    lines = append(lines, postingLine{AccountKey: "input_vat", Summary: "进项税额", Debit: input.TaxAmount})
}
lines = append(lines, postingLine{AccountKey: "ap", Summary: "采购应付", Credit: input.GrossAmount, PartnerID: input.SupplierID})
```

- [ ] **Step 4: 回填、测试、提交**

Run: `go build ./... && go test ./internal/modules/purchase/...`

```bash
git add internal/modules/purchase internal/modules/gl/infra/postgres cmd/api/main.go cmd/demo/main.go
git commit -m "feat(stage-3e): post purchase input tax"
```

---

## Task 6: 发票开票与作废

**Files:**
- Modify: `internal/modules/finance/domain/invoice.go`
- Modify: `internal/modules/finance/domain/repository.go`
- Modify: `internal/modules/finance/app/invoice_service.go`
- Modify: `internal/modules/finance/http/handler.go`
- Modify: `internal/modules/finance/infra/postgres/invoice_repository.go`

- [ ] **Step 1: 域模型和接口**

`Invoice` 增加字段；常量：

```go
VatInvoiceStatusPending = "pending"
VatInvoiceStatusIssued  = "issued"
VatInvoiceStatusVoid    = "void"
VatInvoiceTypeSpecial   = "special"
VatInvoiceTypeGeneral   = "general"
```

`InvoiceRepository` 增加：

```go
Issue(ctx, filter, tenantID, id, input IssueInput) (*Invoice, error)
Void(ctx, filter, tenantID, id, userID string) (*Invoice, error)
```

`IssueInput` 包含发票号、类型、日期、购销方税号。

- [ ] **Step 2: 仓储**

`invoiceSelectSQL`/`scanInvoice` 增加 `net_amount, tax_amount, vat_invoice_*` 等字段。`List` 增加 `vatInvoiceStatus` 参数和 `vat_invoice_no` 关键字条件。`Issue` 使用 `UPDATE ... WHERE status='pending' AND vat_invoice_status='pending'`，唯一索引冲突映射 `ErrConflict`；`Void` 使用 `WHERE vat_invoice_status='issued'`。

- [ ] **Step 3: 服务和 HTTP**

`InvoiceService.Issue/Void` 调用仓储；HTTP 增加：

```go
finance.POST("/invoices/:id/issue", authmw.RequirePermission("finance.invoice.write"), handler.issueInvoice)
finance.POST("/invoices/:id/void", authmw.RequirePermission("finance.invoice.write"), handler.voidInvoice)
```

请求 DTO：

```go
type issueInvoiceRequest struct {
    VatInvoiceNo   string `json:"vat_invoice_no"`
    VatInvoiceType string `json:"vat_invoice_type"`
    VatInvoiceDate string `json:"vat_invoice_date"`
    BuyerTaxNo     string `json:"buyer_tax_no"`
    SellerTaxNo    string `json:"seller_tax_no"`
}
```

校验发票号必填、类型 `special/general`、日期合法；错误映射 `409 INVOICE_STATUS`、`409 VAT_INVOICE_NO_DUPLICATE`。

- [ ] **Step 4: 测试和提交**

Run: `go build ./... && go test ./internal/modules/finance/...`

```bash
git add internal/modules/finance
git commit -m "feat(stage-3e): add invoice issue and void"
```

---

## Task 7: 增值税汇总表

**Files:**
- Modify: `internal/modules/gl/domain/report.go`
- Modify: `internal/modules/gl/domain/repository.go`
- Modify: `internal/modules/gl/infra/postgres/report_repository.go`
- Modify: `internal/modules/gl/app/report_service.go`
- Modify: `internal/modules/gl/http/handler.go`

- [ ] **Step 1: 域类型**

```go
type VATSummaryRow struct {
    Month        string
    OutputTax    decimal.Decimal
    InputTax     decimal.Decimal
    NetPayable   decimal.Decimal
}
type VATSummary struct {
    StartDate, EndDate time.Time
    Rows []*VATSummaryRow
    TotalOutputTax, TotalInputTax, TotalNetPayable decimal.Decimal
}
```

- [ ] **Step 2: 查询**

新增 `VATSummary(ctx, filter, tenantID, startDate, endDate)`。SQL 按 `to_char(v.voucher_date,'YYYY-MM')` 分组：

```sql
SUM(CASE WHEN m.key = 'output_vat' THEN l.credit - l.debit ELSE 0 END)
SUM(CASE WHEN m.key = 'input_vat' THEN l.debit - l.credit ELSE 0 END)
```

只统计 `posted/reversed`，应用 `appendScope`。

- [ ] **Step 3: 服务和路由**

`GET /finance/reports/vat-summary`，权限 `finance.report.read`，返回字符串金额。

- [ ] **Step 4: 测试和提交**

Run: `go build ./... && go test ./internal/modules/gl/...`

```bash
git add internal/modules/gl
git commit -m "feat(stage-3e): add VAT summary report"
```

---

## Task 8: Seed/Demo 与真实税额样例

**Files:**
- Modify: `cmd/seed/main.go`
- Modify: `cmd/demo/main.go`

- [ ] **Step 1: Demo 税率和含税订单**

给 Demo 销售/采购订单行设置税率（如 VAT13/VAT9），确保 `price_includes_tax=true`，金额由新的 `Recalculate` 计算。增加一张不含税订单示例。

- [ ] **Step 2: Demo 发票开票样例**

为一张应收发票设置 `issued` 状态和税控发票号，便于前端展示；其余保持 `pending`。

- [ ] **Step 3: 运行 demo**

Run: `go run ./cmd/demo`
Expected: 成功，无唯一约束/税额错误。

- [ ] **Step 4: Commit**

```bash
git add cmd/seed/main.go cmd/demo/main.go
git commit -m "feat(stage-3e): seed tax demo data"
```

---

## Task 9: SDK、OpenAPI 和前端完整度

**Files:**
- Modify: `packages/sdk/src/sales.ts`
- Modify: `packages/sdk/src/purchase.ts`
- Modify: `packages/sdk/src/finance.ts`
- Modify: `packages/sdk/src/gl.ts`
- Modify: `api/openapi.yaml`
- Modify: `scripts/test/check-frontend.mjs`
- Modify: `package.json`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: SDK 类型**

销售/采购订单、发货、收货、发票类型增加 `price_includes_tax`、`net_amount`、`tax_amount`、行税率字段；发票增加 `vat_invoice_*`；`financeApi` 增加 `issueInvoice`/`voidInvoice`；`glApi.reports` 增加 `vatSummary`。

- [ ] **Step 2: OpenAPI**

补充所有新字段、发票开票/作废接口、增值税汇总接口、`finance.invoice.write` 权限说明；版本更新 `0.6.0`。

- [ ] **Step 3: 前端完整度脚本**

检查 SDK 包含 `issueInvoice`、`voidInvoice`、`vatSummary`、`tax_rate_id`、`vat_invoice_status`；页面包含 `vat`、`tax_amount` 等标记。

- [ ] **Step 4: CI**

Web 任务运行 `pnpm test:frontend`；API 集成测试仍使用 `-run 'TestStage3'`，新测试命名 `TestStage3E` 自动纳入。

- [ ] **Step 5: Commit**

```bash
git add packages/sdk api/openapi.yaml scripts/test/check-frontend.mjs package.json .github/workflows/ci.yml
git commit -m "feat(stage-3e): update SDK and API contract"
```

---

## Task 10: 销售/采购前端价税分离

**Files:**
- Modify: `apps/web/src/views/sales/OrderListView.vue`
- Modify: `apps/web/src/views/sales/ShipmentListView.vue`
- Modify: `apps/web/src/views/purchase/OrderListView.vue`

- [ ] **Step 1: 税率选项**

页面加载 `masterdataApi.taxRates()`（或现有 SDK 方法），建立 `taxRates` 列表；订单行税率默认取物料 `tax_rate_id`。

- [ ] **Step 2: 表单**

表头增加：

```vue
<el-form-item label="价格口径">
  <el-switch v-model="form.price_includes_tax" active-text="含税" inactive-text="不含税" />
</el-form-item>
```

行增加税率下拉；行金额列改为净额、税额、含税金额三列或“金额（含税）”+“税额”两列。表头展示三项合计。

- [ ] **Step 3: 详情**

订单、发货详情展示税率、净额、税额、含税金额；发货详情表头展示含税总额。

- [ ] **Step 4: 校验和构建**

Run: `pnpm typecheck:web && pnpm test:frontend && pnpm build:web`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/views/sales apps/web/src/views/purchase
git commit -m "feat(stage-3e): add tax fields to sales and purchase UI"
```

---

## Task 11: 发票页面和增值税报表页面

**Files:**
- Modify: `apps/web/src/views/finance/InvoiceListView.vue`
- Modify: `apps/web/src/views/finance/ReportView.vue`

- [ ] **Step 1: 发票列表/详情**

列表增加开票状态标签、增值税发票号、净额、税额；增加按 `vat_invoice_status` 过滤；详情展示全部发票字段。

- [ ] **Step 2: 开票/作废对话框**

“开票”对话框包含发票号、类型（专票/普票）、开票日期、购销方税号；提交 `financeApi.issueInvoice`。作废使用确认框调用 `voidInvoice`。根据 `canWrite` 权限显示按钮，仅 `pending`/`issued` 状态可用。

- [ ] **Step 3: 增值税汇总 Tab**

`ReportView.vue` 增加 Tab 和日期范围、表格（月份、销项、进项、应纳税额）及合计；调用 `glApi.reports.vatSummary`。

- [ ] **Step 4: 构建和提交**

Run: `pnpm typecheck:web && pnpm test:frontend && pnpm build:web`

```bash
git add apps/web/src/views/finance
git commit -m "feat(stage-3e): add invoice and VAT summary UI"
```

---

## Task 12: Stage 3E 接口自动化测试

**Files:**
- Create: `tests/api/stage3e_test.go`
- Create: `scripts/test/stage3e-api.ps1`
- Modify: `package.json`
- Modify: `README.md`

- [ ] **Step 1: 测试用例**

`TestStage3E` 至少覆盖：

1. 含税销售订单（13%）：行净额/税额/含税金额、表头合计。
2. 提交/审核/发货：应收发票含税总额、收入净额、销项税、自动凭证科目和金额。
3. 不含税采购订单（9%）：收货库存净额、进项税、应付含税、自动凭证。
4. 含税/不含税混合税率订单。
5. 发票开票：状态 `pending→issued`、发票号唯一、重复返回 409。
6. 发票作废：`issued→void`，重复作废 409；作废后不能再次开票。
7. 增值税汇总：销项/进项/应纳税额与凭证一致。
8. 权限：无 token 401；无 `finance.invoice.write` 403。
9. 历史数据兼容：旧发票净额=总额、税额=0、状态 pending。

- [ ] **Step 2: 测试脚本**

复制 `stage3d-api.ps1` 为 `stage3e-api.ps1`，改 `TestStage3E`、`api-stage3e`。`package.json` 增加 `test:api:stage3e`。

- [ ] **Step 3: 运行**

Run: `pnpm test:api:stage3e`
Expected: PASS。

- [ ] **Step 4: 回归**

Run: `pnpm test:api:stage3b && pnpm test:api:stage3c && pnpm test:api:stage3d`
Expected: PASS（Stage 3D 年度用例在干净库执行）。

- [ ] **Step 5: Commit**

```bash
git add tests/api/stage3e_test.go scripts/test/stage3e-api.ps1 package.json README.md
git commit -m "test(stage-3e): add tax and invoice API tests"
```

---

## Task 13: 文档、静态检查和汇总报告

**Files:**
- Modify: `README.md`
- Modify: `internal/modules/gl/README.md`
- Modify: `internal/modules/sales/README.md`
- Modify: `internal/modules/purchase/README.md`
- Modify: `internal/modules/finance/README.md`
- Create: `docs/reports/2026-11-17-stage-3e-tax-invoice.md`

- [ ] **Step 1: 模块文档**

补充价税分离字段、自动凭证规则、发票流程、增值税汇总和已知限制。

- [ ] **Step 2: 静态检查和构建**

Run:

```bash
gofmt -w ./cmd ./internal ./db ./tests
gofmt -l ./cmd ./internal ./db ./tests
go test ./...
go vet ./...
pnpm typecheck:web
pnpm test:frontend
pnpm build:web
npx -y yaml@2 valid api/openapi.yaml
```

Expected: 全部通过。

- [ ] **Step 3: 汇总报告**

报告包含完成范围、数据库/API、测试结果、代码审核（独立子代理）、已知限制、下一阶段建议和用户审阅清单。

- [ ] **Step 4: 提交并推送**

```bash
git add -A
git commit -m "docs(stage-3e): finalize tax and invoice report"
git push -u origin stage-3e-tax-invoice
```

---

## 验收清单

- [ ] 含税/不含税单据金额正确。
- [ ] 行级税率默认物料、可覆盖、快照保存。
- [ ] 销售发货自动凭证拆分销项税。
- [ ] 采购收货自动凭证拆分进项税。
- [ ] 发票开票/作废和发票号唯一。
- [ ] 增值税汇总表可用。
- [ ] 历史数据兼容。
- [ ] 前端、SDK、OpenAPI、迁移、seed/demo 完整。
- [ ] 自动化测试、静态检查、构建通过。
- [ ] 代码审核和汇总报告完成。
