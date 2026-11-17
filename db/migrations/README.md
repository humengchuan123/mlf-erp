# 数据库迁移

- `000001_init`：租户、组织、用户、角色、成员关系、Outbox 事件基础表。
- `000002_platform_auth_rbac_audit`：权限、角色权限、refresh token、登录日志、审计日志。
- `000003_masterdata_code_rules`：主数据表、编码规则表、Stage 2 权限。
- `000004_fix_org_root_path`：修正根组织 path 为 `/<id>`。
- `000005_rebuild_org_paths`：递归重建全部组织 path，并给现有 admin 角色补授 Stage 2 权限。Down 为 no-op，路径修复不可安全回滚。
- `000006_purchase_inventory`：采购订单/行、收货单/行、库存余额/流水、采购/库存权限和收货编码规则。
- `000007_purchase_inventory_guards`：收货行唯一约束、订单行收货数量检查约束。
- `000008_sales_finance`：销售订单/行、发货单/行、应收应付发票、收付款与核销、库存平均成本、销售/财务权限和编码规则。
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
