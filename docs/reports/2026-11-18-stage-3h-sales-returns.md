# Stage 3H 汇总报告：销售退货与贷项单

> 日期：2026-11-18
> 分支：`stage-3h-sales-returns`
> 提交：`a3e5c6a`（设计/计划）+ `f6b667d..a103db3`（实现与测试）
> 设计：`docs/superpowers/specs/2026-11-18-stage-3h-sales-returns-design.md`
> 计划：`docs/superpowers/plans/2026-11-18-stage-3h-sales-returns.md`
> 状态：待用户审阅

---

## 1. 完成范围

### 1.1 销售退货闭环

- 新增销售退货单/退货行，只能按原发货行退货，支持部分退货和全部退货。
- 发货行累计 `returned_qty`，累计退货数量不能超过发货数量；超退返回 409 `OVER_RETURN`。
- 发货列表/详情响应增加 `returned_qty` 和 `returnable_qty`。
- 退货即时生效（`posted`），不进入审批流程。

### 1.2 库存回库与成本

- 按原发货行 `unit_cost` 回库，增加 `on_hand` 和 `available`。
- 重算移动平均成本：
  `new_avg = (old_on_hand × old_avg + return_cost) / (old_on_hand + return_qty)`。
- 写入 `inventory_transactions`：`direction=in`、`biz_type=sales_return`、业务单据为退货单。
- 余额行不存在时自动创建，平均成本取退货单位成本。

### 1.3 退货凭证

- 新增凭证类型和来源 `sales_return`。
- 冲减收入、销项税和应收，同时回库存、冲销售成本。
- 凭证日期取退货日期，来源为退货单，摘要“销售退货 <退货单号>”。
- 退货凭证与库存回库、贷项单和发票 credited 更新在同一事务内完成。

### 1.4 贷项单与发票

- 销售退货自动生成 `finance_credit_note` 贷项单（编码规则前缀 `CN`）。
- 自动冲减原应收发票未收金额，`applied = min(贷项金额, outstanding)`。
- 未冲完部分保留为客户贷项，贷项单状态 `open`；全部冲完为 `applied`。
- 发票增加 `credited_amount`、`base_credited_amount`，未收金额扣减贷项。
- 发票状态重算同时考虑已收和已冲贷项；`paid` 前端显示“已结清”。
- 收付款核销读取 credited，避免对已由贷项冲减的金额重复收款。

### 1.5 外币与期间重估

- 退货继承原发货币种、汇率和本位币金额。
- 贷项单原币/本位币金额与退货一致。
- 期间外币重估使用扣减 credited 后的未收金额，已由贷项全额冲减的发票不再重估。
- 发票全额冲减时清空 `base_revalued_amount`。

### 1.6 API、SDK、OpenAPI 和前端

- 新增退货创建/列表/详情和贷项单列表/详情 API。
- 新增权限 `sales.return.read`、`sales.return.write`，admin 角色自动授权。
- SDK 增加 `SalesReturn`、`CreditNote` 类型和退货/贷项单方法。
- OpenAPI 增加路径、Schema、发货行 `returned_qty`/`returnable_qty` 和发票 credited 字段，版本更新为 `0.9.0`。
- 发货页面增加退货操作、退货弹窗和已退/可退数量展示。
- 新增“销售退货”和“贷项单”页面/菜单；发票页面增加已冲贷项和未收金额。
- 前端完整度检查增加退货、贷项单和发票 credited 断言。

### 1.7 测试、CI 与文档

- 新增 Stage 3H 真实 HTTP API 集成测试 `tests/api/stage3h_test.go`。
- 新增 PowerShell 测试脚本 `scripts/test/stage3h-api.ps1` 和 `pnpm test:api:stage3h`。
- CI 增加 Stage 3H 测试步骤。
- 更新根 README、sales/finance/gl 模块 README 和迁移说明。

---

## 2. 数据模型

迁移：`000020_sales_returns`（`db/migrations/000020_sales_returns.{up,down}.sql`）。

### 2.1 发货行

`sales_shipment_lines` 新增：

- `returned_qty numeric(19,6) NOT NULL DEFAULT 0`。
- 检查约束 `returned_qty >= 0 AND returned_qty <= quantity`。
- 为 `(tenant_id, id)` 补充唯一约束，供退货行复合外键引用。

### 2.2 销售退货

`sales_returns`：

- 主键 `id`、`tenant_id`、`org_id`、`code`（租户内唯一）。
- 来源：`shipment_id`/`shipment_code`、`order_id`/`order_code`。
- 客户、仓库、`return_date`、`status`（`posted`）。
- 币种和汇率：`currency`、`exchange_rate`。
- 原币金额：`net_amount`、`tax_amount`、`total_amount`。
- 本位币金额：`base_net_amount`、`base_tax_amount`、`base_total_amount`。
- 成本：`cost_amount`。
- 关联：`credit_note_id`、`voucher_code`。
- 原因、备注、创建人、创建时间。
- 外键：发货单、贷项单；索引：发货单/客户 + 退货日期。

`sales_return_lines`：

- `return_id`、`line_no`、`shipment_line_id`、`order_line_id`、`material_id`。
- `quantity`、`unit_price`、`tax_rate_id`、`tax_rate`。
- 原币金额：`amount`、`tax_amount`、`gross_amount`。
- 本位币金额：`base_amount`、`base_tax_amount`、`base_gross_amount`。
- 成本：`unit_cost`、`cost_amount`。
- 唯一约束 `(tenant_id, return_id, shipment_line_id)`，数量必须大于 0。
- 退货行随退货单级联删除。

### 2.3 贷项单

`finance_credit_notes`：

- `code` 租户内唯一；`credit_type`/`source_type` 本期为 `sales_return`。
- 来源：`source_id`/`source_code`、`invoice_id`、`partner_id`。
- `note_date`、`currency`、`exchange_rate`。
- 原币：`amount`、`applied_amount`、`remaining_amount`。
- 本位币：`base_amount`、`applied_base_amount`、`remaining_base_amount`。
- 状态：`open` / `applied`。
- 检查：金额大于 0，applied/remaining 非负，且 `applied + remaining = amount`（本位币同理）。
- 来源唯一约束 `(tenant_id, source_type, source_id)`，一张退货单只生成一张贷项单。
- 索引：租户/组织/状态/日期、租户/客户/状态。

### 2.4 发票

`finance_invoices` 新增：

- `credited_amount numeric(19,4) NOT NULL DEFAULT 0`。
- `base_credited_amount numeric(19,4) NOT NULL DEFAULT 0`。
- 检查约束 `credited_amount >= 0 AND paid_amount + credited_amount <= total_amount`。
- 未收金额统一为 `total_amount - paid_amount - credited_amount`。

### 2.5 权限与编码规则

- 权限：`sales.return.read`、`sales.return.write`，授予 admin 角色。
- 编码规则：`sales_return`（前缀 `SR`）、`finance_credit_note`（前缀 `CN`），按日重置 4 位流水。
- 迁移 down 删除新增表、列、权限和编码规则；数据修复列的回滚风险按迁移 README 说明处理。

---

## 3. 退货计算、库存、凭证与贷项单冲应收

### 3.1 退货数量校验

- 只允许按原发货行退货，退货行必须存在且不能重复。
- 退货日期不能早于发货日期。
- 累计校验：`shipment_line.returned_qty + return_qty <= shipment_line.quantity`。
- 事务内先 `FOR UPDATE` 锁定发货单、订单和全部发货行，再按行更新 `returned_qty`，更新时再次带数量条件，避免并发超退。
- 发货单必须为 `posted` 状态。

### 3.2 退货金额计算

按原发货行累计数量差额计算，避免多次部分退货产生尾差：

```text
prior_net   = round(shipment_line.amount × returned_qty / quantity, 4)
new_net     = round(shipment_line.amount × (returned_qty + return_qty) / quantity, 4)
net         = new_net - prior_net

gross       = round(gross_amount × (returned_qty + return_qty) / quantity, 4)
            - round(gross_amount × returned_qty / quantity, 4)
tax         = gross - net
```

- 本位币净额/含税额和成本按同样方式计算。
- 全额退货时金额与原发货行完全一致；多次部分退货合计等于原发货金额。
- 行金额和表头金额均保留 4 位小数；成本金额同样按累计差额取数。
- 退货净额、税额、含税额和成本均不能为负。

### 3.3 库存回库

- 按物料稳定排序后逐行锁定 `inventory_balances`，降低多物料并发退货死锁风险。
- 回库更新 `on_hand` 和 `available`，重算平均成本。
- 写库存流水：`direction=in`、`biz_type=sales_return`、`biz_id=退货单 ID`、`biz_code=退货单号`。
- 平均成本按退货成本金额和退货数量加权，保留 6 位小数。

### 3.4 退货凭证

凭证类型/来源：`sales_return`；金额取本位币。分录：

| 方向 | 科目 | 金额 |
|---|---|---|
| 借 | `revenue` 主营业务收入 6001 | 退货净额 |
| 借 | `output_vat` 销项税额 222102 | 退货税额（零则省略） |
| 贷 | `ar` 应收账款 1122 | 退货含税总额 |
| 借 | `inventory` 库存商品 1405 | 退货成本 |
| 贷 | `cogs` 主营业务成本 6401 | 退货成本 |

- 净额/税额/成本为零时省略对应行；全部为零时不生成凭证。
- 凭证与业务在同一事务内过账，失败则整个退货回滚。

### 3.5 贷项单冲应收

1. 在同一事务内锁定原应收发票（来源为原发货单、类型 `receivable`）。
2. 校验发票客户、币种和汇率与退货一致。
3. 计算未收金额：
   `outstanding = total_amount - paid_amount - credited_amount`。
4. 自动冲减：
   `applied = min(退货含税金额, outstanding)`；
   `remaining = 退货含税金额 - applied`。
5. 本位币 applied 按发票汇率计算，并受未收本位币金额和退货本位币金额双重上限约束。
6. 更新发票 `credited_amount`/`base_credited_amount` 和状态。
7. 贷项单 `applied_amount`/`remaining_amount` 与状态同步，并回写退货单 `credit_note_id`。
8. 原发票已全额收款时 `applied = 0`，贷项单全部 `open`，保留为客户贷项。

### 3.6 发票未收金额与状态

```text
outstanding = total_amount - paid_amount - credited_amount
paid        : paid_amount + credited_amount >= total_amount
partially_paid : 0 < paid_amount + credited_amount < total_amount
open        : paid_amount + credited_amount = 0
```

- 付款核销锁定发票时同时读取 `credited_amount`，可用未收金额扣减贷项。
- 付款核销更新时校验 `paid_amount + credited_amount + allocation <= total_amount`。
- 发票列表/详情返回 `credited_amount`、`base_credited_amount` 和扣减贷项后的 `outstanding_amount`。

### 3.7 期间外币重估

- 重估查询条件为 `total_amount > paid_amount + credited_amount`。
- 未核销原币 = `total_amount - paid_amount - credited_amount`。
- 账面本位币 = `base_total_amount - base_paid_amount - base_credited_amount`。
- 期末按最新汇率重估差额；已全额冲减的发票不再生成重估。
- 反关账仍按原逻辑撤销重估和次期反向凭证。

---

## 4. API 清单

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/api/v1/sales/shipments/:id/returns` | `sales.return.write` | 创建销售退货 |
| GET | `/api/v1/sales/returns` | `sales.return.read` | 退货列表 |
| GET | `/api/v1/sales/returns/:id` | `sales.return.read` | 退货详情 |
| GET | `/api/v1/finance/credit-notes` | `finance.invoice.read` | 贷项单列表 |
| GET | `/api/v1/finance/credit-notes/:id` | `finance.invoice.read` | 贷项单详情 |

退货创建请求：

```json
{
  "return_date": "2026-11-18",
  "reason": "质量问题",
  "remark": "",
  "lines": [
    { "shipment_line_id": "shipment-line-uuid", "quantity": "1" }
  ]
}
```

列表参数：

- 退货：`shipment_id`、`customer_id`、`keyword`、`start_date`、`end_date`、`page`、`page_size`。
- 贷项单：`status`（`open`/`applied`）、`partner_id`、`keyword`、`page`、`page_size`。

响应要点：

- 退货单：单号、发货/订单、客户/仓库、日期、币种/汇率、原币/本位币净额/税额/含税额、成本、原发票、贷项单号/金额/已冲/剩余/状态、凭证号、行明细。
- 发货行：`returned_qty`、`returnable_qty`。
- 贷项单：来源退货、原发票、客户、日期、币种/汇率、金额、已冲、剩余、状态。
- 发票：`credited_amount`、`base_credited_amount`、扣减贷项后的 `outstanding_amount`。

错误：

- 超退：409 `OVER_RETURN`。
- 期间已关账：409 `PERIOD_CLOSED`。
- 发货/行/发票不存在：404 `NOT_FOUND`。
- 数量非法、退货行为空、日期非法、UUID 非法：400 `BAD_REQUEST`。
- 未登录 401，缺少权限 403；列表/详情应用组织数据范围。

---

## 5. 测试结果

### 5.1 Stage 3H API 测试

执行：

```bash
pnpm test:api:stage3h
```

结果：PASS，`TestStage3H` 通过，共 7 个子测试：

| 子测试 | 覆盖点 | 结果 |
|---|---|---|
| partial and full return with credit note inventory and voucher | 部分/全部退货金额、成本、库存余额/流水、平均成本、凭证借贷、`returned_qty`/`returnable_qty`、贷项 applied/remaining、发票 credited/outstanding/状态、超退 409 | 通过 |
| return list and detail contract | 退货列表/详情字段、行明细、发货/客户/贷项/发票关联 | 通过 |
| credit note list and detail | 贷项单列表/详情、来源退货、原发票、金额/已冲/剩余/状态 | 通过 |
| rounding balanced partial returns | 三次部分退货的 4 位小数尾差、合计与原发货一致、发票全额冲减 | 通过 |
| fully paid invoice return leaves open credit | 已收款发票退货时 applied=0，贷项单全部 `open`，发票保持已结清 | 通过 |
| permissions and validation | 401/403、非法 UUID、空行、零/负数量、非法日期、404 | 通过 |
| foreign currency return | USD 退货原币/本位币金额、贷项单、发票 base credited、凭证本位币金额 | 通过 |

领域单元测试额外覆盖：

- 部分退货比例计算、全额退货金额一致、零税/零成本、剩余可退数量。
- 多次部分退货的累计金额平衡和尾差。
- 贷项单 remaining、状态常量和舍入。
- 发票 outstanding 和状态重算。

### 5.2 静态检查与构建

本次 Task 7 执行：

| 检查 | 结果 |
|---|---|
| `gofmt -w ./cmd ./internal ./db ./tests` + `gofmt -l` | 通过（无未格式化文件） |
| `go test ./...` | 通过 |
| `go vet ./...` | 通过 |
| `pnpm typecheck:web` | 通过 |
| `pnpm test:frontend` | 通过 |
| `pnpm build:web` | 通过（Vite 提示主 chunk 大于 500 kB，属现有构建警告） |
| `npx -y yaml@2 valid api/openapi.yaml` | 通过 |

### 5.3 干净数据库全链路回归

干净数据库 Stage 3B–3H 全链路回归：**待最终验证**，由控制方执行，本任务未运行。

Stage 3H API 测试会写入本地开发库并消耗业务数据；完整回归必须使用干净数据库。

---

## 6. 代码审核

### 6.1 审核方式

待独立代码审核子代理执行。建议审核范围：

- `f6b667d..a103db3`（Stage 3H 实现、API、UI 和测试）。
- 重点覆盖迁移、退货事务、库存回库、凭证、贷项单、发票 credited、期间重估、API、前端和测试。

### 6.2 审核结论

**待独立审核，本报告不作审核通过结论。** 本节将在 Task 8 审核完成后补充结论、修复记录和最终复审意见。

### 6.3 重点审核项

- [ ] 退货数量并发锁和超退校验是否正确，重复行/非法数量是否拦截。
- [ ] 退货金额累计差额和 4 位舍入是否正确，多次部分退货是否与原发货金额一致。
- [ ] 外币退货的本位币金额、贷项单 applied/remaining 和凭证金额是否一致。
- [ ] 库存回库、平均成本重算和库存流水是否正确；并发退货是否可能死锁或覆盖余额。
- [ ] 退货凭证借贷是否平衡，零金额行处理是否正确。
- [ ] 贷项单自动冲原发票、剩余客户贷项、发票状态和付款核销 outstanding 是否正确。
- [ ] 期间外币重估是否正确扣减 credited，已全额冲减发票是否排除。
- [ ] 迁移 up/down、约束、索引、权限和编码规则是否完整。
- [ ] API 权限、错误码、组织数据范围和响应字段是否符合设计。
- [ ] SDK、OpenAPI、前端页面和完整度断言是否同步。
- [ ] 测试覆盖和干净库 Stage 3B–3H 全链路是否通过。

---

## 7. 已知限制

- 本期不提供采购退货、收付款反核销和 VAT 红字发票/税控红冲。
- 贷项单只在创建退货时自动冲原发票；剩余贷项不能抵其他发票，也不提供退款申请。
- 贷项单本期只读，不提供手工创建、手工核销、反核销或作废接口。
- 退货不改订单已发数量，不自动重开订单；不支持无原发货单的销售退货。
- 库存按原发货成本回库并重算移动平均成本，不恢复原成本层/批次成本。
- 退货即时生效，无审批流、质检、换货和退货物流。
- 退货日期不能早于发货日期，不能退回到已关账期间；退货凭证在业务事务内立即入账。
- 未使用的客户贷项余额不参与期间外币重估。
- 一张退货单只生成一张贷项单，贷项单与原发票强绑定。
- 付款核销只按当前未收金额校验，不自动使用客户未用贷项抵扣新发票。
- 前端退货弹窗按行录入数量，暂不支持整单批量退货或按订单行退货。
- 测试写入本地开发库并消耗业务数据，不得在需要保留数据的数据库上重复执行；完整回归使用干净数据库。
- 自动化测试覆盖主要 API 和领域路径，尚缺浏览器 E2E、并发退货/核销和事务故障注入。

---

## 8. 下一阶段建议

- 采购退货与应付贷项单，复用本期退货/贷项模式。
- 收付款反核销、贷项单抵其他发票、客户退款和贷项核销台账。
- VAT 红字发票/税控红冲，与贷项单和销项税联动。
- 账龄分析、客户信用额度、对账单和逾期提醒。
- 退货审批流、质检、换货、退货物流和退货原因主数据。
- 按批次/成本层回库，支持更精细的存货成本核算。
- 未使用客户贷项的外币重估和汇兑损益处理。
- 退货/贷项单幂等键、事件 Outbox 和导入导出。
- 补充浏览器 E2E、并发退货/核销和故障注入测试，完善干净数据库全量回归。
- OpenAPI 生成 SDK、前端 ESLint/Prettier、报表引擎和合并报表折算。

---

## 9. 用户审阅清单

- [ ] 按原发货行部分/全部退货、累计数量校验和超退 409 是否符合业务预期。
- [ ] 退货金额按原发货行比例、累计差额取数和 4 位舍入口径是否认可。
- [ ] 库存按原成本回库并重算平均成本、不恢复原成本层是否可接受。
- [ ] 退货凭证冲收入/销项税/应收、回库存/冲成本的方向和科目是否正确。
- [ ] 贷项单自动冲原发票、剩余保留客户贷项、已收款发票全部 `open` 是否符合业务规则。
- [ ] 发票 credited/outstanding/状态和“已结清”文案是否满足财务与演示要求。
- [ ] 期间外币重估扣减 credited、未用贷项不重估是否可接受。
- [ ] 退货/贷项单 API、权限、错误码和组织数据范围是否完整。
- [ ] 前端退货弹窗、退货页面、贷项单页面和发票展示是否满足使用。
- [ ] 已知限制和下一阶段排期是否认可。
- [ ] 干净数据库 Stage 3B–3H 全链路回归是否允许由控制方执行。
- [ ] 独立代码审核完成并确认后，是否允许合并/进入下一阶段。
