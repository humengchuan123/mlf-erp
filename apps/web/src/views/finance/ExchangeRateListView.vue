<template>
  <div>
    <PageHeader title="汇率管理" description="维护外币对本位币 CNY 的汇率，单据默认取单据日期最近汇率。">
      <el-button v-if="canWrite" type="primary" @click="openCreate">新增汇率</el-button>
    </PageHeader>

    <el-card shadow="never">
      <div class="toolbar">
        <el-select v-model="currencyFilter" placeholder="全部币种" clearable style="width: 140px" @change="search">
          <el-option v-for="item in currencies" :key="item.code" :label="item.code" :value="item.code" />
        </el-select>
        <el-button type="primary" plain @click="search">查询</el-button>
        <el-button @click="resetSearch">重置</el-button>
      </div>

      <el-table v-loading="loading" :data="rates" border stripe>
        <el-table-column prop="currency_code" label="币种" width="100" />
        <el-table-column prop="rate_date" label="生效日期" width="130" />
        <el-table-column prop="rate" label="汇率（1 外币 = CNY）" width="200" align="right" />
        <el-table-column prop="updated_at" label="更新时间" min-width="180">
          <template #default="{ row }">{{ formatDate(row.updated_at) }}</template>
        </el-table-column>
        <el-table-column v-if="canWrite" label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
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

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑汇率' : '新增汇率'" width="520px">
      <el-form :model="form" label-width="100px">
        <el-form-item label="币种" required>
          <el-select v-model="form.currency_code" filterable style="width: 100%">
            <el-option v-for="item in foreignCurrencies" :key="item.code" :label="`${item.code} ${item.name}`" :value="item.code" />
          </el-select>
        </el-form-item>
        <el-form-item label="生效日期" required>
          <el-date-picker v-model="form.rate_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
        <el-form-item label="汇率" required>
          <el-input v-model="form.rate" placeholder="例如 7.12345678" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
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
  type Currency,
  type ExchangeRate,
  type ExchangeRateInput
} from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const canWrite = computed(() => authStore.hasPermission('finance.exchange_rate.write'))

const rates = ref<ExchangeRate[]>([])
const currencies = ref<Currency[]>([])
const loading = ref(false)
const saving = ref(false)
const currencyFilter = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const dialogVisible = ref(false)
const editing = ref<ExchangeRate | null>(null)
const form = reactive<ExchangeRateInput>({
  currency_code: 'USD',
  rate_date: today(),
  rate: ''
})

const foreignCurrencies = computed(() => currencies.value.filter((item) => item.status === 'active' && item.code !== 'CNY'))

function today() {
  const now = new Date()
  const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 10)
}

async function load() {
  loading.value = true
  try {
    const result = await financeApi.exchangeRates.list({
      currency: currencyFilter.value,
      page: page.value,
      page_size: pageSize.value
    })
    rates.value = result.items
    total.value = result.total
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载汇率失败')
  } finally {
    loading.value = false
  }
}

async function loadCurrencies() {
  try {
    const result = await masterDataApi.currencies.list({ page_size: 200 })
    currencies.value = result.items
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载币种失败')
  }
}

function search() {
  page.value = 1
  load()
}

function resetSearch() {
  currencyFilter.value = ''
  page.value = 1
  load()
}

function handleSizeChange() {
  page.value = 1
  load()
}

function openCreate() {
  editing.value = null
  Object.assign(form, { currency_code: 'USD', rate_date: today(), rate: '' })
  dialogVisible.value = true
}

function openEdit(row: ExchangeRate) {
  editing.value = row
  Object.assign(form, { currency_code: row.currency_code, rate_date: row.rate_date, rate: row.rate })
  dialogVisible.value = true
}

async function save() {
  if (!form.currency_code || !form.rate_date || !form.rate) {
    ElMessage.warning('请完整填写币种、日期和汇率')
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await financeApi.exchangeRates.update(editing.value.id, form)
      ElMessage.success('汇率已更新')
    } else {
      await financeApi.exchangeRates.upsert(form)
      ElMessage.success('汇率已保存')
    }
    dialogVisible.value = false
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存汇率失败')
  } finally {
    saving.value = false
  }
}

function formatDate(value: string) {
  return value ? new Date(value).toLocaleString() : '-'
}

onMounted(async () => {
  await Promise.all([load(), loadCurrencies()])
})
</script>

<style scoped>
.toolbar {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}

.pagination {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>
