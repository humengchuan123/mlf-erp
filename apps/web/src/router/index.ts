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
        meta: { title: '采购订单', permission: 'purchase.read' }
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
        path: 'inventory/stock',
        name: 'inventory-stock',
        component: () => import('@/views/PlaceholderView.vue'),
        meta: {
          title: '库存管理',
          description: '库存模块骨架，待接入入库、出库、调拨与盘点。',
          permission: 'inventory.read'
        }
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
