# Stage 3I 采购退货与应付贷项单设计

> 日期：2026-11-18
> 分支：`stage-3i-purchase-returns`
> 状态：设计已确认

## 1. 目标

在采购收货、库存、应付发票和总账基础上增加采购退货闭环：

```text
采购收货
   ↓
采购退货（按原收货行、部分/全部）
   ↓
库存出库 / 冲存货成本
   ↓
冲应付、进项税、存货
   ↓
应付贷项单自动冲原应付发票未付金额
   ↓
剩余供应商贷项保留
```

## 2. 范围

### 本期

- 采购退货单/行，按原采购收货行退货。
- 累计退货数量校验，支持部分/全部退货。
- 按原收货成本出库，重算库存平均成本。
- 采购退货会计凭证。
- 复用 `finance_credit_notes` 生成应付贷项单，冲原应付发票未付金额。
- 超出未付金额部分保留为供应商贷项。
- 发票 credited/outstanding 和期间外币重估继续复用现有逻辑。
- 采购退货/贷项单 API、SDK、OpenAPI、前端和接口测试。

### 后置

- 采购退货审批、质检、换货。
- 供应商退款、贷项抵未来发票。
- VAT 红字进项发票/税控红冲。
- 采购退货物流。
- 恢复原库存成本层。

## 3. 数据模型

迁移 `000021_purchase_returns`：

- `purchase_receipt_lines.returned_qty numeric(19,6) default 0` + 检查 `0 <= returned_qty <= quantity` + `(tenant_id,id)` 唯一约束。
- `purchase_returns`：单头字段与销售退货对称（单号、收货单/订单、供应商、仓库、日期、币种/汇率、金额、本位币、成本、贷项单、凭证号、原因、备注、状态 `posted`、创建人/时间）。
- `purchase_return_lines`：原收货行、物料、数量、单价、税率、金额、本位币金额、单位成本、成本金额。
- 复用 `finance_credit_notes`：`credit_type='purchase_return'`、`source_type='purchase_return'`、`invoice_id` 原应付发票、`partner_id` 供应商。
- 复用 `finance_invoices.credited_amount`/`base_credited_amount`。
- 权限 `purchase.return.read/write`；编码规则 `purchase_return` 前缀 `PR`。

## 4. 退货计算

- 只能按原收货行退货，累计退货数量不能超过收货数量。
- 退货日期不能早于收货日期。
- 金额按累计数量差分：

```text
prior_net = round(receipt_line.amount * returned_qty / quantity, 4)
new_net   = round(receipt_line.amount * (returned_qty + return_qty) / quantity, 4)
net       = new_net - prior_net
gross     = round(gross_amount * new_qty / quantity, 4) - round(gross_amount * prior_qty / quantity, 4)
tax       = gross - net
```

- 本位币金额和成本同理。
- 全额退货金额与原收货行一致；多次部分退货合计无尾差。

## 5. 库存出库

- 锁定库存余额，按原收货成本出库：
  - `on_hand -= return_qty`
  - `available -= return_qty`
  - `new_avg = (old_on_hand * old_avg - return_cost) / new_on_hand`
  - `new_on_hand = 0` 时 `avg_cost = 0`。
- 剩余库存价值小于退货成本时，平均成本按 0 处理，记录为简化口径。
- 库存不足返回 409 `INSUFFICIENT_STOCK`。
- 库存流水：`direction=out`、`biz_type=purchase_return`。

## 6. 会计凭证

凭证类型/来源 `purchase_return`：

- 借：应付账款（本位币含税额）
- 贷：库存商品（本位币净额）
- 贷：进项税额（本位币税额，零则省略）

凭证日期 = 退货日期，来源 = 采购退货单，摘要“采购退货”。

## 7. 应付贷项单

- 创建退货时锁定原应付发票。
- 未付金额 = `total_amount - paid_amount - credited_amount`。
- `applied = min(退货含税金额, 未付金额)`，`remaining = 退货含税金额 - applied`。
- 更新发票 credited、状态和全额结清时的 `base_revalued_amount=0`。
- 贷项单状态 `applied`/`open`。
- 原发票已全额付款时，贷项单全部 open。
- 复用 `/finance/credit-notes` 列表/详情，增加 `credit_type` 筛选和类型字段。
- 本期不实现供应商贷项抵其他发票或退款。

## 8. API

| 方法 | 路径 | 权限 |
|---|---|---|
| POST | `/api/v1/purchase/receipts/:id/returns` | `purchase.return.write` |
| GET | `/api/v1/purchase/returns` | `purchase.return.read` |
| GET | `/api/v1/purchase/returns/:id` | `purchase.return.read` |
| GET | `/api/v1/finance/credit-notes` | `finance.invoice.read` |

- 采购收货行响应增加 `returned_qty`、`returnable_qty`。
- 退货请求/响应与销售退货对称。
- 贷项单列表增加 `credit_type` 参数和字段。
- 错误：超退 409 `OVER_RETURN`、库存不足 409 `INSUFFICIENT_STOCK`、期间关账 409 `PERIOD_CLOSED`、404、400、401/403。

## 9. 前端

- 采购收货列表增加“退货”操作和退货弹窗。
- 新增“采购退货”页面/菜单。
- 采购收货详情显示已退/可退数量。
- 贷项单页面增加类型列/筛选，区分销售贷项和采购贷项。
- 应付发票详情显示 credited/未付金额。
- SDK/OpenAPI/权限同步。

## 10. 测试

- 部分/全额采购退货金额、成本、库存出库、平均成本。
- 应付贷项单 applied/remaining、发票 credited/outstanding/状态。
- 已付款应付发票退货，贷项单全部 open。
- 超退 409、库存不足 409。
- 多次部分退货累计舍入。
- 外币采购退货本位币金额、期间重估扣减 credited。
- 权限 401/403、非法 UUID/日期/数量、数据范围。
- Stage 3B–3I 干净库全链路。

## 11. 已知限制

- 不做采购退货审批、质检、换货和供应商退款。
- 供应商贷项只冲原应付发票，剩余不能抵其他发票或退款。
- 按原收货成本出库；剩余库存价值不足时平均成本按 0 处理。
- 不自动减少采购订单已收数量/重开订单。
- 退货即时生效。
- 未使用供应商贷项不参与外币重估。
- 采购退货接口暂无幂等键。
