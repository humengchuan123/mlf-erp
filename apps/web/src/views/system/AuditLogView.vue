<template>
  <div>
    <PageHeader title="审计日志" description="查看用户、角色、组织等关键操作记录。" />

    <el-card shadow="never">
      <el-table v-loading="loading" :data="logs" border stripe>
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="audit-detail">
              <div>
                <h4>变更前</h4>
                <pre>{{ pretty(row.before_data) }}</pre>
              </div>
              <div>
                <h4>变更后</h4>
                <pre>{{ pretty(row.after_data) }}</pre>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" min-width="170">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="action" label="动作" min-width="150" />
        <el-table-column prop="resource_type" label="资源类型" width="110" />
        <el-table-column prop="resource_id" label="资源 ID" min-width="160" show-overflow-tooltip />
        <el-table-column prop="user_id" label="用户 ID" min-width="160" show-overflow-tooltip />
        <el-table-column prop="request_id" label="请求 ID" min-width="160" show-overflow-tooltip />
        <el-table-column prop="ip" label="IP" width="130" />
      </el-table>

      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        class="pagination"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        @current-change="loadLogs"
        @size-change="handleSizeChange"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import { systemApi, type AuditLog } from '@mlf-erp/sdk'

const logs = ref<AuditLog[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

async function loadLogs() {
  loading.value = true
  try {
    const result = await systemApi.auditLogs.list({
      page: page.value,
      page_size: pageSize.value
    })
    logs.value = result.items
    total.value = result.total
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载审计日志失败')
  } finally {
    loading.value = false
  }
}

function handleSizeChange() {
  page.value = 1
  loadLogs()
}

function formatDate(value: string) {
  return value ? new Date(value).toLocaleString() : '-'
}

function pretty(value: unknown) {
  if (value === null || value === undefined) {
    return '-'
  }
  return JSON.stringify(value, null, 2)
}

onMounted(loadLogs)
</script>

<style scoped>
.pagination {
  margin-top: 16px;
  justify-content: flex-end;
}

.audit-detail {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  padding: 8px 24px;
}

.audit-detail h4 {
  margin: 0 0 8px;
  color: #475569;
}

.audit-detail pre {
  max-height: 240px;
  margin: 0;
  overflow: auto;
  padding: 12px;
  border-radius: 8px;
  background: #0f172a;
  color: #cbd5e1;
  font-size: 12px;
  line-height: 1.6;
}
</style>
