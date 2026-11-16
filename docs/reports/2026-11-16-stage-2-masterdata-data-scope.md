# Stage 2 汇总报告：主数据、编码规则与数据权限

> 日期：2026-11-16  
> 分支：`stage-2-masterdata-data-scope`  
> 功能提交：`1cd7f43`  
> 审核修复提交：`38139ea`  
> 计划：`docs/superpowers/plans/2026-11-16-stage-2-masterdata-data-scope.md`  
> 状态：用户已审阅通过（2026-11-16），已补充 demo 数据

---

## 1. 完成范围

### 1.1 主数据

- 客商：客户/供应商/两者，编码、名称、分类、联系人、电话、邮箱、地址、税号、状态。
- 物料：编码、名称、规格、分类、计量单位、税率、条码、状态。
- 仓库：编码、名称、类型、地址、状态。
- 计量单位：编码、名称、小数精度、状态。
- 币种：编码、名称、符号、小数位、状态。
- 税率：编码、名称、税率、类型、状态。

### 1.2 编码规则

- `erp.code_rules` 表和 CRUD API。
- 支持前缀、日期模式（无/年/年月/年月日）、分隔符、流水号长度、重置策略（不重置/年/月/日）。
- 事务内 `SELECT ... FOR UPDATE` 生成编码。
- 默认规则：客商、物料、仓库、采购订单。

### 1.3 数据权限

- `all`：全租户。
- `company`：公司子树。
- `dept`：部门子树。
- `self`：仅本人创建。
- 应用于客商、物料、仓库的列表、详情、更新、删除。
- 单位、币种、税率做租户级隔离。
- 空组织路径/空公司路径时失败关闭（返回 0 行），不会退化为全租户可见。

### 1.4 前端

- 通用 `MasterDataCrud` 组件。
- 客商、物料、仓库、计量单位、币种、税率页面。
- 编码规则页面。
- 主数据菜单与权限路由。
- 物料页面联动单位和税率选项。

### 1.5 Demo 数据

新增 `cmd/demo`，仅允许 `APP_ENV=dev/test`（或 `ALLOW_DEMO_SEED=true`）运行，幂等、固定编码，使用现有 app service 创建：

- 4 个计量单位、2 个币种、4 个税率、4 个仓库。
- 2 个部门组织：华东事业部、华南事业部。
- 12 个客商：HQ 8 个、华东 2 个、华南 2 个。
- 20 个物料：HQ 10 个、华东 5 个、华南 5 个。
- 3 个演示账号：`demo_admin`（all）、`demo_company`（company）、`demo_dept`（dept）。

命令：

```bash
go run ./cmd/demo
# 或
pnpm demo:seed
```

演示账号密码统一为 `Demo@123456`。

### 1.6 Stage 1 安全修复

本阶段同时修复了 Stage 1 代码审核发现的 Critical/Important 问题：

- 阻止普通用户设置 `is_super_admin`、自我提权、修改超级管理员。
- 阻止自我分配系统角色、分配超出自身权限的角色、修改系统角色或授予未持有权限。
- 保护最后一个超级管理员，禁止自我禁用。
- 登录失败统一返回 401，未知用户执行 dummy bcrypt，降低用户名枚举。
- Refresh token 使用原子消费，重放已轮换 token 会撤销整个令牌族。
- 管理员重置密码后撤销该用户全部 refresh token。
- 登录只更新 `last_login_at`，避免覆盖并发禁用/改密。
- PostgreSQL 唯一键/外键错误映射为 409，非法 UUID 映射为 400。
- 组织禁止自父节点和后代父节点，防止组织树环。
- 非开发环境强制 JWT secret 长度和禁止默认值。
- 移除 `X-Tenant-ID` / `X-User-ID` 等客户端身份头信任。
- 配置可信代理，禁用默认全代理信任，避免 IP 伪造。
- 前端 refresh 后同步 Pinia 和 localStorage，并支持多标签页 token 同步。
- seed 不再错误打印已修改过的默认密码。

---

## 2. 数据库变更

### 000003_masterdata_code_rules

- `erp.units`
- `erp.currencies`
- `erp.tax_rates`
- `erp.partners`
- `erp.materials`
- `erp.warehouses`
- `erp.code_rules`
- 14 个主数据/编码规则权限
- 主数据 scope 索引

### 000004_fix_org_root_path

- 修正默认根组织 path 为 `/<id>`。

### 000005_rebuild_org_paths

- 递归重建全部组织 path，修复 Stage 1 遗留的 `/child-id` 路径。
- 将 Stage 2 新权限追加给现有 admin 角色，迁移-only 升级也能获得权限。

---

## 3. API 清单

| 方法 | 路径 | 权限 |
|---|---|---|
| GET/POST | `/api/v1/masterdata/partners` | `masterdata.partner.read/write` |
| GET/PUT/DELETE | `/api/v1/masterdata/partners/:id` | `masterdata.partner.read/write` |
| GET/POST | `/api/v1/masterdata/materials` | `masterdata.material.read/write` |
| GET/PUT/DELETE | `/api/v1/masterdata/materials/:id` | `masterdata.material.read/write` |
| GET/POST | `/api/v1/masterdata/warehouses` | `masterdata.warehouse.read/write` |
| GET/PUT/DELETE | `/api/v1/masterdata/warehouses/:id` | `masterdata.warehouse.read/write` |
| GET/POST | `/api/v1/masterdata/units` | `masterdata.unit.read/write` |
| GET/PUT/DELETE | `/api/v1/masterdata/units/:id` | `masterdata.unit.read/write` |
| GET/POST | `/api/v1/masterdata/currencies` | `masterdata.currency.read/write` |
| GET/PUT/DELETE | `/api/v1/masterdata/currencies/:id` | `masterdata.currency.read/write` |
| GET/POST | `/api/v1/masterdata/tax-rates` | `masterdata.tax.read/write` |
| GET/PUT/DELETE | `/api/v1/masterdata/tax-rates/:id` | `masterdata.tax.read/write` |
| GET/POST | `/api/v1/system/code-rules` | `system.code_rule.read/write` |
| PUT | `/api/v1/system/code-rules/:id` | `system.code_rule.write` |

列表支持 `keyword`、`page`、`page_size`；删除为禁用（`status=disabled`）。

---

## 4. 测试结果

### 4.1 静态检查与构建

| 检查 | 结果 |
|---|---|
| `gofmt -l ./cmd ./internal ./db` | 通过 |
| `go test ./...` | 通过 |
| `go vet ./...` | 通过 |
| `pnpm typecheck:web` | 通过 |
| `pnpm build:web` | 通过（Element Plus 主包约 956KB，chunk size warning） |

### 4.2 数据库迁移与 seed

- `000003`、`000004` 迁移成功。
- seed 成功，权限和默认编码规则写入。
- 重复 seed 不覆盖已有 admin 密码，输出 `password=unchanged`。

### 4.3 真实 PostgreSQL API 测试

| 场景 | 结果 |
|---|---|
| 创建计量单位、币种、税率 | 成功 |
| 客商自动编码 | `CUS-20261116-0001` |
| 物料自动编码 | `MAT-20261116-0001` |
| 仓库自动编码 | `WH-20261116-001` |
| 客商/物料/仓库列表 | 成功 |
| 客商更新、仓库禁用 | 成功 |
| 编码规则列表 | 4 条默认规则 |
| self 数据权限 | 创建前 0，创建后 1 |
| dept 数据权限 | 1（仅本部门） |
| company 数据权限 | 2（公司子树） |
| all 数据权限 | 2（admin） |
| 无写权限用户创建单位 | 403 |
| 跨租户客商查询 | 0（不可见） |
| 禁用单位后创建物料 | 400 |
| 启用单位后创建物料 | 成功，自动编码 |
| 禁用客商编码规则后创建客商 | 400（不再是 500） |
| 日期模式与重置策略冲突的编码规则 | 400 |
| 无 access token 调用 logout（仅 refresh token） | 200 |
| 测试数据清理 | 完成 |

### 4.4 新增自动化测试

- `internal/modules/masterdata/infra/postgres/repository_scope_test.go`：覆盖 all/self/dept/company 与空路径失败关闭。
- `internal/modules/system/app/code_rule_service_test.go`：覆盖日期模式与重置策略冲突校验。
- `internal/modules/system/app/auth_service_test.go`：覆盖 refresh 宽限期与宽限期外令牌族撤销。

### 4.5 Stage 1 安全回归

| 场景 | 结果 |
|---|---|
| 普通用户设置 `is_super_admin=true` | 403 |
| 自我分配 admin 角色 | 403 |
| 修改系统角色权限 | 403 |
| 授予未持有权限 | 403 |
| 禁用账号登录 | 401（统一失败） |
| 旧 refresh token 重放 | 401 |
| 重放后新 refresh token | 401（令牌族撤销） |

### 4.6 Demo 数据验证

| 账号 | 客商 | 物料 | 仓库 | 单位/币种/税率 |
|---|---:|---:|---:|---:|
| `demo_admin` | 12 | 20 | 4 | 4 / 2 / 4 |
| `demo_company` | 12 | 20 | 4 | 4 / 2 / 4 |
| `demo_dept` | 2 | 5 | 1 | 4 / 2 / 4 |

重复执行 `go run ./cmd/demo` 不产生重复数据。

---

## 5. 代码审核

### 5.1 审核方式

独立代码审核子代理，审查范围 `7b3a8c6..1cd7f43`。

### 5.2 审核结论

**初次审核：Ready to merge: No**

- Critical 1 项：数据权限在组织/公司路径为空时失败开放，可能让 dept/company 用户看到全租户主数据。
- Important 10 项：编码规则缺失/禁用返回 500、日期模式与重置策略冲突、000004 未重建子组织路径、状态字段无效且无法重新启用、会话刷新/登出跨标签页竞态、禁用单位/税率仍可被引用、Stage 2 自动化测试不足、app 层直接依赖 PostgreSQL、数据范围无产品化配置入口、seed 覆盖编码规则/admin 权限。
- Minor：序列号失败留空号、响应时间戳为零、UUID/数值溢出 500、分页溢出、索引不匹配、权限加载多次查询、CORS 空 allowlist、菜单/按钮权限细节、文档未更新。

### 5.3 已修复问题

- **Critical 数据权限失败开放**：`appendScope` 在 dept/company 路径为空时追加 `AND 1 = 0`；self 无用户 ID 时同样拒绝；`DataScopeByUserID` 在无主组织时返回 `self`；`authmw` 组织/公司查找失败时降级为 `self`。新增失败关闭单元测试。
- **编码规则缺失/禁用 500**：`resolveCode` 将 system `ErrNotFound` 转换为 masterdata `ErrInvalidInput`，接口返回 400。
- **日期/重置策略冲突**：服务层强制 `day→yyyyMMdd`、`month→yyyyMM/yyyMMdd`、`year→非 none`，并新增单元测试。
- **组织路径重建**：新增 `000005_rebuild_org_paths` 递归重建全部组织 path；seed 根组织改为 `/<id>`。
- **状态字段无效/无法启用**：移除主数据表单中的无效状态选择；新增 `POST /:id/enable` 和前端“启用”按钮；禁用语义仍为 `status=disabled`。
- **会话竞态**：logout 改为公开接口，access token 过期也能撤销 refresh token；refresh 增加 30 秒宽限期；前端增加 session epoch、refresh 失败时检查其他标签页是否已轮换 token、storage 事件强制 `loadMe`。
- **禁用单位/税率可被引用**：物料引用校验要求 `status=active`；前端单位/税率下拉只加载启用记录。
- **app 层依赖 infra**：新增 `masterdata/domain.CrudRepository[T]` 接口，服务改为依赖接口，不再 import PostgreSQL 实现。
- **seed 覆盖配置**：编码规则改为 `ON CONFLICT DO NOTHING`；admin 权限改为增量 `ON CONFLICT DO NOTHING`；迁移 000005 为现有 admin 角色补授 Stage 2 权限。
- **文档**：Stage 1 报告补充审核结论；Stage 2 报告、README、OpenAPI、迁移 README 已更新。
- **二次复核修复**：并发 refresh 在 `ConsumeByHash` 竞争失败时重新读取 token 并应用宽限期，避免误撤销赢家令牌族；logout 改为撤销该用户整个 refresh token 族；空 DataScope 默认按 `self` 处理；物料只在校验单位/税率发生变化时校验，避免已引用禁用记录无法改名称；storage 事件 `loadMe` 仅在 401 时清理会话；补充 OpenAPI logout/enable 和迁移文档。
- **新增测试**：`TestAuthServiceRefreshDoesNotRevokeFamilyOnConcurrentConsumption`、`TestAuthServiceLogoutRevokesRotatedFamily`。

### 5.4 未修复/后续处理

- 数据范围仍没有产品化 API/UI；目前通过 `org_memberships` SQL 或 seed 配置，建议 Stage 3 增加成员/数据范围管理。
- 禁用被物料引用的单位/税率尚未阻止，只阻止“新引用禁用记录”；引用完整性策略需在单据/库存阶段统一设计。
- 跨标签页 refresh 通过宽限期 + 竞争失败重读 + session epoch + storage 同步缓解，尚未实现严格的跨标签页刷新锁。
- 自动化测试仍缺少真实 PostgreSQL 数据权限矩阵、编码并发生成、迁移/seed 幂等、handler 错误码和浏览器 E2E。
- 刷新宽限期是为多标签页体验做的安全折中；如安全要求更高，可改为幂等轮换或客户端绑定。
- Minor 项（序列号空号、响应时间戳、UUID/数值错误映射、分页溢出、索引、N+1、CORS、菜单细节）记录为后续优化。
- 二次复核结论为 “With fixes”，上述复核问题已修复并通过自动化测试；仍未做第三次完整审核。

---

## 6. 已知限制

- 数据权限目前按用户主组织的 `org_memberships.data_scope` 生效；用户数据范围的 API/UI 管理尚未提供，仍通过 SQL/seed 配置。
- 单位/币种/税率是租户级参考数据，不做组织级数据权限。
- 主数据 `created_by/updated_by` 没有数据库外键到用户表。
- 编码规则并发依赖 PostgreSQL `FOR UPDATE`，尚未做高并发压测；生成失败会留下号段缺口。
- 主数据删除为禁用；禁用被物料引用的单位/税率尚未阻止，引用完整性策略需在后续统一设计。
- 跨标签页 refresh 通过宽限期 + session epoch + storage 同步缓解，尚未实现严格跨标签页刷新锁。
- 前端 Element Plus 全量引入，主包偏大。
- 登录仍无速率限制/失败锁定；访问令牌在改密后最多 15 分钟内仍可能有效（refresh 已撤销）。
- 前端 token 仍存 localStorage，XSS 风险未彻底解决。
- OpenAPI 已覆盖 Stage 1/2 主要接口，详细字段仍以代码 DTO 为准。

---

## 7. 下一阶段建议

Stage 3 建议：

- 单据框架：单据头/行、状态机、来源单据、关联关系。
- 采购订单 PostgreSQL 仓储，替换内存仓储。
- 审批流基础：提交、审批、驳回、转办、撤回。
- 导入导出、附件。
- 数据权限在单据查询中的复用。
- 主数据引用校验与禁用策略。

---

## 8. 用户审阅清单

- [x] Stage 2 范围是否符合预期。
- [x] 编码规则默认格式是否接受。
- [x] 数据权限规则是否符合业务预期。
- [x] 代码审核结论和修复是否认可。
- [x] 是否允许进入 Stage 3。
- [ ] Stage 3 目标行业（贸易/制造/项目）是否需要先确认。

> 用户审阅结论：通过（2026-11-16），并同意补充 demo 测试数据。
