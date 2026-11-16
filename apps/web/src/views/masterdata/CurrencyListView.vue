<template>
  <MasterDataCrud
    title="币种管理"
    description="维护币种、符号和小数位。"
    permission="masterdata.currency"
    :api="api"
    :columns="columns"
    :fields="fields"
    :default-form="defaultForm"
    search-placeholder="搜索编码或名称"
  />
</template>

<script setup lang="ts">
import MasterDataCrud from '@/components/MasterDataCrud.vue'
import type { ColumnConfig, CrudApi, FieldConfig } from '@/components/masterdata-types'
import { masterDataApi } from '@mlf-erp/sdk'

const api = masterDataApi.currencies as unknown as CrudApi

const defaultForm = { code: '', name: '', symbol: '', decimal_places: 2, status: 'active' }

const columns: ColumnConfig[] = [
  { key: 'code', label: '编码', width: 120 },
  { key: 'name', label: '名称', minWidth: 160 },
  { key: 'symbol', label: '符号', width: 100 },
  { key: 'decimal_places', label: '小数位', width: 100 },
  {
    key: 'status',
    label: '状态',
    width: 90,
    formatter: (row) => (row.status === 'active' ? '启用' : '禁用')
  }
]

const fields: FieldConfig[] = [
  { key: 'code', label: '编码', required: true, placeholder: '例如 CNY' },
  { key: 'name', label: '名称', required: true },
  { key: 'symbol', label: '符号', placeholder: '例如 ¥' },
  { key: 'decimal_places', label: '小数位', type: 'number', min: 0, max: 6 },
  {
    key: 'status',
    label: '状态',
    type: 'select',
    options: [
      { label: '启用', value: 'active' },
      { label: '禁用', value: 'disabled' }
    ]
  }
]
</script>
