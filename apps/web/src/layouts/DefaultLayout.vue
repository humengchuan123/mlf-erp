<template>
  <el-container class="layout">
    <el-aside width="220px" class="layout__aside">
      <div class="layout__brand">
        <span class="layout__brand-mark">M</span>
        <div>
          <strong>{{ appStore.appName }}</strong>
          <small>Enterprise ERP</small>
        </div>
      </div>

      <el-menu :default-active="route.path" router class="layout__menu">
        <el-menu-item index="/dashboard">
          <el-icon><DataBoard /></el-icon>
          <span>工作台</span>
        </el-menu-item>

        <el-menu-item-group v-if="canSupplyChain" title="供应链">
          <el-menu-item v-if="authStore.hasPermission('purchase.order.read')" index="/purchase/orders">
            <el-icon><ShoppingCart /></el-icon>
            <span>采购订单</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('sales.order.read')" index="/sales/orders">
            <el-icon><Sell /></el-icon>
            <span>销售订单</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('sales.shipment.read')" index="/sales/shipments">
            <el-icon><Van /></el-icon>
            <span>发货单</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('inventory.balance.read')" index="/inventory/balances">
            <el-icon><Box /></el-icon>
            <span>库存余额</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('inventory.transaction.read')" index="/inventory/transactions">
            <el-icon><List /></el-icon>
            <span>库存流水</span>
          </el-menu-item>
        </el-menu-item-group>

        <el-menu-item-group v-if="canMasterData" title="主数据">
          <el-menu-item v-if="authStore.hasPermission('masterdata.partner.read')" index="/masterdata/partners">
            <el-icon><UserFilled /></el-icon>
            <span>客商管理</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('masterdata.material.read')" index="/masterdata/materials">
            <el-icon><Goods /></el-icon>
            <span>物料管理</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('masterdata.warehouse.read')" index="/masterdata/warehouses">
            <el-icon><House /></el-icon>
            <span>仓库管理</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('masterdata.unit.read')" index="/masterdata/units">
            <el-icon><ScaleToOriginal /></el-icon>
            <span>计量单位</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('masterdata.currency.read')" index="/masterdata/currencies">
            <el-icon><Coin /></el-icon>
            <span>币种管理</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('masterdata.tax.read')" index="/masterdata/tax-rates">
            <el-icon><Discount /></el-icon>
            <span>税率管理</span>
          </el-menu-item>
        </el-menu-item-group>

        <el-menu-item-group v-if="canFinance" title="财务">
          <el-menu-item v-if="authStore.hasPermission('finance.invoice.read')" index="/finance/invoices">
            <el-icon><Tickets /></el-icon>
            <span>应收应付</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('finance.payment.read')" index="/finance/payments">
            <el-icon><Wallet /></el-icon>
            <span>收付款</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('finance.account.read')" index="/finance/accounts">
            <el-icon><Collection /></el-icon>
            <span>会计科目</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('finance.period.read')" index="/finance/periods">
            <el-icon><Calendar /></el-icon>
            <span>会计期间</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('finance.voucher.read')" index="/finance/vouchers">
            <el-icon><Document /></el-icon>
            <span>会计凭证</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('finance.report.read')" index="/finance/reports">
            <el-icon><TrendCharts /></el-icon>
            <span>财务报表</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('finance.exchange_rate.read')" index="/finance/exchange-rates">
            <el-icon><Coin /></el-icon>
            <span>汇率管理</span>
          </el-menu-item>
        </el-menu-item-group>

        <el-menu-item-group v-if="canSystem" title="平台">
          <el-menu-item v-if="authStore.hasPermission('system.user.read')" index="/system/users">
            <el-icon><User /></el-icon>
            <span>用户管理</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('system.role.read')" index="/system/roles">
            <el-icon><Key /></el-icon>
            <span>角色权限</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('system.org.read')" index="/system/orgs">
            <el-icon><OfficeBuilding /></el-icon>
            <span>组织管理</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('system.audit.read')" index="/system/audit-logs">
            <el-icon><Memo /></el-icon>
            <span>审计日志</span>
          </el-menu-item>
          <el-menu-item v-if="authStore.hasPermission('system.code_rule.read')" index="/system/code-rules">
            <el-icon><SetUp /></el-icon>
            <span>编码规则</span>
          </el-menu-item>
        </el-menu-item-group>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="layout__header">
        <div class="layout__header-left">
          <span class="layout__breadcrumb">{{ currentTitle }}</span>
        </div>
        <div class="layout__header-right">
          <el-tag size="small" effect="plain">{{ appStore.deliveryModeLabel }}</el-tag>
          <el-dropdown @command="handleCommand">
            <span class="layout__user">
              {{ displayName }}
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="layout__main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowDown,
  Box,
  Calendar,
  Coin,
  Collection,
  DataBoard,
  Discount,
  Document,
  Goods,
  House,
  Key,
  List,
  Memo,
  OfficeBuilding,
  ScaleToOriginal,
  Sell,
  SetUp,
  ShoppingCart,
  Tickets,
  TrendCharts,
  User,
  UserFilled,
  Van,
  Wallet
} from '@element-plus/icons-vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const currentTitle = computed(() => (route.meta.title as string) ?? '工作台')
const displayName = computed(() => authStore.user?.display_name || authStore.user?.username || '未登录')
const canSupplyChain = computed(
  () =>
    authStore.hasPermission('purchase.order.read') ||
    authStore.hasPermission('sales.order.read') ||
    authStore.hasPermission('sales.shipment.read') ||
    authStore.hasPermission('inventory.balance.read') ||
    authStore.hasPermission('inventory.transaction.read')
)
const canMasterData = computed(
  () =>
    authStore.hasPermission('masterdata.partner.read') ||
    authStore.hasPermission('masterdata.material.read') ||
    authStore.hasPermission('masterdata.warehouse.read') ||
    authStore.hasPermission('masterdata.unit.read') ||
    authStore.hasPermission('masterdata.currency.read') ||
    authStore.hasPermission('masterdata.tax.read')
)
const canFinance = computed(
  () =>
    authStore.hasPermission('finance.invoice.read') ||
    authStore.hasPermission('finance.payment.read') ||
    authStore.hasPermission('finance.account.read') ||
    authStore.hasPermission('finance.period.read') ||
    authStore.hasPermission('finance.voucher.read') ||
    authStore.hasPermission('finance.report.read') ||
    authStore.hasPermission('finance.exchange_rate.read') ||
    authStore.hasPermission('finance.read')
)
const canSystem = computed(
  () =>
    authStore.hasPermission('system.user.read') ||
    authStore.hasPermission('system.role.read') ||
    authStore.hasPermission('system.org.read') ||
    authStore.hasPermission('system.audit.read') ||
    authStore.hasPermission('system.code_rule.read')
)

async function handleCommand(command: string) {
  if (command === 'logout') {
    await authStore.logout()
    router.push('/login')
  }
}
</script>

<style scoped>
.layout {
  height: 100%;
}

.layout__aside {
  display: flex;
  flex-direction: column;
  background: #0f172a;
  color: #fff;
}

.layout__brand {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 64px;
  padding: 0 18px;
}

.layout__brand strong {
  display: block;
  font-size: 15px;
}

.layout__brand small {
  display: block;
  margin-top: 2px;
  color: #94a3b8;
  font-size: 11px;
}

.layout__brand-mark {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: #2563eb;
  font-size: 16px;
  font-weight: 700;
}

.layout__menu {
  flex: 1;
  border-right: none;
  background: transparent;
  --el-menu-bg-color: transparent;
  --el-menu-text-color: #cbd5e1;
  --el-menu-hover-bg-color: rgba(148, 163, 184, 0.12);
  --el-menu-active-color: #ffffff;
}

.layout__menu :deep(.el-menu-item-group__title) {
  padding-left: 20px;
  color: #64748b;
  font-size: 12px;
}

.layout__menu :deep(.el-menu-item.is-active) {
  background: #1d4ed8;
  color: #ffffff;
}

.layout__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 56px;
  border-bottom: 1px solid #e5e7eb;
  background: #ffffff;
}

.layout__breadcrumb {
  color: #475569;
  font-size: 14px;
}

.layout__header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.layout__user {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: #334155;
  cursor: pointer;
}

.layout__main {
  padding: 20px;
  background: #f5f7fa;
}
</style>
