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
| POST | `/api/v1/finance/periods/ensure` | `finance.period.close` |
| POST | `/api/v1/finance/periods/:id/close` | `finance.period.close` |
| POST | `/api/v1/finance/periods/:id/reopen` | `finance.period.close` |
| POST | `/api/v1/finance/years/:year/close` | `finance.period.close` |
| GET | `/api/v1/finance/years/:year/close` | `finance.period.read` |
| GET/POST | `/api/v1/finance/vouchers` | `finance.voucher.read/write` |
| GET/PUT | `/api/v1/finance/vouchers/:id` | `finance.voucher.read/write` |
| POST | `/api/v1/finance/vouchers/:id/post` | `finance.voucher.post` |
| POST | `/api/v1/finance/vouchers/:id/reverse` | `finance.voucher.reverse` |
| GET | `/api/v1/finance/reports/trial-balance` | `finance.report.read` |
| GET | `/api/v1/finance/reports/general-ledger` | `finance.report.read` |
| GET | `/api/v1/finance/reports/balance-sheet` | `finance.report.read` |
| GET | `/api/v1/finance/reports/income-statement` | `finance.report.read` |
| GET | `/api/v1/finance/reports/auxiliary-balance` | `finance.report.read` |
| GET | `/api/v1/finance/dimensions` | `finance.voucher.read` |

## 自动凭证规则

| 业务单据 | 借方 | 贷方 |
|---|---|---|
| 销售发货（收入） | 应收账款 1122（含税） | 主营业务收入 6001（不含税）+ 销项税 222102 |
| 销售发货（成本） | 主营业务成本 6401 | 库存商品 1405 |
| 采购收货 | 库存商品 1405（不含税）+ 进项税 222101 | 应付账款 2202（含税） |
| 客户收款 | 银行存款/现金 | 应收账款 1122 |
| 供应商付款 | 应付账款 2202 | 银行存款/现金 |

- 自动凭证在业务事务内生成，失败则业务回滚。
- 按 `(tenant_id, source_type, source_id)` 幂等。
- 期间不存在时自动创建；关账期间禁止过账。
- 销售/采购自动凭证按不含税金额确认收入/库存，进项/销项税单独入账；税额为 0 时不生成税务行。
- 收付款仍按含税总额核销，不拆分税额。

## 手工凭证

- 创建/编辑草稿、过账、反过账。
- 过账校验借贷平衡、科目末级且启用、期间 open。
- 反过账生成反方向凭证，原凭证标记 `reversed`。
- 本期只允许反过账手工凭证；自动凭证应通过业务退货/反核销等业务反向流程处理，避免总账与业务子账不一致。
- 凭证号按生成/过账时间编号，历史回填使用回填时间；凭证日期单独记录，报表按凭证日期取数。

## 期末结转与年度结转

- 关账自动汇总本期收入/费用余额，生成损益结转凭证到本年利润 4103；无损益时也写关账记录、不生成凭证。
- 反关账生成同期间反向结转凭证，期间恢复 open；再次关账生成新的结转凭证。
- 年度结转要求该年度 12 个期间全部关账，且每个已关账期间都有对应的关账记录；在次年 1 月 1 日把 4103 结转到利润分配 4104。
- 年度结转按年度顺序执行：之后年度已结转或上一年度尚未结转时拒绝；年度结转后该年度期间禁止反关账，且不提供年度反结转。
- Stage 3D 之前关账的期间没有关账记录，需要先反关账再重新关账，才能进行年度结转。
- 利润表、辅助余额表排除结转/年度结转凭证；科目余额表、资产负债表、明细账包含。

## 辅助核算

- 凭证行支持客商、部门、项目编码。
- 自动凭证带入客商；部门/项目可手工录入。
- 辅助余额表按客商/部门/项目分组，展示期初、本期借/贷、期末。
- 项目当前为轻量文本编码，后续可接项目主数据。

## 历史数据回填

```bash
go run ./cmd/glbackfill
```

为已有销售发货、采购收货、收付款补生成凭证，可重复执行。

## 报表

- 科目余额表：期初、本期借/贷、期末。
- 总账/明细账：按科目列出凭证分录和余额。
- 资产负债表：资产、负债、权益、未结转利润（累计）。
- 利润表：收入、费用、净利润。
- 期末关账自动生成损益结转凭证，损益科目余额归零，本年利润进入 4103；年度结转后 4103 进入 4104。
- 利润表、辅助余额表排除期末/年度结转凭证；科目余额表、资产负债表、明细账包含。
- 统计 `posted` 和 `reversed` 凭证；反过账凭证按凭证日期参与报表，原凭证保留在历史期间，反向凭证在反过账期间抵消。
- 报表严格应用组织数据范围。结转凭证归属执行关账的用户组织：公司范围用户可见，部门范围用户只看到本部门业务凭证（未结转经营口径），不会看到全公司损益。
- 增值税汇总表：按月销项税（222102）、进项税（222101）、应纳税额，应用组织数据范围。

## 增值税科目

- `222101 应交增值税-进项税额`：负债类、借方、末级。
- `222102 应交增值税-销项税额`：负债类、贷方、末级。
- `2221 应交税费` 为非末级父科目。
- 科目映射：`input_vat=222101`、`output_vat=222102`。

## 多币种与汇兑损益（Stage 3F）

- 总账只记录 CNY 本位币金额。
- `660301 汇兑损失`、`660302 汇兑收益`，`6603 财务费用` 为非末级。
- 映射：`fx_loss=660301`、`fx_gain=660302`。
- 收付款核销生成 `fx_settlement` 已实现汇兑损益。
- 期末关账重估未核销外币应收应付，生成 `fx_revaluation`，次期初生成 `fx_revaluation_reversal`。
- 反关账生成 `fx_revaluation_reopen` 和 `fx_revaluation_reversal_reopen`，并回退发票累计重估。

## 后续

- 多币种、汇兑损益。
- 现金流量表。
- 完整增值税申报表、附加税、红字发票。
- 存货辅助核算、项目主数据、辅助核算明细账。
- 反结转与更严格的年度重开控制。
- 合并报表、预算、BI。
