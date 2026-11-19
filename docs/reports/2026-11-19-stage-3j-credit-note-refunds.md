# Stage 3J 汇总报告：贷项单抵用与退款

> 日期：2026-11-19
> 分支：`stage-3j-credit-note-refunds`
> HEAD：`8187423`
> 设计：`docs/superpowers/specs/2026-11-19-stage-3j-credit-note-refunds-design.md`
> 计划：`docs/superpowers/plans/2026-11-19-stage-3j-credit-note-refunds.md`
> 状态：报告草稿；独立审核、干净库 Stage 3B–3J 回归待 Task 10

---

## 1. 完成范围

### 1.1 贷项余额与使用闭环

- 销售退货、采购退货生成的贷项单不再只冲原发票，可继续：
  - 抵同客户其他应收发票 / 同供应商其他应付发票。
  - 向客户或供应商部分/多次/全额退款。
- `finance_credit_notes` 增加已退原币/本位币，余额口径从“已冲 + 剩余”扩展为“已冲 + 已退 + 剩余”。
- 贷项状态仍为 `open` / `applied`；剩余原币归零即 `applied`，前端按已退金额显示“已退款”或“已核销”。
- 退货创建时的初始冲减也写入抵用明细；迁移为历史已冲贷项回填原发票抵用记录。

### 1.2 手工抵用

- 手工选择一个或多个目标发票，支持部分抵用和多次抵用。
- 目标发票必须同租户、同组织、同往来单位、同币种、同方向。
- 抵用后更新发票 `credited_amount`、`base_credited_amount`、`outstanding_amount` 和状态。
- 抵用为子账重分类，不生成总账凭证，也不做期间关账校验。
- 提供候选发票接口，只返回未结清且条件匹配的发票。

### 1.3 退款与汇兑

- 新增独立贷项退款单，支持部分/多次/全额退款，退款金额不超过贷项剩余金额。
- 退款按退款日汇率计算现金本位币，并按贷项本位币与现金本位币差额确认汇兑损益。
- 客户退款生成借应收、贷银行/现金的凭证；供应商退款生成借银行/现金、贷应付的凭证。
- 零本位币剩余仍可退款：省略应收/应付行，由现金行和汇兑行平衡。
- 退款日期所在期间关闭时返回 409 `PERIOD_CLOSED`，退款单、贷项余额和凭证在同一事务内整体回滚。

### 1.4 权限、API、SDK、OpenAPI 与前端

- 新增 `finance.credit_note.apply`、`finance.credit_note.refund`，admin 授权并同步 seed。
- 新增抵用、候选、退款 API；贷项详情返回抵用/退款历史；发票详情返回贷项抵用记录。
- SDK 增加抵用/退款类型和 `financeApi.creditNotes.allocate/refund/allocationCandidates`。
- OpenAPI 升级 `1.1.0`，补齐路径、请求/响应模型和错误说明。
- 贷项单页面增加抵用/退款入口、余额、抵用历史、退款历史和凭证号；发票详情增加贷项抵用记录。
- 新增 `tests/api/stage3j_test.go`、`scripts/test/stage3j-api.ps1` 和 `test:api:stage3j`。

### 1.5 提交范围

| 阶段 | 提交 |
|---|---|
| 设计/计划 | `611decd` |
| 迁移 | `46a79e7`、`ac5652b` |
| 领域模型与金额计算 | `9c01473`、`530e861` |
| 初始抵用记录 | `7a6dc95`、`70d4a65` |
| 贷项抵用事务与 API | `9367b0c`、`3348ecd`、`67251d6` |
| 贷项退款与汇兑凭证 | `9991bf6`、`f6e5cc7` |
| API 契约与 OpenAPI/SDK | `8ae74fd`、`7099d97` |
| 前端 | `9402151`、`40bc34d` |
| 接口测试 | `c8b64bf`、`04a7765`、`ddd012c`、`8187423` |

## 2. 数据模型

### 2.1 迁移

新增 `db/migrations/000022_credit_note_usage`。

`finance_credit_notes` 增加：

- `refunded_amount numeric(19,4) NOT NULL DEFAULT 0`
- `refunded_base_amount numeric(19,4) NOT NULL DEFAULT 0`

余额与检查约束：

```text
remaining_amount      = amount      - applied_amount      - refunded_amount
remaining_base_amount = base_amount - applied_base_amount - refunded_base_amount
```

- `amount/base_amount > 0`，已冲、已退、剩余字段均不能为负。
- `applied_amount + refunded_amount + remaining_amount = amount`
- `applied_base_amount + refunded_base_amount + remaining_base_amount = base_amount`
- 原币已冲/已退为 0 时，对应本位币也必须为 0。
- `remaining` 为 0 时状态置 `applied`。

### 2.2 抵用明细表

`erp.finance_credit_note_allocations`：

- `id`、`tenant_id`、`org_id`、`credit_note_id`、`invoice_id`。
- `amount`（原币）、`base_amount`（本位币）、`allocation_date`、`created_by`、`created_at`。
- 复合外键引用贷项单和发票，`(tenant_id, id)` 唯一。
- 索引：`(tenant_id, credit_note_id, allocation_date)`、`(tenant_id, invoice_id)`。
- 不设 `(credit_note_id, invoice_id)` 唯一约束，允许同一贷项多次抵同一张发票。

### 2.3 退款单表

`erp.finance_credit_note_refunds`：

- `id`、租户、组织、单号、贷项单、退款日期、状态 `posted`、币种。
- `exchange_rate`、`amount`、`base_amount`、`cash_base_amount`、`fx_adjustment`。
- `method`（`bank` / `cash`）、`reference`、`remark`、`voucher_code`、创建人/时间。
- `(tenant_id, code)` 唯一。
- 检查：汇率 > 0、退款金额 > 0、`base_amount >= 0`、`cash_base_amount > 0`、`cash_base_amount - base_amount = fx_adjustment`。
- 索引：`(tenant_id, credit_note_id, refund_date DESC)`、`(tenant_id, org_id, refund_date DESC)`。

字段口径：

- `base_amount`：本次退款消耗的贷项本位币。
- `cash_base_amount = round(amount × exchange_rate, 4)`。
- `fx_adjustment = cash_base_amount - base_amount`，可正可负。

### 2.4 历史回填与 down guard

- 对 `applied_amount > 0` 且有不重复历史的贷项单，回填一条原发票抵用记录，使用贷项日期和创建人。
- 新退货贷项在创建时写入初始抵用记录。
- down 迁移在存在退款、非初始抵用（多行/金额不一致/非原发票）或已退余额时拒绝执行，避免丢失子账和总账状态。
- 移除两个新权限、编码规则和两张新表，并恢复旧版金额约束。

### 2.5 权限与编码

- `finance.credit_note.apply`：贷项抵用与候选查询。
- `finance.credit_note.refund`：贷项退款。
- 编码规则 `credit_note_refund`，前缀 `RF`，按日重置，4 位序号。
- 迁移和 seed 均同步权限；admin 默认授权。

## 3. 抵用规则与事务

### 3.1 候选发票

`GET /api/v1/finance/credit-notes/:id/allocation-candidates` 返回：

- 同租户、同组织、同往来单位、同币种。
- 方向匹配：`sales_return` → `receivable`，`purchase_return` → `payable`。
- `status <> paid` 且原币/本位币未付金额均大于 0。
- 不分页，按发票日期和发票号排序。

### 3.2 校验

- 贷项单存在、状态 `open`、剩余原币和本位币均大于 0。
- 请求至少一行，发票不能重复，金额必须 > 0。
- 每张发票不超过其未付金额；合计不超过贷项剩余金额。
- 抵用日期默认当天，不能早于贷项单日期。
- 发票必须同组织、同往来单位、同币种、同方向，且不能已结清。
- 同一 UUID 的不同写法先规范化再查重，避免绕过重复检查。
- 抵用不做期间关账校验。

### 3.3 本位币计算

按请求顺序处理，使用当前贷项剩余金额/本位币：

```text
base = round(remaining_base × amount / remaining_amount, 4)
if amount == remaining_amount:
    base = remaining_base
```

- 处理前校验 `base <= invoice_base_outstanding`，否则 409 `INSUFFICIENT_INVOICE_BASE`。
- 每行处理完后扣减贷项剩余原币和本位币，保证多次抵用和分批抵用无尾差。
- 全额抵完时使用剩余本位币。

### 3.4 事务与更新

1. 锁定贷项单 `FOR UPDATE`。
2. 按发票 ID 排序后逐张锁定发票，避免并发死锁；处理顺序仍按请求顺序。
3. 逐行校验并插入抵用明细。
4. 更新发票：
   - `credited_amount += amount`
   - `base_credited_amount += base`
   - 状态按 `paid + credited` 重算为 `open` / `partially_paid` / `paid`
   - 全额结清时 `base_revalued_amount = 0`
   - SQL 守卫保证 `paid + credited <= total`，本位币同理。
5. 更新贷项单：
   - `applied_amount += total`
   - `applied_base_amount += totalBase`
   - `remaining_amount -= total`
   - `remaining_base_amount -= totalBase`
   - 剩余归零时状态 `applied`
   - SQL 守卫保证剩余不能为负。
6. 不生成总账凭证。

### 3.5 余额不变量

- 贷项 `applied + refunded + remaining = amount`，本位币同理。
- 发票 `paid + credited <= total`，本位币同理。
- 并发抵用/退款通过贷项行锁串行化，失败方返回 `CREDIT_NOTE_CLOSED` 或 `OVER_APPLY` / `OVER_REFUND`。

## 4. 退款规则、汇兑与凭证

### 4.1 校验与金额

- 贷项单存在、状态 `open`、剩余原币大于 0。
- 退款金额 > 0 且不超过剩余原币。
- 退款日期不能早于贷项单日期。
- 币种取贷项单币种；CNY 汇率固定 1，外币未传汇率时取退款日最新汇率。
- 方式只允许 `bank` / `cash`。
- 退款日期所在期间必须 open。

```text
base = round(remaining_base × amount / remaining_amount, 4)
if amount == remaining_amount:
    base = remaining_base

cash_base = round(amount × refund_rate, 4)
fx_adjustment = cash_base - base
```

- 更新贷项 `refunded_amount`、`refunded_base_amount`、`remaining_*`，剩余归零时 `applied`。
- 退款不修改发票 `credited_amount`。
- 零本位币退款：贷项仍有剩余原币但 `remaining_base = 0` 时允许退款，`base = 0`；若 `cash_base` 四舍五入为 0，返回 400 `REFUND_AMOUNT_TOO_SMALL`。

### 4.2 凭证

凭证类型/来源 `credit_note_refund`，日期为退款日期，摘要“贷项退款 <退款单号>”，来源为退款单。

客户退款（`sales_return`）：

| 借方 | 贷方 |
|---|---|
| 应收账款 1122（贷项本位币 `base`） | 银行存款/现金（现金本位币 `cash_base`） |
| 汇兑损失 660301（`fx_adjustment > 0`） | 汇兑收益 660302（`fx_adjustment < 0`） |

供应商退款（`purchase_return`）：

| 借方 | 贷方 |
|---|---|
| 银行存款/现金（现金本位币 `cash_base`） | 应付账款 2202（贷项本位币 `base`） |
| 汇兑损失 660301（`fx_adjustment < 0`） | 汇兑收益 660302（`fx_adjustment > 0`） |

- `fx_adjustment = 0` 时省略汇兑行。
- `base = 0` 时省略应收/应付行，现金行与汇兑行平衡；销售/采购方向都覆盖。
- 退款单、贷项余额更新和凭证在同一事务内完成。
- 期间关账返回 409 `PERIOD_CLOSED`；已插入的退款行和贷项更新随事务回滚，期间内不留下 `credit_note_refund` 凭证。
- 贷项抵用不生成凭证；只有退款形成现金和汇兑损益时过账。

## 5. API、SDK、OpenAPI 与前端

### 5.1 API

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/api/v1/finance/credit-notes/:id/allocations` | `finance.credit_note.apply` | 抵用一张或多张发票 |
| GET | `/api/v1/finance/credit-notes/:id/allocation-candidates` | `finance.credit_note.apply` | 候选发票，不分页 |
| POST | `/api/v1/finance/credit-notes/:id/refunds` | `finance.credit_note.refund` | 创建退款单并过账 |
| GET | `/api/v1/finance/credit-notes/:id` | `finance.invoice.read` | 贷项详情，含抵用/退款数组 |
| GET | `/api/v1/finance/invoices/:id` | `finance.invoice.read` | 发票详情，含贷项抵用数组 |

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

列表与详情：

- 贷项列表返回汇总字段，包括 `refunded_amount`、`refunded_base_amount`、`remaining_*`，不返回明细数组。
- 贷项详情额外返回 `allocations`、`refunds`；无记录时返回空数组。
- 发票详情额外返回 `credit_note_allocations`；无记录时返回空数组。

错误码：

- 409 `CREDIT_NOTE_CLOSED`
- 409 `OVER_APPLY`
- 409 `OVER_REFUND`
- 409 `INVOICE_PAID`
- 409 `INSUFFICIENT_INVOICE_BASE`
- 409 `PERIOD_CLOSED`
- 400 `PARTNER_MISMATCH` / `CURRENCY_MISMATCH` / `INVOICE_TYPE_MISMATCH` / `ORG_MISMATCH`
- 400 `BAD_REQUEST` / `EXCHANGE_RATE_NOT_FOUND` / `REFUND_AMOUNT_TOO_SMALL`
- 401、403、404

### 5.2 SDK 与 OpenAPI

- SDK 增加 `CreditNoteAllocation`、`CreditNoteRefund`、`InvoiceCreditNoteAllocation`、`AllocateCreditNoteInput`、`RefundCreditNoteInput`。
- `CreditNote` 增加已退字段和可选明细数组；`InvoiceDetail` 增加 `credit_note_allocations`。
- `financeApi.creditNotes` 增加 `allocate`、`refund`、`allocationCandidates`。
- OpenAPI 版本 `1.1.0`，新增三条路径、请求/响应模型、候选接口不分页说明和错误响应。

### 5.3 前端

- `/finance/credit-notes`：
  - 列表增加已退款列和“已退款/已核销”状态显示。
  - open 贷项按权限显示“抵用”“退款”。
  - 详情展示已冲、已退、剩余的原币/本位币，以及抵用历史、退款历史和退款凭证号。
- 抵用弹窗：
  - 加载候选发票，展示发票未付和本位币未付。
  - 多行输入金额，前端预估每行本位币、合计和抵用后剩余。
  - 校验金额、合计和日期，提交后刷新列表与详情。
- 退款弹窗：
  - 输入金额、日期、汇率、方式、参考号和备注。
  - 预览贷项本位币、现金本位币和汇兑调整。
- 发票详情增加贷项抵用记录，无记录时显示空态。
- 权限控制和 `scripts/test/check-frontend.mjs` 完整度断言同步。

## 6. 测试结果

### 6.1 Stage 3J API 测试

```bash
pnpm test:api:stage3j
```

结果：PASS，`TestStage3J` 通过，14 个顶层子测试（含 3 个并发嵌套用例，共 17 个命名子测试），总耗时约 16.8 秒。

| 子测试 | 覆盖点 | 结果 |
|---|---|---|
| sales credit applies across receivable invoices | 已付款发票退货留下 open 贷项；抵两张应收的部分/多次/全额；发票 credited/outstanding/状态；贷项 applied/remaining/status；抵用历史；列表与详情汇总一致；抵用无凭证 | 通过 |
| purchase credit applies to payable invoice | 采购贷项抵另一张应付发票，部分抵用和发票状态 | 通过 |
| initial application allocation history | 销售/采购退货自动冲原发票的初始抵用记录；已付款发票退货无抵用记录 | 通过 |
| allocation validation | 跨往来单位/币种/组织/方向、已结清、超贷项、超发票未付、零/负金额、重复发票、早于贷项日期、非法日期、404；付款超核销仍为 400 `OVER_ALLOCATE` | 通过 |
| allocation candidates | 销售/采购候选方向、组织、往来单位、币种、未结清过滤；排除已付、异客商、外币、异组织发票 | 通过 |
| customer refund with fx | USD 客户部分退款 50@7.3：base 360、cash 365、FX loss 5；尾款 50@7.2 无汇兑行；两笔凭证借贷平衡；退款不改发票 credited | 通过 |
| supplier refund with fx | USD 供应商部分退款 40@7.3：bank 292、AP 288、FX gain 4；尾款 60@7.2 无汇兑行；退款不改发票 credited | 通过 |
| zero base refunds through API | 极小汇率 USD 贷项先消耗 0.0001 本位币，再零本位币退款：base 0、cash/FX 0.0001；客户省略 AR、供应商省略 AP，凭证平衡 | 通过 |
| refund required fields and validation | 缺日期/方式、空方式、零/负金额、非法方式、超退、早于贷项日期、404、关闭贷项 409；拒绝后无退款行 | 通过 |
| period closed refund rolls back | 关账期间退款 409 `PERIOD_CLOSED`；退款行、贷项余额和期间内 `credit_note_refund` 凭证均未留下 | 通过 |
| fx revaluation after credit application | 抵用后发票未付本位币减少；期末重估只按剩余 100×7.3 调整 +10；未使用贷项不重估 | 通过 |
| invoice detail lists credit note allocations | 发票详情返回贷项单号、类型、贷项日期、金额、汇率和抵用日期；无记录返回空数组 | 通过 |
| permissions | 匿名 401、demo_dept 抵用/退款/候选 403；demo_dept 贷项列表数据范围不泄露 Stage 3J 贷项 | 通过 |
| concurrent apply and refund | 父用例下覆盖 3 个并发场景，见下 | 通过 |
| ├ concurrent applications do not double credit | 3 个并发抵用仅 1 个成功，其余 `CREDIT_NOTE_CLOSED`；只生成 1 条抵用、发票合计 credited 50、仅 1 张结清 | 通过 |
| ├ concurrent refunds do not double refund | 3 个并发退款仅 1 个成功，其余 `CREDIT_NOTE_CLOSED`；只生成 1 条退款 | 通过 |
| └ concurrent application and refund serialize | 同一贷项并发抵用/退款都成功；applied/refunded/remaining 原币和本位币不变量成立；发票 credited 和退款凭证正确 | 通过 |

### 6.2 静态检查与构建

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

结果：

- `gofmt` 无输出。
- `go test ./... -count=1` 全部包通过，包括贷项本位币分摊、尾差、零现金退款、状态容差和退款凭证方向单元测试。
- `go vet ./...` 通过。
- OpenAPI YAML 校验通过。
- `pnpm typecheck:web` 通过。
- `pnpm test:frontend` 通过（前端完整度断言）。
- `pnpm build:web` 通过，仅有 Vite 大 chunk 体积提示，无构建错误。
- `git diff --check` 无空白错误；Git 仅提示 README 的 LF→CRLF 行尾转换。

### 6.3 干净数据库 Stage 3B–3J 全链路

待 Task 10 完成后补充：在全新数据库执行 migration 22、seed、demo，运行 `go test -tags=integration ./tests/api -run 'TestStage3' -count=1 -v`，并校验迁移版本、贷项抵用/退款数量、凭证借贷平衡和 Stage 3B–3J 全部子测试。

## 7. 代码审核

独立代码审核待 Task 10 完成后补充。实现阶段已通过连续 hardening 提交处理以下风险：

- 迁移：增加 down guard，防止在有退款、非初始抵用或已退余额时回滚丢数据；加强金额约束和原币/本位币为零的一致性。
- 领域金额：抽取并补测 `AllocateBase`、`CashBase`、`FXAdjustment`、`CreditNoteStatus`，覆盖尾差、全额分摊和 0.0001 容差。
- 抵用：UUID 规范化后查重、固定发票锁顺序、请求顺序分摊、发票和贷项 SQL 守卫、无凭证口径。
- 退款：退款日汇率解析、零现金拦截、零本位币凭证、事务回滚、凭证借贷平衡和销售/采购汇兑方向。
- API 契约：列表与详情数组区分、候选接口不分页、错误码与 OpenAPI/SDK 对齐。
- 前端：候选请求竞态保护、预估金额和权限控制、发票详情抵用记录。
- 测试：零本位币、混合并发、关账回滚、FX 重估隔离和跨组织候选排除。

## 8. 已知限制

- 不支持抵用/退款反冲。
- 不允许跨组织抵用（预留配置），当前必须同组织。
- 不自动抵用，必须手工选择发票；不能抵未来发票。
- 抵用为子账操作，不生成凭证，也不校验期间关闭。
- 退款只支持银行/现金，不涉及 VAT 红字发票/税控红冲。
- 退款接口无审批流、无幂等键，客户端重试可能重复退款。
- 抵用不计汇兑；退款按退款日汇率确认汇兑损益。
- 未使用贷项不参与外币重估。
- 贷项列表不返回抵用/退款明细数组，需要进入详情获取。
- 缺少浏览器 E2E 和故障注入测试。

## 9. 用户验收清单

- [ ] 销售贷项可抵同客户应收发票，支持部分/多张/多次。
- [ ] 采购贷项可抵同供应商应付发票。
- [ ] 候选发票只包含同组织、同往来单位、同币种、同方向且未结清的发票。
- [ ] 抵用后发票 credited/outstanding/状态正确。
- [ ] 贷项 applied/refunded/remaining/status 正确，原币和本位币不变量成立。
- [ ] 初始退货抵用记录和历史回填完整。
- [ ] 客户部分/全额退款生成正确凭证和汇兑损益。
- [ ] 供应商部分/全额退款生成正确凭证和汇兑损益。
- [ ] 零本位币退款省略应收/应付行且凭证平衡。
- [ ] 退款不影响发票 credited。
- [ ] 关账期间退款 409 并完整回滚。
- [ ] 权限、错误码、候选接口、贷项/发票详情历史完整。
- [ ] 前端抵用/退款弹窗、余额和详情历史可用。
- [ ] Stage 3J API 测试和静态检查通过。
- [ ] Task 10 干净库 Stage 3B–3J 全链路通过。
- [ ] Task 10 独立审核无 Critical/Important 遗留。
