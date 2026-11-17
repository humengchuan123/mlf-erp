# purchase 模块

采购订单与采购收货模块，Stage 3A 第一条业务闭环：

```text
采购订单 → 提交 → 审批 → 收货 → 库存余额/流水
```

## 分层

- `domain`：订单头/行、收货单头/行、状态机、金额计算、仓储接口。
- `app`：创建/编辑/提交/审批/取消/收货用例，校验供应商、仓库、物料、剩余可收数量。
- `infra/postgres`：PostgreSQL 仓储；收货在一个事务中完成订单行更新、收货单、库存余额 upsert、库存流水。
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

## 状态机

```text
draft → submitted → approved → partially_received → received
   └──────────────┴──────────────┴──→ cancelled（无收货）
```

## 后续

- 采购退货、价格协议、供应商绩效。
- 销售出库、调拨、盘点。
- 采购订单/收货事件接入 Outbox。
