# Stage 4B 审批流与职责分离 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为销售订单和采购订单增加可配置的顺序审批流、禁止自批、待办与审批历史，并保持无流程时的原有直审行为。

**Architecture:** 新增 `internal/modules/approval`（domain/app/infra/http）；迁移 27 建流程/步骤/请求/动作四表；订单 handler 在提交/审批/拒绝/取消时调用审批服务，订单服务新增 `Reopen`（submitted→draft）用于拒绝和补偿；审批动作 append-only。

**Tech Stack:** Go 1.25 + Gin + pgx/v5 + PostgreSQL + shopspring/decimal；Vue 3 + TypeScript + Element Plus。

**Design:** `docs/superpowers/specs/2026-11-26-stage-4b-approval-workflow-design.md`

---

## 0. 范围与关键决策

- 首期只覆盖 `sales_order`、`purchase_order`；退款/反核销/发货/收货/退货/期间重开为 4B-2。
- 无匹配流程：不创建请求，`approve` 保持 `submitted -> approved`。
- 有匹配流程：提交后订单仍为 `submitted`，创建 `pending` 请求；最后一步通过后 `submitted -> approved`；拒绝后 `submitted -> draft`（重新提交创建新请求）。
- 条件：`always` / `amount_gte`（本位币 `base_total_amount`）。
- 审批人：`role` / `user`；顺序审批；禁止自批；超级管理员可越过角色限制但不能自批。
- 流程有审批请求（含历史）时禁止编辑/删除（`FLOW_IN_USE`）；需要调整时停用后新建。
- 权限：`system.approval.read/write/act`；订单审批仍要求 `sales.order.approve` / `purchase.order.approve`。

## 1. 文件结构

**Create:**
- `db/migrations/000027_approval_workflow.up.sql`
- `db/migrations/000027_approval_workflow.down.sql`
- `internal/modules/approval/domain/flow.go`
- `internal/modules/approval/domain/request.go`
- `internal/modules/approval/domain/errors.go`
- `internal/modules/approval/domain/repositories.go`
- `internal/modules/approval/app/flow_service.go`
- `internal/modules/approval/app/request_service.go`
- `internal/modules/approval/app/flow_service_test.go`
- `internal/modules/approval/app/request_service_test.go`
- `internal/modules/approval/infra/postgres/flow_repository.go`
- `internal/modules/approval/infra/postgres/request_repository.go`
- `internal/modules/approval/http/handler.go`
- `packages/sdk/src/approval.ts`
- `apps/web/src/views/system/ApprovalFlowListView.vue`
- `apps/web/src/views/system/PendingApprovalView.vue`
- `tests/api/stage4b_test.go`
- `scripts/test/stage4b-api.ps1`
- `docs/reports/2026-11-26-stage-4b-approval-workflow.md`

**Modify:**
- `db/migrations/README.md`
- `cmd/seed/main.go`
- `cmd/api/main.go`
- `internal/modules/sales/app/order_service.go`
- `internal/modules/sales/infra/postgres/repository.go`
- `internal/modules/sales/http/handler.go`
- `internal/modules/purchase/app/order_service.go`
- `internal/modules/purchase/infra/postgres/order_repository.go`
- `internal/modules/purchase/http/handler.go`
- `packages/sdk/src/client.ts`（如需 POST body）
- `packages/sdk/src/sales.ts`
- `packages/sdk/src/purchase.ts`
- `packages/sdk/src/index.ts`
- `packages/sdk/src/system.ts`（如需用户列表）
- `apps/web/src/router/index.ts`
- `apps/web/src/layouts/DefaultLayout.vue`
- `apps/web/src/views/sales/OrderListView.vue`
- `apps/web/src/views/purchase/OrderListView.vue`
- `api/openapi.yaml`
- `scripts/test/check-frontend.mjs`
- `package.json`
- `README.md`
- `.github/workflows/ci.yml`
- `docs/superpowers/plans/2026-11-25-audit-follow-up.md`（Stage 4B 状态）

---

## 2. Task 1：迁移 27 与权限

**Files:**
- Create: `db/migrations/000027_approval_workflow.up.sql`
- Create: `db/migrations/000027_approval_workflow.down.sql`
- Modify: `db/migrations/README.md`
- Modify: `cmd/seed/main.go`

- [ ] **Step 1: up 迁移**

```sql
CREATE TABLE erp.approval_flows (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid,
    document_type text NOT NULL,
    name text NOT NULL,
    condition_type text NOT NULL DEFAULT 'always',
    threshold_amount numeric(19,4) NOT NULL DEFAULT 0,
    priority integer NOT NULL DEFAULT 0,
    active boolean NOT NULL DEFAULT true,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT approval_flows_document_type_check
        CHECK (document_type IN ('sales_order', 'purchase_order')),
    CONSTRAINT approval_flows_condition_type_check
        CHECK (condition_type IN ('always', 'amount_gte')),
    CONSTRAINT approval_flows_threshold_check CHECK (threshold_amount >= 0),
    CONSTRAINT approval_flows_tenant_id_key UNIQUE (tenant_id, id)
);
CREATE INDEX approval_flows_match_idx
    ON erp.approval_flows (tenant_id, document_type, active, priority DESC);
CREATE INDEX approval_flows_org_idx ON erp.approval_flows (tenant_id, org_id);

CREATE TABLE erp.approval_flow_steps (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    flow_id uuid NOT NULL,
    step_no integer NOT NULL CHECK (step_no > 0),
    name text NOT NULL,
    approver_type text NOT NULL,
    approver_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT approval_flow_steps_approver_type_check
        CHECK (approver_type IN ('role', 'user')),
    CONSTRAINT approval_flow_steps_flow_fk
        FOREIGN KEY (tenant_id, flow_id)
        REFERENCES erp.approval_flows (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT approval_flow_steps_unique UNIQUE (tenant_id, flow_id, step_no)
);

CREATE TABLE erp.approval_requests (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    org_id uuid NOT NULL,
    document_type text NOT NULL,
    document_id uuid NOT NULL,
    document_code text NOT NULL,
    flow_id uuid NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    current_step integer NOT NULL DEFAULT 1,
    requested_by uuid NOT NULL,
    requested_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    amount numeric(19,4) NOT NULL,
    currency text NOT NULL,
    base_amount numeric(19,4) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT approval_requests_document_type_check
        CHECK (document_type IN ('sales_order', 'purchase_order')),
    CONSTRAINT approval_requests_status_check
        CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),
    CONSTRAINT approval_requests_flow_fk
        FOREIGN KEY (tenant_id, flow_id)
        REFERENCES erp.approval_flows (tenant_id, id),
    CONSTRAINT approval_requests_tenant_id_key UNIQUE (tenant_id, id)
);
CREATE UNIQUE INDEX approval_requests_pending_document_idx
    ON erp.approval_requests (tenant_id, document_type, document_id)
    WHERE status = 'pending';
CREATE INDEX approval_requests_pending_idx
    ON erp.approval_requests (tenant_id, status, current_step);
CREATE INDEX approval_requests_document_idx
    ON erp.approval_requests (tenant_id, document_type, document_id, requested_at DESC);

CREATE TABLE erp.approval_actions (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    request_id uuid NOT NULL,
    step_no integer NOT NULL CHECK (step_no > 0),
    action text NOT NULL,
    actor_id uuid NOT NULL,
    comment text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT approval_actions_action_check
        CHECK (action IN ('approve', 'reject', 'cancel')),
    CONSTRAINT approval_actions_request_fk
        FOREIGN KEY (tenant_id, request_id)
        REFERENCES erp.approval_requests (tenant_id, id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX approval_actions_step_idx
    ON erp.approval_actions (tenant_id, request_id, step_no)
    WHERE action IN ('approve', 'reject');
CREATE INDEX approval_actions_request_idx
    ON erp.approval_actions (tenant_id, request_id, created_at);

INSERT INTO erp.permissions (code, name, module, type) VALUES
    ('system.approval.read', '查看审批', 'system', 'api'),
    ('system.approval.write', '管理审批流程', 'system', 'api'),
    ('system.approval.act', '审批动作', 'system', 'api')
ON CONFLICT (code) DO NOTHING;

INSERT INTO erp.role_permissions (tenant_id, role_id, permission_code)
SELECT r.tenant_id, r.id, p.code
FROM erp.roles r
JOIN erp.permissions p ON p.code IN ('system.approval.read','system.approval.write','system.approval.act')
WHERE r.code = 'admin'
ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: down 迁移**

```sql
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM erp.approval_requests) OR EXISTS (SELECT 1 FROM erp.approval_actions) THEN
        RAISE EXCEPTION 'cannot rollback approval workflow while approval data exists';
    END IF;
END $$;

DROP TABLE IF EXISTS erp.approval_actions;
DROP TABLE IF EXISTS erp.approval_requests;
DROP TABLE IF EXISTS erp.approval_flow_steps;
DROP TABLE IF EXISTS erp.approval_flows;

DELETE FROM erp.role_permissions WHERE permission_code IN ('system.approval.read','system.approval.write','system.approval.act');
DELETE FROM erp.permissions WHERE code IN ('system.approval.read','system.approval.write','system.approval.act');
```

- [ ] **Step 3: seed 权限**

`cmd/seed/main.go` 的 `permissions` 增加：

```go
{"system.approval.read", "查看审批", "system", "api"},
{"system.approval.write", "管理审批流程", "system", "api"},
{"system.approval.act", "审批动作", "system", "api"},
```

- [ ] **Step 4: README**

`db/migrations/README.md` 增加 `000027_approval_workflow` 一行：四表、索引、权限、down guard。

- [ ] **Step 5: 应用迁移并验证**

```bash
go run ./cmd/migrate -direction up
docker exec mlf-erp-dev-postgres-1 psql -U mlf -d mlf_erp -c "select version, dirty from schema_migrations;"
```

Expected: 27 / false。

- [ ] **Step 6: 提交**

```bash
git add db/migrations/000027_approval_workflow.up.sql db/migrations/000027_approval_workflow.down.sql db/migrations/README.md cmd/seed/main.go
git commit -m "feat(stage-4b): add approval workflow schema"
```

---

## 3. Task 2：审批领域与 app 服务

**Files:**
- Create: `internal/modules/approval/domain/flow.go`
- Create: `internal/modules/approval/domain/request.go`
- Create: `internal/modules/approval/domain/errors.go`
- Create: `internal/modules/approval/domain/repositories.go`
- Create: `internal/modules/approval/app/flow_service.go`
- Create: `internal/modules/approval/app/request_service.go`
- Create: `internal/modules/approval/app/flow_service_test.go`
- Create: `internal/modules/approval/app/request_service_test.go`

- [ ] **Step 1: 领域模型**

```go
// domain/flow.go
type DocumentType string
const (
    DocumentSalesOrder    DocumentType = "sales_order"
    DocumentPurchaseOrder DocumentType = "purchase_order"
)

type ConditionType string
const (
    ConditionAlways   ConditionType = "always"
    ConditionAmountGTE ConditionType = "amount_gte"
)

type ApproverType string
const (
    ApproverRole ApproverType = "role"
    ApproverUser ApproverType = "user"
)

type Flow struct {
    ID, TenantID, OrgID, Name string
    DocumentType DocumentType
    ConditionType ConditionType
    ThresholdAmount decimal.Decimal
    Priority int
    Active bool
    CreatedBy, UpdatedBy string
    CreatedAt, UpdatedAt time.Time
    Steps []FlowStep
}

type FlowStep struct {
    ID, TenantID, FlowID, Name string
    StepNo int
    ApproverType ApproverType
    ApproverID string
    CreatedAt time.Time
}
```

```go
// domain/request.go
type RequestStatus string
const (
    RequestPending   RequestStatus = "pending"
    RequestApproved  RequestStatus = "approved"
    RequestRejected  RequestStatus = "rejected"
    RequestCancelled RequestStatus = "cancelled"
)

type Request struct {
    ID, TenantID, OrgID string
    DocumentType DocumentType
    DocumentID, DocumentCode, FlowID string
    Status RequestStatus
    CurrentStep int
    RequestedBy string
    RequestedAt time.Time
    CompletedAt *time.Time
    Amount, BaseAmount decimal.Decimal
    Currency string
    CreatedAt, UpdatedAt time.Time
    Steps []FlowStep
    Actions []Action
}

type Action struct {
    ID, TenantID, RequestID string
    StepNo int
    Action string // approve/reject/cancel
    ActorID, Comment string
    CreatedAt time.Time
}

type Actor struct {
    UserID string
    IsSuperAdmin bool
}
```

- [ ] **Step 2: 错误**

```go
var (
    ErrNotFound = errors.New("approval not found")
    ErrInvalidFlow = errors.New("invalid approval flow")
    ErrFlowInUse = errors.New("approval flow has pending requests")
    ErrPendingExists = errors.New("pending approval already exists")
    ErrNoPending = errors.New("no pending approval")
    ErrNotApprover = errors.New("not the current approver")
    ErrSelfApproval = errors.New("cannot approve own request")
    ErrAlreadyActed = errors.New("approval step already acted")
    ErrNoApprover = errors.New("no eligible approver")
    ErrInvalidStatus = errors.New("invalid approval status")
)
```

- [ ] **Step 3: 仓储接口**

```go
type FlowRepository interface {
    List(ctx context.Context, tenantID, documentType string, active *bool) ([]*Flow, error)
    Get(ctx context.Context, tenantID, id string) (*Flow, error)
    Create(ctx context.Context, flow *Flow) error
    Update(ctx context.Context, flow *Flow) error
    Delete(ctx context.Context, tenantID, id string) error
    SetActive(ctx context.Context, tenantID, id string, active bool, updatedBy string) error
    FindMatching(ctx context.Context, tenantID, orgID string, docType DocumentType, baseAmount decimal.Decimal) (*Flow, error)
    HasRequests(ctx context.Context, tenantID, flowID string) (bool, error)
}

type RequestRepository interface {
    Create(ctx context.Context, request *Request) error
    Get(ctx context.Context, filter scope.Filter, tenantID, id string) (*Request, error)
    List(ctx context.Context, filter scope.Filter, tenantID string, docType DocumentType, docID, status string, page, pageSize int) ([]*Request, int, error)
    ListPending(ctx context.Context, filter scope.Filter, tenantID string, actor Actor) ([]*Request, error)
    GetPendingByDocument(ctx context.Context, tenantID string, docType DocumentType, docID string) (*Request, error)
    Approve(ctx context.Context, tenantID, requestID string, actor Actor, comment string) (bool, error)
    Reject(ctx context.Context, tenantID, requestID string, actor Actor, comment string) error
    CancelByDocument(ctx context.Context, tenantID string, docType DocumentType, docID, actorID, reason string) error
}
```

- [ ] **Step 4: app 服务**

`FlowService`:
- `List/Get/Create/Update/Delete`。
- 校验：document type、condition、threshold、steps 连续从 1、approver_type、approver_id 非空。
- `Update/Delete`：`HasRequests` true -> `ErrFlowInUse`（含历史请求，保护审批历史并避免 FK 删除失败）。
- 审批人存在性：注入 `RoleChecker`/`UserChecker` 接口（由 infra 实现，或 app 直接调用 system repositories？为避免耦合，定义接口）：
  ```go
  type ApproverValidator interface {
      RoleExists(ctx, tenantID, roleID string) (bool, error)
      UserExists(ctx, tenantID, userID string) (bool, error)
  }
  ```
- `RequestService`:
  - `Start(ctx, input StartInput) (bool, error)`：匹配 flow；无 flow -> false,nil；创建 request；跳过自批 user 步骤；无可用步骤 -> `ErrNoApprover`。
  - `Pending(ctx, docType, docID) (bool, error)`。
  - `Approve(ctx, docType, docID, actor, comment) (final bool, err error)`：找 pending request，调用 repo.Approve。
  - `Reject(...)`：找 pending，repo.Reject。
  - `Cancel(...)`：repo.CancelByDocument（无 pending 忽略）。
  - `List/Get`.
- `StartInput` 包含 tenant/org/doc/code/requestedBy/amount/currency/baseAmount。
- 单元测试使用 fake repositories/validator 覆盖匹配、跳过自批、无审批人、错误。

- [ ] **Step 5: 测试与提交**

```bash
gofmt -w internal/modules/approval
go test ./internal/modules/approval/... -count=1
git add internal/modules/approval
git commit -m "feat(stage-4b): add approval domain and services"
```

---

## 4. Task 3：PostgreSQL 仓储

**Files:**
- Create: `internal/modules/approval/infra/postgres/flow_repository.go`
- Create: `internal/modules/approval/infra/postgres/request_repository.go`

- [ ] **Step 1: FlowRepository**

- `List/Get` 查询 flow + steps（`ORDER BY step_no`）。
- `Create/Update` 事务：写 flow + steps；Update 先删旧 steps 再插入。
- `FindMatching` 按设计 §5 SQL，加载 steps。
- `HasRequests` 查询任意 `approval_requests flow_id=$2`（含历史）。
- `Delete` 事务：检查 pending；删除 flow（steps cascade）。

- [ ] **Step 2: RequestRepository**

- `Create`：插入 request；唯一冲突映射 `ErrPendingExists`。
- `Get`：request + steps + actions，应用 scope。
- `List`/`ListPending`：分页/筛选/scope；`ListPending` 按当前步骤的 user/role 匹配 actor 或 super admin，排除 requested_by。
- `GetPendingByDocument`。
- `Approve`：事务，`SELECT ... FOR UPDATE`；校验 pending、current step、自批、actor 角色/用户；插入 action（唯一冲突 -> `ErrAlreadyActed`）；推进步骤，跳过自批 user 步骤；最后一步 -> approved；提交。
- `Reject`：同理，插入 action，status rejected。
- `CancelByDocument`：事务，锁 pending；插入 cancel action；status cancelled；无 pending 忽略。
- `ListActions` 内部。
- 角色判断：`SELECT 1 FROM erp.user_roles WHERE tenant_id=$1 AND user_id=$2 AND role_id=$3`。
- 数据范围：复制 finance 的 `appendScope` 逻辑，对 `r.org_id` 应用。

- [ ] **Step 3: 测试**

- 仓储 SQL 通过 Stage 4B 接口测试覆盖；可加纯 helper 测试（步骤推进、自批跳过）。
- `go test ./internal/modules/approval/...`、`go build ./...`。

- [ ] **Step 4: 提交**

```bash
git add internal/modules/approval/infra/postgres
git commit -m "feat(stage-4b): add approval postgres repositories"
```

---

## 5. Task 4：审批 HTTP API 与装配

**Files:**
- Create: `internal/modules/approval/http/handler.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: 路由**

```go
group := group.Group("")
system := group.Group("/system")
system.GET("/approval-flows", authmw.RequirePermission("system.approval.read"), h.listFlows)
system.POST("/approval-flows", authmw.RequirePermission("system.approval.write"), h.createFlow)
system.GET("/approval-flows/:id", authmw.RequirePermission("system.approval.read"), h.getFlow)
system.PUT("/approval-flows/:id", authmw.RequirePermission("system.approval.write"), h.updateFlow)
system.DELETE("/approval-flows/:id", authmw.RequirePermission("system.approval.write"), h.deleteFlow)
system.POST("/approval-flows/:id/active", authmw.RequirePermission("system.approval.write"), h.setFlowActive)

approvals := group.Group("/approvals")
approvals.GET("/pending", authmw.RequirePermission("system.approval.read"), h.listPending)
approvals.GET("/requests", authmw.RequirePermission("system.approval.read"), h.listRequests)
approvals.GET("/requests/:id", authmw.RequirePermission("system.approval.read"), h.getRequest)
```

- [ ] **Step 2: DTO**

Flow 请求/响应；Request 响应含 steps/actions；金额 `StringFixed(4)`，时间 RFC3339。
错误映射：`ErrInvalidFlow` 400 `INVALID_APPROVAL_FLOW`；`ErrFlowInUse` 409 `FLOW_IN_USE`；`ErrNotFound` 404；`ErrNotApprover` 403 `NOT_APPROVER`；`ErrSelfApproval` 403 `SELF_APPROVAL`；`ErrAlreadyActed` 409；`ErrNoApprover` 409 `APPROVAL_NO_APPROVER`。

- [ ] **Step 3: 装配**

`cmd/api/main.go`：
```go
approvalFlowRepository := approvalpostgres.NewFlowRepository(db)
approvalRequestRepository := approvalpostgres.NewRequestRepository(db)
approvalFlowService := approvalapp.NewFlowService(approvalFlowRepository, approvalValidator)
approvalRequestService := approvalapp.NewRequestService(approvalFlowRepository, approvalRequestRepository)
approvalhttp.Register(protected, approvalFlowService, approvalRequestService, auditService, log)
```
`ApproverValidator` 用 `systempostgres` 的 role/user repository 或直接 SQL 实现。

- [ ] **Step 4: 测试/提交**

```bash
go build ./...
go test ./internal/modules/approval/... -count=1
git add internal/modules/approval/http cmd/api/main.go
git commit -m "feat(stage-4b): add approval APIs"
```

---

## 6. Task 5：销售/采购订单集成

**Files:**
- Modify: `internal/modules/sales/app/order_service.go`
- Modify: `internal/modules/sales/infra/postgres/repository.go`
- Modify: `internal/modules/sales/http/handler.go`
- Modify: `internal/modules/purchase/app/order_service.go`
- Modify: `internal/modules/purchase/infra/postgres/order_repository.go`
- Modify: `internal/modules/purchase/http/handler.go`

- [ ] **Step 1: `Reopen`**

- `OrderService.Reopen(ctx, id) error`：`submitted -> draft`。
- Repository 已有 `UpdateStatus`；直接调用。

- [ ] **Step 2: Handler 审批服务依赖**

`Register` 增加 `approvals *approvalapp.RequestService`；`Handler` 增加字段。
`cmd/api` 把 approvalRequestService 传给 sales/purchase Register（顺序：approval 服务先创建）。

- [ ] **Step 3: 提交**

```go
func (h *Handler) submitOrder(c *gin.Context) {
    if err := h.orders.Submit(ctx, id); err != nil { ... }
    order, err := h.orders.Get(ctx, id)
    if err != nil { _ = h.orders.Reopen(ctx, id); handle; return }
    started, err := h.approvals.Start(ctx, approvalapp.StartInput{...})
    if err != nil { _ = h.orders.Reopen(ctx, id); handle; return }
    h.recordAudit(...)
    httpx.OK(c, gin.H{"id": id, "action": "sales.order.submit", "approval_pending": started})
}
```

- [ ] **Step 4: 审批/拒绝/取消**

- approve：解析可选 comment；`pending` 判断；pending -> `Approve`；final -> `orders.Approve`；非 final 返回 `{status:"pending", current_step}`；无 pending -> `orders.Approve`。
- reject：comment 必填；`approvals.Reject`；`orders.Reopen`；返回。
- cancel：`approvals.Cancel`（忽略 `ErrNoPending`）-> `orders.Cancel`。
- 订单不存在/状态错误沿用现有错误映射。

- [ ] **Step 5: 测试/提交**

```bash
go build ./...
go test ./internal/modules/sales/... ./internal/modules/purchase/... -count=1
pnpm test:api:stage3b (legacy no-flow behavior)
git add internal/modules/sales internal/modules/purchase
git commit -m "feat(stage-4b): integrate approvals into orders"
```

---

## 7. Task 6：SDK/OpenAPI

**Files:**
- Create: `packages/sdk/src/approval.ts`
- Modify: `packages/sdk/src/index.ts`
- Modify: `packages/sdk/src/sales.ts`
- Modify: `packages/sdk/src/purchase.ts`
- Modify: `api/openapi.yaml`

- [ ] **Step 1: SDK**

`approvalApi`：flows list/get/create/update/remove/setFlowActive；pending；requests list/get。
类型：`ApprovalFlow`、`ApprovalFlowStep`、`ApprovalRequest`、`ApprovalAction`、`CreateApprovalFlowInput`。
Sales/purchase：`approve(id, input?: {comment?: string})`、`reject(id, input: {comment: string})`。
`api.post` 已支持 body；不需要改 client。

- [ ] **Step 2: OpenAPI**

- `/system/approval-flows` CRUD、`/system/approval-flows/{id}/active`、`/approvals/pending`、`/approvals/requests`、`/approvals/requests/{id}`。
- Sales/purchase approve 增加可选请求体、reject 新增路径。
- 错误码 400/403/404/409。
- 版本升至 `1.3.0`。

- [ ] **Step 3: 验证/提交**

```bash
npx -y yaml@2 valid api/openapi.yaml
pnpm typecheck:web
git add packages/sdk/src/approval.ts packages/sdk/src/index.ts packages/sdk/src/sales.ts packages/sdk/src/purchase.ts api/openapi.yaml
git commit -m "feat(stage-4b): add approval SDK and OpenAPI"
```

---

## 8. Task 7：前端

**Files:**
- Create: `apps/web/src/views/system/ApprovalFlowListView.vue`
- Create: `apps/web/src/views/system/PendingApprovalView.vue`
- Modify: `apps/web/src/router/index.ts`
- Modify: `apps/web/src/layouts/DefaultLayout.vue`
- Modify: `apps/web/src/views/sales/OrderListView.vue`
- Modify: `apps/web/src/views/purchase/OrderListView.vue`
- Modify: `scripts/test/check-frontend.mjs`

- [ ] **Step 1: 路由/菜单**

- `/system/approval-flows`：权限 `system.approval.read`，平台组。
- `/system/pending-approvals`：权限 `system.approval.read`，平台组。
- 菜单文案“审批流程”“待我审批”。

- [ ] **Step 2: 审批流程页**

- 列表：名称、单据类型、组织、条件、阈值、优先级、启停；行内启停开关调用 `setFlowActive`（有历史请求也可停用）。
- 新建/编辑弹窗：名称、单据类型、组织（可空=租户级）、条件、阈值、优先级、启停、步骤编辑器（step_no、名称、角色/用户、角色/用户选择）。
- 删除确认；`FLOW_IN_USE` 错误提示；有历史时引导用行内开关停用后新建。

- [ ] **Step 3: 待办页**

- 列表：单据类型/编号、金额、当前步骤、申请人、申请时间。
- 详情弹窗：步骤、状态、动作历史、审批意见。
- 批准/拒绝：调用对应 sales/purchase API；刷新。
- 无待办空态。

- [ ] **Step 4: 订单详情历史**

- 销售/采购订单详情抽屉增加“审批历史”区：调用 `approvalApi.requests({document_type, document_id})`，展示请求状态/当前步骤/步骤/动作。
- 无请求显示“未配置审批/无需审批”。

- [ ] **Step 5: 完整度断言**

`check-frontend.mjs` 增加：审批流程页、待办页、路由、权限、SDK 方法、OpenAPI 路径。

- [ ] **Step 6: 验证/提交**

```bash
pnpm typecheck:web
pnpm test:frontend
pnpm build:web
git add apps/web/src/views/system/ApprovalFlowListView.vue apps/web/src/views/system/PendingApprovalView.vue apps/web/src/router/index.ts apps/web/src/layouts/DefaultLayout.vue apps/web/src/views/sales/OrderListView.vue apps/web/src/views/purchase/OrderListView.vue scripts/test/check-frontend.mjs
git commit -m "feat(stage-4b): add approval UI"
```

---

## 9. Task 8：API 测试、脚本、README、干净库、报告

**Files:**
- Create: `tests/api/stage4b_test.go`
- Create: `scripts/test/stage4b-api.ps1`
- Modify: `package.json`
- Modify: `README.md`
- Modify: `.github/workflows/ci.yml`
- Create: `docs/reports/2026-11-26-stage-4b-approval-workflow.md`
- Modify: `internal/modules/sales/README.md`
- Modify: `internal/modules/purchase/README.md`
- Modify: `docs/superpowers/plans/2026-11-25-audit-follow-up.md`

- [ ] **Step 1: `TestStage4B`**

至少覆盖：
1. 创建销售订单金额审批流（阈值 100，步骤 user=admin）。
2. demo_admin 提交 100 订单 -> `approval_pending=true`，订单 submitted。
3. demo_admin 审批 -> 403 `SELF_APPROVAL`（如果 demo_admin 是当前步骤审批人；否则 `NOT_APPROVER`）。
4. admin 审批 -> 200，订单 approved，请求 approved，动作历史 1 条。
5. 两步流程：第一步 admin -> pending；第二步另一个用户/角色 -> approved。
6. 拒绝：admin 拒绝 -> 订单 draft，请求 rejected，动作历史。
7. 低于阈值/无流程：提交后 demo_admin 直接 approve -> approved（兼容）。
8. 采购订单流程同样。
9. 角色审批：创建审批角色+用户，按角色步骤审批。
10. 并发审批：同一请求两个并发 -> 一个 200，一个 409。
11. 取消：提交后取消 -> 请求 cancelled。
12. 权限：无 `system.approval.write` 不能配置；无 `.approve` 不能审批；无 `system.approval.read` 不能看待办。
13. 有审批请求（含历史）时编辑/删除流程 -> 409 `FLOW_IN_USE`；`POST /active` 停用成功；停用后新建可用。
14. 全部步骤为申请人 -> 提交 409 `APPROVAL_NO_APPROVER`，订单回 draft。
15. 数据范围/租户隔离（至少 tenant + org scope）。

- [ ] **Step 2: 脚本/包/README/CI**

`stage4b-api.ps1` 复制 Stage 4A 脚本，`TestStage4A` -> `TestStage4B`，端口/文件名改为 4b。
`package.json` 增加 `test:api:stage4b`。
README 增加 Stage 4B 状态/命令/说明。
CI 已用 `TestStage3|TestStage4`，无需改；确认包含。

- [ ] **Step 3: 干净库**

- 新建 `mlf_erp_stage4b_clean`，迁移 27、seed、demo、API。
- `go test -tags=integration ./tests/api -run 'TestStage3|TestStage4'`。
- 数据核对：pending 请求、actions 唯一、订单状态、无重复 pending。

- [ ] **Step 4: 文档/报告**

- `sales/README.md`、`purchase/README.md` 增加审批段。
- 报告：完成范围、数据模型、匹配/自批、订单集成、API/前端、测试、审核、已知限制、验收。
- 更新路线图 Stage 4B 状态。

- [ ] **Step 5: 静态检查/提交/推送**

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

提交并推送 `stage-4b-approval-workflow`。

---

## 10. 验收清单

- [ ] 可配置销售/采购订单审批流（组织、金额阈值、优先级、启停）。
- [ ] 顺序步骤，角色/用户审批人；禁止自批。
- [ ] 提交匹配流程后创建 pending 请求；最后一步审批后订单 approved。
- [ ] 拒绝后订单 draft，请求 rejected，历史完整。
- [ ] 无流程保持原有直审。
- [ ] 待办/详情/历史/权限/数据范围可用。
- [ ] 并发审批只成功一次。
- [ ] 有审批请求（含历史）时流程禁止编辑/删除；可停用。
- [ ] Stage 3B–4A 回归和干净库 Stage 3B–4B 通过。
- [ ] SDK/OpenAPI/前端/文档/报告完整。

## 11. 已知限制

- 首期只覆盖订单；其他单据 4B-2。
- 跨模块状态转换/审批动作非单事务，补偿 + 日志；极端情况需人工核对。
- 无并行/会签/委托/升级/通知/复杂条件。
- 流程无版本历史；有审批请求（含历史）时禁止编辑/删除，需停用后新建。
- 角色步骤可能无人可批，需管理员调整。
- 超级管理员可越过角色限制（不能自批）。
