# Stage 3D 期末结转与辅助核算设计

> 日期：2026-11-17
> 分支：`stage-3d-period-close-auxiliary`
> 状态：设计已确认

## 1. 目标

在 Stage 3C 总账基础上，补齐期末损益结转、反关账、年度结转和辅助核算：

```text
业务凭证 → 期末损益结转 → 本年利润 4103 → 年度结转 → 利润分配 4104
                ↓
        辅助余额表（客商/部门/项目）
```

## 2. 范围

### 本期

- 凭证行项目辅助 `project_code`。
- 辅助维度：客商、部门、项目编码。
- 辅助余额表。
- 关账自动生成损益结转凭证。
- 反关账生成反向结转凭证并恢复 open。
- 年度结转：4103 → 4104。
- 年度期间初始化。
- 前端：凭证辅助字段、期间年度结转、辅助余额表。

### 后置

- 现金流量表、税务/发票、多币种、项目主数据、辅助核算报表深化。

## 3. 数据模型

### 修改

- `gl_voucher_lines.project_code text NOT NULL DEFAULT ''`。

### 新增 `gl_period_closes`

- `id`, `tenant_id`, `period_id`, `voucher_id`（可空）, `reversal_voucher_id`（可空）,
  `status`（`closed`/`reopened`）, `created_by`, `created_at`, `reopened_by`, `reopened_at`。
- 每次关账一条记录；反关账标记最近一条记录为 `reopened`。

### 新增 `gl_year_closes`

- `id`, `tenant_id`, `year`, `voucher_id`（可空）, `status`, `created_by`, `created_at`。
- 唯一 `(tenant_id, year)`。

### 科目映射

- 新增 `profit=4103`，`retained_earnings=4104`。

## 4. 期末损益结转

关账事务内：

1. 锁定期间 `FOR UPDATE`。
2. 校验期间 open、无草稿凭证。
3. 汇总本期间收入/费用科目余额（排除结转/年度结转凭证）。
4. 生成结转凭证：
   - 收入类借方冲平，贷 4103；
   - 费用类贷方冲平，借 4103；
   - 4103 轧差为一行；
   - 无损益余额时不生成凭证。
5. 写 `gl_period_closes`，期间置 `closed`。
6. 结转凭证 `voucher_type=period_close`，来源为关账记录，已过账。

## 5. 反关账

1. 锁定期间 `FOR UPDATE`。
2. 找到最近一条 `closed` 关账记录。
3. 若有结转凭证，生成同期间的反向凭证 `period_close_reversal`。
4. 关账记录置 `reopened`，期间置 `open`。
5. 再次关账生成新的关账记录和新结转凭证；旧结转 + 反向凭证在报表中抵消。

## 6. 年度结转

- 要求该年度 12 个期间全部 `closed`。
- 在次年 1 月 1 日生成年度结转凭证：
  - 4103 贷方余额（盈利）：借 4103 / 贷 4104；
  - 4103 借方余额（亏损）：借 4104 / 贷 4103；
  - 余额为 0 时只写年度关账记录，不生成凭证。
- `voucher_type=year_close`，来源为年度关账记录，已过账。
- 本期不提供年度反结转。

## 7. 报表口径

- 科目余额表、资产负债表、明细账：包含损益结转、反结转、年度结转凭证。
- 利润表、辅助余额表：排除结转/反结转/年度结转凭证，保持经营口径。
- 结转凭证为租户级凭证，报表在数据权限过滤时对结转/年度结转凭证放行，避免关账后报表不平衡。
- 资产负债表权益包含 4103/4104；未结转利润仍按收入-费用计算。

## 8. 辅助核算

- 凭证行支持 `partner_id`、`department_id`、`project_code`。
- 自动凭证带入客商；部门/项目暂不自动带入。
- `GET /finance/dimensions` 返回客商和部门选项。
- 辅助余额表按客商/部门/项目分组，展示期初、本期借/贷、期末。

## 9. API

| 方法 | 路径 | 权限 |
|---|---|---|
| GET | `/finance/dimensions` | `finance.voucher.read` |
| GET | `/finance/reports/auxiliary-balance` | `finance.report.read` |
| POST | `/finance/periods/ensure` | `finance.period.close` |
| POST | `/finance/years/:year/close` | `finance.period.close` |

现有 `POST /finance/periods/:id/close` 和 `/reopen` 改为结转语义。

## 10. 前端

- 凭证行增加客商、部门、项目字段。
- 期间页面增加“创建年度期间”“年度结转”。
- 关账确认提示生成损益结转凭证。
- 财务报表增加“辅助余额表” Tab。

## 11. 测试

- 辅助余额：按客商/部门/项目汇总。
- 关账：生成结转凭证，P&L 科目余额归零，4103 体现利润。
- 利润表排除结转凭证。
- 反关账：生成反向凭证，期间恢复 open，P&L 余额恢复。
- 再次关账：生成新结转凭证。
- 年度结转：4103 → 4104（CI 使用独立年度/干净库）。
- 数据权限、401/403、前端完整度、构建。
