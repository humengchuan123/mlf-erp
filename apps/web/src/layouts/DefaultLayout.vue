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

        <el-menu-item-group title="供应链">
          <el-menu-item index="/purchase/orders">
            <el-icon><ShoppingCart /></el-icon>
            <span>采购订单</span>
          </el-menu-item>
          <el-menu-item index="/sales/orders">
            <el-icon><Goods /></el-icon>
            <span>销售订单</span>
          </el-menu-item>
          <el-menu-item index="/inventory/stock">
            <el-icon><Box /></el-icon>
            <span>库存管理</span>
          </el-menu-item>
        </el-menu-item-group>

        <el-menu-item-group title="财务">
          <el-menu-item index="/finance/gl">
            <el-icon><Money /></el-icon>
            <span>总账</span>
          </el-menu-item>
        </el-menu-item-group>

        <el-menu-item-group title="平台">
          <el-menu-item index="/system/users">
            <el-icon><Setting /></el-icon>
            <span>用户与权限</span>
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
              {{ appStore.userName }}
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
  DataBoard,
  Goods,
  Money,
  Setting,
  ShoppingCart
} from '@element-plus/icons-vue'
import { useAppStore } from '@/stores/app'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const currentTitle = computed(() => (route.meta.title as string) ?? '工作台')

function handleCommand(command: string) {
  if (command === 'logout') {
    appStore.logout()
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
