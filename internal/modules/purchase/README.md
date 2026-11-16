# purchase 模块

采购订单示例模块，用于验证 Monorepo 的纵向链路：

- `domain`：实体、值对象、仓储接口、领域错误。
- `app`：用例编排，从 context 读取租户，调用领域对象和仓储。
- `infra`：当前为内存仓储，后续替换为 PostgreSQL 仓储。
- `http`：Gin Handler，负责请求/响应 DTO、金额字符串转换和错误映射。

## API

- `GET /api/v1/purchase/orders`
- `POST /api/v1/purchase/orders`

请求示例：

```json
{
  "code": "PO-2026-0001",
  "supplier_id": "SUP-001",
  "amount": "1234.5600",
  "currency": "CNY"
}
```

金额在 API 中使用字符串传输，领域层使用 `shopspring/decimal`，禁止 float。

## 下一步

- 增加 PostgreSQL Repository 和采购订单表迁移。
- 增加审批状态、供应商、价格、收货等聚合。
- 通过 Outbox 发布 `purchase.order.created` 事件。
- 增加权限校验与 capability 检查。
