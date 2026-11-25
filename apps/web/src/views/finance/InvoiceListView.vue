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
        <el-select v-model="statusFilter" placeholder="全部核销状态" clearable class="toolbar__status">
          <el-option label="未结清" value="open" />
          <el-option label="部分结清" value="partially_paid" />
          <el-option label="已结清" value="paid" />
        </el-select>
        <el-select v-model="vatStatusFilter" placeholder="全部开票状态" clearable class="toolbar__status">
          <el-option label="待开票" value="pending" />
          <el-option label="已开票" value="issued" />
          <el-option label="已作废" value="void" />
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
        <el-table-column prop="exchange_rate" label="汇率" width="100" align="right" />
        <el-table-column prop="net_amount" label="不含税" width="110" align="right" />
        <el-table-column prop="tax_amount" label="税额" width="100" align="right" />
        <el-table-column prop="total_amount" label="含税总额" width="120" align="right" />
        <el-table-column prop="base_total_amount" label="本位币总额" width="130" align="right" />
        <el-table-column prop="paid_amount" label="已核销" width="110" align="right" />
        <el-table-column prop="credited_amount" label="已抵扣" width="110" align="right" />
        <el-table-column prop="base_credited_amount" label="本位币已抵扣" width="130" align="right" />
        <el-table-column prop="outstanding_amount" label="未核销" width="110" align="right" />
        <el-table-column prop="vat_invoice_no" label="增值税发票号" min-width="170">
          <template #default="{ row }">{{ row.vat_invoice_no || '-' }}</template>
        </el-table-column>
        <el-table-column prop="vat_invoice_status" label="开票状态" width="100">
          <template #default="{ row }">
            <el-tag :type="vatStatusType(row.vat_invoice_status)" size="small">
              {{ vatStatusLabel(row.vat_invoice_status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="核销状态" width="100">
          <template #default="{ row }">
            <el-tag :type="invoiceStatusType(row.status)" size="small">{{ invoiceStatusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="170" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openDetail(row)">详情</el-button>
            <el-button
              v-if="canWrite && row.vat_invoice_status === 'pending'"
              link
              type="success"
              @click="openIssue(row)"
            >
              开票
            </el-button>
            <el-button
              v-if="canWrite && row.vat_invoice_status === 'issued'"
              link
              type="danger"
              @click="voidInvoice(row)"
            >
              作废
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

    <el-drawer v-model="detailVisible" title="发票详情" size="920px">
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
          <el-descriptions-item label="汇率">{{ detail.exchange_rate }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ invoiceStatusLabel(detail.status) }}</el-descriptions-item>
          <el-descriptions-item label="不含税金额">{{ detail.net_amount }}</el-descriptions-item>
          <el-descriptions-item label="税额">{{ detail.tax_amount }}</el-descriptions-item>
          <el-descriptions-item label="含税总额">{{ detail.total_amount }}</el-descriptions-item>
          <el-descriptions-item label="本位币含税总额">{{ detail.base_total_amount }}</el-descriptions-item>
          <el-descriptions-item label="本位币已核销">{{ detail.base_paid_amount }}</el-descriptions-item>
          <el-descriptions-item label="已核销">{{ detail.paid_amount }}</el-descriptions-item>
          <el-descriptions-item label="已抵扣">{{ detail.credited_amount }}</el-descriptions-item>
          <el-descriptions-item label="本位币已抵扣">{{ detail.base_credited_amount }}</el-descriptions-item>
          <el-descriptions-item label="未核销">{{ detail.outstanding_amount }}</el-descriptions-item>
          <el-descriptions-item label="开票状态">{{ vatStatusLabel(detail.vat_invoice_status) }}</el-descriptions-item>
          <el-descriptions-item label="增值税发票号">{{ detail.vat_invoice_no || '-' }}</el-descriptions-item>
          <el-descriptions-item label="发票类型">{{ vatTypeLabel(detail.vat_invoice_type) }}</el-descriptions-item>
          <el-descriptions-item label="开票日期">{{ detail.vat_invoice_date || '-' }}</el-descriptions-item>
          <el-descriptions-item label="购方税号">{{ detail.buyer_tax_no || '-' }}</el-descriptions-item>
          <el-descriptions-item label="销方税号">{{ detail.seller_tax_no || '-' }}</el-descriptions-item>
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

        <el-divider content-position="left">收付款反核销记录</el-divider>
        <el-table :data="detail.payment_allocation_reversals ?? []" border>
          <el-table-column prop="payment_code" label="收付款单号" min-width="160" />
          <el-table-column prop="payment_date" label="收付款日期" width="110" />
          <el-table-column prop="amount" label="反核销金额" width="120" align="right" />
          <el-table-column prop="base_amount" label="付款本位币" width="130" align="right" />
          <el-table-column prop="invoice_base_amount" label="发票本位币" width="130" align="right" />
          <el-table-column prop="reversal_date" label="反核销日期" width="110" />
          <el-table-column prop="reason" label="原因" min-width="150" />
          <el-table-column label="汇兑反核销凭证" min-width="150">
            <template #default="{ row }">{{ reversalVoucherCodes(row) }}</template>
          </el-table-column>
        </el-table>
        <el-empty
          v-if="(detail.payment_allocation_reversals ?? []).length === 0"
          description="暂无付款反核销记录"
          :image-size="80"
        />

        <el-divider content-position="left">贷项抵用记录</el-divider>
        <el-table :data="detail.credit_note_allocations ?? []" border>
          <el-table-column prop="credit_note_code" label="贷项单号" min-width="160" />
          <el-table-column prop="credit_type" label="贷项类型" width="110">
            <template #default="{ row }">{{ creditTypeLabel(row.credit_type) }}</template>
          </el-table-column>
          <el-table-column prop="note_date" label="单据日期" width="110" />
          <el-table-column prop="amount" label="抵用金额" width="130" align="right" />
          <el-table-column prop="base_amount" label="本位币金额" width="130" align="right" />
          <el-table-column prop="allocation_date" label="抵用日期" width="110" />
        </el-table>
        <el-empty
          v-if="(detail.credit_note_allocations ?? []).length === 0"
          description="暂无贷项抵用记录"
          :image-size="80"
        />
      </template>
    </el-drawer>

    <el-dialog v-model="issueVisible" title="开具增值税发票" width="560px">
      <el-form :model="issueForm" label-width="100px">
        <el-form-item label="发票号" required>
          <el-input v-model="issueForm.vat_invoice_no" placeholder="税控发票号" />
        </el-form-item>
        <el-form-item label="发票类型" required>
          <el-select v-model="issueForm.vat_invoice_type" style="width: 100%">
            <el-option label="增值税专用发票" value="special" />
            <el-option label="增值税普通发票" value="general" />
          </el-select>
        </el-form-item>
        <el-form-item label="开票日期" required>
          <el-date-picker v-model="issueForm.vat_invoice_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
        <el-form-item label="购方税号">
          <el-input v-model="issueForm.buyer_tax_no" />
        </el-form-item>
        <el-form-item label="销方税号">
          <el-input v-model="issueForm.seller_tax_no" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="issueVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveIssue">确认开票</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import { financeApi, type Invoice, type InvoiceDetail, type IssueInvoiceInput, type PaymentAllocationReversal } from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const canWrite = computed(() => authStore.hasPermission('finance.invoice.write'))

const invoices = ref<Invoice[]>([])
const loading = ref(false)
const saving = ref(false)
const invoiceType = ref('')
const statusFilter = ref('')
const vatStatusFilter = ref('')
const keyword = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const detailVisible = ref(false)
const detail = ref<InvoiceDetail | null>(null)
const issueVisible = ref(false)
const issueInvoiceID = ref('')
const issueForm = reactive<IssueInvoiceInput>({
  vat_invoice_no: '',
  vat_invoice_type: 'special',
  vat_invoice_date: today(),
  buyer_tax_no: '',
  seller_tax_no: ''
})

function today() {
  const now = new Date()
  const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 10)
}

async function load() {
  loading.value = true
  try {
    const result = await financeApi.invoices({
      invoice_type: invoiceType.value,
      status: statusFilter.value,
      vat_invoice_status: vatStatusFilter.value,
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
  vatStatusFilter.value = ''
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
  if (status === 'paid') return '已结清'
  if (status === 'partially_paid') return '部分结清'
  return '未结清'
}

function invoiceStatusType(status: string) {
  if (status === 'paid') return 'success'
  if (status === 'partially_paid') return 'warning'
  return 'info'
}

function vatStatusLabel(status: string) {
  if (status === 'issued') return '已开票'
  if (status === 'void') return '已作废'
  return '待开票'
}

function vatStatusType(status: string) {
  if (status === 'issued') return 'success'
  if (status === 'void') return 'info'
  return 'warning'
}

function vatTypeLabel(value: string) {
  if (value === 'special') return '专用发票'
  if (value === 'general') return '普通发票'
  return '-'
}

function creditTypeLabel(value: string) {
  if (value === 'sales_return') return '销售退货'
  if (value === 'purchase_return') return '采购退货'
  return value || '-'
}

function reversalVoucherCodes(reversal: PaymentAllocationReversal) {
  const codes = reversal.fx_reversal_voucher_codes ?? []
  return codes.length > 0 ? codes.join('、') : '-'
}

function openIssue(invoice: Invoice) {
  issueInvoiceID.value = invoice.id
  Object.assign(issueForm, {
    vat_invoice_no: '',
    vat_invoice_type: 'special',
    vat_invoice_date: today(),
    buyer_tax_no: '',
    seller_tax_no: ''
  })
  issueVisible.value = true
}

async function saveIssue() {
  if (!issueForm.vat_invoice_no || !issueForm.vat_invoice_date) {
    ElMessage.warning('请填写发票号和开票日期')
    return
  }
  saving.value = true
  try {
    await financeApi.issueInvoice(issueInvoiceID.value, issueForm)
    ElMessage.success('发票已开具')
    issueVisible.value = false
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '开票失败')
  } finally {
    saving.value = false
  }
}

async function voidInvoice(invoice: Invoice) {
  try {
    await ElMessageBox.confirm(`确认作废增值税发票 ${invoice.vat_invoice_no}？`, '作废发票', { type: 'warning' })
  } catch {
    return
  }
  try {
    await financeApi.voidInvoice(invoice.id)
    ElMessage.success('发票已作废')
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '作废发票失败')
  }
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
