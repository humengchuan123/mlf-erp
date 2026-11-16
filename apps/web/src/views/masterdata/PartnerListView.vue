<template>
  <MasterDataCrud
    title="客商管理"
    description="维护客户、供应商及其联系信息。"
    permission="masterdata.partner"
    :api="api"
    :columns="columns"
    :fields="fields"
    :default-form="defaultForm"
    search-placeholder="搜索编码、名称、联系人或电话"
  />
</template>

<script setup lang="ts">
import MasterDataCrud from '@/components/MasterDataCrud.vue'
import type { ColumnConfig, CrudApi, FieldConfig } from '@/components/masterdata-types'
import { masterDataApi } from '@mlf-erp/sdk'

const api = masterDataApi.partners as unknown as CrudApi

const defaultForm = {
  code: '',
  name: '',
  type: 'both',
  category: '',
  contact: '',
  phone: '',
  email: '',
  address: '',
  tax_no: ''
}

const columns: ColumnConfig[] = [
  { key: 'code', label: '编码', width: 160 },
  { key: 'name', label: '名称', minWidth: 180 },
  {
    key: 'type',
    label: '类型',
    width: 120,
    formatter: (row) => {
      const labels: Record<string, string> = { customer: '客户', supplier: '供应商', both: '客户/供应商' }
      return labels[String(row.type)] ?? String(row.type)
    }
  },
  { key: 'contact', label: '联系人', width: 110 },
  { key: 'phone', label: '电话', width: 140 },
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
      { label: '客户', value: 'customer' },
      { label: '供应商', value: 'supplier' },
      { label: '客户/供应商', value: 'both' }
    ]
  },
  { key: 'category', label: '分类' },
  { key: 'contact', label: '联系人' },
  { key: 'phone', label: '电话' },
  { key: 'email', label: '邮箱' },
  { key: 'address', label: '地址', type: 'textarea' },
  { key: 'tax_no', label: '税号' }
]
</script>
