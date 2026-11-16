# SaaS 部署

SaaS 部署建议使用 Kubernetes + Helm/Argo CD：

- 多租户共享数据库，`tenant_id` 行级隔离。
- 镜像由 CI 构建并推送到 GHCR/Harbor，使用 `sha-<commit>` 标签。
- 发布流程：staging → 冒烟/E2E → 人工审批 → production。
- 数据库迁移采用 expand/contract，先迁移后发应用。
- 配置、密钥、域名、证书通过 ConfigMap/Secret 注入。
