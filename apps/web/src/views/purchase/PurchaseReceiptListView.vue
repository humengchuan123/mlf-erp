<template>
  <div>
    <PageHeader title="采购收货" description="查看采购收货、入库成本和对应应付发票；可对可退数量发起采购退货。">
      <el-input v-model="orderId" placeholder="按采购订单 ID 筛选" clearable class="search" @keyup.enter="search" />
      <el-button type="primary" plain @click="search">查询</el-button>
      <el-button @click="resetSearch">重置</el-button>
    </PageHeader>

    <el-card shadow="never">
      <el-table v-loading="loading" :data="receipts" border stripe>
        <el-table-column prop="code" label="收货单号" min-width="150" />
        <el-table-column prop="receipt_date" label="收货日期" width="120" />
        <el-table-column prop="order_code" label="采购订单" min-width="150" />
        <el-table-column prop="supplier_name" label="供应商" min-width="150" />
        <el-table-column prop="warehouse_name" label="仓库" min-width="120" />
        <el-table-column prop="total_amount" label="含税金额" width="130" align="right" />
        <el-table-column prop="base_total_amount" label="本位币含税" width="130" align="right" />
        <el-table-column prop="payable_code" label="应付发票" min-width="150">
          <template #default="{ row }">{{ row.payable_code || '-' }}</template>
        </el-table-column>
        <el-table-column prop="voucher_code" label="凭证号" min-width="140">
          <template #default="{ row }">{{ row.voucher_code || '-' }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'posted' ? 'success' : 'info'" size="small">
              {{ row.status === 'posted' ? '已过账' : row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openDetail(row)">详情</el-button>
            <el-button
              v-if="canReturn && hasReturnable(row)"
              link
              type="warning"
              @click="openReturn(row)"
            >
              退货
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
        @current-change="load"
        @size-change="handleSizeChange"
      />
    </el-card>

    <el-drawer v-model="detailVisible" title="采购收货详情" size="760px">
      <template v-if="detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="收货单号">{{ detail.code }}</el-descriptions-item>
          <el-descriptions-item label="收货日期">{{ detail.receipt_date }}</el-descriptions-item>
          <el-descriptions-item label="采购订单">{{ detail.order_code }}</el-descriptions-item>
          <el-descriptions-item label="供应商">{{ detail.supplier_name }}</el-descriptions-item>
          <el-descriptions-item label="仓库">{{ detail.warehouse_name }}</el-descriptions-item>
          <el-descriptions-item label="汇率">{{ detail.exchange_rate }}</el-descriptions-item>
          <el-descriptions-item label="不含税金额">{{ detail.net_amount }}</el-descriptions-item>
          <el-descriptions-item label="税额">{{ detail.tax_amount }}</el-descriptions-item>
          <el-descriptions-item label="含税金额">{{ detail.total_amount }}</el-descriptions-item>
          <el-descriptions-item label="本位币含税金额">{{ detail.base_total_amount }}</el-descriptions-item>
          <el-descriptions-item label="应付发票">{{ detail.payable_code || '-' }}</el-descriptions-item>
          <el-descriptions-item label="凭证号">{{ detail.voucher_code || '-' }}</el-descriptions-item>
          <el-descriptions-item label="备注" :span="2">{{ detail.remark || '-' }}</el-descriptions-item>
        </el-descriptions>

        <el-table :data="detail.lines" border class="detail-table">
          <el-table-column prop="material_name" label="物料" min-width="160" />
          <el-table-column prop="quantity" label="数量" width="90" align="right" />
          <el-table-column prop="returned_qty" label="已退数量" width="90" align="right" />
          <el-table-column prop="returnable_qty" label="可退数量" width="90" align="right" />
          <el-table-column prop="unit_price" label="单价" width="100" align="right" />
          <el-table-column prop="tax_rate" label="税率" width="80" align="right" />
          <el-table-column prop="amount" label="不含税" width="110" align="right" />
          <el-table-column prop="tax_amount" label="税额" width="100" align="right" />
          <el-table-column prop="gross_amount" label="含税" width="110" align="right" />
          <el-table-column prop="base_gross_amount" label="本位币含税" width="120" align="right" />
        </el-table>
      </template>

      <template #footer>
        <el-button
          v-if="canReturn && detail && hasReturnable(detail)"
          type="warning"
          @click="openReturn(detail)"
        >
          退货
        </el-button>
        <el-button @click="detailVisible = false">关闭</el-button>
      </template>
    </el-drawer>

    <el-dialog v-model="returnVisible" title="采购退货" width="1000px">
      <p v-if="returnTarget" class="return-hint">
        采购收货单 {{ returnTarget.code }} · {{ returnTarget.supplier_name }} · {{ returnTarget.warehouse_name }}
      </p>
      <el-form :model="returnForm" label-width="90px">
        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item label="退货日期" required>
              <el-date-picker
                v-model="returnForm.return_date"
                type="date"
                value-format="YYYY-MM-DD"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="退货原因">
              <el-input v-model="returnForm.reason" placeholder="选填" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="备注">
              <el-input v-model="returnForm.remark" placeholder="选填" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>

      <el-table :data="returnForm.lines" border>
        <el-table-column prop="material_name" label="物料" min-width="170" />
        <el-table-column prop="quantity" label="收货数量" width="100" align="right" />
        <el-table-column prop="returned_qty" label="已退数量" width="100" align="right" />
        <el-table-column prop="returnable_qty" label="可退数量" width="100" align="right" />
        <el-table-column prop="unit_price" label="单价" width="100" align="right" />
        <el-table-column prop="tax_rate" label="税率" width="80" align="right" />
        <el-table-column label="单位成本（原收货净额）" width="180" align="right">
          <template #default="{ row }">{{ unitCostText(row) }}</template>
        </el-table-column>
        <el-table-column label="本次退货" width="150">
          <template #default="{ row }">
            <el-input v-model="row.return_qty" placeholder="0" />
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="returnForm.lines.length === 0" description="没有可退数量的明细" :image-size="80" />

      <div class="return-preview">
        <span>预估不含税：{{ returnPreview.net }}</span>
        <span>预估税额：{{ returnPreview.tax }}</span>
        <span>预估含税：{{ returnPreview.gross }}</span>
        <span>预估成本：{{ returnPreview.cost }}</span>
        <span class="return-preview__hint">按原收货金额/成本累计差额估算，实际以过账结果为准。</span>
      </div>

      <template #footer>
        <el-button @click="returnVisible = false">取消</el-button>
        <el-button type="primary" :loading="returning" @click="saveReturn">确认退货</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import {
  purchaseApi,
  type CreatePurchaseReturnLineInput,
  type PurchaseReceipt,
  type PurchaseReceiptLine
} from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

interface ReturnFormLine {
  receipt_line_id: string
  material_name: string
  quantity: string
  returned_qty: string
  returnable_qty: string
  unit_price: string
  tax_rate: string
  amount: string
  gross_amount: string
  base_amount: string
  base_gross_amount: string
  return_qty: string
}

const authStore = useAuthStore()
const canReturn = computed(() => authStore.hasPermission('purchase.return.write'))

const receipts = ref<PurchaseReceipt[]>([])
const loading = ref(false)
const orderId = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const detailVisible = ref(false)
const detail = ref<PurchaseReceipt | null>(null)

const returnVisible = ref(false)
const returning = ref(false)
const returnTarget = ref<PurchaseReceipt | null>(null)
const returnForm = reactive({
  return_date: today(),
  reason: '',
  remark: '',
  lines: [] as ReturnFormLine[]
})

function today() {
  const now = new Date()
  const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 10)
}

async function load() {
  loading.value = true
  try {
    const result = await purchaseApi.receipts({
      order_id: orderId.value,
      page: page.value,
      page_size: pageSize.value
    })
    receipts.value = result.items
    total.value = result.total
    if (detailVisible.value && detail.value) {
      const currentID = detail.value.id
      detail.value = receipts.value.find((item) => item.id === currentID) ?? detail.value
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载采购收货失败')
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function resetSearch() {
  orderId.value = ''
  page.value = 1
  load()
}

function handleSizeChange() {
  page.value = 1
  load()
}

function openDetail(row: PurchaseReceipt) {
  detail.value = row
  detailVisible.value = true
}

function hasReturnable(row: PurchaseReceipt) {
  return row.lines.some((line) => Number(line.returnable_qty) > 0)
}

function openReturn(row: PurchaseReceipt) {
  returnTarget.value = row
  returnForm.return_date = today()
  returnForm.reason = ''
  returnForm.remark = ''
  returnForm.lines = row.lines
    .filter((line) => Number(line.returnable_qty) > 0)
    .map(toReturnFormLine)
  returnVisible.value = true
}

function toReturnFormLine(line: PurchaseReceiptLine): ReturnFormLine {
  return {
    receipt_line_id: line.id,
    material_name: line.material_name,
    quantity: line.quantity,
    returned_qty: line.returned_qty,
    returnable_qty: line.returnable_qty,
    unit_price: line.unit_price,
    tax_rate: line.tax_rate,
    amount: line.amount,
    gross_amount: line.gross_amount,
    base_amount: line.base_amount,
    base_gross_amount: line.base_gross_amount,
    return_qty: ''
  }
}

function round4(value: number) {
  return Math.round(value * 10000) / 10000
}

function unitCost(line: ReturnFormLine) {
  const quantity = Number(line.quantity)
  if (!Number.isFinite(quantity) || quantity <= 0) {
    return 0
  }
  return round4(Number(line.base_amount) / quantity)
}

function unitCostText(line: ReturnFormLine) {
  return unitCost(line).toFixed(4)
}

const returnPreview = computed(() => {
  let net = 0
  let tax = 0
  let gross = 0
  let cost = 0
  for (const line of returnForm.lines) {
    const returnQty = Number(line.return_qty)
    const quantity = Number(line.quantity)
    const returnedQty = Number(line.returned_qty)
    if (!Number.isFinite(returnQty) || returnQty <= 0 || !Number.isFinite(quantity) || quantity <= 0) {
      continue
    }
    const amount = Number(line.amount)
    const grossAmount = Number(line.gross_amount)
    const baseAmount = Number(line.base_amount)

    const priorNet = round4((amount * returnedQty) / quantity)
    const newNet = round4((amount * (returnedQty + returnQty)) / quantity)
    const netDelta = newNet - priorNet

    const priorGross = round4((grossAmount * returnedQty) / quantity)
    const newGross = round4((grossAmount * (returnedQty + returnQty)) / quantity)
    const grossDelta = newGross - priorGross
    const taxDelta = grossDelta - netDelta

    const priorCost = round4((baseAmount * returnedQty) / quantity)
    const newCost = round4((baseAmount * (returnedQty + returnQty)) / quantity)
    const costDelta = newCost - priorCost

    net += netDelta
    tax += taxDelta
    gross += grossDelta
    cost += costDelta
  }
  return {
    net: net.toFixed(4),
    tax: tax.toFixed(4),
    gross: gross.toFixed(4),
    cost: cost.toFixed(4)
  }
})

async function saveReturn() {
  if (!returnTarget.value) {
    return
  }
  const lines: CreatePurchaseReturnLineInput[] = []
  for (const line of returnForm.lines) {
    if (line.return_qty.trim() === '') {
      continue
    }
    const quantity = Number(line.return_qty)
    if (!Number.isFinite(quantity) || quantity <= 0) {
      ElMessage.warning(`${line.material_name} 退货数量必须大于 0`)
      return
    }
    if (quantity > Number(line.returnable_qty)) {
      ElMessage.warning(`${line.material_name} 退货数量不能超过可退数量`)
      return
    }
    lines.push({ receipt_line_id: line.receipt_line_id, quantity: line.return_qty })
  }
  if (lines.length === 0) {
    ElMessage.warning('请填写至少一行退货数量')
    return
  }

  returning.value = true
  try {
    await purchaseApi.returns.create(returnTarget.value.id, {
      return_date: returnForm.return_date,
      reason: returnForm.reason,
      remark: returnForm.remark,
      lines
    })
    ElMessage.success('采购退货已创建')
    returnVisible.value = false
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '创建采购退货失败')
  } finally {
    returning.value = false
  }
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

.return-hint {
  margin: 0 0 16px;
  color: #475569;
}

.return-preview {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 20px;
  align-items: center;
  margin-top: 14px;
  color: #334155;
  font-size: 13px;
}

.return-preview__hint {
  color: #94a3b8;
}
</style>
