# 审计后续开发路线图 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把《水镜零售 ERP 系统全面审计与后续开发建议》中的有效问题映射到 `mlf-erp` 的真实差距，形成一份按依赖排序、可验收的阶段路线图，并明确下一阶段从资金操作幂等开始。

**Architecture:** 沿用现有模块化单体：`http → app → domain`、`infra → domain`；新增平台级幂等与审批能力，业务模块接入；不改动已完成的总账、汇兑、期末结转和反核销口径。

**Tech Stack:** Go 1.25 + Gin + pgx + PostgreSQL + shopspring/decimal；Vue 3 + TypeScript + Element Plus。

**Sources:**
- 审计原文：`docs/reports/2026-11-25-shuijing-retail-erp-audit.md`
- 评审与对照：`docs/reports/2026-11-25-shuijing-retail-erp-audit-review.md`
- 当前状态：根 `README.md`、`db/migrations/README.md`、`internal/modules/*/README.md`、`api/openapi.yaml`、`tests/api/*`

**Plan type:** 本文件是**项目级路线图**，不是单个阶段的逐行实现计划。每个 Stage 启动前，按仓库现有流程另写一份 `docs/superpowers/plans/YYYY-MM-DD-stage-4x-*.md`，包含精确文件、代码、测试和提交步骤。路线图先用于用户审阅和排序确认。

> **执行状态（2026-11-25）：** Stage 4A 资金操作幂等已完成，实现计划 `docs/superpowers/plans/2026-11-25-stage-4a-idempotency.md`，报告 `docs/reports/2026-11-25-stage-4a-idempotency.md`。Stage 4B/4C 待用户确认。

---

## 0. 结论与建议

### 0.1 不要重做已完成的财务核心

审计报告把“完整总账、自动凭证、期末结转、多币种汇兑”列为 P0，但 `mlf-erp` 已在 Stage 3C–3F 完成：

- 总账：科目、期间、凭证、自动凭证、总账/明细账、试算平衡、资产负债表、利润表。
- 期末：损益结转、反关账、年度结转、利润分配。
- 税务：价税分离、销项/进项、VAT 汇总、发票状态。
- 多币种：汇率、原币/本位币、已实现汇兑、期末重估、反关账撤销重估。
- 现金流量表：直接法 + 简化间接法。
- 退货/贷项/退款：销售/采购退货、贷项单、抵用、退款、退款凭证。
- 反核销：收付款整行反核销、汇兑结算反核销凭证。

**结论：** 审计的 P0 财务清单大部分已落地；重做会浪费并引入回归风险。

### 0.2 真正的近期优先级

| 优先级 | 能力 | 理由 |
|---|---|---|
| P0 | 资金操作幂等与重复提交防护 | 退款/收付款/反核销/发货/收货/退货缺少 `Idempotency-Key`；重复提交会造成真实资金或库存副作用。`internal/modules/finance/README.md` 已明确退款和反核销生产前必须补幂等。 |
| P0 | 审批流与职责分离 | 当前只有提交/审批/取消状态流转；金额阈值、禁止自批、审批历史、职责分离缺失。 |
| P0 | 成本核算收口 | 已有移动平均 + COGS，但缺成本调整、月结成本、成本报表、负库存策略和存货-总账对账。 |
| P1 | 库存作业与批次/序列号 | 当前只有余额/流水查询；调拨、盘点、预留、预警、批次/序列号/保质期缺失，且会阻塞成本和报表。 |
| P1 | 权限精细化 | 有 RBAC + all/company/dept/self；缺字段级权限、数据范围管理 API/UI、权限实时刷新。 |
| P1 | 报表中心与对账 | 有 GL 财务报表；缺销售/采购/库存/账龄/子账-总账对账、导出和图表。 |
| P2 | 性能、安全、可观测性、灾备 | 没有性能基线、限流、RLS、指标/告警、备份恢复演练。 |
| P2 | 测试、文档、前端体验 | API 集成测试较强；缺 app/infra 深度单测、E2E、性能、用户手册、i18n、列配置。 |

### 0.3 推荐顺序

```text
Wave 1（P0，先做）
  Stage 4A 资金操作幂等
  Stage 4B 审批流与职责分离
  Stage 4C 成本核算收口

Wave 2（P1）
  Stage 4D 库存作业与批次/序列号
  Stage 4E 权限精细化
  Stage 4F 报表中心与对账

Wave 3（P2）
  Stage 4G 性能、安全、可观测性、灾备
  Stage 4H 测试、文档、前端体验
```

**立即下一步：Stage 4A。** 它自包含、风险最高、依赖最少，完成后可为后续审批/退款/反核销提供统一的重试安全基础。

---

## 1. 现状对照摘要

完整逐项对照见 `docs/reports/2026-11-25-shuijing-retail-erp-audit-review.md`。本路线图只保留决策相关的差距。

### 1.1 已完成

- 系统：用户、角色、权限、组织、审计日志、登录日志、编码规则。
- 主数据：客商、物料、仓库、单位、币种、税率。
- 采购：订单、收货、退货。
- 销售：订单、发货、退货、贷项。
- 库存：余额、流水、移动平均成本。
- 财务：应收/应付、收付款、核销、反核销、贷项、退款、发票、凭证、科目、期间。
- 总账：自动凭证、总账/明细账、试算平衡、资产负债表、利润表、现金流量表、VAT 汇总、辅助余额。
- 期末：损益结转、反关账、年度结转。
- 多币种：汇率、已实现汇兑、期末重估。
- 测试：Stage 3B–3K 真实 HTTP API 集成测试、113 个 Go 单元测试函数、前端静态完整度检查。

### 1.2 部分完成

- 成本核算：移动平均 + COGS，但缺成本调整/月结成本/成本报表/负库存策略/正式对账。
- 应收应付：发票/核销/反核销/贷项/退款，但缺账龄、对账单、信用额度。
- 权限：RBAC + 数据范围，但缺字段级、范围管理 UI、权限实时刷新。
- 报表：GL 财务报表，但缺销售/采购/库存/账龄/对账报表中心。
- 前端：主要表单有校验/加载/空态，但缺 i18n、列配置、E2E。
- 文档：OpenAPI/模块 README/阶段报告，但缺用户手册/运维 runbook，部分 README 过期。

### 1.3 未实现

- 资金操作幂等键与重复提交防护。
- 可配置审批流、金额阈值、职责分离、审批历史。
- 库存调拨、盘点/调整、预留、预警。
- 批次/序列号/保质期/追溯。
- 字段级权限、数据范围管理 API/UI。
- 销售/采购/库存/账龄/对账报表中心与导出。
- 安全加固（限流、token version、httpOnly、RLS、密钥、依赖扫描、渗透测试）。
- 可观测性（指标、追踪、告警、SLO）。
- 备份/恢复/灾备、迁移/切换方案。
- 用户手册、部署/运维 runbook、E2E/性能测试。
- 前端基础：`el-form` 规则校验、404、改密/个人资料、Dashboard 真实指标、金额/日期格式化、i18n、列配置。
- 契约完整性：OpenAPI 补齐 system/masterdata 路径，修正 `masterdata` enable 路径。
- AR/AP 发票行、手工开票、红字发票。
- 平台适配器与能力控制：Worker/Outbox/Redis/MinIO/RabbitMQ、`RequireCapability`、租户开通。

### 1.4 不适用或延后

- 零售 POS、门店补货、促销、会员、条码/电商：只有产品范围转向零售时才做。
- manufacturing/quality/project：当前目录骨架，且不在贸易/分销近期范围。
- 完整合并报表、多会计准则、完整 BI：长期能力。
- 重做总账/汇兑/期末结转：已完成。

---

## 2. 阶段路线图

### Stage 4A：资金操作幂等与重复提交防护（P0）— 已完成

**状态：** 2026-11-25 完成；实现计划 `docs/superpowers/plans/2026-11-25-stage-4a-idempotency.md`，报告 `docs/reports/2026-11-25-stage-4a-idempotency.md`。

**Goal:** 让所有会产生资金或库存副作用的 POST 操作可安全重试；同一业务请求重复提交只产生一次副作用。

**Why first:** 退款、收付款、反核销、发货、收货、退货都可能因网络超时/客户端重试重复执行；`finance/README.md` 已明确退款和反核销生产前必须补幂等。

**Scope:**
- 新迁移：`idempotency_keys`（`tenant_id`、`key`、`method`、`path`、`request_hash`、`status`、`response_code`、`response_body`、`created_at`、`expires_at`；唯一键 `(tenant_id, key, method, path)`）。
- 平台组件：`internal/platform/idempotency`，提供 `Idempotency-Key` 解析、请求哈希、响应回放、并发占位、过期清理。
- 首批接入：
  - `POST /finance/credit-notes/:id/refunds`
  - `POST /finance/payments`
  - `POST /finance/payments/:id/allocations`
  - `POST /finance/payments/:id/unallocate`
- 第二批接入：
  - `POST /sales/shipments`
  - `POST /purchase/receipts`
  - `POST /sales/returns`
  - `POST /purchase/returns`
- 错误码：400 `IDEMPOTENCY_KEY_REQUIRED`（如启用强制）、409 `IDEMPOTENCY_CONFLICT`、409 `IDEMPOTENCY_IN_PROGRESS`、422 `IDEMPOTENCY_KEY_REUSED`（同 key 不同 payload）。
- 审计：资金操作记录 before/after 快照；幂等回放记录命中次数。
- 前端：提交时生成 UUID，重复点击禁用，超时后允许用同一 key 重试。
- SDK/OpenAPI：`Idempotency-Key` header 参数、错误码、示例。
- 文档：`finance/README.md`、`gl/README.md`、根 README、OpenAPI。

**Acceptance criteria:**
- [ ] 同一 tenant + key + method + path + 相同 payload：重复请求返回首次响应，只产生一次副作用。
- [ ] 同一 key + 不同 payload：409 `IDEMPOTENCY_CONFLICT`，不执行第二次。
- [ ] 并发重复请求：最多一个执行，其余回放或 409；不出现双退款/双核销/双反核销。
- [ ] 不同 tenant 可使用相同 key，互不影响。
- [ ] 过期 key 可重新使用；清理任务不删除未完成记录。
- [ ] 退款、付款、核销、反核销各有 API 集成测试覆盖重复/并发/冲突/回滚。
- [ ] 前端重复提交被阻止；超时重试使用同一 key。
- [ ] OpenAPI/SDK/README 与实际行为一致。

**Non-goals:** 审批流（Stage 4B）；消息队列幂等；Outbox 重试改造（可后续接入）。

**Dependencies:** 无。

**Size:** M（约 1–2 周，单人；含测试/文档）。

**Risks:**
- 响应体可能很大，需设置大小上限/只存必要字段。
- 长事务期间并发请求的占位超时与清理。
- 旧客户端不传 key 的兼容策略（建议先警告后强制）。

---

### Stage 4B：审批流与职责分离（P0）

**Goal:** 提供可配置审批引擎，支持金额阈值、审批节点、审批人、条件、历史和禁止自批，并接入核心单据。

**Scope:**
- 新表：`approval_flows`、`approval_flow_steps`、`approval_requests`、`approval_actions`（或统一 `approval_*` 模型）。
- 领域：流程定义（单据类型、条件、步骤）、请求状态机（pending/approved/rejected/cancelled）、审批人解析（角色/用户/组织）、委托/升级（首期可只做顺序审批）。
- 接入首批：销售订单、采购订单、贷项退款、收付款反核销。
- 接入第二批：销售发货、采购收货、销售/采购退货、期间重开。
- 权限：`system.approval.read/write`、按单据类型的审批动作权限。
- API：流程 CRUD、提交/批准/拒绝/取消、待办列表、历史。
- 前端：审批流程配置页、待办页、单据审批面板。
- 审计：审批动作 append-only，记录审批人、时间、意见、前后状态。
- 测试：阈值路由、禁止自批、顺序/并行、拒绝/取消、权限、历史不可变。

**Acceptance criteria:**
- [ ] 超过阈值的单据必须审批后才能继续；低于阈值按配置自动通过或走默认流程。
- [ ] 申请人不能审批自己的请求；代审/委托有明确规则。
- [ ] 拒绝/取消不产生财务或库存副作用。
- [ ] 审批历史 append-only，可追溯到人、时间、意见、状态。
- [ ] 至少销售订单、采购订单、退款三条链路有 API 集成测试。
- [ ] 审批配置变更被审计；权限 401/403 正确。

**Non-goals:** 图形化流程设计器；复杂条件表达式引擎；移动端审批（首期）。

**Dependencies:** Stage 4A（提交/审批动作也需要幂等）。

**Size:** L（约 3–4 周）。

**Risks:**
- 审批人解析与组织/角色变更的缓存一致性。
- 历史单据的默认流程迁移。
- 自批规则与管理员例外。

---

### Stage 4C：成本核算收口（P0）

**Goal:** 在现有移动平均 + COGS 基础上补齐成本调整、月结成本、成本报表和存货-总账对账。

**Scope:**
- 成本调整单：调整库存成本，生成 GL 凭证，更新 `avg_cost`；支持原因、审批（可接 Stage 4B）、期间校验。
- 月结成本：冻结期间成本、重算 COGS、结转成本差异、生成月结成本报告；重复执行幂等。
- 负库存策略：禁止/允许并估算成本；明确超卖和负库存的会计处理。
- 成本报表：按物料/仓库的存货估值、发货 COGS 明细、成本调整历史、存货-总账对账表。
- 退货/调拨/盘点重算：退货按原成本回库；调拨/盘点依赖 Stage 4D，先做退货。
- 测试：独立重算 100 笔交易、退货、负库存、成本调整、GL 对账、期间关闭回滚。

**Acceptance criteria:**
- [ ] 期末存货 GL 余额 = 库存估值（在文档化舍入范围内）。
- [ ] 发货 COGS = 发货时点移动平均成本；退货按原成本冲回。
- [ ] 成本调整单更新 `avg_cost` 并生成平衡凭证。
- [ ] 月结成本幂等；关账期间拒绝反写；重跑结果一致。
- [ ] 成本报表可按物料/仓库/期间钻取。
- [ ] 负库存策略有明确错误码/会计处理，并有测试。

**Non-goals:** 成本中心、作业成本、标准成本、差异分析、分步成本、多工厂成本卷积、landed cost（后续独立阶段）。

**Dependencies:** GL（已完成）；Stage 4D 的调拨/盘点（可后接）。

**Size:** L（约 3–4 周）。

**Risks:**
- 历史成本数据不一致；需要修复迁移和对账工具。
- 退货/调拨/盘点的成本重算顺序。
- 月结与期间关账的锁顺序。

---

### Stage 4D：库存作业与批次/序列号（P1）

**Goal:** 从“余额/流水查询”扩展为可操作的库存模块：调拨、盘点/调整、预留、预警、批次/序列号/保质期。

**Scope:**
- 调拨：仓库到仓库，事务内扣加库存、写流水；同组织内不产生 GL，跨组织按内部交易处理（首期可限制同组织）。
- 盘点：盘点单、冻结、差异、调整、GL 盘盈/盘亏凭证。
- 预留：销售订单预留、发货/取消释放；`available = on_hand - reserved`。
- 预警：安全库存/最低/最高，生成通知/Outbox 事件。
- 批次/序列号/保质期：批次主数据、序列号、FEFO/FIFO、追溯（收货→发货→退货）。
- 前端：库存调拨、盘点、预留、预警、批次页面。
- 测试：调拨原子性、盘点差异、预留防超卖、批次追溯、保质期。

**Acceptance criteria:**
- [ ] 调拨/盘点后库存余额与流水一致；失败整体回滚。
- [ ] 预留阻止超卖；发货/取消正确释放。
- [ ] 盘点差异生成平衡的盘盈/盘亏凭证。
- [ ] 批次/序列号可从收货追溯到发货/退货。
- [ ] 预警在阈值触发，且有去重/抑制。
- [ ] API 集成测试覆盖并发调拨/盘点/预留。

**Non-goals:** 多工厂 MRP；自动补货；WMS 货位/波次；跨组织调拨（首期）。

**Dependencies:** Stage 4C（成本调整/对账）；销售/采购现有流水。

**Size:** XL；建议拆成 4D-1（调拨/盘点/预留/预警）和 4D-2（批次/序列号/保质期）。

**Risks:**
- 库存锁顺序与死锁；与销售发货/采购收货并发。
- 负库存与成本重算。
- 批次/序列号对现有单据行的迁移。

---

### Stage 4E：权限精细化（P1）

**Goal:** 在 RBAC + 数据范围之上增加字段级权限、数据范围管理 API/UI 和权限变更实时生效。

**Scope:**
- 字段级权限：表/字段、角色/组织、读/写/脱敏；API 响应过滤；前端列/字段隐藏或掩码。
- 数据范围管理：按角色/用户/组织配置 `all/company/dept/self`；预览影响；审计变更。
- 权限版本：角色/权限变更 bump version；token version 或菜单缓存失效；无需重新登录即可刷新（或明确提示）。
- 权限管理页面：独立权限 CRUD、角色-权限矩阵、字段策略。
- 测试：字段脱敏、范围组合、缓存失效、401/403、越权访问。

**Acceptance criteria:**
- [ ] 无字段权限的角色通过 API 也拿不到该字段（不仅是 UI 隐藏）。
- [ ] 数据范围变更在约定时间内生效，并有审计。
- [ ] 权限矩阵/字段策略有管理 UI 和 API。
- [ ] 关键模块（财务、库存、主数据）有字段级测试。
- [ ] 不存在仅靠前端隐藏的越权路径。

**Non-goals:** ABAC/策略引擎；临时授权/委托（可后续）；SSO/SCIM。

**Dependencies:** Stage 4B（审批权限）；跨模块。

**Size:** L（约 3–4 周）。

**Risks:**
- 字段级过滤对查询/导出的全面覆盖。
- 缓存一致性与性能。
- 历史角色/权限迁移。

---

### Stage 4F：报表中心与对账（P1）

**Goal:** 建立销售/采购/库存/账龄/对账报表中心，支持参数、权限、导出和钻取。

**Scope:**
- 前置：AR/AP 发票行/手工开票（当前仅表头）；如首期不做，账龄/对账先按表头金额。
- `reporting` 模块：数据集/查询模型、报表定义、参数、权限、导出（CSV/XLSX）、定时任务（worker）。
- 报表：销售汇总/明细/毛利、采购汇总/明细、库存余额/流水/库龄、应收/应付账龄、子账-总账对账、VAT 汇总（复用）、财务报表（复用）。
- 前端：报表中心、筛选、钻取、导出；Dashboard 接真实指标（订单/库存/财务摘要，可延后到 4H）。
- 测试：报表合计与 GL/子账对账、权限、大数据分页、导出内容一致。

**Acceptance criteria:**
- [ ] 销售/采购/库存/账龄报表合计与对应子账/GL 一致。
- [ ] 导出文件与页面一致；权限/数据范围正确。
- [ ] 大数据量分页/查询满足约定时间。
- [ ] 报表参数、钻取、空态、错误提示完整。
- [ ] 至少 3 张核心报表有 API 集成测试。

**Non-goals:** 自助 BI/拖拽分析；图表大屏；跨租户合并报表。

**Dependencies:** Stage 4C（成本/毛利）、4D（库龄）、4E（字段权限，可选）。

**Size:** L（约 3–4 周）。

**Risks:**
- 报表口径与 GL/子账不一致。
- 大数据查询性能。
- 导出权限与脱敏。

---

### Stage 4G：性能、安全、可观测性、灾备（P2）

**Goal:** 把系统从“功能可用”提升到“生产可运维、可审计、可恢复”。

**Scope:**
- 性能：基线（数据量/并发/p95）、慢查询、索引、N+1、分页、Redis 缓存与失效、压测。
- 安全：登录限流/锁定、token version、httpOnly Cookie 评估、RLS、密钥管理、依赖扫描、审计日志不可篡改、渗透测试。
- 可观测性：指标、追踪、告警、SLO、健康检查、过账失败/队列积压监控。
- 灾备：备份/恢复 runbook、RPO/RTO、HA/故障切换演练。
- 平台：Worker/Outbox/Redis/MinIO/RabbitMQ 适配；capability 强制；租户开通/管理。
- 测试：压测、安全扫描、恢复演练。

**Acceptance criteria:**
- [ ] 在代表性数据集/并发下达到定义的 p95/SLO。
- [ ] 无 Critical/High 安全扫描/渗透发现；限流/锁定生效。
- [ ] 备份恢复演练通过，RPO/RTO 有记录。
- [ ] 关键业务/基础设施指标有仪表盘和告警。
- [ ] RLS 与应用层过滤双保险；审计日志不可篡改。

**Non-goals:** 多区域容灾；全链路混沌工程；等保认证（可后续）。

**Dependencies:** Stage 4A–4F（可并行做安全子集）。

**Size:** L（约 3–4 周）。

**Risks:**
- 缓存与财务数据一致性。
- RLS 对现有查询的兼容。
- 性能优化缺少基线时容易过度设计。

---

### Stage 4H：测试、文档、前端体验（P2）

**Goal:** 补齐质量与可用性：深度单测、E2E、黄金财务数据集、用户手册、运维 runbook、i18n、列配置。

**Scope:**
- 测试：app/domain/infra 深度单测、Playwright E2E、财务黄金数据集、迁移测试、覆盖率目标。
- 工程：OpenAPI 生成 SDK、ESLint/Prettier、CI 门禁。
- 前端：`el-form` 规则校验、404 路由、改密/个人资料页、Dashboard 真实指标、金额/日期格式化、i18n、列配置、统一 loading/empty/error、无障碍。
- 契约：OpenAPI 补齐 12 个 system path（约 19–20 个 operation），修正 `masterdata` enable 路径，生成 SDK 并加契约测试。
- 文档：用户手册、部署/运维 runbook、过期 README 修正（`masterdata`/`inventory`）；修正 `docs/api/README.md` 的 `X-Tenant-ID` 描述（实际用 JWT claims）。
- 测试：补 Stage 3A 专项测试；GL 报表 SQL/库存查询/财务仓储的 DB 级测试；覆盖率、E2E、文档评审。

**Acceptance criteria:**
- [ ] 关键模块覆盖率达到约定目标；E2E 覆盖核心流程。
- [ ] CI 门禁阻止回归；OpenAPI SDK 与契约同步。
- [ ] 用户手册/runbook 经评审；过期 README 修正。
- [ ] OpenAPI 与后端路由一致（含 system/masterdata），`masterdata` enable 路径修正；契约测试通过。
- [ ] 前端有 `el-form` 规则、404、改密/个人资料；金额/日期格式化统一。
- [ ] i18n/列配置/无障碍检查通过。
- [ ] 前端构建/类型检查/完整度检查通过。

**Non-goals:** 100% 覆盖率；全浏览器矩阵；完整设计系统。

**Dependencies:** 持续；最终收口在 4A–4G 后。

**Size:** M–L。

**Risks:**
- 测试数据与真实财务口径偏差。
- i18n 对现有文案的改造量。
- 文档维护成本。

---

## 3. 依赖关系与执行顺序

```text
4A 幂等 ──→ 4B 审批 ──→ 4E 权限
   │            │
   └──→ 4C 成本 ──→ 4D 库存 ──→ 4F 报表
                                   │
                                   └──→ 4G 加固 ──→ 4H 质量
```

- **4A 必须最先**：后续审批、退款、反核销都依赖重试安全。
- **4B 与 4C 可并行**：不同模块，但都需要 4A 的幂等基础。
- **4D 在 4C 之后**：调拨/盘点会影响成本重算；但 4C 可先覆盖现有收货/发货/退货。
- **4E 在 4B 之后**：审批权限与字段级权限有交集。
- **4F 在 4C/4D 之后**：毛利/库龄依赖成本和库存作业。
- **4G/4H 最后**：但安全子集（限流、token version）可在 4A 后提前做。

**如果只有一名工程师：** 按 4A → 4B → 4C → 4D-1 → 4E → 4D-2 → 4F → 4G → 4H。  
**如果有两名：** 4A 后，一人 4B，一人 4C；再合流 4D/4E；最后 4F/4G/4H。

---

## 4. 质量门禁与 Definition of Done

每个 Stage 必须满足：

- [ ] 设计文档：目标、范围、非目标、数据模型、API、错误码、状态机、并发/回滚。
- [ ] 实现计划：精确文件、测试先行、分步提交。
- [ ] 代码审核：规格审核 + 质量审核；Critical/Important 清零。
- [ ] 测试：
  - Go 单元测试（domain/app/infra 关键路径）。
  - 真实 HTTP API 集成测试（新阶段脚本 + `pnpm test:api:stage4x`）。
  - 干净库全链路：当前 Stage 之前的全部 Stage 3 测试 + 新阶段。
  - 前端 `pnpm typecheck:web`、`pnpm test:frontend`、`pnpm build:web`。
- [ ] 数据一致性：金额/库存/GL 对账；迁移 up/down（或明确 no-op）。
- [ ] 文档：模块 README、OpenAPI、SDK、报告、已知限制。
- [ ] 用户审阅：报告 + 演示/验收清单。

**每个 Stage 的报告**沿用 `docs/reports/YYYY-MM-DD-stage-4x-*.md`，包含：完成范围、数据模型、事务/并发、API/前端、测试结果、审核结论、已知限制、验收清单。

---

## 5. 首批任务分解（Stage 4A）

> 以下是启动 Stage 4A 前的任务清单；详细实现计划在用户确认路线图后另写，包含精确文件、代码和测试步骤。

- [ ] **Task 1：迁移与领域模型**
  - 新增 `db/migrations/000026_idempotency_keys.up.sql` / `.down.sql`。
  - 表：`erp.idempotency_keys`；唯一键 `(tenant_id, key, method, path)`；状态 `in_progress/completed`；响应码/响应体/请求哈希/过期时间。
  - 更新 `db/migrations/README.md`。
- [ ] **Task 2：平台幂等组件**
  - `internal/platform/idempotency`：key 校验、请求哈希、占位、回放、冲突、过期。
  - 单元测试：同 key 同 payload、同 key 不同 payload、并发、过期、tenant 隔离。
- [ ] **Task 3：接入退款**
  - `POST /finance/credit-notes/:id/refunds` 读取 `Idempotency-Key`；重复请求回放；审计。
  - API 集成测试：重复退款只出账一次；并发只成功一次；失败回滚。
- [ ] **Task 4：接入收付款/核销/反核销**
  - `POST /finance/payments`、`/allocations`、`/unallocate`。
  - API 集成测试：重复付款/核销/反核销只生效一次；冲突 409；回滚无副作用。
- [ ] **Task 5：前端/SDK/OpenAPI**
  - 前端提交生成 UUID、禁用重复点击、超时用同一 key 重试。
  - SDK/OpenAPI 增加 header/错误码；`check-frontend` 增加断言。
- [ ] **Task 6：文档与报告**
  - 更新 `finance/README.md`、根 README、OpenAPI、Stage 4A 报告。
- [ ] **Task 7：干净库回归**
  - Stage 3B–3K + Stage 4A；重复/并发/回滚用例；数据核对。

---

## 6. 明确不做（近期）

- 零售 POS、门店补货、促销、会员/积分、条码/电商。
- manufacturing/quality/project 的完整实现。
- 完整合并报表、多会计准则、完整 BI/数据仓库。
- 重写总账、汇兑、期末结转、退货/贷项/退款、反核销。
- 把审计报告的 Flask/MySQL 表名/端点/页面直接映射到 `mlf-erp`。

---

## 7. 风险与假设

**假设：**
- 团队规模 1–2 名全栈工程师，沿用现有审核/测试节奏。
- 现有 Stage 3B–3K 测试保持通过；干净库可重建。
- 生产前必须补幂等/审批/安全基线；不把当前开发库当生产库。

**主要风险：**
- **范围过大**：4B/4C/4D 都是 L/XL，必须拆阶段、按验收标准收口。
- **历史数据**：成本调整、权限、审批、批次都可能需要迁移/回填；必须先做数据盘点。
- **会计口径**：成本/对账/报表必须与 GL 口径一致；需要黄金数据集和独立重算。
- **并发/锁**：库存、核销、反核销、审批的锁顺序需要统一设计，避免死锁。
- **性能**：没有基线时不要先上缓存；先测量再优化。
- **安全**：RLS/字段级权限是跨模块改造，需单独设计与回归。

---

## 8. 下一步

1. **用户审阅本路线图**：确认优先级、阶段拆分、是否先做 Stage 4A。
2. 确认后，为 Stage 4A 写详细实现计划：`docs/superpowers/plans/2026-11-25-stage-4a-idempotency.md`。
3. 按仓库流程执行：开发 → 规格/质量审核 → 接口测试 → 报告 → 用户审阅。
4. Stage 4A 完成后再启动 Stage 4B（审批）或 Stage 4C（成本），按团队并行度决定。
