<template>
  <MasterDataCrud
    title="仓库管理"
    description="维护仓库编码、类型与地址。"
    permission="masterdata.warehouse"
    :api="api"
    :columns="columns"
    :fields="fields"
    :default-form="defaultForm"
    search-placeholder="搜索编码、名称或地址"
  />
</template>

<script setup lang="ts">
import MasterDataCrud from '@/components/MasterDataCrud.vue'
import type { ColumnConfig, CrudApi, FieldConfig } from '@/components/masterdata-types'
import { masterDataApi } from '@mlf-erp/sdk'

const api = masterDataApi.warehouses as unknown as CrudApi

const defaultForm = {
  code: '',
  name: '',
  type: 'normal',
  address: '',
  status: 'active'
}

const columns: ColumnConfig[] = [
  { key: 'code', label: '编码', width: 160 },
  { key: 'name', label: '名称', minWidth: 180 },
  {
    key: 'type',
    label: '类型',
    width: 120,
    formatter: (row) => {
      const labels: Record<string, string> = { normal: '普通仓', consignment: '寄售仓', other: '其他' }
      return labels[String(row.type)] ?? String(row.type)
    }
  },
  { key: 'address', label: '地址', minWidth: 200 },
  {
    key: 'status',
    label: '状态',
    width: 90,
    formatter: (row) => (row.status === 'active' ? '启用' : '禁用')
  }
]

const fields: FieldConfig[] = [
  { key: 'code', label: '编码', placeholder: '留空自动生成' },
  { key: 'name', label: '名称', required: true },
  {
    key: 'type',
    label: '类型',
    type: 'select',
    options: [
      { label: '普通仓', value: 'normal' },
      { label: '寄售仓', value: 'consignment' },
      { label: '其他', value: 'other' }
    ]
  },
  { key: 'address', label: '地址', type: 'textarea' },
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
