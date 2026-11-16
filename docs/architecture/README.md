# 架构说明

## 分层

```text
http → app → domain
infra → domain
domain 不依赖框架、数据库和 HTTP
```

- `internal/platform`：共享内核，提供配置、日志、租户上下文、认证上下文、HTTP 响应、数据库、License/entitlement、Outbox 等。
- `internal/modules`：业务模块，每个模块内部按 `domain / app / infra / http` 分层。
- `cmd`：进程入口，只做装配，不写业务逻辑。

## 多租户

- `tenant_id` 是 SaaS 客户隔离边界。
- `org_id` 是租户内部组织边界。
- 所有业务表必须带 `tenant_id`；唯一键和外键尽量带租户。
- 后续接入 PostgreSQL RLS，应用层过滤与 RLS 双保险。

## 金额

- API 金额使用字符串。
- 领域层使用 `shopspring/decimal`。
- 数据库使用 `numeric`。
- 禁止 `float32/float64` 参与金额计算。

## 能力与交付

- 业务代码判断 `capability`，不判断 `edition == "saas"`。
- SaaS 使用订阅 entitlement，私有化使用离线 License。
- 交付差异放在 `deploy/` 和 `internal/platform` 适配器中。
