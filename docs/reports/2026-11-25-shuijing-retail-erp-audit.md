# 水镜零售 ERP 系统全面审计与后续开发建议

> **归档说明**
>
> - 来源：用户提供的外部审计报告，报告日期 2026-08-03。
> - 归档日期：2026-11-25。
> - 原文技术栈：Python Flask + SQLAlchemy + MySQL + Vue 3 + Element Plus + TypeScript。
> - 当前仓库 `mlf-erp` 技术栈：Go + Gin + PostgreSQL + Vue 3 + Element Plus + TypeScript。
> - 本文件保留审计原文的结构与结论，供后续对照核实；与当前代码库的差异、适用性和优先级见 `2026-11-25-shuijing-retail-erp-audit-review.md` 和 `docs/superpowers/plans/2026-11-25-audit-follow-up.md`。

---

## 一、项目现状总览

### 1.1 技术栈

- **后端**：Python 3.10+、Flask 2.3、SQLAlchemy 2.0、Flask-SQLAlchemy、Flask-Migrate、Flask-JWT-Extended、Marshmallow、Celery、Redis。
- **前端**：Vue 3.3、TypeScript 5.0、Element Plus 2.3、Pinia 2.1、Vue Router 4.2、Axios、Vite 4.4。
- **数据库**：MySQL 8.0、Redis 7.0。
- **部署**：Docker、Docker Compose、Nginx。
- **代码规模**：后端约 150 个 Python 文件，前端约 80 个 Vue 文件，数据库 60+ 张表，API 100+ 个。

### 1.2 架构设计

- 后端采用模块化单体架构，按业务域划分为 system、masterdata、sales、purchase、inventory、finance、report 等模块。
- 每个模块包含 models、schemas、services、routes 等分层。
- 前端采用 Vue 3 Composition API + TypeScript + Element Plus，按模块划分 views、components、api、stores。
- 采用 JWT 认证和 RBAC 权限控制。
- 数据库使用 SQLAlchemy ORM，Alembic 管理迁移。

### 1.3 已完成阶段

- 阶段一：基础架构搭建（用户、角色、权限、组织、字典）。
- 阶段二：主数据管理（客户、供应商、物料、仓库、单位、币种、税率）。
- 阶段三：销售管理（报价、订单、发货、退货）。
- 阶段四：采购管理（申请、订单、收货、退货）。
- 阶段五：库存管理（库存查询、流水、盘点、调拨）。
- 阶段六：财务基础（应收、应付、收付款、发票、凭证、科目、期间）。
- 阶段七：报表中心（销售、采购、库存、财务报表）。

## 二、已实现功能模块详细清单

### 2.1 系统管理

- **用户管理**：用户列表、新增、编辑、禁用、重置密码、分配角色。
- **角色管理**：角色列表、新增、编辑、删除、分配权限。
- **权限管理**：权限树、菜单权限、按钮权限、API 权限。
- **组织管理**：组织树、新增、编辑、删除。
- **字典管理**：字典类型、字典项。
- **操作日志**：登录日志、操作日志。
- **系统配置**：参数配置。

### 2.2 主数据

- **客户管理**：客户列表、新增、编辑、删除、导入导出。
- **供应商管理**：供应商列表、新增、编辑、删除、导入导出。
- **物料管理**：物料列表、新增、编辑、删除、分类、单位、税率。
- **仓库管理**：仓库列表、新增、编辑、删除。
- **计量单位**：单位列表、新增、编辑、删除、换算。
- **币种管理**：币种列表、新增、编辑、删除、汇率。
- **税率管理**：税率列表、新增、编辑、删除。

### 2.3 销售管理

- **报价单**：报价列表、新增、编辑、删除、转订单。
- **销售订单**：订单列表、新增、编辑、删除、审核、关闭、转发货。
- **销售发货**：发货列表、新增、编辑、删除、确认发货、转发票。
- **销售退货**：退货列表、新增、编辑、删除、确认退货。
- **价格管理**：价格策略、客户价格、物料价格。

### 2.4 采购管理

- **采购申请**：申请列表、新增、编辑、删除、审核、转订单。
- **采购订单**：订单列表、新增、编辑、删除、审核、关闭、转收货。
- **采购收货**：收货列表、新增、编辑、删除、确认收货、转发票。
- **采购退货**：退货列表、新增、编辑、删除、确认退货。

### 2.5 库存管理

- **库存查询**：按仓库、物料查询库存。
- **库存流水**：入库、出库、调拨、盘点流水。
- **库存盘点**：盘点单、盘点明细、盘点调整。
- **库存调拨**：调拨单、调拨明细、确认调拨。
- **库存预警**：安全库存、最低库存、最高库存。

### 2.6 财务基础

- **应收管理**：应收列表、应收明细、账龄分析。
- **应付管理**：应付列表、应付明细、账龄分析。
- **收款管理**：收款单、收款核销、收款查询。
- **付款管理**：付款单、付款核销、付款查询。
- **发票管理**：销售发票、采购发票、发票查询。
- **凭证管理**：凭证录入、凭证审核、凭证查询、凭证打印。
- **科目管理**：会计科目树、科目新增、科目编辑。
- **期间管理**：会计期间、期间结账、期间反结账。

### 2.7 报表中心

- **销售报表**：销售汇总、销售明细、销售毛利。
- **采购报表**：采购汇总、采购明细。
- **库存报表**：库存汇总、库存明细、库存流水。
- **财务报表**：资产负债表、利润表、现金流量表。

## 三、数据库表清单

### 3.1 系统管理表

| 表名 | 说明 |
|---|---|
| `sys_user` | 用户表 |
| `sys_role` | 角色表 |
| `sys_permission` | 权限表 |
| `sys_user_role` | 用户角色关联表 |
| `sys_role_permission` | 角色权限关联表 |
| `sys_organization` | 组织表 |
| `sys_dictionary` | 字典表 |
| `sys_dictionary_item` | 字典项表 |
| `sys_operation_log` | 操作日志表 |
| `sys_login_log` | 登录日志表 |
| `sys_config` | 系统配置表 |

### 3.2 主数据表

| 表名 | 说明 |
|---|---|
| `md_customer` | 客户表 |
| `md_supplier` | 供应商表 |
| `md_material` | 物料表 |
| `md_material_category` | 物料分类表 |
| `md_warehouse` | 仓库表 |
| `md_unit` | 计量单位表 |
| `md_currency` | 币种表 |
| `md_exchange_rate` | 汇率表 |
| `md_tax_rate` | 税率表 |

### 3.3 销售管理表

| 表名 | 说明 |
|---|---|
| `sd_quotation` | 报价单表 |
| `sd_quotation_item` | 报价单明细表 |
| `sd_sales_order` | 销售订单表 |
| `sd_sales_order_item` | 销售订单明细表 |
| `sd_sales_delivery` | 销售发货表 |
| `sd_sales_delivery_item` | 销售发货明细表 |
| `sd_sales_return` | 销售退货表 |
| `sd_sales_return_item` | 销售退货明细表 |

### 3.4 采购管理表

| 表名 | 说明 |
|---|---|
| `pd_purchase_request` | 采购申请表 |
| `pd_purchase_request_item` | 采购申请明细表 |
| `pd_purchase_order` | 采购订单表 |
| `pd_purchase_order_item` | 采购订单明细表 |
| `pd_purchase_receipt` | 采购收货表 |
| `pd_purchase_receipt_item` | 采购收货明细表 |
| `pd_purchase_return` | 采购退货表 |
| `pd_purchase_return_item` | 采购退货明细表 |

### 3.5 库存管理表

| 表名 | 说明 |
|---|---|
| `inv_stock` | 库存表 |
| `inv_stock_transaction` | 库存流水表 |
| `inv_stock_count` | 盘点表 |
| `inv_stock_count_item` | 盘点明细表 |
| `inv_stock_transfer` | 调拨表 |
| `inv_stock_transfer_item` | 调拨明细表 |

### 3.6 财务管理表

| 表名 | 说明 |
|---|---|
| `fin_receivable` | 应收表 |
| `fin_receivable_item` | 应收明细表 |
| `fin_payable` | 应付表 |
| `fin_payable_item` | 应付明细表 |
| `fin_receipt` | 收款表 |
| `fin_receipt_item` | 收款明细表 |
| `fin_payment` | 付款表 |
| `fin_payment_item` | 付款明细表 |
| `fin_invoice` | 发票表 |
| `fin_invoice_item` | 发票明细表 |
| `fin_voucher` | 凭证表 |
| `fin_voucher_item` | 凭证分录表 |
| `fin_account` | 会计科目表 |
| `fin_period` | 会计期间表 |

### 3.7 报表表

| 表名 | 说明 |
|---|---|
| `rpt_sales_summary` | 销售汇总表 |
| `rpt_purchase_summary` | 采购汇总表 |
| `rpt_stock_summary` | 库存汇总表 |
| `rpt_finance_report` | 财务报表 |

## 四、后端 API 接口清单

### 4.1 认证接口

- `POST /api/auth/login`
- `POST /api/auth/logout`
- `POST /api/auth/refresh`
- `GET /api/auth/userinfo`
- `PUT /api/auth/password`

### 4.2 系统管理接口

- `GET/POST /api/system/users`
- `GET/PUT/DELETE /api/system/users/{id}`
- `PUT /api/system/users/{id}/status`
- `PUT /api/system/users/{id}/password`
- `GET/POST /api/system/roles`
- `GET/PUT/DELETE /api/system/roles/{id}`
- `GET/POST /api/system/roles/{id}/permissions`
- `GET /api/system/permissions`
- `GET/POST /api/system/organizations`
- `GET/PUT/DELETE /api/system/organizations/{id}`
- `GET/POST /api/system/dictionaries`
- `GET/PUT/DELETE /api/system/dictionaries/{id}`
- `GET /api/system/logs/operation`
- `GET /api/system/logs/login`

### 4.3 主数据接口

- `GET/POST /api/masterdata/customers`
- `GET/PUT/DELETE /api/masterdata/customers/{id}`
- `GET/POST /api/masterdata/suppliers`
- `GET/PUT/DELETE /api/masterdata/suppliers/{id}`
- `GET/POST /api/masterdata/materials`
- `GET/PUT/DELETE /api/masterdata/materials/{id}`
- `GET/POST /api/masterdata/warehouses`
- `GET/PUT/DELETE /api/masterdata/warehouses/{id}`
- `GET/POST /api/masterdata/units`
- `GET/PUT/DELETE /api/masterdata/units/{id}`
- `GET/POST /api/masterdata/currencies`
- `GET/PUT/DELETE /api/masterdata/currencies/{id}`
- `GET/POST /api/masterdata/tax-rates`
- `GET/PUT/DELETE /api/masterdata/tax-rates/{id}`

### 4.4 销售管理接口

- `GET/POST /api/sales/quotations`
- `GET/PUT/DELETE /api/sales/quotations/{id}`
- `POST /api/sales/quotations/{id}/convert`
- `GET/POST /api/sales/orders`
- `GET/PUT/DELETE /api/sales/orders/{id}`
- `POST /api/sales/orders/{id}/approve`
- `POST /api/sales/orders/{id}/close`
- `POST /api/sales/orders/{id}/deliver`
- `GET/POST /api/sales/deliveries`
- `GET/PUT/DELETE /api/sales/deliveries/{id}`
- `POST /api/sales/deliveries/{id}/confirm`
- `POST /api/sales/deliveries/{id}/invoice`
- `GET/POST /api/sales/returns`
- `GET/PUT/DELETE /api/sales/returns/{id}`
- `POST /api/sales/returns/{id}/confirm`

### 4.5 采购管理接口

- `GET/POST /api/purchase/requests`
- `GET/PUT/DELETE /api/purchase/requests/{id}`
- `POST /api/purchase/requests/{id}/approve`
- `POST /api/purchase/requests/{id}/convert`
- `GET/POST /api/purchase/orders`
- `GET/PUT/DELETE /api/purchase/orders/{id}`
- `POST /api/purchase/orders/{id}/approve`
- `POST /api/purchase/orders/{id}/close`
- `POST /api/purchase/orders/{id}/receive`
- `GET/POST /api/purchase/receipts`
- `GET/PUT/DELETE /api/purchase/receipts/{id}`
- `POST /api/purchase/receipts/{id}/confirm`
- `POST /api/purchase/receipts/{id}/invoice`
- `GET/POST /api/purchase/returns`
- `GET/PUT/DELETE /api/purchase/returns/{id}`
- `POST /api/purchase/returns/{id}/confirm`

### 4.6 库存管理接口

- `GET /api/inventory/stocks`
- `GET /api/inventory/transactions`
- `GET/POST /api/inventory/counts`
- `GET/PUT/DELETE /api/inventory/counts/{id}`
- `POST /api/inventory/counts/{id}/confirm`
- `GET/POST /api/inventory/transfers`
- `GET/PUT/DELETE /api/inventory/transfers/{id}`
- `POST /api/inventory/transfers/{id}/confirm`
- `GET /api/inventory/alerts`

### 4.7 财务管理接口

- `GET /api/finance/receivables`
- `GET /api/finance/receivables/{id}`
- `GET /api/finance/payables`
- `GET /api/finance/payables/{id}`
- `GET/POST /api/finance/receipts`
- `GET/PUT/DELETE /api/finance/receipts/{id}`
- `POST /api/finance/receipts/{id}/allocate`
- `GET/POST /api/finance/payments`
- `GET/PUT/DELETE /api/finance/payments/{id}`
- `POST /api/finance/payments/{id}/allocate`
- `GET/POST /api/finance/invoices`
- `GET/PUT/DELETE /api/finance/invoices/{id}`
- `GET/POST /api/finance/vouchers`
- `GET/PUT/DELETE /api/finance/vouchers/{id}`
- `POST /api/finance/vouchers/{id}/approve`
- `GET/POST /api/finance/accounts`
- `GET/PUT/DELETE /api/finance/accounts/{id}`
- `GET/POST /api/finance/periods`
- `POST /api/finance/periods/{id}/close`
- `POST /api/finance/periods/{id}/reopen`

### 4.8 报表接口

- `GET /api/reports/sales/summary`
- `GET /api/reports/sales/detail`
- `GET /api/reports/sales/profit`
- `GET /api/reports/purchase/summary`
- `GET /api/reports/purchase/detail`
- `GET /api/reports/inventory/summary`
- `GET /api/reports/inventory/detail`
- `GET /api/reports/inventory/transactions`
- `GET /api/reports/finance/balance-sheet`
- `GET /api/reports/finance/income-statement`
- `GET /api/reports/finance/cash-flow`

## 五、前端页面清单

### 5.1 系统管理页面

- 用户管理 `/system/users`
- 角色管理 `/system/roles`
- 权限管理 `/system/permissions`
- 组织管理 `/system/organizations`
- 字典管理 `/system/dictionaries`
- 操作日志 `/system/logs`
- 系统配置 `/system/config`

### 5.2 主数据页面

- 客户管理 `/masterdata/customers`
- 供应商管理 `/masterdata/suppliers`
- 物料管理 `/masterdata/materials`
- 仓库管理 `/masterdata/warehouses`
- 计量单位 `/masterdata/units`
- 币种管理 `/masterdata/currencies`
- 税率管理 `/masterdata/tax-rates`

### 5.3 销售管理页面

- 报价单 `/sales/quotations`
- 销售订单 `/sales/orders`
- 销售发货 `/sales/deliveries`
- 销售退货 `/sales/returns`
- 价格管理 `/sales/prices`

### 5.4 采购管理页面

- 采购申请 `/purchase/requests`
- 采购订单 `/purchase/orders`
- 采购收货 `/purchase/receipts`
- 采购退货 `/purchase/returns`

### 5.5 库存管理页面

- 库存查询 `/inventory/stocks`
- 库存流水 `/inventory/transactions`
- 库存盘点 `/inventory/counts`
- 库存调拨 `/inventory/transfers`
- 库存预警 `/inventory/alerts`

### 5.6 财务管理页面

- 应收管理 `/finance/receivables`
- 应付管理 `/finance/payables`
- 收款管理 `/finance/receipts`
- 付款管理 `/finance/payments`
- 发票管理 `/finance/invoices`
- 凭证管理 `/finance/vouchers`
- 科目管理 `/finance/accounts`
- 期间管理 `/finance/periods`

### 5.7 报表中心页面

- 销售报表 `/reports/sales`
- 采购报表 `/reports/purchase`
- 库存报表 `/reports/inventory`
- 财务报表 `/reports/finance`

## 六、已发现的问题与风险

### 6.1 财务核算完整性

- 缺少完整的复式记账总账体系，目前凭证与业务单据的关联不够紧密。
- 缺少成本核算模块，无法计算销售成本和库存成本。
- 缺少多币种汇兑损益处理，外币业务无法自动计算汇兑损益。
- 缺少期末结转功能，损益类科目无法自动结转。
- 应收应付与总账的勾稽关系不完整。

### 6.2 权限控制

- 数据权限粒度较粗，目前仅支持全部/部门/个人三种。
- 缺少字段级权限控制。
- 按钮权限和 API 权限的对应关系不够清晰。
- 权限变更后前端菜单未实时刷新。

### 6.3 库存成本

- 库存成本采用移动加权平均法，但成本计算逻辑不完整。
- 缺少批次管理和序列号管理。
- 缺少库龄分析。
- 库存调拨、盘点产生的成本差异处理不完整。

### 6.4 业务流程

- 缺少审批流引擎，目前仅支持简单的状态流转。
- 单据之间的关联关系不够完整，部分单据无法追溯来源。
- 缺少反核销、红冲等逆向业务流程。
- 缺少业务预警和消息通知。

### 6.5 前端体验

- 部分表单缺少前端校验，错误提示不够友好。
- 部分页面缺少加载状态和空状态。
- 错误处理不统一，部分接口错误直接暴露给用户。
- 缺少国际化支持。
- 部分页面表格列过多，缺少列配置功能。

### 6.6 性能

- 部分列表查询存在 N+1 查询问题。
- 缺少数据库索引优化。
- 大数据量下分页查询性能较差。
- 缺少缓存机制。

### 6.7 测试

- 单元测试覆盖率低。
- 集成测试不完整。
- 缺少前端自动化测试。
- 缺少性能测试。

### 6.8 文档

- API 文档不够完整。
- 缺少部署文档和运维文档。
- 缺少用户操作手册。

## 七、后续开发建议

### 7.1 P0 优先级（1–2 个月）

1. **完善财务核算体系**
   - 建立完整的复式记账总账。
   - 实现业务单据自动生成凭证。
   - 完善应收应付与总账勾稽。
   - 实现期末结转。
2. **实现成本核算**
   - 移动加权平均成本计算。
   - 销售成本结转。
   - 库存成本调整。
3. **完善多币种处理**
   - 汇兑损益计算。
   - 外币报表折算。
   - 汇率维护。

### 7.2 P1 优先级（2–4 个月）

1. **权限精细化**
   - 字段级权限。
   - 数据权限扩展。
   - 权限实时刷新。
2. **审批流引擎**
   - 可配置审批流程。
   - 审批节点、审批人、审批条件。
   - 审批历史。
3. **库存管理增强**
   - 批次管理。
   - 序列号管理。
   - 库龄分析。
   - 库存预警。

### 7.3 P2 优先级（4–6 个月）

1. **报表中心增强**
   - 更多报表。
   - 报表自定义。
   - 图表展示。
2. **性能优化**
   - 查询优化。
   - 索引优化。
   - 缓存。
3. **测试与文档**
   - 单元测试。
   - 集成测试。
   - API 文档。
   - 用户手册。

### 7.4 技术债务

- 统一异常处理。
- 统一日志。
- 代码规范。
- 依赖升级。

## 八、结论

水镜零售 ERP 系统是一个功能较为完整的贸易/分销 ERP 系统，涵盖了从销售、采购、库存到财务核算的主要业务流程。系统采用现代化的技术栈，代码结构清晰，模块划分合理。但仍存在一些需要改进的地方，特别是在财务核算的完整性、权限控制的精细度、以及前端用户体验方面。

建议按照上述后续开发建议，分阶段推进系统的完善和升级。
