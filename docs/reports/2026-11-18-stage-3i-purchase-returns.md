# Stage 3I 汇总报告：采购退货与应付贷项单

> 日期：2026-11-18
> 分支：`stage-3i-purchase-returns`
> 提交：`0723c05`（设计/计划）+ `20476a0..`（实现、测试与审核修复）
> 设计：`docs/superpowers/specs/2026-11-18-stage-3i-purchase-returns-design.md`
> 计划：`docs/superpowers/plans/2026-11-18-stage-3i-purchase-returns.md`
> 状态：待用户审阅

---

## 1. 完成范围

### 1.1 采购退货闭环

- 新增采购退货单/退货行，只能按原采购收货行退货，支持部分退货和全部退货。
- 收货行累计 `returned_qty`，累计退货数量不能超过收货数量；超退返回 409 `OVER_RETURN`。
- 采购收货列表/详情响应增加 `id`、`returned_qty` 和 `returnable_qty`。
- 退货即时生效（`posted`），不进入审批流程；退货日期不能早于收货日期。
- 退货金额按原收货行累计数量差分计算，多次部分退货无尾差。

### 1.2 库存出库与成本

- 按原采购收货行单位成本出库，减少 `on_hand` 和 `available`。
- 单位成本取原收货行本位币净额/数量（`base_amount / quantity`，6 位小数）。
- 重算移动平均成本：
  `new_avg = (old_on_hand × old_avg - return_cost) / new_on_hand`。
- `new_on_hand = 0` 时平均成本置 0；剩余库存价值小于退货成本时平均成本按 0 处理，并作为简化口径记录。
- 库存不足返回 409 `INSUFFICIENT_STOCK`。
- 写入 `inventory_transactions`：`direction=out`、`biz_type=purchase_return`、业务单据为退货单。
- 余额行不存在视为库存不足，不自动创建负库存。

### 1.3 采购退货凭证

- 新增凭证类型和来源 `purchase_return`。
- 借应付账款（本位币含税）、贷库存商品（本位币净额）、贷进项税额（本位币税额）。
- 税额为 0 时省略进项税行；净额/税额/含税全为 0 时不生成凭证。
- 凭证日期取退货日期，摘要“采购退货 <退货单号>”。
- 退货凭证与库存出库、贷项单和发票 credited 更新在同一事务内完成；期间关账返回 409 `PERIOD_CLOSED`。

### 1.4 应付贷项单与发票

- 采购退货自动生成 `finance_credit_note` 贷项单（编码规则前缀 `CN`），`credit_type='purchase_return'`、`source_type='purchase_return'`。
- 贷项单自动冲减原应付发票未付金额：
  `outstanding = total - paid - credited`，
  `applied = min(return_total, outstanding)`，`remaining = return_total - applied`。
- 贷项单状态 `applied`（原币和本位币都已全部冲减）或 `open`（仍有供应商贷项）。
- 原应付发票 `credited_amount` 和 `base_credited_amount` 同步累加；全额结清时清空 `base_revalued_amount`。
- 原发票已全额付款时 `applied=0`，贷项单保持 `open`，发票保持已结清。
- 零金额退货跳过贷项单，退货单 `credit_note_id` 为空。
- 未使用的供应商贷项本期不能抵其他发票、不能退款。

### 1.5 贷项单类型筛选

- `finance_credit_notes` 继续作为销售贷项和采购贷项的统一台账。
- 贷项单列表增加 `credit_type` 参数，支持 `sales_return`/`purchase_return`，非法值返回 400。
- 贷项单响应增加/返回 `credit_type`，前端增加类型列、筛选和详情展示。

### 1.6 API、SDK、OpenAPI 与前端

- 新增采购退货 API：
  - `POST /api/v1/purchase/receipts/:id/returns`
  - `GET /api/v1/purchase/returns`
  - `GET /api/v1/purchase/returns/:id`
- 采购收货行增加 `id`、`returned_qty`、`returnable_qty`。
- SDK 增加 `PurchaseReturn`、`PurchaseReturnLine`、`CreatePurchaseReturnInput` 和 `purchaseApi.returns`。
- OpenAPI 升级到 `1.0.0`，增加采购退货路径/模型、收货行可退字段、贷项单类型参数/字段。
- 前端新增“采购收货”页（收货列表、详情、退货入口）和“采购退货”页（列表、详情、贷项单/原发票/凭证）。
- 采购退货弹窗展示可退数量、原单价、税率、原收货单位成本，并按累计差额预览净额/税额/含税/成本。
- 贷项单页面增加类型列/筛选；应付发票继续展示已冲贷项和未付金额。
- 路由、菜单、权限和前端完整度断言同步。

### 1.7 测试与文档

- 新增 `tests/api/stage3i_test.go`、`scripts/test/stage3i-api.ps1` 和 `test:api:stage3i`。
- 覆盖部分/全额退货、累计舍入、应付贷项单、已付款发票、零金额、库存不足、超退、外币和期间重估、权限与校验、贷项类型筛选。
- 更新根 README、purchase/finance/gl 模块 README、迁移 README。
- 本报告记录测试、审核和已知限制。

## 2. 数据模型

### 2.1 迁移

新增 `db/migrations/000021_purchase_returns`：

- `purchase_receipt_lines.returned_qty numeric(19,6) NOT NULL DEFAULT 0`，检查 `0 <= returned_qty <= quantity`。
- `purchase_receipt_lines (tenant_id, id)` 唯一约束，供退货行复合外键引用。
- `purchase_returns`：单头字段与销售退货对称，状态固定 `posted`。
- `purchase_return_lines`：原收货行、订单行、物料、数量、单价、税率、原币/本位币金额、单位成本、成本金额。
- 索引：收货单/供应商/退货日期、退货行明细。
- 权限 `purchase.return.read/write`，admin 授权。
- 编码规则 `purchase_return`，前缀 `PR`。
- 复用 `finance_credit_notes` 和 `finance_invoices.credited_amount/base_credited_amount`，不新增贷项单表。

### 2.2 采购退货单

`purchase_returns` 主要字段：

- 租户、组织、单号、原收货单/单号、原采购订单/单号、供应商、仓库。
- 退货日期、状态、币种、汇率、含税口径。
- 净额、税额、含税总额、本位币净额/税额/含税、退货成本。
- 贷项单 ID、凭证号、原因、备注、创建人/时间。

`purchase_return_lines` 主要字段：

- 原收货行、订单行、物料、行号、数量、单价、税率。
- 净额、税额、含税金额、本位币金额、单位成本、成本金额。

### 2.3 贷项单与发票

- 贷项单复用 `finance_credit_notes`：
  - `credit_type = 'purchase_return'`
  - `source_type = 'purchase_return'`
  - `source_id` = 采购退货单
  - `invoice_id` = 原应付发票
  - `partner_id` = 供应商
- 发票继续使用：
  - `credited_amount`
  - `base_credited_amount`
  - `outstanding_amount = total_amount - paid_amount - credited_amount`
- 贷项单字段：原币/本位币金额、已冲原币/本位币、剩余原币/本位币、状态、原因/备注。

### 2.4 权限与编码

- `purchase.return.read`：查看采购退货。
- `purchase.return.write`：创建采购退货。
- 贷项单继续使用 `finance.invoice.read`。
- 编码规则：`purchase_return` 前缀 `PR`，贷项单沿用 `finance_credit_note` 前缀 `CN`。

## 3. 退货计算、库存、凭证与应付

### 3.1 累计数量与金额

只能按原收货行退货，累计退货数量不能超过收货数量。金额按累计数量差分：

```text
prior_net = round(receipt_line.amount × returned_qty / quantity, 4)
new_net   = round(receipt_line.amount × (returned_qty + return_qty) / quantity, 4)
net       = new_net - prior_net

prior_gross = round(receipt_line.gross_amount × returned_qty / quantity, 4)
new_gross   = round(receipt_line.gross_amount × (returned_qty + return_qty) / quantity, 4)
gross       = new_gross - prior_gross
tax         = gross - net
```

- 本位币金额、成本同理。
- 全额退货金额与原收货行一致；多次部分退货合计无尾差。
- 税额由含税金额减净额得到，保证 `net + tax = gross` 和凭证平衡。

### 3.2 退货行与成本

- 单位成本：`unit_cost = receipt_line.base_amount / quantity`，保留 6 位小数。
- 退货成本：按累计数量差分计算本位币净额，避免多次部分退货尾差。
- 退货行保存原收货单价、税率、净额/税额/含税、本位币金额、单位成本和成本金额。

### 3.3 库存出库

- 按物料/收货行稳定排序后逐行锁定库存余额，降低并发死锁风险。
- 校验 `on_hand >= return_qty` 且 `available >= return_qty`，否则 409 `INSUFFICIENT_STOCK`。
- 更新余额并重算平均成本；平均成本为负时按 0 处理。
- 写入库存流水 `out / purchase_return`，业务单据为退货单。
- 库存流水 `occurred_at` 使用实际过账时间，凭证使用退货日期；两者可能不同。

### 3.4 退货凭证

| 借方 | 贷方 |
|---|---|
| 应付账款 2202（本位币含税） | 库存商品 1405（本位币净额） |
|  | 进项税额 222101（本位币税额，零则省略） |

- 凭证类型/来源 `purchase_return`，日期为退货日期，来源为退货单。
- 净额/税额为零时省略对应行；全部为零时不生成凭证。
- 与库存出库、贷项单和发票更新在同一事务内完成。

### 3.5 应付贷项单

- 锁定原应付发票（`invoice_type='payable'`、`source_type='purchase_receipt'`、`source_id=receipt_id`）。
- 校验供应商、币种和汇率与退货单一致。
- 未付金额：

```text
outstanding      = total_amount - paid_amount - credited_amount
base_outstanding = base_total_amount - base_paid_amount - base_credited_amount
```

- 冲减：

```text
applied      = min(return_total, outstanding)
applied_base = min(return_base, base_outstanding)
remaining    = return_total - applied
```

- 全额应用时 `applied_base` 直接取退货本位币金额，避免外币多行尾差。
- 更新发票 credited/base credited、状态和 `base_revalued_amount`，SQL 守卫保证
  `paid + credited + applied <= total`。
- 已付款发票 `applied=0`，贷项单保持 `open`，不修改发票。
- 零金额退货跳过贷项单，`credit_note_id` 保持为空。

### 3.6 发票未付与状态

- 未付金额：`total - paid - credited`。
- 状态：
  - `open`：`paid + credited = 0`。
  - `partially_paid`：`0 < paid + credited < total`。
  - `paid`：`paid + credited >= total`。
- 付款核销同样扣减 `credited_amount`，不能超过未付金额。
- 期间外币重估的未核销原币扣减 `credited_amount`，账面本位币扣减 `base_credited_amount`；已由贷项全额冲减的发票不再重估。

### 3.7 期间重估

- 外币采购退货减少原应付发票未付原币和本位币。
- 期末关账重估时只按剩余未付金额计算，不按退货前全额重估。
- 示例：USD 发票 200、汇率 7.2、退货 100、期末汇率 7.3：
  - 退货后 outstanding 100，base outstanding 720。
  - 重估金额 `100 × 7.3 - 720 = 10`。
  - 应付贷方增加 10，汇兑损失借方 10。

## 4. API 清单

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/api/v1/purchase/receipts/:id/returns` | `purchase.return.write` | 创建采购退货 |
| GET | `/api/v1/purchase/returns` | `purchase.return.read` | 退货列表 |
| GET | `/api/v1/purchase/returns/:id` | `purchase.return.read` | 退货详情 |
| GET | `/api/v1/purchase/receipts` | `purchase.receipt.read` | 收货列表，含可退数量 |
| GET | `/api/v1/finance/credit-notes` | `finance.invoice.read` | 贷项单列表，支持 `credit_type` |
| GET | `/api/v1/finance/credit-notes/:id` | `finance.invoice.read` | 贷项单详情 |

创建退货请求：

```json
{
  "return_date": "2026-11-18",
  "reason": "质量问题",
  "remark": "",
  "lines": [
    {
      "receipt_line_id": "receipt-line-uuid",
      "quantity": "1"
    }
  ]
}
```

列表过滤：

- `receipt_id`
- `supplier_id`
- `keyword`
- `start_date` / `end_date`
- `page` / `page_size`

错误码：

- 400 `BAD_REQUEST`：非法 UUID、空行、零/负数量、非法日期、退货日期早于收货日期。
- 401 `UNAUTHORIZED`：未登录。
- 403 `FORBIDDEN`：无采购退货权限。
- 404 `NOT_FOUND`：退货单、原收货行不存在，或收货行不属于该收货单。
- 409 `OVER_RETURN`：累计退货数量超过收货数量。
- 409 `INSUFFICIENT_STOCK`：库存不足。
- 409 `PERIOD_CLOSED`：退货日期所在期间已关账。

## 5. 测试结果

### 5.1 Stage 3I API 测试

```bash
pnpm test:api:stage3i
```

结果：PASS，`TestStage3I` 通过，共 13 个子测试，测试耗时约 3.9 秒：

| 子测试 | 覆盖点 | 结果 |
|---|---|---|
| partial and full purchase return with credit note inventory and voucher | 部分/全额退货金额、成本、库存出库、平均成本、库存流水、凭证借贷、收货行已退/可退、贷项 applied/remaining、发票 credited/outstanding/状态、超退 409 | 通过 |
| return list and detail contract | 退货列表/详情字段、行明细、收货/订单/供应商/仓库/贷项/发票/凭证关联、过滤器和 404/400 | 通过 |
| credit note type filter | `credit_type=purchase_return/sales_return` 过滤、非法类型 400、详情类型/来源 | 通过 |
| rounding balanced partial returns | 三次部分退货累计差额 33.3333/33.3334/33.3333、合计 100.0000、贷项和发票全额冲减 | 通过 |
| fully paid payable invoice return leaves open credit | 已付款发票退货 applied=0、贷项 open、发票 credited 保持 0、状态 paid | 通过 |
| partial credit note application | 发票已付 150 后退货 100：applied=50、remaining=50、贷项 open；发票 credited=50、outstanding=0、状态 paid；退货凭证仍按 100 过账 | 通过 |
| payment allocation respects credited amount | 部分贷项后付款核销扣减 credited、超分配拒绝、最终 paid | 通过 |
| zero value purchase return | 零价退货 total/cost 为 0、无贷项单、无凭证、库存仍出库 | 通过 |
| over-return insufficient stock and validation | 超退 409、库存不足 409 且 `returned_qty`/库存/退货单/贷项/凭证全部回滚、非法 UUID/空行/零负数量/非法日期、跨收货单行 404、非法收货行 UUID 400 | 通过 |
| period closed rolls back purchase return | 关账期间退货 409 `PERIOD_CLOSED`，库存/DTO/退货单回滚，测试后重开期间 | 通过 |
| concurrent returns cannot double return | 两个并发退货请求仅一个成功、另一个 409 `OVER_RETURN`，`returned_qty=1`，只生成一张退货单 | 通过 |
| permissions and validation | 匿名 401、demo_dept 写权限 403、读权限/数据范围、未知退货 404 | 通过 |
| foreign currency purchase return and period revaluation | USD 部分退货、贷项本位币、期间重估只按剩余 100×7.3 调整 +10 | 通过 |

测试脚本会执行迁移、seed、demo、构建 API 并在 18080 端口启动，会写入本地开发库；完整回归使用干净数据库。

### 5.2 静态检查与构建

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
- `go test ./...` 全部包通过，包括 `purchaseReturnPostingLines` 的正税额、负税额和零金额单元测试，以及 `applyPurchaseReturnCredit` 的全额/部分/已付款/外币/尾差不变量测试。
- `go vet ./...` 通过。
- OpenAPI YAML 校验通过。
- `pnpm typecheck:web` 通过。
- `pnpm test:frontend` 通过（前端完整度断言）。
- `pnpm build:web` 通过，仅有 Vite chunk 体积提示，无构建错误。
- `git diff --check` 无空白错误。

### 5.3 干净数据库全链路回归

在全新数据库 `mlf_erp_stage3i_clean` 上执行迁移、seed、demo 和：

```bash
go test -tags=integration ./tests/api -run 'TestStage3' -count=1 -v
```

结果：

- `TestStage3B` 11 个子测试通过。
- `TestStage3C` 9 个子测试通过。
- `TestStage3D` 4 个子测试通过。
- `TestStage3E` 6 个子测试通过。
- `TestStage3F` 7 个子测试通过。
- `TestStage3G` 通过。
- `TestStage3H` 10 个子测试通过。
- `TestStage3I` 13 个子测试通过。
- 总耗时约 15.2 秒。

数据校验：

- `schema_migrations` version 21，dirty=false。
- `purchase_returns` 11 条。
- `purchase_return` 贷项单 10 条（1 条零金额退货无贷项单）。
- `purchase_return` 凭证 10 张。
- 借贷不平衡的采购退货凭证 0 张。

Stage 3I API 测试会写入本地开发库并消耗业务数据；完整回归必须使用干净数据库。

## 6. 代码审核

### 6.1 审核方式

独立代码审核子代理，审查范围 `ba5a3dd..4e32de3`，覆盖迁移、退货域计算、库存出库、退货凭证、应付贷项单、发票 credited、付款核销、期间重估、API、SDK、OpenAPI、前端和测试；并复核修复提交 `4e32de3`。

### 6.2 审核结论

初次审核结论：**Ready to merge: With fixes**。

- 审核确认主流程、迁移保护、库存成本、贷项单应用、付款净额和期间重估正确，未发现 Stage 3H/3F 回归。
- 无 Critical。
- **Important 1**：库存不足测试只断言 409，未验证事务回滚；缺少 `PERIOD_CLOSED` 和并发双退测试。
- **Important 2**：贷项单只测试了全额应用和 applied=0，未测试 `applied < 退货金额` 的部分应用分支。
- **Important 3**：负进项税时 `PostPurchaseReturn` 省略税额行，可能导致凭证不平衡。
- 其余为多行测试、测试状态依赖、`occurred_at` 和幂等键等非阻塞问题。

第一次修复后的复审又发现一个 Critical：尾差容差将 `remaining_base_amount` 归零但未同步调整 `applied_base_amount`，会违反贷项单 `applied_base_amount + remaining_base_amount = base_amount` 约束。已改为保留存储金额、仅用容差决定状态，并抽取 `applyPurchaseReturnCredit` 补回归测试。

最终复审结论：**Ready to merge: Yes**。新 Critical 已解决，无新增 Critical/Important，剩余为已记录的非阻塞覆盖和边界问题。

### 6.3 已修复问题

- 抽取 `purchaseReturnPostingLines`，负税额时借记 `input_vat`，AP/库存/税额均支持正负方向，保证 `gross = net + tax` 凭证平衡；新增正税额、负税额和零金额单元测试。
- 凭证号清理条件与 GL 过账条件统一为三个本位币字段全零，过账使用 `ret.VoucherCode`，避免有凭证号无凭证或空凭证号过账。
- 新增部分贷项应用测试：发票已付 150、退货 100 时 applied=50、remaining=50、贷项 open、发票 credited=50 且仍为 paid。
- 库存不足测试增加回滚断言：`returned_qty`、库存余额、退货单、贷项单和凭证均不变。
- 新增关账期间退货测试：409 `PERIOD_CLOSED` 后库存、DTO 和退货单全部回滚，并重开期间。
- 新增并发双退测试：两个并发请求仅一个成功，另一个 409 `OVER_RETURN`，`returned_qty=1` 且只生成一张退货单。
- 收货行 UUID 非法时返回 400 `BAD_REQUEST`，有效但不存在仍返回 404。
- 修正迁移 README 中 000021 down 的描述。
- 抽取 `applyPurchaseReturnCredit`，全额应用时保留 0.0001 本位币尾差并仅用容差决定状态，保证 `applied_base_amount + remaining_base_amount = base_amount` 约束；新增全额/部分/已付款/外币/尾差不变量单元测试。

### 6.4 剩余问题/已知限制

- API 测试以单行收货为主，多行/多税率的表头汇总主要靠域测试和辅助函数测试覆盖。
- 测试使用共享开发库和未来年份，反复运行会累积数据；完整回归使用干净数据库。
- 关账测试已断言退货单/库存回滚，但未单独断言贷项单和凭证数量不变。
- 并发测试证明收货行锁串行化，但条件式 `returned_qty` UPDATE 守卫通常被域层超退校验提前拦截。
- 负税额凭证在 posting helper 层有单元测试，尚未通过完整 API 场景触发。
- 0.0001 本位币尾差可能保留为 `remaining_base_amount` 且状态为 `applied`；这是有意保留的显示/语义边界，不再违反数据库约束。
- 库存流水 `occurred_at` 使用实际过账时间，与凭证日期 `return_date` 可能不同。
- 已全额贷项结清的外币发票在后续反关账时，`base_revalued_amount` 可能显示负值。
- 采购退货接口暂无幂等键。
- 缺少浏览器 E2E 和故障注入测试。

## 7. 已知限制

- 不做采购退货审批、质检、换货和供应商退款。
- 供应商贷项只冲原应付发票，剩余不能抵其他发票或退款。
- 未使用的供应商贷项不参与外币重估。
- 采购退货按原收货成本出库；剩余库存价值不足时平均成本按 0 处理，不恢复原成本层。
- 不自动减少采购订单已收数量/重开订单。
- 退货即时生效，不进入审批流。
- 库存流水 `occurred_at` 使用实际过账时间，与凭证日期 `return_date` 可能不同。
- 零金额退货不生成贷项单和凭证；库存出库仍执行。
- 全额应用时可能保留 0.0001 本位币尾差，状态按容差标记 `applied`；原币金额和发票结清不受影响。
- 已全额贷项结清的外币发票在后续反关账时，`base_revalued_amount` 可能显示负值；暂为展示字段问题。
- 采购退货接口暂无幂等键，客户端重试可能重复退货。
- 缺少并发退货、故障注入、down/up 数据测试和浏览器 E2E。

## 8. 下一阶段建议

- 采购退货审批、质检、换货和供应商退款。
- 供应商贷项抵其他应付发票、退款申请和贷项台账。
- 收付款反核销。
- VAT 红字进项发票/税控红冲。
- 退货接口幂等键。
- 成本层恢复和更精细的存货计价。

## 9. 用户验收清单

- [ ] 采购收货支持部分/全部退货。
- [ ] 收货行显示已退/可退数量。
- [ ] 库存按原收货成本出库并重算平均成本。
- [ ] 退货凭证正确冲应付、进项税和存货。
- [ ] 应付贷项单自动冲原发票未付金额，剩余保留供应商贷项。
- [ ] 已付款发票退货保留 open 贷项，发票保持已结清。
- [ ] 发票 credited/outstanding/状态正确。
- [ ] 期间外币重估扣减贷项。
- [ ] 贷项单页面可区分销售/采购类型。
- [ ] Stage 3I API 测试通过。
- [ ] Stage 3B–3I 干净库全链路通过。
- [ ] 独立代码审核无 Critical/Important 遗留。
