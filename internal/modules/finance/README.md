# finance 模块

统一应收/应付发票、贷项单与收付款核销模块，Stage 3B 覆盖：

```text
销售发货 → 应收发票 ─┐
                      ├→ 收款/付款 → 核销
采购收货 → 应付发票 ─┘

销售退货 → 贷项单 → 自动冲原应收发票未收金额（剩余保留客户贷项）
采购退货 → 贷项单 → 自动冲原应付发票未付金额（剩余保留供应商贷项）
```

## 分层

- `domain`：发票、贷项单、收付款、核销、状态、金额计算、仓储接口。
- `app`：发票/贷项单查询、收付款登记、核销用例。
- `infra/postgres`：PostgreSQL 查询与核销事务；核销时锁定收付款和发票行，校验类型、往来单位、币种、未核销金额，计算时扣减已冲贷项。
- `http`：Gin Handler、金额字符串转换、权限和审计。

## API

| 方法 | 路径 | 权限 |
|---|---|---|
| GET | `/api/v1/finance/invoices` | `finance.invoice.read` |
| GET | `/api/v1/finance/invoices/:id` | `finance.invoice.read` |
| GET | `/api/v1/finance/credit-notes` | `finance.invoice.read` |
| GET | `/api/v1/finance/credit-notes/:id` | `finance.invoice.read` |
| POST | `/api/v1/finance/credit-notes/:id/allocations` | `finance.credit_note.apply` |
| GET | `/api/v1/finance/credit-notes/:id/allocation-candidates` | `finance.credit_note.apply` |
| POST | `/api/v1/finance/credit-notes/:id/refunds` | `finance.credit_note.refund` |
| GET | `/api/v1/finance/payments` | `finance.payment.read` |
| GET | `/api/v1/finance/payments/:id` | `finance.payment.read` |
| POST | `/api/v1/finance/payments` | `finance.payment.write` |
| POST | `/api/v1/finance/payments/:id/allocate` | `finance.payment.write` |

贷项单列表支持 `credit_type`（`sales_return`/`purchase_return`）、`status`（`open`/`applied`）、`partner_id`、`keyword` 和分页。

创建收付款：

```json
{
  "payment_type": "receipt",
  "partner_id": "customer-uuid",
  "payment_date": "2026-11-17",
  "currency": "CNY",
  "amount": "500.00",
  "method": "bank",
  "reference": "BANK-001",
  "remark": ""
}
```

核销：

```json
{
  "allocations": [
    { "invoice_id": "invoice-uuid-1", "amount": "480.00" },
    { "invoice_id": "invoice-uuid-2", "amount": "20.00" }
  ]
}
```

## 发票

- `invoice_type`：`receivable` / `payable`。
- `source_type`：`sales_shipment` / `purchase_receipt`。
- 状态：`open → partially_paid → paid`；`paid` 表示已收款或已由贷项冲清，前端显示“已结清”。
- 应收由销售发货自动生成，应付由采购收货自动生成。
- 来源唯一约束：`(tenant_id, invoice_type, source_type, source_id)`，防止重复开票。
- 净额/税额/含税总额：`net_amount`、`tax_amount`、`total_amount`。
- 已冲贷项：`credited_amount`、`base_credited_amount`。
- 未收金额：`outstanding_amount = total_amount - paid_amount - credited_amount`。
- 状态重算同时考虑 `paid_amount` 和 `credited_amount`：
  - `open`：`paid + credited = 0`。
  - `partially_paid`：`0 < paid + credited < total`。
  - `paid`：`paid + credited >= total`。
- 增值税发票：`vat_invoice_no`、`vat_invoice_type`、`vat_invoice_date`、`vat_invoice_status`、购销方税号。
- 开票状态：`pending → issued → void`；开票/作废不生成凭证，税额已在发货/收货确认。
- 发票号租户内唯一（非空）。

## 贷项单（Stage 3H / 3I）

- 销售退货和采购退货共用 `finance_credit_notes` 台账，编码规则 `finance_credit_note`（前缀 `CN`）。
- `credit_type`：`sales_return`（销售退货，冲应收发票）或 `purchase_return`（采购退货，冲应付发票）。
- `source_type` 与 `credit_type` 一致；`source_id` 为退货单；`invoice_id` 为原应收/应付发票。
- 字段：来源退货单、原发票、往来单位、日期、币种、汇率、原币/本位币金额、已冲金额、已退金额、剩余金额、状态。
- 状态：`applied`（原币已无剩余，本位币在尾差容差内）或 `open`（仍有未使用贷项）。
- 自动冲销规则：锁定原发票，`outstanding = total - paid - credited`，
  `applied = min(credit_note.amount, outstanding)`，`remaining = amount - applied`。
- 原发票已收款/已付款时 `applied = 0`，贷项单保持 `open`，剩余金额分别展示为客户/供应商贷项。
- 原发票 `credited_amount` 和 `base_credited_amount` 同步累加；全额冲减时清空 `base_revalued_amount`。
- 零金额退货跳过贷项单，退货单 `credit_note_id` 为空。
- 贷项单不提供手工创建；Stage 3J 增加手工抵其他发票和退款，见下节。

## 贷项抵用与退款（Stage 3J）

### 余额与不变量

- `finance_credit_notes` 增加 `refunded_amount`、`refunded_base_amount`。
- 余额口径：

```text
remaining_amount      = amount      - applied_amount      - refunded_amount
remaining_base_amount = base_amount - applied_base_amount - refunded_base_amount
```

- 数据库检查保持 `amount/base_amount > 0`、各已用/剩余字段非负，以及：
  - `applied_amount + refunded_amount + remaining_amount = amount`
  - `applied_base_amount + refunded_base_amount + remaining_base_amount = base_amount`
  - 原币已冲/已退为 0 时本位币也必须为 0。
- 状态仍为 `open` / `applied`；剩余原币为 0 时 `applied`，前端按 `refunded_amount` 显示“已退款”或“已核销”。
- 贷项列表只返回汇总字段；详情才返回 `allocations`、`refunds` 数组。发票详情返回 `credit_note_allocations`。

### 抵用明细

`finance_credit_note_allocations` 保存贷项抵发票历史：

- 字段：租户、组织、贷项单、发票、原币/本位币抵用金额、抵用日期、创建人/时间。
- 允许同一贷项多次抵同一张或不同发票，不设 `(credit_note_id, invoice_id)` 唯一约束。
- 销售/采购退货创建时，自动冲原发票的金额也写入一条初始抵用记录；迁移 `000022` 为历史 `applied_amount > 0` 的贷项单回填一条原发票抵用记录。

### 抵用规则

- 手工选择多张发票；必须同租户、同组织、同往来单位、同币种、同方向：
  - `sales_return` → `receivable`
  - `purchase_return` → `payable`
- 支持部分抵用和多次抵用；单张不超过发票未付金额，合计不超过贷项剩余金额，请求内发票不能重复。
- 抵用日期默认当天，不能早于贷项单日期。
- 按请求顺序使用当前贷项剩余金额/本位币计算：

```text
base = round(remaining_base × amount / remaining_amount, 4)
if amount == remaining_amount:
    base = remaining_base
```

- 处理每行后扣减贷项剩余金额和本位币，保证多次抵用无尾差；全额抵完使用剩余本位币。
- 事务内锁定贷项单，按发票 ID 排序锁定发票，逐行校验并插入抵用记录，更新发票：
  - `credited_amount += amount`
  - `base_credited_amount += base`
  - 状态按 `paid + credited` 重算为 `open` / `partially_paid` / `paid`
  - 全额结清时 `base_revalued_amount = 0`
  - SQL 守卫保证 `paid + credited <= total` 且本位币同理。
- 再更新贷项单 `applied_*`、`remaining_*` 和状态，SQL 守卫保证剩余不为负。
- 抵用是子账重分类，不生成总账凭证，也不做期间关账校验。
- 候选接口 `GET /credit-notes/:id/allocation-candidates` 返回同组织、同往来单位、同币种、同方向且未结清的发票，不分页。

### 退款单

`finance_credit_note_refunds` 是独立退款单：

- 字段：单号、贷项单、退款日期、状态 `posted`、币种、退款日汇率、原币金额、贷项本位币 `base_amount`、现金本位币 `cash_base_amount`、汇兑调整 `fx_adjustment`、方式 `bank`/`cash`、参考号、备注、凭证号。
- 编码规则 `credit_note_refund`，前缀 `RF`，按日重置，4 位序号。
- `base_amount` 是本次退款消耗的贷项本位币；`cash_base_amount = round(amount × exchange_rate, 4)`；`fx_adjustment = cash_base_amount - base_amount`。
- 支持部分/多次/全额退款；退款金额不超过贷项剩余原币。
- 币种取贷项单币种；CNY 汇率固定 1，外币未传汇率时取退款日最新汇率。
- 退款日期不能早于贷项单日期，且所在期间必须 open；关账期间返回 409 `PERIOD_CLOSED`，退款单、贷项余额和凭证整体回滚。
- 退款更新 `refunded_*` 和 `remaining_*`，状态按剩余归零；不修改发票 `credited`。
- 零本位币退款：贷项仍有剩余原币但剩余本位币为 0 时允许退款，`base = 0`，凭证省略应收/应付行，由现金行和汇兑行平衡；若现金本位币四舍五入为 0，返回 400 `REFUND_AMOUNT_TOO_SMALL`。
- 客户退款（`sales_return`）生成借应收、贷银行/现金及汇兑损益；供应商退款（`purchase_return`）生成借银行/现金、贷应付及汇兑损益。详见 gl 模块 README。

### 权限与错误码

- `finance.credit_note.apply`：抵用及候选查询。
- `finance.credit_note.refund`：退款。
- 典型错误码：
  - 409 `CREDIT_NOTE_CLOSED`、`OVER_APPLY`、`OVER_REFUND`、`INVOICE_PAID`、`INSUFFICIENT_INVOICE_BASE`、`PERIOD_CLOSED`
  - 400 `PARTNER_MISMATCH`、`CURRENCY_MISMATCH`、`INVOICE_TYPE_MISMATCH`、`ORG_MISMATCH`、`BAD_REQUEST`、`EXCHANGE_RATE_NOT_FOUND`、`REFUND_AMOUNT_TOO_SMALL`
  - 401、403、404

### 已知限制

- 不支持抵用/退款反冲。
- 不允许跨组织抵用（预留配置），当前必须同组织。
- 不自动抵用，必须手工选择发票；不能抵未来发票。
- 抵用为子账操作，不生成凭证，也不校验期间关闭。
- 退款只支持银行/现金，不涉及 VAT 红字发票或税控红冲。
- 退款接口无审批流、无幂等键，客户端重试可能重复退款。
- 抵用不计汇兑；退款按退款日汇率确认汇兑损益；未使用贷项不参与外币重估。

## 多币种与汇兑损益（Stage 3F）

- `finance_exchange_rates` 按币种和生效日期维护汇率。
- 发票/收付款保存 `exchange_rate`、原币金额和本位币金额。
- 收付款核销按付款汇率和发票汇率差额生成 `fx_settlement` 凭证：
  - 应收差额计入汇兑收益/损失并调整应收。
  - 应付差额方向相反。
- 期末关账重估未核销外币应收应付，生成 `fx_revaluation` 和次期 `fx_revaluation_reversal`。
- 反关账撤销重估和次期反向凭证。
- 重估原币未核销金额扣减 `credited_amount`，账面本位币扣减 `base_credited_amount`；已由贷项全额冲减的发票不再重估。

## 收付款

- `payment_type`：`receipt` / `payment`。
- 状态：`unallocated → partially_allocated → allocated`。
- 收款只能核销应收，付款只能核销应付。
- 核销必须同往来单位、同币种。
- 支持部分核销和多张发票一次核销。

## 前端

- `/finance/invoices`：应收/应付列表、筛选、到期/逾期提示、发票详情、核销记录、已冲贷项、未收金额、开票/作废。
- `/finance/credit-notes`：贷项单列表和详情，销售/采购类型筛选与列、来源退货、原发票、已冲/已退/剩余、状态；open 贷项可打开抵用/退款弹窗，详情展示抵用历史、退款历史和退款凭证。
- `/finance/invoices`：发票详情显示贷项抵用记录。
- `/finance/payments`：收付款列表、新建、按往来单位加载未付清发票、批量核销。

## 后续

- 反核销、贷项抵用/退款反冲、红字发票、完整红冲流程。
- 跨组织抵用配置、自动抵用、退款审批和幂等键。
- 账龄分析、对账单、信用额度。
- 总账凭证自动生成（贷项抵用仍保持子账口径）。
