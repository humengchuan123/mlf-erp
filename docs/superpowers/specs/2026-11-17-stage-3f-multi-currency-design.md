# Stage 3F 多币种、汇率与汇兑损益设计

> 日期：2026-11-17
> 分支：`stage-3f-multi-currency`
> 状态：设计已确认

## 1. 目标

在 Stage 3E 价税分离基础上增加多币种：

```text
外币单据（原币金额 + 汇率）
        ↓
本位币金额（CNY）→ 总账自动凭证
        ↓
收付款核销已实现汇兑损益
        ↓
期末应收应付外币重估（未实现汇兑损益）
```

## 2. 范围

### 本期

- 汇率表及汇率管理 API/页面。
- 销售/采购订单、发货、收货、应收应付发票、收付款支持外币和汇率快照。
- 单据保存原币金额、汇率、本位币金额。
- 总账自动凭证使用本位币金额。
- 收付款核销时计算已实现汇兑损益。
- 期末关账自动重估未核销外币应收应付，次期初自动反向。
- 反关账时撤销重估及次期反向凭证。
- 前端汇率、本位币金额和汇兑损益展示。

### 后置

- 银行/现金外币重估。
- 跨币种直接核销。
- 多本位币、合并报表折算。
- 外汇套期、远期汇率。
- 外币预算和报表折算。

## 3. 本位币与汇率

- 本位币固定 `CNY`。
- `CNY` 汇率固定 1，不存汇率表。
- 外币汇率含义：1 单位外币 = X CNY。
- 汇率精度 8 位小数。

新增 `erp.finance_exchange_rates`：

- `id`, `tenant_id`, `currency_code`, `rate_date`, `rate numeric(19,8)`。
- `created_by`, `created_at`, `updated_at`。
- 唯一 `(tenant_id, currency_code, rate_date)`。

取数规则：

- 单据日期最近一条 `rate_date <= 单据日期` 的汇率。
- 单据可手工覆盖汇率，保存快照。
- 找不到汇率时拒绝外币单据。

## 4. 金额模型

单据头（销售/采购订单、发货、收货、应收应付发票）：

- `exchange_rate numeric(19,8) NOT NULL DEFAULT 1`
- `base_net_amount numeric(19,4) NOT NULL DEFAULT 0`
- `base_tax_amount numeric(19,4) NOT NULL DEFAULT 0`
- `base_total_amount numeric(19,4) NOT NULL DEFAULT 0`

单据行：

- `base_amount`、`base_tax_amount`、`base_gross_amount`。

应收应付发票额外：

- `base_paid_amount`：按发票汇率计算的本位币已核销。
- `base_revalued_amount`：当前未转回的累计重估调整。

收付款：

- `exchange_rate`
- `base_amount`
- `base_allocated_amount`

核销：

- `amount`：原币核销金额。
- `base_amount`：按付款汇率计算。
- `invoice_base_amount`：按发票汇率计算。

计算：

- `base = original × exchange_rate`，4 位小数。
- 税额拆分仍按 Stage 3E 原币计算，再折算本位币。
- 表头本位币合计 = 各行本位币金额合计。
- 对含税/不含税的行，`base_net + base_tax = base_gross` 必须成立。

## 5. 自动凭证

- 销售发货：借应收（本位币含税）/ 贷收入（本位币不含税）/ 贷销项税（本位币）。
- 采购收货：借库存（本位币不含税）/ 借进项税（本位币）/ 贷应付（本位币含税）。
- 收付款：按本位币金额。
- 原币仅保留在业务单据和发票/收付款，不进入总账金额。

## 6. 已实现汇兑损益

付款或收款创建时按付款日汇率挂账：

- 收款：借银行（本位币）/ 贷应收（本位币）。
- 付款：借应付（本位币）/ 贷银行（本位币）。

核销时：

```text
fx_diff = 核销原币金额 × (付款汇率 - 发票汇率)
```

- 应收：
  - `fx_diff > 0`：借应收 / 贷汇兑收益。
  - `fx_diff < 0`：借汇兑损失 / 贷应收。
- 应付：
  - `fx_diff > 0`：借汇兑损失 / 贷应付。
  - `fx_diff < 0`：借应付 / 贷汇兑收益。

- 每次核销（含追加核销）生成一条 `finance_fx_settlements` 和一张汇兑损益凭证，来源 `fx_settlement`。
- `fx_diff = 0` 不生成凭证。

## 7. 期末重估

关账事务内，在损益结转之前：

1. 锁定期间。
2. 查询未核销外币应收应付发票。
3. 取期间结束日最近汇率。
4. `target_base = (total_amount - paid_amount) × closing_rate`。
5. `book_base = base_total_amount - base_paid_amount + base_revalued_amount`。
6. `adjustment = target_base - book_base`。
7. 生成重估凭证：
   - 应收：`adjustment > 0` 借应收/贷汇兑收益；`< 0` 借汇兑损失/贷应收。
   - 应付：`adjustment > 0` 借汇兑损失/贷应付；`< 0` 借应付/贷汇兑收益。
8. 写 `gl_fx_revaluations`，更新发票 `base_revalued_amount`。
9. 次期初生成反向凭证；若次期未创建则自动创建；若次期已关账则拒绝关账。

`gl_fx_revaluations`：

- `period_id`, `invoice_id`, `invoice_type`, `currency`
- `foreign_balance`, `old_rate`, `new_rate`, `base_adjustment`
- `voucher_id`, `reversal_voucher_id`
- `reopen_voucher_id`, `reopen_reversal_voucher_id`
- `status`：`posted` / `reopened`
- 唯一：同一期间同一发票只能有一条 `posted` 记录。

反关账：

- 对每条 `posted` 重估记录生成：
  - 反向重估凭证（本期间，来源 `fx_revaluation_reopen`）。
  - 反向的次期反向凭证（次期，来源 `fx_revaluation_reversal_reopen`）。
- 记录置 `reopened`，发票 `base_revalued_amount` 回退。
- 次期已关账时拒绝反关账。

## 8. 科目

新增：

- `660301 汇兑损失`：费用类、借方、末级。
- `660302 汇兑收益`：费用类、贷方、末级。
- `6603 财务费用` 改为非末级。

映射：

- `fx_loss=660301`
- `fx_gain=660302`

## 9. API

| 方法 | 路径 | 权限 |
|---|---|---|
| GET | `/api/v1/finance/exchange-rates` | `finance.exchange_rate.read` |
| GET | `/api/v1/finance/exchange-rates/latest` | `finance.exchange_rate.read` |
| POST | `/api/v1/finance/exchange-rates` | `finance.exchange_rate.write` |
| PUT | `/api/v1/finance/exchange-rates/:id` | `finance.exchange_rate.write` |

- 单据/发票/收付款响应增加汇率和本位币字段。
- 收付款创建请求增加可选 `exchange_rate`。
- 订单/发货/收货请求增加可选 `exchange_rate`。

## 10. 前端

- 新增“汇率管理”页面、路由、菜单。
- 销售/采购订单：币种从主数据选择，展示/输入汇率和本位币合计。
- 发货/收货详情：原币、汇率、本位币。
- 应收应付：币种、汇率、本位币总额/未核销。
- 收付款：币种、汇率、本位币金额。
- 报表注明本位币 CNY；汇兑损益在利润表体现。

## 11. 迁移

`000018_multi_currency`：

- 汇率表。
- 单据/发票/收付款本位币字段和汇率。
- 核销表本位币字段。
- `finance_fx_settlements`、`gl_fx_revaluations`。
- 历史数据：汇率=1、本位币金额=原币金额。
- 660301/660302 科目、映射、汇率权限。

## 12. 测试

- 汇率取数、快照、CNY 固定汇率、缺失汇率。
- 外币销售/采购单据本位币金额和自动凭证。
- 收付款已实现汇兑收益/损失凭证。
- 期末重估、次期反向、反关账撤销、重复关账幂等。
- 权限、401/403。
- Stage 3B/3C/3D/3E 回归和干净库全链路。

## 13. 验收标准

- [ ] 汇率表可维护，单据可自动取汇率并覆盖。
- [ ] 外币单据原币/本位币金额正确。
- [ ] 自动凭证使用本位币金额。
- [ ] 收付款核销生成已实现汇兑损益。
- [ ] 期末重估和次期反向正确。
- [ ] 反关账撤销重估。
- [ ] 前端、SDK、OpenAPI、迁移完整。
- [ ] 自动化测试、静态检查、构建和回归通过。
