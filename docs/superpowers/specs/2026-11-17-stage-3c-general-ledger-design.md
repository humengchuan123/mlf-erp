# Stage 3C 总账与基础报表设计

> 日期：2026-11-17
> 分支：`stage-3c-general-ledger`
> 状态：设计已确认

## 1. 目标

把 Stage 3A/3B 的采购、销售、库存、应收应付、收付款接入总账，形成基础业财一体化闭环：

```text
业务单据 → 自动凭证 → 总账/明细账 → 科目余额表
                              ├→ 资产负债表
                              └→ 利润表
```

## 2. 范围

### 本期

- 会计科目表、会计期间。
- 手工凭证录入、过账、反过账。
- 销售发货、采购收货、客户收款、供应商付款自动生成凭证。
- 科目余额表、总账/明细账、资产负债表、利润表。
- 科目、期间、凭证、报表前端页面。
- 自动凭证幂等和来源追溯。
- 数据权限、审计、Demo 数据、接口自动化测试。

### 明确后置

- 现金流量表。
- 期末损益结转、年度结转。
- 多币种、汇率、汇兑损益。
- 辅助核算报表（客商/部门/项目）。
- 税务发票、价税分离。
- 合并报表。

## 3. 数据模型

### `gl_accounts`

- `id`, `tenant_id`, `code`, `name`, `type`, `direction`, `parent_id`, `level`, `is_leaf`, `status`, `created_at`, `updated_at`。
- `type`：`asset` / `liability` / `equity` / `revenue` / `expense`。
- `direction`：`debit` / `credit`。
- 唯一：`(tenant_id, code)`。

### `gl_periods`

- `id`, `tenant_id`, `year`, `month`, `start_date`, `end_date`, `status`, `closed_at`, `closed_by`。
- 唯一：`(tenant_id, year, month)`。
- 状态：`open` / `closed`。

### `gl_account_mappings`

- `tenant_id`, `key`, `account_code`。
- 默认映射：`ar=1122`、`ap=2202`、`revenue=6001`、`inventory=1405`、`cogs=6401`、`bank=1002`、`cash=1001`。

### `gl_vouchers`

- `id`, `tenant_id`, `org_id`, `code`, `voucher_date`, `period_id`, `voucher_type`, `source_type`, `source_id`, `source_code`, `status`, `memo`, `created_by`, `posted_by`, `posted_at`, `reversal_of_voucher_id`, `created_at`, `updated_at`。
- 状态：`draft` / `posted` / `reversed`。
- 自动凭证按 `(tenant_id, source_type, source_id)` 唯一，保证幂等。

### `gl_voucher_lines`

- `id`, `tenant_id`, `voucher_id`, `line_no`, `account_id`, `summary`, `debit`, `credit`, `partner_id`, `department_id`。
- 检查：借方/贷方非负，单行只能一边有金额，凭证借贷合计相等且大于 0。

## 4. 自动凭证规则

| 业务单据 | 借方 | 贷方 |
|---|---|---|
| 销售发货（收入） | 应收账款 1122 | 主营业务收入 6001 |
| 销售发货（成本） | 主营业务成本 6401 | 库存商品 1405 |
| 采购收货 | 库存商品 1405 | 应付账款 2202 |
| 客户收款 | 银行存款/现金 | 应收账款 1122 |
| 供应商付款 | 应付账款 2202 | 银行存款/现金 |

- 销售发货一张凭证包含收入 + 成本两组分录。
- 收付款按 `method` 选择银行/现金科目。
- 自动凭证直接 `posted`，来源单号可追溯。
- 业务事务内同步生成凭证，凭证失败则业务事务回滚。
- 期间不存在时自动创建 open 期间；期间已关账返回 `PERIOD_CLOSED`。

## 5. 手工凭证与期间

- 手工凭证：创建/编辑草稿、过账、反过账。
- 过账校验：借贷平衡、金额非负、科目末级且启用、期间 open。
- 反过账：生成反方向凭证，原凭证标记 `reversed`，保留审计轨迹。
- 期间关账前校验无草稿凭证；支持重新打开。

## 6. 报表

- 科目余额表：期初、本期借贷发生、期末余额。
- 总账/明细账：按科目查询凭证分录和余额。
- 资产负债表：资产、负债、权益，含未结转利润（累计）。
- 利润表：收入 - 费用 = 净利润。
- 统计 `posted` 和 `reversed` 凭证；原凭证保留在历史期间，反过账凭证在反过账期间抵消。应用租户/数据权限。

## 7. API 与权限

| 模块 | API |
|---|---|
| 科目 | `GET/POST /finance/accounts`、`PUT /finance/accounts/:id` |
| 期间 | `GET /finance/periods`、`POST /finance/periods/:id/close`、`/reopen` |
| 凭证 | `GET/POST /finance/vouchers`、`GET/PUT /finance/vouchers/:id`、`POST /:id/post`、`/:id/reverse` |
| 报表 | `GET /finance/reports/trial-balance`、`/general-ledger`、`/balance-sheet`、`/income-statement` |

权限：

- `finance.account.read/write`
- `finance.period.read/close`
- `finance.voucher.read/write/post/reverse`
- `finance.report.read`

## 8. 前端

财务菜单新增：

- 科目表：树形科目、维护、启用/停用。
- 会计期间：列表、关账/反关账。
- 凭证：列表、手工录入、详情、过账、反过账。
- 财务报表：科目余额表、明细账、资产负债表、利润表。

## 9. 测试

- Go 领域测试：凭证平衡、报表计算。
- API 集成测试：自动凭证、幂等、手工凭证、反过账、关账、报表平衡、数据权限、401/403。
- 前端完整度检查、类型检查、生产构建。
- Demo 回填已有业务单据凭证，展示报表。

## 10. 关键决策

- 自动凭证在业务事务内同步生成，保证业财一致。
- 自动凭证按来源唯一，重复回填/重试不重复生成。
- 增值税、现金流量表、损益结转后置，本期按单据总额入账。
