# Stage 3A 采购与库存闭环 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现贸易/分销 ERP 的第一条业务闭环：采购订单 → 审批 → 收货 → 库存余额/流水，并完成前端页面、接口测试、代码审核和汇总报告。

**Architecture:** 在 `internal/modules/purchase` 中实现采购订单/收货单的领域、应用、PostgreSQL 仓储和 HTTP；新增 `internal/modules/inventory` 提供库存余额/流水查询。收货在一个数据库事务中完成：写收货单、更新订单行已收数量、更新订单状态、upsert 库存余额、写库存流水。

**Tech Stack:** Go 1.25 + Gin + pgx + PostgreSQL + shopspring/decimal；Vue 3 + TypeScript + Element Plus。

---

## 范围

### 采购

- 采购订单头/行：供应商、仓库、币种、订单日期、状态、金额、备注。
- 行：物料、数量、单价、金额、已收数量。
- 状态机：`draft → submitted → approved → partially_received → received → closed`；`cancelled` 可从 draft/submitted/approved（无收货）进入。
- 操作：创建、编辑（draft）、提交、审批、取消、收货。
- 收货单头/行：收货单号、采购订单、仓库、收货日期、备注、行数量。
- 部分收货、超收校验、重复收货校验。

### 库存

- 库存余额：租户/组织/仓库/物料维度，`on_hand`、`reserved`、`available`。
- 库存流水：入库/出库方向、业务类型、业务单据、数量、单位成本、金额。
- 本阶段只做采购入库；销售出库、调拨、盘点后续阶段。

### 权限

- `purchase.order.read`
- `purchase.order.write`
- `purchase.order.approve`
- `purchase.receipt.read`
- `inventory.balance.read`
- `inventory.transaction.read`

### 编码规则

新增默认规则 `purchase_receipt`：`GRN-yyyyMMdd-0001`。

### 前端

- 采购订单列表、创建、详情、提交、审批、收货。
- 库存余额列表。
- 库存流水列表。
- 菜单与权限路由。

---

## 数据库迁移 000006

```sql
CREATE TABLE erp.purchase_orders (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants(id),
    org_id uuid NOT NULL,
    code text NOT NULL,
    supplier_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    order_date date NOT NULL,
    currency text NOT NULL DEFAULT 'CNY',
    status text NOT NULL DEFAULT 'draft',
    total_amount numeric(19,4) NOT NULL DEFAULT 0,
    remark text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, code)
);

CREATE TABLE erp.purchase_order_lines (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    order_id uuid NOT NULL REFERENCES erp.purchase_orders(id) ON DELETE CASCADE,
    line_no integer NOT NULL,
    material_id uuid NOT NULL,
    quantity numeric(19,6) NOT NULL,
    unit_price numeric(19,6) NOT NULL,
    amount numeric(19,4) NOT NULL,
    received_qty numeric(19,6) NOT NULL DEFAULT 0,
    UNIQUE (order_id, line_no)
);

CREATE TABLE erp.purchase_receipts (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    code text NOT NULL,
    order_id uuid NOT NULL REFERENCES erp.purchase_orders(id),
    warehouse_id uuid NOT NULL,
    receipt_date date NOT NULL,
    status text NOT NULL DEFAULT 'posted',
    remark text NOT NULL DEFAULT '',
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, code)
);

CREATE TABLE erp.purchase_receipt_lines (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    receipt_id uuid NOT NULL REFERENCES erp.purchase_receipts(id) ON DELETE CASCADE,
    order_line_id uuid NOT NULL REFERENCES erp.purchase_order_lines(id),
    material_id uuid NOT NULL,
    quantity numeric(19,6) NOT NULL,
    unit_price numeric(19,6) NOT NULL,
    amount numeric(19,4) NOT NULL
);

CREATE TABLE erp.inventory_balances (
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    material_id uuid NOT NULL,
    on_hand numeric(19,6) NOT NULL DEFAULT 0,
    reserved numeric(19,6) NOT NULL DEFAULT 0,
    available numeric(19,6) NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, warehouse_id, material_id)
);

CREATE TABLE erp.inventory_transactions (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    material_id uuid NOT NULL,
    direction text NOT NULL,
    biz_type text NOT NULL,
    biz_id uuid NOT NULL,
    biz_code text NOT NULL DEFAULT '',
    quantity numeric(19,6) NOT NULL,
    unit_cost numeric(19,6) NOT NULL,
    amount numeric(19,4) NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid NOT NULL
);
```

索引：

- `purchase_orders (tenant_id, org_id, status, order_date)`、`(tenant_id, code)`。
- `purchase_order_lines (tenant_id, order_id)`、`(tenant_id, material_id)`。
- `purchase_receipts (tenant_id, order_id, receipt_date)`。
- `inventory_balances (tenant_id, material_id)`、`(tenant_id, warehouse_id)`。
- `inventory_transactions (tenant_id, warehouse_id, material_id, occurred_at)`。

权限和默认编码规则写入迁移/seed。

---

## 文件结构

```text
internal/modules/purchase/
├─ domain/
│  ├─ order.go
│  ├─ receipt.go
│  ├─ status.go
│  └─ repository.go
├─ app/
│  ├─ order_service.go
│  └─ receipt_service.go
├─ infra/postgres/
│  ├─ order_repository.go
│  └─ inventory_writer.go
└─ http/
   ├─ order_handler.go
   └─ dto.go

internal/modules/inventory/
├─ domain/balance.go
├─ app/query_service.go
├─ infra/postgres/query_repository.go
└─ http/query_handler.go
```

---

## Task 1: 迁移与权限

- 创建 `000006_purchase_inventory.up/down.sql`。
- 新增 6 个权限。
- `cmd/seed` 增加 `purchase_receipt` 默认编码规则。
- 迁移验证：up/down/up。

## Task 2: 采购领域

- `PurchaseOrder`、`PurchaseOrderLine`、`PurchaseReceipt`、`PurchaseReceiptLine`。
- 金额/数量使用 `decimal.Decimal`。
- 状态机方法：`Submit`、`Approve`、`Cancel`、`Receive`。
- 行剩余可收数量、超收校验。
- 领域测试：状态迁移、金额计算、超收、取消限制。

## Task 3: 采购仓储与事务

- `OrderRepository`：
  - `Create(ctx, order, lines)`
  - `Get(ctx, filter, tenantID, id)`
  - `List(ctx, filter, tenantID, status, keyword, offset, limit)`
  - `UpdateDraft`
  - `UpdateStatus`
  - `Receive(ctx, filter, receipt, receiptLines, order, lines)`：一个事务内完成收货、库存、流水。
- 事务内 `SELECT ... FOR UPDATE` 锁定采购订单行。
- 库存 upsert：
```sql
INSERT INTO erp.inventory_balances (...) VALUES (...)
ON CONFLICT (tenant_id, warehouse_id, material_id)
DO UPDATE SET on_hand = inventory_balances.on_hand + EXCLUDED.on_hand,
              available = inventory_balances.available + EXCLUDED.available,
              updated_at = now();
```
- 收货单号使用 `CodeRuleService.Next`。
- 数据权限应用到订单列表/详情。

## Task 4: 采购应用服务

- `CreateOrder`：校验供应商、仓库、物料为 active；计算行金额和总金额；生成订单号；写入订单/行。
- `UpdateDraft`：仅 draft 可编辑；重算金额。
- `Submit`、`Approve`、`Cancel`。
- `Receive`：仅 approved/partially_received；校验行数量和剩余量；生成收货单；事务写入。
- `ListOrders`、`GetOrder`、`ListReceipts`。

## Task 5: 库存查询

- `ListBalances(filter, tenantID, warehouseID, materialID, keyword, page, pageSize)`。
- `ListTransactions(filter, tenantID, warehouseID, materialID, page, pageSize)`。
- 关联物料/仓库名称返回。
- 数据权限应用到余额/流水。

## Task 6: HTTP API

| 方法 | 路径 | 权限 |
|---|---|---|
| GET/POST | `/purchase/orders` | `purchase.order.read/write` |
| GET/PUT | `/purchase/orders/:id` | `purchase.order.read/write` |
| POST | `/purchase/orders/:id/submit` | `purchase.order.write` |
| POST | `/purchase/orders/:id/approve` | `purchase.order.approve` |
| POST | `/purchase/orders/:id/cancel` | `purchase.order.write` |
| POST | `/purchase/orders/:id/receive` | `purchase.order.write` |
| GET | `/purchase/receipts` | `purchase.receipt.read` |
| GET | `/inventory/balances` | `inventory.balance.read` |
| GET | `/inventory/transactions` | `inventory.transaction.read` |

金额/数量使用字符串传输。错误码：400 参数/状态错误、403 权限、404 不存在或越权、409 状态冲突/重复单号。

## Task 7: 前端

- `PurchaseOrderListView.vue`：
  - 列表：单号、供应商、仓库、日期、金额、状态。
  - 新建：供应商、仓库、日期、币种、备注、明细行（物料、数量、单价）。
  - 详情：明细、已收数量、操作按钮。
  - 收货：填写本次收货数量，提交后刷新。
- `InventoryBalanceView.vue`：仓库、物料、在手、可用。
- `InventoryTransactionView.vue`：时间、仓库、物料、方向、数量、业务单号。
- 路由/菜单权限。

## Task 8: Demo 数据

- seed 增加 `purchase_receipt` 编码规则。
- `cmd/demo` 增加：
  - 2 张采购订单（1 张已审批待收货，1 张已全部收货）。
  - 对应库存余额和流水。
  - 可重复执行。

## Task 9: 测试与报告

- `go test ./...`、`go vet ./...`、`gofmt -l`。
- `pnpm typecheck:web`、`pnpm build:web`。
- 真实 PostgreSQL API 测试：
  - 创建订单 → 提交 → 审批 → 部分收货 → 库存余额/流水 → 再收货 → 全部收货。
  - 超收 400、重复收货/非法状态 409。
  - 数据权限：demo_dept 看不到 HQ 订单/库存。
  - 无权限 403、无 token 401。
- 代码审核。
- `docs/reports/2026-11-16-stage-3a-purchase-inventory.md`。
- 用户审阅后进入 Stage 3B。

---

## Acceptance Criteria

- [ ] 采购订单 CRUD/提交/审批/取消可用。
- [ ] 部分收货、全部收货、超收校验可用。
- [ ] 收货后库存余额和库存流水正确。
- [ ] 重复收货/非法状态返回冲突。
- [ ] 数据权限应用到采购订单和库存查询。
- [ ] 前端采购订单、库存余额、库存流水页面可用。
- [ ] demo 数据包含采购订单和库存。
- [ ] 静态检查、构建、真实 API 测试通过。
- [ ] 代码审核和汇总报告完成。
