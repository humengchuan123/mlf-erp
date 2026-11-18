<template>
  <div>
    <PageHeader title="贷项单" description="查看销售退货生成的贷项单、核销进度和来源单据。">
      <el-button type="primary" plain @click="search">查询</el-button>
    </PageHeader>

    <el-card shadow="never">
      <div class="toolbar">
        <el-input
          v-model="keyword"
          placeholder="搜索贷项单号、发票号或往来单位"
          clearable
          class="toolbar__search"
          @keyup.enter="search"
        />
        <el-select v-model="statusFilter" placeholder="全部状态" clearable class="toolbar__status">
          <el-option label="未核销" value="open" />
          <el-option label="已核销" value="applied" />
        </el-select>
        <el-button @click="resetSearch">重置</el-button>
      </div>

      <el-table v-loading="loading" :data="creditNotes" border stripe>
        <el-table-column prop="code" label="贷项单号" min-width="150" />
        <el-table-column prop="partner_name" label="往来单位" min-width="150" />
        <el-table-column prop="invoice_code" label="原发票号" min-width="150">
          <template #default="{ row }">{{ row.invoice_code || '-' }}</template>
        </el-table-column>
        <el-table-column prop="note_date" label="日期" width="120" />
        <el-table-column prop="currency" label="币种" width="80" />
        <el-table-column prop="amount" label="贷项金额" width="130" align="right" />
        <el-table-column prop="applied_amount" label="已核销" width="130" align="right" />
        <el-table-column prop="remaining_amount" label="未核销" width="130" align="right" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openDetail(row)">详情</el-button>
          </template>
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

    <el-drawer v-model="detailVisible" title="贷项单详情" size="640px">
      <template v-if="detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="贷项单号">{{ detail.code }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="statusType(detail.status)" size="small">{{ statusLabel(detail.status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="来源退货单">{{ detail.source_code || '-' }}</el-descriptions-item>
          <el-descriptions-item label="来源类型">{{ sourceTypeLabel(detail.source_type) }}</el-descriptions-item>
          <el-descriptions-item label="原发票号">{{ detail.invoice_code || '-' }}</el-descriptions-item>
          <el-descriptions-item label="往来单位">{{ detail.partner_name }}</el-descriptions-item>
          <el-descriptions-item label="日期">{{ detail.note_date }}</el-descriptions-item>
          <el-descriptions-item label="币种/汇率">{{ detail.currency }} / {{ detail.exchange_rate }}</el-descriptions-item>
          <el-descriptions-item label="贷项金额">{{ detail.amount }}</el-descriptions-item>
          <el-descriptions-item label="本位币金额">{{ detail.base_amount }}</el-descriptions-item>
          <el-descriptions-item label="已核销金额">{{ detail.applied_amount }}</el-descriptions-item>
          <el-descriptions-item label="本位币已核销">{{ detail.applied_base_amount }}</el-descriptions-item>
          <el-descriptions-item label="未核销金额">{{ detail.remaining_amount }}</el-descriptions-item>
          <el-descriptions-item label="本位币未核销">{{ detail.remaining_base_amount }}</el-descriptions-item>
          <el-descriptions-item label="原因" :span="2">{{ detail.reason || '-' }}</el-descriptions-item>
          <el-descriptions-item label="备注" :span="2">{{ detail.remark || '-' }}</el-descriptions-item>
        </el-descriptions>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import { financeApi, type CreditNote } from '@mlf-erp/sdk'

const creditNotes = ref<CreditNote[]>([])
const loading = ref(false)
const keyword = ref('')
const statusFilter = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const detailVisible = ref(false)
const detail = ref<CreditNote | null>(null)

async function load() {
  loading.value = true
  try {
    const result = await financeApi.creditNotes.list({
      status: statusFilter.value,
      keyword: keyword.value,
      page: page.value,
      page_size: pageSize.value
    })
    creditNotes.value = result.items
    total.value = result.total
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载贷项单失败')
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
  statusFilter.value = ''
  page.value = 1
  load()
}

function handleSizeChange() {
  page.value = 1
  load()
}

async function openDetail(row: CreditNote) {
  try {
    detail.value = await financeApi.creditNotes.get(row.id)
    detailVisible.value = true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载贷项单详情失败')
  }
}

function statusLabel(status: string) {
  if (status === 'applied') return '已核销'
  if (status === 'open') return '未核销'
  return status
}

function statusType(status: string) {
  if (status === 'applied') return 'success'
  if (status === 'open') return 'warning'
  return 'info'
}

function sourceTypeLabel(value: string) {
  if (value === 'sales_return') return '销售退货'
  return value || '-'
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
  width: 320px;
}

.toolbar__status {
  width: 150px;
}

.pagination {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>
