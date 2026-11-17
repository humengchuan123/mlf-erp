<template>
  <div>
    <PageHeader title="财务报表" description="科目余额表、明细账、资产负债表和利润表。">
      <el-button type="primary" plain :loading="loading" @click="refresh">刷新</el-button>
    </PageHeader>

    <el-card shadow="never">
      <el-tabs v-model="activeTab" @tab-change="onTabChange">
        <el-tab-pane label="科目余额表" name="trial">
          <div class="toolbar">
            <el-date-picker v-model="trialRange" type="daterange" value-format="YYYY-MM-DD" start-placeholder="开始日期" end-placeholder="结束日期" />
            <el-button type="primary" plain @click="loadTrial">查询</el-button>
          </div>
          <el-table v-loading="loading" :data="trial?.rows ?? []" border stripe>
            <el-table-column prop="code" label="科目编码" width="120" />
            <el-table-column prop="name" label="科目名称" min-width="180" />
            <el-table-column prop="type" label="类型" width="90">
              <template #default="{ row }">{{ typeLabel(row.type) }}</template>
            </el-table-column>
            <el-table-column prop="opening" label="期初" width="130" align="right" />
            <el-table-column prop="period_debit" label="本期借方" width="130" align="right" />
            <el-table-column prop="period_credit" label="本期贷方" width="130" align="right" />
            <el-table-column prop="closing" label="期末" width="130" align="right" />
          </el-table>
          <div class="summary">
            <span>本期借方合计：{{ trial?.total_debit ?? '-' }}</span>
            <span>本期贷方合计：{{ trial?.total_credit ?? '-' }}</span>
          </div>
        </el-tab-pane>

        <el-tab-pane label="明细账" name="ledger">
          <div class="toolbar">
            <el-select v-model="ledgerAccountID" filterable placeholder="选择科目" style="width: 260px">
              <el-option
                v-for="account in accountOptions"
                :key="account.id"
                :label="`${account.code} ${account.name}`"
                :value="account.id"
              />
            </el-select>
            <el-date-picker v-model="ledgerRange" type="daterange" value-format="YYYY-MM-DD" start-placeholder="开始日期" end-placeholder="结束日期" />
            <el-button type="primary" plain :disabled="!ledgerAccountID" @click="loadLedger">查询</el-button>
          </div>
          <template v-if="ledger">
            <el-descriptions :column="3" border>
              <el-descriptions-item label="科目">{{ ledger.account.code }} {{ ledger.account.name }}</el-descriptions-item>
              <el-descriptions-item label="期初余额">{{ ledger.opening }}</el-descriptions-item>
              <el-descriptions-item label="期末余额">{{ ledger.closing }}</el-descriptions-item>
            </el-descriptions>
            <el-table v-loading="loading" :data="ledger.entries" border stripe class="detail-table">
              <el-table-column prop="voucher_date" label="日期" width="110" />
              <el-table-column prop="voucher_code" label="凭证号" min-width="160" />
              <el-table-column prop="summary" label="摘要" min-width="180" />
              <el-table-column prop="debit" label="借方" width="130" align="right" />
              <el-table-column prop="credit" label="贷方" width="130" align="right" />
              <el-table-column prop="balance" label="余额" width="130" align="right" />
            </el-table>
          </template>
          <el-empty v-else description="请选择科目并查询" />
        </el-tab-pane>

        <el-tab-pane label="资产负债表" name="balance">
          <div class="toolbar">
            <el-date-picker v-model="balanceAsOf" type="date" value-format="YYYY-MM-DD" placeholder="截止日期" />
            <el-button type="primary" plain @click="loadBalanceSheet">查询</el-button>
          </div>
          <template v-if="balanceSheet">
            <div class="balance-grid">
              <div>
                <h3>资产</h3>
                <el-table :data="balanceSheet.assets" border>
                  <el-table-column prop="code" label="科目" width="100" />
                  <el-table-column prop="name" label="名称" min-width="160" />
                  <el-table-column prop="amount" label="金额" width="130" align="right" />
                </el-table>
                <div class="total">资产合计：{{ balanceSheet.total_assets }}</div>
              </div>
              <div>
                <h3>负债和所有者权益</h3>
                <el-table :data="balanceSheet.liabilities" border>
                  <el-table-column prop="code" label="科目" width="100" />
                  <el-table-column prop="name" label="名称" min-width="160" />
                  <el-table-column prop="amount" label="金额" width="130" align="right" />
                </el-table>
                <el-table :data="balanceSheet.equity" border class="detail-table">
                  <el-table-column prop="code" label="科目" width="100" />
                  <el-table-column prop="name" label="名称" min-width="160" />
                  <el-table-column prop="amount" label="金额" width="130" align="right" />
                </el-table>
                <div class="total">负债合计：{{ balanceSheet.total_liabilities }}</div>
                <div class="total">权益合计：{{ balanceSheet.total_equity }}</div>
                <div class="total">未结转利润（累计）：{{ balanceSheet.current_profit }}</div>
                <div class="total">负债和权益合计：{{ balanceSheet.total_liabilities_and_equity }}</div>
              </div>
            </div>
          </template>
          <el-empty v-else description="点击查询生成资产负债表" />
        </el-tab-pane>

        <el-tab-pane label="辅助余额表" name="auxiliary">
          <div class="toolbar">
            <el-select v-model="auxiliaryDimension" style="width: 130px">
              <el-option label="客商" value="partner" />
              <el-option label="部门" value="department" />
              <el-option label="项目" value="project" />
            </el-select>
            <el-select v-model="auxiliaryAccountID" filterable clearable placeholder="全部科目" style="width: 260px">
              <el-option
                v-for="account in accountOptions"
                :key="account.id"
                :label="`${account.code} ${account.name}`"
                :value="account.id"
              />
            </el-select>
            <el-date-picker v-model="auxiliaryRange" type="daterange" value-format="YYYY-MM-DD" start-placeholder="开始日期" end-placeholder="结束日期" />
            <el-button type="primary" plain @click="loadAuxiliary">查询</el-button>
          </div>
          <el-table v-loading="loading" :data="auxiliaryBalance?.rows ?? []" border stripe>
            <el-table-column prop="account_code" label="科目编码" width="110" />
            <el-table-column prop="account_name" label="科目名称" min-width="160" />
            <el-table-column prop="dimension_code" label="辅助编码" width="130" />
            <el-table-column prop="dimension_name" label="辅助名称" min-width="160" />
            <el-table-column prop="opening" label="期初" width="130" align="right" />
            <el-table-column prop="period_debit" label="本期借方" width="130" align="right" />
            <el-table-column prop="period_credit" label="本期贷方" width="130" align="right" />
            <el-table-column prop="closing" label="期末" width="130" align="right" />
          </el-table>
          <div class="summary">
            <span>本期借方合计：{{ auxiliaryBalance?.total_debit ?? '-' }}</span>
            <span>本期贷方合计：{{ auxiliaryBalance?.total_credit ?? '-' }}</span>
          </div>
        </el-tab-pane>

        <el-tab-pane label="利润表" name="income">
          <div class="toolbar">
            <el-date-picker v-model="incomeRange" type="daterange" value-format="YYYY-MM-DD" start-placeholder="开始日期" end-placeholder="结束日期" />
            <el-button type="primary" plain @click="loadIncomeStatement">查询</el-button>
          </div>
          <template v-if="incomeStatement">
            <h3>收入</h3>
            <el-table :data="incomeStatement.revenue" border>
              <el-table-column prop="code" label="科目" width="110" />
              <el-table-column prop="name" label="名称" min-width="200" />
              <el-table-column prop="amount" label="金额" width="150" align="right" />
            </el-table>
            <div class="total">收入合计：{{ incomeStatement.total_revenue }}</div>

            <h3 class="section-title">费用</h3>
            <el-table :data="incomeStatement.expenses" border>
              <el-table-column prop="code" label="科目" width="110" />
              <el-table-column prop="name" label="名称" min-width="200" />
              <el-table-column prop="amount" label="金额" width="150" align="right" />
            </el-table>
            <div class="total">费用合计：{{ incomeStatement.total_expenses }}</div>
            <div class="total net-profit">净利润：{{ incomeStatement.net_profit }}</div>
          </template>
          <el-empty v-else description="点击查询生成利润表" />
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import {
  glApi,
  type AuxiliaryBalance,
  type BalanceSheet,
  type GeneralLedger,
  type GlAccount,
  type IncomeStatement,
  type TrialBalance
} from '@mlf-erp/sdk'

const activeTab = ref('trial')
const loading = ref(false)
const accounts = ref<GlAccount[]>([])
const trial = ref<TrialBalance | null>(null)
const ledger = ref<GeneralLedger | null>(null)
const balanceSheet = ref<BalanceSheet | null>(null)
const incomeStatement = ref<IncomeStatement | null>(null)

const now = new Date()
const monthStart = new Date(now.getFullYear(), now.getMonth(), 1)
const firstDay = localDate(monthStart)
const today = localDate(now)

const trialRange = ref<[string, string]>([firstDay, today])
const ledgerRange = ref<[string, string]>([firstDay, today])
const incomeRange = ref<[string, string]>([firstDay, today])
const balanceAsOf = ref(today)
const ledgerAccountID = ref('')
const auxiliaryDimension = ref('partner')
const auxiliaryAccountID = ref('')
const auxiliaryRange = ref<[string, string]>([firstDay, today])
const auxiliaryBalance = ref<AuxiliaryBalance | null>(null)

const accountOptions = computed(() => accounts.value.filter((item) => item.is_leaf && item.status === 'active'))

function localDate(value: Date) {
  const local = new Date(value.getTime() - value.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 10)
}

function typeLabel(type: string) {
  return (
    {
      asset: '资产',
      liability: '负债',
      equity: '权益',
      revenue: '收入',
      expense: '费用'
    }[type] ?? type
  )
}

async function loadTrial() {
  loading.value = true
  try {
    trial.value = await glApi.reports.trialBalance({
      start_date: trialRange.value?.[0],
      end_date: trialRange.value?.[1]
    })
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载科目余额表失败')
  } finally {
    loading.value = false
  }
}

async function loadLedger() {
  if (!ledgerAccountID.value) {
    return
  }
  loading.value = true
  try {
    ledger.value = await glApi.reports.generalLedger({
      account_id: ledgerAccountID.value,
      start_date: ledgerRange.value?.[0],
      end_date: ledgerRange.value?.[1]
    })
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载明细账失败')
  } finally {
    loading.value = false
  }
}

async function loadBalanceSheet() {
  loading.value = true
  try {
    balanceSheet.value = await glApi.reports.balanceSheet({ as_of: balanceAsOf.value })
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载资产负债表失败')
  } finally {
    loading.value = false
  }
}

async function loadAuxiliary() {
  loading.value = true
  try {
    auxiliaryBalance.value = await glApi.reports.auxiliaryBalance({
      dimension: auxiliaryDimension.value,
      account_id: auxiliaryAccountID.value || undefined,
      start_date: auxiliaryRange.value?.[0],
      end_date: auxiliaryRange.value?.[1]
    })
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载辅助余额表失败')
  } finally {
    loading.value = false
  }
}

async function loadIncomeStatement() {
  loading.value = true
  try {
    incomeStatement.value = await glApi.reports.incomeStatement({
      start_date: incomeRange.value?.[0],
      end_date: incomeRange.value?.[1]
    })
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载利润表失败')
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

function refresh() {
  onTabChange(activeTab.value)
}

function onTabChange(tab: string) {
  if (tab === 'trial') loadTrial()
  if (tab === 'ledger') loadLedger()
  if (tab === 'balance') loadBalanceSheet()
  if (tab === 'auxiliary') loadAuxiliary()
  if (tab === 'income') loadIncomeStatement()
}

onMounted(async () => {
  await loadAccounts()
  await loadTrial()
})
</script>

<style scoped>
.toolbar {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}

.summary {
  display: flex;
  gap: 24px;
  margin-top: 12px;
  color: #475569;
}

.detail-table {
  margin-top: 16px;
}

.balance-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 24px;
}

.total {
  margin-top: 10px;
  color: #1e293b;
  font-weight: 600;
}

.section-title {
  margin-top: 20px;
}

.net-profit {
  font-size: 16px;
  color: #0f766e;
}
</style>
