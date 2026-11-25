# Stage 3K 收付款反核销 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 支持收付款核销的整行反核销，回退发票/收付款余额、状态和已实现汇兑结算。

**Architecture:** 新增反核销审计表和汇兑结算反核销字段；反核销事务删除有效核销行、更新发票/收付款余额，并为已实现汇兑结算生成反方向凭证。复用现有 payment/invoice/GL 模块。

**Tech Stack:** Go 1.25 + Gin + pgx + PostgreSQL + shopspring/decimal；Vue 3 + TypeScript + Element Plus。

**Design:** `docs/superpowers/specs/2026-11-25-stage-3k-payment-reversals-design.md`

---

## Task 1: 迁移、权限和 seed

**Files:**
- Create: `db/migrations/000024_payment_reversals.up.sql`
- Create: `db/migrations/000024_payment_reversals.down.sql`
- Modify: `db/migrations/README.md`
- Modify: `cmd/seed/main.go`

- [ ] **Step 1: up 迁移**

```sql
CREATE TABLE IF NOT EXISTS erp.finance_payment_allocation_reversals (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    payment_id uuid NOT NULL,
    invoice_id uuid NOT NULL,
    allocation_id uuid NOT NULL,
    amount numeric(19,4) NOT NULL CHECK (amount > 0),
    base_amount numeric(19,4) NOT NULL CHECK (base_amount >= 0),
    invoice_base_amount numeric(19,4) NOT NULL CHECK (invoice_base_amount >= 0),
    reversal_date date NOT NULL,
    reason text NOT NULL,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_payment_allocation_reversals_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT finance_payment_allocation_reversals_payment_fk
        FOREIGN KEY (tenant_id, payment_id)
        REFERENCES erp.finance_payments (tenant_id, id),
    CONSTRAINT finance_payment_allocation_reversals_invoice_fk
        FOREIGN KEY (tenant_id, invoice_id)
        REFERENCES erp.finance_invoices (tenant_id, id)
);

CREATE INDEX IF NOT EXISTS finance_payment_allocation_reversals_payment_idx
    ON erp.finance_payment_allocation_reversals (tenant_id, payment_id, reversal_date DESC);
CREATE INDEX IF NOT EXISTS finance_payment_allocation_reversals_invoice_idx
    ON erp.finance_payment_allocation_reversals (tenant_id, invoice_id);

ALTER TABLE erp.finance_fx_settlements
    ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'posted',
    ADD COLUMN IF NOT EXISTS reversal_id uuid,
    ADD COLUMN IF NOT EXISTS reversal_voucher_id uuid,
    ADD COLUMN IF NOT EXISTS reversed_at timestamptz,
    ADD COLUMN IF NOT EXISTS reversed_by uuid;

ALTER TABLE erp.finance_fx_settlements
    ADD CONSTRAINT finance_fx_settlements_status_check
        CHECK (status IN ('posted', 'reversed')),
    ADD CONSTRAINT finance_fx_settlements_reversal_fk
        FOREIGN KEY (tenant_id, reversal_id)
        REFERENCES erp.finance_payment_allocation_reversals (tenant_id, id),
    ADD CONSTRAINT finance_fx_settlements_reversal_voucher_fk
        FOREIGN KEY (tenant_id, reversal_voucher_id)
        REFERENCES erp.gl_vouchers (tenant_id, id);

CREATE INDEX IF NOT EXISTS finance_fx_settlements_reversal_idx
    ON erp.finance_fx_settlements (tenant_id, payment_id, invoice_id, status);

INSERT INTO erp.permissions (code, name, module, type)
VALUES ('finance.payment.reverse', '收付款反核销', 'finance', 'api')
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name, module = EXCLUDED.module, type = EXCLUDED.type;

INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, 'finance.payment.reverse'
FROM erp.roles r
WHERE r.code = 'admin'
ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: down 迁移**

```sql
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM erp.finance_payment_allocation_reversals) THEN
        RAISE EXCEPTION 'cannot roll back 000024_payment_reversals while reversal records exist';
    END IF;
    IF EXISTS (SELECT 1 FROM erp.finance_fx_settlements WHERE status = 'reversed') THEN
        RAISE EXCEPTION 'cannot roll back 000024_payment_reversals while FX settlements are reversed';
    END IF;
END $$;

DELETE FROM erp.role_permissions WHERE permission_code = 'finance.payment.reverse';
DELETE FROM erp.permissions WHERE code = 'finance.payment.reverse';

ALTER TABLE erp.finance_fx_settlements
    DROP CONSTRAINT IF EXISTS finance_fx_settlements_status_check,
    DROP CONSTRAINT IF EXISTS finance_fx_settlements_reversal_fk,
    DROP CONSTRAINT IF EXISTS finance_fx_settlements_reversal_voucher_fk;
DROP INDEX IF EXISTS erp.finance_fx_settlements_reversal_idx;
ALTER TABLE erp.finance_fx_settlements
    DROP COLUMN IF EXISTS reversed_by,
    DROP COLUMN IF EXISTS reversed_at,
    DROP COLUMN IF EXISTS reversal_voucher_id,
    DROP COLUMN IF EXISTS reversal_id,
    DROP COLUMN IF EXISTS status;

DROP TABLE IF EXISTS erp.finance_payment_allocation_reversals;
```

- [ ] **Step 3: README/seed**

`db/migrations/README.md` add:

```markdown
- `000024_payment_reversals`：收付款反核销记录、汇兑结算反核销字段和 `finance.payment.reverse` 权限。Down 在有反核销数据或已反核销汇兑结算时拒绝执行。
```

`cmd/seed/main.go` permission list add:

```go
{"finance.payment.reverse", "收付款反核销", "finance", "api"},
```

- [ ] **Step 4: Apply/verify**

```bash
gofmt -w cmd/seed/main.go
go run ./cmd/migrate -direction up
go run ./cmd/seed
docker exec mlf-erp-dev-postgres-1 psql -U mlf -d mlf_erp -c "select version, dirty from schema_migrations; select table_name from information_schema.tables where table_schema='erp' and table_name='finance_payment_allocation_reversals'; select column_name from information_schema.columns where table_name='finance_fx_settlements' and column_name in ('status','reversal_id','reversal_voucher_id','reversed_at','reversed_by'); select code from erp.permissions where code='finance.payment.reverse';"
```

Expected: version 24, dirty=f; table/columns/permission exist.

- [ ] **Step 5: Commit**

```bash
git add db/migrations/000024_payment_reversals.up.sql db/migrations/000024_payment_reversals.down.sql db/migrations/README.md cmd/seed/main.go
git commit -m "feat(stage-3k): add payment reversal schema"
```

---

## Task 2: 领域模型、状态计算和接口

**Files:**
- Modify: `internal/modules/finance/domain/payment.go`
- Modify: `internal/modules/finance/domain/errors.go`
- Modify: `internal/modules/finance/domain/repository.go`
- Create: `internal/modules/finance/domain/payment_reversal.go`
- Create: `internal/modules/finance/domain/payment_reversal_test.go`

- [ ] **Step 1: 失败测试**

`payment_reversal_test.go`:

```go
package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestPaymentStatusAfterReversal(t *testing.T) {
	tests := []struct {
		amount, allocated string
		want              string
	}{
		{"100", "0", PaymentStatusUnallocated},
		{"100", "40", PaymentStatusPartiallyAllocated},
		{"100", "100", PaymentStatusAllocated},
	}
	for _, tt := range tests {
		got := PaymentStatusAfterReversal(
			decimal.RequireFromString(tt.amount),
			decimal.RequireFromString(tt.allocated),
		)
		if got != tt.want {
			t.Fatalf("status = %q, want %q", got, tt.want)
		}
	}
}

func TestInvoiceStatusAfterReversal(t *testing.T) {
	if got := InvoiceStatusAfterSettlement(
		decimal.RequireFromString("100"),
		decimal.RequireFromString("40"),
		decimal.RequireFromString("60"),
	); got != InvoiceStatusPaid {
		t.Fatalf("status = %q, want paid", got)
	}
	if got := InvoiceStatusAfterSettlement(
		decimal.RequireFromString("100"),
		decimal.RequireFromString("20"),
		decimal.RequireFromString("60"),
	); got != InvoiceStatusPartiallyPaid {
		t.Fatalf("status = %q, want partially_paid", got)
	}
	if got := InvoiceStatusAfterSettlement(
		decimal.RequireFromString("100"),
		decimal.RequireFromString("0"),
		decimal.RequireFromString("0"),
	); got != InvoiceStatusOpen {
		t.Fatalf("status = %q, want open", got)
	}
}
```

Run `go test ./internal/modules/finance/domain -run 'TestPaymentStatusAfterReversal|TestInvoiceStatusAfterSettlement'`; expect undefined.

- [ ] **Step 2: 实现模型和状态**

`payment_reversal.go`:

```go
package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type PaymentAllocation struct {
	ID                string
	PaymentID         string
	InvoiceID         string
	InvoiceCode       string
	InvoiceDate       time.Time
	Amount            decimal.Decimal
	BaseAmount        decimal.Decimal
	InvoiceBaseAmount decimal.Decimal
	CreatedAt         time.Time
}

type PaymentAllocationReversal struct {
	ID                string
	TenantID          string
	OrgID             string
	PaymentID         string
	PaymentCode       string
	PaymentDate       time.Time
	InvoiceID         string
	InvoiceCode       string
	InvoiceDate       time.Time
	AllocationID      string
	Amount            decimal.Decimal
	BaseAmount        decimal.Decimal
	InvoiceBaseAmount decimal.Decimal
	ReversalDate      time.Time
	Reason                 string
	FXReversalVoucherCodes []string
	CreatedBy              string
	CreatedAt         time.Time
}

type UnallocateInput struct {
	InvoiceIDs   []string
	ReversalDate time.Time
	Reason       string
}

func PaymentStatusAfterReversal(amount, allocated decimal.Decimal) string {
	if !allocated.IsPositive() {
		return PaymentStatusUnallocated
	}
	if allocated.LessThan(amount) {
		return PaymentStatusPartiallyAllocated
	}
	return PaymentStatusAllocated
}

func InvoiceStatusAfterSettlement(total, paid, credited decimal.Decimal) string {
	invoice := Invoice{
		TotalAmount:    total,
		PaidAmount:     paid,
		CreditedAmount: credited,
	}
	return invoice.StatusAfterSettlement()
}
```

`errors.go` add:

```go
ErrPaymentNotAllocated = errors.New("payment is not allocated")
ErrAllocationNotFound  = errors.New("payment allocation not found")
ErrReasonRequired      = errors.New("reversal reason is required")
ErrDuplicateInvoice    = errors.New("duplicate invoice in reversal request")
```

`repository.go` 接口方法在 Task 4 与仓储实现一起添加，避免 Task 2 出现接口未实现的中间态编译失败。Task 4 将添加 `Unallocate`、`ListAllocations`、`ListReversals` 和 `ListPaymentAllocationReversals`。

- [ ] **Step 3: Verify/commit**

```bash
gofmt -w internal/modules/finance/domain/payment.go internal/modules/finance/domain/errors.go internal/modules/finance/domain/payment_reversal.go internal/modules/finance/domain/payment_reversal_test.go
go test ./internal/modules/finance/domain -count=1
go build ./...
git add internal/modules/finance/domain
git commit -m "feat(stage-3k): add payment reversal domain"
```

---

## Task 3: 汇兑结算反核销凭证

**Files:**
- Modify: `internal/modules/gl/domain/voucher.go`
- Modify: `internal/modules/gl/infra/postgres/posting.go`
- Create: `internal/modules/gl/infra/postgres/fx_settlement_reversal_test.go`

- [ ] **Step 1: 常量**

```go
VoucherTypeFXSettlementReversal = "fx_settlement_reversal"
SourceTypeFXSettlementReversal  = "fx_settlement_reversal"
```

- [ ] **Step 2: 反核销过账**

Extract/reuse FX line construction. Add:

```go
type FXSettlementReversalPosting struct {
	TenantID     string
	OrgID        string
	VoucherCode  string
	SourceID     string
	SourceCode   string
	InvoiceType  string
	PartnerID    string
	VoucherDate  time.Time
	BaseAdjustment decimal.Decimal
	CreatedBy    string
}
```

Implement `PostFXSettlementReversal` by posting the opposite lines of `PostFXSettlement`:
- Call the existing line builder with `BaseAdjustment = -input.BaseAdjustment`.
- Use voucher type/source `fx_settlement_reversal`.
- Source ID = settlement ID, source code = payment code, memo `汇兑结算反核销 <code>`.
- If adjustment zero return nil.

Add `CheckPeriodOpen(ctx, tx, tenantID, date)` helper in `posting.go` that calls `ensurePeriodTx` and returns `ErrPeriodClosed` when closed; used for reversals with no FX settlement.

- [ ] **Step 3: Tests**

`fx_settlement_reversal_test.go`:
- receivable positive adjustment reversal -> AR credit / FX loss debit opposite of original.
- payable negative adjustment reversal -> FX loss debit / AP credit opposite of original.
- zero adjustment -> no lines.
- balanced all cases.
- `CheckPeriodOpen` maybe tested via API; unit not possible without DB.

- [ ] **Step 4: Verify/commit**

```bash
gofmt -w internal/modules/gl/domain/voucher.go internal/modules/gl/infra/postgres/posting.go internal/modules/gl/infra/postgres/fx_settlement_reversal_test.go
go test ./internal/modules/gl/... -count=1
go build ./...
go vet ./internal/modules/gl/...
git add internal/modules/gl/domain/voucher.go internal/modules/gl/infra/postgres/posting.go internal/modules/gl/infra/postgres/fx_settlement_reversal_test.go
git commit -m "feat(stage-3k): add fx settlement reversal posting"
```

---

## Task 4: 反核销事务和详情

**Files:**
- Modify: `internal/modules/finance/domain/repository.go`
- Modify: `internal/modules/finance/infra/postgres/payment_repository.go`
- Modify: `internal/modules/finance/app/payment_service.go`
- Modify: `internal/modules/finance/app/invoice_service.go`
- Modify: `internal/modules/finance/infra/postgres/invoice_repository.go`

- [ ] **Step 1: Repository Unallocate**

Add `Unallocate`, `ListAllocations` and `ListReversals` to `domain.PaymentRepository`, and `ListPaymentAllocationReversals` to `domain.InvoiceRepository`, in `internal/modules/finance/domain/repository.go`, then implement `Unallocate` in `PaymentRepository`:

Signature:

```go
func (r *PaymentRepository) Unallocate(
	ctx context.Context,
	filter scope.Filter,
	tenantID, paymentID, userID string,
	input domain.UnallocateInput,
) (*domain.Payment, error)
```

Transaction:
1. Validate non-empty unique invoice IDs; sort.
2. Lock payment `FOR UPDATE` with scope. Validate status and type.
3. Lock invoices `FOR UPDATE` in sorted order; validate same partner/currency/type.
4. Lock allocation rows by payment+invoice; if missing -> ErrAllocationNotFound.
5. Sum amounts/base/invoice base.
6. Check payment allocated >= sum, invoice paid >= sum.
7. Insert reversal rows (one per allocation), delete allocation rows.
8. Update payment:
```sql
UPDATE erp.finance_payments
SET allocated_amount = allocated_amount - $3,
    base_allocated_amount = base_allocated_amount - $4,
    status = $5
WHERE tenant_id=$1 AND id=$2 AND allocated_amount >= $3
```
9. Update each invoice:
```sql
UPDATE erp.finance_invoices
SET paid_amount = paid_amount - $3,
    base_paid_amount = base_paid_amount - $4,
    status = $5,
    base_revalued_amount = CASE WHEN $5 = 'paid' THEN 0 ELSE base_revalued_amount END,
    updated_by = $6, updated_at = now()
WHERE tenant_id=$1 AND id=$2 AND paid_amount >= $3
```
10. Query `finance_fx_settlements` for payment+invoice with status posted `FOR UPDATE`. For each:
   - generate voucher code via `r.codes.Next(ctx, tenantID, "gl_voucher")`.
   - `glpost.PostFXSettlementReversal` with the **original** `settlement.base_adjustment`; the posting helper negates it internally, so callers must not double-negate.
   - update settlement status/reversal_id/reversal_voucher_id/reversed_at/reversed_by.
11. Call `glpost.CheckPeriodOpen` once for reversal date (even if no settlements).
12. Commit, return `GetWithDetails` / payment detail.

- [ ] **Step 2: Detail loading**

Implement the `ListAllocations` and `ListReversals` methods declared on `domain.PaymentRepository`:

```go
func (r *PaymentRepository) ListAllocations(ctx, filter, tenantID, paymentID string) ([]*domain.PaymentAllocation, error)
func (r *PaymentRepository) ListReversals(ctx, filter, tenantID, paymentID string) ([]*domain.PaymentAllocationReversal, error)
```
- allocations join invoices; order payment/invoice.
- reversals join invoices; left join fx settlements on reversal_id; aggregate reversal voucher codes.

`PaymentService.Get` returns detail:
```go
type PaymentDetail struct { Payment *domain.Payment; Allocations []*domain.PaymentAllocation; Reversals []*domain.PaymentAllocationReversal }
```
(Or extend payment struct; choose app detail.)

- [ ] **Step 3: Invoice detail reversals**

Implement `InvoiceRepository.ListPaymentAllocationReversals` (declared on `domain.InvoiceRepository`) to join payments, order by date.
`InvoiceDetail` add `PaymentAllocationReversals`.
`InvoiceService.Get` populate.

- [ ] **Step 4: Service Unallocate**

`PaymentService.Unallocate(ctx, id, input)`:
- canonicalize payment UUID.
- validate invoice IDs UUIDs, non-empty, unique, reason non-empty, date default today, not before payment date.
- call repository.
- return payment detail.

- [ ] **Step 5: Verify/commit**

```bash
gofmt -w internal/modules/finance/domain/repository.go internal/modules/finance/infra/postgres/payment_repository.go internal/modules/finance/app/payment_service.go internal/modules/finance/app/invoice_service.go internal/modules/finance/infra/postgres/invoice_repository.go
go build ./...
go test ./internal/modules/finance/... ./internal/modules/gl/... -count=1
go vet ./internal/modules/finance/... ./internal/modules/gl/...
git add internal/modules/finance/domain/repository.go internal/modules/finance/infra/postgres/payment_repository.go internal/modules/finance/app/payment_service.go internal/modules/finance/app/invoice_service.go internal/modules/finance/infra/postgres/invoice_repository.go
git commit -m "feat(stage-3k): unallocate payments transactionally"
```

---

## Task 5: API、SDK、OpenAPI

**Files:**
- Modify: `internal/modules/finance/http/handler.go`
- Modify: `packages/sdk/src/finance.ts`
- Modify: `api/openapi.yaml`
- Modify: `scripts/test/check-frontend.mjs`

- [ ] **Step 1: API**

Route:
```go
finance.POST("/payments/:id/unallocate", authmw.RequirePermission("finance.payment.reverse"), handler.unallocatePayment)
```

Request:
```go
type unallocatePaymentRequest struct {
	InvoiceIDs []string `json:"invoice_ids"`
	ReversalDate string `json:"reversal_date"`
	Reason string `json:"reason"`
}
```
- parse date, validate non-empty IDs/reason, call service.
- return payment detail response (allocations/reversals).
- error mappings: `PAYMENT_NOT_ALLOCATED`, `ALLOCATION_NOT_FOUND`, `REASON_REQUIRED`, `DUPLICATE_INVOICE`, `PERIOD_CLOSED`, `NOT_FOUND`, `BAD_REQUEST`.

Payment response:
- add `allocations`, `reversals` to detail response only (list summary unchanged).
- allocation fields; reversal fields incl. `fx_reversal_voucher_codes`.

Invoice detail:
- add `payment_allocation_reversals`.

- [ ] **Step 2: SDK**

```ts
export interface PaymentAllocation { id, payment_id, invoice_id, invoice_code, invoice_date, amount, base_amount, invoice_base_amount, created_at }
export interface PaymentAllocationReversal { id, payment_id, payment_code, payment_date, invoice_id, invoice_code, invoice_date, amount, base_amount, invoice_base_amount, reversal_date, reason, fx_reversal_voucher_codes: string[], created_at }
export interface UnallocatePaymentInput { invoice_ids: string[]; reversal_date: string; reason: string }
export interface PaymentDetail { payment: Payment; allocations: PaymentAllocation[]; reversals: PaymentAllocationReversal[] }
```
`financeApi.payments.unallocate(id,input): Promise<PaymentDetail>`, `get` returns `PaymentDetail`? Existing `get` returns `Payment`; changing type may affect UI. Could add `getDetail` or change get to detail. Prefer `get(id): Promise<PaymentDetail>` and update frontend; or add `detail(id)`. Plan: add `getDetail(id)` and keep `get`; API `GET /payments/:id` returns detail, so `get` can return `PaymentDetail`; adjust UI. Simpler `get` returns `PaymentDetail` and list `Payment`. Existing code may expect Payment; update.

- [ ] **Step 3: OpenAPI 1.2.0**

Paths/schemas; payment detail arrays; invoice detail reversals.

- [ ] **Step 4: check-frontend**

Endpoint/SDK literals:
`'/finance/payments/{id}/unallocate:'`, `'unallocate'`, `'PaymentAllocationReversal'`, `'UnallocatePaymentInput'`, `'payment_allocation_reversals'`.

- [ ] **Step 5: Verify/commit**

```bash
gofmt -w internal/modules/finance/http/handler.go
go build ./...
go test ./internal/modules/finance/... -count=1
go vet ./internal/modules/finance/...
npx -y yaml@2 valid api/openapi.yaml
pnpm typecheck:web
pnpm test:frontend
git add internal/modules/finance/http/handler.go packages/sdk/src/finance.ts api/openapi.yaml scripts/test/check-frontend.mjs
git commit -m "feat(stage-3k): add payment reversal APIs"
```

---

## Task 6: 前端反核销

**Files:**
- Modify: `apps/web/src/views/finance/PaymentListView.vue`
- Modify: `apps/web/src/views/finance/InvoiceListView.vue`
- Modify: `scripts/test/check-frontend.mjs`

- [ ] **Step 1: 收付款页**

- 列表已核销/部分核销行显示“反核销”按钮，权限 `finance.payment.reverse`。
- 反核销弹窗：
  - 加载 `financeApi.payments.get(id)` 的 allocations。
  - 表格：发票号、日期、金额、付款本位币、发票本位币；勾选核销行。
  - 反核销日期默认今天；原因必填。
  - 提交 `financeApi.payments.unallocate(id, { invoice_ids, reversal_date, reason })`。
  - 成功后刷新列表/详情。
- 详情 drawer：核销历史 + 反核销历史（日期、发票、金额、原因、汇兑反核销凭证）。
- 状态/金额更新。

- [ ] **Step 2: 发票详情**

- 增加 `payment_allocation_reversals` 表：收付款号、日期、金额、反核销日期、原因、汇兑凭证。

- [ ] **Step 3: 完整度断言**

Add assertions:
- PaymentListView: `payments.unallocate`, `finance.payment.reverse`, `reversals`, `PaymentAllocationReversal`, `反核销`.
- InvoiceListView: `payment_allocation_reversals`.

- [ ] **Step 4: Verify/commit**

```bash
pnpm typecheck:web
pnpm test:frontend
pnpm build:web
git add apps/web/src/views/finance/PaymentListView.vue apps/web/src/views/finance/InvoiceListView.vue scripts/test/check-frontend.mjs
git commit -m "feat(stage-3k): add payment reversal UI"
```

---

## Task 7: API 测试、脚本、README、CI

**Files:**
- Create: `tests/api/stage3k_test.go`
- Create: `scripts/test/stage3k-api.ps1`
- Modify: `package.json`
- Modify: `README.md`

- [ ] **Test cases**

1. `reverse receivable allocation restores invoice and payment`
   - Create shipment/invoice, receive payment, allocate; reverse one line.
   - Assert payment allocated reduced/status, invoice paid/outstanding/status, allocation removed, reversal record.
2. `reverse payable allocation`
   - Purchase receipt/payable, payment, allocate, reverse.
3. `multi-invoice reversal and reallocation`
   - Payment allocated to two invoices; reverse both; reverse one; reallocate after reversal.
4. `fx settlement reversal`
   - USD payment rate vs invoice rate; reverse; original settlement status reversed; reversal voucher lines opposite and balanced; payment/invoice base restored.
   - Assert reversal voucher `voucher_type` and `source_type` are `fx_settlement_reversal`, `source_id` equals the original settlement ID, `source_code` equals the payment code, and memo contains `汇兑结算反核销`.
5. `credited invoice status after reversal`
   - Invoice with credit note; reverse payment; status/outstanding recalculated.
6. `validation and permissions`
   - unknown payment/invoice, non-payment allocation, duplicate IDs, empty selection, missing reason, closed period, already reversed, 401/403.
7. `concurrent double reversal`
   - Two concurrent unallocate requests; one success, one 409.
8. `data scope`
   - demo_dept cannot see/reverse admin payment.
9. `payment/invoice detail history`
   - payment detail allocations/reversals, invoice detail reversals, list summary no arrays.
10. `reversal date before payment/closed period`
    - 400/409 rollback.
11. `rounding parity on multi-invoice reversal`
    - USD payment rate `7.12345678`; allocate `1.0000` to two invoices in one call; reverse both (and separately one then the other).
    - Assert payment `allocated_amount`/`base_allocated_amount` are zero, both invoices paid/base paid zero, statuses `unallocated`/`open`.

- [ ] Script/package/README

Copy `stage3j-api.ps1`:
- `api-stage3k.exe`, `TestStage3K`.
- package `test:api:stage3k`.
- README status/commands/section; clean DB warning.

- [ ] Verify/commit

```bash
gofmt -w tests/api/stage3k_test.go
go vet -tags=integration ./tests/api
go test -tags=integration ./tests/api -run '^$'
pnpm test:api:stage3k
git add tests/api/stage3k_test.go scripts/test/stage3k-api.ps1 package.json README.md
git commit -m "test(stage-3k): add payment reversal API tests"
```

---

## Task 8: 文档、静态检查、报告草稿

**Files:**
- Modify: `internal/modules/finance/README.md`
- Modify: `internal/modules/gl/README.md`
- Create: `docs/reports/2026-11-25-stage-3k-payment-reversals.md`

- finance README: reversal rules, API, permissions, status, FX, limitations.
- gl README: `fx_settlement_reversal` lines, no cash voucher reversal.
- Report sections: scope, data model, reversal transaction, FX/GL, API/UI, tests, review pending, known limitations, acceptance.
- Run static checks.

Commit `docs(stage-3k): add payment reversal docs and report draft`.

---

## Task 9: 独立审核、干净库 Stage 3B–3K、推送

- Independent review of `60c3542..HEAD`.
- Fix Critical/Important, rerun Stage 3K and static checks.
- Clean DB `mlf_erp_stage3k_clean`, migration 24, Stage 3B–3K.
- Validate: payment/invoice balances, reversal records, fx settlements status/reversal, vouchers balanced, no cross-ledger inconsistencies.
- Fill report, push `stage-3k-payment-reversals`.

---

## 验收清单

- [ ] 收付款核销行可整行反核销。
- [ ] 收付款/发票金额和状态正确回退。
- [ ] 多发票和重新核销可用。
- [ ] 已实现汇兑结算生成反方向凭证并标记 reversed。
- [ ] 贷项冲减后的发票状态重算正确。
- [ ] 权限、错误码、付款/发票详情历史完整。
- [ ] 前端反核销弹窗和历史可用。
- [ ] Stage 3B–3K 干净库全链路通过。
- [ ] 独立审核无 Critical/Important 遗留。
