# 数据库迁移

- `000001_init`：租户、组织、用户、角色、成员关系、Outbox 事件基础表。
- `000002_platform_auth_rbac_audit`：权限、角色权限、refresh token、登录日志、审计日志。
- `000003_masterdata_code_rules`：主数据表、编码规则表、Stage 2 权限。
- `000004_fix_org_root_path`：修正根组织 path 为 `/<id>`。
- `000005_rebuild_org_paths`：递归重建全部组织 path，并给现有 admin 角色补授 Stage 2 权限。Down 为 no-op，路径修复不可安全回滚。
- `000006_purchase_inventory`：采购订单/行、收货单/行、库存余额/流水、采购/库存权限和收货编码规则。
- `000007_purchase_inventory_guards`：收货行唯一约束、订单行收货数量检查约束。
- `000008_sales_finance`：销售订单/行、发货单/行、应收应付发票、收付款与核销、库存平均成本、销售/财务权限和编码规则。
- `000009_inventory_org_repair`：把库存余额/流水的 `org_id` 修正为仓库所属组织。Down 为 no-op，数据修复不可安全回滚。
- `000010_legacy_payables`：为 Stage 3B 之前的采购收货单补建应付发票（`AP-LEGACY-*`）。Down 为 no-op。
- `000011_sales_finance_permission_backfill`：把旧 `sales.read/write`、`finance.read/write` 授权映射到新的销售/财务权限。Down 为 no-op。
- `000012_order_line_guards`：订单/发货/收货行数量必须为正、单价/成本非负的数据库检查约束。
- `000013_general_ledger`：会计科目、期间、科目映射、凭证/分录、总账权限、`gl_voucher` 编码规则，并为现有租户初始化标准科目、映射和当前年度期间。
- `000014_general_ledger_indexes`：总账报表和期间关账查询索引。
- `000015_period_close_auxiliary`：凭证行项目辅助、期末关账/年度关账记录、损益结转和利润分配科目映射。
- `000016_period_close_integrity`：关账记录唯一索引、辅助核算外键和项目编码长度约束。
- `000017_tax_invoice`：单据价税分离字段、增值税发票状态、进项/销项税科目和映射、发票开具权限。
- `000018_repair_tax_headers`：回填 Stage 3E 历史发货/收货表头的净额、税额和含税总额。
- `000019_multi_currency`：汇率表、单据/发票/收付款本位币字段、已实现汇兑损益、期末外币重估和汇兑科目。
- `000020_sales_returns`：销售退货单/行、发货行已退数量、贷项单、发票已冲贷项金额和退货权限/编码规则。
- `000021_purchase_returns`：采购退货单/行、收货行已退数量、复用贷项单和采购退货权限/编码规则。Down 在仍存在采购退货数据（`purchase_returns`）时会直接拒绝执行，需先归档或冲销采购退货单；它只删除本迁移新增的采购退货表、`returned_qty` 列和相关权限/编码规则，不会删除 Stage 3H 销售退货的表或列。
- `000022_credit_note_usage`：贷项已退金额、抵用明细、贷项退款单、退款凭证和贷项抵用/退款权限/编码规则。Down 在存在退款单、非初始抵用明细或已退余额时拒绝执行。
- `000023_credit_note_status_repair`：把 `remaining_amount = 0` 但仍为 `open` 的贷项单修正为 `applied`。Down 为 no-op，只修复状态，无法可靠还原修复前的展示状态。
- `000024_payment_reversals`：收付款反核销记录、汇兑结算反核销字段和 `finance.payment.reverse` 权限。Down 在有反核销数据或已反核销汇兑结算时拒绝执行；如被拒绝，需在核对 schema 后把迁移记录恢复为 version 24 / dirty=false，或走归档/已反核销数据恢复路径。
- `000025_payment_base_allocation_repair`：把收付款已核销本位币金额回填为有效核销明细 `base_amount` 之和，修正历史按总额一次性四舍五入造成的差异。Down 为 no-op，数据修复不可安全回滚。
- `000026_idempotency_keys`：资金/库存 POST 操作的幂等键、请求哈希、响应回放和过期清理；Down 直接删除该表。
- 迁移由 `cmd/migrate` 执行，使用 `golang-migrate`，SQL 文件通过 `embed` 打进二进制。
- 所有业务表遵循 `tenant_id` 非空、唯一键带租户、外键尽量带租户的原则。

## 常用命令

```bash
go run ./cmd/migrate -direction up
go run ./cmd/migrate -direction down -steps 1
```

## 后续

- 增加 RLS 策略迁移，并在事务内设置 `app.tenant_id`。
- 增加采购、销售、库存、财务模块表。
- 对长耗时迁移采用 expand/contract，保证 SaaS 滚动发布兼容。
