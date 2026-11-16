import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAppStore } from '@/stores/app'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { public: true, title: '登录' }
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
        meta: { title: '采购订单' }
      },
      {
        path: 'sales/orders',
        name: 'sales-orders',
        component: () => import('@/views/PlaceholderView.vue'),
        meta: { title: '销售订单', description: '销售模块骨架，待接入报价、订单、发货与收款。' }
      },
      {
        path: 'inventory/stock',
        name: 'inventory-stock',
        component: () => import('@/views/PlaceholderView.vue'),
        meta: { title: '库存管理', description: '库存模块骨架，待接入入库、出库、调拨与盘点。' }
      },
      {
        path: 'finance/gl',
        name: 'finance-gl',
        component: () => import('@/views/PlaceholderView.vue'),
        meta: { title: '总账', description: '财务模块骨架，待接入科目、凭证、期间与报表。' }
      },
      {
        path: 'system/users',
        name: 'system-users',
        component: () => import('@/views/PlaceholderView.vue'),
        meta: { title: '用户与权限', description: '平台底座骨架，待接入用户、角色、数据权限与审计。' }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to) => {
  const appStore = useAppStore()

  if (to.meta.public !== true && !appStore.token) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  if (to.name === 'login' && appStore.token) {
    return { name: 'dashboard' }
  }

  return true
})

export default router
