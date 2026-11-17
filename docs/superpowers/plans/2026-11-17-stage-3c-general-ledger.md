# Stage 3C 总账与基础报表 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Stage 3B 财务应收应付之上，实现会计科目、期间、凭证、自动业财凭证、总账/明细账、科目余额表、资产负债表和利润表。

**Architecture:** 新增 `internal/modules/gl`，按 `domain/app/infra/http` 分层。自动凭证在业务事务内同步写入 `gl_vouchers/gl_voucher_lines`，按 `(tenant_id, source_type, source_id)` 幂等。报表从已过账凭证实时聚合，应用租户和数据权限。

**Tech Stack:** Go 1.25 + Gin + pgx + PostgreSQL + shopspring/decimal；Vue 3 + TypeScript + Element Plus。

---

## 1. 数据库迁移 000013

### 1.1 表

```sql
CREATE TABLE erp.gl_accounts (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    code text NOT NULL,
    name text NOT NULL,
    type text NOT NULL,
    direction text NOT NULL,
    parent_id uuid,
    level integer NOT NULL DEFAULT 1,
    is_leaf boolean NOT NULL DEFAULT true,
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT gl_accounts_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT gl_accounts_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT gl_accounts_type_check CHECK (type IN ('asset','liability','equity','revenue','expense')),
    CONSTRAINT gl_accounts_direction_check CHECK (direction IN ('debit','credit'))
);

CREATE TABLE erp.gl_periods (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    year integer NOT NULL,
    month integer NOT NULL,
    start_date date NOT NULL,
    end_date date NOT NULL,
    status text NOT NULL DEFAULT 'open',
    closed_at timestamptz,
    closed_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT gl_periods_tenant_year_month_key UNIQUE (tenant_id, year, month),
    CONSTRAINT gl_periods_month_check CHECK (month BETWEEN 1 AND 12),
    CONSTRAINT gl_periods_status_check CHECK (status IN ('open','closed'))
);

CREATE TABLE erp.gl_account_mappings (
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    key text NOT NULL,
    account_code text NOT NULL,
    PRIMARY KEY (tenant_id, key)
);

CREATE TABLE erp.gl_vouchers (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    code text NOT NULL,
    voucher_date date NOT NULL,
    period_id uuid NOT NULL,
    voucher_type text NOT NULL,
    source_type text NOT NULL DEFAULT 'manual',
    source_id uuid,
    source_code text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft',
    memo text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    posted_by uuid,
    posted_at timestamptz,
    reversal_of_voucher_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT gl_vouchers_tenant_code_key UNIQUE (tenant_id, code),
    CONSTRAINT gl_vouchers_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT gl_vouchers_period_fk FOREIGN KEY (tenant_id, period_id)
        REFERENCES erp.gl_periods (tenant_id, id),
    CONSTRAINT gl_vouchers_status_check CHECK (status IN ('draft','posted','reversed'))
);

CREATE UNIQUE INDEX gl_vouchers_source_key
    ON erp.gl_vouchers (tenant_id, source_type, source_id)
    WHERE source_id IS NOT NULL AND source_type <> 'manual' AND source_type <> 'reversal';

CREATE TABLE erp.gl_voucher_lines (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    voucher_id uuid NOT NULL,
    line_no integer NOT NULL,
    account_id uuid NOT NULL,
    summary text NOT NULL DEFAULT '',
    debit numeric(19,4) NOT NULL DEFAULT 0,
    credit numeric(19,4) NOT NULL DEFAULT 0,
    partner_id uuid,
    department_id uuid,
    CONSTRAINT gl_voucher_lines_voucher_fk FOREIGN KEY (tenant_id, voucher_id)
        REFERENCES erp.gl_vouchers (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT gl_voucher_lines_line_key UNIQUE (voucher_id, line_no),
    CONSTRAINT gl_voucher_lines_amount_check CHECK (
        debit >= 0 AND credit >= 0 AND ((debit > 0 AND credit = 0) OR (credit > 0 AND debit = 0))
    )
);
```

索引：voucher_date、period、source、account、org。

### 1.2 初始化

- 权限 8 个。
- 编码规则 `gl_voucher`：`GL-yyyyMM-0001`，月重置。
- 为所有租户插入标准科目（至少 20 个）。
- 插入默认科目映射。
- 为所有租户创建当前年度 12 个会计期间。

## 2. 文件结构

```text
internal/modules/gl/
├─ domain/
│  ├─ account.go
│  ├─ period.go
│  ├─ voucher.go
│  ├─ report.go
│  ├─ errors.go
│  └─ repository.go
├─ app/
│  ├─ common.go
│  ├─ account_service.go
│  ├─ period_service.go
│  ├─ voucher_service.go
│  ├─ report_service.go
│  └─ backfill_service.go
├─ infra/postgres/
│  ├─ common.go
│  ├─ account_repository.go
│  ├─ period_repository.go
│  ├─ voucher_repository.go
│  ├─ report_repository.go
│  └─ posting.go
└─ http/
   └─ handler.go

cmd/glbackfill/main.go
tests/api/stage3c_test.go
```

## 3. Task 1：迁移、科目、期间

- 创建 `000013_general_ledger.up/down.sql`。
- `cmd/seed` 增加权限、编码规则、科目、映射、当前年度期间。
- `gl/domain` 定义科目、期间、凭证、报表、错误。
- `gl/infra/postgres/account_repository.go`、`period_repository.go`。
- `gl/app/account_service.go`、`period_service.go`。
- 科目 CRUD：列表、创建、更新、启用/停用；父科目自动设为非末级。
- 期间：列表、关账（无草稿凭证）、重新打开。

## 4. Task 2：凭证与自动过账

- `gl/infra/postgres/voucher_repository.go`：
  - 创建草稿/已过账凭证。
  - 列表、详情、更新草稿、过账、反过账。
  - 借贷平衡校验。
- `gl/app/voucher_service.go`：
  - 手工凭证创建/编辑/过账/反过账。
  - 编码 `gl_voucher`。
- `gl/infra/postgres/posting.go`：
  - `PostSalesShipment`：Dr AR / Cr Revenue；Dr COGS / Cr Inventory。
  - `PostPurchaseReceipt`：Dr Inventory / Cr AP。
  - `PostPayment`：收款 Dr Bank/Cash / Cr AR；付款 Dr AP / Cr Bank/Cash。
  - 按来源幂等，期间自动创建/校验。
- 业务集成：
  - `sales`：`Shipment` 增加 `VoucherCode`；发货服务生成 `gl_voucher`；仓储事务内调用 `PostSalesShipment`。
  - `purchase`：`Receipt` 增加 `VoucherCode`；收货服务生成；仓储事务内调用 `PostPurchaseReceipt`。
  - `finance`：`Payment` 增加 `VoucherCode`；创建服务生成；支付仓储改为事务并调用 `PostPayment`。

## 5. Task 3：报表

- `gl/infra/postgres/report_repository.go`：
  - 科目余额表：期初、本期借/贷、期末。
  - 总账/明细账：按科目列出分录和余额。
  - 资产负债表：资产/负债/权益 + 本年利润。
  - 利润表：收入/费用/净利润。
- 所有查询：只统计 `posted` 且 `reversal_of_voucher_id IS NULL`；应用租户和数据权限。
- `gl/app/report_service.go` 和 HTTP。

## 6. Task 4：HTTP 与权限

| 方法 | 路径 | 权限 |
|---|---|---|
| GET/POST | `/finance/accounts` | `finance.account.read/write` |
| PUT | `/finance/accounts/:id` | `finance.account.write` |
| GET | `/finance/periods` | `finance.period.read` |
| POST | `/finance/periods/:id/close` | `finance.period.close` |
| POST | `/finance/periods/:id/reopen` | `finance.period.close` |
| GET/POST | `/finance/vouchers` | `finance.voucher.read/write` |
| GET/PUT | `/finance/vouchers/:id` | `finance.voucher.read/write` |
| POST | `/finance/vouchers/:id/post` | `finance.voucher.post` |
| POST | `/finance/vouchers/:id/reverse` | `finance.voucher.reverse` |
| GET | `/finance/reports/trial-balance` | `finance.report.read` |
| GET | `/finance/reports/general-ledger` | `finance.report.read` |
| GET | `/finance/reports/balance-sheet` | `finance.report.read` |
| GET | `/finance/reports/income-statement` | `finance.report.read` |

错误码：400 参数/借贷不平、403 权限、404 不存在、409 期间关账/状态冲突。

## 7. Task 5：前端与 SDK

- `packages/sdk/src/gl.ts`：科目、期间、凭证、报表 API。
- `apps/web/src/views/finance/AccountListView.vue`
- `apps/web/src/views/finance/PeriodListView.vue`
- `apps/web/src/views/finance/VoucherListView.vue`
- `apps/web/src/views/finance/ReportView.vue`
- 路由 `/finance/accounts`、`/finance/periods`、`/finance/vouchers`、`/finance/reports`。
- 菜单：科目表、会计期间、凭证、财务报表。
- 权限守卫。

## 8. Task 6：Demo 与回填

- `cmd/glbackfill`：为已有销售发货、采购收货、收付款补生成凭证，幂等。
- `cmd/demo` 调用回填，确保 Demo 报表有数据。
- Demo 数据：现有 Stage 3B 单据 + 自动凭证。

## 9. Task 7：自动化接口测试

- `tests/api/stage3c_test.go`：
  - 自动凭证：采购收货、销售发货、收款、付款。
  - 幂等：回填/重试不重复。
  - 手工凭证：创建、过账、反过账。
  - 关账：关账后过账 409，重新打开后成功。
  - 科目余额表借贷平衡。
  - 资产负债表：资产 = 负债 + 权益 + 本年利润。
  - 利润表：净利润 = 收入 - 费用。
  - 数据权限：demo_dept 看不到 HQ 凭证/报表。
  - 401/403。
- 前端完整度检查、类型检查、构建。
- CI 集成测试运行 Stage 3B + 3C。

## 10. Task 8：文档与报告

- `README.md`、`internal/modules/gl/README.md`、`db/migrations/README.md`。
- `api/openapi.yaml` 增加总账接口。
- `docs/reports/2026-11-17-stage-3c-general-ledger.md`。
- 代码审核、修复、回归测试。

## 11. Acceptance Criteria

- [ ] 科目表、期间可用。
- [ ] 手工凭证创建/过账/反过账可用。
- [ ] 采购收货、销售发货、收付款自动生成凭证。
- [ ] 自动凭证幂等、来源可追溯。
- [ ] 科目余额表、明细账、资产负债表、利润表可用。
- [ ] 关账期间禁止过账。
- [ ] 数据权限和错误码正确。
- [ ] 前端页面、路由、菜单、SDK 完整。
- [ ] Demo 报表有数据。
- [ ] 自动化接口测试、静态检查、构建通过。
- [ ] 代码审核和汇总报告完成。
