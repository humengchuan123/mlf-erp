# Stage 3B 汇总报告：销售、发货出库与应收应付

> 日期：2026-11-17
> 分支：`stage-3b-sales-finance`
> 提交：`f25db52`（实现）+ `ceb06aa`（范围收紧/API 契约）+ `37c0640`（收付款校验/演示数据）+ `ceed621`（审核修复）
> 计划：`docs/superpowers/plans/2026-11-17-stage-3b-sales-finance.md`
> 状态：待用户审阅

---

## 1. 完成范围

### 1.1 销售

- 销售订单头/行：客户、仓库、订单日期、币种、金额、备注。
- 状态机：`draft → submitted → approved → partially_shipped → shipped`，`cancelled` 可从无发货的 draft/submitted/approved 进入。
- 创建、编辑草稿、提交、审批、取消、详情、列表。
- 发货单头/行、部分发货、全部发货、超发校验、重复行校验、库存不足校验。
- 发货单编码 `DN-yyyyMMdd-0001`，应收发票编码 `AR-yyyyMMdd-0001`。

### 1.2 库存

- 库存余额增加移动平均成本 `avg_cost`。
- 采购收货按采购单价更新移动平均成本。
- 销售发货扣减 `on_hand`、`available`，写 `direction='out'`、`biz_type='sales_shipment'` 库存流水，单位成本取当前 `avg_cost`。
- 数据库增加库存非负检查约束。

### 1.3 财务

- 统一 `finance_invoices`：应收/应付、来源单据、往来单位、日期、到期日、币种、总额、已核销、状态。
- 销售发货自动生成应收发票。
- 采购收货自动生成应付发票。
- 统一 `finance_payments` 和 `finance_payment_allocations`。
- 收付款登记、部分核销、多次核销、重复核销、超额核销、类型/往来单位/币种匹配校验。
- 付款状态 `unallocated → partially_allocated → allocated`；发票状态 `open → partially_paid → paid`。

### 1.4 权限

- `sales.order.read`
- `sales.order.write`
- `sales.order.approve`
- `sales.shipment.read`
- `finance.invoice.read`
- `finance.payment.read`
- `finance.payment.write`

### 1.5 前端

- 销售订单列表、新建/编辑、详情、提交、审批、取消、发货。
- 发货单列表、详情、出库成本。
- 应收应付列表、类型/状态/关键字筛选、到期/逾期提示、发票详情、核销记录。
- 收付款列表、新建、按往来单位加载未付清发票、批量核销。
- 菜单、路由、权限、SDK。

### 1.6 自动化测试

- `tests/api/stage3b_test.go`：真实 HTTP API 集成测试。
- `scripts/test/stage3b-api.ps1`：Docker/Postgres、迁移、seed、demo、API 启动和测试编排。
- `scripts/test/check-frontend.mjs`：路由、菜单、页面文件、SDK 方法完整度检查。
- `pnpm test:api:stage3b`、`pnpm test:frontend`。

---

## 2. 数据库变更

迁移 `000008_sales_finance`：

- `erp.sales_orders`
- `erp.sales_order_lines`
- `erp.sales_shipments`
- `erp.sales_shipment_lines`
- `erp.finance_invoices`
- `erp.finance_payments`
- `erp.finance_payment_allocations`
- `erp.inventory_balances.avg_cost`
- 库存非负约束
- 7 个销售/财务权限
- 6 个编码规则：`sales_order`、`sales_shipment`、`sales_receivable`、`purchase_payable`、`finance_receipt`、`finance_payment`

迁移 `000009_inventory_org_repair`：

- 把库存余额和库存流水的 `org_id` 修正为仓库所属组织，修复 Stage 3A 早期按操作人组织归属的数据。
- Down 为 no-op，数据修复不可安全回滚。

迁移 `000010_legacy_payables`：

- 为 Stage 3B 之前的采购收货补建 `AP-LEGACY-*` 应付发票。
- Down 为 no-op，避免删除可能已有核销的发票。

迁移 `000011_sales_finance_permission_backfill`：

- 把旧 `sales.read/write`、`finance.read/write` 授权映射到新的销售/财务权限，避免升级后自定义角色失去页面访问。
- Down 为 no-op。

迁移 `000012_order_line_guards`：

- 订单/发货/收货行数量必须为正，单价/成本非负的数据库检查约束。

关键约束：

- 销售订单号 `(tenant_id, code)` 唯一。
- 销售订单行 `(order_id, line_no)` 唯一，`shipped_qty <= quantity`。
- 发货单号 `(tenant_id, code)` 唯一。
- 发货行 `(shipment_id, order_line_id)` 唯一。
- 发票来源 `(tenant_id, invoice_type, source_type, source_id)` 唯一，防止重复开票。
- 发票金额 `paid_amount <= total_amount`。
- 收付款金额 `allocated_amount <= amount`。
- 核销明细 `(payment_id, invoice_id)` 唯一。

---

## 3. API 清单

### 销售

| 方法 | 路径 | 权限 |
|---|---|---|
| GET/POST | `/api/v1/sales/orders` | `sales.order.read/write` |
| GET/PUT | `/api/v1/sales/orders/:id` | `sales.order.read/write` |
| POST | `/api/v1/sales/orders/:id/submit` | `sales.order.write` |
| POST | `/api/v1/sales/orders/:id/approve` | `sales.order.approve` |
| POST | `/api/v1/sales/orders/:id/cancel` | `sales.order.write` |
| POST | `/api/v1/sales/orders/:id/ship` | `sales.order.write` |
| GET | `/api/v1/sales/shipments` | `sales.shipment.read` |

### 财务

| 方法 | 路径 | 权限 |
|---|---|---|
| GET | `/api/v1/finance/invoices` | `finance.invoice.read` |
| GET | `/api/v1/finance/invoices/:id` | `finance.invoice.read` |
| GET | `/api/v1/finance/payments` | `finance.payment.read` |
| GET | `/api/v1/finance/payments/:id` | `finance.payment.read` |
| POST | `/api/v1/finance/payments` | `finance.payment.write` |
| POST | `/api/v1/finance/payments/:id/allocate` | `finance.payment.write` |

金额和数量均使用字符串传输。`api/openapi.yaml` 已同步 Stage 3A/3B 采购、销售、库存和财务接口。

---

## 4. 测试结果

### 4.1 静态检查与构建

| 检查 | 结果 |
|---|---|
| `gofmt -l ./cmd ./internal ./db ./tests` | 通过 |
| `go test ./...` | 通过 |
| `go vet ./...` | 通过 |
| `pnpm typecheck:web` | 通过 |
| `pnpm build:web` | 通过（Element Plus 主包约 957KB，chunk size warning） |
| `pnpm test:frontend` | 通过 |

### 4.2 自动化接口测试

执行命令：

```bash
pnpm test:api:stage3b
```

结果：`TestStage3B`（11 个子测试）与 `TestStage3BAnonymous` 全部通过。审核修复后已重新执行并确认通过。

| 用例 | 结果 |
|---|---|
| 无 token 访问销售订单/财务发票 | 401 |
| 采购入库 20 × 50，生成应付 1000 | 通过 |
| 销售订单 10 × 120 = 1200，提交/审批 | 通过 |
| 发货 4，订单 partially_shipped，库存 -4，应收 480 | 通过 |
| 再发货 6，订单 shipped，库存 -10，应收 720 | 通过 |
| 超发 11/10 | 409 `OVER_SHIPMENT` |
| 同一请求重复行 10+10 | 409 `OVER_SHIPMENT`，库存、已发数量和发货单数不变 |
| 无库存物料发货 | 409 `INSUFFICIENT_STOCK` |
| 草稿订单发货 | 409 `INVALID_STATUS` |
| 重复审批 | 409 `INVALID_STATUS` |
| 审批后取消 | 200 |
| 取消后发货 | 409 `INVALID_STATUS` |
| 部分发货后取消 | 409 `INVALID_STATUS` |
| 草稿订单 PUT 编辑 | 金额重算正确 |
| 负数量/负单价 | 400 `BAD_REQUEST` |
| 收款 500，核销 480 + 20 | 第一张发票 paid，第二张 partially_paid |
| 已全额核销收款继续核销 | 409 `PAYMENT_ALLOCATED` |
| 同一发票先核销 6 再补核销 4 | 发票 paid，核销明细累加 |
| 核销金额超过发票未核销额 | 400 `OVER_ALLOCATE` |
| 收款币种与发票不一致 | 400 `CURRENCY_MISMATCH` |
| 收款往来单位与发票不一致 | 400 `PARTNER_MISMATCH` |
| 同一请求重复发票 ID | 400 `BAD_REQUEST` |
| 支付金额 `0.00004` | 400 `BAD_REQUEST` |
| 付款 1000 核销应付 | 应付发票 paid |
| 收款核销应付发票 | 400 `PAYMENT_TYPE_MISMATCH` |
| demo_dept 查看 HQ 销售订单/发票/发货单/收付款 | 0 / 0 / 0 / 0 |
| demo_dept 创建销售订单/收付款 | 403 |
| 同一物料多行发货 | 通过，金额正确 |
| 零单价发货 | 应收发票 total=0，status=paid |
| 零金额发票重复核销 | 409 `INVOICE_PAID` |
| 采购收货列表返回应付发票号/到期日 | 通过 |
| 发货单列表返回应收发票号/到期日 | 通过 |
| 库存平均成本 | 采购 50 后 `avg_cost=50` |
| 发货成本/库存流水 | 4 × 50，`total_cost=200`，出库流水 `unit_cost=50`、`amount=200` |
| 分页 `page_size=1`、状态筛选 | 通过 |
| 精度 0.1 × 3 | `0.3000` |
| 零数量明细 | 400 |

### 4.3 前端完整度

`check-frontend.mjs` 验证：

- 路由包含销售订单、发货单、应收应付、收付款及对应权限。
- 菜单包含销售订单、发货单、应收应付、收付款。
- 四个页面文件存在，包含表格、弹窗/抽屉和对应 SDK 调用。
- SDK 导出 `salesApi`、`financeApi`，包含列表、创建、提交、审批、取消、发货、收付款、核销方法。

### 4.4 Demo 数据

重复执行 `go run ./cmd/demo` 幂等。

当前演示数据：

| 数据 | 内容 |
|---|---|
| 销售订单 1 | CUS-DEMO-001，2 × 150，已发货，应收 300 已收款 |
| 销售订单 2 | CUS-DEMO-002，3 × 80，已发 1，应收 80 未收 |
| 采购订单 3 | SUP-DEMO-001，2 × 100，已收货，应付 200 已付款 |
| 历史采购收货 | 应付 400 / 250，未付款，用于账龄演示 |
| 收付款 | 收款 300、付款 200，均已核销 |

---

## 5. 代码审核

### 5.1 审核方式

独立代码审核子代理，审查范围 `526b90d..f25db52`，覆盖销售、发货、库存扣减、应收应付、收付款核销、迁移、前端和测试。

### 5.2 审核结论

**Ready to merge: With fixes（初次审核）**

- **Critical 1 项**：同一收付款对同一发票第二次核销会命中 `(payment_id, invoice_id)` 唯一约束，导致“部分核销后再补核销”失败。
- **Important 9 项**：支付金额先校验后四舍五入可能返回 500；销售/采购状态更新缺少数据范围谓词；发票核销记录查询忽略数据范围；收付款组织归属与发票不一致；Stage 3A 历史收货未回填应付；旧角色权限拆分后可能失去访问；前端同一物料多行发货库存提示不准确；测试缺少关键失败路径；`000009` 和报告未提交。
- 审核同时确认发货事务、库存行锁、收付款核销锁、金额字符串、租户/范围过滤和自动 AR/AP 主流程设计正确。
- **审核修复后结论：Ready to merge: Yes**。上述 Critical/Important 已修复并回归，剩余项均记录为已知限制或后续优化。

### 5.3 已修复问题

- **Critical：同一发票补核销**
  - 核销明细插入改为 `ON CONFLICT (payment_id, invoice_id) DO UPDATE SET amount = amount + EXCLUDED.amount`。
  - 新增集成测试：收款 10，先核销 6，再核销 4；发票从 `partially_paid` 变为 `paid`。
- **支付金额舍入**
  - 先 `Round(4)` 再校验正数，`0.00004` 返回 400。
  - `23514` 检查约束错误映射为 `ErrInvalidInput`，销售/采购/财务统一处理。
- **状态更新数据范围**
  - 销售/采购 `UpdateStatus` 增加 `scope.Filter` 谓词，避免读取后组织被移出范围的 TOCTOU。
- **核销记录数据范围**
  - 发票详情核销记录按收付款数据范围过滤。
- **历史应付回填**
  - 新增迁移 `000010_legacy_payables`，为 Stage 3A 历史收货补建 `AP-LEGACY-*` 应付发票。
- **旧角色权限兼容**
  - 新增迁移 `000011_sales_finance_permission_backfill`，把旧 `sales.read/write`、`finance.read/write` 映射到新权限。
- **前端同物料多行发货**
  - 按物料聚合可用量并递减预填数量，保存时按物料汇总校验，避免逐行都通过、服务端却库存不足。
- **测试补强**
  - 增加发货成本/库存流水断言、拒绝发货后的库存/发货单不变断言、超额核销、币种/往来单位不匹配、重复发票 ID、补核销、草稿 PUT 编辑、负数量/负单价、取消后发货、重复审批等用例。
- **其他**
  - 发货单列表增加状态列。
  - 创建订单/发货/收付款响应补 `created_at`。
  - 发货仓储校验发货仓库/组织/客户与锁定订单一致。
  - 库存余额 API/SDK/页面增加 `avg_cost`。
  - 新增迁移 `000012_order_line_guards`，数据库约束订单/发货/收货行数量为正、单价/成本非负。
  - 新增迁移 `000009_inventory_org_repair`，修复历史库存组织归属。
  - CI 增加真实 PostgreSQL Stage 3B API 集成测试作业。

### 5.4 未修复/后续处理

- 发货、收货、收付款创建仍无客户端幂等键，网络重试可能重复扣库存或重复登记收付款。
- 自动化测试仍缺少并发发货/并发核销、事务故障注入和真实浏览器 E2E。
- 销售/发货订单列表存在 N+1 查询，后续批量加载行。
- 前端金额展示/预填仍使用 JavaScript `Number`，后端金额校验和计算是权威的；后续可引入 decimal 前端库。
- 收付款归属操作人组织；跨组织核销需要 `all` 范围用户，后续可按发票组织或明确业务组织归属。
- 编码规则只为默认租户初始化，新租户开通时需要复制销售/财务编码规则。
- 未实现销售退货、反核销、发票红冲、税额拆分、客户信用和账龄分析。

---

## 6. 已知限制

- 审批仍是简化状态操作，未接入可配置审批流引擎。
- 发货不支持批次/序列号/保质期。
- 库存成本为移动加权平均，暂不支持先进先出、个别计价、多币种成本折算。
- 销售退货、采购退货、红冲、反核销未实现。
- 发货和收付款没有客户端幂等键，网络重试可能重复提交。
- 收付款核销只支持正向核销，不支持反核销和跨币种核销。
- 发票到期日固定为业务日期 + 30 天，未接入客户账期主数据。
- 应收/应付按单据金额开票，未拆分税额、价税分离和税率计算。
- 数据范围管理 API/UI 仍未提供，仍通过 `org_memberships` 配置。
- 收付款归属操作人组织；跨组织核销需要 `all` 范围用户。
- 历史应付回填发票编码为 `AP-LEGACY-*`，与原编码规则格式不同，仅用于升级兼容。
- OpenAPI 已补充采购、销售、库存、应收应付、收付款路径和 schema，但尚未接入自动契约校验。
- 前端主包偏大，后续按需引入 Element Plus。
- 自动化测试覆盖主要业务路径，但缺少并发发货/并发核销、事务故障注入、真实浏览器 E2E。
- `pnpm test:api:stage3b` 会在当前 dev 数据库留下测试单据和库存；建议在独立测试库执行，或在测试后重建数据库。

---

## 7. 下一阶段建议

Stage 3C：总账、凭证、三大报表

- 会计科目、期间、凭证字、凭证分录。
- 销售/采购/收付款/库存单据生成凭证。
- 总账、明细账、试算平衡。
- 资产负债表、利润表、现金流量表。
- 月结、结账、反结账。

也可先补 Stage 3B 的退货、反核销、账龄和客户信用，再进入总账。

---

## 8. 用户审阅清单

- [ ] 销售订单状态机和发货规则是否符合业务。
- [ ] 部分发货、超发、库存不足处理是否接受。
- [ ] 库存移动平均成本和出库成本口径是否接受。
- [ ] 应收/应付自动开票规则是否接受。
- [ ] 收付款核销规则（同往来单位、同币种、类型匹配）是否接受。
- [ ] 前端页面和 Demo 数据是否满足演示。
- [ ] 代码审核结论和修复是否认可。
- [ ] 是否允许进入 Stage 3C。
