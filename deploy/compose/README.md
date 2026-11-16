# Compose 部署

`docker-compose.dev.yml` 只启动本地开发依赖：

- PostgreSQL 16
- Redis 7
- RabbitMQ 3.13（含管理台）
- MinIO

启动：

```bash
docker compose -f deploy/compose/docker-compose.dev.yml up -d
```

私有化正式交付时，应使用与版本绑定的固定镜像标签、内置数据库/中间件、安装器和离线包，不要直接使用 `latest`。
