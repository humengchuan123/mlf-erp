<template>
  <div>
    <PageHeader title="销售订单" description="销售订单 → 审批 → 发货出库 → 应收发票。">
      <el-button v-if="canWrite" type="primary" @click="openCreate">新建销售订单</el-button>
    </PageHeader>

    <el-card shadow="never">
      <div class="toolbar">
        <el-input
          v-model="keyword"
          placeholder="搜索单号、客户或仓库"
          clearable
          class="toolbar__search"
          @keyup.enter="search"
        />
        <el-select v-model="statusFilter" placeholder="全部状态" clearable class="toolbar__status">
          <el-option v-for="option in statusOptions" :key="option.value" :label="option.label" :value="option.value" />
        </el-select>
        <el-button type="primary" plain @click="search">查询</el-button>
        <el-button @click="resetSearch">重置</el-button>
      </div>

      <el-table v-loading="loading" :data="orders" border stripe>
        <el-table-column prop="code" label="单号" min-width="150" />
        <el-table-column prop="customer_name" label="客户" min-width="150" />
        <el-table-column prop="warehouse_name" label="仓库" min-width="120" />
        <el-table-column prop="order_date" label="订单日期" width="120" />
        <el-table-column prop="total_amount" label="金额" width="130" align="right" />
        <el-table-column prop="status" label="状态" width="130">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="330" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openDetail(row)">详情</el-button>
            <el-button v-if="canWrite && row.status === 'draft'" link type="primary" @click="openEdit(row)">
              编辑
            </el-button>
            <el-button v-if="canWrite && row.status === 'draft'" link type="success" @click="submitOrder(row)">
              提交
            </el-button>
            <el-button v-if="canApprove && row.status === 'submitted'" link type="success" @click="approveOrder(row)">
              审批
            </el-button>
            <el-button
              v-if="canWrite && (row.status === 'approved' || row.status === 'partially_shipped')"
              link
              type="warning"
              @click="openShip(row)"
            >
              发货
            </el-button>
            <el-button v-if="canWrite && canCancel(row.status)" link type="danger" @click="cancelOrder(row)">
              取消
            </el-button>
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
        @current-change="loadOrders"
        @size-change="handleSizeChange"
      />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingOrder ? '编辑销售订单' : '新建销售订单'" width="900px">
      <el-form :model="form" label-width="90px">
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="客户" required>
              <el-select v-model="form.customer_id" filterable style="width: 100%" placeholder="选择客户">
                <el-option v-for="partner in customers" :key="partner.id" :label="partner.name" :value="partner.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="仓库" required>
              <el-select v-model="form.warehouse_id" filterable style="width: 100%" placeholder="选择仓库">
                <el-option v-for="warehouse in warehouses" :key="warehouse.id" :label="warehouse.name" :value="warehouse.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="订单日期" required>
              <el-date-picker v-model="form.order_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="币种">
              <el-select v-model="form.currency" style="width: 100%">
                <el-option label="CNY" value="CNY" />
                <el-option label="USD" value="USD" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="备注">
              <el-input v-model="form.remark" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">明细</el-divider>
        <el-table :data="form.lines" border>
          <el-table-column label="物料" min-width="220">
            <template #default="{ row }">
              <el-select v-model="row.material_id" filterable style="width: 100%" placeholder="选择物料">
                <el-option
                  v-for="material in materials"
                  :key="material.id"
                  :label="`${material.name}（${material.code}）`"
                  :value="material.id"
                />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="数量" width="140">
            <template #default="{ row }">
              <el-input v-model="row.quantity" placeholder="0" />
            </template>
          </el-table-column>
          <el-table-column label="单价" width="140">
            <template #default="{ row }">
              <el-input v-model="row.unit_price" placeholder="0.0000" />
            </template>
          </el-table-column>
          <el-table-column label="金额" width="130" align="right">
            <template #default="{ row }">{{ lineAmount(row) }}</template>
          </el-table-column>
          <el-table-column width="80">
            <template #default="{ $index }">
              <el-button link type="danger" @click="removeLine($index)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-button class="add-line" plain @click="addLine">添加明细行</el-button>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveOrder">保存</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="detailVisible" title="销售订单详情" size="720px">
      <template v-if="detailOrder">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="单号">{{ detailOrder.code }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="statusTagType(detailOrder.status)" size="small">{{ statusLabel(detailOrder.status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="客户">{{ detailOrder.customer_name }}</el-descriptions-item>
          <el-descriptions-item label="仓库">{{ detailOrder.warehouse_name }}</el-descriptions-item>
          <el-descriptions-item label="订单日期">{{ detailOrder.order_date }}</el-descriptions-item>
          <el-descriptions-item label="金额">{{ detailOrder.total_amount }}</el-descriptions-item>
          <el-descriptions-item label="备注" :span="2">{{ detailOrder.remark || '-' }}</el-descriptions-item>
        </el-descriptions>

        <el-table :data="detailOrder.lines" border class="detail-table">
          <el-table-column prop="line_no" label="#" width="60" />
          <el-table-column prop="material_name" label="物料" min-width="180" />
          <el-table-column prop="quantity" label="数量" width="100" align="right" />
          <el-table-column prop="unit_price" label="单价" width="110" align="right" />
          <el-table-column prop="amount" label="金额" width="120" align="right" />
          <el-table-column prop="shipped_qty" label="已发" width="100" align="right" />
          <el-table-column prop="remaining_qty" label="未发" width="100" align="right" />
        </el-table>

        <div class="drawer-actions">
          <el-button v-if="canWrite && detailOrder.status === 'draft'" type="primary" @click="submitOrder(detailOrder)">
            提交
          </el-button>
          <el-button v-if="canApprove && detailOrder.status === 'submitted'" type="success" @click="approveOrder(detailOrder)">
            审批
          </el-button>
          <el-button
            v-if="canWrite && (detailOrder.status === 'approved' || detailOrder.status === 'partially_shipped')"
            type="warning"
            @click="openShip(detailOrder)"
          >
            发货
          </el-button>
          <el-button v-if="canWrite && canCancel(detailOrder.status)" type="danger" plain @click="cancelOrder(detailOrder)">
            取消
          </el-button>
        </div>
      </template>
    </el-drawer>

    <el-dialog v-model="shipVisible" title="销售发货" width="760px">
      <p v-if="shipOrderTarget" class="ship-hint">
        销售单 {{ shipOrderTarget.code }} · {{ shipOrderTarget.customer_name }} · {{ shipOrderTarget.warehouse_name }}
      </p>
      <el-form :model="shipForm" label-width="90px">
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="发货日期" required>
              <el-date-picker v-model="shipForm.shipment_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="备注">
              <el-input v-model="shipForm.remark" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-table :data="shipForm.lines" border>
          <el-table-column prop="material_name" label="物料" min-width="180" />
          <el-table-column prop="remaining_qty" label="未发数量" width="110" align="right" />
          <el-table-column prop="available_qty" label="库存可用" width="110" align="right" />
          <el-table-column label="本次发货" width="160">
            <template #default="{ row }">
              <el-input v-model="row.quantity" placeholder="0" />
            </template>
          </el-table-column>
        </el-table>
      </el-form>
      <template #footer>
        <el-button @click="shipVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveShip">确认发货</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import {
  inventoryApi,
  masterDataApi,
  salesApi,
  type Material,
  type Partner,
  type SalesOrder,
  type Warehouse
} from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

interface OrderFormLine {
  material_id: string
  quantity: string
  unit_price: string
}

interface ShipFormLine {
  order_line_id: string
  material_name: string
  remaining_qty: string
  available_qty: string
  quantity: string
}

const authStore = useAuthStore()
const canWrite = computed(() => authStore.hasPermission('sales.order.write'))
const canApprove = computed(() => authStore.hasPermission('sales.order.approve'))

const orders = ref<SalesOrder[]>([])
const partners = ref<Partner[]>([])
const warehouses = ref<Warehouse[]>([])
const materials = ref<Material[]>([])
const loading = ref(false)
const saving = ref(false)
const keyword = ref('')
const statusFilter = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const dialogVisible = ref(false)
const editingOrder = ref<SalesOrder | null>(null)
const form = reactive({
  customer_id: '',
  warehouse_id: '',
  order_date: today(),
  currency: 'CNY',
  remark: '',
  lines: [] as OrderFormLine[]
})

const detailVisible = ref(false)
const detailOrder = ref<SalesOrder | null>(null)
const shipVisible = ref(false)
const shipOrderTarget = ref<SalesOrder | null>(null)
const shipForm = reactive({
  shipment_date: today(),
  remark: '',
  lines: [] as ShipFormLine[]
})

const statusOptions = [
  { label: '草稿', value: 'draft' },
  { label: '已提交', value: 'submitted' },
  { label: '已审批', value: 'approved' },
  { label: '部分发货', value: 'partially_shipped' },
  { label: '已发货', value: 'shipped' },
  { label: '已取消', value: 'cancelled' }
]

const customers = computed(() =>
  partners.value.filter(
    (partner) => partner.status === 'active' && (partner.type === 'customer' || partner.type === 'both')
  )
)

function today() {
  const now = new Date()
  const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 10)
}

function statusLabel(status: string) {
  return statusOptions.find((option) => option.value === status)?.label ?? status
}

function statusTagType(status: string) {
  if (status === 'approved' || status === 'shipped') return 'success'
  if (status === 'submitted' || status === 'partially_shipped') return 'warning'
  if (status === 'cancelled') return 'info'
  return ''
}

function canCancel(status: string) {
  return status === 'draft' || status === 'submitted' || status === 'approved'
}

async function loadOrders() {
  loading.value = true
  try {
    const result = await salesApi.list({
      status: statusFilter.value,
      keyword: keyword.value,
      page: page.value,
      page_size: pageSize.value
    })
    orders.value = result.items
    total.value = result.total
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载销售订单失败')
  } finally {
    loading.value = false
  }
}

async function loadOptions() {
  try {
    const [partnerResult, warehouseResult, materialResult] = await Promise.all([
      masterDataApi.partners.list({ page_size: 200 }),
      masterDataApi.warehouses.list({ page_size: 200 }),
      masterDataApi.materials.list({ page_size: 200 })
    ])
    partners.value = partnerResult.items
    warehouses.value = warehouseResult.items.filter((item) => item.status === 'active')
    materials.value = materialResult.items.filter((item) => item.status === 'active')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载销售选项失败')
  }
}

function search() {
  page.value = 1
  loadOrders()
}

function resetSearch() {
  keyword.value = ''
  statusFilter.value = ''
  page.value = 1
  loadOrders()
}

function handleSizeChange() {
  page.value = 1
  loadOrders()
}

function factoryLine(): OrderFormLine {
  return { material_id: '', quantity: '', unit_price: '' }
}

function openCreate() {
  editingOrder.value = null
  Object.assign(form, {
    customer_id: '',
    warehouse_id: '',
    order_date: today(),
    currency: 'CNY',
    remark: '',
    lines: [factoryLine()]
  })
  dialogVisible.value = true
}

function openEdit(order: SalesOrder) {
  editingOrder.value = order
  Object.assign(form, {
    customer_id: order.customer_id,
    warehouse_id: order.warehouse_id,
    order_date: order.order_date,
    currency: order.currency,
    remark: order.remark,
    lines: order.lines.map((line) => ({
      material_id: line.material_id,
      quantity: line.quantity,
      unit_price: line.unit_price
    }))
  })
  dialogVisible.value = true
}

function addLine() {
  form.lines.push(factoryLine())
}

function removeLine(index: number) {
  form.lines.splice(index, 1)
}

function lineAmount(line: OrderFormLine) {
  const quantity = Number(line.quantity)
  const price = Number(line.unit_price)
  if (!Number.isFinite(quantity) || !Number.isFinite(price)) {
    return '0.0000'
  }
  return (quantity * price).toFixed(4)
}

async function saveOrder() {
  if (!form.customer_id || !form.warehouse_id || !form.order_date) {
    ElMessage.warning('请选择客户、仓库和订单日期')
    return
  }
  if (form.lines.length === 0) {
    ElMessage.warning('至少添加一行明细')
    return
  }
  for (const line of form.lines) {
    if (!line.material_id || !line.quantity || !line.unit_price) {
      ElMessage.warning('请完整填写明细行的物料、数量和单价')
      return
    }
  }

  saving.value = true
  try {
    const input = {
      customer_id: form.customer_id,
      warehouse_id: form.warehouse_id,
      order_date: form.order_date,
      currency: form.currency,
      remark: form.remark,
      lines: form.lines.map((line) => ({
        material_id: line.material_id,
        quantity: line.quantity,
        unit_price: line.unit_price
      }))
    }
    if (editingOrder.value) {
      await salesApi.update(editingOrder.value.id, input)
      ElMessage.success('销售订单已更新')
    } else {
      await salesApi.create(input)
      ElMessage.success('销售订单已创建')
    }
    dialogVisible.value = false
    await loadOrders()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存销售订单失败')
  } finally {
    saving.value = false
  }
}

async function openDetail(order: SalesOrder) {
  try {
    detailOrder.value = await salesApi.get(order.id)
    detailVisible.value = true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载销售订单详情失败')
  }
}

async function submitOrder(order: SalesOrder) {
  await runAction(order, '提交', () => salesApi.submit(order.id))
}

async function approveOrder(order: SalesOrder) {
  await runAction(order, '审批', () => salesApi.approve(order.id))
}

async function cancelOrder(order: SalesOrder) {
  await runAction(order, '取消', () => salesApi.cancel(order.id))
}

async function runAction(order: SalesOrder, label: string, action: () => Promise<unknown>) {
  try {
    await ElMessageBox.confirm(`确认${label}销售订单 ${order.code}？`, label, {
      type: label === '取消' ? 'warning' : 'info'
    })
  } catch {
    return
  }

  try {
    await action()
    ElMessage.success(`${label}成功`)
    await loadOrders()
    if (detailVisible.value && detailOrder.value?.id === order.id) {
      detailOrder.value = await salesApi.get(order.id)
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : `${label}失败`)
  }
}

async function openShip(order: SalesOrder) {
  try {
    const detail = await salesApi.get(order.id)
    shipOrderTarget.value = detail
    shipForm.shipment_date = today()
    shipForm.remark = ''
    const balanceResult = await inventoryApi.balances({ page_size: 200 })
    const availableByMaterial = new Map(
      balanceResult.items.map((item) => [item.material_id, Number(item.available)])
    )
    shipForm.lines = detail.lines
      .filter((line) => Number(line.remaining_qty) > 0)
      .map((line) => ({
        order_line_id: line.id,
        material_name: line.material_name,
        remaining_qty: line.remaining_qty,
        available_qty: String(availableByMaterial.get(line.material_id) ?? 0),
        quantity: line.remaining_qty
      }))
    shipVisible.value = true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载发货明细失败')
  }
}

async function saveShip() {
  if (!shipOrderTarget.value) {
    return
  }
  for (const line of shipForm.lines) {
    if (line.quantity === '') {
      continue
    }
    const quantity = Number(line.quantity)
    if (!Number.isFinite(quantity) || quantity < 0) {
      ElMessage.warning('发货数量必须是非负数字')
      return
    }
    if (quantity > Number(line.remaining_qty)) {
      ElMessage.warning(`${line.material_name} 发货数量不能超过未发数量`)
      return
    }
    if (quantity > Number(line.available_qty)) {
      ElMessage.warning(`${line.material_name} 库存不足`)
      return
    }
  }
  const lines = shipForm.lines
    .filter((line) => Number(line.quantity) > 0)
    .map((line) => ({ order_line_id: line.order_line_id, quantity: line.quantity }))
  if (lines.length === 0) {
    ElMessage.warning('请填写本次发货数量')
    return
  }

  saving.value = true
  try {
    await salesApi.ship(shipOrderTarget.value.id, {
      shipment_date: shipForm.shipment_date,
      remark: shipForm.remark,
      lines
    })
    ElMessage.success('发货成功，库存已扣减并生成应收发票')
    shipVisible.value = false
    await loadOrders()
    if (detailVisible.value && detailOrder.value?.id === shipOrderTarget.value.id) {
      detailOrder.value = await salesApi.get(shipOrderTarget.value.id)
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '发货失败')
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadOrders(), loadOptions()])
})
</script>

<style scoped>
.toolbar {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}

.toolbar__search {
  width: 280px;
}

.toolbar__status {
  width: 150px;
}

.pagination {
  margin-top: 16px;
  justify-content: flex-end;
}

.add-line {
  margin-top: 12px;
}

.detail-table {
  margin-top: 16px;
}

.drawer-actions {
  display: flex;
  gap: 8px;
  margin-top: 16px;
}

.ship-hint {
  margin: 0 0 16px;
  color: #475569;
}
</style>
