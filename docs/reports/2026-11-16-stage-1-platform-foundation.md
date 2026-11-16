# Stage 1 汇总报告：平台底座（认证、组织、RBAC、审计）

> 日期：2026-11-16  
> 分支：`stage-1-platform-foundation`  
> 提交：`7b3a8c6`  
> 计划：`docs/superpowers/plans/2026-11-16-stage-1-platform-foundation.md`  
> 状态：用户已审阅通过，进入 Stage 2（2026-11-16）

---

## 1. 完成范围

### 1.1 后端

- JWT access token（HS256，默认 15 分钟）。
- Refresh token（随机 32 字节，数据库只存 SHA-256，7 天有效）。
- 登录、刷新、退出、当前用户、修改密码。
- 登录成功/失败日志。
- 用户、角色、权限、组织 CRUD。
- 用户角色分配、角色权限分配。
- 组织树与路径维护。
- 操作审计日志（用户/角色/组织变更）。
- RBAC 中间件与路由权限守卫。
- 默认管理员 seed。
- 采购示例接口接入 `purchase.read` / `purchase.write` 权限。

### 1.2 前端

- 真实登录页，默认账号 `admin / Admin@123456`。
- access/refresh token 持久化。
- 请求自动带 `Authorization`。
- 401 自动刷新一次，刷新失败清理会话并跳转登录。
- 路由守卫、权限菜单、403 页面。
- 用户管理、角色权限、组织管理、审计日志页面。
- 采购订单页面接入真实登录。

### 1.3 数据库

新增迁移 `000002_platform_auth_rbac_audit`：

- `erp.permissions`
- `erp.role_permissions`
- `erp.refresh_tokens`
- `erp.login_logs`
- `erp.audit_logs`
- `erp.users.is_super_admin`
- `erp.users.last_login_at`
- `erp.users.primary_org_id`
- `erp.roles.is_system`
- 15 个基础权限码

---

## 2. API 清单

### 公开

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/auth/login` | 用户名/密码登录 |
| POST | `/api/v1/auth/refresh` | 刷新 access/refresh token |

### 需要登录

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/auth/logout` | 撤销 refresh token |
| GET | `/api/v1/auth/me` | 当前用户、角色、权限 |
| POST | `/api/v1/auth/change-password` | 修改密码并撤销全部 refresh token |
| GET/POST | `/api/v1/system/users` | 用户列表/创建 |
| PUT/DELETE | `/api/v1/system/users/:id` | 更新/禁用用户 |
| GET/POST | `/api/v1/system/users/:id/roles` | 查询/设置用户角色 |
| GET/POST | `/api/v1/system/roles` | 角色列表/创建 |
| PUT/DELETE | `/api/v1/system/roles/:id` | 更新/删除角色 |
| PUT | `/api/v1/system/roles/:id/permissions` | 设置角色权限 |
| GET | `/api/v1/system/permissions` | 权限列表 |
| GET/POST | `/api/v1/system/orgs` | 组织树/创建 |
| PUT/DELETE | `/api/v1/system/orgs/:id` | 更新/删除组织 |
| GET | `/api/v1/system/audit-logs` | 审计日志分页 |
| GET | `/api/v1/purchase/orders` | 需要 `purchase.read` |
| POST | `/api/v1/purchase/orders` | 需要 `purchase.write` |

---

## 3. 测试结果

### 3.1 静态检查与构建

| 检查 | 结果 |
|---|---|
| `gofmt -l ./cmd ./internal ./db` | 通过，无输出 |
| `go test ./...` | 通过 |
| `go vet ./...` | 通过 |
| `pnpm typecheck:web` | 通过 |
| `pnpm build:web` | 通过（Element Plus 主包约 953KB，存在 chunk size warning，不阻塞） |

### 3.2 数据库与 API 冒烟测试

环境：Docker Desktop + PostgreSQL 16，真实数据库。

| 场景 | 结果 |
|---|---|
| 迁移 `000001` + `000002` | 成功 |
| `cmd/seed` | 成功，创建 admin / Admin@123456 |
| 管理员登录 | 成功，15 个权限，角色 `admin` |
| `/auth/me` | 成功，`is_super_admin=true` |
| 用户/角色/权限/组织列表 | 成功，users=1，roles=1，permissions=15，orgs=1 |
| 创建测试角色并分配 `purchase.read`、`system.user.read` | 成功 |
| 创建测试用户并分配测试角色 | 成功 |
| 测试用户访问用户列表 | 200 |
| 测试用户访问角色列表 | 403 |
| 测试用户读取采购订单 | 200 |
| 测试用户创建采购订单 | 403 |
| 无 token 访问 `/auth/me` | 401 |
| refresh token 轮换 | 成功 |
| 旧 refresh token 再次使用 | 401 |
| logout 后 refresh | 401 |
| 审计日志 | 成功记录 user.create、role.create、role.set_permissions |
| 测试数据清理 | 已清理测试用户、角色和对应审计数据 |

---

## 4. 代码审核

### 4.1 审核方式

独立代码审核子代理，审查范围 `4269444..7b3a8c6`。

### 4.2 审核结论

> 待补充。

### 4.3 已修复问题

> 待补充。

### 4.4 未修复/后续处理

> 待补充。

---

## 5. 已知限制

- 数据权限（本人/部门/公司/全部）尚未落到查询层，目前只做租户隔离和功能权限。
- 字段权限、按钮权限、审批流尚未实现。
- 登录没有速率限制、失败锁定、验证码、MFA/SSO。
- Access token 在修改密码后仍可能有效至过期（默认 15 分钟），后续可增加 token version 或黑名单。
- 前端 token 存在 `localStorage`，存在 XSS 场景下的令牌泄露风险；后续可评估 httpOnly Cookie 方案。
- 采购订单仍使用内存仓储，后续替换为 PostgreSQL。
- 审计日志目前记录操作后数据，字段级 before/after diff 尚未实现。
- 多组织/多租户管理页面尚未实现，当前只有默认租户和默认组织。
- 前端 Element Plus 全量引入导致主包偏大，后续改为按需引入。

---

## 6. 下一阶段建议

Stage 2 建议范围：

- 主数据：客商、物料、仓库、币种、税率、计量单位。
- 编码规则。
- 单据头/行模型、状态机。
- 审批流基础能力。
- 导入导出、附件。
- 数据权限（本人/部门/公司/全部）落地。
- 采购订单 PostgreSQL 仓储。

---

## 7. 用户审阅清单

- [x] Stage 1 范围是否符合预期。
- [x] 默认账号和密码策略是否接受。
- [x] 代码审核结论和修复是否认可。
- [x] 是否允许进入 Stage 2。
- [ ] Stage 3 目标行业（贸易/制造/项目）是否需要提前确认。

> 用户审阅结论：通过，进入 Stage 2（2026-11-16）。
