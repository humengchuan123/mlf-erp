<template>
  <div>
    <PageHeader title="采购订单" description="采购订单 → 审批 → 收货 → 库存入库。">
      <el-button v-if="canWrite" type="primary" @click="openCreate">新建采购订单</el-button>
    </PageHeader>

    <el-card shadow="never">
      <div class="toolbar">
        <el-input
          v-model="keyword"
          placeholder="搜索单号、供应商或仓库"
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
        <el-table-column prop="supplier_name" label="供应商" min-width="150" />
        <el-table-column prop="warehouse_name" label="仓库" min-width="120" />
        <el-table-column prop="order_date" label="订单日期" width="120" />
        <el-table-column prop="total_amount" label="金额" width="130" align="right" />
        <el-table-column prop="status" label="状态" width="120">
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
              v-if="canWrite && row.status === 'approved' || canWrite && row.status === 'partially_received'"
              link
              type="warning"
              @click="openReceive(row)"
            >
              收货
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

    <el-dialog v-model="dialogVisible" :title="editingOrder ? '编辑采购订单' : '新建采购订单'" width="900px">
      <el-form :model="form" label-width="90px">
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="供应商" required>
              <el-select v-model="form.supplier_id" filterable style="width: 100%" placeholder="选择供应商">
                <el-option v-for="partner in suppliers" :key="partner.id" :label="partner.name" :value="partner.id" />
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
                <el-option v-for="material in materials" :key="material.id" :label="`${material.name}（${material.code}）`" :value="material.id" />
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

    <el-drawer v-model="detailVisible" title="采购订单详情" size="720px">
      <template v-if="detailOrder">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="单号">{{ detailOrder.code }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="statusTagType(detailOrder.status)" size="small">{{ statusLabel(detailOrder.status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="供应商">{{ detailOrder.supplier_name }}</el-descriptions-item>
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
          <el-table-column prop="received_qty" label="已收" width="100" align="right" />
          <el-table-column prop="remaining_qty" label="未收" width="100" align="right" />
        </el-table>

        <div class="drawer-actions">
          <el-button v-if="canWrite && detailOrder.status === 'draft'" type="primary" @click="submitOrder(detailOrder)">
            提交
          </el-button>
          <el-button v-if="canApprove && detailOrder.status === 'submitted'" type="success" @click="approveOrder(detailOrder)">
            审批
          </el-button>
          <el-button
            v-if="canWrite && (detailOrder.status === 'approved' || detailOrder.status === 'partially_received')"
            type="warning"
            @click="openReceive(detailOrder)"
          >
            收货
          </el-button>
          <el-button v-if="canWrite && canCancel(detailOrder.status)" type="danger" plain @click="cancelOrder(detailOrder)">
            取消
          </el-button>
        </div>
      </template>
    </el-drawer>

    <el-dialog v-model="receiveVisible" title="采购收货" width="760px">
      <p v-if="receiveOrder" class="receive-hint">
        采购单 {{ receiveOrder.code }} · {{ receiveOrder.supplier_name }} · {{ receiveOrder.warehouse_name }}
      </p>
      <el-form :model="receiveForm" label-width="90px">
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="收货日期" required>
              <el-date-picker v-model="receiveForm.receipt_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="备注">
              <el-input v-model="receiveForm.remark" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-table :data="receiveForm.lines" border>
          <el-table-column prop="material_name" label="物料" min-width="180" />
          <el-table-column prop="remaining_qty" label="未收数量" width="110" align="right" />
          <el-table-column label="本次收货" width="160">
            <template #default="{ row }">
              <el-input v-model="row.quantity" placeholder="0" />
            </template>
          </el-table-column>
        </el-table>
      </el-form>
      <template #footer>
        <el-button @click="receiveVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveReceive">确认收货</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import {
  masterDataApi,
  purchaseApi,
  type Material,
  type Partner,
  type PurchaseOrder,
  type Warehouse
} from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

interface OrderFormLine {
  material_id: string
  quantity: string
  unit_price: string
}

interface ReceiveFormLine {
  order_line_id: string
  material_name: string
  remaining_qty: string
  quantity: string
}

const authStore = useAuthStore()
const canWrite = computed(() => authStore.hasPermission('purchase.order.write'))
const canApprove = computed(() => authStore.hasPermission('purchase.order.approve'))

const orders = ref<PurchaseOrder[]>([])
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
const editingOrder = ref<PurchaseOrder | null>(null)
const form = reactive({
  supplier_id: '',
  warehouse_id: '',
  order_date: today(),
  currency: 'CNY',
  remark: '',
  lines: [] as OrderFormLine[]
})

const detailVisible = ref(false)
const detailOrder = ref<PurchaseOrder | null>(null)
const receiveVisible = ref(false)
const receiveOrder = ref<PurchaseOrder | null>(null)
const receiveForm = reactive({
  receipt_date: today(),
  remark: '',
  lines: [] as ReceiveFormLine[]
})

const statusOptions = [
  { label: '草稿', value: 'draft' },
  { label: '已提交', value: 'submitted' },
  { label: '已审批', value: 'approved' },
  { label: '部分收货', value: 'partially_received' },
  { label: '已收货', value: 'received' },
  { label: '已关闭', value: 'closed' },
  { label: '已取消', value: 'cancelled' }
]

const suppliers = computed(() =>
  partners.value.filter(
    (partner) => partner.status === 'active' && (partner.type === 'supplier' || partner.type === 'both')
  )
)

function today() {
  return new Date().toISOString().slice(0, 10)
}

function statusLabel(status: string) {
  return statusOptions.find((option) => option.value === status)?.label ?? status
}

function statusTagType(status: string) {
  if (status === 'approved' || status === 'received') return 'success'
  if (status === 'submitted' || status === 'partially_received') return 'warning'
  if (status === 'cancelled') return 'info'
  return ''
}

function canCancel(status: string) {
  return status === 'draft' || status === 'submitted' || status === 'approved'
}

async function loadOrders() {
  loading.value = true
  try {
    const result = await purchaseApi.list({
      status: statusFilter.value,
      keyword: keyword.value,
      page: page.value,
      page_size: pageSize.value
    })
    orders.value = result.items
    total.value = result.total
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载采购订单失败')
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
    ElMessage.error(error instanceof Error ? error.message : '加载采购选项失败')
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
    supplier_id: '',
    warehouse_id: '',
    order_date: today(),
    currency: 'CNY',
    remark: '',
    lines: [factoryLine()]
  })
  dialogVisible.value = true
}

function openEdit(order: PurchaseOrder) {
  editingOrder.value = order
  Object.assign(form, {
    supplier_id: order.supplier_id,
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
  if (!form.supplier_id || !form.warehouse_id || !form.order_date) {
    ElMessage.warning('请选择供应商、仓库和订单日期')
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
      supplier_id: form.supplier_id,
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
      await purchaseApi.update(editingOrder.value.id, input)
      ElMessage.success('采购订单已更新')
    } else {
      await purchaseApi.create(input)
      ElMessage.success('采购订单已创建')
    }
    dialogVisible.value = false
    await loadOrders()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存采购订单失败')
  } finally {
    saving.value = false
  }
}

async function openDetail(order: PurchaseOrder) {
  try {
    detailOrder.value = await purchaseApi.get(order.id)
    detailVisible.value = true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载采购订单详情失败')
  }
}

async function submitOrder(order: PurchaseOrder) {
  await runAction(order, '提交', () => purchaseApi.submit(order.id))
}

async function approveOrder(order: PurchaseOrder) {
  await runAction(order, '审批', () => purchaseApi.approve(order.id))
}

async function cancelOrder(order: PurchaseOrder) {
  await runAction(order, '取消', () => purchaseApi.cancel(order.id))
}

async function runAction(order: PurchaseOrder, label: string, action: () => Promise<unknown>) {
  try {
    await ElMessageBox.confirm(`确认${label}采购订单 ${order.code}？`, label, {
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
      detailOrder.value = await purchaseApi.get(order.id)
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : `${label}失败`)
  }
}

async function openReceive(order: PurchaseOrder) {
  try {
    const detail = await purchaseApi.get(order.id)
    receiveOrder.value = detail
    receiveForm.receipt_date = today()
    receiveForm.remark = ''
    receiveForm.lines = detail.lines
      .filter((line) => Number(line.remaining_qty) > 0)
      .map((line) => ({
        order_line_id: line.id,
        material_name: line.material_name,
        remaining_qty: line.remaining_qty,
        quantity: line.remaining_qty
      }))
    receiveVisible.value = true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载收货明细失败')
  }
}

async function saveReceive() {
  if (!receiveOrder.value) {
    return
  }
  const lines = receiveForm.lines
    .filter((line) => Number(line.quantity) > 0)
    .map((line) => ({ order_line_id: line.order_line_id, quantity: line.quantity }))
  if (lines.length === 0) {
    ElMessage.warning('请填写本次收货数量')
    return
  }

  saving.value = true
  try {
    await purchaseApi.receive(receiveOrder.value.id, {
      receipt_date: receiveForm.receipt_date,
      remark: receiveForm.remark,
      lines
    })
    ElMessage.success('收货成功，库存已入库')
    receiveVisible.value = false
    await loadOrders()
    if (detailVisible.value && detailOrder.value?.id === receiveOrder.value.id) {
      detailOrder.value = await purchaseApi.get(receiveOrder.value.id)
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '收货失败')
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

.receive-hint {
  margin: 0 0 16px;
  color: #475569;
}
</style>
