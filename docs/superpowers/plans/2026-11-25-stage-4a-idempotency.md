# Stage 4A 资金操作幂等 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为会产生资金/库存副作用的 POST 接口增加 `Idempotency-Key` 重试安全：同一 key + 同一请求只执行一次，重复请求回放首次响应，冲突请求返回 409。

**Architecture:** 新增平台级幂等表、PostgreSQL 仓储和 Gin 中间件；中间件按路由白名单生效，鉴权之后、业务 handler 之前执行，缓冲响应并在完成后持久化；业务模块、服务层和总账事务不改。

**Tech Stack:** Go 1.25 + Gin + pgx/v5 + PostgreSQL + shopspring/decimal；Vue 3 + TypeScript + Element Plus；`crypto/sha256`、`crypto/rand`。

**Sources:**
- 路线图：`docs/superpowers/plans/2026-11-25-audit-follow-up.md` Stage 4A
- 评审：`docs/reports/2026-11-25-shuijing-retail-erp-audit-review.md`
- 现有财务错误码：`internal/modules/finance/http/handler.go`
- 现有测试客户端：`tests/api/stage3b_test.go`

---

## 0. 设计决策

### 0.1 保护范围（白名单）

中间件只在以下路由生效（`c.FullPath()`）：

```text
POST /api/v1/finance/credit-notes/:id/allocations
POST /api/v1/finance/credit-notes/:id/refunds
POST /api/v1/finance/payments
POST /api/v1/finance/payments/:id/allocate
POST /api/v1/finance/payments/:id/unallocate
POST /api/v1/sales/orders/:id/ship
POST /api/v1/purchase/orders/:id/receive
POST /api/v1/sales/shipments/:id/returns
POST /api/v1/purchase/receipts/:id/returns
```

### 0.2 Key 语义

- Header：`Idempotency-Key`。
- 格式：去空白后 8–128 字符，允许 `A-Z a-z 0-9 . _ : -`。
- 作用域：`(tenant_id, user_id, key, method, path)`；`path` 用实际请求路径（含资源 ID）。
- 请求哈希：`sha256(method + "\n" + path + "\n" + body)` 的十六进制。
- `Idempotency-Key` 缺失时：默认放行并记录 debug 日志；`IDEMPOTENCY_REQUIRED=true` 时返回 400 `IDEMPOTENCY_KEY_REQUIRED`。
- 默认 TTL 24h；过期 key 可重新使用。
- 同一 key 且首次请求仍在执行：无论 payload 是否相同，409 `IDEMPOTENCY_IN_PROGRESS`。
- 同一 key 不同 payload 且首次请求已完成：409 `IDEMPOTENCY_CONFLICT`。
- 已完成且相同 payload：回放首次响应，加响应头 `Idempotency-Replayed: true`。
- 5xx/401/403/408/429：标记 `failed`，允许用同一 key 重试（可带修正后的 payload）。

### 0.3 状态机

```text
insert -> in_progress
in_progress --2xx/3xx/4xx(非401/403/408/429)--> completed
in_progress --5xx/401/403/408/429--> failed
failed --any hash--> in_progress（允许修正后重试）
completed --same hash--> replay
completed --different hash--> 409 IDEMPOTENCY_CONFLICT
in_progress --any hash--> 409 IDEMPOTENCY_IN_PROGRESS
expired --any hash--> in_progress（重新占用）
```

### 0.4 错误码

| HTTP | code | 场景 |
|---|---|---|
| 400 | `IDEMPOTENCY_KEY_REQUIRED` | 强制模式下缺少 header |
| 400 | `IDEMPOTENCY_KEY_INVALID` | key 格式不合法 |
| 409 | `IDEMPOTENCY_CONFLICT` | 已完成/失败记录 + 同 key 不同请求 |
| 409 | `IDEMPOTENCY_IN_PROGRESS` | 同 key 请求仍在执行（含不同 payload） |
| 409 | `IDEMPOTENCY_RESPONSE_UNAVAILABLE` | 首次响应过大未保存，无法回放 |

---

## 1. 文件结构

**Create:**
- `db/migrations/000026_idempotency_keys.up.sql`
- `db/migrations/000026_idempotency_keys.down.sql`
- `internal/platform/idempotency/config.go`
- `internal/platform/idempotency/model.go`
- `internal/platform/idempotency/repository.go`
- `internal/platform/idempotency/postgres_repository.go`
- `internal/platform/idempotency/middleware.go`
- `internal/platform/idempotency/cleanup.go`
- `internal/platform/idempotency/middleware_test.go`
- `internal/platform/idempotency/model_test.go`
- `apps/web/src/utils/idempotency.ts`
- `tests/api/stage4a_test.go`
- `scripts/test/stage4a-api.ps1`
- `docs/reports/2026-11-25-stage-4a-idempotency.md`（执行后写）

**Modify:**
- `db/migrations/README.md`
- `internal/platform/config/config.go`
- `internal/platform/httpx/response.go`
- `cmd/api/main.go`
- `.env.example`
- `packages/sdk/src/client.ts`
- `packages/sdk/src/finance.ts`
- `packages/sdk/src/sales.ts`
- `packages/sdk/src/purchase.ts`
- `apps/web/src/views/finance/PaymentListView.vue`
- `apps/web/src/views/finance/CreditNoteListView.vue`
- `apps/web/src/views/sales/OrderListView.vue`
- `apps/web/src/views/purchase/OrderListView.vue`
- `apps/web/src/views/sales/ShipmentListView.vue`
- `apps/web/src/views/purchase/PurchaseReceiptListView.vue`
- `api/openapi.yaml`
- `scripts/test/check-frontend.mjs`
- `package.json`
- `README.md`
- `.github/workflows/ci.yml`
- `tests/api/stage3b_test.go`（只加 header 支持，不改现有断言）

---

## 2. Task 1：迁移与配置

**Files:**
- Create: `db/migrations/000026_idempotency_keys.up.sql`
- Create: `db/migrations/000026_idempotency_keys.down.sql`
- Modify: `db/migrations/README.md`
- Modify: `internal/platform/config/config.go`
- Modify: `.env.example`

- [ ] **Step 1: up 迁移**

```sql
CREATE TABLE IF NOT EXISTS erp.idempotency_keys (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    idempotency_key text NOT NULL,
    method text NOT NULL,
    path text NOT NULL,
    request_hash text NOT NULL,
    status text NOT NULL DEFAULT 'in_progress',
    response_code integer,
    response_body text,
    response_content_type text,
    response_truncated boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    CONSTRAINT idempotency_keys_status_check
        CHECK (status IN ('in_progress', 'completed', 'failed')),
    CONSTRAINT idempotency_keys_scope_key
        UNIQUE (tenant_id, user_id, idempotency_key, method, path)
);

CREATE INDEX IF NOT EXISTS idempotency_keys_expires_at_idx
    ON erp.idempotency_keys (expires_at);
```

- [ ] **Step 2: down 迁移**

```sql
DROP TABLE IF EXISTS erp.idempotency_keys;
```

- [ ] **Step 3: 更新 `db/migrations/README.md`**

在 `000025` 后加：

```text
- `000026_idempotency_keys`：资金/库存 POST 操作的幂等键、请求哈希、响应回放和过期清理；Down 直接删除该表。
```

- [ ] **Step 4: 配置**

在 `internal/platform/config/config.go` 增加：

```go
type IdempotencyConfig struct {
    TTL             time.Duration
    Required        bool
    MaxBodyBytes    int64
    MaxResponseBytes int64
    CleanupInterval time.Duration
}
```

`Config` 增加 `Idempotency IdempotencyConfig`。默认值：

```go
Idempotency: IdempotencyConfig{
    TTL:              getDuration("IDEMPOTENCY_TTL", 24*time.Hour),
    Required:         getBool("IDEMPOTENCY_REQUIRED", false),
    MaxBodyBytes:     int64(getInt("IDEMPOTENCY_MAX_BODY_BYTES", 1<<20)),
    MaxResponseBytes: int64(getInt("IDEMPOTENCY_MAX_RESPONSE_BYTES", 4<<20)),
    CleanupInterval:  getDuration("IDEMPOTENCY_CLEANUP_INTERVAL", time.Hour),
},
```

校验：TTL > 0、MaxBodyBytes > 0、MaxResponseBytes > 0、CleanupInterval > 0。

- [ ] **Step 5: `.env.example`**

```text
# Idempotency
IDEMPOTENCY_TTL=24h
IDEMPOTENCY_REQUIRED=false
IDEMPOTENCY_MAX_BODY_BYTES=1048576
IDEMPOTENCY_MAX_RESPONSE_BYTES=4194304
IDEMPOTENCY_CLEANUP_INTERVAL=1h
```

- [ ] **Step 6: 应用迁移**

```bash
go run ./cmd/migrate -direction up
docker exec mlf-erp-dev-postgres-1 psql -U mlf -d mlf_erp -c "select version, dirty from schema_migrations;"
```

Expected: version `26`, dirty `false`。

- [ ] **Step 7: 提交**

```bash
git add db/migrations/000026_idempotency_keys.up.sql db/migrations/000026_idempotency_keys.down.sql db/migrations/README.md internal/platform/config/config.go .env.example
git commit -m "feat(stage-4a): add idempotency schema and config"
```

---

## 3. Task 2：平台幂等组件与单元测试

**Files:**
- Create: `internal/platform/idempotency/config.go`
- Create: `internal/platform/idempotency/model.go`
- Create: `internal/platform/idempotency/repository.go`
- Create: `internal/platform/idempotency/postgres_repository.go`
- Create: `internal/platform/idempotency/middleware.go`
- Create: `internal/platform/idempotency/cleanup.go`
- Create: `internal/platform/idempotency/middleware_test.go`
- Create: `internal/platform/idempotency/model_test.go`
- Modify: `internal/platform/httpx/response.go`

- [ ] **Step 1: 模型与接口**

```go
// model.go
package idempotency

import "time"

type State int

const (
    StateAcquired State = iota
    StateReplay
    StateConflict
    StateInProgress
    StateResponseUnavailable
)

type Record struct {
    ID                  string
    RequestHash         string
    Status              string
    ResponseCode        int
    ResponseBody        []byte
    ResponseContentType string
    ResponseTruncated   bool
    ExpiresAt           time.Time
}

type AcquireInput struct {
    TenantID    string
    UserID      string
    Key         string
    Method      string
    Path        string
    RequestHash string
    Now         time.Time
    ExpiresAt   time.Time
}

type AcquireResult struct {
    State  State
    Record *Record
}
```

```go
// repository.go
type Repository interface {
    Acquire(ctx context.Context, in AcquireInput) (AcquireResult, error)
    Complete(ctx context.Context, id string, code int, body []byte, contentType string, truncated bool) error
    Fail(ctx context.Context, id string, code int, body []byte, contentType string) error
    DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}
```

- [ ] **Step 2: 错误码常量**

在 `internal/platform/httpx/response.go` 增加：

```go
CodeIdempotencyKeyRequired       = "IDEMPOTENCY_KEY_REQUIRED"
CodeIdempotencyKeyInvalid        = "IDEMPOTENCY_KEY_INVALID"
CodeIdempotencyConflict          = "IDEMPOTENCY_CONFLICT"
CodeIdempotencyInProgress        = "IDEMPOTENCY_IN_PROGRESS"
CodeIdempotencyResponseUnavailable = "IDEMPOTENCY_RESPONSE_UNAVAILABLE"
```

- [ ] **Step 3: 中间件核心**

```go
// middleware.go
type Config struct {
    TTL              time.Duration
    Required         bool
    MaxBodyBytes     int64
    MaxResponseBytes int64
    ProtectedPaths   map[string]struct{}
}

var DefaultProtectedPaths = map[string]struct{}{
    "/api/v1/finance/credit-notes/:id/allocations": {},
    "/api/v1/finance/credit-notes/:id/refunds":     {},
    "/api/v1/finance/payments":                     {},
    "/api/v1/finance/payments/:id/allocate":         {},
    "/api/v1/finance/payments/:id/unallocate":       {},
    "/api/v1/sales/orders/:id/ship":                 {},
    "/api/v1/purchase/orders/:id/receive":           {},
    "/api/v1/sales/shipments/:id/returns":           {},
    "/api/v1/purchase/receipts/:id/returns":         {},
}

func Middleware(repo Repository, log *slog.Logger, cfg Config) gin.HandlerFunc
```

中间件步骤：

1. 只处理 `POST`，且 `cfg.ProtectedPaths[c.FullPath()]` 存在；否则 `c.Next()`。
2. 读取并恢复 body（不超过 `MaxBodyBytes`）；计算 hash。
3. 读取 `Idempotency-Key`；空则按 `Required` 放行或 400。
4. 校验 key 格式；非法 400。
5. 从 `auth.ActorFrom` 取 `TenantID`/`UserID`；缺失 401。
6. `repo.Acquire`：
   - `StateReplay`：写回放头 + 状态码/内容类型/body，`c.Abort()`。
   - `StateConflict`：409 `IDEMPOTENCY_CONFLICT`。
   - `StateInProgress`：409 `IDEMPOTENCY_IN_PROGRESS`。
   - `StateResponseUnavailable`：409 `IDEMPOTENCY_RESPONSE_UNAVAILABLE`。
   - `StateAcquired`：继续。
7. 用 `captureWriter` 包装 `c.Writer`，`c.Next()`。
8. 读取捕获的 status/body/content-type：
   - `status >= 500 || status == 401 || status == 403 || status == 408 || status == 429` → `repo.Fail`。
   - 否则 `repo.Complete`；body 超过 `MaxResponseBytes` 时传 `nil` + `truncated=true`。
9. 恢复 `c.Writer`，写回捕获响应；204/304 不写 body。

`captureWriter` 必须实现 `gin.ResponseWriter`，覆盖 `WriteHeader`、`WriteHeaderNow`、`Write`、`WriteString`、`Status`、`Size`、`Written`、`Flush`（no-op），嵌入原始 `gin.ResponseWriter`。

- [ ] **Step 4: PostgreSQL 仓储**

`postgres_repository.go` 使用 `*pgxpool.Pool`。`Acquire` 在一个事务内：

1. `INSERT ... ON CONFLICT ... DO NOTHING RETURNING id`；插入成功 → `StateAcquired`。
2. 未插入 → `SELECT ... FOR UPDATE`。
3. 若 `expires_at <= now`：`UPDATE` 重置为 `in_progress`、新 hash/expiry → `StateAcquired`。
4. 若 `status = 'failed'`：无论 hash 是否相同，重置 → `StateAcquired`（失败尝试无成功副作用，允许修正后重试）。
5. 若 `status = 'completed'`：hash 不同 → `StateConflict`；hash 相同 → `StateReplay` / `StateResponseUnavailable`（`response_truncated`）。
6. 若 `status = 'in_progress'`：hash 相同 → `StateInProgress`；不同 → `StateConflict`。
7. 提交事务。

`Complete` / `Fail` 用 `UPDATE ... WHERE id = $1`；`DeleteExpired` 用 `DELETE ... WHERE expires_at < $1`。

- [ ] **Step 5: 清理循环**

```go
func RunCleanup(ctx context.Context, repo Repository, log *slog.Logger, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if n, err := repo.DeleteExpired(ctx, time.Now()); err != nil {
                log.Warn("idempotency cleanup failed", "error", err)
            } else if n > 0 {
                log.Info("idempotency cleanup", "deleted", n)
            }
        }
    }
}
```

- [ ] **Step 6: 单元测试**

`model_test.go`：
- key 校验：合法 UUID/带点/带冒号；长度 <8、>128、空格、中文、斜杠均拒绝。
- 请求哈希：同 method/path/body 稳定；body 变化 hash 变化。

`middleware_test.go`（`httptest` + `gin.New()` + fake repo）：
- 首次请求执行 handler，`Complete` 被调用；第二次同 key 同 body 回放，handler 不执行。
- 同 key 不同 body → 409 `IDEMPOTENCY_CONFLICT`。
- `StateInProgress` → 409 `IDEMPOTENCY_IN_PROGRESS`。
- `StateResponseUnavailable` → 409 `IDEMPOTENCY_RESPONSE_UNAVAILABLE`。
- 缺少 key 默认放行；`Required=true` → 400。
- 非白名单路由 / GET 放行。
- handler 返回 500 时 `Fail` 被调用，第二次可重试。
- `captureWriter` 捕获状态码/body/Content-Type。

Run:

```bash
gofmt -w internal/platform/idempotency internal/platform/config internal/platform/httpx
go test ./internal/platform/idempotency/... -count=1
go vet ./internal/platform/idempotency/...
```

- [ ] **Step 7: 提交**

```bash
git add internal/platform/idempotency internal/platform/httpx/response.go
git commit -m "feat(stage-4a): add idempotency middleware and repository"
```

---

## 4. Task 3：装配与路由白名单

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: 装配**

在 `cmd/api/main.go` 创建：

```go
idempotencyRepository := idempotencypostgres.NewRepository(db)
idempotencyMiddleware := idempotency.Middleware(idempotencyRepository, log, idempotency.Config{
    TTL:              cfg.Idempotency.TTL,
    Required:         cfg.Idempotency.Required,
    MaxBodyBytes:     cfg.Idempotency.MaxBodyBytes,
    MaxResponseBytes: cfg.Idempotency.MaxResponseBytes,
    ProtectedPaths:   idempotency.DefaultProtectedPaths,
})
```

在 `protected := api.Group("", authmw.Auth(...))` 之后、所有 handler 注册之前：

```go
protected.Use(idempotencyMiddleware)
```

清理循环：

```go
cleanupCtx, cancelCleanup := context.WithCancel(context.Background())
defer cancelCleanup()
go idempotency.RunCleanup(cleanupCtx, idempotencyRepository, log, cfg.Idempotency.CleanupInterval)
```

- [ ] **Step 2: 验证**

```bash
gofmt -w cmd/api/main.go
go build ./...
go test ./... -count=1
go vet ./...
```

- [ ] **Step 3: 提交**

```bash
git add cmd/api/main.go
git commit -m "feat(stage-4a): wire idempotency middleware"
```

---

## 5. Task 4：SDK、OpenAPI、前端

**Files:**
- Modify: `packages/sdk/src/client.ts`
- Modify: `packages/sdk/src/finance.ts`
- Modify: `packages/sdk/src/sales.ts`
- Modify: `packages/sdk/src/purchase.ts`
- Modify: `api/openapi.yaml`
- Modify: `scripts/test/check-frontend.mjs`

- [ ] **Step 1: SDK client**

```ts
export interface RequestOptions {
  idempotencyKey?: string
}

post<T>(path: string, data?: unknown, options?: RequestOptions): Promise<T> {
  const headers: Record<string, string> = {}
  if (options?.idempotencyKey) {
    headers['Idempotency-Key'] = options.idempotencyKey
  }
  return this.request<T>(path, {
    method: 'POST',
    headers,
    body: data === undefined ? undefined : JSON.stringify(data)
  })
}
```

- [ ] **Step 2: finance 方法**

```ts
createPayment(input: PaymentInput, idempotencyKey?: string): Promise<Payment> {
  return api.post<Payment>('/finance/payments', input, { idempotencyKey })
},
allocate(paymentId: string, allocations: AllocationInput[], idempotencyKey?: string): Promise<Payment> {
  return api.post<Payment>(`/finance/payments/${paymentId}/allocate`, { allocations }, { idempotencyKey })
},
```

`payments.unallocate`、`creditNotes.allocate`、`creditNotes.refund` 同样增加可选 `idempotencyKey`。

- [ ] **Step 3: sales/purchase 方法**

`ship`、`receive`、`returns.create` 增加可选 `idempotencyKey`，传给 `api.post`。

- [ ] **Step 4: OpenAPI**

新增 component：

```yaml
IdempotencyKey:
  name: Idempotency-Key
  in: header
  required: false
  description: 客户端生成的幂等键；同一 key + 同一请求只执行一次。
  schema:
    type: string
    minLength: 8
    maxLength: 128
```

给 9 个受保护 POST 操作加：

```yaml
parameters:
  - $ref: '#/components/parameters/IdempotencyKey'
```

并补充 409 `IDEMPOTENCY_CONFLICT` / `IDEMPOTENCY_IN_PROGRESS` 响应说明。

- [ ] **Step 5: 前端**

在 6 个视图的提交路径增加 key：

- `PaymentListView.vue`：`createIdempotencyKey`、`allocateIdempotencyKey`、`reverseIdempotencyKey`；打开弹窗时 `newIdempotencyKey()`，提交时传给 SDK，成功后清空。
- `CreditNoteListView.vue`：`applyIdempotencyKey`、`refundIdempotencyKey`。
- `sales/OrderListView.vue`：`shipIdempotencyKey`。
- `purchase/OrderListView.vue`：`receiveIdempotencyKey`。
- `sales/ShipmentListView.vue`：`returnIdempotencyKey`。
- `purchase/PurchaseReceiptListView.vue`：`returnIdempotencyKey`。

模式：

```ts
import { newIdempotencyKey, shouldRetainIdempotencyKey } from '../../utils/idempotency'

const createIdempotencyKey = ref('')

function openCreate() {
  createIdempotencyKey.value = newIdempotencyKey()
  // ...
}

try {
  await financeApi.createPayment({...}, createIdempotencyKey.value)
  createIdempotencyKey.value = ''
} catch (error) {
  if (!shouldRetainIdempotencyKey(error)) {
    createIdempotencyKey.value = newIdempotencyKey()
  }
  // show error
}
```

`newIdempotencyKey()` 优先 `crypto.randomUUID()`，非 HTTPS 私有化部署回退 `crypto.getRandomValues()`，最后回退 `Math.random`。网络/5xx/401/403/408/429 及幂等冲突/进行中/不可回放保留 key；其余 4xx 旋转新 key。关闭弹窗时重新生成。

- [ ] **Step 6: check-frontend**

增加断言：

```js
'Idempotency-Key'
'idempotencyKey'
'createIdempotencyKey'
'reverseIdempotencyKey'
'newIdempotencyKey'
'shouldRetainIdempotencyKey'
'getRandomValues'
'IDEMPOTENCY_CONFLICT'
```

- [ ] **Step 7: 验证**

```bash
npx -y yaml@2 valid api/openapi.yaml
pnpm typecheck:web
pnpm test:frontend
pnpm build:web
```

- [ ] **Step 8: 提交**

```bash
git add packages/sdk/src/client.ts packages/sdk/src/finance.ts packages/sdk/src/sales.ts packages/sdk/src/purchase.ts api/openapi.yaml scripts/test/check-frontend.mjs apps/web/src/views/finance/PaymentListView.vue apps/web/src/views/finance/CreditNoteListView.vue apps/web/src/views/sales/OrderListView.vue apps/web/src/views/purchase/OrderListView.vue apps/web/src/views/sales/ShipmentListView.vue apps/web/src/views/purchase/PurchaseReceiptListView.vue
git commit -m "feat(stage-4a): add idempotency keys to SDK and UI"
```

---

## 6. Task 5：API 集成测试、脚本、README、CI

**Files:**
- Modify: `tests/api/stage3b_test.go`（只加 header 支持）
- Create: `tests/api/stage4a_test.go`
- Create: `scripts/test/stage4a-api.ps1`
- Modify: `package.json`
- Modify: `README.md`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: 测试客户端支持 header**

把 `client.request` 改为可变 header：

```go
func (c *client) request(method, path string, body any, token string, headerSets ...map[string]string) (int, apiPayload) {
    // ...
    for _, headers := range headerSets {
        for key, value := range headers {
            req.Header.Set(key, value)
        }
    }
    // ...
}

func (c *client) postWithHeaders(path string, body any, headers map[string]string) (int, apiPayload) {
    return c.request(http.MethodPost, path, body, c.token, headers)
}
```

现有调用不受影响。

- [ ] **Step 2: `tests/api/stage4a_test.go`**

`//go:build integration`，`func TestStage4A(t *testing.T)`。复用 `newClient`、`createShippedOrder`、`stage3JCreateOpenCreditNote`、`stage3JRefund`、`stage3JRawPost`。

子测试：

1. `payment create replay`
   - 唯一 reference；`postWithHeaders("/finance/payments", body, key)` → 201。
   - 同 key 同 body → 201，同 payment ID。
   - 同 key 不同 amount → 409 `IDEMPOTENCY_CONFLICT`。
   - 按 reference 查列表，只有 1 条。
2. `allocate replay`
   - 创建应收发票/付款；allocate with key → 200；重复 → 同响应；详情 `allocations` 只有 1 条。
3. `unallocate replay`
   - 先 allocate；unallocate with key → 200；重复 → 同响应；`reversals` 只有 1 条。
4. `refund replay`
   - `stage3JCreateOpenCreditNote`；refund with key → 200；重复 → 同 refund ID；credit note `refunded_amount` 只加一次。
   - 同 key 不同 amount → 409 `IDEMPOTENCY_CONFLICT`。
5. `concurrent refund same key`
   - 两个 goroutine 同 key 同 body；结果一个 200、一个 409 `IDEMPOTENCY_IN_PROGRESS`/`IDEMPOTENCY_CONFLICT`（或 200 回放）；最终只有 1 条 refund。
6. `sales ship replay`
   - 创建/提交/审批销售订单；ship with key → 201；重复 → 同 shipment ID；按 order_id 查 shipment 列表只有 1 条。
7. `purchase receive replay`
   - 创建/提交/审批采购订单；receive with key → 201；重复 → 同 receipt ID；按 order_id 查 receipt 列表只有 1 条。
8. `missing key still works`
   - 无 header 创建付款 → 201（默认非强制）。

并发 helper：

```go
func stage4AConcurrentPost(c *client, path string, body any, key string) []stage4AResult
```

- [ ] **Step 3: 脚本**

复制 `stage3k-api.ps1` 为 `stage4a-api.ps1`，把 `api-stage3k` 改为 `api-stage4a`，`TestStage3K` 改为 `TestStage4A`。

- [ ] **Step 4: package/README/CI**

`package.json`：

```json
"test:api:stage4a": "powershell -ExecutionPolicy Bypass -File ./scripts/test/stage4a-api.ps1"
```

README：
- 状态行加 Stage 4A。
- 命令列表加 `pnpm test:api:stage4a`。
- 新增 `### Stage 4A 资金操作幂等` 段，列出 9 个受保护端点、错误码、TTL、可选/强制模式、测试命令。

CI：

```yaml
- run: TEST_API_BASE_URL=http://localhost:18080 go test -tags=integration ./tests/api -run 'TestStage3|TestStage4' -count=1 -v
```

- [ ] **Step 5: 验证**

```bash
gofmt -w tests/api/stage4a_test.go tests/api/stage3b_test.go
go vet -tags=integration ./tests/api
go test -tags=integration ./tests/api -run '^$'
pnpm test:api:stage4a
```

Expected: `TestStage4A` 全部通过。

- [ ] **Step 6: 提交**

```bash
git add tests/api/stage4a_test.go tests/api/stage3b_test.go scripts/test/stage4a-api.ps1 package.json README.md .github/workflows/ci.yml
git commit -m "test(stage-4a): add idempotency API tests"
```

---

## 7. Task 6：干净库回归、文档、报告

**Files:**
- Create: `docs/reports/2026-11-25-stage-4a-idempotency.md`
- Modify: `internal/modules/finance/README.md`
- Modify: `internal/modules/purchase/README.md`
- Modify: `internal/modules/sales/README.md`
- Modify: `docs/superpowers/plans/2026-11-25-audit-follow-up.md`（标记 Stage 4A 完成）

- [ ] **Step 1: 干净库**

```bash
# 新建 mlf_erp_stage4a_clean，migrate/seed/demo，启动 API，执行：
go test -tags=integration ./tests/api -run 'TestStage3|TestStage4' -count=1 -v
```

断言：
- `schema_migrations` 版本 26。
- `idempotency_keys` 有记录；无重复副作用。
- Stage 3B–3K 全部通过。
- Stage 4A 全部通过。

- [ ] **Step 2: 文档**

`finance/README.md`：新增“资金操作幂等（Stage 4A）”，列出受保护端点、header、错误码、TTL、前端 key 生成和重试约定。
`sales/README.md`、`purchase/README.md`：在发货/收货/退货段说明 `Idempotency-Key`。

- [ ] **Step 3: 报告**

报告结构：
1. 完成范围
2. 数据模型（迁移 26）
3. 幂等状态机与错误码
4. 受保护端点与前端行为
5. 测试结果（单元/接口/干净库）
6. 代码审核结论
7. 已知限制（可选 header、TTL、响应大小、清理任务）
8. 验收清单

- [ ] **Step 4: 静态检查**

```bash
gofmt -l ./cmd ./internal ./db ./tests
go test ./... -count=1
go vet ./...
go vet -tags=integration ./tests/api
npx -y yaml@2 valid api/openapi.yaml
pnpm typecheck:web
pnpm test:frontend
pnpm build:web
git diff --check
```

- [ ] **Step 5: 提交**

```bash
git add docs/reports/2026-11-25-stage-4a-idempotency.md internal/modules/finance/README.md internal/modules/purchase/README.md internal/modules/sales/README.md docs/superpowers/plans/2026-11-25-audit-follow-up.md
git commit -m "docs(stage-4a): add idempotency docs and report"
```

---

## 8. 验收清单

- [ ] 同一 tenant + user + key + method + path + 相同 payload：重复请求回放首次响应，只产生一次副作用。
- [ ] 已完成记录 + 同一 key 不同 payload：409 `IDEMPOTENCY_CONFLICT`；`in_progress` 时任何 payload 均 409 `IDEMPOTENCY_IN_PROGRESS`；`failed` 时任意 payload 重置并允许重试。
- [ ] 并发同 key 同 payload：最多一个执行，其余回放或 409；无双退款/双核销/双反核销/双发货/双收货。
- [ ] 不同 tenant/user 可使用相同 key，互不影响。
- [ ] 过期 key 可重新使用；清理任务不删除未过期记录。
- [ ] 5xx/401/403/408/429 可用同一 key 重试。
- [ ] 前端 6 个视图、9 个提交动作生成并传递 key；重复点击被阻止。
- [ ] 9 个受保护端点全部有 OpenAPI header 和 409 说明。
- [ ] `TestStage4A` 通过；Stage 3B–3K 回归通过。
- [ ] 干净库版本 26，全链路通过。
- [ ] 文档/报告/已知限制完整。

---

## 9. 已知限制（首期）

- Header 默认可选，未传 key 的旧客户端仍可能重复提交；生产强制前需先完成客户端接入。
- 幂等记录存响应体明文，TTL 24h；敏感数据依赖数据库访问控制，后续可加密/脱敏。
- 响应超过 4 MB 时无法回放，返回 409 `IDEMPOTENCY_RESPONSE_UNAVAILABLE`。
- 清理任务在 API 进程内运行；后续可移到 Worker。
- 不覆盖消息队列/Outbox 重试；不覆盖 GET/查询。
- 不实现客户端自动重试 UI；只保证同一 key 重试安全。
