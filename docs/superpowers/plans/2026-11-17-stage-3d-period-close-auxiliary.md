# Stage 3D 期末结转与辅助核算 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Stage 3C 总账基础上实现期末损益结转、反关账、年度结转、辅助核算和辅助余额表。

**Architecture:** 扩展 `internal/modules/gl`。关账在期间行锁事务内汇总收入/费用余额，生成 `period_close` 凭证并写关账记录；反关账生成反向凭证。年度结转向 4104 结转 4103。凭证行增加项目辅助字段，报表增加辅助余额查询。

**Tech Stack:** Go 1.25 + Gin + pgx + PostgreSQL + shopspring/decimal；Vue 3 + TypeScript + Element Plus。

---

## 1. 迁移 000015

- `ALTER TABLE erp.gl_voucher_lines ADD COLUMN project_code text NOT NULL DEFAULT '';`
- `gl_period_closes`：关账记录。
- `gl_year_closes`：年度关账记录。
- `gl_account_mappings` 增加 `profit=4103`、`retained_earnings=4104`。
- 索引：period close、year close、project_code。

## 2. Domain

- `VoucherLine`/`VoucherLineInput` 增加 `ProjectCode`。
- `PeriodClose`、`YearClose` 结构。
- `PeriodRepository`：
  - `Close(ctx, tenantID, id, userID, voucherCode string) error`
  - `Reopen(ctx, tenantID, id, userID, reversalCode string) error`
  - `EnsureYear(ctx, tenantID, year int) error`
  - `CloseYear(ctx, tenantID, year int, orgID, userID, voucherCode string) error`
- `DimensionRepository`：客商、部门列表。
- `ReportRepository`：`AuxiliaryBalance`。
- 错误：`ErrYearNotClosed`。

## 3. 期末结转

`PeriodRepository.Close` 事务：

1. `FOR UPDATE` 锁期间；校验 open。
2. 校验无草稿凭证。
3. 查询本期收入/费用科目余额：
   - 排除 `period_close`、`period_close_reversal`、`year_close`。
4. 构造结转分录：
   - 收入余额（贷方）借收入、贷 4103；
   - 费用余额（借方）借 4103、贷费用；
   - 4103 轧差一行。
5. 插入 posted 凭证 `period_close`。
6. 插入 `gl_period_closes`。
7. 期间置 closed。

## 4. 反关账

1. `FOR UPDATE` 锁期间；校验 closed。
2. 找最近 closed 关账记录。
3. 有结转凭证则生成反向凭证 `period_close_reversal`，同期间日期。
4. 关账记录置 reopened，期间 open。
5. 无记录则直接 open。

## 5. 年度结转

1. 校验该年度 12 个期间全部 closed。
2. 校验无已有年度关账。
3. 查询 4103 余额（含期间结转，排除 year_close）。
4. 余额非零时生成次年 1 月 1 日凭证：4103 → 4104。
5. 插入 `gl_year_closes`。

## 6. 辅助核算

- 凭证行 `project_code`。
- `GET /finance/dimensions` 返回客商/部门。
- 辅助余额 SQL：按维度 + 科目汇总期初/本期/期末，排除结转凭证，应用报表范围（结转凭证放行）。
- 前端凭证行增加客商/部门/项目；报表增加辅助余额 Tab。

## 7. 报表范围

- 新增 `appendReportScope`：数据权限 OR 结转/年度结转凭证可见。
- 利润表、辅助余额排除结转类凭证。
- 科目余额表、资产负债表、明细账包含结转类凭证。

## 8. 测试

- `tests/api/stage3d_test.go`：
  - dimensions、辅助凭证、辅助余额。
  - 关账生成结转凭证，P&L 归零，4103 有余额。
  - 利润表排除结转。
  - 反关账反向凭证、余额恢复。
  - 再关账新凭证。
  - 年度结转测试通过环境变量启用（CI/干净库）。
  - 数据权限、401/403。
- 前端完整度检查、类型检查、构建。
- Stage 3B/3C 回归。

## 9. 文档

- `internal/modules/gl/README.md`
- `README.md`
- `db/migrations/README.md`
- `api/openapi.yaml`
- `docs/reports/2026-11-17-stage-3d-period-close-auxiliary.md`

## 10. Acceptance Criteria

- [ ] 关账自动生成损益结转凭证。
- [ ] 反关账生成反向凭证并恢复期间。
- [ ] 再次关账生成新结转凭证。
- [ ] 年度结转 4103 → 4104。
- [ ] 辅助核算客商/部门/项目可用。
- [ ] 辅助余额表可用。
- [ ] 利润表排除结转凭证。
- [ ] 报表在关账后保持平衡。
- [ ] 前端页面/SDK/路由/菜单完整。
- [ ] 自动化测试、静态检查、构建通过。
- [ ] 代码审核和汇总报告完成。

## 11. 审核修订

- 年度结转余额包含以前年度 `year_close`，并增加两年连续结转回归测试。
- 年度结转要求每个已关账期间有关账记录；无损益期间也写记录；已年度结转年度禁止反关账。
- 年度结转按年度顺序执行，增加 `YEAR_OUT_OF_ORDER`、`PERIOD_CLOSE_MISSING` 错误码。
- 报表严格应用组织数据范围，不再对结转凭证放行；补充部门/公司范围测试。
- 新增年度结转状态查询、前端状态展示和重复操作禁用。
- 辅助核算增加 UUID/长度校验、数据库外键和项目编码长度约束。
