# Stage 3H 销售退货与贷项单 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 增加销售退货闭环：按原发货行退货、原成本回库、冲收入/销项税/应收、生成贷项单并冲原发票未收金额。

**Architecture:** 新增销售退货单/行和财务贷项单；发货行累计 `returned_qty`；退货事务内完成数量校验、库存回库、退货凭证、贷项单和发票 credited 更新；发票未收金额和期间外币重估统一扣减 credited。

**Tech Stack:** Go 1.25 + Gin + pgx + PostgreSQL + shopspring/decimal；Vue 3 + TypeScript + Element Plus。

**Design:** `docs/superpowers/specs/2026-11-18-stage-3h-sales-returns-design.md`

---

## Task 1: 迁移、表、权限和编码规则

**Files:**
- Create: `db/migrations/000020_sales_returns.up.sql`
- Create: `db/migrations/000020_sales_returns.down.sql`
- Modify: `db/migrations/README.md`
- Modify: `cmd/seed/main.go`

- [ ] **Step 1: 写 up 迁移**

包含：

- `sales_shipment_lines.returned_qty numeric(19,6) NOT NULL DEFAULT 0` + 检查。
- `sales_returns`、`sales_return_lines`。
- `finance_credit_notes`。
- `finance_invoices.credited_amount`、`base_credited_amount` + 检查 `paid_amount + credited_amount <= total_amount`。
- 索引和唯一约束。
- 编码规则：
  - `sales_return` prefix `SR`
  - `finance_credit_note` prefix `CN`
- 权限：
  - `sales.return.read`
  - `sales.return.write`
  - 授予 admin 角色。

关键列和约束按设计 §3。贷项单表先创建，`sales_returns.credit_note_id` 可在贷项单创建后更新，避免循环外键。

- [ ] **Step 2: 写 down 迁移**

删除新增表、列、权限、编码规则；数据修复列回滚需评估。

- [ ] **Step 3: seed**

`cmd/seed/main.go` 增加编码规则和权限，并在标准科目/映射不变的情况下确保新权限存在。

- [ ] **Step 4: 验证**

Run: `go run ./cmd/migrate -direction up && go run ./cmd/seed`

Expected: 迁移/seed 成功；`\d erp.sales_returns`、`\d erp.finance_credit_notes` 存在。

- [ ] **Step 5: Commit**

```bash
git add db/migrations/000020_sales_returns.up.sql db/migrations/000020_sales_returns.down.sql db/migrations/README.md cmd/seed/main.go
git commit -m "feat(stage-3h): add sales return schema"
```

---

## Task 2: 财务贷项单、发票 credited 和未收金额

**Files:**
- Create: `internal/modules/finance/domain/credit_note.go`
- Modify: `internal/modules/finance/domain/invoice.go`
- Modify: `internal/modules/finance/domain/repository.go`
- Create: `internal/modules/finance/app/credit_note_service.go`
- Create: `internal/modules/finance/infra/postgres/credit_note_repository.go`
- Modify: `internal/modules/finance/infra/postgres/invoice_repository.go`
- Modify: `internal/modules/finance/infra/postgres/payment_repository.go`
- Modify: `internal/modules/finance/http/handler.go`
- Modify: `internal/modules/gl/infra/postgres/period_repository.go`
- Test: `internal/modules/finance/domain/credit_note_test.go`

- [ ] **Step 1: 域模型**

`CreditNote`：

- ID、TenantID、OrgID、Code、CreditType、SourceType、SourceID、SourceCode、InvoiceID、PartnerID、NoteDate、Currency、ExchangeRate、Amount、BaseAmount、AppliedAmount、AppliedBaseAmount、RemainingAmount、RemainingBaseAmount、Status、Reason、Remark、CreatedBy、CreatedAt、UpdatedAt。
- `Remaining()`/`Outstanding()` 辅助。
- 状态常量 `open`、`applied`。

`Invoice` 增加：

- `CreditedAmount`、`BaseCreditedAmount`。
- `Outstanding()` = `TotalAmount - PaidAmount - CreditedAmount`。
- 状态重算辅助 `StatusAfterSettlement()`。

- [ ] **Step 2: 仓储**

`CreditNoteRepository`：

- List/Get/Create.
- `ApplyToInvoiceTx(ctx, tx, tenantID, creditNoteID, invoiceID, userID)`：锁定贷项单/发票，计算 applied，更新发票 credited、状态和贷项单 applied/remaining/status。
- 本期创建退货时自动调用；列表/详情只读。

`InvoiceRepository`：

- Select/scan 增加 credited 字段。
- 列表/详情响应增加 credited。
- 未收金额使用 Outstanding。

`PaymentRepository`：

- 锁发票时读取 credited。
- outstanding = total - paid - credited。
- 更新发票时 `paid_amount + credited_amount + allocation <= total_amount`。
- 状态重算考虑 credited。

`period_repository.go` 外币重估：

- 查询 `credited_amount`、`base_credited_amount`。
- 未核销原币 = total - paid - credited。
- 账面本位币 = base_total - base_paid - base_credited + base_revalued。
- 发票全冲/全退不再重估。

- [ ] **Step 3: 服务/HTTP**

- `GET /finance/credit-notes`
- `GET /finance/credit-notes/:id`
- 权限 `finance.invoice.read`。
- 响应字段：id、code、credit_type、source_type/source_id/source_code、invoice_id、partner_id/name、note_date、currency、exchange_rate、amount、base_amount、applied_amount、applied_base_amount、remaining_amount、remaining_base_amount、status、reason、remark、created_at。
- 发票响应增加 `credited_amount`、`base_credited_amount`，`outstanding_amount` 扣减 credited。

- [ ] **Step 4: 测试/编译**

Run: `go test ./internal/modules/finance/... ./internal/modules/gl/... -count=1`

Expected: 通过；现有付款/发票测试需按 credited=0 保持原行为。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/finance internal/modules/gl/infra/postgres/period_repository.go
git commit -m "feat(stage-3h): add credit notes and invoice credited amounts"
```

---

## Task 3: 销售退货域模型、计算和仓储

**Files:**
- Create: `internal/modules/sales/domain/return.go`
- Create: `internal/modules/sales/domain/return_test.go`
- Modify: `internal/modules/sales/domain/repository.go`
- Create: `internal/modules/sales/infra/postgres/return_repository.go`
- Modify: `internal/modules/sales/infra/postgres/repository.go`
- Modify: `internal/modules/gl/domain/voucher.go`
- Modify: `internal/modules/gl/infra/postgres/posting.go`

- [ ] **Step 1: 域模型和计算**

`SalesReturn`、`SalesReturnLine` 按设计 §3.2。

`BuildLines(shipment, inputs)`：

- 校验 shipment line 存在、quantity > 0、`quantity + returned_qty <= shipment_line.quantity`。
- 比例计算净额/税额/含税额/本位币/成本，4 位舍入。
- 表头合计。

单元测试：

- 部分退货比例。
- 全额退货金额一致。
- 零税、零成本。
- 超退错误。

- [ ] **Step 2: Repository Create**

事务流程：

1. 锁发货、发货行、原发票。
2. 校验 customer/currency/warehouse。
3. 计算退货行。
4. 生成退货单号 `sales_return`、贷项单号 `finance_credit_note`、凭证号 `gl_voucher`。
5. 插入 `sales_returns`、`sales_return_lines`。
6. 更新 `sales_shipment_lines.returned_qty += qty`。
7. 库存回库：锁余额/更新 on_hand/available/avg_cost，插入 `inventory_transactions`（in/sales_return）。
8. 创建贷项单并自动冲原发票（调用财务贷项单逻辑或在本事务内实现）。
9. 生成 `sales_return` 会计凭证：
   - 借 revenue / 借 output_vat / 贷 ar
   - 借 inventory / 贷 cogs
10. 更新退货单 credit_note_id/voucher_code。
11. Commit。

- [ ] **Step 3: GL posting**

`PostSalesReturn`：

- 使用 base net/tax/gross/cost。
- 零金额行省略。
- source type/voucher type `sales_return`。

- [ ] **Step 4: 测试/编译**

Run: `go test ./internal/modules/sales/... ./internal/modules/gl/... -count=1`

Expected: 通过。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/sales internal/modules/gl/domain/voucher.go internal/modules/gl/infra/postgres/posting.go
git commit -m "feat(stage-3h): create sales returns with inventory and GL"
```

---

## Task 4: 销售退货 API、发货可退数量和 SDK/OpenAPI

**Files:**
- Modify: `internal/modules/sales/app/order_service.go`
- Create: `internal/modules/sales/app/return_service.go`
- Modify: `internal/modules/sales/http/handler.go`
- Modify: `internal/modules/sales/infra/postgres/repository.go`
- Modify: `cmd/api/main.go`
- Modify: `packages/sdk/src/sales.ts`
- Modify: `packages/sdk/src/finance.ts`
- Modify: `api/openapi.yaml`
- Modify: `scripts/test/check-frontend.mjs`

- [ ] **Step 1: 服务**

`ReturnService`：

- CreateForShipment(ctx, shipmentID, input).
- List/Get.
- 解析/校验日期、数量、原因。

- [ ] **Step 2: HTTP**

- `POST /sales/shipments/:id/returns` permission `sales.return.write`
- `GET /sales/returns` permission `sales.return.read`
- `GET /sales/returns/:id` permission `sales.return.read`
- 发货响应增加 `returned_qty`、`returnable_qty`。
- 错误映射：`OVER_RETURN` 409、`PERIOD_CLOSED` 409、404、400、403。

- [ ] **Step 3: SDK/OpenAPI**

- SalesReturn 类型、CreateSalesReturnInput、returnApi.
- CreditNote 类型、financeApi.creditNotes.list/get.
- OpenAPI 路径/schema，版本 0.9.0。
- 完整度检查增加退货/贷项单字符串。

- [ ] **Step 4: 验证**

Run: `go build ./... && pnpm typecheck:web && npx -y yaml@2 valid api/openapi.yaml`

- [ ] **Step 5: Commit**

```bash
git add internal/modules/sales cmd/api/main.go packages/sdk api/openapi.yaml scripts/test/check-frontend.mjs
git commit -m "feat(stage-3h): add sales return APIs"
```

---

## Task 5: 前端退货、贷项单和发票展示

**Files:**
- Modify: `apps/web/src/views/sales/ShipmentListView.vue`
- Create: `apps/web/src/views/sales/SalesReturnListView.vue`
- Create: `apps/web/src/views/finance/CreditNoteListView.vue`
- Modify: `apps/web/src/views/finance/InvoiceListView.vue`
- Modify: `apps/web/src/router/index.ts`
- Modify: `apps/web/src/layouts/DefaultLayout.vue`
- Modify: `scripts/test/check-frontend.mjs`

- [ ] **Step 1: 发货退货**

- 发货列表增加“退货”按钮。
- 退货弹窗：日期、原因、备注、行可退数量、数量输入、金额/成本预览。
- 发货详情显示已退/可退数量。

- [ ] **Step 2: 退货页面**

- 列表：退货单号、发货单、客户、日期、金额、成本、贷项状态。
- 详情：行、贷项已冲/剩余、凭证号、原发票。

- [ ] **Step 3: 贷项单和发票**

- 贷项单列表/详情页面，只读。
- 发票列表/详情增加已冲贷项、未收金额；`paid` 显示“已结清”。
- 路由/菜单/权限。

- [ ] **Step 4: 验证**

Run: `pnpm typecheck:web && pnpm test:frontend && pnpm build:web`

- [ ] **Step 5: Commit**

```bash
git add apps/web scripts/test/check-frontend.mjs
git commit -m "feat(stage-3h): add sales return and credit note UI"
```

---

## Task 6: Stage 3H API 测试、脚本和 CI

**Files:**
- Create: `tests/api/stage3h_test.go`
- Create: `scripts/test/stage3h-api.ps1`
- Modify: `package.json`
- Modify: `README.md`
- Modify: `scripts/test/check-frontend.mjs`
- Modify: `.github/workflows/ci.yml` if needed

- [ ] **Step 1: 测试**

覆盖：

- 创建销售订单/发货，部分退货：
  - 退货金额/成本比例。
  - 库存余额/流水增加，平均成本重算。
  - 凭证借贷。
  - 发货 returned_qty/returnable_qty。
  - 贷项单 applied/remaining。
  - 发票 credited/outstanding/status。
- 全额退货：金额与原发货一致，发票结清。
- 超退 409。
- 已收款发票退货：贷项单全部 open。
- 多行、零税、外币发货退货。
- 退货/贷项单列表/详情。
- 期间外币重估扣减 credited。
- 权限 401/403、数据范围、非法数量/日期。
- Stage 3B–3H 回归。

- [ ] **Step 2: 脚本/package/README**

复制 stage3g 脚本为 stage3h，`TestStage3H`；package 增加 `test:api:stage3h`；README 增加命令；check-frontend 增加断言。

- [ ] **Step 3: 验证**

Run: `pnpm test:api:stage3h`，干净库 Stage 3B–3H 全链路。

- [ ] **Step 4: Commit**

```bash
git add tests/api/stage3h_test.go scripts/test/stage3h-api.ps1 package.json README.md scripts/test/check-frontend.mjs .github/workflows/ci.yml
git commit -m "test(stage-3h): add sales return API tests"
```

---

## Task 7: 文档、静态检查、报告草稿

**Files:**
- Modify: `README.md`
- Modify: `internal/modules/sales/README.md`
- Modify: `internal/modules/finance/README.md`
- Modify: `internal/modules/gl/README.md`
- Modify: `db/migrations/README.md`
- Create: `docs/reports/2026-11-18-stage-3h-sales-returns.md`

- [ ] **Step 1: 文档**

补充退货流程、贷项单、发票 credited、期间重估、限制。

- [ ] **Step 2: 静态检查**

```bash
gofmt -w ./cmd ./internal ./db ./tests
gofmt -l ./cmd ./internal ./db ./tests
go test ./...
go vet ./...
pnpm typecheck:web
pnpm test:frontend
pnpm build:web
npx -y yaml@2 valid api/openapi.yaml
```

- [ ] **Step 3: 报告草稿**

完成范围、数据模型、API、测试结果、代码审核占位、已知限制、审阅清单。

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "docs(stage-3h): add sales return report draft"
```

---

## Task 8: 独立代码审核、修复、最终回归和推送

**Files:**
- Modify: `docs/reports/2026-11-18-stage-3h-sales-returns.md`

- [ ] **Step 1: 派发独立审核**

重点：

- 退货数量/并发锁、超退。
- 退货金额比例/舍入、外币/base 金额。
- 库存回库/平均成本。
- 退货凭证平衡和科目方向。
- 贷项单自动冲发票、remaining、发票状态、付款核销 outstanding。
- 期间外币重估 credited。
- 迁移/down/权限/编码规则。
- API 权限、错误码、数据范围。
- 前端、SDK、OpenAPI。
- 测试覆盖和干净库回归。

- [ ] **Step 2: 修复并回归**

Critical 必须修复；Important 修复或记录；运行 Stage 3H、Stage 3 全链路和静态检查。

- [ ] **Step 3: 填写报告审核章节，提交推送**

```bash
git add -A
git commit -m "fix(stage-3h): address code review findings and finalize report"
git push -u origin stage-3h-sales-returns
```

---

## 验收清单

- [ ] 销售发货支持部分/全部退货。
- [ ] 库存按原成本回库并重算平均成本。
- [ ] 退货凭证正确冲收入/销项税/应收并回库存/冲成本。
- [ ] 贷项单自动冲原发票未收金额，剩余保留客户贷项。
- [ ] 发票 credited/outstanding/状态正确，付款核销和期间重估考虑 credited。
- [ ] 前端退货、贷项单、发票展示完整。
- [ ] SDK/OpenAPI/迁移/权限完整。
- [ ] Stage 3B–3H 干净库全链路和静态检查通过。
- [ ] 代码审核和汇总报告完成。
