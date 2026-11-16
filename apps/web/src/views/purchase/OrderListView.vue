<template>
  <div>
    <PageHeader
      title="采购订单"
      description="示例模块：Go API + 内存仓储 + Vue 页面，用于验证 Monorepo 纵向链路。"
    >
      <el-button v-if="canWrite" type="primary" @click="dialogVisible = true">新建订单</el-button>
    </PageHeader>

    <el-card shadow="never">
      <el-table v-loading="loading" :data="orders" border stripe>
        <el-table-column prop="code" label="订单号" min-width="140" />
        <el-table-column prop="supplier_id" label="供应商" min-width="120" />
        <el-table-column prop="amount" label="金额" width="140" align="right" />
        <el-table-column prop="currency" label="币种" width="90" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" min-width="180">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无采购订单，点击右上角新建。" />
        </template>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" title="新建采购订单" width="480px" @closed="resetForm">
      <el-form :model="form" label-width="90px">
        <el-form-item label="订单号" required>
          <el-input v-model="form.code" placeholder="例如 PO-2026-0001" />
        </el-form-item>
        <el-form-item label="供应商">
          <el-input v-model="form.supplier_id" placeholder="供应商 ID" />
        </el-form-item>
        <el-form-item label="金额" required>
          <el-input v-model="form.amount" placeholder="例如 1234.5600" />
        </el-form-item>
        <el-form-item label="币种">
          <el-select v-model="form.currency" style="width: 100%">
            <el-option label="人民币 CNY" value="CNY" />
            <el-option label="美元 USD" value="USD" />
            <el-option label="欧元 EUR" value="EUR" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import { purchaseApi, type PurchaseOrder } from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const canWrite = computed(() => authStore.hasPermission('purchase.write'))
const orders = ref<PurchaseOrder[]>([])
const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const form = reactive({ code: '', supplier_id: '', amount: '', currency: 'CNY' })

async function load() {
  loading.value = true
  try {
    orders.value = await purchaseApi.list()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载采购订单失败')
  } finally {
    loading.value = false
  }
}

async function submit() {
  if (!form.code.trim() || !form.amount.trim()) {
    ElMessage.warning('订单号和金额不能为空')
    return
  }

  submitting.value = true
  try {
    await purchaseApi.create({
      code: form.code.trim(),
      supplier_id: form.supplier_id.trim(),
      amount: form.amount.trim(),
      currency: form.currency
    })
    ElMessage.success('采购订单已创建')
    dialogVisible.value = false
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '创建采购订单失败')
  } finally {
    submitting.value = false
  }
}

function resetForm() {
  form.code = ''
  form.supplier_id = ''
  form.amount = ''
  form.currency = 'CNY'
}

function formatDate(value: string) {
  return value ? new Date(value).toLocaleString() : '-'
}

onMounted(load)
</script>
