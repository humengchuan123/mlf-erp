# MLF ERP

企业级 ERP Monorepo 骨架，支持 **SaaS 订阅** 与 **私有化安装** 两种交付形态。

> 当前状态：Stage 1 平台底座 + Stage 2 主数据/编码规则/数据权限 + Stage 3A 采购/库存 + Stage 3B 销售/应收应付 + Stage 3C 总账/自动凭证/基础报表 + Stage 3D 期末结转/辅助核算 + Stage 3E 税务/发票/价税分离 + Stage 3F 多币种/汇率/汇兑损益 + Stage 3G 现金流量表 + Stage 3H 销售退货/贷项单 + Stage 3I 采购退货/应付贷项单 + Stage 3J 贷项抵用/退款。
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

演示数据包含：4 个计量单位、2 个币种、4 个税率、4 个仓库、12 个客商、20 个物料、2 个部门组织、3 张采购订单、2 张销售订单、发货单、应收/应付发票、收付款、库存余额/流水、自动会计凭证、辅助核算凭证和财务报表数据。

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

curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/sales/orders

curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/sales/shipments

curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/sales/returns

curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/inventory/balances

curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/finance/invoices

curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/finance/credit-notes

curl -H "Authorization: Bearer <access_token>" \
  "http://localhost:8080/api/v1/finance/payments"

curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/finance/accounts

curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/finance/vouchers

curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/finance/reports/trial-balance
```

采购、销售、库存和财务均使用 PostgreSQL 持久化。

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
go run ./cmd/glbackfill

# Local development
pnpm install
pnpm dev          # API + Web
pnpm dev:api
pnpm dev:web
pnpm dev:worker

# Frontend checks
pnpm typecheck:web
pnpm build:web
pnpm test:frontend

# Stage 3B API integration test (starts Postgres, migrates, seeds, starts API)
pnpm test:api:stage3b

# Stage 3C API integration test (GL, auto vouchers, reports)
pnpm test:api:stage3c

# Stage 3D API integration test (period close, auxiliary accounting)
pnpm test:api:stage3d

# Stage 3E API integration test (tax, VAT invoices, price-tax separation)
pnpm test:api:stage3e

# Stage 3F API integration test (multi-currency, FX settlement and revaluation)
pnpm test:api:stage3f

# Stage 3G API integration test (cash flow statement)
# Writes to the local dev database and consumes a clean year; use a clean database for full regression.
pnpm test:api:stage3g

# Stage 3H API integration test (sales returns, credit notes)
pnpm test:api:stage3h

# Stage 3I API integration test (purchase returns, payable credit notes)
# Writes to the local dev database and consumes a clean year for FX revaluation; use a clean database for full regression.
pnpm test:api:stage3i

# Stage 3J API integration test (credit note allocation and refunds)
# Writes to the local dev database and consumes a clean year for period-close rollback; use a clean database for full regression.
pnpm test:api:stage3j

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

### Stage 3B 销售、发货与应收应付

- 销售订单头/行、提交、审批、取消。
- 部分发货、全部发货、超发校验、重复行校验、库存不足校验。
- 发货事务内扣减库存、写库存流水、生成应收发票。
- 库存移动平均成本 `avg_cost`，发货按平均成本记出库成本。
- 采购收货自动生成应付发票。
- 统一应收/应付发票、收款/付款、部分核销、重复/超额/类型/往来单位/币种校验。
- 销售订单、发货单、应收应付、收付款前端页面。
- Stage 3B 真实 HTTP API 集成测试和前端完整度检查。

### Stage 3C 总账与基础报表

- 会计科目表、会计期间、关账/反关账。
- 手工凭证录入、过账、反过账。
- 销售发货、采购收货、客户收款、供应商付款自动生成凭证。
- 自动凭证按来源幂等，业务事务内同步入账。
- 科目余额表、总账/明细账、资产负债表、利润表。
- 科目、期间、凭证、报表前端页面。
- `cmd/glbackfill` 为历史业务单据补生成凭证。
- Stage 3C 真实 HTTP API 集成测试和前端完整度检查。

### Stage 3D 期末结转与辅助核算

- 关账自动生成损益结转凭证，收入/费用结转到本年利润 4103。
- 反关账生成反向结转凭证，期间恢复 open；再次关账生成新结转凭证。
- 年度结转：本年利润 4103 结转到利润分配 4104；要求 12 个期间全部关账且有关账记录，按年度顺序执行，结转后不可反关账。
- 凭证行支持客商、部门、项目辅助核算。
- 辅助余额表按客商/部门/项目汇总。
- 报表严格应用组织数据范围，部门用户不显示公司级结转凭证。
- 期间页面增加创建年度期间、年度结转状态；凭证页面增加辅助字段；报表增加辅助余额表。
- Stage 3D 真实 HTTP API 集成测试和前端完整度检查。

### Stage 3E 税务/发票与价税分离

- 销售订单、销售发货、采购订单、采购收货支持含税/不含税录入和行级税率，默认取物料税率。
- 单据行保存不含税金额、税额、含税金额和税率快照；表头保存净额、税额、含税总额。
- 销售发货自动凭证拆分收入与销项税；采购收货拆分库存与进项税。
- 应收/应付发票增加净额、税额、增值税发票号、开票状态和购销方税号，支持开票/作废。
- 新增增值税汇总表（按月销项、进项、应纳税额）。
- 前端订单/发货/发票/报表增加税务字段和操作；SDK/OpenAPI 同步。
- Stage 3E 真实 HTTP API 集成测试和前端完整度检查。

### Stage 3F 多币种/汇率/汇兑损益

- 新增汇率表，按币种和生效日期维护；单据默认取最近汇率并可覆盖。
- 销售/采购订单、发货、收货、应收应付发票、收付款保存原币金额、汇率快照和 CNY 本位币金额。
- 销售发货、采购收货自动凭证按本位币入账。
- 收付款核销按付款汇率与发票汇率差额生成已实现汇兑损益凭证。
- 期末关账自动重估未核销外币应收应付，次期初反向；反关账撤销重估。
- 新增 660301 汇兑损失、660302 汇兑收益科目和映射。
- 前端增加汇率管理、本位币金额和汇率字段；SDK/OpenAPI 同步。
- Stage 3F 真实 HTTP API 集成测试和前端完整度检查。

### Stage 3G 现金流量表

- 直接法现金流量表：经营活动、投资活动、筹资活动的现金流入、流出和净额。
- 18 个标准现金流项目，按对方科目编码前缀自动归类，现金/银行映射支持子科目。
- 无法归类的现金凭证进入“其他经营活动现金流”，并返回未分类凭证明细和原因。
- 简化间接法附表：净利润、非付现费用、经营性应收/存货/应付/税费/职工薪酬变动、间接法经营净额、直接法经营净额和差额。
- 期初现金、期末现金和现金净增加额，按现金/银行科目总账余额计算。
- API、SDK、OpenAPI 和财务报表前端页签同步；权限沿用 `finance.report.read`。
- Stage 3G 真实 HTTP API 集成测试和前端完整度检查。
- API 测试命令：`pnpm test:api:stage3g`（写入本地开发库并消耗一个干净年度，完整回归请使用干净数据库）。

### Stage 3H 销售退货与贷项单

- 按原发货行部分/全部退货，累计退货数量校验，超退返回 `OVER_RETURN`。
- 库存按原发货成本回库并重算移动平均成本，写入 `sales_return` 库存流水。
- 冲收入/销项税/应收，同时回库存/冲成本，生成 `sales_return` 自动凭证。
- 贷项单自动冲原发票未收金额，超出部分保留为客户贷项（`open`）。
- 发票增加已冲贷项金额，未收金额和状态扣减 credited（`paid` 前端显示“已结清”）。
- 期间外币重估按扣减 credited 后的未收金额计算，已全额冲减的发票不再重估。
- 退货/贷项单 API、SDK、OpenAPI 和前端页面同步；权限 `sales.return.read/write`。
- Stage 3H 真实 HTTP API 集成测试和前端完整度检查。
- API 测试命令：`pnpm test:api:stage3h`（写入本地开发库并消耗数据，完整回归请使用干净数据库）。

### Stage 3I 采购退货与应付贷项单

- 按原收货行部分/全部退货，累计退货数量校验，超退返回 `OVER_RETURN`。
- 库存按原采购成本出库并重算移动平均成本，写入 `purchase_return` 库存流水。
- 冲应付/进项税/库存，生成 `purchase_return` 自动凭证。
- 贷项单自动冲原应付发票未付金额，超出部分保留为供应商贷项（`open`）；已付款发票退货不减少 `credited_amount`。
- 发票增加已冲贷项金额，未付金额和状态扣减 credited（`paid` 前端显示“已结清”）。
- 期间外币重估按扣减 credited 后的未付金额计算，已全额冲减的发票不再重估。
- 退货/贷项单 API、SDK、OpenAPI 和前端页面同步；权限 `purchase.return.read/write`。
- Stage 3I 真实 HTTP API 集成测试和前端完整度检查。
- API 测试命令：`pnpm test:api:stage3i`（写入本地开发库并消耗一个干净年度，完整回归请使用干净数据库）。

### Stage 3J 贷项抵用与退款

- 销售退货贷项可手工抵同客户其他应收发票，采购退货贷项可抵同供应商应付发票，支持部分/多次/多张抵用。
- 抵用按贷项剩余原币/本位币比例计算，更新发票 `credited_amount`、未收金额和状态；抵用为子账操作，不生成凭证。
- 贷项单增加 `refunded_amount`、`refunded_base_amount`，余额口径为 `amount - applied - refunded`。
- 客户/供应商退款支持银行/现金、部分/多次退款；外币退款按退款日汇率确认现金本位币和汇兑损益，生成 `credit_note_refund` 凭证。
- 退款受期间关账控制，关账期间返回 `PERIOD_CLOSED` 并整体回滚；退款不修改发票 `credited_amount`。
- 贷项详情返回抵用/退款历史，发票详情返回 `credit_note_allocations`；候选发票按同组织/往来单位/币种/未结清过滤。
- 新增权限 `finance.credit_note.apply`、`finance.credit_note.refund`；API、SDK、OpenAPI 和前端同步。
- 抵用/退款并发通过贷项行锁串行化，余额和本位币不变量不被破坏。
- Stage 3J 真实 HTTP API 集成测试和前端完整度检查。
- API 测试命令：`pnpm test:api:stage3j`（写入本地开发库并消耗一个干净年度用于关账回滚，完整回归请使用干净数据库）。

### 工程与交付

- Monorepo 工作区与共享包。
- PostgreSQL 迁移、seed、Docker Compose。
- Dockerfile、GitHub Actions CI、SaaS/私有化发布模板。
- CI 包含单元测试、构建、Stage 3B/3C/3D/3E/3F/3G/3H/3I/3J 真实 PostgreSQL API 集成测试。
- OpenAPI 契约与模块文档。

## 下一步

- 银行/现金外币重估、跨币种核销、多本位币/合并报表折算。
- 价格协议、客户信用。
- 收付款反核销、红字发票、账龄分析。
- 审批流引擎、导入导出、附件。
- 数据范围管理 API/UI。
- 登录限流、token version、httpOnly Cookie 评估。
- Redis、MinIO、RabbitMQ 适配器。
- Outbox Worker、MRP/成本/月结计算服务。
- 报表引擎、离线 License/安装器。
- OpenAPI 生成 SDK、前端 ESLint/Prettier、单元/E2E 测试。
