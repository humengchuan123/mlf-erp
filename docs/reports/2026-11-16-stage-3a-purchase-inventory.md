# Stage 3A 汇总报告：采购与库存闭环

> 日期：2026-11-16  
> 分支：`stage-3a-purchase-inventory`  
> 提交：`eeabcf0`  
> 计划：`docs/superpowers/plans/2026-11-16-stage-3a-purchase-inventory.md`  
> 状态：用户已要求进入 Stage 3B（2026-11-17），视为审阅通过

---

## 1. 完成范围

### 1.1 采购

- 采购订单头/行：供应商、仓库、订单日期、币种、金额、备注。
- 状态机：`draft → submitted → approved → partially_received → received`，`cancelled` 可从无收货的 draft/submitted/approved 进入。
- 创建、编辑草稿、提交、审批、取消、详情、列表。
- 采购收货单头/行、部分收货、全部收货、超收校验。
- 收货单编码：`GRN-yyyyMMdd-0001`。

### 1.2 库存

- 收货事务内更新订单行已收数量、订单状态。
- 库存余额 upsert：`on_hand / reserved / available`。
- 库存流水：方向、业务类型、业务单据、数量、单位成本、金额。
- 库存余额、库存流水查询 API 和前端页面。

### 1.3 权限

- `purchase.order.read`
- `purchase.order.write`
- `purchase.order.approve`
- `purchase.receipt.read`
- `inventory.balance.read`
- `inventory.transaction.read`

### 1.4 前端

- 采购订单列表、新建/编辑、详情抽屉、提交、审批、取消、收货。
- 库存余额列表。
- 库存流水列表。
- 菜单/路由权限更新。

### 1.5 Demo 数据

- 2 张采购订单：
  - `demo-seed-order-1`：10 × 100，部分收货 4，状态 `partially_received`。
  - `demo-seed-order-2`：5 × 50，全部收货 5，状态 `received`。
- 对应库存余额：物料 1 在手 4，物料 2 在手 5。
- 2 条采购入库库存流水。

---

## 2. 数据库变更

迁移 `000006_purchase_inventory`：

- `erp.purchase_orders`
- `erp.purchase_order_lines`
- `erp.purchase_receipts`
- `erp.purchase_receipt_lines`
- `erp.inventory_balances`
- `erp.inventory_transactions`
- 6 个采购/库存权限
- `purchase_receipt` 默认编码规则

迁移 `000007_purchase_inventory_guards`：

- `purchase_receipt_lines (receipt_id, order_line_id)` 唯一约束，防止同一收货单重复行。
- `purchase_order_lines` 增加 `received_qty >= 0 AND received_qty <= quantity` 检查约束。

关键约束：

- 订单号 `(tenant_id, code)` 唯一。
- 订单行 `(order_id, line_no)` 唯一。
- 收货单 `(tenant_id, code)` 唯一。
- 收货行关联订单行，防止跨订单收货。
- 库存余额主键 `(tenant_id, warehouse_id, material_id)`。

---

## 3. API 清单

| 方法 | 路径 | 权限 |
|---|---|---|
| GET/POST | `/api/v1/purchase/orders` | `purchase.order.read/write` |
| GET/PUT | `/api/v1/purchase/orders/:id` | `purchase.order.read/write` |
| POST | `/api/v1/purchase/orders/:id/submit` | `purchase.order.write` |
| POST | `/api/v1/purchase/orders/:id/approve` | `purchase.order.approve` |
| POST | `/api/v1/purchase/orders/:id/cancel` | `purchase.order.write` |
| POST | `/api/v1/purchase/orders/:id/receive` | `purchase.order.write` |
| GET | `/api/v1/purchase/receipts` | `purchase.receipt.read` |
| GET | `/api/v1/inventory/balances` | `inventory.balance.read` |
| GET | `/api/v1/inventory/transactions` | `inventory.transaction.read` |

金额和数量均使用字符串传输。

---

## 4. 测试结果

### 4.1 静态检查与构建

| 检查 | 结果 |
|---|---|
| `gofmt -l ./cmd ./internal ./db` | 通过 |
| `go test ./...` | 通过 |
| `go vet ./...` | 通过 |
| `pnpm typecheck:web` | 通过 |
| `pnpm build:web` | 通过（Element Plus 主包约 956KB，chunk size warning） |

### 4.2 数据库迁移与 seed

- `000006` 迁移成功。
- seed 成功，新增采购/库存权限和 `purchase_receipt` 编码规则。
- demo 数据重复执行幂等。

### 4.3 真实 PostgreSQL API 测试

| 场景 | 结果 |
|---|---|
| 创建采购订单 10 × 100 | 成功，总额 1000.0000 |
| 提交 | 成功 |
| 审批 | 成功，状态 approved |
| 部分收货 4 | 成功，状态 partially_received，已收 4，未收 6 |
| 部分收货后库存余额 | on_hand=4 |
| 再收货 6 | 成功，状态 received，已收 10 |
| 全部收货后库存余额 | on_hand=10 |
| 库存流水数量 | 2 条 |
| 超收 11/10 | 409 |
| 草稿订单收货 | 409 |
| demo_dept 查看 HQ 采购订单/库存 | 0 / 0 |
| demo_dept 创建采购订单 | 403 |
| 同一收货请求重复 `order_line_id` | 409，收货单 0，已收 0，状态保持 approved |
| 重复请求后正常收货 10 | 成功，状态 received |
| 华东仓收货后 demo_dept 查看库存 | 1 条，on_hand=2 |
| 测试数据清理 | 完成 |

### 4.4 审核修复回归

- 重复行聚合后校验总量，超出剩余数量返回 409。
- 数据库增加唯一约束和 `received_qty <= quantity` 检查。
- 收货行稳定排序，降低多物料并发收货死锁风险。
- 库存组织归属改为仓库组织，demo_dept 能看到华东仓库存。
- 库存 `self` 数据范围改为精确组织匹配，未知范围失败关闭。

---

## 5. 代码审核

### 5.1 审核方式

独立代码审核子代理，审查范围 `91d3725..eeabcf0`。

### 5.2 审核结论

**Ready to merge: No（初次审核）**

- Critical 1 项：同一收货请求重复 `order_line_id` 可绕过超收校验，导致订单行超收和库存翻倍。
- Important 6 项：库存组织归属使用操作人组织而非仓库组织、库存 `self` 范围语义不一致、收货不幂等、缺少事务自动化测试、OpenAPI 未更新、以及若干错误语义/精度/N+1/死锁风险。

### 5.3 已修复问题

- **Critical 重复收货行**：
  - 应用层按 `order_line_id` 聚合数量后再校验剩余量。
  - 仓储层拒绝重复行，并按物料/订单行稳定排序。
  - 订单行更新改为条件更新 `received_qty + qty <= quantity`，并检查 RowsAffected。
  - 迁移 000007 增加收货行唯一约束和订单行收货数量检查。
- **库存组织归属**：订单/收货/库存余额/流水统一使用仓库组织 `warehouse.OrgID`，不再使用操作人组织。
- **库存 self 范围**：`scope.Filter` 增加 `OrgID`，库存 `self` 改为精确组织匹配；未知范围失败关闭。
- **并发死锁**：收货行按 `material_id + order_line_id` 排序后处理。
- **Demo 权限**：补充 `purchase.receipt.read`。
- **迁移回滚**：`000006` down 增加 `purchase_receipt` 编码规则清理。
- **前端**：默认日期改为本地日期；收货数量非法时提示，不再静默丢弃。

### 5.4 未修复/后续处理

- 收货幂等键/客户端请求 ID 尚未实现，网络重试仍可能重复收货；建议 Stage 3B 前补 `Idempotency-Key`。
- 采购/库存事务缺少 PostgreSQL 集成测试、并发测试、回滚故障注入测试。
- OpenAPI 仍保留旧采购契约，需补新采购/库存路径和 schema。
- 数量/单价小数位未在应用层限制到 6 位，极端精度会被数据库舍入。
- 订单/收货列表存在 N+1 查询；多币种库存估值未定义。
- `received → closed` 关闭操作、采购退货、批次/序列号、成本计价留待后续。
- 初次审核未重新跑完整审核；Critical 和主要 Important 已修复并回归验证。

---

## 6. 已知限制

- 采购审批是简化状态操作，尚未接入可配置审批流引擎。
- 收货不支持批次/序列号/保质期，后续库存深化阶段补充。
- 库存成本采用采购单价，尚未实现移动加权/先进先出。
- 采购退货、红字、换货未实现。
- 收货单没有幂等键，重复提交依赖订单剩余数量和收货单号唯一约束；网络重试仍可能重复收货。
- 库存余额只做入库，销售出库/调拨/盘点/预留后续实现。
- 库存成本使用采购单价，多币种估值尚未定义。
- 采购/库存事务的自动化集成测试仍需补齐。
- 数据范围管理 API/UI 仍未提供，仍通过 `org_memberships` 配置。
- 前端主包偏大，后续按需引入 Element Plus。

---

## 7. 下一阶段建议

Stage 3B：销售与应收闭环

- 销售订单头/行、提交/审批/取消。
- 发货出库、库存扣减、库存流水。
- 客户信用检查、价格/折扣。
- 应收单、收款、核销、账龄。
- 销售/应收前端页面。

也可先补 Stage 3A 的采购退货和库存成本计价，再进入销售。

---

## 8. 用户审阅清单

- [ ] 采购订单状态机是否符合业务。
- [ ] 部分收货/超收规则是否接受。
- [ ] 库存余额和流水口径是否接受。
- [ ] Demo 采购/库存数据是否满足演示。
- [ ] 代码审核结论和修复是否认可。
- [x] 是否允许进入 Stage 3B。
