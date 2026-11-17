# Stage 3F 多币种、汇率与汇兑损益 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为销售、采购、发票、收付款和总账增加 CNY 本位币折算、已实现汇兑损益和期末外币应收应付重估。

**Architecture:** 汇率表保存币种日汇率；业务单据保存原币金额、汇率快照和本位币金额；总账只记本位币。收付款核销按付款汇率与发票汇率差额生成汇兑损益凭证；期间关账时重估外币应收应付并在次期反向。

**Tech Stack:** Go 1.25 + Gin + pgx + PostgreSQL + shopspring/decimal；Vue 3 + TypeScript + Element Plus。

**Design:** `docs/superpowers/specs/2026-11-17-stage-3f-multi-currency-design.md`

---

## Task 1: 迁移、汇率表、汇兑科目和权限

**Files:**
- Create: `db/migrations/000018_multi_currency.up.sql`
- Create: `db/migrations/000018_multi_currency.down.sql`
- Modify: `db/migrations/README.md`
- Modify: `cmd/seed/main.go`

- [ ] **Step 1: 写 up 迁移**

```sql
CREATE TABLE IF NOT EXISTS erp.finance_exchange_rates (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    currency_code text NOT NULL,
    rate_date date NOT NULL,
    rate numeric(19,8) NOT NULL,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_exchange_rates_tenant_rate_key UNIQUE (tenant_id, currency_code, rate_date),
    CONSTRAINT finance_exchange_rates_rate_check CHECK (rate > 0)
);
CREATE INDEX IF NOT EXISTS finance_exchange_rates_lookup_idx
    ON erp.finance_exchange_rates (tenant_id, currency_code, rate_date DESC);

ALTER TABLE erp.sales_orders
    ADD COLUMN IF NOT EXISTS exchange_rate numeric(19,8) NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS base_net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_total_amount numeric(19,4) NOT NULL DEFAULT 0;
ALTER TABLE erp.sales_order_lines
    ADD COLUMN IF NOT EXISTS base_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_gross_amount numeric(19,4) NOT NULL DEFAULT 0;
-- purchase_orders / purchase_order_lines 同样
-- sales_shipments / sales_shipment_lines 同样
-- purchase_receipts / purchase_receipt_lines 同样

ALTER TABLE erp.finance_invoices
    ADD COLUMN IF NOT EXISTS exchange_rate numeric(19,8) NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS base_net_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_tax_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_total_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_paid_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_revalued_amount numeric(19,4) NOT NULL DEFAULT 0;
ALTER TABLE erp.finance_payments
    ADD COLUMN IF NOT EXISTS exchange_rate numeric(19,8) NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS base_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_allocated_amount numeric(19,4) NOT NULL DEFAULT 0;
ALTER TABLE erp.finance_payment_allocations
    ADD COLUMN IF NOT EXISTS base_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS invoice_base_amount numeric(19,4) NOT NULL DEFAULT 0;

UPDATE erp.sales_orders SET base_net_amount = net_amount, base_tax_amount = tax_amount, base_total_amount = total_amount WHERE base_total_amount = 0;
UPDATE erp.sales_order_lines SET base_amount = amount, base_tax_amount = tax_amount, base_gross_amount = gross_amount WHERE base_gross_amount = 0;
-- 其余单据、发票、收付款、核销同样回填
UPDATE erp.finance_invoices SET base_paid_amount = paid_amount WHERE base_paid_amount = 0;

CREATE TABLE IF NOT EXISTS erp.finance_fx_settlements (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    payment_id uuid NOT NULL,
    invoice_id uuid NOT NULL,
    amount numeric(19,4) NOT NULL,
    payment_rate numeric(19,8) NOT NULL,
    invoice_rate numeric(19,8) NOT NULL,
    base_adjustment numeric(19,4) NOT NULL,
    voucher_id uuid,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS erp.gl_fx_revaluations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    period_id uuid NOT NULL,
    invoice_id uuid NOT NULL,
    invoice_type text NOT NULL,
    currency text NOT NULL,
    foreign_balance numeric(19,4) NOT NULL,
    old_rate numeric(19,8) NOT NULL,
    new_rate numeric(19,8) NOT NULL,
    base_adjustment numeric(19,4) NOT NULL,
    status text NOT NULL DEFAULT 'posted',
    voucher_id uuid,
    reversal_voucher_id uuid,
    reopen_voucher_id uuid,
    reopen_reversal_voucher_id uuid,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    reversed_at timestamptz,
    CONSTRAINT gl_fx_revaluations_status_check CHECK (status IN ('posted', 'reopened'))
);
CREATE UNIQUE INDEX IF NOT EXISTS gl_fx_revaluations_posted_key
    ON erp.gl_fx_revaluations (tenant_id, period_id, invoice_id)
    WHERE status = 'posted';

INSERT INTO erp.gl_accounts (id, tenant_id, code, name, type, direction, level, is_leaf, status)
SELECT gen_random_uuid(), t.id, a.code, a.name, 'expense', a.direction, 3, true, 'active'
FROM erp.tenants t
CROSS JOIN (VALUES
    ('660301', '汇兑损失', 'debit'),
    ('660302', '汇兑收益', 'credit')
) AS a(code, name, direction)
ON CONFLICT (tenant_id, code) DO NOTHING;
UPDATE erp.gl_accounts SET is_leaf = false WHERE code = '6603';
UPDATE erp.gl_accounts child SET parent_id = parent.id
FROM erp.gl_accounts parent
WHERE child.tenant_id = parent.tenant_id AND parent.code = '6603'
  AND child.code IN ('660301', '660302');

INSERT INTO erp.gl_account_mappings (tenant_id, key, account_code)
SELECT t.id, m.key, m.account_code
FROM erp.tenants t
CROSS JOIN (VALUES ('fx_loss', '660301'), ('fx_gain', '660302')) AS m(key, account_code)
ON CONFLICT DO NOTHING;

INSERT INTO erp.permissions (code, name, module, type)
VALUES
    ('finance.exchange_rate.read', '查看汇率', 'finance', 'api'),
    ('finance.exchange_rate.write', '管理汇率', 'finance', 'api')
ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, module = EXCLUDED.module, type = EXCLUDED.type;
INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, p.code
FROM erp.roles r CROSS JOIN (VALUES ('finance.exchange_rate.read'), ('finance.exchange_rate.write')) AS p(code)
WHERE r.code = 'admin'
ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: 写 down 迁移**

删除新增表、列、科目、映射和权限；仅删除无发生额的 660301/660302，并把 6603 在无子科目时恢复末级。

- [ ] **Step 3: seed**

标准科目加入 660301/660302，6603 改非末级；父级映射和 `fx_loss`/`fx_gain`；权限列表加入汇率权限。

- [ ] **Step 4: 验证**

Run: `go run ./cmd/migrate -direction up && go run ./cmd/seed`
Expected: 迁移和 seed 成功；`select code,is_leaf from gl_accounts where code like '6603%'` 显示 6603 非末级、两个子科目末级。

- [ ] **Step 5: Commit**

```bash
git add db/migrations cmd/seed/main.go
git commit -m "feat(stage-3f): add multi-currency schema"
```

---

## Task 2: 共享汇率计算包

**Files:**
- Create: `internal/platform/fx/fx.go`
- Create: `internal/platform/fx/fx_test.go`

- [ ] **Step 1: 写测试**

覆盖：CNY 固定 1；USD 7.2 折算 100 → 720；4 位舍入；负数/零汇率拒绝；`Diff` 正负；`Resolve` 覆盖优先、无汇率报错。

- [ ] **Step 2: 实现**

```go
package fx

const BaseCurrency = "CNY"

var ErrRateNotFound = errors.New("exchange rate not found")

type RateReader interface {
    Latest(ctx context.Context, tenantID, currency string, date time.Time) (decimal.Decimal, error)
}

func IsBase(currency string) bool { return strings.EqualFold(currency, BaseCurrency) }

func Convert(amount, rate decimal.Decimal) (decimal.Decimal, error) {
    if rate.LessThanOrEqual(decimal.Zero) { return decimal.Zero, ErrInvalidRate }
    return amount.Mul(rate).Round(4), nil
}

func Diff(amount, paymentRate, invoiceRate decimal.Decimal) decimal.Decimal {
    return amount.Mul(paymentRate.Sub(invoiceRate)).Round(4)
}

func Resolve(ctx context.Context, reader RateReader, tenantID, currency string, date time.Time, override decimal.Decimal) (decimal.Decimal, error) {
    // base -> 1; override > 0 -> override; else reader.Latest
}
```

- [ ] **Step 3: 测试并提交**

Run: `go test ./internal/platform/fx -v`

```bash
git add internal/platform/fx
git commit -m "feat(stage-3f): add FX conversion helpers"
```

---

## Task 3: 汇率管理 API

**Files:**
- Create: `internal/modules/finance/domain/exchange_rate.go`
- Modify: `internal/modules/finance/domain/repository.go`
- Create: `internal/modules/finance/app/exchange_rate_service.go`
- Create: `internal/modules/finance/infra/postgres/exchange_rate_repository.go`
- Modify: `internal/modules/finance/http/handler.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: 域和仓储**

```go
type ExchangeRate struct {
    ID, TenantID, CurrencyCode string
    RateDate time.Time
    Rate decimal.Decimal
    CreatedAt, UpdatedAt time.Time
}
type ExchangeRateInput struct { CurrencyCode string; RateDate time.Time; Rate decimal.Decimal }

type ExchangeRateRepository interface {
    List(ctx, tenantID, currency string, start, end time.Time, offset, limit int) ([]*ExchangeRate, int, error)
    Get(ctx, tenantID, id string) (*ExchangeRate, error)
    Upsert(ctx, tenantID, userID string, input ExchangeRateInput) (*ExchangeRate, error)
    Update(ctx, tenantID, id, userID string, input ExchangeRateInput) (*ExchangeRate, error)
    Latest(ctx, tenantID, currency string, date time.Time) (decimal.Decimal, error)
}
```

- [ ] **Step 2: SQL**

- `List`：按币种/日期范围过滤，按 `rate_date DESC`。
- `Upsert`：`INSERT ... ON CONFLICT (tenant_id, currency_code, rate_date) DO UPDATE SET rate=EXCLUDED.rate, updated_at=now()`。
- `Latest`：`SELECT rate FROM ... WHERE tenant_id=$1 AND currency_code=$2 AND rate_date <= $3 ORDER BY rate_date DESC LIMIT 1`，无记录返回 `fx.ErrRateNotFound`。
- 禁止 CNY 和非正汇率。

- [ ] **Step 3: 服务/HTTP**

Routes:

```go
finance.GET("/exchange-rates", authmw.RequirePermission("finance.exchange_rate.read"), handler.listExchangeRates)
finance.GET("/exchange-rates/latest", authmw.RequirePermission("finance.exchange_rate.read"), handler.latestExchangeRate)
finance.POST("/exchange-rates", authmw.RequirePermission("finance.exchange_rate.write"), handler.upsertExchangeRate)
finance.PUT("/exchange-rates/:id", authmw.RequirePermission("finance.exchange_rate.write"), handler.updateExchangeRate)
```

响应金额字符串 8 位。

- [ ] **Step 4: 测试/提交**

Run: `go build ./...`

```bash
git add internal/modules/finance cmd/api/main.go
git commit -m "feat(stage-3f): add exchange rate API"
```

---

## Task 4: 销售单据多币种

**Files:**
- Modify: `internal/modules/sales/domain/order.go`
- Modify: `internal/modules/sales/domain/shipment.go`
- Modify: `internal/modules/sales/app/order_service.go`
- Modify: `internal/modules/sales/app/shipment_service.go`
- Modify: `internal/modules/sales/http/handler.go`
- Modify: `internal/modules/sales/infra/postgres/repository.go`
- Modify: `internal/modules/sales/README.md`
- Modify: `cmd/api/main.go`, `cmd/demo/main.go`

- [ ] **Step 1: 域模型**

订单/发货头和行增加 `ExchangeRate`、`BaseNetAmount/BaseTaxAmount/BaseTotalAmount`；行增加 `BaseAmount/BaseTaxAmount/BaseGrossAmount`。

`Recalculate`：

```go
baseNet := result.Net.Mul(o.ExchangeRate).Round(4)
baseGross := result.Gross.Mul(o.ExchangeRate).Round(4)
baseTax := baseGross.Sub(baseNet).Round(4)
```

- [ ] **Step 2: 服务**

`OrderService` 增加 `rates fx.RateReader`；`OrderInput` 增加 `ExchangeRate decimal.Decimal`。
创建/编辑时：

```go
rate, err := fx.Resolve(ctx, s.rates, tenantID, currency, orderDate, input.ExchangeRate)
```

发货服务增加 `rates fx.RateReader` 和 `ShipInput.ExchangeRate`；按发货日期取汇率，重新计算行/表头本位币金额。

- [ ] **Step 3: 仓储/SQL**

订单、发货头/行插入、更新、查询、扫描增加汇率和本位币列。应收发票插入增加 `exchange_rate, base_net_amount, base_tax_amount, base_total_amount, base_paid_amount`。GL 调用改传本位币字段。

- [ ] **Step 4: HTTP/SDK 响应**

订单/发货请求增加 `exchange_rate` 字符串，响应增加 `exchange_rate`、`base_net_amount`、`base_tax_amount`、`base_total_amount` 和行本位币字段。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/sales cmd/api/main.go cmd/demo/main.go
git commit -m "feat(stage-3f): add sales multi-currency amounts"
```

---

## Task 5: 采购单据多币种

**Files:**
- Modify: `internal/modules/purchase/domain/order.go`
- Modify: `internal/modules/purchase/domain/receipt.go`
- Modify: `internal/modules/purchase/app/order_service.go`
- Modify: `internal/modules/purchase/app/receipt_service.go`
- Modify: `internal/modules/purchase/http/handler.go`
- Modify: `internal/modules/purchase/infra/postgres/order_repository.go`
- Modify: `internal/modules/purchase/README.md`
- Modify: `cmd/api/main.go`, `cmd/demo/main.go`

按 Task 4 实现采购订单/收货的汇率、本位币金额；收货库存成本使用本位币不含税金额，应付发票使用本位币含税金额。

---

## Task 6: 总账本位币和期末重估

**Files:**
- Modify: `internal/modules/gl/infra/postgres/posting.go`
- Modify: `internal/modules/gl/infra/postgres/backfill.go`
- Modify: `internal/modules/gl/infra/postgres/period_repository.go`
- Modify: `internal/modules/gl/domain/errors.go`
- Modify: `internal/modules/gl/domain/voucher.go`
- Modify: `internal/modules/gl/README.md`

- [ ] **Step 1: 自动凭证字段**

`SalesShipmentPosting`/`PurchaseReceiptPosting` 使用 `BaseNetAmount/BaseTaxAmount/BaseGrossAmount`；`PaymentPosting` 使用 `BaseAmount`。SQL 和回填查询改为本位币列。

- [ ] **Step 2: 重估**

`PeriodRepository.Close` 在 `closePeriodTx` 前调用 `revalueForeignInvoicesTx(ctx, tx, tenantID, orgID, period, userID, code)`。

SQL 查询：

```sql
SELECT i.id, i.invoice_type, i.currency, i.total_amount, i.paid_amount,
       i.exchange_rate, i.base_total_amount, i.base_paid_amount, i.base_revalued_amount
FROM erp.finance_invoices i
WHERE i.tenant_id=$1 AND i.currency <> 'CNY'
  AND i.total_amount > i.paid_amount
  AND i.invoice_date <= $2
FOR UPDATE OF i
```

对每张发票：

```go
foreign := total.Sub(paid)
closing, err := latestRateTx(...)
target := foreign.Mul(closing).Round(4)
book := baseTotal.Sub(basePaid).Add(baseRevalued)
adjustment := target.Sub(book).Round(4)
```

生成重估凭证和次期反向凭证；写 `gl_fx_revaluations`，更新 `base_revalued_amount`。

- [ ] **Step 3: 反关账撤销重估**

`Reopen` 在 P&L 反结转前：

- 查 `gl_fx_revaluations` `status='posted'` 的本期记录。
- 生成反向重估凭证（本期）和反向次期凭证（次期），更新记录 `reopened`，回退 `base_revalued_amount`。
- 次期已关账返回 `ErrPeriodClosed`。

- [ ] **Step 4: 错误码**

新增 `ErrFXRateNotFound`、`ErrFXRevaluationExists`（如果采用限制）、`ErrFXPeriodClosed`，HTTP 映射 409/400。

- [ ] **Step 5: 测试/提交**

Run: `go test ./internal/modules/gl/...`

```bash
git add internal/modules/gl
git commit -m "feat(stage-3f): post base currency and FX revaluation"
```

---

## Task 7: 发票和收付款多币种/已实现汇兑损益

**Files:**
- Modify: `internal/modules/finance/domain/invoice.go`
- Modify: `internal/modules/finance/domain/payment.go`
- Modify: `internal/modules/finance/domain/repository.go`
- Modify: `internal/modules/finance/app/payment_service.go`
- Modify: `internal/modules/finance/http/handler.go`
- Modify: `internal/modules/finance/infra/postgres/invoice_repository.go`
- Modify: `internal/modules/finance/infra/postgres/payment_repository.go`
- Modify: `internal/modules/finance/README.md`

- [ ] **Step 1: 域字段**

发票增加 `ExchangeRate`、`BaseNetAmount`、`BaseTaxAmount`、`BaseTotalAmount`、`BasePaidAmount`、`BaseRevaluedAmount`。
收付款增加 `ExchangeRate`、`BaseAmount`、`BaseAllocatedAmount`。

- [ ] **Step 2: 收付款创建**

`PaymentService` 增加 `rates fx.RateReader`，`PaymentInput.ExchangeRate`；取付款日汇率并保存原币/本位币金额。GL 使用 `BaseAmount`。

- [ ] **Step 3: 核销**

`Allocate` 锁定付款和发票，读取两者汇率；计算：

```go
basePaid := allocationAmount.Mul(invoiceRate).Round(4)
basePayment := allocationAmount.Mul(paymentRate).Round(4)
fxDiff := basePayment.Sub(basePaid).Round(4)
```

更新发票 `paid_amount`、`base_paid_amount`；更新收付款 `allocated_amount`、`base_allocated_amount`；插入/累加 allocation 的原币/本位币金额；插入 `finance_fx_settlements`；调用 `glpost.PostFXSettlement` 生成凭证。

- [ ] **Step 4: HTTP/SDK**

发票/收付款响应增加汇率和本位币字段；创建收付款请求增加 `exchange_rate`。

- [ ] **Step 5: 测试/提交**

Run: `go build ./... && go test ./internal/modules/finance/...`

```bash
git add internal/modules/finance
git commit -m "feat(stage-3f): add FX settlement on allocation"
```

---

## Task 8: 前端汇率管理和多币种字段

**Files:**
- Create: `apps/web/src/views/finance/ExchangeRateListView.vue`
- Modify: `apps/web/src/router/index.ts`
- Modify: `apps/web/src/layouts/DefaultLayout.vue`
- Modify: `apps/web/src/views/sales/OrderListView.vue`
- Modify: `apps/web/src/views/purchase/OrderListView.vue`
- Modify: `apps/web/src/views/sales/ShipmentListView.vue`
- Modify: `apps/web/src/views/finance/InvoiceListView.vue`
- Modify: `apps/web/src/views/finance/PaymentListView.vue`

- [ ] **Step 1: 汇率页面**

列表、新增/编辑对话框：币种、生效日期、汇率（8 位）。权限 `finance.exchange_rate.read/write`。

- [ ] **Step 2: 销售/采购订单**

币种从 `masterDataApi.currencies` 加载；增加汇率输入（可空，后端自动取）；展示本位币净额/税额/总额。详情展示汇率和本位币。

- [ ] **Step 3: 发货/详情**

展示汇率和本位币金额。

- [ ] **Step 4: 发票/收付款**

发票列表/详情增加汇率、本位币总额/未核销；收付款表单增加汇率和本位币金额。

- [ ] **Step 5: 构建/提交**

Run: `pnpm typecheck:web && pnpm test:frontend && pnpm build:web`

```bash
git add apps/web
git commit -m "feat(stage-3f): add multi-currency UI"
```

---

## Task 9: SDK、OpenAPI 和完整度检查

**Files:**
- Modify: `packages/sdk/src/finance.ts`
- Modify: `packages/sdk/src/sales.ts`
- Modify: `packages/sdk/src/purchase.ts`
- Modify: `api/openapi.yaml`
- Modify: `scripts/test/check-frontend.mjs`

- [ ] **Step 1: SDK**

新增 `ExchangeRate`、`ExchangeRateInput`；`financeApi.exchangeRates.list/latest/upsert/update`。
销售/采购/发票/收付款类型增加 `exchange_rate`、`base_*` 字段。

- [ ] **Step 2: OpenAPI**

新增汇率接口和 schema；单据/发票/收付款字段；版本 `0.7.0`。

- [ ] **Step 3: 完整度检查**

检查汇率页面、订单汇率、本位币字段、SDK 方法。

- [ ] **Step 4: Commit**

```bash
git add packages/sdk api/openapi.yaml scripts/test/check-frontend.mjs
git commit -m "feat(stage-3f): update SDK and API contract"
```

---

## Task 10: Stage 3F 接口测试

**Files:**
- Create: `tests/api/stage3f_test.go`
- Create: `scripts/test/stage3f-api.ps1`
- Modify: `package.json`
- Modify: `README.md`

- [ ] **Step 1: 测试**

覆盖：

1. CNY 汇率固定 1；USD 汇率新增/查询/最近汇率。
2. USD 销售订单含税 113、汇率 7.2 → 本位币净额 720、税额 93.6、总额 813.6。
3. 发货按发货日汇率生成本位币凭证：借应收/贷收入/贷销项税。
4. USD 采购收货按汇率生成库存/进项税/应付本位币凭证。
5. 收款汇率高于发票汇率生成汇兑收益；低于生成汇兑损失。
6. 付款汇率差异生成对应汇兑损益。
7. 期末关账外币未核销发票重估，次期反向；重复关账幂等。
8. 反关账撤销重估和次期反向。
9. 缺失汇率返回 400；汇率权限 401/403。
10. CNY 单据保持原行为。

- [ ] **Step 2: 脚本/package**

复制 stage3e 脚本为 stage3f，`TestStage3F`；`package.json` 增加 `test:api:stage3f`。

- [ ] **Step 3: 回归**

Run: `pnpm test:api:stage3f`，以及 `TestStage3` 全链路。

- [ ] **Step 4: Commit**

```bash
git add tests/api/stage3f_test.go scripts/test/stage3f-api.ps1 package.json README.md
git commit -m "test(stage-3f): add multi-currency API tests"
```

---

## Task 11: 文档、静态检查和汇总报告

**Files:**
- Modify: `README.md`, module READMEs
- Create: `docs/reports/2026-11-17-stage-3f-multi-currency.md`

- [ ] **Step 1: 文档**

补充汇率、本位币、汇兑损益、重估和限制。

- [ ] **Step 2: 静态检查**

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

- [ ] **Step 3: 报告**

完成范围、数据库/API、测试结果、代码审核、限制和审阅清单。

- [ ] **Step 4: 提交推送**

```bash
git add -A
git commit -m "docs(stage-3f): finalize multi-currency report"
git push -u origin stage-3f-multi-currency
```

---

## 验收清单

- [ ] 汇率表和汇率管理页面可用。
- [ ] 外币单据原币/汇率/本位币金额正确。
- [ ] 自动凭证使用本位币。
- [ ] 收付款已实现汇兑损益正确。
- [ ] 期末重估和次期反向正确。
- [ ] 反关账撤销重估。
- [ ] 前端/SDK/OpenAPI/迁移完整。
- [ ] 自动化测试、静态检查、构建和回归通过。
- [ ] 代码审核和汇总报告完成。
