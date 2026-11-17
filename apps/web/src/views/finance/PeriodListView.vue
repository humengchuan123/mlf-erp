<template>
  <div>
    <PageHeader title="会计期间" description="按月关账，关账后该期间禁止新增或过账凭证。">
      <el-select v-model="year" style="width: 120px" @change="load">
        <el-option v-for="item in years" :key="item" :label="`${item} 年`" :value="item" />
      </el-select>
    </PageHeader>

    <el-card shadow="never">
      <el-table v-loading="loading" :data="periods" border stripe>
        <el-table-column prop="year" label="年度" width="90" />
        <el-table-column prop="month" label="月份" width="90" />
        <el-table-column prop="start_date" label="开始日期" width="130" />
        <el-table-column prop="end_date" label="结束日期" width="130" />
        <el-table-column prop="status" label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="row.status === 'open' ? 'success' : 'info'" size="small">
              {{ row.status === 'open' ? '开放' : '已关账' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="closed_at" label="关账时间" min-width="180">
          <template #default="{ row }">{{ row.closed_at ? formatDate(row.closed_at) : '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="110" fixed="right">
          <template #default="{ row }">
            <el-button v-if="canClose && row.status === 'open'" link type="warning" @click="closePeriod(row)">
              关账
            </el-button>
            <el-button v-if="canClose && row.status === 'closed'" link type="primary" @click="reopenPeriod(row)">
              反关账
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import { glApi, type GlPeriod } from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const canClose = computed(() => authStore.hasPermission('finance.period.close'))

const periods = ref<GlPeriod[]>([])
const loading = ref(false)
const now = new Date()
const year = ref(now.getFullYear())
const years = [now.getFullYear() - 1, now.getFullYear(), now.getFullYear() + 1]

async function load() {
  loading.value = true
  try {
    periods.value = await glApi.periods.list(year.value)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载会计期间失败')
  } finally {
    loading.value = false
  }
}

async function closePeriod(period: GlPeriod) {
  try {
    await ElMessageBox.confirm(`确认关闭 ${period.year}-${String(period.month).padStart(2, '0')} 期间？`, '关账', {
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    await glApi.periods.close(period.id)
    ElMessage.success('期间已关账')
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '关账失败')
  }
}

async function reopenPeriod(period: GlPeriod) {
  try {
    await ElMessageBox.confirm(`确认重新打开 ${period.year}-${String(period.month).padStart(2, '0')} 期间？`, '反关账', {
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    await glApi.periods.reopen(period.id)
    ElMessage.success('期间已重新打开')
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '反关账失败')
  }
}

function formatDate(value: string) {
  return value ? new Date(value).toLocaleString() : '-'
}

onMounted(load)
</script>
