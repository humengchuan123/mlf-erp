import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { public: true, title: '登录' }
  },
  {
    path: '/403',
    name: 'forbidden',
    component: () => import('@/views/ForbiddenView.vue'),
    meta: { public: true, title: '无权限' }
  },
  {
    path: '/',
    component: () => import('@/layouts/DefaultLayout.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'dashboard',
        component: () => import('@/views/DashboardView.vue'),
        meta: { title: '工作台' }
      },
      {
        path: 'purchase/orders',
        name: 'purchase-orders',
        component: () => import('@/views/purchase/OrderListView.vue'),
        meta: { title: '采购订单', permission: 'purchase.order.read' }
      },
      {
        path: 'purchase/receipts',
        name: 'purchase-receipts',
        component: () => import('@/views/purchase/PurchaseReceiptListView.vue'),
        meta: { title: '采购收货', permission: 'purchase.receipt.read' }
      },
      {
        path: 'purchase/returns',
        name: 'purchase-returns',
        component: () => import('@/views/purchase/PurchaseReturnListView.vue'),
        meta: { title: '采购退货', permission: 'purchase.return.read' }
      },
      {
        path: 'sales/orders',
        name: 'sales-orders',
        component: () => import('@/views/sales/OrderListView.vue'),
        meta: { title: '销售订单', permission: 'sales.order.read' }
      },
      {
        path: 'sales/shipments',
        name: 'sales-shipments',
        component: () => import('@/views/sales/ShipmentListView.vue'),
        meta: { title: '发货单', permission: 'sales.shipment.read' }
      },
      {
        path: 'sales/returns',
        name: 'sales-returns',
        component: () => import('@/views/sales/SalesReturnListView.vue'),
        meta: { title: '销售退货', permission: 'sales.return.read' }
      },
      {
        path: 'inventory/balances',
        name: 'inventory-balances',
        component: () => import('@/views/inventory/BalanceListView.vue'),
        meta: { title: '库存余额', permission: 'inventory.balance.read' }
      },
      {
        path: 'inventory/transactions',
        name: 'inventory-transactions',
        component: () => import('@/views/inventory/TransactionListView.vue'),
        meta: { title: '库存流水', permission: 'inventory.transaction.read' }
      },
      {
        path: 'finance/invoices',
        name: 'finance-invoices',
        component: () => import('@/views/finance/InvoiceListView.vue'),
        meta: { title: '应收应付', permission: 'finance.invoice.read' }
      },
      {
        path: 'finance/credit-notes',
        name: 'finance-credit-notes',
        component: () => import('@/views/finance/CreditNoteListView.vue'),
        meta: { title: '贷项单', permission: 'finance.invoice.read' }
      },
      {
        path: 'finance/payments',
        name: 'finance-payments',
        component: () => import('@/views/finance/PaymentListView.vue'),
        meta: { title: '收付款', permission: 'finance.payment.read' }
      },
      {
        path: 'finance/accounts',
        name: 'finance-accounts',
        component: () => import('@/views/finance/AccountListView.vue'),
        meta: { title: '会计科目', permission: 'finance.account.read' }
      },
      {
        path: 'finance/periods',
        name: 'finance-periods',
        component: () => import('@/views/finance/PeriodListView.vue'),
        meta: { title: '会计期间', permission: 'finance.period.read' }
      },
      {
        path: 'finance/vouchers',
        name: 'finance-vouchers',
        component: () => import('@/views/finance/VoucherListView.vue'),
        meta: { title: '会计凭证', permission: 'finance.voucher.read' }
      },
      {
        path: 'finance/reports',
        name: 'finance-reports',
        component: () => import('@/views/finance/ReportView.vue'),
        meta: { title: '财务报表', permission: 'finance.report.read' }
      },
      {
        path: 'finance/exchange-rates',
        name: 'finance-exchange-rates',
        component: () => import('@/views/finance/ExchangeRateListView.vue'),
        meta: { title: '汇率管理', permission: 'finance.exchange_rate.read' }
      },
      {
        path: 'finance/gl',
        redirect: '/finance/vouchers'
      },
      {
        path: 'masterdata/partners',
        name: 'masterdata-partners',
        component: () => import('@/views/masterdata/PartnerListView.vue'),
        meta: { title: '客商管理', permission: 'masterdata.partner.read' }
      },
      {
        path: 'masterdata/materials',
        name: 'masterdata-materials',
        component: () => import('@/views/masterdata/MaterialListView.vue'),
        meta: { title: '物料管理', permission: 'masterdata.material.read' }
      },
      {
        path: 'masterdata/warehouses',
        name: 'masterdata-warehouses',
        component: () => import('@/views/masterdata/WarehouseListView.vue'),
        meta: { title: '仓库管理', permission: 'masterdata.warehouse.read' }
      },
      {
        path: 'masterdata/units',
        name: 'masterdata-units',
        component: () => import('@/views/masterdata/UnitListView.vue'),
        meta: { title: '计量单位', permission: 'masterdata.unit.read' }
      },
      {
        path: 'masterdata/currencies',
        name: 'masterdata-currencies',
        component: () => import('@/views/masterdata/CurrencyListView.vue'),
        meta: { title: '币种管理', permission: 'masterdata.currency.read' }
      },
      {
        path: 'masterdata/tax-rates',
        name: 'masterdata-tax-rates',
        component: () => import('@/views/masterdata/TaxRateListView.vue'),
        meta: { title: '税率管理', permission: 'masterdata.tax.read' }
      },
      {
        path: 'system/code-rules',
        name: 'system-code-rules',
        component: () => import('@/views/system/CodeRuleView.vue'),
        meta: { title: '编码规则', permission: 'system.code_rule.read' }
      },
      {
        path: 'system/users',
        name: 'system-users',
        component: () => import('@/views/system/UserListView.vue'),
        meta: { title: '用户管理', permission: 'system.user.read' }
      },
      {
        path: 'system/roles',
        name: 'system-roles',
        component: () => import('@/views/system/RoleListView.vue'),
        meta: { title: '角色管理', permission: 'system.role.read' }
      },
      {
        path: 'system/orgs',
        name: 'system-orgs',
        component: () => import('@/views/system/OrgListView.vue'),
        meta: { title: '组织管理', permission: 'system.org.read' }
      },
      {
        path: 'system/audit-logs',
        name: 'system-audit-logs',
        component: () => import('@/views/system/AuditLogView.vue'),
        meta: { title: '审计日志', permission: 'system.audit.read' }
      },
      {
        path: 'system/approval-flows',
        name: 'system-approval-flows',
        component: () => import('@/views/system/ApprovalFlowListView.vue'),
        meta: { title: '审批流程', permission: 'system.approval.read' }
      },
      {
        path: 'system/pending-approvals',
        name: 'system-pending-approvals',
        component: () => import('@/views/system/PendingApprovalView.vue'),
        meta: { title: '待我审批', permission: 'system.approval.read' }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to) => {
  const authStore = useAuthStore()

  if (authStore.hasToken && !authStore.user) {
    try {
      await authStore.loadMe()
    } catch {
      authStore.clear()
    }
  }

  if (to.meta.public !== true && !authStore.hasToken) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  if (to.name === 'login' && authStore.hasToken) {
    return { name: 'dashboard' }
  }

  const permission = to.meta.permission as string | undefined
  if (permission && !authStore.hasPermission(permission)) {
    return { name: 'forbidden' }
  }

  return true
})

export default router
