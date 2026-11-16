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
        path: 'sales/orders',
        name: 'sales-orders',
        component: () => import('@/views/PlaceholderView.vue'),
        meta: {
          title: '销售订单',
          description: '销售模块骨架，待接入报价、订单、发货与收款。',
          permission: 'sales.read'
        }
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
        path: 'finance/gl',
        name: 'finance-gl',
        component: () => import('@/views/PlaceholderView.vue'),
        meta: {
          title: '总账',
          description: '财务模块骨架，待接入科目、凭证、期间与报表。',
          permission: 'finance.read'
        }
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
