# Stage 4B 审批流与职责分离设计

> 日期：2026-11-26
> 分支：`stage-4b-approval-workflow`
> 范围：首期（销售订单、采购订单）；退款/反核销/发货/收货/退货/期间重开审批留到 4B-2
> 关联：`docs/superpowers/plans/2026-11-25-audit-follow-up.md` Stage 4B

---

## 1. 目标

为销售订单和采购订单增加可配置审批流：

- 按单据类型、组织、金额阈值匹配审批流程。
- 顺序审批，每步一个审批人（角色或指定用户）。
- 禁止自批；申请人不能审批自己的请求。
- 审批动作 append-only，记录审批人、时间、意见、步骤。
- 无匹配流程时保持现有行为：提交后可直接由有权限的用户审批。
- 审批中的订单保持 `submitted` 状态；审批通过后进入 `approved`，拒绝后回到 `draft`。
- 流程配置、待办列表、单据审批历史在前端可用。

## 2. 非目标（首期不做）

- 贷项退款、收付款反核销、发货、收货、退货、期间重开的审批（4B-2）。
- 并行审批、会签、或签、委托、转办、升级、超时提醒。
- 复杂条件表达式（只支持 always / 金额 >= 阈值）。
- 邮件/短信/站内通知。
- 审批流版本历史（首期通过“有审批请求（含历史）的流程禁止编辑/删除”保证稳定；需要调整时停用后新建）。
- 手工凭证审批。

## 3. 角色与权限

新增权限：

| 权限 | 用途 |
|---|---|
| `system.approval.read` | 查看审批流程、待办、审批历史 |
| `system.approval.write` | 创建/编辑/删除审批流程 |
| `system.approval.act` | 预留：通用审批动作；首期订单审批仍走 `sales.order.approve` / `purchase.order.approve` |

- 流程配置页面要求 `system.approval.read` / `write`。
- 待办页面要求 `system.approval.read`。
- 销售订单审批仍要求 `sales.order.approve`；采购订单审批仍要求 `purchase.order.approve`。
- 审批服务额外校验当前步骤的角色/用户；超级管理员可越过步骤角色限制，但仍不能审批自己的申请。
- 迁移 27 和 `cmd/seed` 给 admin 角色授予三个新权限。

## 4. 数据模型（迁移 27）

### 4.1 `erp.approval_flows`

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid PK | |
| `tenant_id` | uuid NOT NULL | |
| `org_id` | uuid NULL | NULL = 租户级；否则只匹配该组织 |
| `document_type` | text NOT NULL | `sales_order` / `purchase_order` |
| `name` | text NOT NULL | 流程名称 |
| `condition_type` | text NOT NULL | `always` / `amount_gte` |
| `threshold_amount` | numeric(19,4) NOT NULL DEFAULT 0 | 本位币阈值（CNY） |
| `priority` | int NOT NULL DEFAULT 0 | 越大越优先 |
| `active` | boolean NOT NULL DEFAULT true | |
| `created_by` / `updated_by` | uuid NOT NULL | |
| `created_at` / `updated_at` | timestamptz | |

约束：`UNIQUE (tenant_id, id)`（供步骤/请求复合外键引用）。

索引：`(tenant_id, document_type, active, priority DESC)`、`(tenant_id, org_id)`。

有审批请求（含历史）时禁止编辑/删除流程字段/步骤；`active` 可通过独立停用接口切换，用于“停用旧流程后新建”。

### 4.2 `erp.approval_flow_steps`

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid PK | |
| `tenant_id` | uuid NOT NULL | |
| `flow_id` | uuid NOT NULL | |
| `step_no` | int NOT NULL | 从 1 连续递增 |
| `name` | text NOT NULL | 步骤名 |
| `approver_type` | text NOT NULL | `role` / `user` |
| `approver_id` | uuid NOT NULL | 角色 ID 或用户 ID |
| `created_at` | timestamptz | |

约束：`UNIQUE (tenant_id, flow_id, step_no)`；`FOREIGN KEY (tenant_id, flow_id) REFERENCES approval_flows (tenant_id, id) ON DELETE CASCADE`。

### 4.3 `erp.approval_requests`

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid PK | |
| `tenant_id` | uuid NOT NULL | |
| `org_id` | uuid NOT NULL | 单据组织 |
| `document_type` | text NOT NULL | `sales_order` / `purchase_order` |
| `document_id` | uuid NOT NULL | |
| `document_code` | text NOT NULL | 便于待办展示 |
| `flow_id` | uuid NOT NULL | |
| `status` | text NOT NULL | `pending` / `approved` / `rejected` / `cancelled` |
| `current_step` | int NOT NULL DEFAULT 1 | 当前步骤号 |
| `requested_by` | uuid NOT NULL | 申请人 |
| `requested_at` | timestamptz NOT NULL | |
| `completed_at` | timestamptz NULL | |
| `amount` | numeric(19,4) NOT NULL | 原币金额 |
| `currency` | text NOT NULL | |
| `base_amount` | numeric(19,4) NOT NULL | 本位币金额（条件匹配/展示） |
| `created_at` / `updated_at` | timestamptz | |

约束：

- `UNIQUE (tenant_id, id)`。
- 部分唯一索引 `(tenant_id, document_type, document_id) WHERE status = 'pending'`，保证同一单据最多一个待审批请求。
- 索引 `(tenant_id, status, current_step)`、`(tenant_id, document_type, document_id, requested_at DESC)`。

### 4.4 `erp.approval_actions`

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid PK | |
| `tenant_id` | uuid NOT NULL | |
| `request_id` | uuid NOT NULL | |
| `step_no` | int NOT NULL | 审批/拒绝为当前步骤；取消为当前步骤 |
| `action` | text NOT NULL | `approve` / `reject` / `cancel` |
| `actor_id` | uuid NOT NULL | |
| `comment` | text NOT NULL DEFAULT '' | |
| `created_at` | timestamptz | |

约束：

- `FOREIGN KEY (tenant_id, request_id) REFERENCES approval_requests (tenant_id, id) ON DELETE CASCADE`。
- 部分唯一索引 `(tenant_id, request_id, step_no) WHERE action IN ('approve','reject')`，防止同一步重复审批。
- 索引 `(tenant_id, request_id, created_at)`。

### 4.5 Down

- 存在 `approval_requests` 或 `approval_actions` 时拒绝 down（需先归档/清理）。
- 删除三个新表、权限和 admin 授权。

## 5. 流程匹配

输入：`tenant_id`、`org_id`、`document_type`、`base_amount`。

```sql
SELECT id
FROM erp.approval_flows
WHERE tenant_id = $1
  AND document_type = $2
  AND active = true
  AND (org_id IS NULL OR org_id = $3)
  AND (condition_type = 'always'
       OR (condition_type = 'amount_gte' AND threshold_amount <= $4))
ORDER BY (org_id IS NOT NULL) DESC, priority DESC, created_at DESC
LIMIT 1;
```

- 组织专属优先于租户级；再按 priority 降序、创建时间降序。
- 无匹配流程：不创建审批请求，保持现有直审行为。
- 条件金额使用单据本位币 `base_total_amount`（CNY），避免多币种阈值歧义。

## 6. 审批人解析与禁止自批

- `approver_type = user`：指定用户。
- `approver_type = role`：该角色下任意用户（通过 `erp.user_roles` 判断）；超级管理员可作为任意步骤的兜底审批人。
- 申请人不能审批自己的请求：
  - 创建请求时跳过 `user` 类型且 `approver_id = requested_by` 的步骤。
  - 角色步骤不预先跳过；申请人不能以该角色身份审批（服务层拒绝）。
  - 若所有步骤都被跳过（流程只包含申请人本人），返回 `APPROVAL_NO_APPROVER`，订单提交补偿回 `draft`。
- 审批通过后推进到下一步；若后续步骤是申请人本人的用户步骤，自动跳过；全部跳过则请求完成（`approved`）。

## 7. 请求状态机

```text
pending --approve(非最后一步)--> pending（current_step + 1）
pending --approve(最后一步)--> approved
pending --reject--> rejected
pending --cancel--> cancelled
```

- `approved` / `rejected` / `cancelled` 为终态。
- 审批动作 append-only。
- 请求与单据的联动：
  - 提交：`draft -> submitted`；匹配到流程则创建 `pending` 请求。
  - 审批：有 pending 请求时按步骤审批；最后一步通过后 `submitted -> approved`。
  - 拒绝：`pending -> rejected`，单据 `submitted -> draft`（申请人可修改后重新提交；重新提交创建新请求）。
  - 取消：单据取消时，pending 请求 `pending -> cancelled`，单据 `-> cancelled`。
  - 无 pending 请求时：审批保持原有 `submitted -> approved` 行为（兼容旧数据和未配置流程）。

## 8. 集成点

### 8.1 销售/采购订单

- `POST /sales/orders/:id/submit`：
  1. 现有 `Submit` 状态转换 `draft -> submitted`。
  2. 查询订单详情；调用审批服务 `Start`。
  3. 无匹配流程：正常返回。
  4. 匹配流程：创建 pending 请求；如创建失败，补偿 `submitted -> draft` 并返回错误。
- `POST /sales/orders/:id/approve`：
  - Body 可选 `{ "comment": "..." }`。
  - 有 pending 请求：调用审批服务 `Approve`；未到最后一步返回 `{status:"pending", current_step}`；最后一步通过后执行 `submitted -> approved`。
  - 无 pending 请求：保持原有 `submitted -> approved`。
- `POST /sales/orders/:id/reject`（新增）：
  - Body `{ "comment": "..." }`（必填或至少非空？首期允许空，但建议必填；设计为必填）。
  - 有 pending 请求：审批服务 `Reject`，然后 `submitted -> draft`。
  - 无 pending 请求：409 `NO_PENDING_APPROVAL`。
- `POST /sales/orders/:id/cancel`：先取消 pending 请求，再执行原有取消。
- 采购订单同样。
- 新增 `OrderService.Reopen`（`submitted -> draft`），用于拒绝/补偿；仅内部使用。
- 审批历史通过通用接口查询，不修改订单 DTO。

### 8.2 跨模块一致性

- 订单状态转换与审批请求创建/更新不在同一数据库事务（不同模块仓储）。
- 提交补偿：审批创建失败时回退 `submitted -> draft`。
- 审批最后一步与订单 `approved` 转换之间若失败，请求已 approved 但订单仍 submitted；记录错误日志，人工可重试/核对。首期接受该限制。
- 拒绝同理：请求 rejected 后订单回 draft 失败时记录错误。

## 9. API

### 9.1 审批流程配置

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/system/approval-flows` | `system.approval.read` | 列表；支持 `document_type`、`active` |
| POST | `/api/v1/system/approval-flows` | `system.approval.write` | 创建 |
| GET | `/api/v1/system/approval-flows/:id` | `system.approval.read` | 详情 |
| PUT | `/api/v1/system/approval-flows/:id` | `system.approval.write` | 更新；有审批请求（含历史）时 409 `FLOW_IN_USE`，建议停用后新建 |
| DELETE | `/api/v1/system/approval-flows/:id` | `system.approval.write` | 删除；有审批请求（含历史）时 409 `FLOW_IN_USE` |
| POST | `/api/v1/system/approval-flows/:id/active` | `system.approval.write` | 启用/停用；有历史请求也允许（只改 active） |

创建/更新请求：

```json
{
  "name": "销售订单金额审批",
  "document_type": "sales_order",
  "org_id": "",
  "condition_type": "amount_gte",
  "threshold_amount": "10000.0000",
  "priority": 10,
  "active": true,
  "steps": [
    {"step_no": 1, "name": "销售主管", "approver_type": "role", "approver_id": "role-uuid"},
    {"step_no": 2, "name": "财务经理", "approver_type": "user", "approver_id": "user-uuid"}
  ]
}
```

### 9.2 待办与历史

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/approvals/pending` | `system.approval.read` | 当前用户待审批请求 |
| GET | `/api/v1/approvals/requests` | `system.approval.read` | 按 `document_type`/`document_id`/`status` 查询 |
| GET | `/api/v1/approvals/requests/:id` | `system.approval.read` | 请求详情、步骤、动作历史 |

待办返回字段：请求 ID、单据类型/ID/编号、金额/币种/本位币、当前步骤、步骤名、申请人、申请时间。

### 9.3 订单审批/拒绝

| 方法 | 路径 | 权限 | Body |
|---|---|---|---|
| POST | `/api/v1/sales/orders/:id/approve` | `sales.order.approve` | `{comment?}` |
| POST | `/api/v1/sales/orders/:id/reject` | `sales.order.approve` | `{comment}` |
| POST | `/api/v1/purchase/orders/:id/approve` | `purchase.order.approve` | `{comment?}` |
| POST | `/api/v1/purchase/orders/:id/reject` | `purchase.order.approve` | `{comment}` |

## 10. 错误码

| HTTP | code | 场景 |
|---|---|---|
| 400 | `BAD_REQUEST` | 参数/流程配置非法 |
| 400 | `INVALID_APPROVAL_FLOW` | 步骤不连续、条件非法、审批人不存在 |
| 403 | `NOT_APPROVER` | 当前用户不是当前步骤审批人 |
| 403 | `SELF_APPROVAL` | 申请人试图审批自己的请求 |
| 404 | `NOT_FOUND` | 流程/请求不存在 |
| 409 | `FLOW_IN_USE` | 流程有审批请求（含历史），禁止编辑/删除 |
| 409 | `PENDING_APPROVAL_EXISTS` | 同一单据已有 pending 请求 |
| 409 | `NO_PENDING_APPROVAL` | 单据没有 pending 请求，不能拒绝 |
| 409 | `ALREADY_ACTED` | 当前步骤已审批/拒绝（并发） |
| 409 | `APPROVAL_NO_APPROVER` | 流程所有步骤均为申请人本人，无法审批 |
| 409 | `INVALID_STATUS` | 单据状态不允许提交/审批/拒绝 |

## 11. 并发

- `Approve` / `Reject` 对 `approval_requests` 行 `FOR UPDATE`。
- `approval_actions` 部分唯一索引 `(request_id, step_no) WHERE action IN ('approve','reject')` 兜底并发重复审批。
- 同一请求并发审批：一个成功，另一个 409 `ALREADY_ACTED` 或 `INVALID_STATUS`。
- 订单状态转换继续使用 `UpdateStatus` 的 `WHERE status = from` 守卫。
- 流程编辑/删除与审批请求：更新/删除前检查任意 `approval_requests`（含历史）；存在则拒绝并建议停用后新建，避免历史步骤/动作被改写或 FK 删除失败。并发创建请求时可能竞争；首期在事务内检查并接受极窄竞争窗口。

## 12. 前端

- 平台 > 审批流程：列表、条件、阈值、优先级、启停；创建/编辑弹窗含步骤编辑器（角色/用户选择）。列表行内开关调用停用接口；有历史请求的流程不能编辑字段/步骤，但可以停用。
- 平台 > 待我审批：待办列表；详情弹窗显示步骤/历史；批准/拒绝（意见必填拒绝）。
- 销售/采购订单详情抽屉：增加“审批历史”区，展示请求状态、当前步骤、步骤和动作历史。
- 订单列表的“审批”按钮保持；无权限或非当前审批人时后端返回 403，前端展示错误。
- 订单详情“拒绝”按钮仅在有 pending 请求且当前用户可能为审批人时显示；首期可在待办页统一拒绝，订单详情只展示历史。为减少误操作，首期订单详情不新增拒绝按钮，拒绝在待办页完成。

## 13. 测试

### 13.1 单元测试

- 流程匹配：组织优先、priority、金额阈值、always、无匹配。
- 审批人：角色/用户、超级管理员兜底、申请人自批拒绝、用户步骤自跳过、全部跳过报错。
- 状态机：多步顺序、最后一步 approved、reject、cancel、并发重复动作。
- 流程校验：步骤连续、审批人存在、条件/阈值、有审批请求的流程禁止编辑/删除。

### 13.2 接口测试（`TestStage4B`）

- 配置销售订单金额审批流（阈值 100），demo_admin 提交 100 订单 -> pending；demo_admin（申请人）审批 -> 403 `SELF_APPROVAL`；admin 审批 -> approved，订单 approved。
- 两步流程：admin 第一步 -> 仍 pending；第二步审批人 -> approved。
- 拒绝：admin 拒绝 -> 订单 draft，请求 rejected，历史记录。
- 低于阈值/无流程：提交后直接审批 -> approved（兼容旧行为）。
- 采购订单流程同样。
- 角色审批：创建审批角色/用户，按角色步骤审批。
- 并发审批：同一请求两个并发 -> 一个 200，一个 409。
- 取消：提交后取消订单 -> 请求 cancelled。
- 权限：无 `system.approval.write` 不能配置；无 `.approve` 不能审批；无 `system.approval.read` 不能看待办。
- 流程编辑/删除有审批请求（含历史）-> 409 `FLOW_IN_USE`；`POST /active` 可停用。
- 全部步骤为申请人 -> 提交返回 `APPROVAL_NO_APPROVER`，订单回 draft。
- 数据范围：dept/self 用户待办只看到自己组织/相关请求（首期至少 tenant 隔离；org scope 视实现）。

### 13.3 干净库

- 迁移 27；Stage 3B–4B 全链路；新增权限 seed；无重复 pending；审批动作唯一；订单状态一致。

## 14. 已知限制

- 首期只覆盖销售/采购订单；退款/反核销/发货/收货/退货/期间重开在 4B-2。
- 跨模块提交/审批/拒绝不是单事务；有补偿和错误日志，极端情况下需人工核对。
- 不支持并行/会签/委托/升级/通知/复杂条件。
- 流程编辑/删除在有审批请求（含历史）时禁止；可通过 `POST /active` 停用后新建。无版本历史。
- 角色步骤的“角色下无其他可用用户”无法在配置时完全校验，可能导致请求无人可批；需管理员调整角色或流程。
- 无审批 SLA/超时提醒。
- 超级管理员可越过步骤角色限制（不能自批）；这是运维兜底，不是日常审批路径。

## 15. 验收标准

- [ ] 可配置销售/采购订单审批流，支持组织、金额阈值、优先级、启停。
- [ ] 顺序步骤，角色/用户审批人，禁止自批。
- [ ] 提交匹配流程后创建 pending 请求；审批最后一步后订单 approved；拒绝后订单 draft。
- [ ] 无流程时保持原有直审行为。
- [ ] 待办、详情、历史、权限、数据范围可用。
- [ ] 并发审批只成功一次。
- [ ] 流程有审批请求（含历史）时禁止编辑/删除；可停用。
- [ ] Stage 3B–4A 回归和干净库 Stage 3B–4B 通过。
- [ ] SDK/OpenAPI/前端/文档/报告完整。
