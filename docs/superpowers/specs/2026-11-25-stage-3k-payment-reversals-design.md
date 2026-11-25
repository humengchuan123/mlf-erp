# Stage 3K 收付款反核销设计

> 日期：2026-11-25
> 分支：`stage-3k-payment-reversals`
> 状态：设计已确认

## 1. 目标

补收付款核销的纠错闭环：

```text
收款/付款核销
   ↓
整行反核销
   ↓
回退发票已收/已付和状态
   ↓
回退收付款已核销金额和状态
   ↓
回退已实现汇兑结算（反方向凭证）
```

## 2. 范围

### 本期

- 对已核销的收款/付款，按发票核销行整行反核销。
- 支持一笔收付款选择一条或多条核销行。
- 回退发票 `paid_amount`、`base_paid_amount` 和状态。
- 回退收付款 `allocated_amount`、`base_allocated_amount` 和状态。
- 回退该核销行对应的已实现汇兑结算，生成反方向凭证。
- 保留原核销、原汇兑结算和原凭证；新增反核销记录和反方向凭证。
- 新权限、API、SDK、OpenAPI、前端和接口测试。

### 后置

- 部分金额反核销。
- 贷项抵用/退款反冲。
- 反核销审批。
- 幂等键。
- 反核销的反向恢复。

## 3. 数据模型

迁移 `000024_payment_reversals`。

### 3.1 finance_payment_allocation_reversals

```sql
CREATE TABLE erp.finance_payment_allocation_reversals (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    payment_id uuid NOT NULL,
    invoice_id uuid NOT NULL,
    allocation_id uuid NOT NULL,
    amount numeric(19,4) NOT NULL CHECK (amount > 0),
    base_amount numeric(19,4) NOT NULL CHECK (base_amount >= 0),
    invoice_base_amount numeric(19,4) NOT NULL CHECK (invoice_base_amount >= 0),
    reversal_date date NOT NULL,
    reason text NOT NULL,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT finance_payment_allocation_reversals_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT finance_payment_allocation_reversals_payment_fk
        FOREIGN KEY (tenant_id, payment_id)
        REFERENCES erp.finance_payments (tenant_id, id),
    CONSTRAINT finance_payment_allocation_reversals_invoice_fk
        FOREIGN KEY (tenant_id, invoice_id)
        REFERENCES erp.finance_invoices (tenant_id, id)
);
```

索引：

- `(tenant_id, payment_id, reversal_date DESC)`
- `(tenant_id, invoice_id)`

`allocation_id` 保留原核销行 ID，便于审计；原核销行反核销后删除。

### 3.2 finance_fx_settlements 扩展

```sql
ALTER TABLE erp.finance_fx_settlements
    ADD COLUMN status text NOT NULL DEFAULT 'posted',
    ADD COLUMN reversal_id uuid,
    ADD COLUMN reversal_voucher_id uuid,
    ADD COLUMN reversed_at timestamptz,
    ADD COLUMN reversed_by uuid;

ALTER TABLE erp.finance_fx_settlements
    ADD CONSTRAINT finance_fx_settlements_status_check
        CHECK (status IN ('posted', 'reversed')),
    ADD CONSTRAINT finance_fx_settlements_reversal_fk
        FOREIGN KEY (tenant_id, reversal_id)
        REFERENCES erp.finance_payment_allocation_reversals (tenant_id, id),
    ADD CONSTRAINT finance_fx_settlements_reversal_voucher_fk
        FOREIGN KEY (tenant_id, reversal_voucher_id)
        REFERENCES erp.gl_vouchers (tenant_id, id);
```

索引：

- `(tenant_id, payment_id, invoice_id, status)`

### 3.3 权限

- `finance.payment.reverse`：反核销。
- 收付款列表/详情继续使用 `finance.payment.read`；发票详情继续使用 `finance.invoice.read`。

### 3.4 Down 迁移

- 有反核销记录或已反核销汇兑结算时拒绝执行。
- 否则删除新表、扩展列、索引、权限，恢复 `finance_fx_settlements` 原结构。
- README 记录 down guard。

## 4. 反核销规则

### 4.1 校验

- 收付款必须存在、状态 `allocated` / `partially_allocated`。
- 请求至少选择一条发票核销行，发票不能重复。
- 每条核销行必须属于该收付款，且仍有有效核销金额。
- 反核销日期默认当天，不能早于收付款日期。
- 反核销日期必须落在 open 期间。
- 必须填写非空原因。
- 收付款、发票和核销行均应用组织数据范围。

### 4.2 事务

1. 锁定收付款 `FOR UPDATE`。
2. 按发票 ID 排序锁定发票 `FOR UPDATE`。
3. 锁定对应 `finance_payment_allocations` 行；不存在则返回 404/409。
4. 汇总反核销金额。
5. 对每条核销行：
   - 记录原核销行 ID、原币金额、付款本位币金额、发票本位币金额。
   - 删除核销行。
   - 插入 `finance_payment_allocation_reversals`。
6. 更新收付款：
   - `allocated_amount -= 反核销金额`
   - `base_allocated_amount -= 付款本位币金额`
   - 状态按剩余未核销重算。
   - SQL 守卫 `allocated_amount >= 金额`。
7. 更新发票：
   - `paid_amount -= 反核销金额`
   - `base_paid_amount -= 发票本位币金额`
   - 状态按 `paid + credited` 重算：
     - `paid`：`paid + credited >= total`
     - `partially_paid`：`paid + credited > 0`
     - `open`：否则
   - 状态为 `paid` 时 `base_revalued_amount = 0`。
   - SQL 守卫 `paid_amount >= 金额`。
8. 对该收付款+发票下所有 `status='posted'` 的汇兑结算：
   - 生成反方向 `fx_settlement_reversal` 凭证。
   - 标记结算 `reversed`，记录 `reversal_id`、`reversal_voucher_id`、`reversed_at`、`reversed_by`。
9. Commit，返回收付款详情（含核销和反核销历史）。

### 4.3 状态计算

收付款状态：

```text
allocated = allocated_amount
unallocated       : allocated == 0
partially_allocated: 0 < allocated < amount
allocated         : allocated == amount
```

发票状态：

```text
settled = paid_amount + credited_amount
open              : settled == 0
partially_paid    : 0 < settled < total_amount
paid              : settled >= total_amount
```

### 4.4 与贷项的关系

- 反核销只减少 `paid_amount` 和 `base_paid_amount`，不修改 `credited_amount`。
- 发票状态按 `paid + credited` 重算；已由贷项冲减的发票反核销后可能仍为 `paid` 或变为 `partially_paid`。
- 反核销不修改贷项单、贷项抵用或退款记录。

## 5. 汇兑反核销

- 原 `finance_fx_settlements.base_adjustment` 保留。
- 反方向凭证使用 `-base_adjustment`：
  - 应收：原汇兑收益/损失反向。
  - 应付：原汇兑收益/损失反向。
- 凭证类型/来源 `fx_settlement_reversal`，凭证日期为反核销日期，摘要“汇兑结算反核销 <收付款号>”。
- 原汇兑结算标记 `reversed`；同一核销行有多个结算时逐个反核销。
- 无汇兑结算时，反核销为子账操作，不生成凭证；仍要求反核销日期 open。

## 6. API

| 方法 | 路径 | 权限 |
|---|---|---|
| POST | `/api/v1/finance/payments/:id/unallocate` | `finance.payment.reverse` |
| GET | `/api/v1/finance/payments/:id` | `finance.payment.read` |
| GET | `/api/v1/finance/invoices/:id` | `finance.invoice.read` |

请求：

```json
{
  "invoice_ids": ["invoice-uuid-1", "invoice-uuid-2"],
  "reversal_date": "2026-11-25",
  "reason": "核销错误"
}
```

响应：

- 收付款详情增加 `allocations` 和 `reversals`。
- 发票详情增加 `payment_allocation_reversals`。
- 反核销记录包含发票号、原币金额、付款本位币、发票本位币、日期、原因、创建人、汇兑反核销凭证号。

错误码：

- 400 `BAD_REQUEST` / `REASON_REQUIRED` / `DUPLICATE_INVOICE`
- 404 `NOT_FOUND`
- 409 `PAYMENT_NOT_ALLOCATED` / `ALLOCATION_NOT_FOUND` / `PERIOD_CLOSED`
- 401/403

## 7. SDK、OpenAPI 与前端

### SDK/OpenAPI

- `PaymentAllocationReversal`、`PaymentAllocation`、`UnallocatePaymentInput`。
- `financeApi.payments.unallocate(id, input)`。
- `Payment` 详情增加 `allocations`/`reversals`。
- `InvoiceDetail` 增加 `payment_allocation_reversals`。
- OpenAPI 升级 `1.2.0`。

### 前端

- 收付款页：
  - 已核销/部分核销行显示“反核销”。
  - 反核销弹窗加载当前核销行，支持多选发票、反核销日期、原因。
  - 详情展示核销历史和反核销历史（含汇兑反核销凭证）。
- 发票详情增加付款核销/反核销记录。
- 权限控制 `finance.payment.reverse`。
- 前端完整度断言。

## 8. 测试

- 单张/多张发票整行反核销，收付款/发票金额和状态正确。
- 部分核销、全额核销、反核销后重新核销。
- 有已实现汇兑结算的反核销：结算 reversed，反方向凭证借贷平衡、汇兑方向正确。
- 已由贷项冲减的发票反核销后状态按 `paid + credited` 重算。
- 拒绝：未知收付款/发票、非本收付款核销行、重复发票、空选择、已反核销、关账期间、原因缺失、权限 401/403、数据范围。
- 并发双反核销只成功一次。
- 干净库 Stage 3B–3K 全链路。

## 9. 已知限制

- 只支持整行反核销，不支持部分金额。
- 不撤销收付款本身的现金/银行凭证。
- 不包含贷项抵用/退款反冲、反核销审批、幂等键。
- 无汇兑结算的反核销不生成总账凭证。
- 反核销日期必须在 open 期间。
- 已反核销不能恢复。
- 反核销记录和原核销行是删除+审计表模型，不在原核销行上保留历史。
