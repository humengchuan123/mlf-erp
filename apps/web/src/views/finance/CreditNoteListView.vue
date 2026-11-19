<template>
  <div>
    <PageHeader title="贷项单" description="查看销售/采购退货生成的贷项单、抵用与退款进度。">
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
        <el-select v-model="creditTypeFilter" placeholder="全部类型" clearable class="toolbar__status">
          <el-option label="销售退货" value="sales_return" />
          <el-option label="采购退货" value="purchase_return" />
        </el-select>
        <el-select v-model="statusFilter" placeholder="全部状态" clearable class="toolbar__status">
          <el-option label="未核销" value="open" />
          <el-option label="已核销" value="applied" />
        </el-select>
        <el-button @click="resetSearch">重置</el-button>
      </div>

      <el-table v-loading="loading" :data="creditNotes" border stripe>
        <el-table-column prop="code" label="贷项单号" min-width="150" />
        <el-table-column prop="credit_type" label="类型" width="100">
          <template #default="{ row }">{{ creditTypeLabel(row.credit_type) }}</template>
        </el-table-column>
        <el-table-column prop="partner_name" label="往来单位" min-width="150" />
        <el-table-column prop="invoice_code" label="原发票号" min-width="150">
          <template #default="{ row }">{{ row.invoice_code || '-' }}</template>
        </el-table-column>
        <el-table-column prop="note_date" label="日期" width="110" />
        <el-table-column prop="currency" label="币种" width="70" />
        <el-table-column prop="amount" label="贷项金额" width="120" align="right" />
        <el-table-column prop="applied_amount" label="已核销" width="120" align="right" />
        <el-table-column prop="refunded_amount" label="已退款" width="120" align="right" />
        <el-table-column prop="remaining_amount" label="剩余" width="120" align="right" />
        <el-table-column prop="status" label="状态" width="130">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">{{ statusLabel(row) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openDetail(row)">详情</el-button>
            <el-button
              v-if="isActionable(row) && canApply"
              link
              type="success"
              @click="openAllocate(row)"
            >
              抵用
            </el-button>
            <el-button
              v-if="isActionable(row) && canRefund"
              link
              type="warning"
              @click="openRefund(row)"
            >
              退款
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

    <el-drawer v-model="detailVisible" title="贷项单详情" size="920px">
      <template v-if="detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="贷项单号">{{ detail.code }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="statusType(detail.status)" size="small">{{ statusLabel(detail) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="贷项类型">{{ creditTypeLabel(detail.credit_type) }}</el-descriptions-item>
          <el-descriptions-item label="来源类型">{{ sourceTypeLabel(detail.source_type) }}</el-descriptions-item>
          <el-descriptions-item label="来源退货单">{{ detail.source_code || '-' }}</el-descriptions-item>
          <el-descriptions-item label="原发票号">{{ detail.invoice_code || '-' }}</el-descriptions-item>
          <el-descriptions-item label="往来单位">{{ detail.partner_name }}</el-descriptions-item>
          <el-descriptions-item label="日期">{{ detail.note_date }}</el-descriptions-item>
          <el-descriptions-item label="币种/汇率">{{ detail.currency }} / {{ detail.exchange_rate }}</el-descriptions-item>
          <el-descriptions-item label="贷项金额">{{ detail.amount }}</el-descriptions-item>
          <el-descriptions-item label="本位币金额">{{ detail.base_amount }}</el-descriptions-item>
          <el-descriptions-item label="已核销金额">{{ detail.applied_amount }}</el-descriptions-item>
          <el-descriptions-item label="本位币已核销">{{ detail.applied_base_amount }}</el-descriptions-item>
          <el-descriptions-item label="已退款金额">{{ detail.refunded_amount }}</el-descriptions-item>
          <el-descriptions-item label="本位币已退款">{{ detail.refunded_base_amount }}</el-descriptions-item>
          <el-descriptions-item label="剩余金额">{{ detail.remaining_amount }}</el-descriptions-item>
          <el-descriptions-item label="本位币剩余">{{ detail.remaining_base_amount }}</el-descriptions-item>
          <el-descriptions-item label="退款凭证" :span="2">{{ refundVoucherCodes }}</el-descriptions-item>
          <el-descriptions-item label="原因" :span="2">{{ detail.reason || '-' }}</el-descriptions-item>
          <el-descriptions-item label="备注" :span="2">{{ detail.remark || '-' }}</el-descriptions-item>
        </el-descriptions>

        <el-divider content-position="left">抵用历史</el-divider>
        <el-table :data="detail.allocations ?? []" border>
          <el-table-column prop="invoice_code" label="发票号" min-width="160" />
          <el-table-column prop="invoice_date" label="发票日期" width="110" />
          <el-table-column prop="amount" label="抵用金额" width="130" align="right" />
          <el-table-column prop="base_amount" label="本位币金额" width="130" align="right" />
          <el-table-column prop="allocation_date" label="抵用日期" width="110" />
        </el-table>
        <el-empty
          v-if="(detail.allocations ?? []).length === 0"
          description="暂无抵用记录"
          :image-size="80"
        />

        <el-divider content-position="left">退款历史</el-divider>
        <el-table :data="detail.refunds ?? []" border>
          <el-table-column prop="code" label="退款单号" min-width="160" />
          <el-table-column prop="refund_date" label="退款日期" width="110" />
          <el-table-column prop="amount" label="退款金额" width="120" align="right" />
          <el-table-column prop="base_amount" label="本位币" width="120" align="right" />
          <el-table-column prop="cash_base_amount" label="现金本位币" width="130" align="right" />
          <el-table-column prop="fx_adjustment" label="汇兑调整" width="120" align="right" />
          <el-table-column prop="method" label="方式" width="80">
            <template #default="{ row }">{{ methodLabel(row.method) }}</template>
          </el-table-column>
          <el-table-column prop="voucher_code" label="凭证号" min-width="150" />
        </el-table>
        <el-empty
          v-if="(detail.refunds ?? []).length === 0"
          description="暂无退款记录"
          :image-size="80"
        />
      </template>
    </el-drawer>

    <el-dialog v-model="applyVisible" title="贷项抵用" width="980px">
      <template v-if="applyTarget">
        <p class="dialog-hint">
          {{ applyTarget.code }} · {{ applyTarget.partner_name }} · {{ applyTarget.currency }} ·
          剩余可抵 {{ applyTarget.remaining_amount }}
        </p>
        <el-form label-width="90px">
          <el-form-item label="抵用日期" required>
            <el-date-picker
              v-model="allocationDate"
              type="date"
              value-format="YYYY-MM-DD"
              style="width: 200px"
            />
          </el-form-item>
        </el-form>

        <el-table v-loading="candidatesLoading" :data="allocationLines" border>
          <el-table-column prop="code" label="发票号" min-width="150" />
          <el-table-column prop="invoice_date" label="发票日期" width="110" />
          <el-table-column prop="currency" label="币种" width="70" />
          <el-table-column prop="total_amount" label="含税总额" width="120" align="right" />
          <el-table-column prop="paid_amount" label="已收/已付" width="120" align="right" />
          <el-table-column prop="credited_amount" label="已冲" width="120" align="right" />
          <el-table-column prop="outstanding_amount" label="未付" width="120" align="right" />
          <el-table-column label="本次抵用" width="170">
            <template #default="{ row }">
              <el-input v-model="row.amount" placeholder="0.0000" />
            </template>
          </el-table-column>
        </el-table>
        <el-empty
          v-if="!candidatesLoading && allocationLines.length === 0"
          description="没有可抵用的发票"
          :image-size="80"
        />
        <div class="dialog-total">
          本次合计：{{ allocationTotal.toFixed(4) }} · 抵用后剩余：{{ allocationRemainingAfter }}
        </div>
      </template>
      <template #footer>
        <el-button @click="applyVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="saving"
          :disabled="candidatesLoading || allocationLines.length === 0"
          @click="saveAllocate"
        >
          确认抵用
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="refundVisible" title="贷项退款" width="680px">
      <template v-if="refundTarget">
        <p class="dialog-hint">
          {{ refundTarget.code }} · {{ refundTarget.partner_name }} · {{ refundTarget.currency }} ·
          剩余 {{ refundTarget.remaining_amount }}
        </p>
        <el-form :model="refundForm" label-width="110px">
          <el-form-item label="退款日期" required>
            <el-date-picker
              v-model="refundForm.refund_date"
              type="date"
              value-format="YYYY-MM-DD"
              style="width: 100%"
            />
          </el-form-item>
          <el-form-item label="退款金额" required>
            <el-input v-model="refundForm.amount" placeholder="0.0000" />
          </el-form-item>
          <el-form-item v-if="refundTarget.currency !== 'CNY'" label="汇率">
            <el-input v-model="refundForm.exchange_rate" placeholder="留空取退款日汇率" />
          </el-form-item>
          <el-form-item label="方式" required>
            <el-select v-model="refundForm.method" style="width: 100%">
              <el-option label="银行" value="bank" />
              <el-option label="现金" value="cash" />
            </el-select>
          </el-form-item>
          <el-form-item label="参考号">
            <el-input v-model="refundForm.reference" />
          </el-form-item>
          <el-form-item label="备注">
            <el-input v-model="refundForm.remark" />
          </el-form-item>
        </el-form>

        <el-descriptions :column="2" border>
          <el-descriptions-item label="贷项剩余">
            {{ refundTarget.remaining_amount }} / 本位币 {{ refundTarget.remaining_base_amount }}
          </el-descriptions-item>
          <el-descriptions-item label="退款金额">{{ refundAmount.toFixed(4) }}</el-descriptions-item>
          <el-descriptions-item label="预计贷项本位币">{{ refundCreditBase }}</el-descriptions-item>
          <el-descriptions-item label="预计现金本位币">
            {{ refundCashBase ?? '按退款日汇率' }}
          </el-descriptions-item>
          <el-descriptions-item label="预计汇兑调整" :span="2">
            {{ refundFxAdjustment ?? '-' }}
          </el-descriptions-item>
        </el-descriptions>
      </template>
      <template #footer>
        <el-button @click="refundVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveRefund">确认退款</el-button>
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
  type CreditNote,
  type CreditNoteListParams,
  type CreditNoteSummary,
  type Invoice
} from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

interface AllocationLine {
  invoice_id: string
  code: string
  invoice_date: string
  currency: string
  total_amount: string
  paid_amount: string
  credited_amount: string
  outstanding_amount: string
  amount: string
}

const authStore = useAuthStore()
const canApply = computed(() => authStore.hasPermission('finance.credit_note.apply'))
const canRefund = computed(() => authStore.hasPermission('finance.credit_note.refund'))

const creditNotes = ref<CreditNoteSummary[]>([])
const loading = ref(false)
const saving = ref(false)
const keyword = ref('')
const creditTypeFilter = ref<CreditNoteListParams['credit_type'] | ''>('')
const statusFilter = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const detailVisible = ref(false)
const detail = ref<CreditNote | null>(null)

const applyVisible = ref(false)
const applyTarget = ref<CreditNoteSummary | null>(null)
const candidatesLoading = ref(false)
const allocationDate = ref(today())
const allocationLines = ref<AllocationLine[]>([])

const refundVisible = ref(false)
const refundTarget = ref<CreditNoteSummary | null>(null)
const refundForm = reactive({
  refund_date: today(),
  amount: '',
  exchange_rate: '',
  method: 'bank',
  reference: '',
  remark: ''
})

const refundVoucherCodes = computed(() => {
  const codes = (detail.value?.refunds ?? [])
    .map((refund) => refund.voucher_code)
    .filter((code) => Boolean(code))
  return codes.length > 0 ? codes.join('、') : '-'
})

const allocationTotal = computed(() =>
  allocationLines.value.reduce((sum, line) => {
    const amount = Number(line.amount)
    return sum + (Number.isFinite(amount) && amount > 0 ? amount : 0)
  }, 0)
)

const allocationRemainingAfter = computed(() => {
  const remaining = Number(applyTarget.value?.remaining_amount ?? 0)
  return (remaining - allocationTotal.value).toFixed(4)
})

const refundAmount = computed(() => {
  const amount = Number(refundForm.amount)
  return Number.isFinite(amount) ? amount : 0
})

const refundCreditBase = computed(() => {
  if (!refundTarget.value || refundAmount.value <= 0) return '0.0000'
  const remaining = Number(refundTarget.value.remaining_amount)
  const remainingBase = Number(refundTarget.value.remaining_base_amount)
  if (!Number.isFinite(remaining) || remaining <= 0) return '0.0000'
  if (Math.abs(refundAmount.value - remaining) < 0.0000001) {
    return round4(remainingBase).toFixed(4)
  }
  return round4((remainingBase * refundAmount.value) / remaining).toFixed(4)
})

const refundCashBase = computed(() => {
  if (!refundTarget.value || refundAmount.value <= 0) return null
  if (refundTarget.value.currency === 'CNY') return refundAmount.value.toFixed(4)
  const rate = Number(refundForm.exchange_rate)
  if (!refundForm.exchange_rate || !Number.isFinite(rate) || rate <= 0) return null
  return round4(refundAmount.value * rate).toFixed(4)
})

const refundFxAdjustment = computed(() => {
  if (refundCashBase.value === null) return null
  return round4(Number(refundCashBase.value) - Number(refundCreditBase.value)).toFixed(4)
})

function today() {
  const now = new Date()
  const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 10)
}

function round4(value: number) {
  return Math.round((value + Number.EPSILON) * 10000) / 10000
}

async function load() {
  loading.value = true
  try {
    const result = await financeApi.creditNotes.list({
      credit_type: creditTypeFilter.value || undefined,
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
  creditTypeFilter.value = ''
  statusFilter.value = ''
  page.value = 1
  load()
}

function handleSizeChange() {
  page.value = 1
  load()
}

async function openDetail(row: CreditNoteSummary) {
  try {
    detail.value = await financeApi.creditNotes.get(row.id)
    detailVisible.value = true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载贷项单详情失败')
  }
}

async function refreshDetail() {
  if (!detailVisible.value || !detail.value) return
  try {
    detail.value = await financeApi.creditNotes.get(detail.value.id)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载贷项单详情失败')
  }
}

async function reloadAfterChange() {
  await load()
  await refreshDetail()
}

function isActionable(note: CreditNoteSummary) {
  return note.status === 'open' && Number(note.remaining_amount) > 0
}

function statusLabel(note: { status: string; refunded_amount?: string }) {
  if (note.status === 'applied') {
    return Number(note.refunded_amount ?? 0) > 0 ? '已退款/已核销' : '已核销'
  }
  if (note.status === 'open') return '未核销'
  return note.status
}

function statusType(status: string) {
  if (status === 'applied') return 'success'
  if (status === 'open') return 'warning'
  return 'info'
}

function creditTypeLabel(value: string) {
  if (value === 'sales_return') return '销售退货'
  if (value === 'purchase_return') return '采购退货'
  return value || '-'
}

function sourceTypeLabel(value: string) {
  if (value === 'sales_return') return '销售退货'
  if (value === 'purchase_return') return '采购退货'
  return value || '-'
}

function methodLabel(value: string) {
  if (value === 'bank') return '银行'
  if (value === 'cash') return '现金'
  return value || '-'
}

async function openAllocate(note: CreditNoteSummary) {
  applyTarget.value = note
  allocationDate.value = today()
  allocationLines.value = []
  applyVisible.value = true
  candidatesLoading.value = true
  try {
    const candidates = await financeApi.creditNotes.allocationCandidates(note.id)
    allocationLines.value = candidates
      .filter((invoice) => Number(invoice.outstanding_amount) > 0)
      .map((invoice: Invoice) => ({
        invoice_id: invoice.id,
        code: invoice.code,
        invoice_date: invoice.invoice_date,
        currency: invoice.currency,
        total_amount: invoice.total_amount,
        paid_amount: invoice.paid_amount,
        credited_amount: invoice.credited_amount,
        outstanding_amount: invoice.outstanding_amount,
        amount: ''
      }))
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载可抵用发票失败')
    applyVisible.value = false
  } finally {
    candidatesLoading.value = false
  }
}

function validateAllocation() {
  if (!applyTarget.value) return false
  if (applyTarget.value.status !== 'open' || Number(applyTarget.value.remaining_amount) <= 0) {
    ElMessage.warning('该贷项单已无剩余可抵用金额')
    return false
  }
  if (!allocationDate.value) {
    ElMessage.warning('请选择抵用日期')
    return false
  }
  if (allocationDate.value < applyTarget.value.note_date) {
    ElMessage.warning('抵用日期不能早于贷项单日期')
    return false
  }

  const invoiceIDs = new Set<string>()
  let total = 0
  for (const line of allocationLines.value) {
    if (!line.amount || !line.amount.trim()) continue
    const amount = Number(line.amount)
    if (!Number.isFinite(amount) || amount < 0) {
      ElMessage.warning(`发票 ${line.code} 的抵用金额无效`)
      return false
    }
    if (amount === 0) continue
    if (invoiceIDs.has(line.invoice_id)) {
      ElMessage.warning('同一张发票不能重复抵用')
      return false
    }
    invoiceIDs.add(line.invoice_id)
    if (amount > Number(line.outstanding_amount)) {
      ElMessage.warning(`发票 ${line.code} 抵用金额不能超过未付金额 ${line.outstanding_amount}`)
      return false
    }
    total += amount
  }

  if (invoiceIDs.size === 0) {
    ElMessage.warning('请至少填写一行抵用金额')
    return false
  }
  if (total > Number(applyTarget.value.remaining_amount)) {
    ElMessage.warning('抵用合计不能超过贷项单剩余金额')
    return false
  }
  return true
}

async function saveAllocate() {
  if (!applyTarget.value || !validateAllocation()) return
  const allocations = allocationLines.value
    .filter((line) => Number(line.amount) > 0)
    .map((line) => ({ invoice_id: line.invoice_id, amount: line.amount }))

  saving.value = true
  try {
    await financeApi.creditNotes.allocate(applyTarget.value.id, {
      allocation_date: allocationDate.value,
      allocations
    })
    ElMessage.success('抵用成功')
    applyVisible.value = false
    await reloadAfterChange()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '抵用失败')
  } finally {
    saving.value = false
  }
}

function openRefund(note: CreditNoteSummary) {
  refundTarget.value = note
  Object.assign(refundForm, {
    refund_date: today(),
    amount: note.remaining_amount,
    exchange_rate: '',
    method: 'bank',
    reference: '',
    remark: ''
  })
  refundVisible.value = true
}

function validateRefund() {
  if (!refundTarget.value) return false
  if (refundTarget.value.status !== 'open' || Number(refundTarget.value.remaining_amount) <= 0) {
    ElMessage.warning('该贷项单已无剩余可退款金额')
    return false
  }
  if (!refundForm.refund_date) {
    ElMessage.warning('请选择退款日期')
    return false
  }
  if (refundForm.refund_date < refundTarget.value.note_date) {
    ElMessage.warning('退款日期不能早于贷项单日期')
    return false
  }
  const amount = Number(refundForm.amount)
  if (!Number.isFinite(amount) || amount <= 0) {
    ElMessage.warning('退款金额必须是正数')
    return false
  }
  if (amount > Number(refundTarget.value.remaining_amount)) {
    ElMessage.warning('退款金额不能超过贷项单剩余金额')
    return false
  }
  if (!refundForm.method) {
    ElMessage.warning('请选择退款方式')
    return false
  }
  if (refundTarget.value.currency !== 'CNY' && refundForm.exchange_rate) {
    const rate = Number(refundForm.exchange_rate)
    if (!Number.isFinite(rate) || rate <= 0) {
      ElMessage.warning('汇率必须是正数')
      return false
    }
  }
  return true
}

async function saveRefund() {
  if (!refundTarget.value || !validateRefund()) return
  saving.value = true
  try {
    await financeApi.creditNotes.refund(refundTarget.value.id, {
      refund_date: refundForm.refund_date,
      amount: refundForm.amount,
      exchange_rate: refundForm.exchange_rate || undefined,
      method: refundForm.method,
      reference: refundForm.reference,
      remark: refundForm.remark
    })
    ElMessage.success('退款成功')
    refundVisible.value = false
    await reloadAfterChange()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '退款失败')
  } finally {
    saving.value = false
  }
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

.dialog-hint {
  margin: 0 0 16px;
  color: #475569;
}

.dialog-total {
  margin-top: 12px;
  text-align: right;
  color: #1e293b;
  font-weight: 600;
}
</style>
