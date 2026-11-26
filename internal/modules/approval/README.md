# approval 模块

Stage 4B 通用审批流引擎，首期接入销售订单和采购订单。

## 数据模型

迁移 27 创建四张表：

- `approval_flows`：流程定义（单据类型、组织、条件、阈值、优先级、启停）。
- `approval_flow_steps`：顺序步骤（`role` / `user` 审批人）。
- `approval_requests`：单据审批请求（`pending/approved/rejected/cancelled`、当前步骤、申请人、金额）。
- `approval_actions`：append-only 审批动作（`approve/reject/cancel/skip`、意见、审批人、时间）；`skip` 表示申请人本人的用户步骤被自动跳过，不代表通过。

同一单据最多一个 `pending` 请求（部分唯一索引）。有审批请求（含历史）的流程禁止编辑/删除，但可通过独立停用接口切换 `active`。

## 匹配

- 按 `document_type` + `active` + 组织（NULL 为租户级，或等于单据组织）+ 条件匹配。
- 条件：`always` 或 `amount_gte`（使用单据本位币 `base_total_amount`）。
- 排序：组织专属优先、`priority DESC`、创建时间 DESC。
- 无匹配流程：不创建请求，订单保持原有直审行为。

## 审批人

- `role`：该角色下任意用户；通过 `erp.user_roles` 判断。
- `user`：指定用户。
- 顺序审批，每步一次动作；`approval_actions` 部分唯一索引防并发重复。
- 禁止自批：申请人不能审批自己的请求；创建/推进时跳过申请人的 `user` 步骤；全部跳过返回 `APPROVAL_NO_APPROVER`。
- 超级管理员可越过步骤审批人限制（角色或指定用户），但仍不能自批。

## 状态与单据联动

```text
pending --approve(非最后一步)--> pending（current_step + 1）
pending --approve(最后一步)--> approved -> 订单 submitted -> approved
pending --reject--> rejected -> 订单 submitted -> draft
pending --cancel--> cancelled -> 订单 -> cancelled
```

- 审批中的订单保持 `submitted`。
- 拒绝后订单回 `draft`，重新提交创建新请求。
- 审批最终通过/拒绝后的订单转换使用审批授权范围（租户内），不要求审批人拥有订单的数据范围。

## API

| 方法 | 路径 | 权限 |
|---|---|---|
| GET/POST | `/api/v1/system/approval-flows` | `system.approval.read` / `write` |
| GET/PUT/DELETE | `/api/v1/system/approval-flows/:id` | `system.approval.read` / `write` |
| POST | `/api/v1/system/approval-flows/:id/active` | `system.approval.write` |
| GET | `/api/v1/approvals/pending` | `system.approval.read` |
| GET | `/api/v1/approvals/requests` | `system.approval.read` |
| GET | `/api/v1/approvals/requests/:id` | `system.approval.read` |

订单动作：

- `POST /sales/orders/:id/approve` / `reject`，权限 `sales.order.approve`。
- `POST /purchase/orders/:id/approve` / `reject`，权限 `purchase.order.approve`。
- 提交 `POST /orders/:id/submit` 返回 `approval_pending`；如已有 pending 请求，保持 submitted 并返回 409 `PENDING_APPROVAL_EXISTS`。
- 非最后一步审批返回 `{status:"pending", current_step, approval_pending:true}`；最后一步通过后订单 approved。
- 取消订单会取消 pending 请求。

## 错误码

- 400 `INVALID_APPROVAL_FLOW`
- 403 `NOT_APPROVER` / `SELF_APPROVAL`
- 404 `NOT_FOUND`
- 409 `FLOW_IN_USE`（含历史请求的流程编辑/删除）/ `PENDING_APPROVAL_EXISTS`（已有 pending，提交保持 submitted）/ `NO_PENDING_APPROVAL` / `ALREADY_ACTED` / `APPROVAL_NO_APPROVER` / `INVALID_STATUS`

## 已知限制

- 首期只覆盖销售/采购订单；退款、反核销、发货、收货、退货、期间重开在 4B-2。
- 跨模块状态转换/审批动作不是单事务；有补偿和错误日志，极端情况需人工核对。
- 不支持并行/会签/委托/升级/通知/复杂条件。
- 流程无版本历史；有请求时不能编辑/删除，只能停用后新建。
- 角色步骤可能无人可批，需要管理员调整角色或流程。
- 超级管理员可越过步骤审批人限制（角色或指定用户，不能自批）。
