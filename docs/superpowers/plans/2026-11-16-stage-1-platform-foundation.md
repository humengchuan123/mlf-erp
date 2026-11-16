# Stage 1 平台底座：认证、组织、RBAC 与审计 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有 `mlf-erp` 骨架上落地平台底座第一版：JWT 登录、租户/组织、用户、角色、权限、审计日志，并完成前端系统管理页面与接口测试。

**Architecture:** 在 `internal/modules/system` 内按 `domain / app / infra / http` 分层实现平台管理模块；`internal/platform` 提供安全、上下文、HTTP 基础能力；PostgreSQL 持久化，测试使用内存 fake；前端在 `packages/sdk` 增加认证/系统 API，在 `apps/web` 增加真实登录、权限路由和系统管理页面。

**Tech Stack:** Go 1.25 + Gin + pgx + golang-migrate + bcrypt + golang-jwt/jwt/v5；Vue 3 + TypeScript + Element Plus + Pinia + Vue Router；PostgreSQL 16。

---

## 阶段总览

| 阶段 | 范围 | 状态 |
|---|---|---|
| Stage 0 | Monorepo 骨架、采购纵向示例、CI/CD 模板 | 已完成 |
| **Stage 1** | **认证、租户/组织、用户/角色/权限、审计** | **本计划** |
| Stage 2 | 主数据、编码规则、单据头行、状态机、审批流、导入导出、附件 | 待审阅后开始 |
| Stage 3 | 首个业务闭环（采购→库存→销售→应收应付→总账→报表） | 需确认目标行业 |
| Stage 4 | 离线安装、License、升级、备份、监控、诊断 | 待开始 |
| Stage 5 | 生产/BOM/MRP/成本、质量、项目、BI、开放平台 | 待开始 |

> 每个 Stage 完成后执行：代码审核 → 接口测试 → 汇总报告 → 用户审阅 → 下一阶段。

---

## Stage 1 范围

### 数据模型

新增或修改表：

- `erp.permissions`：全局权限码。
- `erp.role_permissions`：角色-权限。
- `erp.refresh_tokens`：刷新令牌。
- `erp.login_logs`：登录日志。
- `erp.audit_logs`：操作审计。
- `erp.users`：增加 `is_super_admin`、`last_login_at`。
- `erp.roles`：增加 `is_system`。

### 后端 API

公开：

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`

需要登录：

- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/me`
- `POST /api/v1/auth/change-password`

系统管理（按权限）：

- `GET/POST /api/v1/system/users`
- `PUT/DELETE /api/v1/system/users/:id`
- `POST /api/v1/system/users/:id/roles`
- `GET/POST /api/v1/system/roles`
- `PUT/DELETE /api/v1/system/roles/:id`
- `PUT /api/v1/system/roles/:id/permissions`
- `GET /api/v1/system/permissions`
- `GET/POST /api/v1/system/orgs`
- `PUT/DELETE /api/v1/system/orgs/:id`
- `GET /api/v1/system/audit-logs`

业务示例：

- `GET /api/v1/purchase/orders` 需要 `purchase.read`。
- `POST /api/v1/purchase/orders` 需要 `purchase.write`。

### 前端

- 真实登录，保存 access/refresh token。
- 请求自动带 `Authorization`，401 自动刷新一次。
- `/auth/me` 恢复会话。
- 菜单/路由按权限显示。
- 系统管理：用户、角色、组织、审计日志。
- 采购订单页面接入真实登录。

### 默认账号

`cmd/seed` 创建：

- 租户：`default` / `00000000-0000-0000-0000-000000000001`
- 组织：`HQ` / `00000000-0000-0000-0000-000000000002`
- 管理员：`admin` / `Admin@123456`
- 角色：`admin`（全部权限）
- 权限码：平台、系统管理、采购、销售、库存、财务等基础权限。

---

## File Structure

```text
cmd/
├─ api/main.go                         # 装配 system 模块、认证中间件
├─ seed/main.go                        # 初始化管理员/角色/权限
└─ migrate/main.go                     # 不变

internal/modules/system/
├─ domain/
│  ├─ user.go
│  ├─ role.go
│  ├─ permission.go
│  ├─ org.go
│  ├─ refresh_token.go
│  ├─ audit_log.go
│  ├─ login_log.go
│  ├─ errors.go
│  └─ repositories.go
├─ app/
│  ├─ auth_service.go
│  ├─ user_service.go
│  ├─ role_service.go
│  ├─ org_service.go
│  └─ audit_service.go
├─ infra/postgres/
│  ├─ user_repository.go
│  ├─ role_repository.go
│  ├─ permission_repository.go
│  ├─ org_repository.go
│  ├─ refresh_token_repository.go
│  ├─ audit_repository.go
│  └─ login_log_repository.go
└─ http/
   ├─ auth_handler.go
   ├─ user_handler.go
   ├─ role_handler.go
   ├─ org_handler.go
   ├─ audit_handler.go
   └─ dto.go

internal/platform/
├─ auth/context.go                     # Actor 扩展权限
├─ authmw/middleware.go                # JWT 认证 + 权限守卫
├─ security/password.go                # bcrypt
├─ security/token.go                   # JWT + refresh token
└─ config/config.go                    # JWT/TTL 配置

db/migrations/
├─ 000002_platform_auth_rbac_audit.up.sql
└─ 000002_platform_auth_rbac_audit.down.sql

packages/sdk/src/
├─ client.ts                          # token、401 刷新
├─ auth.ts                            # login/refresh/me/logout
└─ system.ts                          # users/roles/orgs/audit

apps/web/src/
├─ stores/auth.ts
├─ router/index.ts                    # 权限路由
├─ views/LoginView.vue                # 真实登录
├─ views/system/UserListView.vue
├─ views/system/RoleListView.vue
├─ views/system/OrgListView.vue
└─ views/system/AuditLogView.vue
```

---

## Task 1: 配置与安全基础

**Files:**
- Modify: `internal/platform/config/config.go`
- Create: `internal/platform/security/password.go`
- Create: `internal/platform/security/token.go`
- Test: `internal/platform/security/password_test.go`
- Test: `internal/platform/security/token_test.go`

- [ ] **Step 1: 扩展配置**

在 `Config` 增加：

```go
type AuthConfig struct {
    JWTSecret       string
    JWTIssuer       string
    AccessTokenTTL  time.Duration
    RefreshTokenTTL time.Duration
}
```

环境变量：

```text
JWT_SECRET=dev-only-change-me
JWT_ISSUER=mlf-erp
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=168h
```

生产环境校验：`APP_ENV=prod` 且 `JWT_SECRET` 为空或等于 `dev-only-change-me` 时返回错误。

- [ ] **Step 2: 密码哈希**

```go
func HashPassword(password string) (string, error)
func CheckPassword(hash, password string) bool
```

使用 `golang.org/x/crypto/bcrypt`，默认 cost。

- [ ] **Step 3: JWT 与刷新令牌**

```go
type Claims struct {
    UserID   string `json:"sub"`
    TenantID string `json:"tid"`
    OrgID    string `json:"oid"`
    Username string `json:"username"`
    Type     string `json:"typ"`
    jwt.RegisteredClaims
}

type TokenService struct {
    secret    []byte
    issuer    string
    accessTTL time.Duration
}

func NewTokenService(secret, issuer string, accessTTL time.Duration) *TokenService
func (s *TokenService) GenerateAccessToken(userID, tenantID, orgID, username string) (string, time.Time, error)
func (s *TokenService) ParseAccessToken(raw string) (*Claims, error)
func GenerateRefreshToken() (raw string, hash string, err error)
```

刷新令牌：32 字节随机数，base64.RawURLEncoding；数据库存 SHA-256 hex。

- [ ] **Step 4: 测试**

```bash
go test ./internal/platform/security/...
```

覆盖：正确密码、错误密码、JWT 生成/解析、过期令牌、错误签名、刷新令牌哈希稳定性。

---

## Task 2: 数据库迁移 000002

**Files:**
- Create: `db/migrations/000002_platform_auth_rbac_audit.up.sql`
- Create: `db/migrations/000002_platform_auth_rbac_audit.down.sql`

- [ ] **Step 1: 写迁移 SQL**

核心表：

```sql
ALTER TABLE erp.users ADD COLUMN IF NOT EXISTS is_super_admin boolean NOT NULL DEFAULT false;
ALTER TABLE erp.users ADD COLUMN IF NOT EXISTS last_login_at timestamptz;
ALTER TABLE erp.roles ADD COLUMN IF NOT EXISTS is_system boolean NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS erp.permissions (
    code text PRIMARY KEY,
    name text NOT NULL,
    module text NOT NULL,
    type text NOT NULL DEFAULT 'api',
    description text NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS erp.role_permissions (
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    role_id uuid NOT NULL,
    permission_code text NOT NULL REFERENCES erp.permissions (code),
    PRIMARY KEY (tenant_id, role_id, permission_code),
    CONSTRAINT role_permissions_role_fk FOREIGN KEY (tenant_id, role_id)
        REFERENCES erp.roles (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS erp.refresh_tokens (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES erp.tenants (id),
    user_id uuid NOT NULL,
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    user_agent text NOT NULL DEFAULT '',
    ip text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT refresh_tokens_user_fk FOREIGN KEY (tenant_id, user_id)
        REFERENCES erp.users (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS erp.login_logs (
    id uuid PRIMARY KEY,
    tenant_id uuid,
    user_id uuid,
    username text NOT NULL,
    status text NOT NULL,
    failure_reason text NOT NULL DEFAULT '',
    ip text NOT NULL DEFAULT '',
    user_agent text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS erp.audit_logs (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid,
    user_id uuid,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text NOT NULL DEFAULT '',
    request_id text NOT NULL DEFAULT '',
    ip text NOT NULL DEFAULT '',
    user_agent text NOT NULL DEFAULT '',
    before_data jsonb,
    after_data jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS refresh_tokens_user_idx
    ON erp.refresh_tokens (tenant_id, user_id, expires_at);
CREATE INDEX IF NOT EXISTS audit_logs_tenant_created_idx
    ON erp.audit_logs (tenant_id, created_at DESC);

INSERT INTO erp.permissions (code, name, module, type) VALUES
('system.user.read', '查看用户', 'system', 'api'),
('system.user.write', '管理用户', 'system', 'api'),
('system.role.read', '查看角色', 'system', 'api'),
('system.role.write', '管理角色', 'system', 'api'),
('system.org.read', '查看组织', 'system', 'api'),
('system.org.write', '管理组织', 'system', 'api'),
('system.audit.read', '查看审计日志', 'system', 'api'),
('purchase.read', '查看采购', 'purchase', 'api'),
('purchase.write', '管理采购', 'purchase', 'api'),
('sales.read', '查看销售', 'sales', 'api'),
('sales.write', '管理销售', 'sales', 'api'),
('inventory.read', '查看库存', 'inventory', 'api'),
('inventory.write', '管理库存', 'inventory', 'api'),
('finance.read', '查看财务', 'finance', 'api'),
('finance.write', '管理财务', 'finance', 'api')
ON CONFLICT (code) DO NOTHING;
```

Down：删除新增索引/表，撤销新增列。

- [ ] **Step 2: 验证迁移**

```bash
go run ./cmd/migrate -direction up
go run ./cmd/migrate -direction down -steps 1
go run ./cmd/migrate -direction up
```

需要 PostgreSQL 运行；若 Docker 未启动，记录为待验证项。

---

## Task 3: 领域模型与仓储接口

**Files:**
- Create: `internal/modules/system/domain/*.go`
- Test: `internal/modules/system/domain/*_test.go`

- [ ] **Step 1: 定义实体**

- `User`：ID、TenantID、Username、DisplayName、Email、PasswordHash、Status、IsSuperAdmin、LastLoginAt、CreatedAt、UpdatedAt。
- `Role`：ID、TenantID、Code、Name、IsSystem、Permissions。
- `Permission`：Code、Name、Module、Type、Description。
- `Org`：ID、TenantID、ParentID、Type、Code、Name、Path、Status、CreatedAt、UpdatedAt。
- `RefreshToken`：ID、TenantID、UserID、TokenHash、ExpiresAt、RevokedAt、UserAgent、IP、CreatedAt。
- `AuditLog`：ID、TenantID、OrgID、UserID、Action、ResourceType、ResourceID、RequestID、IP、UserAgent、BeforeData、AfterData、CreatedAt。
- `LoginLog`：ID、TenantID、UserID、Username、Status、FailureReason、IP、UserAgent、CreatedAt。

- [ ] **Step 2: 定义接口**

`repositories.go` 包含：

```go
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    Update(ctx context.Context, user *User) error
    FindByID(ctx context.Context, tenantID, id string) (*User, error)
    FindByUsername(ctx context.Context, tenantID, username string) (*User, error)
    List(ctx context.Context, tenantID, keyword string, offset, limit int) ([]*User, int, error)
    SetRoles(ctx context.Context, tenantID, userID string, roleIDs []string) error
    GetRoleIDs(ctx context.Context, tenantID, userID string) ([]string, error)
}

type RoleRepository interface {
    Create(ctx context.Context, role *Role) error
    Update(ctx context.Context, role *Role) error
    Delete(ctx context.Context, tenantID, id string) error
    FindByID(ctx context.Context, tenantID, id string) (*Role, error)
    List(ctx context.Context, tenantID string) ([]*Role, error)
    SetPermissions(ctx context.Context, tenantID, roleID string, permissionCodes []string) error
    PermissionCodesByUserID(ctx context.Context, tenantID, userID string) ([]string, error)
    RoleCodesByUserID(ctx context.Context, tenantID, userID string) ([]string, error)
}

type PermissionRepository interface {
    List(ctx context.Context) ([]*Permission, error)
}

type OrgRepository interface {
    Create(ctx context.Context, org *Org) error
    Update(ctx context.Context, org *Org) error
    Delete(ctx context.Context, tenantID, id string) error
    FindByID(ctx context.Context, tenantID, id string) (*Org, error)
    List(ctx context.Context, tenantID string) ([]*Org, error)
}

type RefreshTokenRepository interface {
    Create(ctx context.Context, token *RefreshToken) error
    FindByHash(ctx context.Context, hash string) (*RefreshToken, error)
    RevokeByHash(ctx context.Context, hash string) error
    RevokeAllForUser(ctx context.Context, tenantID, userID string) error
    DeleteExpired(ctx context.Context) error
}

type AuditRepository interface {
    Create(ctx context.Context, log *AuditLog) error
    List(ctx context.Context, tenantID string, offset, limit int) ([]*AuditLog, int, error)
}

type LoginLogRepository interface {
    Create(ctx context.Context, log *LoginLog) error
}
```

- [ ] **Step 3: 领域错误**

`errors.go`：`ErrNotFound`、`ErrConflict`、`ErrInvalidCredentials`、`ErrUserDisabled`、`ErrForbidden`、`ErrInvalidToken`、`ErrOrgHasChildren`、`ErrSystemRole`。

- [ ] **Step 4: 测试**

```bash
go test ./internal/modules/system/domain/...
```

---

## Task 4: PostgreSQL 仓储

**Files:**
- Create: `internal/modules/system/infra/postgres/*.go`

- [ ] **Step 1: 实现 UserRepository**

SQL 使用 `erp.users`、`erp.user_roles`。`List` 返回分页数据和总数。`SetRoles` 在事务中先删后插。所有查询带 `tenant_id`。

- [ ] **Step 2: 实现 RoleRepository**

`List` 返回角色及权限码；`PermissionCodesByUserID` 联查 `user_roles → role_permissions → permissions`；`RoleCodesByUserID` 联查 `user_roles → roles`。

- [ ] **Step 3: 实现 Permission/Org/RefreshToken/Audit/LoginLog 仓储**

- Org List 返回平铺列表，Tree 在 app 层组装。
- RefreshToken 按 hash 查、按 hash 撤销、按用户撤销、删除过期。
- AuditList 按 `created_at DESC` 分页。

- [ ] **Step 4: 编译与测试**

```bash
go build ./...
go test ./internal/modules/system/...
```

数据库集成测试在 `TEST_DATABASE_URL` 存在时运行，否则 `t.Skip`。

---

## Task 5: 认证与应用服务

**Files:**
- Create: `internal/modules/system/app/auth_service.go`
- Create: `internal/modules/system/app/user_service.go`
- Create: `internal/modules/system/app/role_service.go`
- Create: `internal/modules/system/app/org_service.go`
- Create: `internal/modules/system/app/audit_service.go`
- Test: `internal/modules/system/app/*_test.go`

- [ ] **Step 1: AuthService**

```go
type AuthService struct {
    users        domain.UserRepository
    roles        domain.RoleRepository
    tokens       domain.RefreshTokenRepository
    loginLogs    domain.LoginLogRepository
    tokenService *security.TokenService
    refreshTTL   time.Duration
}

type LoginInput struct {
    TenantID  string
    Username  string
    Password  string
    IP        string
    UserAgent string
}

type LoginResult struct {
    AccessToken  string
    RefreshToken string
    ExpiresIn    int64
    User         *domain.User
    Roles        []string
    Permissions  []string
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*LoginResult, error)
func (s *AuthService) Refresh(ctx context.Context, rawToken, ip, userAgent string) (*LoginResult, error)
func (s *AuthService) Logout(ctx context.Context, rawToken string) error
func (s *AuthService) Me(ctx context.Context, tenantID, userID string) (*MeResult, error)
func (s *AuthService) ChangePassword(ctx context.Context, tenantID, userID, oldPassword, newPassword string) error
```

规则：

- 用户不存在/密码错误/禁用：返回 `ErrInvalidCredentials`，写登录失败日志。
- 登录成功：更新 `last_login_at`，写登录成功日志，签发 access/refresh。
- Refresh：校验 hash、过期、撤销；轮换 refresh token。
- ChangePassword：校验旧密码；更新 hash；撤销该用户全部 refresh token。

- [ ] **Step 2: UserService / RoleService / OrgService**

- User：列表、创建、更新、删除（软删除或 status=disabled）、设置角色。
- Role：列表、创建、更新、删除（系统角色不可删）、设置权限。
- Org：列表树、创建、更新、删除（有子节点不可删）。

- [ ] **Step 3: AuditService**

```go
type AuditService struct { repo domain.AuditRepository }
func (s *AuditService) Record(ctx context.Context, entry domain.AuditLog) error
func (s *AuditService) List(ctx context.Context, tenantID string, offset, limit int) ([]*domain.AuditLog, int, error)
```

- [ ] **Step 4: 测试**

使用 fake repositories 覆盖：

- 登录成功/失败/禁用。
- Refresh 轮换与过期。
- 修改密码后旧 refresh token 失效。
- 用户创建重复用户名返回冲突。
- 系统角色不可删除、组织有子节点不可删除。

```bash
go test ./internal/modules/system/app/...
```

---

## Task 6: HTTP 中间件与 Handler

**Files:**
- Modify: `internal/platform/auth/context.go`
- Create: `internal/platform/authmw/middleware.go`
- Create: `internal/modules/system/http/*.go`
- Modify: `internal/platform/httpserver/server.go`
- Modify: `cmd/api/main.go`
- Modify: `internal/modules/purchase/http/handler.go`

- [ ] **Step 1: 扩展 Actor**

```go
type Actor struct {
    UserID       string
    UserName     string
    TenantID     string
    OrgID        string
    Roles        []string
    Permissions  []string
    IsSuperAdmin bool
}
```

- [ ] **Step 2: authmw**

```go
func Auth(tokenService *security.TokenService, users domain.UserRepository, roles domain.RoleRepository) gin.HandlerFunc
func RequirePermission(code string) gin.HandlerFunc
```

- 解析 `Authorization: Bearer <token>`。
- 校验 token 类型、用户状态。
- 加载角色码和权限码，写入 context。
- `RequirePermission` 支持 super admin 直通。
- 失败返回 401/403 统一响应。

- [ ] **Step 3: Handler**

统一 DTO：

```json
{
  "code": "OK",
  "message": "ok",
  "data": {},
  "request_id": "..."
}
```

列表响应：

```json
{
  "data": {
    "items": [],
    "total": 0
  }
}
```

- [ ] **Step 4: 路由装配**

`cmd/api/main.go`：

- 创建 `security.NewTokenService`。
- 创建 system repositories/services。
- `api := engine.Group("/api/v1")`
- 公开：`auth/login`、`auth/refresh`。
- 受保护：`auth/logout`、`auth/me`、`auth/change-password`、`system/*`。
- 采购路由加上 `purchase.read` / `purchase.write` 权限。
- 取消前端伪登录依赖的 `X-Tenant-ID` 作为唯一身份来源；认证后以 JWT 中的租户为准。

- [ ] **Step 5: 测试**

```bash
go test ./internal/modules/system/...
go test ./...
```

---

## Task 7: Seed 命令

**Files:**
- Create: `cmd/seed/main.go`
- Modify: `package.json` 增加 `seed` 脚本
- Modify: `README.md`

- [ ] **Step 1: 实现 seed**

流程：

1. 连接 PostgreSQL。
2. 查询默认租户和默认组织。
3. Upsert 权限码。
4. 创建/更新 `admin` 角色，`is_system=true`，绑定全部权限。
5. 创建/更新 `admin` 用户，`is_super_admin=true`，密码 `Admin@123456`（bcrypt）。
6. 绑定 `admin` 角色和 `HQ` 组织。
7. 输出默认账号信息。

- [ ] **Step 2: 命令**

```bash
go run ./cmd/seed
```

期望输出：

```text
seed completed: tenant=default username=admin password=Admin@123456
```

---

## Task 8: 前端认证与 SDK

**Files:**
- Modify: `packages/sdk/src/client.ts`
- Create: `packages/sdk/src/auth.ts`
- Create: `packages/sdk/src/system.ts`
- Modify: `packages/sdk/src/index.ts`
- Create: `apps/web/src/stores/auth.ts`
- Modify: `apps/web/src/stores/app.ts`
- Modify: `apps/web/src/main.ts`
- Modify: `apps/web/src/App.vue`
- Modify: `apps/web/src/router/index.ts`
- Modify: `apps/web/src/views/LoginView.vue`

- [ ] **Step 1: SDK token**

`client.ts`：

```ts
export function setAuthTokens(accessToken: string | null, refreshToken: string | null): void
export function clearAuthTokens(): void
export function setUnauthorizedHandler(handler: (() => void) | null): void
```

请求自动带 `Authorization`；401 时用 refresh token 刷新一次；刷新失败清理 token 并调用 unauthorized handler。

- [ ] **Step 2: auth API**

```ts
export interface LoginRequest { username: string; password: string; tenant_id?: string }
export interface LoginResponse { access_token: string; refresh_token: string; expires_in: number; user: CurrentUser; roles: string[]; permissions: string[] }
export interface CurrentUser { id: string; tenant_id: string; username: string; display_name: string; email: string; is_super_admin: boolean }
export const authApi = {
  login(input: LoginRequest): Promise<LoginResponse>,
  refresh(refreshToken: string): Promise<LoginResponse>,
  me(): Promise<MeResponse>,
  logout(refreshToken: string): Promise<void>,
  changePassword(input: { old_password: string; new_password: string }): Promise<void>
}
```

- [ ] **Step 3: system API**

```ts
export const systemApi = {
  users: { list, create, update, remove, setRoles },
  roles: { list, create, update, remove, setPermissions },
  permissions: { list },
  orgs: { list, create, update, remove },
  auditLogs: { list }
}
```

- [ ] **Step 4: auth store**

```ts
export const useAuthStore = defineStore('auth', () => {
  const user = ref<CurrentUser | null>(null)
  const permissions = ref<string[]>([])
  const roles = ref<string[]>([])
  const isAuthenticated = computed(() => Boolean(user.value))
  function hasPermission(code: string): boolean
  async function login(username: string, password: string): Promise<void>
  async function loadMe(): Promise<void>
  async function logout(): Promise<void>
  function clear(): void
})
```

- 登录后保存 token 到 localStorage 并调用 `setAuthTokens`。
- 启动时从 localStorage 恢复 token，调用 `loadMe`。
- 退出时调用后端 logout、清理 token、跳转登录。

- [ ] **Step 5: 路由与登录页**

- 路由 `meta.permission` 控制访问。
- 登录页调用真实 API，错误显示 `ElMessage.error`。
- 默认提示账号 `admin / Admin@123456`。
- 菜单按 `auth.hasPermission` 过滤；系统管理菜单需要 `system.*.read`。

- [ ] **Step 6: 测试**

```bash
pnpm typecheck:web
pnpm build:web
```

---

## Task 9: 前端系统管理页面

**Files:**
- Create: `apps/web/src/views/system/UserListView.vue`
- Create: `apps/web/src/views/system/RoleListView.vue`
- Create: `apps/web/src/views/system/OrgListView.vue`
- Create: `apps/web/src/views/system/AuditLogView.vue`
- Create: `apps/web/src/views/ForbiddenView.vue`
- Modify: `apps/web/src/router/index.ts`
- Modify: `apps/web/src/layouts/DefaultLayout.vue`

- [ ] **Step 1: 用户管理**

- 表格：用户名、显示名、邮箱、状态、超级管理员、创建时间。
- 搜索、分页。
- 新建/编辑弹窗：用户名、显示名、邮箱、密码（新建）、状态。
- 分配角色弹窗：多选角色。
- 权限：`system.user.read` / `system.user.write`。

- [ ] **Step 2: 角色管理**

- 表格：编码、名称、系统角色、权限数量。
- 新建/编辑弹窗。
- 权限分配：按模块分组的 checkbox 列表。
- 系统角色不可删除。
- 权限：`system.role.read` / `system.role.write`。

- [ ] **Step 3: 组织管理**

- 树形展示：集团/公司/工厂/部门。
- 新建子组织、编辑、删除。
- 有子节点不可删。
- 权限：`system.org.read` / `system.org.write`。

- [ ] **Step 4: 审计日志**

- 表格：时间、用户、动作、资源、请求 ID、IP。
- 分页。
- 权限：`system.audit.read`。

- [ ] **Step 5: 布局与路由**

- 平台菜单下增加：用户管理、角色管理、组织管理、审计日志。
- 路由 meta 配置权限。
- 无权限访问跳转 `/403`。

---

## Task 10: 测试、审核与报告

**Files:**
- Create: `docs/reports/2026-11-16-stage-1-platform-foundation.md`

- [ ] **Step 1: 静态检查**

```bash
gofmt -l ./cmd ./internal ./db
go vet ./...
go test ./...
pnpm typecheck:web
pnpm build:web
```

- [ ] **Step 2: 接口测试**

启动 PostgreSQL 和 API，执行：

```bash
# login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Admin@123456"}'

# me
curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/auth/me

# users
curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/system/users

# roles
curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/system/roles

# orgs
curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/system/orgs

# audit logs
curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/system/audit-logs

# purchase with permission
curl -H "Authorization: Bearer <access_token>" \
  http://localhost:8080/api/v1/purchase/orders
```

验证 401/403：

- 无 token 访问 `/auth/me` → 401。
- 无 `system.user.read` 的用户访问用户列表 → 403。
- refresh token 过期/撤销 → 401。

- [ ] **Step 3: 代码审核**

使用独立代码审核（subagent 或 `requesting-code-review` 技能）检查：

- 租户隔离是否完整。
- 权限检查是否遗漏。
- 密码/Token 是否安全存储。
- SQL 注入、越权、批量赋值风险。
- 错误处理与日志是否泄露敏感信息。
- 前后端 DTO 是否一致。

- [ ] **Step 4: 汇总报告**

报告包含：

- 完成范围。
- 新增/修改文件。
- 数据库变更。
- API 清单和测试结果。
- 代码审核发现与修复。
- 未完成/风险项。
- 下一阶段建议。

---

## Acceptance Criteria

- [ ] `admin` 可以登录并获取 access/refresh token。
- [ ] access token 可访问 `/auth/me`、系统管理和采购接口。
- [ ] 无 token 访问受保护接口返回 401。
- [ ] 无权限访问返回 403。
- [ ] 用户、角色、组织 CRUD 可用。
- [ ] 角色权限分配生效，重新登录或刷新后权限更新。
- [ ] 登录成功/失败写入 `login_logs`。
- [ ] 用户/角色/组织变更写入 `audit_logs`。
- [ ] 前端登录、会话恢复、401 刷新、退出登录可用。
- [ ] 菜单和路由按权限显示。
- [ ] 系统管理页面可完成基本 CRUD。
- [ ] `go test ./...`、`go vet ./...`、`pnpm typecheck:web`、`pnpm build:web` 通过。
- [ ] 代码审核完成并修复问题。
- [ ] 汇总报告完成，等待用户审阅。
