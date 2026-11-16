# MLF ERP

企业级 ERP Monorepo 骨架，支持 **SaaS 订阅** 与 **私有化安装** 两种交付形态。

> 当前状态：Stage 1 平台底座 + Stage 2 主数据/编码规则/数据权限 + Stage 3A 采购与库存闭环。
> 设计参考：工作区根目录的《企业ERP应用开发指南》。

## 技术栈

| 层次 | 选型 |
|---|---|
| 前端 | Vue 3 + TypeScript + Vite + Element Plus |
| 后端 | Go 1.25 + Gin |
| 数据库 | PostgreSQL 16 |
| 缓存 | Redis 7 |
| 消息 | RabbitMQ 3.13 |
| 文件 | MinIO |
| Monorepo | pnpm workspace + Turborepo |
| 迁移 | golang-migrate + embed SQL |
| 部署 | Docker Compose / Kubernetes + Helm（规划中） |
| CI/CD | GitHub Actions / Jenkins（模板） |

## 目录结构

```text
mlf-erp/
├─ apps/
│  └─ web/                    # Vue 3 前端
├─ cmd/
│  ├─ api/                    # API 服务入口
│  ├─ worker/                 # 计算/定时任务入口
│  └─ migrate/                # 数据库迁移入口
├─ internal/
│  ├─ platform/               # 配置、日志、租户、认证、HTTP、数据库、Outbox
│  └─ modules/                # 业务模块
│     ├─ purchase/            # 采购示例
│     ├─ masterdata/          # 客商/物料/仓库/单位/币种/税率
│     ├─ sales/
│     ├─ inventory/
│     ├─ finance/
│     ├─ manufacturing/
│     ├─ quality/
│     ├─ project/
│     └─ reporting/
├─ packages/
│  ├─ config/                 # 前端共享 TS 配置
│  ├─ shared/                 # 类型、能力码
│  ├─ sdk/                    # API Client
│  └─ ui/                     # 共享 UI 组件
├─ db/
│  └─ migrations/             # SQL 迁移 + embed
├─ deploy/
│  ├─ compose/                # 本地依赖
│  ├─ docker/                 # 镜像构建
│  ├─ helm/                   # Helm 规划
│  ├─ saas/                   # SaaS 部署说明
│  ├─ offline/                # 离线包说明
│  └─ profiles/               # 交付 Profile
├─ tools/                     # 安装器/License/诊断（规划）
├─ api/openapi.yaml           # OpenAPI 契约
├─ docs/                      # 架构、API、交付文档
├─ .github/workflows/         # CI/CD
├─ go.mod
├─ pnpm-workspace.yaml
└─ turbo.json
```

## 环境要求

- Go 1.25.11+（Go 1.25.5 会自动下载所需工具链）
- Node.js 20+
- pnpm 8.6+
- Docker + Docker Compose
- PostgreSQL 16 / Redis 7 / RabbitMQ 3.13 / MinIO（通过 Compose 启动）

## 快速开始

### 1. 准备环境变量

PowerShell：

```powershell
Copy-Item .env.example .env
```

Bash：

```bash
cp .env.example .env
```

### 2. 启动本地依赖

```bash
docker compose -f deploy/compose/docker-compose.dev.yml up -d
```

### 3. 初始化数据库与管理员

```bash
go mod tidy
go run ./cmd/migrate -direction up
go run ./cmd/seed
```

默认管理员：

```text
用户名：admin
密码：Admin@123456
```

需要演示数据时：

```bash
go run ./cmd/demo
# 或
pnpm demo:seed
```

演示账号（仅 dev/test）：

```text
demo_admin   / Demo@123456   数据范围 all
demo_company / Demo@123456   数据范围 company
demo_dept    / Demo@123456   数据范围 dept
```

演示数据包含：4 个计量单位、2 个币种、4 个税率、4 个仓库、12 个客商、20 个物料、2 个部门组织、2 张采购订单和对应库存余额/流水。

> Stage 1 起 API 要求 PostgreSQL 可用；`DB_REQUIRED=true`。

### 4. 同时启动前端和后端

```bash
pnpm install
pnpm dev
```

`pnpm dev` 使用 `concurrently` 同时启动：

- Go API：`http://localhost:8080`
- Vite 前端：`http://localhost:5173`

按 `Ctrl+C` 会同时停止两个进程。

### 5. 分开启动（可选）

只启动后端：

```bash
pnpm dev:api
```

只启动前端：

```bash
pnpm dev:web
```

需要 Worker 时单独启动：

```bash
pnpm dev:worker
```

默认登录账号：`admin / Admin@123456`。

### 6. 启动 Worker（可选）

```bash
go run ./cmd/worker
```

## 验证 API

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/api/v1/runtime-config

# 登录，获取 access_token / refresh_token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Admin@123456"}'

# 使用返回的 access_token
curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/auth/me

curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/system/users

curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/system/code-rules

curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/masterdata/partners

curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/masterdata/materials

curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/purchase/orders

curl -X POST http://localhost:8080/api/v1/purchase/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <access_token>" \
  -d '{"code":"PO-2026-0001","supplier_id":"SUP-001","amount":"1234.5600","currency":"CNY"}'
```

采购订单当前使用内存仓储，重启 API 后数据丢失。后续替换为 PostgreSQL 仓储。

## 常用命令

```bash
# Go
go test ./...
go vet ./...
gofmt -w ./cmd ./internal ./db
go run ./cmd/migrate -direction up
go run ./cmd/migrate -direction down -steps 1
go run ./cmd/seed
go run ./cmd/demo

# Local development
pnpm install
pnpm dev          # API + Web
pnpm dev:api
pnpm dev:web
pnpm dev:worker

# Frontend checks
pnpm typecheck:web
pnpm build:web

# Docker
docker build -f deploy/docker/api.Dockerfile -t mlf-erp/api:dev .
docker build -f deploy/docker/worker.Dockerfile -t mlf-erp/worker:dev .
docker build -f deploy/docker/web.Dockerfile -t mlf-erp/web:dev .
```

## 架构约定

1. **一套核心代码，两种交付**：SaaS 与私有化共用领域代码、前端和数据库迁移；差异收敛到配置、`deploy/` 和平台适配器。
2. **模块化单体**：`internal/modules` 按业务域拆分，模块内部按 `domain / app / infra / http` 分层。
3. **依赖方向**：`http → app → domain`，`infra → domain`，`domain` 不依赖框架和数据库。
4. **金额精度**：API 使用字符串，领域使用 `shopspring/decimal`，数据库使用 `numeric`，禁止 float。
5. **租户隔离**：业务表带 `tenant_id`；唯一键和外键尽量带租户；后续接入 PostgreSQL RLS。
6. **能力控制**：业务代码判断 capability，不判断 `edition == "saas"`。
7. **构建一次，多环境晋级**：镜像使用 `sha-<commit>` 或版本标签，不使用 `latest`。

## 当前已完成

### Stage 1 平台底座

- JWT 登录、refresh token 轮换、退出、改密。
- 用户、角色、权限、组织 CRUD。
- RBAC 路由权限、登录日志、操作审计。
- 真实前端登录、401 自动刷新、权限路由/菜单。
- 用户/角色/组织/审计日志页面。

### Stage 2 主数据与数据权限

- 客商、物料、仓库、计量单位、币种、税率 CRUD。
- 编码规则：前缀、日期模式、流水号、重置策略、事务内并发生成。
- 数据权限：`all` / `company` / `dept` / `self`。
- 主数据前端页面和编码规则页面。
- 物料联动计量单位/税率。

### Stage 3A 采购与库存闭环

- 采购订单头/行、提交、审批、取消。
- 采购收货、部分收货、超收校验。
- 收货事务内更新订单行、库存余额和库存流水。
- 库存余额、库存流水查询。
- 采购订单、库存余额、库存流水前端页面。
- Demo 数据包含 2 张采购订单和对应库存。

### 工程与交付

- Monorepo 工作区与共享包。
- PostgreSQL 迁移、seed、Docker Compose。
- Dockerfile、GitHub Actions CI、SaaS/私有化发布模板。
- OpenAPI 契约与模块文档。

## 下一步

- Stage 3B：销售订单、发货出库、应收/应付。
- 采购退货、价格协议、供应商绩效。
- 审批流引擎、导入导出、附件。
- 数据范围管理 API/UI。
- 登录限流、token version、httpOnly Cookie 评估。
- Redis、MinIO、RabbitMQ 适配器。
- Outbox Worker、MRP/成本/月结计算服务。
- 报表引擎、离线 License/安装器。
- OpenAPI 生成 SDK、前端 ESLint/Prettier、单元/E2E 测试。
