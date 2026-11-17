# sales 模块

销售订单与发货出库模块，Stage 3B 业务闭环：

```text
销售订单 → 提交 → 审批 → 发货出库 → 库存扣减 → 应收发票
```

## 分层

- `domain`：销售订单头/行、发货单头/行、状态机、金额计算、剩余可发数量、仓储接口。
- `app`：创建/编辑/提交/审批/取消/发货用例，校验客户、仓库、物料、重复行和剩余可发数量。
- `infra/postgres`：PostgreSQL 仓储；发货在一个事务中完成订单行更新、库存行锁、库存扣减、库存流水、应收发票和订单状态更新。
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

## 前端

- `/sales/orders`：销售订单列表、新建/编辑、详情、提交、审批、取消、发货。
- `/sales/shipments`：发货单列表和详情。

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

## 后续

- 销售退货、报价、价格协议、折扣、客户信用。
- 批次/序列号、预留库存、调拨。
- 发货幂等键、事件 Outbox。
