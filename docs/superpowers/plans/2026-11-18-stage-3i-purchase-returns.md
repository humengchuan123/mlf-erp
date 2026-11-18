# Stage 3I 采购退货与应付贷项单 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 增加采购退货闭环：按原收货行退货、原成本出库、冲应付/进项税/存货、生成应付贷项单并冲原应付发票未付金额。

**Architecture:** 复用 Stage 3H 的退货/贷项模式；新增 purchase_returns/lines 和 receipt returned_qty；复用 finance_credit_notes 的 purchase_return 类型、发票 credited 和期间重估逻辑。

**Tech Stack:** Go 1.25 + Gin + pgx + PostgreSQL + shopspring/decimal；Vue 3 + TypeScript + Element Plus。

**Design:** `docs/superpowers/specs/2026-11-18-stage-3i-purchase-returns-design.md`

---

## Task 1: 迁移、权限和编码规则

**Files:**
- Create: `db/migrations/000021_purchase_returns.up.sql`
- Create: `db/migrations/000021_purchase_returns.down.sql`
- Modify: `db/migrations/README.md`
- Modify: `cmd/seed/main.go`

- [ ] **Step 1: up 迁移**

- `purchase_receipt_lines.returned_qty` + 检查 + `(tenant_id,id)` 唯一。
- `purchase_returns`、`purchase_return_lines`，字段与销售退货对称。
- 索引：收货单/供应商/日期、退货日期。
- 权限 `purchase.return.read/write`，admin 授权。
- 编码规则 `purchase_return` 前缀 `PR`。
- 不改 `finance_credit_notes` 表结构；复用 `credit_type`。

- [ ] **Step 2: down 迁移**

有采购退货数据时 `RAISE EXCEPTION`，否则删除表/列/权限/编码规则。

- [ ] **Step 3: seed/迁移 README**

- seed 增加权限和编码规则。
- README 增加 000021 说明。

- [ ] **Step 4: 验证**

`go run ./cmd/migrate -direction up && go run ./cmd/seed`

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(stage-3i): add purchase return schema"
```

---

## Task 2: 采购退货域模型、累计计算和凭证

**Files:**
- Create: `internal/modules/purchase/domain/return.go`
- Create: `internal/modules/purchase/domain/return_test.go`
- Modify: `internal/modules/purchase/domain/receipt.go`（ReturnedQty）
- Modify: `internal/modules/gl/domain/voucher.go`
- Modify: `internal/modules/gl/infra/postgres/posting.go`

- [ ] **Step 1: 域模型**

`PurchaseReturn`/`PurchaseReturnLine`/`ReturnLineInput`，字段与销售退货对称。

`BuildReturnLines`：

- 校验收货行存在、数量 >0、累计不超收。
- 累计差分计算净额/税额/含税额/本位币/成本。
- `tax = gross - net`，保证凭证平衡。
- 全额退货等于原收货行。

- [ ] **Step 2: 测试**

- 部分/全额、零税/零成本、三次部分退货无尾差、超退、未知行、重复行。

- [ ] **Step 3: GL**

- `VoucherTypePurchaseReturn`/`SourceTypePurchaseReturn`。
- `PostPurchaseReturn`：借 AP 含税，贷 inventory 净额，贷 input_vat 税额；零行省略。

- [ ] **Step 4: 验证/提交**

```bash
go test ./internal/modules/purchase/... ./internal/modules/gl/... -count=1
git commit -m "feat(stage-3i): add purchase return domain and posting"
```

---

## Task 3: 采购退货事务仓储

**Files:**
- Modify: `internal/modules/purchase/domain/repository.go`
- Create: `internal/modules/purchase/infra/postgres/return_repository.go`
- Modify: `internal/modules/purchase/infra/postgres/order_repository.go`（receipt line returned_qty scan）

- [ ] **Step 1: Repository Create**

事务顺序：

1. 锁收货单/订单/收货行。
2. 校验状态、供应商、币种、退货日期。
3. 计算退货行。
4. 插入退货单/行。
5. 更新 receipt line returned_qty（带条件）。
6. 库存出库：按原收货成本，更新 on_hand/available/avg_cost；库存不足 409；写 inventory_transactions out/purchase_return。
7. GL `PostPurchaseReturn`（期间锁）。
8. 锁原应付发票，创建 `finance_credit_notes`（credit_type/source_type purchase_return），自动冲发票 credited，剩余 open。
9. 更新退货单 credit_note_id。
10. Commit，返回 Get 完整对象。

- [ ] **Step 2: Get/List**

- 列表加载行；过滤器 receipt_id/supplier_id/keyword/date range/scope。
- 详情 join 供应商/仓库/原发票/贷项单/凭证。

- [ ] **Step 3: 验证/提交**

```bash
go build ./...
go test ./internal/modules/purchase/... ./internal/modules/gl/... -count=1
git commit -m "feat(stage-3i): create purchase returns transactionally"
```

---

## Task 4: API、SDK、OpenAPI

**Files:**
- Create: `internal/modules/purchase/app/return_service.go`
- Modify: `internal/modules/purchase/http/handler.go`
- Modify: `internal/modules/finance/domain/credit_note.go` / `repository.go` / `credit_note_repository.go` / `http/handler.go`（credit_type 筛选/字段）
- Modify: `cmd/api/main.go`
- Modify: `packages/sdk/src/purchase.ts`、`packages/sdk/src/finance.ts`
- Modify: `api/openapi.yaml`
- Modify: `scripts/test/check-frontend.mjs`

- [ ] **Step 1: Purchase return service/HTTP**

- `POST /purchase/receipts/:id/returns`
- `GET /purchase/returns`
- `GET /purchase/returns/:id`
- 权限、UUID/日期/数量校验、OVER_RETURN/INSUFFICIENT_STOCK/PERIOD_CLOSED 错误码。
- 收货行增加 returned_qty/returnable_qty。

- [ ] **Step 2: Credit note type**

- `CreditNote` 增加 `CreditType` 已有；列表增加 `credit_type` 参数。
- 响应字段 `credit_type` 已有/补充。
- SDK/OpenAPI 增加 purchase return/credit note 类型。

- [ ] **Step 3: 验证/提交**

```bash
go build ./...
pnpm typecheck:web
npx -y yaml@2 valid api/openapi.yaml
git commit -m "feat(stage-3i): add purchase return APIs"
```

---

## Task 5: 前端

**Files:**
- Modify: `apps/web/src/views/purchase/OrderListView.vue` 或收货列表所在页面
- Create: `apps/web/src/views/purchase/PurchaseReturnListView.vue`
- Modify: `apps/web/src/views/finance/CreditNoteListView.vue`
- Modify: `apps/web/src/views/finance/InvoiceListView.vue`
- Modify: router/layout/check-frontend

- [ ] **Step 1: 收货退货**

- 收货列表增加退货操作（如果收货在采购订单页面，就在订单详情/收货记录上加）。
- 退货弹窗：可退数量、原单价/税率/成本、数量输入、金额/成本预览。
- 提交 `purchaseApi.returns.create(receiptID, input)`。

- [ ] **Step 2: 采购退货页**

- 列表/详情、贷项单状态、原应付发票、凭证号、已退/可退。

- [ ] **Step 3: 贷项单/发票**

- 贷项单页面增加类型筛选/列（销售/采购）。
- 应付发票显示 credited/未付。
- 路由/菜单/完整度断言。

- [ ] **Step 4: 验证/提交**

```bash
pnpm typecheck:web && pnpm test:frontend && pnpm build:web
git commit -m "feat(stage-3i): add purchase return UI"
```

---

## Task 6: API 测试、脚本和 CI

**Files:**
- Create: `tests/api/stage3i_test.go`
- Create: `scripts/test/stage3i-api.ps1`
- Modify: `package.json`、`README.md`、`scripts/test/check-frontend.mjs`

- [ ] **Step 1: 测试**

覆盖：

- 部分/全额采购退货、库存出库、平均成本、凭证。
- 应付贷项单 applied/remaining、发票 credited/outstanding/状态。
- 已付款应付发票退货，贷项 open。
- 超退 409、库存不足 409、权限、非法 UUID/日期/数量。
- 多次部分退货累计舍入。
- 外币采购退货和期间重估扣 credited。
- 贷项单 credit_type 筛选。
- Stage 3B–3I 干净库全链路。

- [ ] **Step 2: 脚本/package/README**

复制 stage3h 脚本为 stage3i，`TestStage3I`；package/README/CI 更新。

- [ ] **Step 3: 验证/提交**

```bash
pnpm test:api:stage3i
git commit -m "test(stage-3i): add purchase return API tests"
```

---

## Task 7: 文档、静态检查、报告草稿

- 更新 README、purchase/finance/gl/迁移 README。
- 创建 `docs/reports/2026-11-18-stage-3i-purchase-returns.md`。
- 运行 gofmt/go test/go vet/typecheck/frontend/build/OpenAPI。
- 提交 `docs(stage-3i): add purchase return report draft`。

---

## Task 8: 独立审核、修复、干净库回归、推送

- 派发独立代码审核，修复 Critical/Important。
- 跑 Stage 3I API、干净库 Stage 3B–3I、静态检查。
- 填写报告审核章节，提交并推送 `stage-3i-purchase-returns`。

---

## 验收清单

- [ ] 采购收货支持部分/全部退货。
- [ ] 库存按原成本出库并重算平均成本。
- [ ] 退货凭证正确冲应付/进项税/存货。
- [ ] 应付贷项单自动冲原发票未付金额，剩余保留供应商贷项。
- [ ] 发票 credited/outstanding/状态和期间重估正确。
- [ ] 前端、SDK、OpenAPI、迁移、权限完整。
- [ ] Stage 3B–3I 干净库和静态检查通过。
- [ ] 代码审核和报告完成。
