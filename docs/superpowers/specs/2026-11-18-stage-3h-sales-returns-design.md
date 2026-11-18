# Stage 3H 销售退货与贷项单设计

> 日期：2026-11-18
> 分支：`stage-3h-sales-returns`
> 状态：设计已确认

## 1. 目标

在现有销售发货、库存、应收发票和总账基础上增加销售退货闭环：

```text
销售发货
   ↓
销售退货（按原发货行、部分/全部）
   ↓
库存回库 / 冲销售成本
   ↓
冲收入、销项税、应收
   ↓
贷项单自动冲原发票未收金额
   ↓
剩余贷项保留为客户贷项
```

## 2. 范围

### 本期

- 销售退货单和退货行，按原发货行退货。
- 累计退货数量校验，支持部分/全部退货。
- 按原发货成本回库，重算库存平均成本。
- 自动生成销售退货会计凭证。
- 自动生成销售贷项单，冲减原应收发票未收金额。
- 贷项单超出未收金额部分保留为客户贷项。
- 发票增加已冲贷项金额，未收金额扣减贷项。
- 期间外币重估使用扣减贷项后的未收金额。
- 退货/贷项单 API、SDK、OpenAPI、前端页面和接口测试。

### 后置

- 采购退货。
- 收付款反核销。
- VAT 红字发票/税控红冲。
- 贷项单抵未来发票、退款申请。
- 退货审批流。
- 退货物流、质检、换货。
- 恢复原库存成本层。

## 3. 数据模型

迁移 `000020_sales_returns`。

### 3.1 发货行

`sales_shipment_lines` 增加：

- `returned_qty numeric(19,6) NOT NULL DEFAULT 0`
- 检查 `returned_qty >= 0 AND returned_qty <= quantity`
- 索引 `(tenant_id, shipment_id)` 已有。

### 3.2 销售退货

`sales_returns`：

- `id uuid PRIMARY KEY`
- `tenant_id`, `org_id`
- `code text NOT NULL`，唯一 `(tenant_id, code)`
- `shipment_id`, `shipment_code`
- `order_id`, `order_code`
- `customer_id`
- `warehouse_id`
- `return_date date NOT NULL`
- `status text NOT NULL DEFAULT 'posted'`
- `currency text NOT NULL`
- `exchange_rate numeric(19,8) NOT NULL CHECK > 0`
- `price_includes_tax boolean NOT NULL`
- `net_amount`, `tax_amount`, `total_amount numeric(19,4) NOT NULL DEFAULT 0`
- `base_net_amount`, `base_tax_amount`, `base_total_amount numeric(19,4) NOT NULL DEFAULT 0`
- `cost_amount numeric(19,4) NOT NULL DEFAULT 0`
- `credit_note_id uuid`
- `voucher_code text NOT NULL DEFAULT ''`
- `reason text NOT NULL DEFAULT ''`
- `remark text NOT NULL DEFAULT ''`
- `created_by`, `created_at`
- 外键：`(tenant_id, shipment_id)` -> `sales_shipments`；`(tenant_id, credit_note_id)` -> `finance_credit_notes`（可在创建后更新，或先建贷项单再更新退货单，避免循环外键）。

`sales_return_lines`：

- `id uuid PRIMARY KEY`
- `tenant_id`, `return_id`
- `line_no int NOT NULL`
- `shipment_line_id`, `order_line_id`, `material_id`
- `quantity numeric(19,6) NOT NULL CHECK > 0`
- `unit_price numeric(19,6) NOT NULL`
- `tax_rate_id uuid`
- `tax_rate numeric(9,6) NOT NULL DEFAULT 0`
- `amount`, `tax_amount`, `gross_amount numeric(19,4) NOT NULL DEFAULT 0`
- `base_amount`, `base_tax_amount`, `base_gross_amount numeric(19,4) NOT NULL DEFAULT 0`
- `unit_cost numeric(19,6) NOT NULL DEFAULT 0`
- `cost_amount numeric(19,4) NOT NULL DEFAULT 0`
- 外键：`(tenant_id, return_id)` -> `sales_returns` ON DELETE CASCADE；`(tenant_id, shipment_line_id)` -> `sales_shipment_lines`。
- 唯一 `(tenant_id, return_id, shipment_line_id)`。

### 3.3 贷项单

`finance_credit_notes`：

- `id uuid PRIMARY KEY`
- `tenant_id`, `org_id`
- `code text NOT NULL`，唯一 `(tenant_id, code)`
- `credit_type text NOT NULL DEFAULT 'sales_return'`
- `source_type text NOT NULL`（本期 `sales_return`）
- `source_id`, `source_code`
- `invoice_id` 原应收发票
- `partner_id`
- `note_date date NOT NULL`
- `currency text NOT NULL`
- `exchange_rate numeric(19,8) NOT NULL CHECK > 0`
- `amount numeric(19,4) NOT NULL CHECK > 0`
- `base_amount numeric(19,4) NOT NULL CHECK > 0`
- `applied_amount numeric(19,4) NOT NULL DEFAULT 0`
- `applied_base_amount numeric(19,4) NOT NULL DEFAULT 0`
- `remaining_amount numeric(19,4) NOT NULL`
- `remaining_base_amount numeric(19,4) NOT NULL`
- `status text NOT NULL`（`open` 或 `applied`）
- `reason`, `remark`
- `created_by`, `created_at`, `updated_at`
- 检查：`applied_amount >= 0`、`remaining_amount >= 0`、`applied_amount + remaining_amount = amount`，本位币同理。
- 索引：`(tenant_id, partner_id, status, note_date DESC)`、`(tenant_id, invoice_id)`、`(tenant_id, source_type, source_id)` 唯一。

### 3.4 发票

`finance_invoices` 增加：

- `credited_amount numeric(19,4) NOT NULL DEFAULT 0`
- `base_credited_amount numeric(19,4) NOT NULL DEFAULT 0`
- 检查：`credited_amount >= 0 AND paid_amount + credited_amount <= total_amount`
- 未收金额 = `total_amount - paid_amount - credited_amount`
- 状态：
  - `open`：未收 = total，paid/credited = 0
  - `partially_paid`：0 < paid+credited < total
  - `paid`：paid+credited >= total（前端文案“已结清”）
- 现有现金付款状态逻辑保持，付款核销时同时考虑 credited_amount。

## 4. 退货计算

- 只能按原发货行退货。
- 累计退货数量不能超过发货数量：
  `returned_qty + return_qty <= shipment_line.quantity`
- 行金额按原发货行比例：
  ```text
  ratio = return_qty / shipment_line.quantity
  net = shipment_line.amount × ratio
  tax = shipment_line.tax_amount × ratio
  gross = shipment_line.gross_amount × ratio
  base_net = shipment_line.base_amount × ratio
  base_tax = shipment_line.base_tax_amount × ratio
  base_gross = shipment_line.base_gross_amount × ratio
  cost = shipment_line.cost_amount × ratio
  ```
- 每行四舍五入 4 位；全额退货时 ratio=1，与原发货金额完全一致。
- 表头金额 = 各行金额合计。
- 退货单位成本 = 原发货行 `unit_cost`；成本金额按比例。
- 原发货行 `returned_qty += return_qty`。

## 5. 库存回库

- 锁定 `inventory_balances` 行（tenant_id, warehouse_id, material_id）。
- 回库：
  - `on_hand += return_qty`
  - `available += return_qty`
  - 平均成本：
    ```text
    new_avg = (old_on_hand × old_avg + return_cost) / (old_on_hand + return_qty)
    ```
- 插入 `inventory_transactions`：
  - direction `in`
  - biz_type `sales_return`
  - biz_id 退货单 ID，biz_code 退货单号
  - quantity、unit_cost、amount=return_cost
- 若余额行不存在，创建余额行，平均成本取退货单位成本。

## 6. 会计凭证

新增凭证类型和来源 `sales_return`。

凭证行（本位币）：

- 借：主营业务收入（`revenue`）——退货净额
- 借：销项税额（`output_vat`）——退货税额（零则省略）
- 贷：应收账款（`ar`）——退货含税额
- 借：库存商品（`inventory`）——退货成本
- 贷：主营业务成本（`cogs`）——退货成本

凭证日期 = 退货日期，来源 = 退货单，来源号 = 退货单号，摘要“销售退货”。

- 若净额/税额/成本为零，相应行省略。
- 退货凭证与发票/贷项单在同一事务内生成。

## 7. 贷项单冲应收

- 创建退货时，锁定原应收发票。
- 原发票未收金额：
  ```text
  outstanding = total_amount - paid_amount - credited_amount
  ```
- 贷项单自动冲减：
  ```text
  applied = min(credit_note.amount, outstanding)
  remaining = credit_note.amount - applied
  ```
- 更新发票：
  - `credited_amount += applied`
  - `base_credited_amount += applied_base`
  - 状态重算：`paid` / `partially_paid` / `open`
- 贷项单：
  - `applied_amount = applied`
  - `remaining_amount = remaining`
  - 状态 `applied`（remaining=0）或 `open`（remaining>0）
- 原发票已全额收款时，applied=0，贷项单全部 open。
- 本期不提供贷项单抵其他发票或退款接口，剩余贷项在退货详情和贷项单列表展示。

## 8. 外币与期间重估

- 退货使用原发货汇率，继承原发货本位币金额。
- 贷项单金额/本位币金额与退货一致。
- 期间外币重估查询：
  - 未核销原币 = `total_amount - paid_amount - credited_amount`
  - 账面本位币 = `base_total_amount - base_paid_amount - base_credited_amount + base_revalued_amount`
  - 已全额退货/贷项冲减的发票不再重估。
- 未使用的客户贷项余额不参与本期发票重估；作为已知限制记录。

## 9. API

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/api/v1/sales/shipments/:id/returns` | `sales.return.write` | 创建销售退货 |
| GET | `/api/v1/sales/returns` | `sales.return.read` | 退货列表 |
| GET | `/api/v1/sales/returns/:id` | `sales.return.read` | 退货详情 |
| GET | `/api/v1/finance/credit-notes` | `finance.invoice.read` | 贷项单列表 |
| GET | `/api/v1/finance/credit-notes/:id` | `finance.invoice.read` | 贷项单详情 |

- 发货列表/详情响应增加 `returned_qty`、`returnable_qty`。
- 退货创建请求：
  ```json
  {
    "return_date": "2026-11-18",
    "reason": "质量问题",
    "remark": "",
    "lines": [
      { "shipment_line_id": "uuid", "quantity": "1.0000" }
    ]
  }
  ```
- 退货详情返回：
  - 退货单头、行。
  - 原发货/订单/客户/仓库。
  - 贷项单号、贷项金额、已冲金额、剩余金额、状态。
  - 凭证号。
- 发票响应增加 `credited_amount`、`base_credited_amount`，`outstanding_amount` 扣减贷项。
- 错误：
  - 超退 409 `OVER_RETURN`
  - 发货/行/发票不存在 404
  - 数量非法/退货行为空 400
  - 期间已关账 409 `PERIOD_CLOSED`
  - 权限 401/403

## 10. 前端

- 销售发货列表增加“退货”操作；退货弹窗：
  - 退货日期、原因、备注。
  - 可退数量、原单价、税率、原成本。
  - 输入退货数量，自动预览退货净额/税额/含税额/成本。
- 新增“销售退货”页面/菜单：
  - 列表：退货单号、原发货单、客户、日期、金额、成本、贷项状态。
  - 详情：行、贷项单已冲/剩余、凭证号、原发票。
- 发货详情/列表显示已退/可退数量。
- 发票列表/详情：
  - 已冲贷项金额。
  - 未收金额扣减贷项。
  - `paid` 状态文案改为“已结清”。
- 新增“贷项单”只读页面/菜单：
  - 列表/详情，来源退货、原发票、金额、已冲、剩余、状态。
- 权限、路由、菜单、SDK、OpenAPI 同步。

## 11. 测试

### 单元测试

- 退货行比例计算：部分退货、全额退货、零税、零成本。
- 超退校验。
- 库存平均成本重算。
- 贷项单 applied/remaining 计算。
- 发票未收金额和状态重算。

### API 测试

- 创建部分退货：金额、成本、库存余额/流水、凭证借贷、发货 returned_qty、贷项单 applied/remaining、发票 credited/outstanding。
- 全额退货：金额与原发货一致，发票结清。
- 超退 409。
- 已收款发票退货：贷项单全部 open。
- 多行退货、外币发货退货本位币金额。
- 贷项单列表/详情。
- 发货已退/可退数量。
- 期间重估扣减贷项。
- 权限 401/403、数据范围、非法数量/日期。
- Stage 3B–3H 回归和干净库全链路。

## 12. 验收标准

- [ ] 销售发货可部分/全部退货。
- [ ] 库存按原成本回库并重算平均成本。
- [ ] 退货凭证正确冲收入/销项税/应收并回库存/冲成本。
- [ ] 贷项单自动冲原发票未收金额，剩余保留为客户贷项。
- [ ] 发票已冲/未收金额和状态正确。
- [ ] 期间外币重估排除已冲贷项。
- [ ] 前端退货、贷项单、发票展示完整。
- [ ] SDK/OpenAPI/迁移/权限完整。
- [ ] 自动化测试、静态检查、构建和回归通过。
- [ ] 代码审核和汇总报告完成。

## 13. 已知限制

- 本期不做采购退货、收付款反核销、VAT 红字发票。
- 贷项单只自动冲原发票，剩余贷项不能抵其他发票或退款。
- 退货不改订单已发数量，不自动重开订单。
- 库存按原发货成本回库并重算平均成本，不恢复原成本层。
- 退货即时生效，无审批流、质检和换货。
- 退货数量以原发货行为准，不支持无原发货单的销售退货。
- 未使用的客户贷项余额不参与外币重估。
