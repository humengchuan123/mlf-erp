<template>
  <div>
    <PageHeader title="销售退货" description="查看销售退货、退回成本以及退货生成的贷项单。">
      <el-button type="primary" plain @click="search">查询</el-button>
    </PageHeader>

    <el-card shadow="never">
      <div class="toolbar">
        <el-input
          v-model="keyword"
          placeholder="搜索退货单号、发货单号或客户"
          clearable
          class="toolbar__search"
          @keyup.enter="search"
        />
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          value-format="YYYY-MM-DD"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
        />
        <el-button @click="resetSearch">重置</el-button>
      </div>

      <el-table v-loading="loading" :data="returns" border stripe>
        <el-table-column prop="code" label="退货单号" min-width="150" />
        <el-table-column prop="shipment_code" label="发货单号" min-width="150" />
        <el-table-column prop="customer_name" label="客户" min-width="150" />
        <el-table-column prop="return_date" label="退货日期" width="120" />
        <el-table-column prop="total_amount" label="退货金额" width="130" align="right" />
        <el-table-column prop="cost_amount" label="退货成本" width="130" align="right" />
        <el-table-column prop="credit_note_status" label="贷项单状态" width="120">
          <template #default="{ row }">
            <el-tag v-if="row.credit_note_status" :type="creditNoteStatusType(row.credit_note_status)" size="small">
              {{ creditNoteStatusLabel(row.credit_note_status) }}
            </el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="voucher_code" label="凭证号" min-width="140">
          <template #default="{ row }">{{ row.voucher_code || '-' }}</template>
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

    <el-drawer v-model="detailVisible" title="销售退货详情" size="720px">
      <template v-if="detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="退货单号">{{ detail.code }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="detail.status === 'posted' ? 'success' : 'info'" size="small">
              {{ detail.status === 'posted' ? '已过账' : detail.status }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="发货单号">{{ detail.shipment_code }}</el-descriptions-item>
          <el-descriptions-item label="销售订单">{{ detail.order_code }}</el-descriptions-item>
          <el-descriptions-item label="客户">{{ detail.customer_name }}</el-descriptions-item>
          <el-descriptions-item label="仓库">{{ detail.warehouse_name }}</el-descriptions-item>
          <el-descriptions-item label="退货日期">{{ detail.return_date }}</el-descriptions-item>
          <el-descriptions-item label="币种/汇率">{{ detail.currency }} / {{ detail.exchange_rate }}</el-descriptions-item>
          <el-descriptions-item label="价格口径">{{ detail.price_includes_tax ? '含税' : '不含税' }}</el-descriptions-item>
          <el-descriptions-item label="不含税金额">{{ detail.net_amount }}</el-descriptions-item>
          <el-descriptions-item label="税额">{{ detail.tax_amount }}</el-descriptions-item>
          <el-descriptions-item label="含税金额">{{ detail.total_amount }}</el-descriptions-item>
          <el-descriptions-item label="本位币含税金额">{{ detail.base_total_amount }}</el-descriptions-item>
          <el-descriptions-item label="退货成本">{{ detail.cost_amount }}</el-descriptions-item>
          <el-descriptions-item label="应收发票">{{ detail.invoice_code || '-' }}</el-descriptions-item>
          <el-descriptions-item label="凭证号">{{ detail.voucher_code || '-' }}</el-descriptions-item>
          <el-descriptions-item label="退货原因" :span="2">{{ detail.reason || '-' }}</el-descriptions-item>
          <el-descriptions-item label="备注" :span="2">{{ detail.remark || '-' }}</el-descriptions-item>
        </el-descriptions>

        <el-divider content-position="left">贷项单</el-divider>
        <el-descriptions :column="2" border>
          <el-descriptions-item label="贷项单号">{{ detail.credit_note_code || '-' }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag
              v-if="detail.credit_note_status"
              :type="creditNoteStatusType(detail.credit_note_status)"
              size="small"
            >
              {{ creditNoteStatusLabel(detail.credit_note_status) }}
            </el-tag>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item label="贷项金额">{{ detail.credit_note_amount }}</el-descriptions-item>
          <el-descriptions-item label="已核销金额">{{ detail.credit_note_applied_amount }}</el-descriptions-item>
          <el-descriptions-item label="未核销金额">{{ detail.credit_note_remaining_amount }}</el-descriptions-item>
          <el-descriptions-item label="原发票">{{ detail.invoice_code || '-' }}</el-descriptions-item>
        </el-descriptions>

        <el-divider content-position="left">退货明细</el-divider>
        <el-table :data="detail.lines" border>
          <el-table-column prop="line_no" label="#" width="60" />
          <el-table-column prop="material_name" label="物料" min-width="160" />
          <el-table-column prop="quantity" label="数量" width="90" align="right" />
          <el-table-column prop="unit_price" label="单价" width="100" align="right" />
          <el-table-column prop="tax_rate" label="税率" width="80" align="right" />
          <el-table-column prop="amount" label="不含税" width="110" align="right" />
          <el-table-column prop="tax_amount" label="税额" width="100" align="right" />
          <el-table-column prop="gross_amount" label="含税" width="110" align="right" />
          <el-table-column prop="unit_cost" label="单位成本" width="100" align="right" />
          <el-table-column prop="cost_amount" label="成本金额" width="110" align="right" />
        </el-table>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import { salesApi, type SalesReturn } from '@mlf-erp/sdk'

const returns = ref<SalesReturn[]>([])
const loading = ref(false)
const keyword = ref('')
const dateRange = ref<[string, string] | null>(null)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const detailVisible = ref(false)
const detail = ref<SalesReturn | null>(null)

async function load() {
  loading.value = true
  try {
    const result = await salesApi.returns.list({
      keyword: keyword.value,
      start_date: dateRange.value?.[0],
      end_date: dateRange.value?.[1],
      page: page.value,
      page_size: pageSize.value
    })
    returns.value = result.items
    total.value = result.total
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载销售退货失败')
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
  dateRange.value = null
  page.value = 1
  load()
}

function handleSizeChange() {
  page.value = 1
  load()
}

async function openDetail(row: SalesReturn) {
  try {
    detail.value = await salesApi.returns.get(row.id)
    detailVisible.value = true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载销售退货详情失败')
  }
}

function creditNoteStatusLabel(status: string) {
  if (status === 'applied') return '已核销'
  if (status === 'open') return '未核销'
  return status
}

function creditNoteStatusType(status: string) {
  if (status === 'applied') return 'success'
  if (status === 'open') return 'warning'
  return 'info'
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

.pagination {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>
