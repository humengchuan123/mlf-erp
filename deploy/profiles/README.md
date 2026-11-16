# 交付 Profile

当前 Go 服务通过环境变量加载配置，SaaS 与私有化的差异通过环境变量和部署清单注入。

建议的配置分组：

| 配置 | SaaS | 私有化 |
|---|---|---|
| `APP_DELIVERY_MODE` | `saas` | `onprem` |
| `DB_REQUIRED` | `true` | `true` |
| `DEFAULT_TENANT_ID` | 每租户动态 | 固定默认租户 |
| License | 订阅 entitlement | 离线 License |
| 对象存储 | 云 OSS/S3 | MinIO |
| 邮件/短信 | 云服务 | 本地 SMTP/站内信 |
| 监控 | 平台统一 | 本地 Prometheus/Grafana/Loki |

后续可以在此目录增加 `saas.yaml`、`onprem.yaml`，并由 Helm values 或配置加载器读取。
