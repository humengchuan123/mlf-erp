<template>
  <div class="login">
    <el-card class="login__card" shadow="always">
      <div class="login__brand">
        <div class="login__mark">M</div>
        <div>
          <h1>MLF ERP</h1>
          <p>企业资源计划系统</p>
        </div>
      </div>

      <el-form :model="form" label-position="top">
        <el-form-item label="用户名">
          <el-input v-model="form.username" placeholder="请输入用户名" size="large" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="开发模式任意密码"
            size="large"
            show-password
          />
        </el-form-item>
        <el-button type="primary" size="large" class="login__submit" @click="submit">
          登录
        </el-button>
      </el-form>

      <p class="login__hint">当前为开发骨架，任意用户名和密码均可登录。</p>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAppStore } from '@/stores/app'

const router = useRouter()
const appStore = useAppStore()
const form = reactive({ username: 'admin', password: 'admin' })

function submit() {
  if (!form.username.trim()) {
    ElMessage.warning('请输入用户名')
    return
  }

  appStore.login(form.username.trim())
  router.push('/')
}
</script>

<style scoped>
.login {
  display: grid;
  min-height: 100%;
  place-items: center;
  background: radial-gradient(circle at top left, #1d4ed8 0, #0f172a 42%, #020617 100%);
}

.login__card {
  width: 380px;
  border-radius: 14px;
}

.login__brand {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
}

.login__mark {
  display: grid;
  place-items: center;
  width: 42px;
  height: 42px;
  border-radius: 10px;
  background: #1d4ed8;
  color: #ffffff;
  font-size: 20px;
  font-weight: 700;
}

.login__brand h1 {
  margin: 0;
  font-size: 20px;
}

.login__brand p {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 12px;
}

.login__submit {
  width: 100%;
}

.login__hint {
  margin: 16px 0 0;
  color: #94a3b8;
  font-size: 12px;
  text-align: center;
}
</style>
