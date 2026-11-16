<template>
  <MasterDataCrud
    title="税率管理"
    description="维护销项、进项和其他税率。"
    permission="masterdata.tax"
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

const api = masterDataApi.taxRates as unknown as CrudApi

const defaultForm = { code: '', name: '', rate: '0.1300', type: 'vat_output', status: 'active' }

const columns: ColumnConfig[] = [
  { key: 'code', label: '编码', width: 140 },
  { key: 'name', label: '名称', minWidth: 160 },
  { key: 'rate', label: '税率', width: 100 },
  {
    key: 'type',
    label: '类型',
    width: 120,
    formatter: (row) => {
      const labels: Record<string, string> = { vat_output: '销项', vat_input: '进项', other: '其他' }
      return labels[String(row.type)] ?? String(row.type)
    }
  },
  {
    key: 'status',
    label: '状态',
    width: 90,
    formatter: (row) => (row.status === 'active' ? '启用' : '禁用')
  }
]

const fields: FieldConfig[] = [
  { key: 'code', label: '编码', required: true },
  { key: 'name', label: '名称', required: true },
  { key: 'rate', label: '税率', required: true, placeholder: '例如 0.1300' },
  {
    key: 'type',
    label: '类型',
    type: 'select',
    options: [
      { label: '销项', value: 'vat_output' },
      { label: '进项', value: 'vat_input' },
      { label: '其他', value: 'other' }
    ]
  },
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
