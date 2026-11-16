<template>
  <div class="forbidden">
    <el-result icon="warning" title="403" sub-title="你没有权限访问该页面">
      <template #extra>
        <el-button type="primary" @click="router.push('/')">返回工作台</el-button>
        <el-button @click="relogin">重新登录</el-button>
      </template>
    </el-result>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

async function relogin() {
  await authStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.forbidden {
  display: grid;
  min-height: 100%;
  place-items: center;
  background: #f5f7fa;
}
</style>
