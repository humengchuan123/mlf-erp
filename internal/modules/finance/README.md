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
| POST | `/api/v1/finance/payments/:id/unallocate` | `finance.payment.reverse` |

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
- 状态：`applied`（原币已无剩余）或 `open`（仍有剩余原币）。原币归零后即使仍有本位币尾差也保持 `applied`；该本位币余额只是展示/汇兑残值，不可再抵用或退款。
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
- 状态仍为 `open` / `applied`；剩余原币为 0 时一律 `applied`，不因本位币残值（正数 `remaining_base_amount`）回到 `open`。该本位币残值仅用于展示和汇兑口径，`remaining_amount = 0` 时不能抵用或退款。前端按 `refunded_amount` 显示“已退款”或“已核销”。
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
- 所有跨汇率抵用均拒绝（部分和全额）；贷项汇率必须与目标发票汇率一致，否则返回 409 `CROSS_RATE_ALLOCATION`。抵用是子账重分类，不生成汇兑凭证；跨汇率抵用会使贷项消耗的本位币与发票应冲减的本位币不一致，留下无法在抵用时结清的 AR/AP 本位币残差。全额抵清仍保留本位币相等守卫（4 位小数容差）作为第二道防线，覆盖历史跨汇率部分抵用或舍入造成的偏差。
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
- 候选接口 `GET /credit-notes/:id/allocation-candidates` 返回同组织、同往来单位、同币种、同汇率且同方向且未结清的发票，不分页。

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
  - 409 `CREDIT_NOTE_CLOSED`、`OVER_APPLY`、`OVER_REFUND`、`INVOICE_PAID`、`INSUFFICIENT_INVOICE_BASE`、`CROSS_RATE_ALLOCATION`、`PERIOD_CLOSED`
  - 400 `PARTNER_MISMATCH`、`CURRENCY_MISMATCH`、`INVOICE_TYPE_MISMATCH`、`ORG_MISMATCH`、`BAD_REQUEST`、`INVALID_EXCHANGE_RATE`、`EXCHANGE_RATE_NOT_FOUND`、`REFUND_AMOUNT_TOO_SMALL`
  - 401、403、404

### 已知限制

- 不支持抵用/退款反冲。
- 不允许跨组织抵用（预留配置），当前必须同组织。
- 不自动抵用，必须手工选择发票；不能抵未来发票。
- 抵用为子账操作，不生成凭证，也不校验期间关闭。
- 退款只支持银行/现金，不涉及 VAT 红字发票或税控红冲。
- 退款只能退给贷项单往来单位，不支持退给其他往来单位。
- 退款接口无审批流，且会真实出账。Stage 4A 已支持可选 `Idempotency-Key`：同一 key 重试不会重复退款；生产强制前客户端必须传 key，超时/断连应使用同一 key 重试，不要人工重复提交。
- 所有跨汇率抵用（部分和全额）均被拒绝（409 `CROSS_RATE_ALLOCATION`）：贷项汇率必须与每张目标发票汇率一致。抵用为子账重分类且不生成汇兑凭证，跨汇率抵用无法在抵用时结清贷项消耗本位币与发票冲减本位币之间的差额，会留下 AR/AP 本位币残差；全额抵清的本位币相等守卫（4 位小数容差）仅作为历史数据/舍入偏差的第二道防线。尚无自动换汇或差额单独过账。
- 原币归零的贷项单一律 `applied`，退货创建时也按原币归零判定；正数本位币残值只用于展示/汇兑，不能抵用或退款。
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

## 收付款反核销（Stage 3K）

- 对已核销/部分核销的收付款按发票整行反核销；一次请求可选择一张或多张发票，每张发票撤销其全部有效核销行，不支持部分金额。
- 反核销日期默认当天，不能早于收付款日期，且必须落在 open 期间；原因必填；请求内发票不能重复，UUID 先规范化再查重。
- 事务内锁定收付款，按发票 ID 排序锁定发票，再锁定对应核销行；先写入反核销记录，再删除有效核销行，并回退：
  - 收付款 `allocated_amount`、`base_allocated_amount`，状态按剩余已核销金额重算为 `unallocated` / `partially_allocated` / `allocated`。
  - 发票 `paid_amount`、`base_paid_amount`，状态按 `paid_amount + credited_amount` 重算为 `open` / `partially_paid` / `paid`；结清时清空 `base_revalued_amount`。`outstanding_amount` 不落库，由 API 按 `total_amount - paid_amount - credited_amount` 实时计算。
  - 收付款和发票更新均带 SQL 守卫，余额不足时整体回滚。
- 反核销只减少已收/已付，不修改 `credited_amount`、贷项单、贷项抵用或退款记录；已由贷项冲减的发票可能从 `paid` 回到 `partially_paid`。
- 已实现汇兑结算：对同一收付款+发票下每条 `posted` 结算逐笔生成反方向 `fx_settlement_reversal` 凭证，结算标记 `reversed` 并记录 `reversal_id`、`reversed_at`、`reversed_by`，非零调整额另记录 `reversal_voucher_id`；原结算和原 `fx_settlement` 凭证保留。调整额四舍五入为 0 的结算只标记 `reversed`，不生成凭证。
- 无汇兑结算时反核销是纯子账操作，不生成总账凭证；收付款登记时的现金/银行凭证也不撤销。无论是否有汇兑结算，都校验反核销日期所在期间 open；关账返回 409 `PERIOD_CLOSED` 并整体回滚。
- API：`POST /api/v1/finance/payments/:id/unallocate`，权限 `finance.payment.reverse`。请求：

```json
{
  "invoice_ids": ["invoice-uuid-1", "invoice-uuid-2"],
  "reversal_date": "2026-11-25",
  "reason": "核销错误"
}
```

- 响应为收付款详情，增加 `allocations` 和 `reversals`；反核销记录包含原币金额、付款本位币金额、发票本位币金额、日期、原因、创建人，以及 `fx_reversal_voucher_codes`（每个已反核销汇兑结算凭证一项）。原核销行 ID 仅保存在审计表 `finance_payment_allocation_reversals.allocation_id`，不在 API 响应中暴露。发票详情增加 `payment_allocation_reversals`。列表仍只返回汇总字段，不返回明细数组。
- 典型错误码：400 `BAD_REQUEST` / `REASON_REQUIRED` / `DUPLICATE_INVOICE` / `PAYMENT_TYPE_MISMATCH` / `PARTNER_MISMATCH` / `CURRENCY_MISMATCH`；404 `NOT_FOUND`；409 `PAYMENT_NOT_ALLOCATED` / `ALLOCATION_NOT_FOUND` / `PERIOD_CLOSED`；401/403。
- 已知限制：
  - 只支持整行反核销，不支持部分金额。
  - 不包含反核销审批；Stage 4A 已支持可选 `Idempotency-Key`，同一 key 重试不会重复反核销；生产强制前客户端必须传 key。
  - 不撤销收付款本身的现金/银行凭证，也不包含贷项抵用/退款反冲、红字发票。
  - 反核销日期必须在 open 期间。
  - 已反核销不能恢复；需要重新核销时重新执行核销。
  - 原核销行删除、历史只保留在反核销审计表，不在原核销行上保留版本。
  - self scope 的负向隔离已纳入 API 回归：测试通过 `/system/roles`、`/system/users` 动态创建 self 用户（无 `org_memberships`，默认 `self`），验证其读取他人创建的收付款/发票详情返回 404。正向跨创建人可见性（本人收付款+他人发票、本人发票+他人收付款）仍未纳入 API 回归：用户 API 不能设置 `data_scope`，构造该场景还需要更宽的写入权限组合。父单据范围数组由 admin 详情验证；范围 SQL 的专项回归仍未覆盖，仅由代码审查确认；该缺口见 `tests/api/stage3k_test.go` 注释和报告已知限制。

## 资金操作幂等（Stage 4A）

- 受保护端点：
  - `POST /api/v1/finance/credit-notes/:id/allocations`
  - `POST /api/v1/finance/credit-notes/:id/refunds`
  - `POST /api/v1/finance/payments`
  - `POST /api/v1/finance/payments/:id/allocate`
  - `POST /api/v1/finance/payments/:id/unallocate`
- 请求头 `Idempotency-Key`：8–128 字符，允许 `A-Z a-z 0-9 . _ : -`；作用域为租户+用户+方法+路径。
- 同一 key + 相同请求体：回放首次响应，响应头 `Idempotency-Replayed: true`，不重复产生退款/收付款/核销/反核销。
- 同一 key + 不同请求体：409 `IDEMPOTENCY_CONFLICT`。
- 并发同 key 同请求：409 `IDEMPOTENCY_IN_PROGRESS`；首次完成后可重试回放。
- Header 默认可选；`IDEMPOTENCY_REQUIRED=true` 时缺失返回 400 `IDEMPOTENCY_KEY_REQUIRED`；非法 key 返回 400 `IDEMPOTENCY_KEY_INVALID`。
- 默认 TTL 24 小时；5xx/401/403/429 标记失败，可用同一 key 重试。
- 前端在打开退款/收付款/核销/反核销弹窗时生成 `newIdempotencyKey()`（优先 `crypto.randomUUID`，非 HTTPS 回退 `getRandomValues`），成功后清空；网络/5xx/401/403/408/429 及幂等冲突/进行中保留 key，其余 4xx 旋转新 key。
- 已知限制：响应体明文存 PostgreSQL；超过 4 MB 无法回放（409 `IDEMPOTENCY_RESPONSE_UNAVAILABLE`）；清理任务运行在 API 进程内；不覆盖消息队列/Outbox 重试。

## 前端

- `/finance/invoices`：应收/应付列表、筛选、到期/逾期提示、发票详情、核销记录、已冲贷项、未收金额、开票/作废。
- `/finance/credit-notes`：贷项单列表和详情，销售/采购类型筛选与列、来源退货、原发票、已冲/已退/剩余、状态；open 贷项可打开抵用/退款弹窗，详情展示抵用历史、退款历史和退款凭证。
- `/finance/invoices`：发票详情显示贷项抵用记录。
- `/finance/payments`：收付款列表、新建、按往来单位加载未付清发票、批量核销；已核销/部分核销行可按核销记录整行反核销，详情展示核销历史和反核销历史（含汇兑反核销凭证）。

## 后续

- 反核销的反向恢复/审批、贷项抵用/退款反冲、红字发票、完整红冲流程。
- 跨组织抵用配置、自动抵用、退款审批和幂等键。
- 账龄分析、对账单、信用额度。
- 总账凭证自动生成（贷项抵用仍保持子账口径）。
