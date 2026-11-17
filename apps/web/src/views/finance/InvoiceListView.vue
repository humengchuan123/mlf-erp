<template>
  <div>
    <PageHeader title="应收应付" description="销售发货自动生成应收，采购收货自动生成应付；按发票查看核销和账龄。">
      <el-radio-group v-model="invoiceType" @change="search">
        <el-radio-button label="">全部</el-radio-button>
        <el-radio-button label="receivable">应收</el-radio-button>
        <el-radio-button label="payable">应付</el-radio-button>
      </el-radio-group>
    </PageHeader>

    <el-card shadow="never">
      <div class="toolbar">
        <el-input
          v-model="keyword"
          placeholder="搜索发票号、来源单号或往来单位"
          clearable
          class="toolbar__search"
          @keyup.enter="search"
        />
        <el-select v-model="statusFilter" placeholder="全部状态" clearable class="toolbar__status">
          <el-option label="未核销" value="open" />
          <el-option label="部分核销" value="partially_paid" />
          <el-option label="已核销" value="paid" />
        </el-select>
        <el-button type="primary" plain @click="search">查询</el-button>
        <el-button @click="resetSearch">重置</el-button>
      </div>

      <el-table v-loading="loading" :data="invoices" border stripe>
        <el-table-column prop="code" label="发票号" min-width="150" />
        <el-table-column prop="invoice_type" label="类型" width="90">
          <template #default="{ row }">
            <el-tag :type="row.invoice_type === 'receivable' ? 'success' : 'warning'" size="small">
              {{ row.invoice_type === 'receivable' ? '应收' : '应付' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="source_code" label="来源单号" min-width="150" />
        <el-table-column prop="partner_name" label="往来单位" min-width="150" />
        <el-table-column prop="invoice_date" label="发票日期" width="110" />
        <el-table-column prop="due_date" label="到期日" width="110">
          <template #default="{ row }">
            {{ row.due_date }}
            <el-tag v-if="isOverdue(row)" type="danger" size="small" effect="plain">逾期</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="currency" label="币种" width="70" />
        <el-table-column prop="total_amount" label="总额" width="120" align="right" />
        <el-table-column prop="paid_amount" label="已核销" width="120" align="right" />
        <el-table-column prop="outstanding_amount" label="未核销" width="120" align="right" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="invoiceStatusType(row.status)" size="small">{{ invoiceStatusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
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

    <el-drawer v-model="detailVisible" title="发票详情" size="700px">
      <template v-if="detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="发票号">{{ detail.code }}</el-descriptions-item>
          <el-descriptions-item label="类型">
            {{ detail.invoice_type === 'receivable' ? '应收' : '应付' }}
          </el-descriptions-item>
          <el-descriptions-item label="来源单号">{{ detail.source_code }}</el-descriptions-item>
          <el-descriptions-item label="往来单位">{{ detail.partner_name }}</el-descriptions-item>
          <el-descriptions-item label="发票日期">{{ detail.invoice_date }}</el-descriptions-item>
          <el-descriptions-item label="到期日">{{ detail.due_date }}</el-descriptions-item>
          <el-descriptions-item label="币种">{{ detail.currency }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ invoiceStatusLabel(detail.status) }}</el-descriptions-item>
          <el-descriptions-item label="总额">{{ detail.total_amount }}</el-descriptions-item>
          <el-descriptions-item label="已核销">{{ detail.paid_amount }}</el-descriptions-item>
          <el-descriptions-item label="未核销">{{ detail.outstanding_amount }}</el-descriptions-item>
          <el-descriptions-item label="备注">{{ detail.remark || '-' }}</el-descriptions-item>
        </el-descriptions>

        <el-divider content-position="left">核销记录</el-divider>
        <el-table :data="detail.allocations" border>
          <el-table-column prop="payment_code" label="收付款单号" min-width="160" />
          <el-table-column prop="payment_type" label="类型" width="90">
            <template #default="{ row }">{{ row.payment_type === 'receipt' ? '收款' : '付款' }}</template>
          </el-table-column>
          <el-table-column prop="payment_date" label="日期" width="110" />
          <el-table-column prop="amount" label="核销金额" width="130" align="right" />
        </el-table>
        <el-empty v-if="detail.allocations.length === 0" description="暂无核销记录" :image-size="80" />
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import { financeApi, type Invoice, type InvoiceDetail } from '@mlf-erp/sdk'

const invoices = ref<Invoice[]>([])
const loading = ref(false)
const invoiceType = ref('')
const statusFilter = ref('')
const keyword = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const detailVisible = ref(false)
const detail = ref<InvoiceDetail | null>(null)

async function load() {
  loading.value = true
  try {
    const result = await financeApi.invoices({
      invoice_type: invoiceType.value,
      status: statusFilter.value,
      keyword: keyword.value,
      page: page.value,
      page_size: pageSize.value
    })
    invoices.value = result.items
    total.value = result.total
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载应收应付失败')
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function resetSearch() {
  invoiceType.value = ''
  statusFilter.value = ''
  keyword.value = ''
  page.value = 1
  load()
}

function handleSizeChange() {
  page.value = 1
  load()
}

async function openDetail(row: Invoice) {
  try {
    detail.value = await financeApi.invoice(row.id)
    detailVisible.value = true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载发票详情失败')
  }
}

function invoiceStatusLabel(status: string) {
  if (status === 'paid') return '已核销'
  if (status === 'partially_paid') return '部分核销'
  return '未核销'
}

function invoiceStatusType(status: string) {
  if (status === 'paid') return 'success'
  if (status === 'partially_paid') return 'warning'
  return 'info'
}

function isOverdue(row: Invoice) {
  if (row.status === 'paid' || Number(row.outstanding_amount) <= 0) {
    return false
  }
  const today = new Date()
  const local = new Date(today.getTime() - today.getTimezoneOffset() * 60_000)
  return row.due_date < local.toISOString().slice(0, 10)
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
