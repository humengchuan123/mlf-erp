# Stage 4B 汇总报告：审批流与职责分离

> 日期：2026-11-26
> 分支：`stage-4b-approval-workflow`
> 功能/测试 HEAD：`56e3a12`（报告与最终审核提交前）
> 设计：`docs/superpowers/specs/2026-11-26-stage-4b-approval-workflow-design.md`
> 计划：`docs/superpowers/plans/2026-11-26-stage-4b-approval-workflow.md`
> 状态：开发、19 个接口子测试、干净库 Stage 3B–4B 回归和静态检查完成；最终规格审核 **SPEC COMPLIANT**、质量审核 **APPROVED**

---

## 1. 完成范围

### 1.1 首期范围

Stage 4B 首期覆盖**销售订单和采购订单**：

- 按单据类型、组织、金额阈值（本位币 `base_total_amount`）和优先级匹配审批流。
- 条件：`always` / `amount_gte`；组织为空表示租户级，组织专属优先。
- 顺序步骤，每步一个审批人（角色或指定用户）；一步一次动作。
- 禁止自批：申请人不能审批自己的请求；用户步骤为申请人本人时自动跳过。
- 超级管理员可越过步骤审批人限制（角色或指定用户），但仍不能自批。
- 审批中的订单保持 `submitted`；最后一步通过后 `approved`；拒绝后回 `draft`；取消订单会取消 pending 请求。
- 无匹配流程时保持原有直审行为（提交后由有权限用户直接审批）。
- 审批请求/动作 append-only；待办、请求详情、订单审批历史、流程配置和启停可用。
- 有审批请求（含历史）的流程禁止编辑/删除；只允许停用（`POST /active`），需要调整时停用后新建。

### 1.2 非目标（4B-2）

- 贷项退款、收付款反核销、发货、收货、退货、期间重开的审批。
- 并行/会签/或签、委托、转办、升级、超时提醒。
- 复杂条件表达式、通知、流程版本历史。
- 手工凭证审批。

### 1.3 提交范围

| 内容 | 提交 |
|---|---|
| 迁移 27、权限、seed | `616cef6` |
| 设计/计划 | `2493a60`；历史保护收紧 `dc435d4` |
| 领域模型/app 服务 | `28d8af4` |
| 审批 HTTP API | `6b752dc` |
| PostgreSQL 仓储 | `cc8bbdd` |
| 订单集成 + `cmd/api` | `aa20e03` |
| SDK/OpenAPI 1.3.0 | `c7cdd6c` |
| Stage 4B 接口测试/脚本/README | `52bb998` |
| 前端审批流程/待办/历史 | `ffd10f0` |
| 审批授权范围、停用、actions、pending 修复 | `7818ea9` |
| 停用文档/SDK/UI/测试 | `944e764`、`95a273f`、`0e68273` |
| 模块文档 | `57497e9` |
| 规格审核修复：非最终响应、PENDING、日志、事务锁 | `099d62a` |
| SDK/OpenAPI/审批历史可见性 | `38014a2` |
| 契约文档同步 | `8e7f3ed` |
| 匹配/超级管理员/非审批人测试 | `bde81d1` |
| skip 动作持久化（迁移 28） | `6bb29de` |
| skip 动作 UI/OpenAPI | `513849b` |
| 非最终审批 current_step 守卫 | `f5fdf52` |
| 待办按钮权限 gating | `56e3a12` |
| skip/超级管理员/路线图文档 | `e0f50db` |
| skip 混合步骤测试 | `d4e6a83` |

## 2. 数据模型（迁移 27）

四张表：

- `erp.approval_flows`：`document_type`（`sales_order`/`purchase_order`）、`org_id`（NULL=租户级）、`condition_type`（`always`/`amount_gte`）、`threshold_amount`、`priority`、`active`、创建/更新人。
- `erp.approval_flow_steps`：`step_no`、`name`、`approver_type`（`role`/`user`）、`approver_id`；`UNIQUE (tenant_id, flow_id, step_no)`；流程删除级联。
- `erp.approval_requests`：单据类型/ID/编号、`flow_id`、`status`（`pending/approved/rejected/cancelled`）、`current_step`、申请人、金额/币种/本位币、完成时间；部分唯一索引保证同一单据最多一个 pending 请求。
- `erp.approval_actions`：`step_no`、`action`（`approve/reject/cancel/skip`）、`actor_id`、`comment`、时间；部分唯一索引防止同一步重复审批/拒绝；迁移 28 增加 `skip` 动作类型和 skip 唯一索引。
  - `skip` 表示申请人本人的用户步骤被自动跳过，不代表审批通过；请求创建和审批推进都会写入 skip 动作。

权限：`system.approval.read/write/act`；迁移和 seed 为 admin 授权。Down 在存在审批请求/动作时拒绝。

## 3. 匹配、审批人与状态机

### 3.1 匹配

- 单据类型 + 启用 + 组织（NULL 或单据组织）+ 条件。
- `amount_gte` 使用单据本位币总额；组织专属优先，再按 `priority DESC`、创建时间 DESC。
- 无匹配流程：不创建请求。

### 3.2 审批人

- 角色步骤：`user_roles` 中存在该角色即可审批。
- 用户步骤：指定用户。
- 申请人自批：服务层拒绝；创建/推进时跳过申请人的用户步骤，并为每个跳过步骤写入 `skip` 动作；全部跳过返回 `APPROVAL_NO_APPROVER`。
- 请求创建在事务内 `FOR SHARE` 锁定 flow 并重新加载步骤，避免流程编辑与请求创建的 stale-step 竞态。
- 超级管理员：可越过步骤审批人限制（角色或指定用户），但不能自批。
- 并发：请求行 `FOR UPDATE` + actions 部分唯一索引，同一步只成功一次。

### 3.3 状态机

```text
pending --approve(非最后一步)--> pending（current_step 前进，跳过申请人用户步骤）
pending --approve(最后一步)--> approved -> 订单 submitted -> approved
pending --reject--> rejected -> 订单 submitted -> draft
pending --cancel--> cancelled -> 订单 -> cancelled
```

- 请求终态不可再变更。
- 审批最终通过/拒绝后的订单转换使用 `ApproveByApproval`/`ReopenByApproval`：审批引擎已授权，转换只保留租户边界，不要求审批人拥有订单的数据范围；解决 self-scope 审批人无法完成订单的问题。
- 提交时匹配到流程但创建请求失败：非 `PENDING_APPROVAL_EXISTS` 回退 `submitted -> draft`；`PENDING_APPROVAL_EXISTS` 保持 submitted（已有 pending 请求）。

## 4. API、SDK 与前端

### 4.1 审批配置

| 方法 | 路径 | 权限 |
|---|---|---|
| GET/POST | `/api/v1/system/approval-flows` | `system.approval.read/write` |
| GET/PUT/DELETE | `/api/v1/system/approval-flows/:id` | `system.approval.read/write` |
| POST | `/api/v1/system/approval-flows/:id/active` | `system.approval.write` |

- 步骤必须从 1 连续；审批人必须存在；阈值非负。
- 有审批请求（含历史）时编辑/删除返回 409 `FLOW_IN_USE`；停用接口只更新 `active`，允许历史请求。

### 4.2 待办与历史

| 方法 | 路径 | 权限 |
|---|---|---|
| GET | `/api/v1/approvals/pending` | `system.approval.read` |
| GET | `/api/v1/approvals/requests` | `system.approval.read` |
| GET | `/api/v1/approvals/requests/:id` | `system.approval.read` |

- 待办按当前步骤审批人（用户/角色）匹配，排除申请人；超级管理员见全部待办。
- 列表返回 steps 和 actions（actions 用于订单详情审批历史）；详情返回完整 steps/actions。
- 数组始终为 `[]`，金额 4 位小数，时间 RFC3339。

### 4.3 订单动作

- `POST /sales/orders/:id/approve|reject`，权限 `sales.order.approve`。
- `POST /purchase/orders/:id/approve|reject`，权限 `purchase.order.approve`。
- `approve` body 可选 `{comment}`；`reject` body 必填 `{comment}`（空返回 400 `REASON_REQUIRED`）。
- 提交返回 `approval_pending`；非最后一步审批返回 `status:"pending"`、`current_step`、`approval_pending=true`；最后一步返回 false 并更新订单。
- 取消订单先取消 pending 请求。

### 4.4 SDK/OpenAPI/前端

- SDK `approvalApi`：flows CRUD、setFlowActive、pending、requests、request；销售/采购 `approve(id,{comment?})`、`reject(id,{comment})`。
- OpenAPI 1.3.0：审批路径/模型/错误码、订单 approve/reject。
- 前端新增：
  - `/system/approval-flows`：列表、条件/阈值/优先级、步骤编辑器（角色/用户）、行内启停。
  - `/system/pending-approvals`：待办、详情、批准/拒绝、意见必填（拒绝）。
  - 销售/采购订单详情“审批历史”：状态、当前步骤、步骤与动作历史；`skip` 动作显示“已跳过”。
- 路由/菜单归入平台组，权限 `system.approval.read`。

## 5. 测试结果

### 5.1 单元测试

- `go test ./internal/modules/approval/...`：domain/app/infra 包通过；app 覆盖率约 89.4%。
- 覆盖流程校验、匹配、自批跳过、无审批人、pending 创建、审批最后/非最后、拒绝、取消、历史保护和 SetActive。

### 5.2 接口测试

`TestStage4B`（专用库 `mlf_erp_stage4b_test`，每次 DROP/CREATE + migrate/seed/demo）共 **19 个顶层子测试**，连续两次运行通过：

1. legacy no-flow 销售/采购直审
2. 创建销售审批流并提交 pending
3. 申请人不能审批，admin 审批通过并记录历史
4. 两步顺序审批
5. 拒绝回 draft，原因必填
6. 取消订单取消 pending 请求
7. 低于阈值保持直审
8. 仅申请人步骤 -> 409 `APPROVAL_NO_APPROVER`，订单补偿回 draft
9. 角色审批人
10. self-scope 审批人完成审批（`ApproveByApproval`）
11. 并发审批只成功一次
12. 审批权限（配置/待办/请求/订单审批）
13. 有历史请求的流程 PUT/DELETE 409，`POST /active` 停用/恢复
14. 采购订单流程
15. 待办/请求详情契约（steps/actions/金额/租户）
16. 数据范围边界（demo_dept 待办 403）
17. 流程匹配优先级：组织专属（低优先级）仍胜出、停用后不再匹配、tenant-wide priority 3001 胜出
18. 超级管理员兜底 + 非审批人 `403 NOT_APPROVER`；非最终审批返回 `status=pending`/`current_step=2`
19. 混合自批步骤：创建时跳过申请人步骤并写 skip；推进时跳过中间申请人步骤并写 skip；current_step/动作数/actor 正确

### 5.3 干净库

数据库：`mlf_erp_stage4b_clean3`；迁移 28、seed、demo、独立 API（端口 18096），使用最终代码 `d4e6a83`。

```bash
go test -tags=integration ./tests/api -run 'TestStage3|TestStage4' -count=1 -v
```

结果：Stage 3B、3C、3D、3E、3F、3G、3H、3I、3J、3K、4A、4B 全部通过；Stage 4B 19 个子测试通过；总耗时约 43.9s。

数据核对：

| 指标 | 结果 |
|---|---|
| `schema_migrations` | 版本 28，`dirty = false` |
| 审批流程 | 17 |
| 审批请求 | 16（11 approved / 1 rejected / 4 cancelled） |
| 审批动作 | 21（含 2 条 `skip`） |
| 同一单据重复 pending | 0 |
| pending 请求 current_step 不存在于流程 | 0 |
| approved 请求与订单状态不一致 | 0（另有 2 条审批通过后订单被取消，属合法历史） |

### 5.4 静态检查

| 命令 | 结果 |
|---|---|
| `gofmt -l ./cmd ./internal ./db ./tests` | 无输出 |
| `go test ./... -count=1` | 全部包通过 |
| `go vet ./...` | 通过 |
| `go vet -tags=integration ./tests/api` | 通过 |
| `npx -y yaml@2 valid api/openapi.yaml` | 通过 |
| `pnpm typecheck:web` | 通过 |
| `pnpm test:frontend` | `Frontend completeness check passed.` |
| `pnpm build:web` | 通过（仅既有 chunk-size 警告） |
| `git diff --check` | 无空白错误 |

## 6. 代码审核

### 6.1 规格审核

首轮结论 **ISSUES**，已修复：

- 非最终审批响应补 `status:"pending"` + `current_step`。
- `PENDING_APPROVAL_EXISTS` 返回 409 并保持订单 submitted（已有 pending 请求）。
- 最终审批/拒绝后订单转换失败增加错误日志；提交补偿失败也记录。
- 流程 Update 在事务内 `FOR UPDATE` 锁 flow 并复查审批请求，关闭并发窗口。
- SDK/OpenAPI 修 `completed_at` 可空、`org_id` 租户级语义、审批响应字段。
- 无 `system.approval.read` 时订单详情隐藏审批历史。
- 补流程匹配优先级/组织/停用、超级管理员兜底、非审批人 `NOT_APPROVER`、非最终响应测试。

最终规格复审：**SPEC COMPLIANT**。此前 ISSUES 全部修复；`PENDING_APPROVAL_EXISTS` 作为防御性内部竞态契约保留，无正常 API 触发路径。

### 6.2 质量审核

复审 **CHANGES REQUIRED**：跳过的申请人步骤没有持久化，读取时重新加载未过滤步骤，导致 UI 误显示，且存在 stale-step 竞态。

修复：

- 迁移 28 增加 `skip` 动作和唯一索引。
- 请求创建在事务内锁 flow、重新加载步骤、计算 current_step，并写入 skip 动作。
- 审批推进写入后续自动跳过步骤的 skip 动作。
- 前端/SDK/OpenAPI 识别 `skip`，显示“已跳过”。
- 新增混合自批步骤测试；组织优先测试改为“组织专属低优先级仍胜出”。

修复后质量复审结论 **APPROVED**：skip 持久化阻塞已修复，无新增 Critical/Important。

剩余 Medium/Low（不阻塞）：

- 非最终审批后 `PendingRequest` 二次查询与审批事务分离，极端并发下响应可能 409/500，但动作已记录；后续可由审批事务直接返回下一步。
- 请求创建在 `FOR SHARE` 锁后重新加载步骤，但未复验 flow 的 active/条件/组织；并发停用/更新可能让请求落在不再匹配的 flow 上。
- 待办页批准/拒绝按钮已按 `.approve` 权限隐藏（`56e3a12`）。
- 订单审批历史只展示最近一条请求，拒绝后重新提交/取消的历史被隐藏。
- 审批路径/查询非法 UUID 可能返回 500；非法 document_type/status 过滤返回空。
- 迁移 28 无历史 skip 回填；升级库的旧请求可能缺少跳过历史。
- 测试 19 的 advance-time skip 断言不够独立（创建时已跳过的步骤也可能让断言通过）。
- flow org/审批人生命周期校验有限；角色删除/用户禁用可能留下不可审批流程。
- 请求列表 N+1；超级管理员待办未分页。
- OpenAPI update 的 active 默认值与 handler 保留旧值不一致；active 409 实际不可达。
- `approvals.Cancel` 后订单取消失败未日志/补偿。
- 组织级待办正向可见性未 API 覆盖（已在上文限制说明）。

## 7. 已知限制

- 首期只覆盖销售/采购订单；退款、反核销、发货、收货、退货、期间重开在 4B-2。
- 跨模块状态转换/审批动作不在同一数据库事务；提交/审批/拒绝有补偿和错误日志，极端情况需人工核对。
- 不支持并行/会签/委托/升级/通知/复杂条件。
- 流程无版本历史；有审批请求（含历史）时不能编辑/删除，只能停用后新建。
- 角色步骤可能无人可批，需要管理员调整角色或流程。
- 超级管理员可越过步骤审批人限制（角色或指定用户，不能自批）。
- 组织级待办正向可见性未用 API 正向覆盖（用户 API 不能设置 data_scope/org membership）；当前用 self-scope 审批人正向用例、demo_dept 缺少权限的 403 和租户断言覆盖，org/company 正向可见性未覆盖。
- 订单审批历史通过请求列表加载 actions；列表为分页接口，默认只显示最近请求。

## 8. 验收清单

- [x] 可配置销售/采购订单审批流（组织、金额阈值、优先级、启停）。
- [x] 顺序步骤，角色/用户审批人；禁止自批。
- [x] 提交匹配流程后创建 pending 请求；最后一步审批后订单 approved。
- [x] 拒绝后订单 draft，请求 rejected，历史完整。
- [x] 无流程保持原有直审。
- [x] 待办/详情/历史/权限/数据范围可用。
- [x] 并发审批只成功一次。
- [x] 有审批请求（含历史）时流程禁止编辑/删除；可停用。
- [x] Stage 3B–4A 回归和干净库 Stage 3B–4B 通过（`mlf_erp_stage4b_clean3`，迁移 28）。
- [x] SDK/OpenAPI/前端/文档完整。
- [x] 规格审核 **SPEC COMPLIANT**；质量审核 **APPROVED**，无 Critical/Important 遗留。
- [ ] 用户审阅通过。

## 9. 后续

- 4B-2：退款、反核销、发货、收货、退货、期间重开审批；并行/会签/委托/通知。
- 生产启用前：明确审批矩阵、金额阈值、角色成员和职责分离；对关键资金动作补充审批。
- 评估把跨模块状态转换纳入 Outbox/事务消息，减少补偿路径。
