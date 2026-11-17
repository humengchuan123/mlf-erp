<template>
  <div>
    <PageHeader title="发货单" description="查看销售发货、出库成本和对应应收发票。">
      <el-input v-model="orderId" placeholder="按销售订单 ID 筛选" clearable class="search" @keyup.enter="search" />
      <el-button type="primary" plain @click="search">查询</el-button>
    </PageHeader>

    <el-card shadow="never">
      <el-table v-loading="loading" :data="shipments" border stripe>
        <el-table-column prop="code" label="发货单号" min-width="150" />
        <el-table-column prop="order_code" label="销售订单" min-width="150" />
        <el-table-column prop="customer_name" label="客户" min-width="150" />
        <el-table-column prop="warehouse_name" label="仓库" min-width="120" />
        <el-table-column prop="shipment_date" label="发货日期" width="120" />
        <el-table-column prop="total_amount" label="发货金额" width="130" align="right" />
        <el-table-column prop="total_cost" label="出库成本" width="130" align="right" />
        <el-table-column prop="invoice_code" label="应收发票" min-width="150" />
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

    <el-drawer v-model="detailVisible" title="发货单详情" size="640px">
      <template v-if="detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="发货单号">{{ detail.code }}</el-descriptions-item>
          <el-descriptions-item label="销售订单">{{ detail.order_code }}</el-descriptions-item>
          <el-descriptions-item label="客户">{{ detail.customer_name }}</el-descriptions-item>
          <el-descriptions-item label="仓库">{{ detail.warehouse_name }}</el-descriptions-item>
          <el-descriptions-item label="发货日期">{{ detail.shipment_date }}</el-descriptions-item>
          <el-descriptions-item label="应收发票">{{ detail.invoice_code || '-' }}</el-descriptions-item>
          <el-descriptions-item label="发货金额">{{ detail.total_amount }}</el-descriptions-item>
          <el-descriptions-item label="出库成本">{{ detail.total_cost }}</el-descriptions-item>
        </el-descriptions>

        <el-table :data="detail.lines" border class="detail-table">
          <el-table-column prop="material_name" label="物料" min-width="180" />
          <el-table-column prop="quantity" label="数量" width="110" align="right" />
          <el-table-column prop="unit_price" label="售价" width="110" align="right" />
          <el-table-column prop="amount" label="金额" width="120" align="right" />
          <el-table-column prop="unit_cost" label="单位成本" width="110" align="right" />
          <el-table-column prop="cost_amount" label="成本金额" width="120" align="right" />
        </el-table>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import { salesApi, type Shipment } from '@mlf-erp/sdk'

const shipments = ref<Shipment[]>([])
const loading = ref(false)
const orderId = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const detailVisible = ref(false)
const detail = ref<Shipment | null>(null)

async function load() {
  loading.value = true
  try {
    const result = await salesApi.shipments({
      order_id: orderId.value,
      page: page.value,
      page_size: pageSize.value
    })
    shipments.value = result.items
    total.value = result.total
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载发货单失败')
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function handleSizeChange() {
  page.value = 1
  load()
}

function openDetail(row: Shipment) {
  detail.value = row
  detailVisible.value = true
}

onMounted(load)
</script>

<style scoped>
.search {
  width: 300px;
}

.pagination {
  margin-top: 16px;
  justify-content: flex-end;
}

.detail-table {
  margin-top: 16px;
}
</style>
