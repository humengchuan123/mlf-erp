<template>
  <div>
    <PageHeader title="会计凭证" description="手工凭证录入、过账和反过账；业务单据自动生成的凭证也会显示在这里。">
      <el-button v-if="canWrite" type="primary" @click="openCreate">新建凭证</el-button>
    </PageHeader>

    <el-card shadow="never">
      <div class="toolbar">
        <el-input v-model="keyword" placeholder="搜索凭证号、来源单号或摘要" clearable class="toolbar__search" @keyup.enter="search" />
        <el-select v-model="statusFilter" placeholder="全部状态" clearable class="toolbar__filter">
          <el-option label="草稿" value="draft" />
          <el-option label="已过账" value="posted" />
          <el-option label="已反过账" value="reversed" />
        </el-select>
        <el-select v-model="typeFilter" placeholder="全部类型" clearable class="toolbar__filter">
          <el-option v-for="(label, value) in typeLabels" :key="value" :label="label" :value="value" />
        </el-select>
        <el-date-picker v-model="dateRange" type="daterange" value-format="YYYY-MM-DD" start-placeholder="开始日期" end-placeholder="结束日期" />
        <el-button type="primary" plain @click="search">查询</el-button>
        <el-button @click="resetSearch">重置</el-button>
      </div>

      <el-table v-loading="loading" :data="vouchers" border stripe>
        <el-table-column prop="code" label="凭证号" min-width="160" />
        <el-table-column prop="voucher_date" label="日期" width="110" />
        <el-table-column prop="voucher_type" label="类型" width="110">
          <template #default="{ row }">{{ typeLabels[row.voucher_type] ?? row.voucher_type }}</template>
        </el-table-column>
        <el-table-column prop="source_code" label="来源单号" min-width="150" />
        <el-table-column prop="memo" label="摘要" min-width="180" />
        <el-table-column prop="total_amount" label="金额" width="130" align="right" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">{{ statusLabels[row.status] ?? row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openDetail(row)">详情</el-button>
            <el-button v-if="canWrite && row.status === 'draft'" link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button v-if="canPost && row.status === 'draft'" link type="success" @click="postVoucher(row)">过账</el-button>
            <el-button
              v-if="canReverse && row.status === 'posted' && row.voucher_type === 'manual'"
              link
              type="warning"
              @click="reverseVoucher(row)"
            >
              反过账
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

    <el-dialog v-model="dialogVisible" :title="editingVoucher ? '编辑凭证' : '新建凭证'" width="980px">
      <el-form :model="form" label-width="80px">
        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item label="凭证日期" required>
              <el-date-picker v-model="form.voucher_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="16">
            <el-form-item label="摘要">
              <el-input v-model="form.memo" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-table :data="form.lines" border>
          <el-table-column label="科目" min-width="220">
            <template #default="{ row }">
              <el-select v-model="row.account_id" filterable style="width: 100%" placeholder="选择末级科目">
                <el-option
                  v-for="account in accountOptions"
                  :key="account.id"
                  :label="`${account.code} ${account.name}`"
                  :value="account.id"
                />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="摘要" min-width="150">
            <template #default="{ row }">
              <el-input v-model="row.summary" />
            </template>
          </el-table-column>
          <el-table-column label="借方" width="130">
            <template #default="{ row }">
              <el-input v-model="row.debit" placeholder="0.0000" />
            </template>
          </el-table-column>
          <el-table-column label="贷方" width="130">
            <template #default="{ row }">
              <el-input v-model="row.credit" placeholder="0.0000" />
            </template>
          </el-table-column>
          <el-table-column width="70">
            <template #default="{ $index }">
              <el-button link type="danger" @click="removeLine($index)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
        <div class="line-actions">
          <el-button plain @click="addLine">添加分录</el-button>
          <span>借方合计：{{ totalDebit }}</span>
          <span>贷方合计：{{ totalCredit }}</span>
          <span :class="{ 'balance-error': !balanced }">差额：{{ difference }}</span>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveVoucher">保存草稿</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="detailVisible" title="凭证详情" size="760px">
      <template v-if="detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="凭证号">{{ detail.code }}</el-descriptions-item>
          <el-descriptions-item label="日期">{{ detail.voucher_date }}</el-descriptions-item>
          <el-descriptions-item label="类型">{{ typeLabels[detail.voucher_type] ?? detail.voucher_type }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ statusLabels[detail.status] ?? detail.status }}</el-descriptions-item>
          <el-descriptions-item label="来源单号">{{ detail.source_code || '-' }}</el-descriptions-item>
          <el-descriptions-item label="金额">{{ detail.total_amount }}</el-descriptions-item>
          <el-descriptions-item label="摘要" :span="2">{{ detail.memo || '-' }}</el-descriptions-item>
          <el-descriptions-item label="过账时间" :span="2">{{ detail.posted_at ? formatDate(detail.posted_at) : '-' }}</el-descriptions-item>
        </el-descriptions>

        <el-table :data="detail.lines" border class="detail-table">
          <el-table-column prop="line_no" label="#" width="60" />
          <el-table-column prop="account_code" label="科目编码" width="110" />
          <el-table-column prop="account_name" label="科目名称" min-width="160" />
          <el-table-column prop="summary" label="摘要" min-width="150" />
          <el-table-column prop="debit" label="借方" width="120" align="right" />
          <el-table-column prop="credit" label="贷方" width="120" align="right" />
        </el-table>

        <div class="drawer-actions">
          <el-button v-if="canPost && detail.status === 'draft'" type="success" @click="postVoucher(detail)">过账</el-button>
          <el-button
            v-if="canReverse && detail.status === 'posted' && detail.voucher_type === 'manual'"
            type="warning"
            @click="reverseVoucher(detail)"
          >
            反过账
          </el-button>
          <el-tag v-if="detail.voucher_type !== 'manual'" type="info" size="small">自动凭证请通过业务反向流程处理</el-tag>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import {
  glApi,
  type GlAccount,
  type GlVoucher,
  type GlVoucherInput
} from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

interface VoucherFormLine {
  account_id: string
  summary: string
  debit: string
  credit: string
}

const authStore = useAuthStore()
const canWrite = computed(() => authStore.hasPermission('finance.voucher.write'))
const canPost = computed(() => authStore.hasPermission('finance.voucher.post'))
const canReverse = computed(() => authStore.hasPermission('finance.voucher.reverse'))

const typeLabels: Record<string, string> = {
  manual: '手工',
  sales_shipment: '销售发货',
  purchase_receipt: '采购收货',
  receipt_payment: '收款',
  supplier_payment: '付款',
  reversal: '反过账'
}
const statusLabels: Record<string, string> = {
  draft: '草稿',
  posted: '已过账',
  reversed: '已反过账'
}

const vouchers = ref<GlVoucher[]>([])
const accounts = ref<GlAccount[]>([])
const loading = ref(false)
const saving = ref(false)
const keyword = ref('')
const statusFilter = ref('')
const typeFilter = ref('')
const dateRange = ref<[string, string] | null>(null)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const dialogVisible = ref(false)
const editingVoucher = ref<GlVoucher | null>(null)
const form = reactive({
  voucher_date: today(),
  memo: '',
  lines: [] as VoucherFormLine[]
})

const detailVisible = ref(false)
const detail = ref<GlVoucher | null>(null)

const accountOptions = computed(() => accounts.value.filter((item) => item.is_leaf && item.status === 'active'))
const totalDebit = computed(() => sum(form.lines.map((line) => line.debit)).toFixed(4))
const totalCredit = computed(() => sum(form.lines.map((line) => line.credit)).toFixed(4))
const difference = computed(() => (Number(totalDebit.value) - Number(totalCredit.value)).toFixed(4))
const balanced = computed(() => Number(difference.value) === 0 && Number(totalDebit.value) > 0)

function today() {
  const now = new Date()
  const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 10)
}

function sum(values: string[]) {
  return values.reduce((total, value) => total + Number(value || '0'), 0)
}

function statusType(status: string) {
  if (status === 'posted') return 'success'
  if (status === 'reversed') return 'warning'
  return 'info'
}

function formatDate(value: string) {
  return value ? new Date(value).toLocaleString() : '-'
}

async function load() {
  loading.value = true
  try {
    const result = await glApi.vouchers.list({
      status: statusFilter.value,
      voucher_type: typeFilter.value,
      keyword: keyword.value,
      start_date: dateRange.value?.[0],
      end_date: dateRange.value?.[1],
      page: page.value,
      page_size: pageSize.value
    })
    vouchers.value = result.items
    total.value = result.total
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载凭证失败')
  } finally {
    loading.value = false
  }
}

async function loadAccounts() {
  try {
    accounts.value = await glApi.accounts.list()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载科目失败')
  }
}

function search() {
  page.value = 1
  load()
}

function resetSearch() {
  keyword.value = ''
  statusFilter.value = ''
  typeFilter.value = ''
  dateRange.value = null
  page.value = 1
  load()
}

function handleSizeChange() {
  page.value = 1
  load()
}

function factoryLine(): VoucherFormLine {
  return { account_id: '', summary: '', debit: '', credit: '' }
}

function openCreate() {
  editingVoucher.value = null
  Object.assign(form, { voucher_date: today(), memo: '', lines: [factoryLine(), factoryLine()] })
  dialogVisible.value = true
}

async function openEdit(row: GlVoucher) {
  try {
    const voucher = await glApi.vouchers.get(row.id)
    editingVoucher.value = voucher
    Object.assign(form, {
      voucher_date: voucher.voucher_date,
      memo: voucher.memo,
      lines: voucher.lines.map((line) => ({
        account_id: line.account_id,
        summary: line.summary,
        debit: line.debit,
        credit: line.credit
      }))
    })
    dialogVisible.value = true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载凭证失败')
  }
}

function addLine() {
  form.lines.push(factoryLine())
}

function removeLine(index: number) {
  form.lines.splice(index, 1)
}

async function saveVoucher() {
  if (!form.voucher_date || form.lines.length === 0) {
    ElMessage.warning('请填写日期和至少一条分录')
    return
  }
  if (!balanced.value) {
    ElMessage.warning('借贷必须平衡且金额大于 0')
    return
  }
  for (const line of form.lines) {
    if (!line.account_id) {
      ElMessage.warning('请选择每条分录的科目')
      return
    }
    if (!line.debit && !line.credit) {
      ElMessage.warning('每条分录必须填写借方或贷方金额')
      return
    }
  }

  saving.value = true
  try {
    const input: GlVoucherInput = {
      voucher_date: form.voucher_date,
      memo: form.memo,
      lines: form.lines.map((line) => ({
        account_id: line.account_id,
        summary: line.summary,
        debit: line.debit || '0',
        credit: line.credit || '0'
      }))
    }
    if (editingVoucher.value) {
      await glApi.vouchers.update(editingVoucher.value.id, input)
      ElMessage.success('凭证已更新')
    } else {
      await glApi.vouchers.create(input)
      ElMessage.success('凭证已保存为草稿')
    }
    dialogVisible.value = false
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存凭证失败')
  } finally {
    saving.value = false
  }
}

async function openDetail(row: GlVoucher) {
  try {
    detail.value = await glApi.vouchers.get(row.id)
    detailVisible.value = true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载凭证详情失败')
  }
}

async function postVoucher(voucher: GlVoucher) {
  try {
    await ElMessageBox.confirm(`确认过账凭证 ${voucher.code}？`, '过账', { type: 'warning' })
  } catch {
    return
  }
  try {
    await glApi.vouchers.post(voucher.id)
    ElMessage.success('凭证已过账')
    await load()
    if (detailVisible.value && detail.value?.id === voucher.id) {
      detail.value = await glApi.vouchers.get(voucher.id)
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '过账失败')
  }
}

async function reverseVoucher(voucher: GlVoucher) {
  try {
    await ElMessageBox.confirm(`确认反过账凭证 ${voucher.code}？系统会生成一张反向凭证。`, '反过账', {
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    await glApi.vouchers.reverse(voucher.id)
    ElMessage.success('凭证已反过账')
    await load()
    if (detailVisible.value && detail.value?.id === voucher.id) {
      detail.value = await glApi.vouchers.get(voucher.id)
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '反过账失败')
  }
}

onMounted(async () => {
  await Promise.all([load(), loadAccounts()])
})
</script>

<style scoped>
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
}

.toolbar__search {
  width: 260px;
}

.toolbar__filter {
  width: 130px;
}

.pagination {
  margin-top: 16px;
  justify-content: flex-end;
}

.line-actions {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-top: 12px;
  color: #475569;
}

.balance-error {
  color: #dc2626;
  font-weight: 600;
}

.detail-table {
  margin-top: 16px;
}

.drawer-actions {
  display: flex;
  gap: 8px;
  margin-top: 16px;
}
</style>
