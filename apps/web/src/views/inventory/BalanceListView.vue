<template>
  <div>
    <PageHeader title="库存余额" description="按仓库和物料查看当前在手、占用和可用数量。" />

    <el-card shadow="never">
      <div class="toolbar">
        <el-input
          v-model="keyword"
          placeholder="搜索仓库或物料"
          clearable
          class="toolbar__search"
          @keyup.enter="search"
        />
        <el-button type="primary" plain @click="search">查询</el-button>
        <el-button @click="resetSearch">重置</el-button>
      </div>

      <el-table v-loading="loading" :data="rows" border stripe>
        <el-table-column prop="warehouse_name" label="仓库" min-width="160" />
        <el-table-column prop="material_name" label="物料" min-width="200" />
        <el-table-column prop="on_hand" label="在手" width="120" align="right" />
        <el-table-column prop="reserved" label="占用" width="120" align="right" />
        <el-table-column prop="available" label="可用" width="120" align="right" />
        <el-table-column prop="updated_at" label="更新时间" min-width="180">
          <template #default="{ row }">{{ formatDate(row.updated_at) }}</template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        class="pagination"
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        @current-change="load"
        @size-change="handleSizeChange"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import { inventoryApi, type InventoryBalance } from '@mlf-erp/sdk'

const rows = ref<InventoryBalance[]>([])
const loading = ref(false)
const keyword = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

async function load() {
  loading.value = true
  try {
    const result = await inventoryApi.balances({
      keyword: keyword.value,
      page: page.value,
      page_size: pageSize.value
    })
    rows.value = result.items
    total.value = result.total
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载库存余额失败')
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function resetSearch() {
  keyword.value = ''
  page.value = 1
  load()
}

function handleSizeChange() {
  page.value = 1
  load()
}

function formatDate(value: string) {
  return value ? new Date(value).toLocaleString() : '-'
}

onMounted(load)
</script>

<style scoped>
.toolbar {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}

.toolbar__search {
  width: 300px;
}

.pagination {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>
