# sales 模块

销售订单、发货出库与销售退货模块，Stage 3B/3H 业务闭环：

```text
销售订单 → 提交 → 审批 → 发货出库 → 库存扣减 → 应收发票
                              ↓
                        销售退货 → 库存回库 → 贷项单冲应收
```

## 分层

- `domain`：销售订单头/行、发货单头/行、退货单头/行、状态机、金额计算、剩余可发数量、剩余可退数量、仓储接口。
- `app`：创建/编辑/提交/审批/取消/发货/退货用例，校验客户、仓库、物料、重复行、剩余可发数量和累计可退数量。
- `infra/postgres`：PostgreSQL 仓储；发货在一个事务中完成订单行更新、库存行锁、库存扣减、库存流水、应收发票和订单状态更新；退货在同一事务中完成退货数量校验、库存回库、库存流水、退货凭证、贷项单和发票 credited 更新。
- `http`：Gin Handler、金额/数量字符串转换、权限和审计。

## API

| 方法 | 路径 | 权限 |
|---|---|---|
| GET/POST | `/api/v1/sales/orders` | `sales.order.read/write` |
| GET/PUT | `/api/v1/sales/orders/:id` | `sales.order.read/write` |
| POST | `/api/v1/sales/orders/:id/submit` | `sales.order.write` |
| POST | `/api/v1/sales/orders/:id/approve` | `sales.order.approve` |
| POST | `/api/v1/sales/orders/:id/cancel` | `sales.order.write` |
| POST | `/api/v1/sales/orders/:id/ship` | `sales.order.write` |
| GET | `/api/v1/sales/shipments` | `sales.shipment.read` |
| POST | `/api/v1/sales/shipments/:id/returns` | `sales.return.write` |
| GET | `/api/v1/sales/returns` | `sales.return.read` |
| GET | `/api/v1/sales/returns/:id` | `sales.return.read` |

创建订单示例：

```json
{
  "customer_id": "customer-uuid",
  "warehouse_id": "warehouse-uuid",
  "order_date": "2026-11-17",
  "currency": "CNY",
  "remark": "",
  "lines": [
    {
      "material_id": "material-uuid",
      "quantity": "10",
      "unit_price": "120.0000"
    }
  ]
}
```

发货示例：

```json
{
  "shipment_date": "2026-11-17",
  "remark": "",
  "lines": [
    {
      "order_line_id": "order-line-uuid",
      "quantity": "4"
    }
  ]
}
```

## 状态机

```text
draft → submitted → approved → partially_shipped → shipped
   └──────────────┴──────────────┘
              cancelled（无发货）
```

## 发货事务

1. `FOR UPDATE` 锁定销售订单和订单行。
2. 校验状态、重复行、剩余可发数量。
3. 按物料稳定排序并 `FOR UPDATE` 锁定库存余额。
4. 校验可用库存。
5. 插入发货单/行、更新已发数量、扣减库存、写库存流水。
6. 生成应收发票。
7. 重算订单状态并提交。

## 资金/库存操作幂等（Stage 4A）

- `POST /sales/orders/:id/ship` 和 `POST /sales/shipments/:id/returns` 支持 `Idempotency-Key`。
- 同一 key + 相同请求体只生成一次发货/退货；重复请求回放首次响应并返回 `Idempotency-Replayed: true`。
- 已完成记录 + 同一 key 不同请求体返回 409 `IDEMPOTENCY_CONFLICT`；并发/进行中同 key 返回 409 `IDEMPOTENCY_IN_PROGRESS`；`failed` 记录任意请求体可重置并重试。
- 前端在发货/退货弹窗打开时生成 `newIdempotencyKey()`（优先 `crypto.randomUUID`，非 HTTPS 回退 `getRandomValues`），成功后清空；网络/5xx/401/403/408/429 及幂等冲突/进行中保留 key，其余 4xx 旋转新 key。
- Header 默认可选；生产强制前需客户端全部接入。实现细节见 finance 模块“资金操作幂等”。

## 前端

- `/sales/orders`：销售订单列表、新建/编辑、详情、提交、审批、取消、发货。
- `/sales/shipments`：发货单列表和详情，显示已退/可退数量，提供退货操作。
- `/sales/returns`：销售退货列表和详情，展示退货行、原发票、贷项单已冲/剩余和凭证号。

## 价税分离（Stage 3E）

- 订单头支持 `price_includes_tax`，默认含税。
- 行支持 `tax_rate_id`，默认取物料税率，可覆盖；保存 `tax_rate` 快照。
- 行金额：`amount` 不含税、`tax_amount` 税额、`gross_amount` 含税金额。
- 订单头：`net_amount`、`tax_amount`、`total_amount`（含税）。
- 发货行从订单行复制税率并重新按发货数量计算；应收发票使用含税总额。
- 自动凭证：借应收（含税）/ 贷收入（不含税）/ 贷销项税（`output_vat`）。

## 多币种（Stage 3F）

- 订单/发货保存 `currency`、`exchange_rate` 和 `base_*` 本位币金额；CNY 汇率为 1。
- 发货按发货日汇率重新计算本位币金额，应收发票继承发货汇率。
- 自动凭证只使用本位币金额。
- 收付款核销按付款汇率与发票汇率差额生成汇兑损益凭证。

## 销售退货（Stage 3H）

- 只能按原发货行退货，支持部分/全部退货；退货日期不能早于发货日期。
- 发货行累计 `returned_qty`，响应返回 `returned_qty` 和 `returnable_qty = quantity - returned_qty`。
- 累计退货数量校验，超退返回 409 `OVER_RETURN`；期间已关账返回 409 `PERIOD_CLOSED`。
- 退货行金额按原发货行比例计算（净额/税额/含税/本位币/成本），按累计数量差额取数并保留 4 位小数，多次部分退货合计与原发货金额一致。
- 库存按原发货行 `unit_cost` 回库，重算移动平均成本：
  `new_avg = (old_on_hand × old_avg + return_cost) / (old_on_hand + return_qty)`。
- 退货事务内写入 `inventory_transactions`（`direction=in`、`biz_type=sales_return`）。
- 生成 `sales_return` 自动凭证：借收入/销项税/库存，贷应收/成本。
- 同时生成贷项单并自动冲原应收发票，退货详情返回 `credit_note_id`、`credit_note_*` 金额和状态。

退货示例：

```json
{
  "return_date": "2026-11-18",
  "reason": "质量问题",
  "remark": "",
  "lines": [
    {
      "shipment_line_id": "shipment-line-uuid",
      "quantity": "1"
    }
  ]
}
```

## 后续

- 报价、价格协议、折扣、客户信用。
- 批次/序列号、预留库存、调拨。
- 事件 Outbox。
