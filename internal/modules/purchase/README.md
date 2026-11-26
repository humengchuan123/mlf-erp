# purchase 模块

采购订单、采购收货与采购退货模块，Stage 3A 建立第一条业务闭环，Stage 3I 补齐退货：

```text
采购订单 → 提交 → 审批 → 收货 → 库存余额/流水
                              ↓
                        采购退货 → 库存出库 / 冲应付、进项税、存货
```

## 分层

- `domain`：订单头/行、收货单头/行、退货单头/行、状态机、金额计算、仓储接口。
- `app`：创建/编辑/提交/审批/取消/收货/退货用例，校验供应商、仓库、物料、剩余可收/可退数量。
- `infra/postgres`：PostgreSQL 仓储；收货在一个事务中完成订单行更新、收货单、库存余额 upsert、库存流水；退货在一个事务中完成收货行已退数量、库存出库、退货凭证、应付贷项单和发票 credited 更新。
- `http`：Gin Handler、金额/数量字符串转换、权限和审计。

## API

| 方法 | 路径 | 权限 |
|---|---|---|
| GET/POST | `/api/v1/purchase/orders` | `purchase.order.read/write` |
| GET/PUT | `/api/v1/purchase/orders/:id` | `purchase.order.read/write` |
| POST | `/api/v1/purchase/orders/:id/submit` | `purchase.order.write` |
| POST | `/api/v1/purchase/orders/:id/approve` | `purchase.order.approve` |
| POST | `/api/v1/purchase/orders/:id/cancel` | `purchase.order.write` |
| POST | `/api/v1/purchase/orders/:id/receive` | `purchase.order.write` |
| GET | `/api/v1/purchase/receipts` | `purchase.receipt.read` |
| POST | `/api/v1/purchase/receipts/:id/returns` | `purchase.return.write` |
| GET | `/api/v1/purchase/returns` | `purchase.return.read` |
| GET | `/api/v1/purchase/returns/:id` | `purchase.return.read` |

创建订单示例：

```json
{
  "supplier_id": "supplier-uuid",
  "warehouse_id": "warehouse-uuid",
  "order_date": "2026-11-16",
  "currency": "CNY",
  "remark": "",
  "lines": [
    {
      "material_id": "material-uuid",
      "quantity": "10",
      "unit_price": "100.0000"
    }
  ]
}
```

收货示例：

```json
{
  "receipt_date": "2026-11-16",
  "remark": "",
  "lines": [
    {
      "order_line_id": "order-line-uuid",
      "quantity": "4"
    }
  ]
}
```

金额和数量在 API 中使用字符串，领域层使用 `shopspring/decimal`，数据库使用 `numeric`，禁止 float。

## 价税分离（Stage 3E）

- 订单头支持 `price_includes_tax`，默认含税。
- 行支持 `tax_rate_id`，默认取物料税率，可覆盖；保存 `tax_rate` 快照。
- 行金额：`amount` 不含税、`tax_amount` 税额、`gross_amount` 含税金额。
- 采购收货按不含税金额更新库存平均成本和库存流水；应付发票使用含税总额。
- 自动凭证：借库存（不含税）/ 借进项税（`input_vat`）/ 贷应付（含税）。

## 多币种（Stage 3F）

- 订单/收货保存 `currency`、`exchange_rate` 和 `base_*` 本位币金额。
- 收货按收货日汇率计算本位币库存成本和应付发票金额。
- 自动凭证只使用本位币金额；库存成本按本位币不含税金额计价。

## 采购退货（Stage 3I）

- 只能按原采购收货行退货，支持部分/全部退货；退货日期不能早于收货日期。
- 收货行累计 `returned_qty` 不能超过 `quantity`，超退返回 409 `OVER_RETURN`。
- 退货即时生效，状态固定 `posted`，不进入审批流。
- 金额按累计数量差分计算，避免多次部分退货尾差：

```text
priorNet = round(receipt_line.amount × returned_qty / quantity, 4)
newNet   = round(receipt_line.amount × (returned_qty + return_qty) / quantity, 4)
net      = newNet - priorNet
gross    = round(gross_amount × (returned_qty + return_qty) / quantity, 4)
           - round(gross_amount × returned_qty / quantity, 4)
tax      = gross - net
```

- 本位币金额、退货成本同理；全额退货金额与原收货行一致，多次部分退货合计无尾差。
- 退货成本按原收货行本位币净额/数量计算，默认 `unit_cost = base_amount / quantity`（6 位小数）。
- 库存按原收货成本出库：`on_hand`/`available` 减少，重算移动平均成本；库存价值不足时平均成本按 0 处理。
- 库存不足返回 409 `INSUFFICIENT_STOCK`；库存流水 `direction=out`、`biz_type=purchase_return`，`occurred_at` 取实际过账时间。
- 退货凭证：借应付账款（本位币含税）、贷库存商品（本位币净额）、贷进项税（本位币税额，零则省略）；全零不生成凭证。
- 自动生成 `purchase_return` 贷项单，冲原应付发票未付金额，剩余保留为供应商贷项；详见 finance 模块。
- 零金额退货跳过贷项单，库存出库仍执行；采购退货成本取自原收货净额，金额为零时成本也为零，因此不生成凭证。
- 错误码：`OVER_RETURN`/`INSUFFICIENT_STOCK`/`PERIOD_CLOSED` 409，收货行不存在 404，非法 UUID/数量/日期 400。

## 资金/库存操作幂等（Stage 4A）

- `POST /purchase/orders/:id/receive` 和 `POST /purchase/receipts/:id/returns` 支持 `Idempotency-Key`。
- 同一 key + 相同请求体只生成一次收货/退货；重复请求回放首次响应并返回 `Idempotency-Replayed: true`。
- 已完成记录 + 同一 key 不同请求体返回 409 `IDEMPOTENCY_CONFLICT`；并发/进行中同 key 返回 409 `IDEMPOTENCY_IN_PROGRESS`；`failed` 记录任意请求体可重置并重试。
- 前端在收货/退货弹窗打开时生成 `newIdempotencyKey()`（优先 `crypto.randomUUID`，非 HTTPS 回退 `getRandomValues`），成功后清空；网络/5xx/401/403/408/429 及幂等冲突/进行中保留 key，其余 4xx 旋转新 key。
- Header 默认可选；生产强制前需客户端全部接入。实现细节见 finance 模块“资金操作幂等”。

## 审批流（Stage 4B）

- 提交时按单据类型、组织、金额阈值匹配审批流；无匹配流程保持原有 `draft -> submitted -> approved` 直审行为。
- 有匹配流程：订单保持 `submitted`，创建 pending 审批请求；`approve` 按顺序步骤推进，最后一步通过后订单 `approved`；`reject` 后订单回 `draft`；取消订单会取消 pending 请求。
- 审批人支持角色/指定用户；禁止自批；超级管理员可越过角色限制但不能自批。
- 审批最终通过/拒绝后的订单转换使用审批授权范围，审批人不需要拥有订单的数据范围。
- 权限：订单审批 `purchase.order.approve`；流程配置/待办 `system.approval.read/write`。
- 订单详情显示审批历史；待办页统一批准/拒绝。
- 首期只覆盖订单；退款/反核销/发货/收货/退货/期间重开审批在 4B-2。

## 状态机

```text
draft → submitted → approved → partially_received → received
   └──────────────┴──────────────┴──→ cancelled（无收货）
```

## 后续

- 采购退货审批、质检、换货、供应商退款。
- 供应商贷项抵其他发票、VAT 红字进项发票、成本层恢复。
- 价格协议、供应商绩效。
- 销售出库、调拨、盘点。
- 采购订单/收货/退货事件接入 Outbox。
