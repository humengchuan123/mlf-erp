# finance 模块

统一应收/应付发票与收付款核销模块，Stage 3B 覆盖：

```text
销售发货 → 应收发票 ─┐
                      ├→ 收款/付款 → 核销
采购收货 → 应付发票 ─┘
```

## 分层

- `domain`：发票、收付款、核销、状态、金额计算、仓储接口。
- `app`：发票查询、收付款登记、核销用例。
- `infra/postgres`：PostgreSQL 查询与核销事务；核销时锁定收付款和发票行，校验类型、往来单位、币种、未核销金额。
- `http`：Gin Handler、金额字符串转换、权限和审计。

## API

| 方法 | 路径 | 权限 |
|---|---|---|
| GET | `/api/v1/finance/invoices` | `finance.invoice.read` |
| GET | `/api/v1/finance/invoices/:id` | `finance.invoice.read` |
| GET | `/api/v1/finance/payments` | `finance.payment.read` |
| GET | `/api/v1/finance/payments/:id` | `finance.payment.read` |
| POST | `/api/v1/finance/payments` | `finance.payment.write` |
| POST | `/api/v1/finance/payments/:id/allocate` | `finance.payment.write` |

创建收付款：

```json
{
  "payment_type": "receipt",
  "partner_id": "customer-uuid",
  "payment_date": "2026-11-17",
  "currency": "CNY",
  "amount": "500.00",
  "method": "bank",
  "reference": "BANK-001",
  "remark": ""
}
```

核销：

```json
{
  "allocations": [
    { "invoice_id": "invoice-uuid-1", "amount": "480.00" },
    { "invoice_id": "invoice-uuid-2", "amount": "20.00" }
  ]
}
```

## 发票

- `invoice_type`：`receivable` / `payable`。
- `source_type`：`sales_shipment` / `purchase_receipt`。
- 状态：`open → partially_paid → paid`。
- 应收由销售发货自动生成，应付由采购收货自动生成。
- 来源唯一约束：`(tenant_id, invoice_type, source_type, source_id)`，防止重复开票。

## 收付款

- `payment_type`：`receipt` / `payment`。
- 状态：`unallocated → partially_allocated → allocated`。
- 收款只能核销应收，付款只能核销应付。
- 核销必须同往来单位、同币种。
- 支持部分核销和多张发票一次核销。

## 前端

- `/finance/invoices`：应收/应付列表、筛选、到期/逾期提示、发票详情、核销记录。
- `/finance/payments`：收付款列表、新建、按往来单位加载未付清发票、批量核销。

## 后续

- 反核销、红冲、发票作废。
- 账龄分析、对账单、信用额度。
- 多币种汇率和本位币折算。
- 总账凭证自动生成。
