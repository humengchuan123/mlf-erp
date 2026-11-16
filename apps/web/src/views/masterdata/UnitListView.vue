<template>
  <MasterDataCrud
    title="计量单位"
    description="维护物料计量单位和小数精度。"
    permission="masterdata.unit"
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

const api = masterDataApi.units as unknown as CrudApi

const defaultForm = { code: '', name: '', precision: 0 }

const columns: ColumnConfig[] = [
  { key: 'code', label: '编码', width: 160 },
  { key: 'name', label: '名称', minWidth: 180 },
  { key: 'precision', label: '精度', width: 100 },
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
  { key: 'precision', label: '小数精度', type: 'number', min: 0, max: 6 }
]
</script>
