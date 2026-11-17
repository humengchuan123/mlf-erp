# Stage 3E 税务/发票与价税分离设计

> 日期：2026-11-17
> 分支：`stage-3e-tax-invoice`
> 状态：设计已确认

## 1. 目标

在 Stage 3D 基础上补齐增值税价税分离、进项/销项税入账、发票开具状态和增值税汇总：

```text
含税/不含税单据
      ↓
行级税率快照 → 不含税金额 + 税额 = 含税金额
      ↓
销售发货：应收(含税) / 收入(不含税) + 销项税
采购收货：库存(不含税) + 进项税 / 应付(含税)
      ↓
增值税汇总表（销项、进项、应纳税额）
```

## 2. 范围

### 本期

- 销售订单、销售发货、采购订单、采购收货、应收/应付发票支持价税分离。
- 单据级 `price_includes_tax`，默认含税。
- 行级 `tax_rate_id` + `tax_rate` 快照，默认取物料税率，可覆盖。
- 应收/应付发票增加增值税发票信息、开票状态、开票/作废操作。
- 自动凭证拆分进项税/销项税。
- 增值税汇总表。
- 前端、SDK、OpenAPI、迁移、seed/demo、自动化测试。

### 后置

- 多币种、汇率、汇兑损益。
- 现金流量表。
- 完整增值税申报表、附加税、红字发票、开票审批流。
- 发票与税控系统对接。

## 3. 金额模型

### 3.1 字段

单据头（`sales_orders`、`sales_shipments`、`purchase_orders`、`purchase_receipts`）：

- `price_includes_tax boolean NOT NULL DEFAULT true`
- `net_amount numeric(19,4) NOT NULL DEFAULT 0`
- `tax_amount numeric(19,4) NOT NULL DEFAULT 0`
- `total_amount numeric(19,4)` 保持含税总额语义。

单据行（对应 `*_lines`）：

- `tax_rate_id uuid NULL`
- `tax_rate numeric(9,4) NOT NULL DEFAULT 0`
- `amount numeric(19,4)` 改为不含税金额语义。
- `tax_amount numeric(19,4) NOT NULL DEFAULT 0`
- `gross_amount numeric(19,4) NOT NULL DEFAULT 0`

应收/应付发票：

- `net_amount`、`tax_amount`，`total_amount` 为含税总额。
- `vat_invoice_no text NOT NULL DEFAULT ''`
- `vat_invoice_type text NOT NULL DEFAULT ''`，允许 `special`/`general`。
- `vat_invoice_date date NULL`
- `vat_invoice_status text NOT NULL DEFAULT 'pending'`，允许 `pending`/`issued`/`void`。
- `buyer_tax_no text NOT NULL DEFAULT ''`
- `seller_tax_no text NOT NULL DEFAULT ''`

### 3.2 计算

新增共享包 `internal/platform/taxcalc`：

```go
type Result struct {
    Net   decimal.Decimal
    Tax   decimal.Decimal
    Gross decimal.Decimal
}

func Calculate(quantity, unitPrice, rate decimal.Decimal, priceIncludesTax bool) (Result, error)
```

规则（行级 4 位小数）：

- 含税：`gross = round(qty × unitPrice, 4)`；`net = round(gross / (1 + rate), 4)`；`tax = gross - net`。
- 不含税：`net = round(qty × unitPrice, 4)`；`tax = round(net × rate, 4)`；`gross = net + tax`。
- 表头：`net_amount = Σ line.amount`，`tax_amount = Σ line.tax_amount`，`total_amount = Σ line.gross_amount`。
- 税率必须 `0 <= rate < 1`，数量/单价非负。
- 含税与不含税都必须保证 `net + tax = gross`。

### 3.3 历史数据

- 历史单据：`price_includes_tax=true`、`net_amount=total_amount`、`tax_amount=0`；行 `tax_amount=0`、`gross_amount=amount`、`tax_rate=0`、`tax_rate_id=NULL`。
- 历史凭证不重算。

## 4. 税率解析

- 订单行未指定税率时，使用物料 `tax_rate_id`。
- 订单行显式指定税率时，校验租户内启用税率。
- 保存 `tax_rate_id` 和 `tax_rate` 快照；税率主数据后续变更不影响历史单据。
- 发货/收货行从订单行复制税率快照。
- 税率主数据由现有 `GET /api/v1/masterdata/tax-rates` 提供。

## 5. 自动凭证

### 销售发货

- 借：应收账款 `ar`，金额 = 含税总额。
- 贷：主营业务收入 `revenue`，金额 = 不含税金额。
- 贷：应交税费-销项税额 `output_vat`，金额 = 销项税额（为 0 时不生成该行）。
- 成本分录不变：借主营业务成本 / 贷库存商品。

### 采购收货

- 借：库存商品 `inventory`，金额 = 不含税金额。
- 借：应交税费-进项税额 `input_vat`，金额 = 进项税额（为 0 时不生成该行）。
- 贷：应付账款 `ap`，金额 = 含税总额。

### 收付款

- 仍按含税总额核销应收/应付，不拆分税额。

### 发票开票/作废

- 不生成凭证；税额已在发货/收货时确认。
- 作废不回冲收入或税额，仅改变发票状态。

## 6. 科目与映射

新增：

- `222101 应交增值税-进项税额`：负债类，借方，末级。
- `222102 应交增值税-销项税额`：负债类，贷方，末级。
- `2221 应交税费` 改为非末级。

新增映射：

- `input_vat=222101`
- `output_vat=222102`

余额表按末级科目展示进项和销项；资产负债表中两者按负债正负相抵，净额为应交税费。

## 7. 发票

扩展现有 `finance_invoices`，不新建独立发票模块。

### 状态

- `pending`：待开票，应收/应付发票创建后的默认状态。
- `issued`：已开票，已录入税控发票号。
- `void`：已作废。

### 接口

| 方法 | 路径 | 权限 |
|---|---|---|
| POST | `/api/v1/finance/invoices/:id/issue` | `finance.invoice.write` |
| POST | `/api/v1/finance/invoices/:id/void` | `finance.invoice.write` |

开票请求：

```json
{
  "vat_invoice_no": "31000000000123456789",
  "vat_invoice_type": "special",
  "vat_invoice_date": "2026-11-17",
  "buyer_tax_no": "91310000XXXXXXXXXX",
  "seller_tax_no": "91310000YYYYYYYYYY"
}
```

规则：

- 仅 `pending` 可开票。
- `vat_invoice_no` 必填，租户内唯一（非空）。
- 仅 `issued` 可作废。
- 已作废发票不能再次开票。
- 开票/作废记录审计日志。
- 发票列表支持 `vat_invoice_status` 过滤和 `vat_invoice_no` 关键字。

## 8. 增值税汇总表

`GET /api/v1/finance/reports/vat-summary?start_date&end_date`，权限 `finance.report.read`。

返回：

- 按月行：`month`、`output_tax`、`input_tax`、`net_payable`。
- 合计：`total_output_tax`、`total_input_tax`、`total_net_payable`。

口径：

- 从 `222102` 销项税、`222101` 进项税已过账/已反过账凭证取数。
- 销项税按贷方余额；进项税按借方余额。
- 应用组织数据范围。
- 结转/年度结转凭证不涉及税务科目，无需特殊排除。

## 9. 前端

### 销售/采购订单

- 表头增加“含税/不含税”开关。
- 行增加税率选择（默认物料税率）。
- 行展示不含税金额、税额、含税金额。
- 表头展示不含税合计、税额合计、含税合计。
- 详情页展示同样字段。

### 发货/收货

- 详情展示税率、不含税金额、税额、含税金额。
- 发货/收货金额来自订单行。

### 应收应付发票

- 列表增加开票状态、增值税发票号、不含税金额、税额。
- 详情展示发票信息。
- 增加“开票”“作废”操作和对话框。
- 按开票状态过滤。

### 财务报表

- 增加“增值税汇总” Tab。
- 展示按月销项、进项、应纳税额和合计。

## 10. 迁移与兼容

新增迁移 `000017_tax_invoice`：

- 增加单据头/行、发票字段。
- 回填历史数据。
- 新增 `222101`/`222102` 科目，更新 `2221` 为非末级。
- 新增 `input_vat`/`output_vat` 映射。
- 新增 `finance.invoice.write` 权限并授予 admin 角色。
- 发票号部分唯一索引。

更新 `cmd/seed` 和 `cmd/demo`；Demo 增加含税和不含税示例。

## 11. 测试

- `taxcalc` 单元测试：含税、不含税、混合税率、零税率、四舍五入、非法税率。
- 销售订单→发货：含税总额、不含税收入、销项税、自动凭证。
- 采购订单→收货：不含税库存、进项税、含税应付、自动凭证。
- 发票开票/作废、重复发票号、状态限制、权限、401/403。
- 增值税汇总表销项/进项/应纳税额。
- 历史数据兼容、Stage 3B/3C/3D 回归。
- 前端完整度、类型检查、构建、OpenAPI 校验。

## 12. 验收标准

- [ ] 销售/采购单据支持含税/不含税切换和行级税率。
- [ ] 行级与表头净额、税额、含税金额计算正确且可追溯。
- [ ] 销售发货和采购收货自动凭证拆分进项/销项税。
- [ ] 应收应付发票支持开票、作废和发票号唯一。
- [ ] 增值税汇总表可用。
- [ ] 历史数据和已有报表不受影响。
- [ ] 前端、SDK、OpenAPI、迁移、seed/demo 完整。
- [ ] 自动化测试、静态检查、构建和回归通过。
