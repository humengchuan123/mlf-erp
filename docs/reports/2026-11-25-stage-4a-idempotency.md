# Stage 4A 汇总报告：资金操作幂等

> 日期：2026-11-25
> 分支：`stage-4a-idempotency`
> 功能/测试 HEAD：`b21f076`（报告与最终审核提交前）
> 设计/计划：`docs/superpowers/plans/2026-11-25-stage-4a-idempotency.md`
> 路线图：`docs/superpowers/plans/2026-11-25-audit-follow-up.md`
> 状态：开发、接口测试、干净库 Stage 3B–4A 回归和静态检查完成；代码审核结论待回填

---

## 1. 完成范围

### 1.1 目标

为会产生资金或库存副作用的 POST 接口增加 `Idempotency-Key` 重试安全：

- 同一租户 + 用户 + key + method + path + 相同请求体只执行一次。
- 重复请求回放首次响应，响应头 `Idempotency-Replayed: true`。
- 同一 key + 不同请求体返回 409 `IDEMPOTENCY_CONFLICT`。
- 并发同一请求返回 409 `IDEMPOTENCY_IN_PROGRESS`，首次完成后可重试回放。
- 记录默认保留 24 小时，过期后可重新使用；后台清理过期记录。

### 1.2 受保护端点

| 模块 | 端点 |
|---|---|
| 财务 | `POST /api/v1/finance/credit-notes/:id/allocations` |
| 财务 | `POST /api/v1/finance/credit-notes/:id/refunds` |
| 财务 | `POST /api/v1/finance/payments` |
| 财务 | `POST /api/v1/finance/payments/:id/allocate` |
| 财务 | `POST /api/v1/finance/payments/:id/unallocate` |
| 销售 | `POST /api/v1/sales/orders/:id/ship` |
| 销售 | `POST /api/v1/sales/shipments/:id/returns` |
| 采购 | `POST /api/v1/purchase/orders/:id/receive` |
| 采购 | `POST /api/v1/purchase/receipts/:id/returns` |

### 1.3 前端

- 6 个页面、8 个提交动作在打开弹窗时生成 `crypto.randomUUID()`：
  - 收付款创建、核销、反核销。
  - 贷项抵用、退款。
  - 销售发货、销售退货。
  - 采购收货、采购退货。
- 成功提交后清空 key；网络/5xx 保留 key 以便用同一 key 重试；4xx 重置 key，修正请求体后用新 key 重新提交。
- Header 默认可选；`IDEMPOTENCY_REQUIRED=true` 时强制。

### 1.4 提交范围

| 内容 | 提交 |
|---|---|
| 设计/计划 | `32ffaa1` |
| 迁移 26、配置、env | `caad72d` |
| 平台中间件、PostgreSQL 仓储、单元测试 | `e280006` |
| SDK、OpenAPI、前端 | `da5b11f` |
| 接口测试、脚本、README、CI | `8554955` |
| `cmd/api` 装配、状态刷新 | `b21f076` |
| 模块文档、报告、审核修复 | 本报告随最终文档提交 |

## 2. 数据模型（迁移 26）

新增 `erp.idempotency_keys`：

- 作用域：`(tenant_id, user_id, idempotency_key, method, path)` 唯一。
- 请求：`request_hash`（SHA-256）。
- 状态：`in_progress` / `completed` / `failed`。
- 响应回放：`response_code`、`response_body`（text，保留精确字节）、`response_content_type`、`response_truncated`。
- 时间：`created_at`、`updated_at`、`expires_at`。
- 索引：`expires_at`，用于清理。
- Down：直接删除表；幂等记录是短期运行数据，不做归档。

## 3. 状态机与错误码

```text
insert -> in_progress
in_progress --2xx/3xx/4xx(非401/403/429)--> completed
in_progress --5xx/401/403/429--> failed
failed --same hash--> in_progress（允许重试）
completed --same hash--> replay
completed/in_progress --different hash--> 409 conflict
expired --any hash--> in_progress（重新占用）
```

| HTTP | code | 场景 |
|---|---|---|
| 400 | `IDEMPOTENCY_KEY_REQUIRED` | 强制模式下缺少 header |
| 400 | `IDEMPOTENCY_KEY_INVALID` | key 格式不合法 |
| 409 | `IDEMPOTENCY_CONFLICT` | 同 key 不同请求 |
| 409 | `IDEMPOTENCY_IN_PROGRESS` | 同 key 同请求仍在执行 |
| 409 | `IDEMPOTENCY_RESPONSE_UNAVAILABLE` | 首次响应过大未保存，无法回放 |

- Key 格式：去空白后 8–128 字符，允许 `A-Z a-z 0-9 . _ : -`。
- Header 缺失默认放行；强制模式返回 400。
- 5xx/401/403/429 标记 `failed`，同一 key 可重试。
- 幂等记录先于业务事务存在；业务失败回滚后记录标记 `failed`，不会留下半成品业务数据。

## 4. 实现结构

- `internal/platform/idempotency`：
  - `middleware.go`：路由白名单、key 校验、请求哈希、占用/回放/冲突、响应捕获、完成/失败。
  - `postgres_repository.go`：`INSERT ON CONFLICT` + `SELECT FOR UPDATE` 状态机。
  - `cleanup.go`：周期删除过期记录。
  - `model.go` / `config.go`：模型、默认值、默认白名单。
- `cmd/api/main.go`：鉴权之后、业务路由之前 `protected.Use(...)`；启动清理 goroutine。
- `internal/platform/config`：`IDEMPOTENCY_TTL`、`IDEMPOTENCY_REQUIRED`、请求/响应大小、清理间隔。
- `internal/platform/httpx`：新增幂等错误码。
- SDK：`RequestOptions`，`post` 支持 `Idempotency-Key`；财务/销售/采购方法可选 key。
- OpenAPI 1.2.0：新增 `IdempotencyKey` header，9 个操作和 400/409 说明。
- 前端：6 个视图 8 个 key，打开弹窗生成、成功清空、失败保留。
- 测试：`TestStage4A` 8 个顶层子测试；`scripts/test/stage4a-api.ps1`；CI 改为 `TestStage3|TestStage4`。

## 5. 测试结果

### 5.1 单元测试

```bash
go test ./internal/platform/idempotency/... -count=1
```

覆盖：
- 首次执行/完成、同 key 回放、不同 body 冲突。
- in-progress、response unavailable。
- 缺少 key 可选/强制、非法 key、非白名单路由。
- 500 后重试、401/403/429 可重试。
- 响应截断、204 空响应、capture writer 状态/正文/Content-Type。
- key 校验、请求哈希稳定性。

### 5.2 接口测试

`TestStage4A`（本地 Compose PostgreSQL，版本 26）：

| 子测试 | 结果 |
|---|---|
| `payment create replay` | PASS |
| `allocate replay` | PASS |
| `unallocate replay` | PASS |
| `refund replay` | PASS |
| `concurrent refund same key` | PASS |
| `sales ship replay` | PASS |
| `purchase receive replay` | PASS |
| `missing key still works` | PASS |

`PASS`，约 1.6–2.0s。

### 5.3 干净库全链路

数据库：`mlf_erp_stage4a_clean`；迁移、seed、demo、独立 API（端口 18091）。

```bash
go test -tags=integration ./tests/api -run 'TestStage3|TestStage4' -count=1 -v
```

结果：Stage 3B、3C、3D、3E、3F、3G、3H、3I、3J、3K、4A 全部通过；Stage 4A 8 个子测试通过；总耗时约 36.8s。

数据核对：

| 指标 | 结果 |
|---|---|
| `schema_migrations` | 版本 26，`dirty = false` |
| 幂等记录 | 7 条，全部 `completed` |
| 反核销记录 | 23 条 |
| 汇兑结算 | 4 posted / 7 reversed |
| `fx_settlement_reversal` 凭证 | 7 张 |
| 反核销凭证借贷不平衡 | 0 |
| 付款本位币与核销明细不一致 | 0 |
| 发票本位币已付与核销明细不一致 | 0 |

### 5.4 静态检查与前端

| 命令 | 结果 |
|---|---|
| `gofmt -l ./cmd ./internal ./db ./tests` | 无输出 |
| `go build ./...` | 通过 |
| `go test ./... -count=1` | 全部包通过 |
| `go vet ./...` | 通过 |
| `go vet -tags=integration ./tests/api` | 通过 |
| `npx -y yaml@2 valid api/openapi.yaml` | 通过 |
| `pnpm typecheck:web` | 通过 |
| `pnpm test:frontend` | `Frontend completeness check passed.` |
| `pnpm build:web` | 通过（仅既有 chunk-size 警告） |
| `git diff --check` | 无空白错误 |

## 6. 代码审核

**pending。** 待规格审核和质量审核结论回填；Critical/Important 修复后重跑 Stage 4A 和干净库相关用例。

## 7. 已知限制

- `Idempotency-Key` 默认可选；未传 key 的旧客户端仍可能重复提交。生产强制前需完成所有客户端接入，再设置 `IDEMPOTENCY_REQUIRED=true`。
- 幂等记录以明文保存响应体，保留 24 小时；依赖数据库访问控制和租户隔离，后续可加密/脱敏。
- 响应超过 4 MB 时无法回放，返回 409 `IDEMPOTENCY_RESPONSE_UNAVAILABLE`。
- 清理任务运行在 API 进程内；后续可移到 Worker。
- 只持久化 `Content-Type`，其他首次响应头不回放。
- 业务 handler panic 时记录会保持 `in_progress` 直到过期，客户端无法立即重试；这是保守的 fail-closed 行为。
- `Complete`/`Fail` 持久化失败时仍返回业务响应，记录可能保持 `in_progress`；不会重复执行，但客户端需等待过期或人工处理。
- 不覆盖消息队列/Outbox 重试、GET 查询和客户端自动重试 UI。
- 4xx 响应会被缓存；前端在 4xx 时重置 key，修正后使用新 key；网络/5xx 和 `IDEMPOTENCY_IN_PROGRESS`/`IDEMPOTENCY_RESPONSE_UNAVAILABLE` 保留 key。

## 8. 验收清单

- [x] 同一 tenant + user + key + method + path + 相同 payload：重复请求回放首次响应，只产生一次副作用。
- [x] 同一 key + 不同 payload：409 `IDEMPOTENCY_CONFLICT`。
- [x] 并发同 key 同 payload：最多一个执行，其余回放或 409；无双退款/双核销/双反核销/双发货/双收货。
- [x] 不同 tenant/user 可使用相同 key，互不影响（repository 作用域 + 鉴权）。
- [x] 过期 key 可重新使用；清理任务删除过期记录。
- [x] 5xx/401/403/429 可用同一 key 重试。
- [x] 前端 6 个视图提交时生成并传递 key；成功后清空。
- [x] 9 个受保护端点全部有 OpenAPI header 和 409 说明。
- [x] `TestStage4A` 通过；Stage 3B–3K 回归通过。
- [x] 干净库版本 26，全链路通过。
- [ ] 模块文档/报告/审核结论完整（本提交完成）。
- [ ] 用户审阅通过。

## 9. 后续

- Stage 4A 完成后进入 Stage 4B（审批流与职责分离）或 Stage 4C（成本核算收口），按用户确认的路线图执行。
- 生产强制 `IDEMPOTENCY_REQUIRED=true` 前，先完成客户端全量接入和灰度观察。
- 将清理任务迁移到 Worker；评估响应体加密/脱敏和幂等记录归档策略。
