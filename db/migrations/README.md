# 数据库迁移

- `000001_init`：租户、组织、用户、角色、成员关系、Outbox 事件基础表。
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
