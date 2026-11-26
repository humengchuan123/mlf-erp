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

    <el-dialog v-model="dialogVisible" :title="editingOrder ? '编辑采购订单' : '新建采购订单'" width="1180px">
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
              <el-date-picker v-model="form.order_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" @change="clearExchangeRate" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="币种">
              <el-select v-model="form.currency" style="width: 100%" @change="clearExchangeRate">
                <el-option v-for="code in currencyCodes" :key="code" :label="code" :value="code" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="汇率">
              <el-input v-model="form.exchange_rate" placeholder="留空自动取汇率" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="备注">
              <el-input v-model="form.remark" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="价格口径">
              <el-switch v-model="form.price_includes_tax" active-text="含税" inactive-text="不含税" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">明细</el-divider>
        <el-table :data="form.lines" border>
          <el-table-column label="物料" min-width="200">
            <template #default="{ row }">
              <el-select v-model="row.material_id" filterable style="width: 100%" placeholder="选择物料" @change="onMaterialChange(row)">
                <el-option v-for="material in materials" :key="material.id" :label="`${material.name}（${material.code}）`" :value="material.id" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="数量" width="110">
            <template #default="{ row }">
              <el-input v-model="row.quantity" placeholder="0" />
            </template>
          </el-table-column>
          <el-table-column label="单价" width="120">
            <template #default="{ row }">
              <el-input v-model="row.unit_price" placeholder="0.0000" />
            </template>
          </el-table-column>
          <el-table-column label="税率" width="130">
            <template #default="{ row }">
              <el-select v-model="row.tax_rate_id" clearable style="width: 100%" placeholder="默认物料">
                <el-option v-for="rate in taxRates" :key="rate.id" :label="rate.name" :value="rate.id" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="不含税金额" width="120" align="right">
            <template #default="{ row }">{{ lineNet(row) }}</template>
          </el-table-column>
          <el-table-column label="税额" width="100" align="right">
            <template #default="{ row }">{{ lineTax(row) }}</template>
          </el-table-column>
          <el-table-column label="含税金额" width="120" align="right">
            <template #default="{ row }">{{ lineGross(row) }}</template>
          </el-table-column>
          <el-table-column width="70">
            <template #default="{ $index }">
              <el-button link type="danger" @click="removeLine($index)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
        <div class="line-totals">
          <span>不含税合计：{{ formNetTotal }}</span>
          <span>税额合计：{{ formTaxTotal }}</span>
          <span>含税合计：{{ formGrossTotal }}</span>
          <span>本位币含税合计：{{ formBaseGrossTotal }}</span>
        </div>
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
          <el-descriptions-item label="币种/汇率">{{ detailOrder.currency }} / {{ detailOrder.exchange_rate }}</el-descriptions-item>
          <el-descriptions-item label="价格口径">{{ detailOrder.price_includes_tax ? '含税' : '不含税' }}</el-descriptions-item>
          <el-descriptions-item label="不含税金额">{{ detailOrder.net_amount }}</el-descriptions-item>
          <el-descriptions-item label="税额">{{ detailOrder.tax_amount }}</el-descriptions-item>
          <el-descriptions-item label="含税金额">{{ detailOrder.total_amount }}</el-descriptions-item>
          <el-descriptions-item label="本位币含税金额">{{ detailOrder.base_total_amount }}</el-descriptions-item>
          <el-descriptions-item label="备注" :span="2">{{ detailOrder.remark || '-' }}</el-descriptions-item>
        </el-descriptions>

        <el-table :data="detailOrder.lines" border class="detail-table">
          <el-table-column prop="line_no" label="#" width="60" />
          <el-table-column prop="material_name" label="物料" min-width="160" />
          <el-table-column prop="quantity" label="数量" width="90" align="right" />
          <el-table-column prop="unit_price" label="单价" width="100" align="right" />
          <el-table-column prop="tax_rate" label="税率" width="90" align="right" />
          <el-table-column prop="amount" label="不含税" width="110" align="right" />
          <el-table-column prop="tax_amount" label="税额" width="100" align="right" />
          <el-table-column prop="gross_amount" label="含税" width="110" align="right" />
          <el-table-column prop="received_qty" label="已收" width="90" align="right" />
          <el-table-column prop="remaining_qty" label="未收" width="90" align="right" />
        </el-table>

        <el-divider v-if="canReadApproval" content-position="left">审批历史</el-divider>
        <div v-if="canReadApproval" v-loading="approvalHistoryLoading" class="approval-history">
          <div v-if="approvalHistoryError" class="approval-history__hint">
            {{ approvalHistoryError }}
          </div>
          <el-empty
            v-else-if="approvalRequests.length === 0"
            description="无需审批/未配置审批流程"
            :image-size="60"
          />
          <template v-else-if="latestApproval">
            <el-descriptions :column="3" border size="small">
              <el-descriptions-item label="状态">
                <el-tag :type="approvalStatusTagType(latestApproval.status)" size="small">
                  {{ approvalStatusLabel(latestApproval.status) }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="当前步骤">
                {{ latestApproval.current_step }} / {{ latestApproval.steps.length || '-' }}
              </el-descriptions-item>
              <el-descriptions-item label="申请时间">
                {{ formatApprovalTime(latestApproval.requested_at) }}
              </el-descriptions-item>
            </el-descriptions>
            <el-table
              :data="latestApproval.steps"
              border
              size="small"
              class="approval-history__table"
            >
              <el-table-column prop="step_no" label="步骤" width="60" />
              <el-table-column prop="name" label="步骤名称" min-width="130" />
              <el-table-column label="审批人类型" width="100">
                <template #default="{ row }">
                  {{ approvalApproverTypeLabel(row.approver_type) }}
                </template>
              </el-table-column>
              <el-table-column prop="approver_id" label="审批人 ID" min-width="180" />
              <el-table-column label="状态" width="90">
                <template #default="{ row }">
                  {{ approvalStepState(latestApproval, row.step_no) }}
                </template>
              </el-table-column>
            </el-table>
            <el-table
              :data="latestApproval.actions"
              border
              size="small"
              class="approval-history__table"
            >
              <el-table-column prop="step_no" label="步骤" width="60" />
              <el-table-column label="动作" width="80">
                <template #default="{ row }">{{ approvalActionLabel(row.action) }}</template>
              </el-table-column>
              <el-table-column prop="actor_id" label="操作人" min-width="180" />
              <el-table-column label="意见" min-width="160">
                <template #default="{ row }">{{ row.comment || '-' }}</template>
              </el-table-column>
              <el-table-column label="时间" width="170">
                <template #default="{ row }">{{ formatApprovalTime(row.created_at) }}</template>
              </el-table-column>
            </el-table>
            <div v-if="latestApproval.actions.length === 0" class="approval-history__hint">
              暂无审批动作
            </div>
          </template>
        </div>

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
  approvalApi,
  masterDataApi,
  purchaseApi,
  type ApprovalRequest,
  type Material,
  type Partner,
  type PurchaseOrder,
  type TaxRate,
  type Warehouse
} from '@mlf-erp/sdk'
import { newIdempotencyKey, shouldRetainIdempotencyKey } from '../../utils/idempotency'
import { useAuthStore } from '@/stores/auth'

interface OrderFormLine {
  material_id: string
  quantity: string
  unit_price: string
  tax_rate_id: string
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
const canReadApproval = computed(() => authStore.hasPermission('system.approval.read'))

const orders = ref<PurchaseOrder[]>([])
const partners = ref<Partner[]>([])
const warehouses = ref<Warehouse[]>([])
const materials = ref<Material[]>([])
const taxRates = ref<TaxRate[]>([])
const currencyCodes = ref<string[]>(['CNY', 'USD'])
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
  exchange_rate: '',
  price_includes_tax: true,
  remark: '',
  lines: [] as OrderFormLine[]
})

const detailVisible = ref(false)
const detailOrder = ref<PurchaseOrder | null>(null)
const approvalRequests = ref<ApprovalRequest[]>([])
const approvalHistoryLoading = ref(false)
const approvalHistoryError = ref('')
const latestApproval = computed(() => approvalRequests.value[0] ?? null)
const receiveVisible = ref(false)
const receiveIdempotencyKey = ref('')
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
  const now = new Date()
  const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 10)
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
    const [partnerResult, warehouseResult, materialResult, taxRateResult] = await Promise.all([
      masterDataApi.partners.list({ page_size: 200 }),
      masterDataApi.warehouses.list({ page_size: 200 }),
      masterDataApi.materials.list({ page_size: 200 }),
      masterDataApi.taxRates.list({ page_size: 200 })
    ])
    partners.value = partnerResult.items
    warehouses.value = warehouseResult.items.filter((item) => item.status === 'active')
    materials.value = materialResult.items.filter((item) => item.status === 'active')
    taxRates.value = taxRateResult.items.filter((item) => item.status === 'active')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载采购选项失败')
  }
  try {
    const currencyResult = await masterDataApi.currencies.list({ page_size: 200 })
    currencyCodes.value = currencyResult.items.filter((item) => item.status === 'active').map((item) => item.code)
  } catch {
    // Currency options fall back to CNY/USD so order entry still works without masterdata.currency.read.
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
  return { material_id: '', quantity: '', unit_price: '', tax_rate_id: '' }
}

function openCreate() {
  editingOrder.value = null
  Object.assign(form, {
    supplier_id: '',
    warehouse_id: '',
    order_date: today(),
    currency: 'CNY',
    exchange_rate: '',
    price_includes_tax: true,
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
    exchange_rate: order.exchange_rate,
    price_includes_tax: order.price_includes_tax,
    remark: order.remark,
    lines: order.lines.map((line) => ({
      material_id: line.material_id,
      quantity: line.quantity,
      unit_price: line.unit_price,
      tax_rate_id: line.tax_rate_id
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

function onMaterialChange(line: OrderFormLine) {
  const material = materials.value.find((item) => item.id === line.material_id)
  line.tax_rate_id = material?.tax_rate_id ?? ''
}

function clearExchangeRate() {
  form.exchange_rate = ''
}

function taxRateValue(line: OrderFormLine) {
  const rateID = line.tax_rate_id || materials.value.find((item) => item.id === line.material_id)?.tax_rate_id
  const rate = taxRates.value.find((item) => item.id === rateID)
  return rate ? Number(rate.rate) : 0
}

function calcLine(line: OrderFormLine) {
  const quantity = Number(line.quantity)
  const unitPrice = Number(line.unit_price)
  const rate = taxRateValue(line)
  if (!Number.isFinite(quantity) || !Number.isFinite(unitPrice) || !Number.isFinite(rate)) {
    return { net: 0, tax: 0, gross: 0 }
  }
  const entered = quantity * unitPrice
  if (form.price_includes_tax) {
    const net = entered / (1 + rate)
    const tax = entered - net
    return { net: round4(net), tax: round4(tax), gross: round4(entered) }
  }
  const tax = entered * rate
  return { net: round4(entered), tax: round4(tax), gross: round4(entered + tax) }
}

function round4(value: number) {
  return Math.round(value * 10000) / 10000
}

function lineNet(line: OrderFormLine) {
  return calcLine(line).net.toFixed(4)
}

function lineTax(line: OrderFormLine) {
  return calcLine(line).tax.toFixed(4)
}

function lineGross(line: OrderFormLine) {
  return calcLine(line).gross.toFixed(4)
}

const formNetTotal = computed(() => form.lines.reduce((total, line) => total + calcLine(line).net, 0).toFixed(4))
const formTaxTotal = computed(() => form.lines.reduce((total, line) => total + calcLine(line).tax, 0).toFixed(4))
const formGrossTotal = computed(() => form.lines.reduce((total, line) => total + calcLine(line).gross, 0).toFixed(4))
const formBaseGrossTotal = computed(() => {
  const rate = Number(form.exchange_rate)
  if (!Number.isFinite(rate) || rate <= 0) {
    return '-'
  }
  return (Number(formGrossTotal.value) * rate).toFixed(4)
})

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
      exchange_rate: form.exchange_rate || undefined,
      price_includes_tax: form.price_includes_tax,
      remark: form.remark,
      lines: form.lines.map((line) => ({
        material_id: line.material_id,
        quantity: line.quantity,
        unit_price: line.unit_price,
        tax_rate_id: line.tax_rate_id || undefined
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
    if (canReadApproval.value) {
      await loadApprovalHistory(order.id)
    } else {
      approvalRequests.value = []
      approvalHistoryError.value = ''
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载采购订单详情失败')
  }
}

async function loadApprovalHistory(orderID: string) {
  if (!canReadApproval.value) {
    approvalRequests.value = []
    approvalHistoryError.value = ''
    approvalHistoryLoading.value = false
    return
  }
  approvalHistoryLoading.value = true
  approvalHistoryError.value = ''
  try {
    const result = await approvalApi.requests({
      document_type: 'purchase_order',
      document_id: orderID,
      page_size: 20
    })
    approvalRequests.value = result.items
  } catch {
    approvalRequests.value = []
    approvalHistoryError.value = '审批历史加载失败'
  } finally {
    approvalHistoryLoading.value = false
  }
}

function approvalStatusLabel(status: string) {
  if (status === 'pending') return '审批中'
  if (status === 'approved') return '已通过'
  if (status === 'rejected') return '已拒绝'
  if (status === 'cancelled') return '已取消'
  return status || '-'
}

function approvalStatusTagType(status: string) {
  if (status === 'approved') return 'success'
  if (status === 'rejected') return 'danger'
  if (status === 'cancelled') return 'info'
  return 'warning'
}

function approvalApproverTypeLabel(value: string) {
  if (value === 'role') return '角色'
  if (value === 'user') return '用户'
  return value || '-'
}

function approvalActionLabel(value: string) {
  if (value === 'approve') return '批准'
  if (value === 'reject') return '拒绝'
  if (value === 'cancel') return '取消'
  return value || '-'
}

function approvalStepState(request: ApprovalRequest, stepNo: number) {
  const action = request.actions.find((item) => item.step_no === stepNo)
  if (action) {
    if (action.action === 'approve') return '已通过'
    if (action.action === 'reject') return '已拒绝'
    return '已取消'
  }
  if (request.status === 'pending') {
    if (stepNo === request.current_step) return '待审批'
    if (stepNo < request.current_step) return '已处理'
    return '未开始'
  }
  if (request.status === 'approved') return '已通过'
  return '未开始'
}

function formatApprovalTime(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
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
      await loadApprovalHistory(order.id)
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : `${label}失败`)
  }
}

async function openReceive(order: PurchaseOrder) {
  receiveIdempotencyKey.value = newIdempotencyKey()
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
  for (const line of receiveForm.lines) {
    if (line.quantity === '') {
      continue
    }
    const quantity = Number(line.quantity)
    if (!Number.isFinite(quantity) || quantity < 0) {
      ElMessage.warning('收货数量必须是非负数字')
      return
    }
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
    }, receiveIdempotencyKey.value)
    receiveIdempotencyKey.value = ''
    ElMessage.success('收货成功，库存已入库')
    receiveVisible.value = false
    await loadOrders()
    if (detailVisible.value && detailOrder.value?.id === receiveOrder.value.id) {
      detailOrder.value = await purchaseApi.get(receiveOrder.value.id)
    }
  } catch (error) {
    if (!shouldRetainIdempotencyKey(error)) {
      receiveIdempotencyKey.value = newIdempotencyKey()
    }
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

.line-totals {
  display: flex;
  gap: 20px;
  justify-content: flex-end;
  margin-top: 10px;
  color: #475569;
  font-size: 13px;
}

.detail-table {
  margin-top: 16px;
}

.drawer-actions {
  display: flex;
  gap: 8px;
  margin-top: 16px;
}

.approval-history {
  min-height: 40px;
}

.approval-history__table {
  margin-top: 12px;
}

.approval-history__hint {
  margin-top: 8px;
  color: #64748b;
  font-size: 12px;
}

.receive-hint {
  margin: 0 0 16px;
  color: #475569;
}
</style>
