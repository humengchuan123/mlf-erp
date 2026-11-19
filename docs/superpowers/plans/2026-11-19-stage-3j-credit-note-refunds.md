# Stage 3J 贷项单抵用与退款 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让销售/采购退货贷项可以抵其他应收/应付发票，并支持客户/供应商退款、汇兑损益和完整审计。

**Architecture:** 在 `finance_credit_notes` 上增加已退金额；新增抵用明细表和退款表；抵用为子账事务，不生成凭证；退款生成 `credit_note_refund` 凭证。扩展 finance 领域/仓储/服务、GL 过账、API、SDK、OpenAPI 和前端。

**Tech Stack:** Go 1.25 + Gin + pgx + PostgreSQL + shopspring/decimal；Vue 3 + TypeScript + Element Plus。

**Design:** `docs/superpowers/specs/2026-11-19-stage-3j-credit-note-refunds-design.md`

---

## Task 1: 迁移、回填、权限和编码规则

**Files:**
- Create: `db/migrations/000022_credit_note_usage.up.sql`
- Create: `db/migrations/000022_credit_note_usage.down.sql`
- Modify: `db/migrations/README.md`
- Modify: `cmd/seed/main.go`

- [ ] **Step 1: 写 up 迁移**

创建 `db/migrations/000022_credit_note_usage.up.sql`：

```sql
ALTER TABLE erp.finance_credit_notes
    ADD COLUMN IF NOT EXISTS refunded_amount numeric(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS refunded_base_amount numeric(19,4) NOT NULL DEFAULT 0;

ALTER TABLE erp.finance_credit_notes
    DROP CONSTRAINT IF EXISTS finance_credit_notes_amount_check;
ALTER TABLE erp.finance_credit_notes
    ADD CONSTRAINT finance_credit_notes_amount_check CHECK (
        amount > 0 AND base_amount > 0
        AND applied_amount >= 0 AND applied_base_amount >= 0
        AND refunded_amount >= 0 AND refunded_base_amount >= 0
        AND remaining_amount >= 0 AND remaining_base_amount >= 0
        AND applied_amount + refunded_amount + remaining_amount = amount
        AND applied_base_amount + refunded_base_amount + remaining_base_amount = base_amount
    );

CREATE TABLE IF NOT EXISTS erp.finance_credit_note_allocations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    credit_note_id uuid NOT NULL,
    invoice_id uuid NOT NULL,
    amount numeric(19,4) NOT NULL CHECK (amount > 0),
    base_amount numeric(19,4) NOT NULL DEFAULT 0 CHECK (base_amount >= 0),
    allocation_date date NOT NULL,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_credit_note_allocations_note_fk
        FOREIGN KEY (tenant_id, credit_note_id)
        REFERENCES erp.finance_credit_notes (tenant_id, id),
    CONSTRAINT finance_credit_note_allocations_invoice_fk
        FOREIGN KEY (tenant_id, invoice_id)
        REFERENCES erp.finance_invoices (tenant_id, id)
);

CREATE INDEX IF NOT EXISTS finance_credit_note_allocations_note_idx
    ON erp.finance_credit_note_allocations (tenant_id, credit_note_id, allocation_date);
CREATE INDEX IF NOT EXISTS finance_credit_note_allocations_invoice_idx
    ON erp.finance_credit_note_allocations (tenant_id, invoice_id);

CREATE TABLE IF NOT EXISTS erp.finance_credit_note_refunds (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    code text NOT NULL,
    credit_note_id uuid NOT NULL,
    refund_date date NOT NULL,
    status text NOT NULL DEFAULT 'posted',
    currency text NOT NULL,
    exchange_rate numeric(19,8) NOT NULL CHECK (exchange_rate > 0),
    amount numeric(19,4) NOT NULL CHECK (amount > 0),
    base_amount numeric(19,4) NOT NULL CHECK (base_amount >= 0),
    cash_base_amount numeric(19,4) NOT NULL CHECK (cash_base_amount >= 0),
    fx_adjustment numeric(19,4) NOT NULL DEFAULT 0,
    method text NOT NULL DEFAULT 'bank',
    reference text NOT NULL DEFAULT '',
    remark text NOT NULL DEFAULT '',
    voucher_code text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_credit_note_refunds_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT finance_credit_note_refunds_note_fk
        FOREIGN KEY (tenant_id, credit_note_id)
        REFERENCES erp.finance_credit_notes (tenant_id, id),
    CONSTRAINT finance_credit_note_refunds_status_check
        CHECK (status IN ('posted')),
    CONSTRAINT finance_credit_note_refunds_fx_check
        CHECK (cash_base_amount - base_amount = fx_adjustment)
);

CREATE INDEX IF NOT EXISTS finance_credit_note_refunds_note_idx
    ON erp.finance_credit_note_refunds (tenant_id, credit_note_id, refund_date DESC);
CREATE INDEX IF NOT EXISTS finance_credit_note_refunds_list_idx
    ON erp.finance_credit_note_refunds (tenant_id, org_id, refund_date DESC);

INSERT INTO erp.finance_credit_note_allocations (
    id, tenant_id, org_id, credit_note_id, invoice_id,
    amount, base_amount, allocation_date, created_by
)
SELECT gen_random_uuid(), c.tenant_id, c.org_id, c.id, c.invoice_id,
       c.applied_amount, c.applied_base_amount, c.note_date, c.created_by
FROM erp.finance_credit_notes c
WHERE c.applied_amount > 0
  AND c.invoice_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM erp.finance_credit_note_allocations a
      WHERE a.tenant_id = c.tenant_id
        AND a.credit_note_id = c.id
        AND a.invoice_id = c.invoice_id
        AND a.allocation_date = c.note_date
        AND a.amount = c.applied_amount
  );

INSERT INTO erp.code_rules (
    id, tenant_id, entity, prefix, date_pattern, separator, sequence_length, reset_policy, status
)
SELECT gen_random_uuid(), t.id, 'credit_note_refund', 'RF', 'yyyyMMdd', '-', 4, 'day', 'active'
FROM erp.tenants t
ON CONFLICT (tenant_id, entity) DO NOTHING;

INSERT INTO erp.permissions (code, name, module, type)
VALUES
    ('finance.credit_note.apply', '贷项单抵用', 'finance', 'api'),
    ('finance.credit_note.refund', '贷项单退款', 'finance', 'api')
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name, module = EXCLUDED.module, type = EXCLUDED.type;

INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, p.code
FROM erp.roles r
CROSS JOIN (VALUES ('finance.credit_note.apply'), ('finance.credit_note.refund')) AS p(code)
WHERE r.code = 'admin'
ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: 写 down 迁移**

创建 `db/migrations/000022_credit_note_usage.down.sql`：

```sql
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM erp.finance_credit_notes
        WHERE refunded_amount > 0 OR refunded_base_amount > 0
    ) THEN
        RAISE EXCEPTION 'cannot roll back 000022_credit_note_usage while credit note refunds exist; reverse refunds first';
    END IF;
END $$;

DELETE FROM erp.role_permissions WHERE permission_code IN ('finance.credit_note.apply', 'finance.credit_note.refund');
DELETE FROM erp.permissions WHERE code IN ('finance.credit_note.apply', 'finance.credit_note.refund');
DELETE FROM erp.code_rules WHERE entity = 'credit_note_refund';

DROP TABLE IF EXISTS erp.finance_credit_note_refunds;
DROP TABLE IF EXISTS erp.finance_credit_note_allocations;

ALTER TABLE erp.finance_credit_notes
    DROP CONSTRAINT IF EXISTS finance_credit_notes_amount_check;
ALTER TABLE erp.finance_credit_notes
    ADD CONSTRAINT finance_credit_notes_amount_check CHECK (
        amount > 0 AND base_amount > 0
        AND applied_amount >= 0 AND applied_base_amount >= 0
        AND remaining_amount >= 0 AND remaining_base_amount >= 0
        AND applied_amount + remaining_amount = amount
        AND applied_base_amount + remaining_base_amount = base_amount
    );

ALTER TABLE erp.finance_credit_notes
    DROP COLUMN IF EXISTS refunded_base_amount,
    DROP COLUMN IF EXISTS refunded_amount;
```

- [ ] **Step 3: 更新迁移 README 和 seed**

`db/migrations/README.md` 增加：

```markdown
- `000022_credit_note_usage`：贷项已退金额、抵用明细、贷项退款单、退款凭证和贷项抵用/退款权限/编码规则。Down 在有退款数据时拒绝执行。
```

`cmd/seed/main.go` 权限列表增加：

```go
{"finance.credit_note.apply", "贷项单抵用", "finance", "api"},
{"finance.credit_note.refund", "贷项单退款", "finance", "api"},
```

编码规则列表增加：

```go
{"credit_note_refund", "RF", "yyyyMMdd", "-", 4, "day"},
```

- [ ] **Step 4: 应用并验证**

```bash
gofmt -w cmd/seed/main.go
go run ./cmd/migrate -direction up
go run ./cmd/seed
docker exec mlf-erp-dev-postgres-1 psql -U mlf -d mlf_erp -c "select version, dirty from schema_migrations; select column_name from information_schema.columns where table_name='finance_credit_notes' and column_name like 'refunded%'; select table_name from information_schema.tables where table_schema='erp' and table_name in ('finance_credit_note_allocations','finance_credit_note_refunds');"
```

Expected: version 22, dirty=f; both columns and both tables exist.

- [ ] **Step 5: Commit**

```bash
git add db/migrations/000022_credit_note_usage.up.sql db/migrations/000022_credit_note_usage.down.sql db/migrations/README.md cmd/seed/main.go
git commit -m "feat(stage-3j): add credit note usage schema"
```

---

## Task 2: 领域模型、错误和金额计算

**Files:**
- Modify: `internal/modules/finance/domain/credit_note.go`
- Modify: `internal/modules/finance/domain/errors.go`
- Modify: `internal/modules/finance/domain/repository.go`
- Create: `internal/modules/finance/domain/credit_note_usage.go`
- Create: `internal/modules/finance/domain/credit_note_usage_test.go`

- [ ] **Step 1: 先写失败测试**

`credit_note_usage_test.go`：

```go
package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestAllocateBase(t *testing.T) {
	tests := []struct {
		remainingAmount, remainingBase, amount, want string
	}{
		{"100.0000", "720.0000", "50.0000", "360.0000"},
		{"3.0000", "100.0000", "1.0000", "33.3333"},
		{"3.0000", "100.0000", "2.0000", "66.6667"},
		{"3.0000", "100.0000", "3.0000", "100.0000"},
	}
	for _, tt := range tests {
		got := AllocateBase(decimal.RequireFromString(tt.remainingAmount),
			decimal.RequireFromString(tt.remainingBase),
			decimal.RequireFromString(tt.amount))
		if want := decimal.RequireFromString(tt.want); !got.Equal(want) {
			t.Fatalf("base = %s, want %s", got, want)
		}
	}
}

func TestCashBaseAndFXAdjustment(t *testing.T) {
	cash := CashBase(decimal.RequireFromString("100"), decimal.RequireFromString("7.3"))
	if !cash.Equal(decimal.RequireFromString("730.0000")) {
		t.Fatalf("cash base = %s, want 730.0000", cash)
	}
	if got := FXAdjustment(cash, decimal.RequireFromString("720.0000")); !got.Equal(decimal.RequireFromString("10.0000")) {
		t.Fatalf("fx adjustment = %s, want 10.0000", got)
	}
}

func TestCreditNoteStatus(t *testing.T) {
	if got := CreditNoteStatus(decimal.Zero, decimal.Zero); got != CreditNoteStatusApplied {
		t.Fatalf("status = %q, want applied", got)
	}
	if got := CreditNoteStatus(decimal.Zero, decimal.RequireFromString("0.0001")); got != CreditNoteStatusApplied {
		t.Fatalf("residual status = %q, want applied", got)
	}
	if got := CreditNoteStatus(decimal.RequireFromString("1"), decimal.Zero); got != CreditNoteStatusOpen {
		t.Fatalf("status = %q, want open", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/modules/finance/domain -run 'TestAllocateBase|TestCashBaseAndFXAdjustment|TestCreditNoteStatus'
```

Expected: undefined symbols.

- [ ] **Step 3: 实现领域计算**

`credit_note_usage.go`：

```go
package domain

import "github.com/shopspring/decimal"

const baseResidualTolerance = "0.0001"

func AllocateBase(remainingAmount, remainingBase, amount decimal.Decimal) decimal.Decimal {
	if !remainingAmount.IsPositive() || !amount.IsPositive() {
		return decimal.Zero
	}
	if amount.GreaterThanOrEqual(remainingAmount) {
		return remainingBase.Round(4)
	}
	return remainingBase.Mul(amount).Div(remainingAmount).Round(4)
}

func CashBase(amount, rate decimal.Decimal) decimal.Decimal {
	return amount.Mul(rate).Round(4)
}

func FXAdjustment(cashBase, creditBase decimal.Decimal) decimal.Decimal {
	return cashBase.Sub(creditBase).Round(4)
}

func CreditNoteStatus(remainingAmount, remainingBase decimal.Decimal) string {
	if remainingAmount.IsZero() &&
		remainingBase.Abs().LessThanOrEqual(decimal.RequireFromString(baseResidualTolerance)) {
		return CreditNoteStatusApplied
	}
	return CreditNoteStatusOpen
}
```

- [ ] **Step 4: 扩展模型和错误**

`credit_note.go` 增加字段：

```go
RefundedAmount      decimal.Decimal
RefundedBaseAmount  decimal.Decimal
Allocations         []*CreditNoteAllocation
Refunds             []*CreditNoteRefund
```

增加结构体：

```go
type CreditNoteAllocation struct {
	ID             string
	TenantID       string
	OrgID          string
	CreditNoteID   string
	InvoiceID      string
	InvoiceCode    string
	InvoiceDate    time.Time
	Amount         decimal.Decimal
	BaseAmount     decimal.Decimal
	AllocationDate time.Time
	CreatedBy      string
	CreatedAt      time.Time
}

type CreditNoteRefund struct {
	ID             string
	TenantID       string
	OrgID          string
	Code           string
	CreditNoteID   string
	RefundDate     time.Time
	Status         string
	Currency       string
	ExchangeRate   decimal.Decimal
	Amount         decimal.Decimal
	BaseAmount     decimal.Decimal
	CashBaseAmount decimal.Decimal
	FXAdjustment   decimal.Decimal
	Method         string
	Reference      string
	Remark         string
	VoucherCode    string
	CreatedBy      string
	CreatedAt      time.Time
}

type AllocationInput struct {
	InvoiceID string
	Amount    decimal.Decimal
}

type RefundInput struct {
	RefundDate   time.Time
	Amount       decimal.Decimal
	ExchangeRate decimal.Decimal
	Method       string
	Reference    string
	Remark       string
}
```

`Remaining()` 改为：

```go
func (c *CreditNote) Remaining() decimal.Decimal {
	return c.Amount.Sub(c.AppliedAmount).Sub(c.RefundedAmount).Round(4)
}
```

`errors.go` 增加：

```go
ErrCreditNoteClosed        = errors.New("credit note is closed")
ErrOverApply               = errors.New("credit note allocation exceeds remaining amount")
ErrOverRefund              = errors.New("credit note refund exceeds remaining amount")
ErrInsufficientInvoiceBase = errors.New("insufficient invoice base amount")
ErrInvoiceTypeMismatch     = errors.New("invoice type does not match credit note")
ErrOrgMismatch             = errors.New("organization does not match")
ErrRefundAmountTooSmall    = errors.New("refund cash base amount is zero")
```

复用现有 `ErrPartnerMismatch`、`ErrCurrencyMismatch`、`ErrInvoicePaid`。

`repository.go` 接口改为：

```go
type CreditNoteRepository interface {
	List(...) ([]*CreditNote, int, error)
	Get(...) (*CreditNote, error)
	Allocate(ctx context.Context, filter scope.Filter, tenantID, creditNoteID, userID string,
		allocationDate time.Time, allocations []*AllocationInput) (*CreditNote, error)
	AllocationCandidates(ctx context.Context, filter scope.Filter, tenantID, creditNoteID string) ([]*Invoice, error)
	Refund(ctx context.Context, filter scope.Filter, tenantID, creditNoteID, userID string,
		input RefundInput, code, voucherCode string, exchangeRate decimal.Decimal) (*CreditNote, error)
}
```

- [ ] **Step 5: 运行测试并提交**

```bash
gofmt -w internal/modules/finance/domain/credit_note.go internal/modules/finance/domain/errors.go internal/modules/finance/domain/repository.go internal/modules/finance/domain/credit_note_usage.go internal/modules/finance/domain/credit_note_usage_test.go
go test ./internal/modules/finance/domain -count=1
git add internal/modules/finance/domain
git commit -m "feat(stage-3j): add credit note usage domain"
```

---

## Task 3: 退货创建时写入初始抵用记录

**Files:**
- Modify: `internal/modules/sales/infra/postgres/return_repository.go`
- Modify: `internal/modules/purchase/infra/postgres/return_repository.go`

- [ ] **Step 1: 销售退货写入初始抵用**

在销售退货仓储创建贷项单并更新发票后，若 `applied.IsPositive()`，插入：

```go
if _, err := tx.Exec(ctx, `
	INSERT INTO erp.finance_credit_note_allocations (
		id, tenant_id, org_id, credit_note_id, invoice_id,
		amount, base_amount, allocation_date, created_by
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
	uuid.NewString(), tenantID, ret.OrgID, creditNoteID, invoiceID,
	applied.String(), appliedBase.String(), ret.ReturnDate, userID,
); err != nil {
	return nil, fmt.Errorf("insert initial credit note allocation: %w", err)
}
```

- [ ] **Step 2: 采购退货写入初始抵用**

在采购退货仓储同样位置插入相同 SQL，使用采购退货的 `creditNoteID`、`invoiceID`、`applied`、`appliedBase`、`ret.ReturnDate`、`userID`。

- [ ] **Step 3: 验证 Stage 3H/3I 不回归**

```bash
gofmt -w internal/modules/sales/infra/postgres/return_repository.go internal/modules/purchase/infra/postgres/return_repository.go
go test ./internal/modules/sales/... ./internal/modules/purchase/... -count=1
pnpm test:api:stage3h
pnpm test:api:stage3i
```

Expected: 两个测试套件通过；新表插入不破坏现有流程。

- [ ] **Step 4: Commit**

```bash
git add internal/modules/sales/infra/postgres/return_repository.go internal/modules/purchase/infra/postgres/return_repository.go
git commit -m "feat(stage-3j): record initial credit note allocations"
```

---

## Task 4: 贷项抵用仓储、候选发票和应用服务

**Files:**
- Modify: `internal/modules/finance/infra/postgres/credit_note_repository.go`
- Modify: `internal/modules/finance/app/credit_note_service.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: 扩展贷项查询和详情**

`credit_note_repository.go`：

- `creditNoteSelectSQL` 增加 `c.refunded_amount::text, c.refunded_base_amount::text`，扫描到新字段。
- `Get` 加载：
  - `credit_note_allocations` 联表 `finance_invoices` 取发票号/日期。
  - `finance_credit_note_refunds`。
- `List` 返回已退金额字段（不需要明细）。

- [ ] **Step 2: 实现 Allocate**

签名：

```go
func (r *CreditNoteRepository) Allocate(
	ctx context.Context,
	filter scope.Filter,
	tenantID, creditNoteID, userID string,
	allocationDate time.Time,
	allocations []*domain.AllocationInput,
) (*domain.CreditNote, error)
```

事务步骤：

1. `FOR UPDATE` 锁定贷项单，读取 `credit_type/partner_id/org_id/currency/status/amount/applied/refunded/remaining/base_*`。
2. 校验 `status == open`、`remaining_amount > 0`、`remaining_base_amount >= 0`。
3. 对 `allocations` 按 `InvoiceID` 排序；拒绝重复发票。
4. 逐张锁定发票，读取 `invoice_type/partner_id/org_id/currency/status/total/paid/credited/base_total/base_paid/base_credited`。
5. 校验方向、往来单位、币种、组织、状态、未付金额。
6. 按顺序计算 `base := domain.AllocateBase(remainingAmount, remainingBase, amount)`；校验 `base <= invoiceBaseOutstanding`。
7. 插入 `finance_credit_note_allocations`。
8. 更新发票 `credited_amount/base_credited_amount/status/base_revalued_amount`，带 SQL 守卫。
9. 扣减贷项剩余，累加 `applied_amount/applied_base_amount`，状态用 `domain.CreditNoteStatus`。
10. Commit，返回 `Get`。

关键更新 SQL：

```sql
UPDATE erp.finance_invoices
SET credited_amount = credited_amount + $3,
    base_credited_amount = base_credited_amount + $4,
    status = CASE
        WHEN paid_amount + credited_amount + $3 >= total_amount THEN 'paid'
        WHEN paid_amount + credited_amount + $3 > 0 THEN 'partially_paid'
        ELSE 'open'
    END,
    base_revalued_amount = CASE
        WHEN paid_amount + credited_amount + $3 >= total_amount THEN 0
        ELSE base_revalued_amount
    END,
    updated_by = $5, updated_at = now()
WHERE tenant_id = $1 AND id = $2
  AND paid_amount + credited_amount + $3 <= total_amount
  AND base_credited_amount + $4 <= base_total_amount
```

```sql
UPDATE erp.finance_credit_notes
SET applied_amount = applied_amount + $3,
    applied_base_amount = applied_base_amount + $4,
    remaining_amount = remaining_amount - $3,
    remaining_base_amount = remaining_base_amount - $4,
    status = $5,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
  AND remaining_amount >= $3
  AND remaining_base_amount >= $4
```

- [ ] **Step 3: 实现候选发票查询**

```go
func (r *CreditNoteRepository) AllocationCandidates(
	ctx context.Context,
	filter scope.Filter,
	tenantID, creditNoteID string,
) ([]*domain.Invoice, error)
```

- 锁定/读取贷项单（不需要 `FOR UPDATE`），得到 `credit_type/partner_id/org_id/currency`。
- 查询 `finance_invoices`：
  - 同租户、同组织、同往来单位、同币种。
  - `sales_return` → `invoice_type='receivable'`；`purchase_return` → `'payable'`。
  - `status <> 'paid'` 且 `paid_amount + credited_amount < total_amount`。
- 应用 `appendScope`。
- 复用 `scanInvoice` 或 invoice select 字段。

- [ ] **Step 4: 服务层**

`credit_note_service.go` 增加：

```go
type CreditNoteService struct {
	notes domain.CreditNoteRepository
	codes CodeGenerator
	rates fx.RateReader
}

func NewCreditNoteService(notes domain.CreditNoteRepository, codes CodeGenerator, rates fx.RateReader) *CreditNoteService
```

方法：

```go
func (s *CreditNoteService) Allocate(ctx context.Context, id string, allocationDate time.Time, allocations []domain.AllocationInput) (*domain.CreditNote, error)
func (s *CreditNoteService) AllocationCandidates(ctx context.Context, id string) ([]*domain.Invoice, error)
```

- UUID 校验。
- `allocationDate` 为空取当天；不能早于贷项单日期（在仓储校验）。
- 金额 > 0、发票 ID 非空、无重复。
- 调用仓储。

`cmd/api/main.go` 修改：

```go
creditNoteService := financeapp.NewCreditNoteService(creditNoteRepository, codeRuleService, exchangeRateRepository)
```

- [ ] **Step 5: 编译和测试**

```bash
gofmt -w internal/modules/finance/infra/postgres/credit_note_repository.go internal/modules/finance/app/credit_note_service.go cmd/api/main.go
go build ./...
go test ./internal/modules/finance/... -count=1
go vet ./internal/modules/finance/...
```

- [ ] **Step 6: Commit**

```bash
git add internal/modules/finance/infra/postgres/credit_note_repository.go internal/modules/finance/app/credit_note_service.go cmd/api/main.go
git commit -m "feat(stage-3j): allocate credit notes to invoices"
```

---

## Task 5: 贷项退款、凭证和汇兑损益

**Files:**
- Modify: `internal/modules/gl/domain/voucher.go`
- Modify: `internal/modules/gl/infra/postgres/posting.go`
- Create: `internal/modules/gl/infra/postgres/credit_note_refund_test.go`
- Modify: `internal/modules/finance/infra/postgres/credit_note_repository.go`
- Modify: `internal/modules/finance/app/credit_note_service.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: 先写 GL 行失败测试**

`credit_note_refund_test.go`：

```go
package postgres

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestCreditNoteRefundPostingLines(t *testing.T) {
	t.Run("customer refund with fx loss", func(t *testing.T) {
		lines := creditNoteRefundPostingLines(CreditNoteRefundPosting{
			CreditType: "sales_return", PartnerID: "p1",
			BaseAmount: decimal.RequireFromString("720"),
			CashBaseAmount: decimal.RequireFromString("730"),
			FXAdjustment: decimal.RequireFromString("10"),
		})
		assertRefundLines(t, lines, "ar", "bank", "fx_loss", "10")
	})
	t.Run("supplier refund with fx gain", func(t *testing.T) {
		lines := creditNoteRefundPostingLines(CreditNoteRefundPosting{
			CreditType: "purchase_return", PartnerID: "p1",
			BaseAmount: decimal.RequireFromString("720"),
			CashBaseAmount: decimal.RequireFromString("730"),
			FXAdjustment: decimal.RequireFromString("10"),
		})
		assertRefundLines(t, lines, "ap", "bank", "fx_gain", "10")
	})
	t.Run("zero base customer refund", func(t *testing.T) {
		lines := creditNoteRefundPostingLines(CreditNoteRefundPosting{
			CreditType: "sales_return", PartnerID: "p1",
			BaseAmount: decimal.Zero,
			CashBaseAmount: decimal.RequireFromString("100"),
			FXAdjustment: decimal.RequireFromString("100"),
		})
		assertRefundLines(t, lines, "", "bank", "fx_loss", "100")
	})
}
```

`assertRefundLines` 校验借贷平衡、账户和金额。

- [ ] **Step 2: 实现 GL 凭证**

`voucher.go` 增加：

```go
VoucherTypeCreditNoteRefund = "credit_note_refund"
SourceTypeCreditNoteRefund  = "credit_note_refund"
```

`posting.go` 增加：

```go
type CreditNoteRefundPosting struct {
	TenantID       string
	OrgID          string
	VoucherCode    string
	SourceID       string
	SourceCode     string
	CreditType     string
	PartnerID      string
	VoucherDate    time.Time
	BaseAmount     decimal.Decimal
	CashBaseAmount decimal.Decimal
	FXAdjustment   decimal.Decimal
	Method         string
	CreatedBy      string
}
```

`creditNoteRefundPostingLines`：

```go
cashKey := "bank"
if input.Method == "cash" { cashKey = "cash" }
lines := []postingLine{}
fx := input.FXAdjustment.Round(4)
if input.CreditType == "sales_return" {
	if input.BaseAmount.IsPositive() {
		lines = append(lines, postingLine{AccountKey: "ar", Summary: "贷项退款应收", Debit: input.BaseAmount, PartnerID: input.PartnerID})
	}
	lines = append(lines, postingLine{AccountKey: cashKey, Summary: "客户退款", Credit: input.CashBaseAmount})
} else {
	lines = append(lines, postingLine{AccountKey: cashKey, Summary: "供应商退款", Debit: input.CashBaseAmount})
	if input.BaseAmount.IsPositive() {
		lines = append(lines, postingLine{AccountKey: "ap", Summary: "贷项退款应付", Credit: input.BaseAmount, PartnerID: input.PartnerID})
	}
}
if fx.IsPositive() {
	if input.CreditType == "sales_return" {
		lines = append(lines, postingLine{AccountKey: "fx_loss", Summary: "退款汇兑损失", Debit: fx, PartnerID: input.PartnerID})
	} else {
		lines = append(lines, postingLine{AccountKey: "fx_gain", Summary: "退款汇兑收益", Credit: fx, PartnerID: input.PartnerID})
	}
} else if fx.IsNegative() {
	if input.CreditType == "sales_return" {
		lines = append(lines, postingLine{AccountKey: "fx_gain", Summary: "退款汇兑收益", Credit: fx.Abs(), PartnerID: input.PartnerID})
	} else {
		lines = append(lines, postingLine{AccountKey: "fx_loss", Summary: "退款汇兑损失", Debit: fx.Abs(), PartnerID: input.PartnerID})
	}
}
```

`PostCreditNoteRefund` 调用 `postVoucherTx`，类型/来源 `credit_note_refund`，摘要 `贷项退款 <code>`。

- [ ] **Step 3: 仓储退款事务**

`CreditNoteRepository.Refund`：

1. `FOR UPDATE` 锁定贷项单，读取金额/剩余/币种/汇率/信用类型/往来单位/组织/状态。
2. 校验 `status == open`、`remaining_amount > 0`、`amount <= remaining_amount`、`refundDate >= noteDate`、method 合法。
3. `base := domain.AllocateBase(remainingAmount, remainingBase, amount)`。
4. `cashBase := domain.CashBase(amount, exchangeRate)`；`fx := domain.FXAdjustment(cashBase, base)`。
5. 插入退款单（code/voucherCode 由服务生成），状态 `posted`。
6. 更新贷项：
   ```sql
   UPDATE erp.finance_credit_notes
   SET refunded_amount = refunded_amount + $3,
       refunded_base_amount = refunded_base_amount + $4,
       remaining_amount = remaining_amount - $3,
       remaining_base_amount = remaining_base_amount - $4,
       status = $5, updated_at = now()
   WHERE tenant_id = $1 AND id = $2
     AND remaining_amount >= $3 AND remaining_base_amount >= $4
   ```
7. `glpost.PostCreditNoteRefund`。
8. 更新退款单 `voucher_code`。
9. Commit，返回 `Get`。

- [ ] **Step 4: 服务层退款**

`CreditNoteService.Refund`：

- UUID 校验、金额 > 0、method `bank`/`cash`。
- 获取贷项单（用于币种/汇率/日期校验，仓储会再次锁定）。
- 汇率：
  ```go
  rate, err := fx.Resolve(ctx, s.rates, tenantID, note.Currency, refundDate, input.ExchangeRate)
  ```
- 校验 `domain.CashBase(input.Amount, rate).IsPositive()`；否则返回 `domain.ErrRefundAmountTooSmall`（HTTP 映射 400 `REFUND_AMOUNT_TOO_SMALL`）。
- 生成退款编码 `credit_note_refund` 和凭证编码 `gl_voucher`。
- 调用仓储。

- [ ] **Step 5: 验证**

```bash
gofmt -w internal/modules/gl/domain/voucher.go internal/modules/gl/infra/postgres/posting.go internal/modules/gl/infra/postgres/credit_note_refund_test.go internal/modules/finance/infra/postgres/credit_note_repository.go internal/modules/finance/app/credit_note_service.go cmd/api/main.go
go test ./internal/modules/gl/... ./internal/modules/finance/... -count=1
go vet ./internal/modules/gl/... ./internal/modules/finance/...
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add internal/modules/gl/domain/voucher.go internal/modules/gl/infra/postgres/posting.go internal/modules/gl/infra/postgres/credit_note_refund_test.go internal/modules/finance/infra/postgres/credit_note_repository.go internal/modules/finance/app/credit_note_service.go cmd/api/main.go
git commit -m "feat(stage-3j): refund credit notes with fx"
```

---

## Task 6: API、SDK 和 OpenAPI

**Files:**
- Modify: `internal/modules/finance/http/handler.go`
- Modify: `internal/modules/finance/app/invoice_service.go`
- Modify: `internal/modules/finance/infra/postgres/invoice_repository.go`
- Modify: `packages/sdk/src/finance.ts`
- Modify: `api/openapi.yaml`
- Modify: `scripts/test/check-frontend.mjs`

- [ ] **Step 1: HTTP 路由和 DTO**

`finance/http/handler.go` 路由：

```go
finance.POST("/credit-notes/:id/allocations", authmw.RequirePermission("finance.credit_note.apply"), handler.allocateCreditNote)
finance.GET("/credit-notes/:id/allocation-candidates", authmw.RequirePermission("finance.credit_note.apply"), handler.creditNoteAllocationCandidates)
finance.POST("/credit-notes/:id/refunds", authmw.RequirePermission("finance.credit_note.refund"), handler.refundCreditNote)
```

请求 DTO：

```go
type creditNoteAllocationRequest struct {
	AllocationDate string `json:"allocation_date"`
	Allocations    []struct {
		InvoiceID string `json:"invoice_id"`
		Amount    string `json:"amount"`
	} `json:"allocations"`
}

type creditNoteRefundRequest struct {
	RefundDate   string `json:"refund_date"`
	Amount       string `json:"amount"`
	ExchangeRate string `json:"exchange_rate"`
	Method       string `json:"method"`
	Reference    string `json:"reference"`
	Remark       string `json:"remark"`
}
```

- 解析日期、金额、汇率，校验必填。
- 调用服务。
- 响应使用 `toCreditNoteResponse`；详情包含 allocations/refunds。
- 错误映射新增：
  - `ErrCreditNoteClosed` → 409 `CREDIT_NOTE_CLOSED`
  - `ErrOverApply` → 409 `OVER_APPLY`
  - `ErrOverRefund` → 409 `OVER_REFUND`
  - `ErrInsufficientInvoiceBase` → 409 `INSUFFICIENT_INVOICE_BASE`
  - `ErrInvoiceTypeMismatch` → 400 `INVOICE_TYPE_MISMATCH`
  - `ErrOrgMismatch` → 400 `ORG_MISMATCH`
  - `ErrPartnerMismatch` → 400 `PARTNER_MISMATCH`
  - `ErrCurrencyMismatch` → 400 `CURRENCY_MISMATCH`
  - `ErrInvoicePaid` → 409 `INVOICE_PAID`
  - `ErrRefundAmountTooSmall` → 400 `REFUND_AMOUNT_TOO_SMALL`
  - `gldomain.ErrPeriodClosed` → 409 `PERIOD_CLOSED`
  - `fx.ErrRateNotFound` → 400 `EXCHANGE_RATE_NOT_FOUND`

- [ ] **Step 2: 发票详情抵用记录**

`invoice_service.go` 的 `InvoiceDetail` 增加：

```go
CreditNoteAllocations []*domain.CreditNoteAllocation
```

`invoice_repository.go` 增加：

```go
ListCreditNoteAllocations(ctx context.Context, filter scope.Filter, tenantID, invoiceID string) ([]*domain.CreditNoteAllocation, error)
```

查询 `finance_credit_note_allocations` 联表 `finance_credit_notes`，返回贷项单号/日期/金额/本位币/抵用日期。

`Get` 详情填充。

- [ ] **Step 3: SDK**

`packages/sdk/src/finance.ts` 增加：

```ts
export interface CreditNoteAllocation { id: string; credit_note_id: string; invoice_id: string; invoice_code: string; invoice_date: string; amount: string; base_amount: string; allocation_date: string; }
export interface CreditNoteRefund { id: string; code: string; credit_note_id: string; refund_date: string; currency: string; exchange_rate: string; amount: string; base_amount: string; cash_base_amount: string; fx_adjustment: string; method: string; reference: string; remark: string; voucher_code: string; }
export interface AllocateCreditNoteInput { allocation_date?: string; allocations: { invoice_id: string; amount: string }[] }
export interface RefundCreditNoteInput { refund_date: string; amount: string; exchange_rate?: string; method: string; reference?: string; remark?: string }
```

`CreditNote` 增加 `refunded_amount`、`refunded_base_amount`、`allocations?`、`refunds?`。
`InvoiceDetail` 增加 `credit_note_allocations`.

API：

```ts
creditNotes: {
  list(params?: CreditNoteListParams): Promise<Paged<CreditNote>>,
  get(id: string): Promise<CreditNote>,
  allocate(id: string, input: AllocateCreditNoteInput): Promise<CreditNote>,
  allocationCandidates(id: string): Promise<Invoice[]>,
  refund(id: string, input: RefundCreditNoteInput): Promise<CreditNote>
}
```

- [ ] **Step 4: OpenAPI 1.1.0**

增加：
- `/finance/credit-notes/{id}/allocations`
- `/finance/credit-notes/{id}/allocation-candidates`
- `/finance/credit-notes/{id}/refunds`
- 请求/响应 schema。
- `CreditNote` 新字段、`CreditNoteAllocation`、`CreditNoteRefund`。
- `InvoiceDetail.credit_note_allocations`。

- [ ] **Step 5: 完整度断言**

`scripts/test/check-frontend.mjs` 增加端点/SDK 字符串：

```js
'/finance/credit-notes/{id}/allocations:',
'/finance/credit-notes/{id}/allocation-candidates:',
'/finance/credit-notes/{id}/refunds:',
'CreditNoteAllocation',
'CreditNoteRefund',
'AllocateCreditNoteInput',
'RefundCreditNoteInput',
'creditNotes.allocate',
'creditNotes.refund',
'credit_note_allocations'
```

- [ ] **Step 6: 验证和提交**

```bash
gofmt -w internal/modules/finance/http/handler.go internal/modules/finance/app/invoice_service.go internal/modules/finance/infra/postgres/invoice_repository.go
go build ./...
go test ./internal/modules/finance/... ./internal/modules/gl/... -count=1
go vet ./internal/modules/finance/... ./internal/modules/gl/...
npx -y yaml@2 valid api/openapi.yaml
pnpm typecheck:web
pnpm test:frontend
git add internal/modules/finance/http/handler.go internal/modules/finance/app/invoice_service.go internal/modules/finance/infra/postgres/invoice_repository.go packages/sdk/src/finance.ts api/openapi.yaml scripts/test/check-frontend.mjs
git commit -m "feat(stage-3j): add credit note usage APIs"
```

---

## Task 7: 前端抵用/退款和历史

**Files:**
- Modify: `apps/web/src/views/finance/CreditNoteListView.vue`
- Modify: `apps/web/src/views/finance/InvoiceListView.vue`
- Modify: `scripts/test/check-frontend.mjs`

- [ ] **Step 1: 贷项列表和详情**

- 表格增加 `已退`、`剩余` 列（已有已冲/剩余则补已退）。
- open 行根据权限显示：
  - `抵用`（`finance.credit_note.apply`）
  - `退款`（`finance.credit_note.refund`）
- 详情 drawer：
  - 增加 `已退款`、`已退款本位币`。
  - 增加抵用历史表：发票号、发票日期、抵用金额、本位币、抵用日期。
  - 增加退款历史表：退款单号、日期、金额、本位币、现金本位币、汇兑调整、方式、凭证号。
  - 状态标签：`applied` + refunded>0 显示“已退款/已核销”。

- [ ] **Step 2: 抵用弹窗**

- 打开时调用 `financeApi.creditNotes.allocationCandidates(id)`。
- 表格列：发票号、日期、币种、含税总额、已收/已付、已冲、未付；输入 `抵用金额`。
- 默认抵用日期今天。
- 校验：
  - 至少一行金额 >0。
  - 单行 <= 未付。
  - 合计 <= 贷项剩余。
- 提交 `financeApi.creditNotes.allocate`。
- 成功后关闭并刷新列表/详情。

- [ ] **Step 3: 退款弹窗**

- 显示剩余金额。
- 字段：退款日期、金额、汇率（外币）、方式（bank/cash）、参考号、备注。
- 预览：贷项本位币（按比例）、现金本位币、汇兑调整。
- 校验金额 >0 且 <= 剩余。
- 提交 `financeApi.creditNotes.refund`。
- 成功后刷新。

- [ ] **Step 4: 发票详情抵用记录**

- `InvoiceListView.vue` 详情 drawer 增加 `贷项抵用` 表：
  - 贷项单号、抵用日期、金额、本位币。
- 使用 `detail.credit_note_allocations`。

- [ ] **Step 5: 完整度断言**

`scripts/test/check-frontend.mjs` 增加：
- `CreditNoteListView.vue`：`creditNotes.allocate`、`creditNotes.refund`、`allocationCandidates`、`refunded_amount`、`credit_note_allocations`。
- `InvoiceListView.vue`：`credit_note_allocations`。
- 菜单/路由无新增。

- [ ] **Step 6: 验证和提交**

```bash
pnpm typecheck:web
pnpm test:frontend
pnpm build:web
git add apps/web/src/views/finance/CreditNoteListView.vue apps/web/src/views/finance/InvoiceListView.vue scripts/test/check-frontend.mjs
git commit -m "feat(stage-3j): add credit note usage UI"
```

---

## Task 8: Stage 3J 接口测试、脚本、README 和 CI

**Files:**
- Create: `tests/api/stage3j_test.go`
- Create: `scripts/test/stage3j-api.ps1`
- Modify: `package.json`
- Modify: `README.md`
- Modify: `.github/workflows/ci.yml` if it enumerates stage scripts.

- [ ] **Step 1: 写测试**

`tests/api/stage3j_test.go`，`//go:build integration`，`package api_test`，`TestStage3J`。

复用：
- `createShippedOrder`（stage3h）创建应收发票。
- `postReturn`（stage3h）创建销售退货贷项。
- `stage3ICreatePurchaseReceipt` + `stage3IPostPurchaseReturn` 创建采购贷项。
- `ensureStage3HStock` 保证销售库存。

子测试：

1. `sales credit applies to another receivable invoice`
   - 创建发票 A（qty 2, 100, VAT0），退货 1 -> 贷项 100。
   - 创建发票 B（qty 1, 50, VAT0），未付 50。
   - 抵用 30 到 B：
     - 贷项 applied 30/remaining 70/status open。
     - B credited 30/outstanding 20/status partially_paid。
     - 抵用明细一条。
   - 再抵 70 到 B？B 未付 20，只抵 20，剩余 50；再创建发票 C 抵 50。
   - 校验贷项 applied 100/remaining 0/status applied，B/C 状态。
2. `purchase credit applies to another payable invoice`
   - 采购收货 A qty 2 100 -> 退货 1 -> 贷项 100。
   - 采购收货 B qty 1 50 -> 应付未付 50。
   - 抵用 50 -> B credited 50/paid；贷项 applied 50/remaining 50。
3. `initial application allocation history`
   - 退货贷项 applied 到原发票，详情 allocations 至少一条，invoice_id=原发票。
4. `allocation validation and permissions`
   - 跨往来单位/币种/组织/发票类型、已结清、超抵、超发票未付、零/负金额、已关闭贷项 -> 对应错误。
   - 匿名 401、demo_dept 403。
5. `customer credit refund with fx`
   - USD 销售发票 100 @7.2，退货 1 -> 贷项 100/base 720。
   - 零本位币退款：构造贷项 `remaining_amount > 0`、`remaining_base_amount = 0`（例如抵用/退款尾差场景），退款金额 > 0，断言凭证不含应收行，现金行与汇兑行平衡。
   - 退款 50，退款汇率 7.3：
     - 贷项 refunded 50/base 360，remaining 50/base 360。
     - 退款现金本位币 365，汇兑调整 +5。
     - 凭证 AR debit 360, bank credit 365, fx_loss debit 5；借贷平衡。
   - 再退 50 @7.2 -> 剩余 0，状态 applied；凭证无汇兑行。
6. `supplier credit refund with fx`
   - 采购 USD 发票 100 @7.2，退货 1 -> 贷项 100/base 720。
   - 退款 100 @7.3 -> 供应商退款凭证 bank debit 730, AP credit 720, fx_gain credit 10。
   - 零本位币退款：断言凭证不含应付行，现金行与汇兑行平衡。
7. `refund validation and rollback`
   - 超退、零/负金额、非法方式、已关闭贷项、关账期间退款 409 `PERIOD_CLOSED`，贷项/退款表/凭证回滚。
8. `refund does not change invoice credited`
   - 退款后原发票 credited 不变。
9. `invoice detail lists credit note allocations`
   - `GET /finance/invoices/:id` 返回 `credit_note_allocations`。
10. `allocation candidates`
    - 候选只包含同组织/往来单位/币种/方向且未结清发票。
11. `fx revaluation after credit application`
    - USD 发票 B 被贷项抵用后，期末重估只按剩余 base outstanding。
12. `concurrent apply/refund`
    - 两个并发抵用或退款请求只有一个成功，余额不双扣。

- [ ] **Step 2: 脚本和 package**

复制 `scripts/test/stage3i-api.ps1` 为 `stage3j-api.ps1`，改：
- `dist/api-stage3j.exe`
- `go test -tags=integration ./tests/api -run 'TestStage3J' -count=1 -v`

`package.json`：

```json
"test:api:stage3j": "powershell -ExecutionPolicy Bypass -File ./scripts/test/stage3j-api.ps1"
```

- [ ] **Step 3: README 和 CI**

`README.md`：
- 当前状态行增加 Stage 3J。
- 测试命令增加 `pnpm test:api:stage3j`。
- 增加 `### Stage 3J 贷项抵用与退款` 小节。

CI：如果 `.github/workflows/ci.yml` 枚举 stage 脚本，增加 stage3j；否则说明无需修改。

- [ ] **Step 4: 运行**

```bash
gofmt -w tests/api/stage3j_test.go
go vet -tags=integration ./tests/api
go test -tags=integration ./tests/api -run '^$'
pnpm test:api:stage3j
```

Expected: 全部通过。

- [ ] **Step 5: Commit**

```bash
git add tests/api/stage3j_test.go scripts/test/stage3j-api.ps1 package.json README.md
git commit -m "test(stage-3j): add credit note usage API tests"
```

---

## Task 9: 文档、静态检查和报告草稿

**Files:**
- Modify: `internal/modules/finance/README.md`
- Modify: `internal/modules/gl/README.md`
- Create: `docs/reports/2026-11-19-stage-3j-credit-note-refunds.md`

- [ ] **Step 1: 模块文档**

finance README：
- 贷项单字段增加 `refunded_amount`、`refunded_base_amount`。
- 增加 `finance_credit_note_allocations`、`finance_credit_note_refunds`、候选发票、抵用/退款 API、权限、错误码和已知限制。

gl README：
- 自动凭证规则增加 `credit_note_refund` 借贷方向。
- 抵用无凭证说明。

- [ ] **Step 2: 报告草稿**

创建 `docs/reports/2026-11-19-stage-3j-credit-note-refunds.md`，结构参考 Stage 3I：
1. 完成范围
2. 数据模型
3. 抵用计算与事务
4. 退款计算、汇兑与凭证
5. API/SDK/前端
6. 测试结果
7. 代码审核
8. 已知限制
9. 用户验收清单

测试/审核章节先留待回填，但不得保留“TODO”式空话；写“待 Task 10 完成后补充”。

- [ ] **Step 3: 静态检查**

```bash
gofmt -l ./cmd ./internal ./db ./tests
go test ./... -count=1
go vet ./...
npx -y yaml@2 valid api/openapi.yaml
pnpm typecheck:web
pnpm test:frontend
pnpm build:web
git diff --check
```

- [ ] **Step 4: Commit**

```bash
git add internal/modules/finance/README.md internal/modules/gl/README.md docs/reports/2026-11-19-stage-3j-credit-note-refunds.md
git commit -m "docs(stage-3j): add credit note usage report draft"
```

---

## Task 10: 独立审核、干净库 Stage 3B–3J、最终报告和推送

**Files:**
- Modify: `docs/reports/2026-11-19-stage-3j-credit-note-refunds.md`
- Any fix files from review.

- [ ] **Step 1: 独立代码审核**

派发审核子代理，审查 `fb0b80a..HEAD`，重点：
- 迁移约束/回填/down guard。
- 抵用和退款并发锁、余额不变量、发票 credited/status、候选过滤。
- 退款汇兑方向和凭证平衡。
- 抵用无凭证的口径。
- API 错误码、SDK/OpenAPI/前端一致性。
- 测试覆盖和 Stage 3H/3I 回归。

- [ ] **Step 2: 修复 Critical/Important**

按审核结论修复，重跑 Stage 3J 和 Stage 3H/3I 相关测试。

- [ ] **Step 3: 干净库全链路**

```bash
docker exec mlf-erp-dev-postgres-1 psql -U mlf -d postgres -c "DROP DATABASE IF EXISTS mlf_erp_stage3j_clean WITH (FORCE);"
docker exec mlf-erp-dev-postgres-1 psql -U mlf -d postgres -c "CREATE DATABASE mlf_erp_stage3j_clean;"
# DATABASE_URL 指向 clean 库，migrate/seed/demo，启动 API 18084
go test -tags=integration ./tests/api -run 'TestStage3' -count=1 -v
```

校验：
- migration 22、dirty=false。
- 贷项抵用/退款数量、凭证借贷平衡。
- Stage 3B–3J 全部通过。

- [ ] **Step 4: 填写报告审核和测试章节**

- 记录审核结论、修复项、最终复审。
- 记录 Stage 3J 子测试、静态检查、干净库结果和数据校验。
- 更新已知限制和用户验收清单。

- [ ] **Step 5: 提交并推送**

```bash
git add docs/reports/2026-11-19-stage-3j-credit-note-refunds.md
git commit -m "docs(stage-3j): finalize credit note usage report"
git push -u origin stage-3j-credit-note-refunds
```

---

## 验收清单

- [ ] 销售贷项可抵同客户应收发票，支持部分/多张。
- [ ] 采购贷项可抵同供应商应付发票。
- [ ] 发票 credited/outstanding/状态正确。
- [ ] 贷项 applied/refunded/remaining/status 正确。
- [ ] 初始退货抵用记录和历史回填完整。
- [ ] 客户/供应商退款生成正确凭证和汇兑损益。
- [ ] 退款不影响发票 credited。
- [ ] 权限、错误码、候选发票、详情历史完整。
- [ ] 前端抵用/退款操作和详情历史可用。
- [ ] Stage 3B–3J 干净库全链路通过。
- [ ] 独立审核无 Critical/Important 遗留。
