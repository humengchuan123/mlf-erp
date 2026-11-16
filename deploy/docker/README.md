# Docker 镜像

| 文件 | 用途 |
|---|---|
| `api.Dockerfile` | Go API 服务 |
| `worker.Dockerfile` | Go 计算/定时任务 |
| `migrate.Dockerfile` | 数据库迁移 |
| `web.Dockerfile` | 前端静态资源 + Nginx |

构建示例：

```bash
docker build -f deploy/docker/api.Dockerfile -t mlf-erp/api:dev .
docker build -f deploy/docker/worker.Dockerfile -t mlf-erp/worker:dev .
docker build -f deploy/docker/web.Dockerfile -t mlf-erp/web:dev .
```

生产环境镜像标签应使用 `sha-<commit>` 或 `v<version>`，不要使用 `latest`。
