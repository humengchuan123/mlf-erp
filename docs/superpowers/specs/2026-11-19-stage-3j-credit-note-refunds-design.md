# Stage 3J 贷项单抵用与退款设计

> 日期：2026-11-19
> 分支：`stage-3j-credit-note-refunds`
> 状态：设计已确认

## 1. 目标

让 Stage 3H/3I 产生的销售/采购退货贷项形成完整闭环：

```text
销售退货 / 采购退货
        ↓
   贷项单（open）
        ├→ 抵其他应收/应付发票 → 更新发票 credited / outstanding
        └→ 客户/供应商退款 → 银行/现金 + 汇兑损益
```

## 2. 范围

### 本期

- 销售退货贷项抵其他应收发票，采购退货贷项抵其他应付发票。
- 手工选择多张发票，支持部分/多次抵用。
- 贷项单增加已退款金额，区分已抵、已退和剩余。
- 独立贷项退款单，支持部分/多次退款。
- 退款按退款日汇率计算现金本位币，并生成汇兑损益。
- 抵用为子账操作，不生成总账凭证。
- 新权限、API、SDK、OpenAPI、前端和接口测试。

### 后置

- 抵用/退款反冲。
- 跨组织抵用配置。
- 自动抵用、抵未来发票。
- VAT 红字发票/税控红冲。
- 退款审批和幂等键。

## 3. 数据模型

迁移 `000022_credit_note_usage`。

### 3.1 finance_credit_notes

新增：

- `refunded_amount numeric(19,4) NOT NULL DEFAULT 0`
- `refunded_base_amount numeric(19,4) NOT NULL DEFAULT 0`

余额口径改为：

```text
remaining_amount      = amount      - applied_amount      - refunded_amount
remaining_base_amount = base_amount - applied_base_amount - refunded_base_amount
```

替换 `finance_credit_notes_amount_check`：

```sql
CHECK (
  amount > 0 AND base_amount > 0
  AND applied_amount >= 0 AND applied_base_amount >= 0
  AND refunded_amount >= 0 AND refunded_base_amount >= 0
  AND remaining_amount >= 0 AND remaining_base_amount >= 0
  AND applied_amount + refunded_amount + remaining_amount = amount
  AND applied_base_amount + refunded_base_amount + remaining_base_amount = base_amount
)
```

状态保持 `open` / `applied`。剩余金额为零时 `applied`；前端根据 `refunded_amount` 显示“已退款”或“已核销”。

### 3.2 finance_credit_note_allocations

贷项抵发票明细：

```sql
CREATE TABLE erp.finance_credit_note_allocations (
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
```

索引：

- `(tenant_id, credit_note_id, allocation_date)`
- `(tenant_id, invoice_id)`

允许同一贷项多次抵同一/不同发票，不设 `(credit_note_id, invoice_id)` 唯一约束。

### 3.3 finance_credit_note_refunds

独立退款单：

```sql
CREATE TABLE erp.finance_credit_note_refunds (
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
```

索引：

- `(tenant_id, credit_note_id, refund_date DESC)`
- `(tenant_id, org_id, refund_date DESC)`

字段口径：

- `base_amount`：本次退款消耗的贷项本位币金额。
- `cash_base_amount`：`round(amount × exchange_rate, 4)`。
- `fx_adjustment`：`cash_base_amount - base_amount`，可正可负。

### 3.4 历史回填

对现有 `applied_amount > 0` 的贷项单，回填一条抵用记录到原发票：

```sql
INSERT INTO erp.finance_credit_note_allocations (...)
SELECT gen_random_uuid(), c.tenant_id, c.org_id, c.id, c.invoice_id,
       c.applied_amount, c.applied_base_amount, c.note_date, c.created_by
FROM erp.finance_credit_notes c
WHERE c.applied_amount > 0 AND c.invoice_id IS NOT NULL;
```

新产生的退货贷项在创建时也写入初始抵用记录。

### 3.5 权限与编码

- `finance.credit_note.apply`：贷项抵用。
- `finance.credit_note.refund`：贷项退款。
- 编码规则 `credit_note_refund`，前缀 `RF`，按日重置，4 位序号。
- admin 授权；seed 同步。

## 4. 抵用规则

### 4.1 校验

- 贷项单必须存在、状态 `open`、剩余原币和本位币大于 0。
- 发票必须同租户、同组织、同往来单位、同币种、同方向：
  - `sales_return` → `receivable`
  - `purchase_return` → `payable`
- 发票不能已结清，`outstanding = total - paid - credited > 0`。
- 抵用金额 > 0，单张不超过发票未付金额。
- 请求内发票不能重复。
- 抵用合计不超过贷项剩余金额。
- `allocation_date` 默认当天，不能早于贷项单日期。
- 抵用为子账操作，不做期间关账校验。

### 4.2 本位币计算

按请求顺序处理抵用行，使用当前贷项剩余金额/本位币：

```text
base = round(remaining_base × amount / remaining_amount, 4)
if amount == remaining_amount:
    base = remaining_base
```

- 必须先校验 `base <= invoice_base_outstanding`，否则返回 409 `INSUFFICIENT_INVOICE_BASE`。
- 每行处理完后扣减贷项剩余金额和本位币，保证多次抵用无尾差。
- 全额抵完时使用剩余本位币全额。

### 4.3 事务与更新

1. 锁定贷项单 `FOR UPDATE`。
2. 按发票 ID 排序锁定发票 `FOR UPDATE`。
3. 逐行校验并插入抵用明细。
4. 更新发票：
   - `credited_amount += amount`
   - `base_credited_amount += base`
   - 状态：`paid` / `partially_paid` / `open`
   - 全额结清时 `base_revalued_amount = 0`
   - SQL 守卫：`paid + credited + amount <= total` 且 `base_credited + base <= base_total`
5. 更新贷项单：
   - `applied_amount += total`
   - `applied_base_amount += totalBase`
   - `remaining_amount -= total`
   - `remaining_base_amount -= totalBase`
   - 剩余为零时 `applied`
   - 守卫：剩余金额/本位币不能为负
6. 不生成总账凭证。

## 5. 退款规则

### 5.1 校验

- 贷项单必须存在、状态 `open`、剩余金额大于 0。
- 退款金额 > 0，且不超过剩余金额。
- 退款日期不能早于贷项单日期。
- 币种取贷项单币种。
- 汇率：
  - CNY 固定 1。
  - 外币未传汇率时取退款日最新汇率。
  - 汇率必须 > 0。
- 方式 `bank` / `cash`。
- 退款日期所在期间必须 open。

### 5.2 金额计算

```text
base = round(remaining_base × amount / remaining_amount, 4)
if amount == remaining_amount:
    base = remaining_base

cash_base = round(amount × refund_rate, 4)
fx_adjustment = cash_base - base
```

- 更新贷项单 `refunded_amount`、`refunded_base_amount`、`remaining_*`，状态按剩余归零。
- 退款不修改发票 credited。

### 5.3 凭证

凭证类型/来源 `credit_note_refund`，日期为退款日期，摘要“贷项退款 <退款单号>”。

客户退款（`sales_return`）：

| 借方 | 贷方 |
|---|---|
| 应收账款（贷项本位币 base） | 银行/现金（现金本位币 cash_base） |
| 汇兑损失（fx_adjustment > 0） | 汇兑收益（fx_adjustment < 0） |

供应商退款（`purchase_return`）：

| 借方 | 贷方 |
|---|---|
| 银行/现金（现金本位币 cash_base） | 应付账款（贷项本位币 base） |
| 汇兑损失（fx_adjustment < 0） | 汇兑收益（fx_adjustment > 0） |

- `fx_adjustment = 0` 时省略汇兑行。
- 退款单、贷项余额更新和凭证在同一事务内完成。
- 凭证失败整体回滚；期间关账返回 409 `PERIOD_CLOSED`。

## 6. API

| 方法 | 路径 | 权限 |
|---|---|---|
| POST | `/api/v1/finance/credit-notes/:id/allocations` | `finance.credit_note.apply` |
| GET | `/api/v1/finance/credit-notes/:id/allocation-candidates` | `finance.credit_note.apply` |
| POST | `/api/v1/finance/credit-notes/:id/refunds` | `finance.credit_note.refund` |
| GET | `/api/v1/finance/credit-notes/:id` | `finance.invoice.read` |
| GET | `/api/v1/finance/invoices/:id` | `finance.invoice.read` |

抵用请求：

```json
{
  "allocation_date": "2026-11-19",
  "allocations": [
    { "invoice_id": "invoice-uuid", "amount": "100.00" }
  ]
}
```

退款请求：

```json
{
  "refund_date": "2026-11-19",
  "amount": "50.00",
  "exchange_rate": "7.3",
  "method": "bank",
  "reference": "REFUND-001",
  "remark": ""
}
```

响应：

- 贷项详情增加 `refunded_amount`、`refunded_base_amount`、`allocations`、`refunds`。
- 发票详情增加 `credit_note_allocations`。
- 候选发票返回同组织、同往来单位、同币种、同方向且未结清的发票。

错误码：

- 409 `CREDIT_NOTE_CLOSED`
- 409 `OVER_APPLY`
- 409 `OVER_REFUND`
- 409 `INVOICE_PAID`
- 409 `INSUFFICIENT_INVOICE_BASE`
- 409 `PERIOD_CLOSED`
- 400 `PARTNER_MISMATCH` / `CURRENCY_MISMATCH` / `INVOICE_TYPE_MISMATCH`
- 400 `BAD_REQUEST` / `EXCHANGE_RATE_NOT_FOUND`
- 404、401、403

## 7. SDK、OpenAPI 与前端

### SDK/OpenAPI

- `CreditNoteAllocation`、`CreditNoteRefund`、`AllocateCreditNoteInput`、`RefundCreditNoteInput`。
- `financeApi.creditNotes.allocate/refund/allocationCandidates`。
- `CreditNote` 增加已退字段和明细数组；`Invoice` 详情增加 `credit_note_allocations`。
- OpenAPI 升级 `1.1.0`。

### 前端

- 贷项单页面：
  - open 贷项显示“抵用”“退款”。
  - 详情展示已抵、已退、剩余，以及抵用/退款历史和凭证号。
- 抵用弹窗：
  - 加载候选发票。
  - 多行输入金额，展示发票未付和贷项剩余。
  - 校验合计、单张未付和日期。
- 退款弹窗：
  - 金额、日期、汇率、方式、参考号、备注。
  - 预览贷项本位币、现金本位币和汇兑调整。
- 发票详情显示贷项抵用记录。
- 权限控制和前端完整度断言。

## 8. 测试

- 销售贷项抵另一张应收发票：部分/全额/多张。
- 采购贷项抵另一张应付发票。
- 初始退货自动抵原发票的抵用记录和历史回填。
- 拒绝跨往来单位、跨币种、跨组织、错误发票类型、已结清、超抵、超发票未付、零/负金额、已关闭贷项。
- 客户部分/全额退款，校验贷项已退/剩余、凭证借贷和汇兑损益。
- 供应商部分/全额退款。
- 关账期间退款 409 并回滚。
- 退款不影响发票 credited。
- 期间重估按抵用后的发票未付本位币计算。
- 权限、数据范围、并发重复抵用/退款。
- Stage 3B–3J 干净库全链路。

## 9. 已知限制

- 不做抵用/退款反冲。
- 不允许跨组织（预留配置）。
- 不自动抵用，必须手工选择发票。
- 抵用为子账操作，不生成凭证，不校验期间关闭。
- 退款只支持银行/现金，不支持抵未来发票或退给其他往来单位。
- 不做 VAT 红字发票。
- 无审批流、无幂等键。
- 抵用不计汇兑；退款按退款日汇率确认汇兑损益。
- 未使用贷项不参与外币重估。
