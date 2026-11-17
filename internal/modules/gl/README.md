# gl 模块

总账与基础报表模块，Stage 3C 业财一体化落点：

```text
销售发货 / 采购收货 / 收款 / 付款
              ↓
          自动凭证
              ↓
     总账 / 明细账 / 科目余额表
              ├→ 资产负债表
              └→ 利润表
```

## 分层

- `domain`：科目、期间、凭证、分录、报表行、仓储接口。
- `app`：科目、期间、凭证、报表应用服务。
- `infra/postgres`：科目/期间/凭证/报表仓储，以及业务单据自动过账和凭证回填。
- `http`：Gin Handler、金额字符串转换、权限和审计。

## API

| 方法 | 路径 | 权限 |
|---|---|---|
| GET/POST | `/api/v1/finance/accounts` | `finance.account.read/write` |
| PUT | `/api/v1/finance/accounts/:id` | `finance.account.write` |
| GET | `/api/v1/finance/periods` | `finance.period.read` |
| POST | `/api/v1/finance/periods/:id/close` | `finance.period.close` |
| POST | `/api/v1/finance/periods/:id/reopen` | `finance.period.close` |
| GET/POST | `/api/v1/finance/vouchers` | `finance.voucher.read/write` |
| GET/PUT | `/api/v1/finance/vouchers/:id` | `finance.voucher.read/write` |
| POST | `/api/v1/finance/vouchers/:id/post` | `finance.voucher.post` |
| POST | `/api/v1/finance/vouchers/:id/reverse` | `finance.voucher.reverse` |
| GET | `/api/v1/finance/reports/trial-balance` | `finance.report.read` |
| GET | `/api/v1/finance/reports/general-ledger` | `finance.report.read` |
| GET | `/api/v1/finance/reports/balance-sheet` | `finance.report.read` |
| GET | `/api/v1/finance/reports/income-statement` | `finance.report.read` |

## 自动凭证规则

| 业务单据 | 借方 | 贷方 |
|---|---|---|
| 销售发货（收入） | 应收账款 1122 | 主营业务收入 6001 |
| 销售发货（成本） | 主营业务成本 6401 | 库存商品 1405 |
| 采购收货 | 库存商品 1405 | 应付账款 2202 |
| 客户收款 | 银行存款/现金 | 应收账款 1122 |
| 供应商付款 | 应付账款 2202 | 银行存款/现金 |

- 自动凭证在业务事务内生成，失败则业务回滚。
- 按 `(tenant_id, source_type, source_id)` 幂等。
- 期间不存在时自动创建；关账期间禁止过账。
- 暂不拆分增值税，按单据总额入账。

## 手工凭证

- 创建/编辑草稿、过账、反过账。
- 过账校验借贷平衡、科目末级且启用、期间 open。
- 反过账生成反方向凭证，原凭证标记 `reversed`。

## 历史数据回填

```bash
go run ./cmd/glbackfill
```

为已有销售发货、采购收货、收付款补生成凭证，可重复执行。

## 报表

- 科目余额表：期初、本期借/贷、期末。
- 总账/明细账：按科目列出凭证分录和余额。
- 资产负债表：资产、负债、权益、未结转本年利润。
- 利润表：收入、费用、净利润。
- 只统计已过账且未反过账凭证，并应用租户/数据权限。

## 后续

- 现金流量表、损益结转、年度结转。
- 辅助核算（客商、部门、项目、存货）。
- 多币种、汇兑损益、税额拆分。
- 合并报表、预算、BI。
