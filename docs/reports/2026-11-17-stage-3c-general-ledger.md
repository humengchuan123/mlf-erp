# Stage 3C 汇总报告：总账、自动凭证与基础报表

> 日期：2026-11-17
> 分支：`stage-3c-general-ledger`
> 提交：`6ff728a`（实现）+ `427bd71`（会计完整性加固）+ `1524ba9`（报告/设计收尾）+ `ce22a2e`（审核修复）
> 设计：`docs/superpowers/specs/2026-11-17-stage-3c-general-ledger-design.md`
> 计划：`docs/superpowers/plans/2026-11-17-stage-3c-general-ledger.md`
> 状态：待用户审阅

---

## 1. 完成范围

### 1.1 总账基础

- 会计科目表：23 个标准科目，支持资产/负债/权益/收入/费用、借贷方向、上级科目、启停。
- 会计期间：按年/月，支持关账、反关账；关账后禁止新增/过账凭证。
- 科目映射：应收、应付、收入、库存、成本、银行、现金默认科目。
- 凭证：凭证头/分录、来源单据、状态 `draft/posted/reversed`、借贷平衡校验。
- 手工凭证：创建/编辑草稿、过账、反过账。
- 自动凭证：销售发货、采购收货、客户收款、供应商付款在业务事务内同步生成。
- 凭证回填：`cmd/glbackfill` 为历史业务单据补生成凭证，幂等。

### 1.2 自动凭证规则

| 业务单据 | 借方 | 贷方 |
|---|---|---|
| 销售发货（收入） | 应收账款 1122 | 主营业务收入 6001 |
| 销售发货（成本） | 主营业务成本 6401 | 库存商品 1405 |
| 采购收货 | 库存商品 1405 | 应付账款 2202 |
| 客户收款 | 银行存款/现金 | 应收账款 1122 |
| 供应商付款 | 应付账款 2202 | 银行存款/现金 |

- 自动凭证按 `(tenant_id, source_type, source_id)` 幂等。
- 业务事务失败/关账期间会整体回滚，不会出现业务单据已生成但凭证缺失的情况。

### 1.3 报表

- 科目余额表：期初、本期借方、本期贷方、期末余额。
- 总账/明细账：按科目列示凭证分录和余额。
- 资产负债表：资产、负债、权益、未结转利润（累计）。
- 利润表：收入、费用、净利润。
- 统计 `posted` 和 `reversed` 凭证；原凭证保留在历史期间，反过账凭证在反过账期间抵消，应用租户/数据权限。

### 1.4 前端

- 会计科目：树形列表、新建/编辑、启停。
- 会计期间：列表、关账/反关账。
- 会计凭证：列表、手工录入、详情、过账、反过账。
- 财务报表：科目余额表、明细账、资产负债表、利润表。
- 菜单、路由、权限、SDK。

### 1.5 权限

- `finance.account.read/write`
- `finance.period.read/close`
- `finance.voucher.read/write/post/reverse`
- `finance.report.read`

---

## 2. 数据库变更

迁移 `000013_general_ledger`：

- `erp.gl_accounts`
- `erp.gl_periods`
- `erp.gl_account_mappings`
- `erp.gl_vouchers`
- `erp.gl_voucher_lines`
- 9 个总账权限
- `gl_voucher` 编码规则：`GL-yyyyMM-0001`，月重置
- 标准科目、科目映射、当前年度 12 个会计期间

关键约束：

- 科目 `(tenant_id, code)` 唯一。
- 期间 `(tenant_id, year, month)` 唯一。
- 自动凭证来源 `(tenant_id, source_type, source_id)` 唯一，防止重复过账。
- 凭证分录借方/贷方非负，单行只能一边有金额。
- 凭证行关联科目，凭证关联期间。

---

## 3. API 清单

| 方法 | 路径 | 权限 |
|---|---|---|
| GET/POST | `/api/v1/finance/accounts` | `finance.account.read/write` |
| PUT | `/api/v1/finance/accounts/:id` | `finance.account.write` |
| GET | `/api/v1/finance/periods` | `finance.period.read` |
| POST | `/api/v1/finance/periods/:id/close` | `finance.period.close` |
| POST | `/api/v1/finance/periods/:id/reopen` | `finance.period.close` |
| GET/POST | `/api/v1/finance/vouchers` | `finance.voucher.read/write` |
| GET/PUT | `/api/v1/finance/vouchers/:id` | `finance.voucher.read/write` |
| POST | `/api/v1/finance/vouchers/:id/post` | `finance.voucher.post` |
| POST | `/api/v1/finance/vouchers/:id/reverse` | `finance.voucher.reverse` |
| GET | `/api/v1/finance/reports/trial-balance` | `finance.report.read` |
| GET | `/api/v1/finance/reports/general-ledger` | `finance.report.read` |
| GET | `/api/v1/finance/reports/balance-sheet` | `finance.report.read` |
| GET | `/api/v1/finance/reports/income-statement` | `finance.report.read` |

金额和数量均使用字符串传输。OpenAPI 已补充总账科目、期间、凭证和报表接口。

---

## 4. 测试结果

### 4.1 静态检查与构建

| 检查 | 结果 |
|---|---|
| `gofmt -l ./cmd ./internal ./db ./tests` | 通过 |
| `go test ./...` | 通过 |
| `go vet ./...` | 通过 |
| `pnpm typecheck:web` | 通过 |
| `pnpm build:web` | 通过（Element Plus 主包约 958KB，chunk size warning） |
| `pnpm test:frontend` | 通过 |
| OpenAPI YAML 校验 | 通过 |

### 4.2 自动化接口测试

执行：

```bash
pnpm test:api:stage3c
# 以及
pnpm test:api:stage3b
```

结果：`TestStage3C` 9 个子测试全部通过，`TestStage3B` 11 个子测试全部通过；同一 API 进程执行 `TestStage3`（Stage 3B + 3C）也通过。

| 用例 | 结果 |
|---|---|
| 采购收货自动凭证：借库存 1405 / 贷应付 2202 | 通过 |
| 销售发货自动凭证：借应收/贷收入 + 借成本/贷库存 | 通过 |
| 客户收款自动凭证：借银行/贷应收 | 通过 |
| 供应商付款自动凭证：借应付/贷银行 | 通过 |
| 自动凭证金额与库存出库成本一致 | 通过 |
| 科目被使用后修改编码/类型/方向 | 409 `CONFLICT` |
| 手工凭证创建/过账 | 通过 |
| 手工凭证反过账，原凭证 reversed，生成反向凭证 | 通过 |
| 历史期间凭证在当期反过账 | 历史期间报表不变，当期出现反向分录 |
| 自动凭证直接反过账 | 409 `INVALID_STATUS` |
| 自动凭证返回 `voucher_code` | 通过 |
| 期间存在草稿凭证时关账 | 409 `CONFLICT` |
| 关账后手工凭证过账 | 409 `PERIOD_CLOSED` |
| 关账后销售发货/收付款自动过账 | 409 `PERIOD_CLOSED` |
| 科目余额表借贷平衡 | 通过 |
| 资产负债表：资产 = 负债 + 权益 + 本年利润 | 通过 |
| 利润表：净利润 = 收入 - 费用 | 通过 |
| 明细账期初/分录/期末 | 通过 |
| demo_dept 查看 HQ 凭证/报表 | 0 |
| demo_dept 创建凭证 | 403 |
| 无 token 访问总账接口 | 401 |

### 4.3 历史数据回填

首次执行 `go run ./cmd/glbackfill`：

```text
gl backfill completed: sales_shipments=53 purchase_receipts=12 payments=57
```

再次执行：

```text
gl backfill completed: sales_shipments=0 purchase_receipts=0 payments=0
```

说明自动凭证回填幂等，不会重复生成。

### 4.4 报表抽样

截至测试时：

```text
已过账凭证借贷合计：72,964.1477 / 72,964.1477
资产 signed balance：10,657.8523
负债 signed balance：-1,950.0000
权益 signed balance：-5.0000
收入 signed balance：-20,410.0000
费用 signed balance：11,707.1477
本年利润：8,702.8523
资产 = 负债 + 权益 + 本年利润
```

### 4.5 前端完整度

`check-frontend.mjs` 验证：

- 路由包含会计科目、会计期间、会计凭证、财务报表及对应权限。
- 菜单包含四个总账入口。
- 四个页面文件存在，包含表格、弹窗/抽屉和对应 SDK 调用。
- SDK 导出 `glApi`，包含科目、期间、凭证、报表方法。

---

## 5. 代码审核

### 5.1 审核方式

独立代码审核子代理，审查范围 `28d178d..6ff728a`；审核发现已在 `427bd71` 及后续提交中修复并回归。覆盖自动凭证事务、幂等、总账凭证生命周期、报表口径、数据权限、迁移、前端和测试。

### 5.2 审核结论

**Ready to merge: With fixes（初次审核）**

- **Critical 3 项**：
  1. 科目余额表只取启用末级科目，禁用科目或已有发生额的非末级科目余额会从报表消失，导致报表间不一致。
  2. 反过账把原凭证标记 `reversed` 并排除，反过账凭证也被排除，历史/已关账期间报表会被追溯修改。
  3. 期间关账与并发过账没有期间行锁，凭证可能提交到刚关账的期间。
- **Important**：自动凭证错误未映射到业务接口（500 而非 409/400）、草稿过账未重新校验科目状态、科目编码/类型/状态修改可能破坏自动过账、历史回填凭证号按当前时间编号、自动凭证允许直接反过账、测试缺少关键失败路径。
- 审核同时确认自动凭证事务位置、借贷方向、移动平均成本、来源幂等、报表计算和前端支撑代码整体正确。
- **审核修复后结论：Ready to merge: Yes**。Critical 3 项和主要 Important 项已修复并回归，剩余项记录为已知限制。

### 5.3 已修复问题

- **Critical 1：报表科目完整性**
  - 科目余额表改为包含所有末级科目，以及已有发生额的非末级科目。
  - 明细账允许查询禁用/非末级历史科目。
  - 增加科目使用检查：已使用或已映射科目禁止修改编码/类型/方向、禁止停用、禁止在其下新增子科目。
- **Critical 2：反过账报表语义**
  - 报表改为统计 `posted` 和 `reversed` 凭证，不再排除反过账凭证。
  - 原凭证保留在历史期间，反向凭证在反过账期间抵消。
  - 新增历史反过账测试：上月凭证本月反过账，上月报表不变，本月出现反向分录。
- **Critical 3：关账并发**
  - 所有过账/凭证创建路径使用期间行 `FOR SHARE`，关账使用 `FOR UPDATE`。
  - 关账后业务发货/收货/收付款自动过账返回 409 `PERIOD_CLOSED`，业务事务整体回滚。
- **Important**
  - 销售/采购/财务 Handler 增加 GL 错误映射：预算关账、科目状态、配置缺失、凭证不平。
  - 草稿过账时重新校验科目末级/启用状态。
  - 科目层级增加自引用/循环校验、父科目使用检查。
  - 自动凭证只允许手工凭证反过账；自动凭证反过账返回 409，需走业务反向流程。
  - 新增科目完整性、草稿阻止关账、关账后自动过账、历史反过账、自动凭证反向限制等测试。
  - 销售发货/采购收货/收付款响应返回 `voucher_code`，SDK 同步。
- **Minor**
  - 新增迁移 `000014_general_ledger_indexes`。
  - 报表日期范围 `start > end` 返回 400。
  - 凭证行空借方/贷方按 0 处理。
  - 凭证创建/更新响应补金额和时间戳，修正 `CreatedBy`。
  - 前端“本年利润”改为“未结转利润（累计）”。

### 5.4 未修复/后续处理

- 来源幂等仍是“先查后插 + 唯一索引兜底”；并发同来源回填可能有一次 409，不会重复过账。
- 凭证号按生成/过账时间编号，历史回填使用回填时间；凭证日期单独记录，报表按日期取数。已作为审计约定记录。
- 科目层级没有数据库 FK/触发器，父级 `is_leaf` 和 level 的数据库级一致性依赖服务层。
- 新租户开通需要复制标准科目、映射、期间和编码规则；目前仅 seed/demo 覆盖。
- 报表为实时聚合，大数据量需要余额表/物化视图。
- 未实现现金流量表、损益结转、税额拆分、辅助核算报表、多币种。
- 缺少并发关账/过账、事务故障注入、真实浏览器 E2E 测试。

---

## 6. 已知限制

> 注：Stage 3D 已实现期末损益结转、年度结转和辅助核算，下列限制仅反映 Stage 3C 当时状态；最新状态见 Stage 3D 报告。

- 未实现现金流量表、期末损益结转、年度结转。
- 未结转利润按“收入 - 费用”累计实时计算，4103 本年利润科目在未实现损益结转前应保持为零。
- 未拆分增值税/价税分离，自动凭证按单据总额入账。
- 未实现辅助核算（客商/部门/项目/存货）报表；凭证行已预留 partner_id/department_id。
- 未实现多币种、汇率、汇兑损益。
- 未实现合并报表、预算、BI。
- 自动凭证的科目映射目前通过初始化和数据库维护，未提供映射配置页面。
- 自动凭证不允许直接反过账，需通过业务退货/反核销等业务反向流程处理；本期业务反向流程尚未实现，可先使用手工调整凭证。
- 凭证号按生成/过账时间编号，历史回填使用回填时间；凭证日期单独记录，报表按凭证日期取数。
- 禁用科目仍会出现在历史报表中，以保证总账平衡；禁用后不能再用于新凭证。
- 新租户开户时需要复制标准科目、映射和编码规则。
- 报表为实时聚合，大数据量下需要余额表/物化视图优化。
- 前端金额展示/预填仍使用 JavaScript `Number`，后端金额计算是权威的。
- 自动化测试覆盖主要路径，缺少并发关账/过账、事务故障注入、真实浏览器 E2E。

---

## 7. 下一阶段建议

Stage 3D：

- 现金流量表、期末损益结转、年度结转。
- 辅助核算（客商、部门、项目、存货）和辅助余额表。
- 税务/发票、价税分离。
- 多币种、汇率、汇兑损益。
- 报表性能优化和月结批处理。

也可先补 Stage 3B 遗留：销售/采购退货、反核销、账龄分析、客户信用。

---

## 8. 用户审阅清单

- [ ] 会计科目和期间设计是否符合业务。
- [ ] 自动凭证规则和科目映射是否接受。
- [ ] 关账后禁止业务过账是否符合预期。
- [ ] 科目余额表、资产负债表、利润表口径是否接受。
- [ ] 前端页面和 Demo 数据是否满足演示。
- [ ] 代码审核结论和修复是否认可。
- [ ] 是否允许进入 Stage 3D。
