<template>
  <MasterDataCrud
    title="物料管理"
    description="维护物料编码、规格、计量单位与税率。"
    permission="masterdata.material"
    :api="api"
    :columns="columns"
    :fields="fields"
    :default-form="defaultForm"
    search-placeholder="搜索编码、名称、规格或条码"
  />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import MasterDataCrud from '@/components/MasterDataCrud.vue'
import type { ColumnConfig, CrudApi, FieldConfig, FieldOption } from '@/components/masterdata-types'
import { ElMessage } from 'element-plus'
import { masterDataApi } from '@mlf-erp/sdk'

const api = masterDataApi.materials as unknown as CrudApi
const unitOptions = ref<FieldOption[]>([])
const taxRateOptions = ref<FieldOption[]>([])

const defaultForm = {
  code: '',
  name: '',
  spec: '',
  category: '',
  unit_id: '',
  tax_rate_id: '',
  barcode: '',
  status: 'active'
}

const columns: ColumnConfig[] = [
  { key: 'code', label: '编码', width: 160 },
  { key: 'name', label: '名称', minWidth: 180 },
  { key: 'spec', label: '规格', minWidth: 140 },
  { key: 'category', label: '分类', width: 110 },
  { key: 'unit_id', label: '单位', width: 100, formatter: (row) => unitLabel(row.unit_id) },
  { key: 'tax_rate_id', label: '税率', width: 100, formatter: (row) => taxRateLabel(row.tax_rate_id) },
  {
    key: 'status',
    label: '状态',
    width: 90,
    formatter: (row) => (row.status === 'active' ? '启用' : '禁用')
  }
]

const fields = computed<FieldConfig[]>(() => [
  { key: 'code', label: '编码', placeholder: '留空自动生成' },
  { key: 'name', label: '名称', required: true },
  { key: 'spec', label: '规格' },
  { key: 'category', label: '分类' },
  { key: 'unit_id', label: '计量单位', type: 'select', options: unitOptions.value },
  { key: 'tax_rate_id', label: '税率', type: 'select', options: taxRateOptions.value },
  { key: 'barcode', label: '条码' },
  {
    key: 'status',
    label: '状态',
    type: 'select',
    options: [
      { label: '启用', value: 'active' },
      { label: '禁用', value: 'disabled' }
    ]
  }
])

function unitLabel(value: string) {
  return unitOptions.value.find((option) => option.value === value)?.label ?? value ?? '-'
}

function taxRateLabel(value: string) {
  return taxRateOptions.value.find((option) => option.value === value)?.label ?? value ?? '-'
}

onMounted(async () => {
  try {
    const [units, taxRates] = await Promise.all([
      masterDataApi.units.list({ page_size: 200 }),
      masterDataApi.taxRates.list({ page_size: 200 })
    ])
    unitOptions.value = units.items.map((unit) => ({
      label: `${unit.name}（${unit.code}）`,
      value: unit.id
    }))
    taxRateOptions.value = taxRates.items.map((taxRate) => ({
      label: `${taxRate.name}（${taxRate.rate}）`,
      value: taxRate.id
    }))
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载单位或税率失败')
  }
})
</script>
