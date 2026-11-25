# Stage 3K 汇总报告：收付款反核销

> 日期：2026-11-25
> 分支：`stage-3k-payment-reversals`
> 功能/测试 HEAD：`7060259`（Task 8 文档提交前）
> 设计：`docs/superpowers/specs/2026-11-25-stage-3k-payment-reversals-design.md`
> 计划：`docs/superpowers/plans/2026-11-25-stage-3k-payment-reversals.md`
> 状态：开发、接口测试用例和静态检查完成；独立审核、干净库 Stage 3B–3K 回归与最终验收待 Task 9

---

## 1. 完成范围

### 1.1 整行反核销闭环

- 对已核销/部分核销的收付款，按发票选择一条或多条核销记录整行反核销；不支持部分金额。
- 反核销删除有效核销行，写入独立反核销审计记录，并回退收付款已核销金额、本位币已核销金额和状态。
- 同步回退发票已收/已付金额、本位币已收/已付金额、未收/未付金额和状态。
- 发票状态按 `paid_amount + credited_amount` 重算，与现有核销、贷项抵用共用同一状态口径；反核销不修改贷项单、贷项抵用或退款记录。
- 反核销日期默认当天，不能早于收付款日期，且必须落在 open 期间；原因必填；请求内发票不能重复。

### 1.2 汇兑结算回退

- 对同一收付款+发票下每条 `posted` 的已实现汇兑结算，逐笔生成反方向 `fx_settlement_reversal` 凭证。
- 原汇兑结算标记 `reversed`，保留原结算金额和原 `fx_settlement` 凭证；反核销记录返回每个已反核销结算凭证号。
- 无汇兑结算时反核销为纯子账操作，不生成总账凭证；仍校验反核销日期所在期间 open。
- 收付款登记时的现金/银行凭证不撤销。

### 1.3 API、SDK、OpenAPI 与前端

- 新增 `POST /api/v1/finance/payments/:id/unallocate` 和 `finance.payment.reverse` 权限，admin 授权并同步 seed。
- 收付款详情增加 `allocations`、`reversals`；发票详情增加 `payment_allocation_reversals`；列表继续只返回汇总字段。
- SDK 增加 `PaymentAllocation`、`PaymentAllocationReversal`、`UnallocatePaymentInput`、`PaymentDetail` 和 `financeApi.payments.unallocate`。
- OpenAPI 升级 `1.2.0`，补齐路径、请求/响应模型和错误说明。
- 收付款页支持按核销行多选反核销、反核销日期和原因、反核销历史与汇兑凭证号；发票详情增加付款反核销记录。
- 新增 `tests/api/stage3k_test.go`、`scripts/test/stage3k-api.ps1` 和 `test:api:stage3k`。

### 1.4 提交范围

| 阶段 | 提交 |
|---|---|
| 设计/计划 | `599124c`；状态语义澄清 `50271e7` |
| 迁移 | `a84741a`、`bed889d` |
| 领域模型与状态 | `7af2136`、`aab837d` |
| 汇兑结算反核销凭证 | `d9f5624`、`fc757b9` |
| 反核销事务与详情 | `62dff3c`、`1316230`（含 `000025` 修复）、`b2a463b` |
| API/SDK/OpenAPI | `d082c6a`、`f1436a1` |
| 前端 | `838960d`、`4ab6b01` |
| 接口测试/脚本 | `7060259` |
| 文档/报告 | 本报告随 `docs(stage-3k): add payment reversal docs and report draft` 提交 |

## 2. 数据模型

### 2.1 迁移 24：反核销审计表

新增 `db/migrations/000024_payment_reversals`，创建 `erp.finance_payment_allocation_reversals`：

- 字段：`id`、`tenant_id`、`org_id`、`payment_id`、`invoice_id`、`allocation_id`。
- 金额：`amount`（原币，`> 0`）、`base_amount`（付款本位币，`>= 0`）、`invoice_base_amount`（发票本位币，`>= 0`），均为 `numeric(19,4)`。
- 审计：`reversal_date`、`reason`（`btrim(reason) <> ''`）、`created_by`、`created_at`。
- `(tenant_id, id)` 唯一；`(tenant_id, allocation_id)` 唯一，保证同一原核销行只能反核销一次。
- 复合外键引用 `finance_payments` 和 `finance_invoices`；索引 `(tenant_id, payment_id, reversal_date DESC)`、`(tenant_id, invoice_id)`。

`allocation_id` 保留原核销行 ID 便于审计；原核销行在反核销事务中删除。

### 2.2 迁移 24：汇兑结算反核销字段

`erp.finance_fx_settlements` 增加：

- `status text NOT NULL DEFAULT 'posted'`，取值 `posted` / `reversed`。
- `reversal_id uuid`：关联反核销审计记录。
- `reversal_voucher_id uuid`：关联 `gl_vouchers`；零调整额结算只标记 reversed 时为 `NULL`。
- `reversed_at timestamptz`、`reversed_by uuid`。
- 检查约束 `(status = 'reversed') = (reversal_id IS NOT NULL AND reversed_at IS NOT NULL AND reversed_by IS NOT NULL)`，防止状态和审计字段不一致。
- 索引 `(tenant_id, payment_id, invoice_id, status)` 和按 `reversal_id` 的部分索引。

迁移同时新增权限 `finance.payment.reverse`，并为 admin 角色授权。

### 2.3 迁移 25：已核销本位币修复

新增 `db/migrations/000025_payment_base_allocation_repair`，把 `finance_payments.base_allocated_amount` 回填为有效核销明细 `base_amount` 之和。

旧口径按付款总额一次性四舍五入，和逐条核销行本位币之和可能相差 0.0001；反核销必须按原核销行精确扣减，因此先修复历史余额，避免反核销后留下无法归零的本位币残差。当前迁移版本 25。

### 2.4 Down guard

`000024` down 迁移在存在反核销记录，或存在 `status = 'reversed'` / `reversal_id` / `reversed_at` / `reversed_by` 的汇兑结算时拒绝执行，避免删除反核销子账和总账审计状态；锁表后检查，再删除权限、约束、索引、扩展列和新表。`000025` 为数据修复迁移，down 为 no-op。

## 3. 反核销规则与事务

### 3.1 校验

- 收付款必须存在且属于当前数据范围，状态为 `allocated` / `partially_allocated`。
- 请求至少包含一张发票；发票 UUID 先规范化，再检查空白、非法和重复。
- 反核销原因去空白后不能为空；反核销日期默认当天，不能早于收付款日期。
- 每张发票必须存在、属于当前数据范围，且与收付款同往来单位、同币种、方向匹配：收款只能对应收，付款只能对应付。
- 发票必须仍有有效核销行；核销行必须属于该收付款，且反核销金额不超过付款已核销金额和发票已收/已付金额。
- 反核销日期所在期间必须 open。

### 3.2 事务步骤

1. 开启事务，锁定收付款 `FOR UPDATE`。
2. 按规范化后的发票 ID 排序，逐张锁定发票 `FOR UPDATE`，固定锁顺序避免并发死锁。
3. 锁定请求发票对应的 `finance_payment_allocations` 行；任一发票没有有效核销行返回 409 `ALLOCATION_NOT_FOUND`。
4. 汇总原币、付款本位币和发票本位币金额，校验不超过收付款/发票当前已核销余额。
5. 按核销行逐条插入反核销审计记录，记录原核销行 ID、三类金额、日期、原因和创建人。
6. 删除已反核销的有效核销行。
7. 更新收付款：`allocated_amount -= 原币合计`、`base_allocated_amount -= 付款本位币合计`，按剩余已核销金额重算状态；SQL 守卫要求两个余额均足够。
8. 更新每张发票：`paid_amount -= 原币金额`、`base_paid_amount -= 发票本位币金额`，按 `paid + credited` 重算状态；状态为 `paid` 时清空 `base_revalued_amount`；SQL 守卫要求余额足够。
9. 对每条 `posted` 汇兑结算生成反方向凭证并标记 `reversed`（见第 4 节）。
10. 最后统一执行一次反核销日期的期间 open 校验，提交事务；任一步失败整体回滚。
11. 返回收付款详情，包含剩余核销行和反核销历史。

### 3.3 状态与贷项

收付款状态：

```text
remaining = allocated_amount
unallocated        : remaining == 0
partially_allocated: 0 < remaining < amount
allocated          : remaining == amount
```

发票状态：

```text
settled = paid_amount + credited_amount
open              : total_amount > 0 且 settled == 0
partially_paid    : 0 < settled < total_amount
paid              : total_amount == 0 或 settled >= total_amount（4 位小数舍入）
```

- 发票状态统一调用 `Invoice.StatusAfterSettlement`，与核销、贷项抵用保持同一口径。
- 反核销只减少 `paid_amount` / `base_paid_amount`，不修改 `credited_amount` / `base_credited_amount`。
- 已由贷项冲减的发票反核销后可能从 `paid` 回到 `partially_paid`；贷项单、抵用记录和退款记录保持不变。

### 3.4 并发与回滚

- 收付款行锁串行化同一收付款的反核销；并发双反核销只成功一次，失败方返回 `PAYMENT_NOT_ALLOCATED` 或 `ALLOCATION_NOT_FOUND`。
- 并发反核销不同发票可同时成功；同一付款的核销与反核销通过行锁和 SQL 守卫收敛，余额始终等于有效核销行合计。
- 多发票请求中任一张发票不存在、无核销行或余额不足时，已插入的反核销记录、已删除的核销行和已更新的余额全部回滚。
- 关账期间在事务提交前返回 409 `PERIOD_CLOSED`，收付款、发票、核销行和凭证均不留下部分变更。

## 4. 汇兑结算回退与凭证

- 对同一收付款+发票下 `status = 'posted'` 的每条汇兑结算，生成一张反方向凭证。
- 凭证类型/来源：`fx_settlement_reversal`；凭证日期为反核销日期；摘要“汇兑结算反核销 <收付款号>”；来源 ID 为原汇兑结算 ID，来源单号为收付款号。
- 分录为原 `fx_settlement` 的完全反向：

| 原结算方向 | 原凭证 | 反核销凭证 |
|---|---|---|
| 应收，调整额 > 0 | 借应收 1122 / 贷汇兑收益 660302 | 借汇兑损失 660301 / 贷应收 1122 |
| 应收，调整额 < 0 | 借汇兑损失 660301 / 贷应收 1122 | 借应收 1122 / 贷汇兑收益 660302 |
| 应付，调整额 > 0 | 借汇兑损失 660301 / 贷应付 2202 | 借应付 2202 / 贷汇兑收益 660302 |
| 应付，调整额 < 0 | 借应付 2202 / 贷汇兑收益 660302 | 借汇兑损失 660301 / 贷应付 2202 |

- 原结算 `base_adjustment` 保留；反核销凭证使用其相反数，调用方传入原调整额，由过账函数内部取反一次。
- 原汇兑结算标记 `reversed` 并记录 `reversal_id`、`reversed_at`、`reversed_by`；非零调整额另记录 `reversal_voucher_id`。原 `fx_settlement` 凭证保持 `posted`，不修改、不删除。
- 调整额四舍五入为 0 的结算只标记 `reversed`，`reversal_voucher_id` 为空，不生成空凭证。
- 无汇兑结算时反核销不生成凭证；但仍执行期间校验，关账返回 409 `PERIOD_CLOSED`。
- 收付款登记时的现金/银行凭证不撤销；反核销只反向已实现汇兑结算。

## 5. API、SDK、OpenAPI 与前端

### 5.1 API

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/api/v1/finance/payments/:id/unallocate` | `finance.payment.reverse` | 按发票整行反核销 |
| GET | `/api/v1/finance/payments/:id` | `finance.payment.read` | 收付款详情，含核销/反核销数组 |
| GET | `/api/v1/finance/invoices/:id` | `finance.invoice.read` | 发票详情，含付款反核销数组 |

请求：

```json
{
  "invoice_ids": ["invoice-uuid-1", "invoice-uuid-2"],
  "reversal_date": "2026-11-25",
  "reason": "核销错误"
}
```

- `reversal_date` 可省略，省略时默认当天。
- 响应为收付款详情，包含 `allocations` 和 `reversals`；反核销记录字段包括 `payment_code`、`payment_date`、`invoice_code`、`invoice_date`、`amount`、`base_amount`、`invoice_base_amount`、`reversal_date`、`reason`、`fx_reversal_voucher_codes`、`created_by`、`created_at`。
- 发票详情 `payment_allocation_reversals` 使用同一反核销记录模型。
- 收付款列表不返回 `allocations` / `reversals`，避免列表负载膨胀。
- 错误码：
  - 400 `BAD_REQUEST` / `REASON_REQUIRED` / `DUPLICATE_INVOICE` / `PAYMENT_TYPE_MISMATCH` / `PARTNER_MISMATCH` / `CURRENCY_MISMATCH`
  - 404 `NOT_FOUND`
  - 409 `PAYMENT_NOT_ALLOCATED` / `ALLOCATION_NOT_FOUND` / `PERIOD_CLOSED`
  - 401、403

### 5.2 SDK 与 OpenAPI

- `PaymentAllocation`：核销行 ID、发票号、发票日期、原币/付款本位币/发票本位币金额、创建时间。
- `PaymentAllocationReversal`：反核销记录及 `fx_reversal_voucher_codes`。
- `UnallocatePaymentInput`：`invoice_ids`、可选 `reversal_date`、`reason`。
- `PaymentDetail extends Payment`，增加 `allocations` / `reversals`；`financeApi.payments.get` 返回详情，`financeApi.payments.unallocate` 返回更新后的详情；兼容入口 `financeApi.payment(id)` 返回同一详情。
- OpenAPI `1.2.0` 新增反核销路径、请求/响应模型、收付款详情数组、发票详情数组和错误响应。

### 5.3 前端

- `/finance/payments`：
  - 已核销/部分核销行按权限显示“反核销”。
  - 反核销弹窗加载收付款详情中的有效核销行，支持多选发票，展示原币、付款本位币和发票本位币金额。
  - 反核销日期默认当天，原因必填；提交 `financeApi.payments.unallocate`，成功后刷新列表和详情。
  - 详情展示核销历史和反核销历史，包括反核销日期、金额、原因和汇兑反核销凭证号。
  - 弹窗切换/关闭时用请求序号丢弃过期响应，避免异步竞态覆盖当前数据。
- `/finance/invoices`：发票详情增加收付款反核销记录。
- `scripts/test/check-frontend.mjs` 增加端点、SDK、权限、类型和中文文案完整度断言。

## 6. 测试结果

### 6.1 TestStage3K 用例数量

`tests/api/stage3k_test.go` 共有 **10 个顶层子测试**，其中 7 个嵌套命名子测试（3 个并发、2 个 full reversal、2 个 rounding），合计 **17 个命名子测试**。

### 6.2 覆盖清单

| 子测试 | 覆盖点 |
|---|---|
| `full reversal receivable and payable` | 父用例：全额反核销应收/应付 |
| `├ receivable` | CNY 应收 100 全额收款并反核销：收付款已核销/本位币归零、`unallocated`、未核销金额恢复；发票已收/本位币/未收/状态恢复；核销行删除、1 条反核销记录、无汇兑凭证；发票详情保留反核销记录 |
| `└ payable` | 同上，应付方向 |
| `multi invoice reversal and reallocation` | 一笔 200 收款核销两张 100 发票；反核销一张后状态 `partially_allocated`、核销/反核销数组正确；发票 B 恢复 open；重新核销 B；再次反核销后累计 2 条反核销记录；发票 A 保持 paid |
| `rounding parity multi invoice` | 父用例：USD 汇率 7.12345678 的多发票本位币舍入一致性 |
| `├ one allocation call` | 一次核销两张 USD 1.0000 发票；付款 `base_amount=14.2469`、`base_allocated_amount=14.2470`（逐行 7.1235 合计）；反核销两行后收付款与两张发票本位币全额归零，状态恢复 |
| `└ two allocation calls reversed one by one` | 同一付款分两次各核销 1.0000，逐张反核销：第一次后 `base_allocated_amount=7.1235`，第二次归零；覆盖逐行舍入和部分状态 |
| `fx settlement reversal and multiple settlements` | USD 应付发票 100@7.2、付款 100@7.3；分 40+60 两次核销聚合为一条核销行；反核销后原 2 张 `fx_settlement` 凭证保留 posted，生成 2 张 `fx_settlement_reversal` 凭证；校验类型/来源、`source_id`/`source_code`、摘要、反方向分录和借贷平衡；另覆盖应收正/负调整方向 |
| `credited invoice reversal status` | 发票 100 已收 40、贷项冲 60 后 paid；反核销后 paid 归零、credited 不变、outstanding 40、状态 `partially_paid`；贷项单金额/已冲/已退/剩余/状态和抵用历史不变；发票详情保留贷项抵用和付款反核销记录 |
| `closed period reversal rolls back` | 3 月已核销 USD 收付款，3 月关账后反核销返回 409 `PERIOD_CLOSED`；凭证总数、收付款、发票、核销行均不变，无 `fx_settlement_reversal`；重开期间后反核销成功并生成 1 张反核销凭证 |
| `validation` | 空选择、空/空白原因、空白/非法/重复（大小写和花括号变体）发票 ID、未知收付款/发票、未核销发票、收付款类型不匹配、早于付款日期、非法日期、未核销收付款、已反核销；拒绝后收付款/发票/核销/反核销状态不变 |
| `concurrency` | 父用例：并发反核销串行化 |
| `├ concurrent double reversal succeeds once` | 同一发票 40+60 两笔核销，两个并发反核销仅 1 个成功，另一个返回 409 `PAYMENT_NOT_ALLOCATED`/`ALLOCATION_NOT_FOUND`；最终 1 条反核销记录、2 张汇兑反核销凭证 |
| `├ concurrent different invoices both succeed` | 同一付款两张发票并发反核销均成功；最终核销行 0、反核销记录 2、两张发票 open |
| `└ concurrent allocate and unallocate invariant` | 同一付款并发核销/反核销，按允许结果收敛；校验 payment allocated/base 等于核销行合计、状态与未核销金额、1 条反核销记录、invoice paid 等于核销行合计及发票状态 |
| `detail and scope/permissions` | 空/已核销/反核销后的收付款与发票详情数组；列表不返回 `allocations`/`reversals`；匿名 401、demo_dept 反核销 403、详情 403/404、列表隔离 |
| `reversal date and failure rollback` | 早于付款日期/非法日期拒绝且无副作用；省略日期默认当天；多发票请求含未知第二张发票返回 404 并整体回滚，收付款/两张发票/核销行不变 |

### 6.3 self scope 覆盖限制

`detail and scope/permissions` 覆盖了匿名 401、部门用户 403、详情 403/404 和列表隔离，但以下正向场景未纳入 API 回归：

- self-scope 用户查看本人创建收付款、但发票由他人创建时，仍应看到该收付款的核销/反核销数组。
- self-scope 用户查看本人创建发票、但收付款或贷项单由他人创建时，仍应看到付款核销/反核销数组。

原因：当前没有 self-scope 演示账号，用户 API 也不能设置 `data_scope`。测试改为由 admin 详情验证父单据范围下的数组返回，控制器范围逻辑由单元测试覆盖。该缺口已登记为已知限制，本报告不声称上述正向跨创建人场景已被 API 测试覆盖。

### 6.4 执行状态

Task 7 已提交 `tests/api/stage3k_test.go`、`scripts/test/stage3k-api.ps1` 和 `pnpm test:api:stage3k`。该套件依赖 PowerShell、Docker 和可写的 Compose PostgreSQL `mlf_erp` 数据库，并会消耗测试期间/年度；Task 8 未在本机重跑，因此本报告不声称 `TestStage3K` 已通过。最终通过结果和干净库 Stage 3B–3K 回归由 Task 9 执行并回填。

`go vet -tags=integration ./tests/api` 已通过，确认集成测试包可编译。

### 6.5 静态检查与构建

本次 Task 8 实际执行：

| 命令 | 结果 |
|---|---|
| `gofmt -l ./cmd ./internal ./db ./tests` | 无输出 |
| `go test ./... -count=1` | 全部包通过，无失败包 |
| `go vet ./...` | 通过，无输出 |
| `npx -y yaml@2 valid api/openapi.yaml` | 通过，无输出 |
| `pnpm typecheck:web` | 通过，无类型错误 |
| `pnpm test:frontend` | `Frontend completeness check passed.` |
| `pnpm build:web` | 通过；Vite 仅提示 `index-*.js` 约 967.60 kB 超过 500 kB 的 chunk 体积告警，无构建错误 |
| `git diff --check` | 无空白错误；Git 仅提示 finance/gl README 的 LF→CRLF 行尾转换 |

## 7. 代码审核（pending Task 9）

**pending Task 9。** 截至本报告草稿，尚未执行独立代码审核，不给出审核通过结论。

Task 9 计划：

- 独立审核 `60c3542..HEAD`（Stage 3J 最终提交之后到 Stage 3K 当前 HEAD）。
- 修复 Critical/Important，并重跑 Stage 3K API 测试和静态检查。
- 在干净库 `mlf_erp_stage3k_clean` 应用 migration 24/25、seed、demo，执行 Stage 3B–3K 全链路。
- 校验收付款/发票余额、反核销记录、汇兑结算状态、凭证借贷平衡和跨账一致性。
- 回填本报告测试结果、遗留风险和最终提交信息，推送 `stage-3k-payment-reversals`。

实现阶段已通过连续 hardening 提交处理以下风险：

- 迁移：反核销审计唯一键、原因非空、汇兑结算状态/审计一致性检查、down guard。
- 领域：收付款/发票状态统一口径，UUID 规范化后查重。
- 事务：固定发票锁顺序、SQL 余额守卫、多发票失败整体回滚。
- 金额：逐行核销本位币合计与付款已核销本位币对齐，迁移 25 修复历史舍入差异。
- 汇兑：反方向凭证只取反一次，原结算/原凭证保留，零调整额不生成空凭证。
- 数据范围：收付款/反核销数组按父收付款范围过滤，发票反核销数组按父发票范围过滤。
- API 契约：列表/详情数组区分，错误码、SDK 和 OpenAPI 对齐。
- 前端：反核销弹窗请求竞态保护、整行选择和权限控制。

## 8. 已知限制

- 只支持整行反核销，不支持部分金额。
- 不包含反核销审批流和幂等键。客户端重试可能重复提交；第二次通常因核销行已不存在返回 409，但生产启用前应补服务端幂等键和审批控制。
- 不撤销收付款本身的现金/银行凭证。
- 不包含贷项抵用/退款反冲、红字发票或完整红冲流程；反核销只重算发票状态，不修改贷项相关数据。
- 反核销日期必须在 open 期间，且不能早于收付款日期。
- 已反核销不能恢复；需要恢复时只能重新执行核销。
- 无汇兑结算的反核销不生成总账凭证；只有已实现汇兑结算会生成 `fx_settlement_reversal` 凭证。
- 反核销采用“删除有效核销行 + 反核销审计表”模型，不在原核销行上保留版本历史。
- `000025_payment_base_allocation_repair` 为数据修复迁移，down 为 no-op，不能自动还原修复前的历史本位币余额。
- self scope 正向跨创建人场景没有 API 回归：当前没有 self-scope 演示账号，用户 API 不能设置 `data_scope`；父单据范围数组由 admin 详情断言，控制器范围由单元测试覆盖。
- 缺少浏览器 E2E 和故障注入测试。

## 9. 用户验收清单

> 待 Task 9 干净库回归和用户验收后勾选。

- [ ] 收付款核销行可按发票整行反核销，不支持部分金额的边界明确。
- [ ] 反核销后收付款已核销金额、未核销金额、本位币金额和状态正确回退。
- [ ] 反核销后发票已收/已付、未收/未付、本位币金额和状态正确回退。
- [ ] 多发票、部分核销、全额核销和反核销后重新核销可用。
- [ ] 贷项已冲减的发票反核销后按 `paid + credited` 重算，贷项单/抵用/退款数据不变。
- [ ] 已实现汇兑结算逐笔生成反方向 `fx_settlement_reversal` 凭证，原结算/原凭证保留，结算状态 `reversed`，凭证借贷平衡。
- [ ] 无汇兑结算反核销不生成凭证；关账期间返回 409 `PERIOD_CLOSED` 并整体回滚。
- [ ] 权限 401/403、错误码、数据范围、收付款/发票详情历史和列表汇总符合契约。
- [ ] 前端反核销弹窗、整行多选、日期/原因校验、历史展示和权限控制可用。
- [ ] `TestStage3K` 10 个顶层/17 个命名子测试通过（Task 9 回填）。
- [ ] 静态检查全部通过（本任务已实测）。
- [ ] 干净库 Stage 3B–3K 全链路通过（Task 9）。
- [ ] 独立审核无 Critical/Important 遗留（Task 9）。
