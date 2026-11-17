<template>
  <div>
    <PageHeader title="收付款" description="登记收款/付款，并核销到对应的应收/应付发票。">
      <el-button v-if="canWrite" type="primary" @click="openCreate">新建收付款</el-button>
    </PageHeader>

    <el-card shadow="never">
      <div class="toolbar">
        <el-input
          v-model="keyword"
          placeholder="搜索单号、参考号或往来单位"
          clearable
          class="toolbar__search"
          @keyup.enter="search"
        />
        <el-select v-model="typeFilter" placeholder="全部类型" clearable class="toolbar__filter" @change="search">
          <el-option label="收款" value="receipt" />
          <el-option label="付款" value="payment" />
        </el-select>
        <el-select v-model="statusFilter" placeholder="全部状态" clearable class="toolbar__filter" @change="search">
          <el-option label="未核销" value="unallocated" />
          <el-option label="部分核销" value="partially_allocated" />
          <el-option label="已核销" value="allocated" />
        </el-select>
        <el-button type="primary" plain @click="search">查询</el-button>
        <el-button @click="resetSearch">重置</el-button>
      </div>

      <el-table v-loading="loading" :data="payments" border stripe>
        <el-table-column prop="code" label="单号" min-width="150" />
        <el-table-column prop="payment_type" label="类型" width="80">
          <template #default="{ row }">
            <el-tag :type="row.payment_type === 'receipt' ? 'success' : 'warning'" size="small">
              {{ row.payment_type === 'receipt' ? '收款' : '付款' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="partner_name" label="往来单位" min-width="150" />
        <el-table-column prop="payment_date" label="日期" width="110" />
        <el-table-column prop="currency" label="币种" width="70" />
        <el-table-column prop="amount" label="金额" width="120" align="right" />
        <el-table-column prop="allocated_amount" label="已核销" width="120" align="right" />
        <el-table-column prop="unallocated_amount" label="未核销" width="120" align="right" />
        <el-table-column prop="status" label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="paymentStatusType(row.status)" size="small">{{ paymentStatusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="method" label="方式" width="90" />
        <el-table-column label="操作" width="130" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="canWrite && row.status !== 'allocated'"
              link
              type="primary"
              @click="openAllocate(row)"
            >
              核销
            </el-button>
            <el-button v-else link type="primary" disabled>已核销</el-button>
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

    <el-dialog v-model="createVisible" title="新建收付款" width="620px">
      <el-form :model="form" label-width="90px">
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="类型" required>
              <el-radio-group v-model="form.payment_type" @change="form.partner_id = ''">
                <el-radio-button label="receipt">收款</el-radio-button>
                <el-radio-button label="payment">付款</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="日期" required>
              <el-date-picker v-model="form.payment_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="往来单位" required>
              <el-select v-model="form.partner_id" filterable style="width: 100%" placeholder="选择往来单位">
                <el-option v-for="partner in selectablePartners" :key="partner.id" :label="partner.name" :value="partner.id" />
              </el-select>
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
            <el-form-item label="金额" required>
              <el-input v-model="form.amount" placeholder="0.0000" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="方式">
              <el-select v-model="form.method" style="width: 100%">
                <el-option label="银行" value="bank" />
                <el-option label="现金" value="cash" />
                <el-option label="其他" value="other" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="参考号">
              <el-input v-model="form.reference" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="备注">
              <el-input v-model="form.remark" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="savePayment">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="allocateVisible" title="收付款核销" width="820px">
      <template v-if="allocateTarget">
        <p class="allocate-hint">
          {{ allocateTarget.code }} · {{ allocateTarget.payment_type === 'receipt' ? '收款' : '付款' }} ·
          {{ allocateTarget.partner_name }} · 未核销 {{ allocateTarget.unallocated_amount }}
        </p>
        <el-table :data="allocateLines" border>
          <el-table-column prop="code" label="发票号" min-width="150" />
          <el-table-column prop="source_code" label="来源单号" min-width="140" />
          <el-table-column prop="due_date" label="到期日" width="110" />
          <el-table-column prop="outstanding_amount" label="未核销" width="120" align="right" />
          <el-table-column label="本次核销" width="160">
            <template #default="{ row }">
              <el-input v-model="row.amount" placeholder="0.0000" />
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="allocateLines.length === 0" description="没有可核销的发票" :image-size="80" />
        <div class="allocate-total">本次合计：{{ allocateTotal }}</div>
      </template>
      <template #footer>
        <el-button @click="allocateVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" :disabled="allocateLines.length === 0" @click="saveAllocate">
          确认核销
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import {
  financeApi,
  masterDataApi,
  type Invoice,
  type Partner,
  type Payment
} from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

interface AllocateLine {
  id: string
  code: string
  source_code: string
  due_date: string
  outstanding_amount: string
  amount: string
}

const authStore = useAuthStore()
const canWrite = computed(() => authStore.hasPermission('finance.payment.write'))

const payments = ref<Payment[]>([])
const partners = ref<Partner[]>([])
const loading = ref(false)
const saving = ref(false)
const keyword = ref('')
const typeFilter = ref('')
const statusFilter = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const createVisible = ref(false)
const form = reactive({
  payment_type: 'receipt',
  partner_id: '',
  payment_date: today(),
  currency: 'CNY',
  amount: '',
  method: 'bank',
  reference: '',
  remark: ''
})

const allocateVisible = ref(false)
const allocateTarget = ref<Payment | null>(null)
const allocateLines = ref<AllocateLine[]>([])

const selectablePartners = computed(() => {
  const type = form.payment_type === 'receipt' ? 'customer' : 'supplier'
  return partners.value.filter(
    (partner) => partner.status === 'active' && (partner.type === type || partner.type === 'both')
  )
})

const allocateTotal = computed(() => {
  return allocateLines.value
    .reduce((sum, line) => sum + Number(line.amount || '0'), 0)
    .toFixed(4)
})

function today() {
  const now = new Date()
  const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 10)
}

function paymentStatusLabel(status: string) {
  if (status === 'allocated') return '已核销'
  if (status === 'partially_allocated') return '部分核销'
  return '未核销'
}

function paymentStatusType(status: string) {
  if (status === 'allocated') return 'success'
  if (status === 'partially_allocated') return 'warning'
  return 'info'
}

async function load() {
  loading.value = true
  try {
    const result = await financeApi.payments({
      payment_type: typeFilter.value,
      status: statusFilter.value,
      keyword: keyword.value,
      page: page.value,
      page_size: pageSize.value
    })
    payments.value = result.items
    total.value = result.total
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载收付款失败')
  } finally {
    loading.value = false
  }
}

async function loadPartners() {
  try {
    const result = await masterDataApi.partners.list({ page_size: 200 })
    partners.value = result.items
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载往来单位失败')
  }
}

function search() {
  page.value = 1
  load()
}

function resetSearch() {
  keyword.value = ''
  typeFilter.value = ''
  statusFilter.value = ''
  page.value = 1
  load()
}

function handleSizeChange() {
  page.value = 1
  load()
}

function openCreate() {
  Object.assign(form, {
    payment_type: 'receipt',
    partner_id: '',
    payment_date: today(),
    currency: 'CNY',
    amount: '',
    method: 'bank',
    reference: '',
    remark: ''
  })
  createVisible.value = true
}

async function savePayment() {
  if (!form.partner_id || !form.payment_date || !form.amount) {
    ElMessage.warning('请填写往来单位、日期和金额')
    return
  }
  const amount = Number(form.amount)
  if (!Number.isFinite(amount) || amount <= 0) {
    ElMessage.warning('金额必须是正数')
    return
  }

  saving.value = true
  try {
    await financeApi.createPayment({
      payment_type: form.payment_type,
      partner_id: form.partner_id,
      payment_date: form.payment_date,
      currency: form.currency,
      amount: form.amount,
      method: form.method,
      reference: form.reference,
      remark: form.remark
    })
    ElMessage.success('收付款已创建')
    createVisible.value = false
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '创建收付款失败')
  } finally {
    saving.value = false
  }
}

async function openAllocate(payment: Payment) {
  try {
    const invoiceType = payment.payment_type === 'receipt' ? 'receivable' : 'payable'
    const result = await financeApi.invoices({
      invoice_type: invoiceType,
      partner_id: payment.partner_id,
      page_size: 200
    })
    const outstandingInvoices = result.items.filter(
      (invoice) => invoice.status !== 'paid' && Number(invoice.outstanding_amount) > 0
    )
    let remaining = Number(payment.unallocated_amount)
    allocateLines.value = outstandingInvoices.map((invoice: Invoice) => {
      const outstanding = Number(invoice.outstanding_amount)
      const amount = Math.min(remaining, outstanding)
      remaining = Math.max(0, remaining - amount)
      return {
        id: invoice.id,
        code: invoice.code,
        source_code: invoice.source_code,
        due_date: invoice.due_date,
        outstanding_amount: invoice.outstanding_amount,
        amount: amount > 0 ? amount.toFixed(4) : '0'
      }
    })
    allocateTarget.value = payment
    allocateVisible.value = true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载可核销发票失败')
  }
}

async function saveAllocate() {
  if (!allocateTarget.value) {
    return
  }
  const allocations = allocateLines.value
    .filter((line) => Number(line.amount) > 0)
    .map((line) => ({ invoice_id: line.id, amount: line.amount }))
  if (allocations.length === 0) {
    ElMessage.warning('请填写本次核销金额')
    return
  }
  if (Number(allocateTotal.value) > Number(allocateTarget.value.unallocated_amount)) {
    ElMessage.warning('核销合计不能超过收付款未核销金额')
    return
  }

  saving.value = true
  try {
    await financeApi.allocate(allocateTarget.value.id, allocations)
    ElMessage.success('核销成功')
    allocateVisible.value = false
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '核销失败')
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  await Promise.all([load(), loadPartners()])
})
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

.toolbar__filter {
  width: 140px;
}

.pagination {
  margin-top: 16px;
  justify-content: flex-end;
}

.allocate-hint {
  margin: 0 0 16px;
  color: #475569;
}

.allocate-total {
  margin-top: 12px;
  text-align: right;
  color: #1e293b;
  font-weight: 600;
}
</style>
