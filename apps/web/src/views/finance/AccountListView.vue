<template>
  <div>
    <PageHeader title="会计科目" description="维护总账科目、借贷方向和启停状态。">
      <el-button v-if="canWrite" type="primary" @click="openCreate">新建科目</el-button>
    </PageHeader>

    <el-card shadow="never">
      <el-table v-loading="loading" :data="rows" row-key="id" :tree-props="{ children: 'children' }" default-expand-all border>
        <el-table-column prop="code" label="科目编码" width="140" />
        <el-table-column prop="name" label="科目名称" min-width="200" />
        <el-table-column prop="type" label="类型" width="100">
          <template #default="{ row }">{{ typeLabel(row.type) }}</template>
        </el-table-column>
        <el-table-column prop="direction" label="方向" width="80">
          <template #default="{ row }">{{ row.direction === 'debit' ? '借' : '贷' }}</template>
        </el-table-column>
        <el-table-column prop="level" label="级次" width="80" align="center" />
        <el-table-column prop="is_leaf" label="末级" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.is_leaf ? 'success' : 'info'" size="small">{{ row.is_leaf ? '是' : '否' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">
              {{ row.status === 'active' ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button v-if="canWrite" link type="primary" @click="openEdit(row)">编辑</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑科目' : '新建科目'" width="560px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="科目编码" required>
          <el-input v-model="form.code" placeholder="如 6604" />
        </el-form-item>
        <el-form-item label="科目名称" required>
          <el-input v-model="form.name" />
        </el-form-item>
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="类型" required>
              <el-select v-model="form.type" style="width: 100%">
                <el-option label="资产" value="asset" />
                <el-option label="负债" value="liability" />
                <el-option label="权益" value="equity" />
                <el-option label="收入" value="revenue" />
                <el-option label="费用" value="expense" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="方向" required>
              <el-select v-model="form.direction" style="width: 100%">
                <el-option label="借方" value="debit" />
                <el-option label="贷方" value="credit" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="上级科目">
          <el-select v-model="form.parent_id" clearable filterable style="width: 100%" placeholder="不选则为一级科目">
            <el-option
              v-for="account in parentOptions"
              :key="account.id"
              :label="`${account.code} ${account.name}`"
              :value="account.id"
              :disabled="account.id === editingId"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="form.status" style="width: 100%">
            <el-option label="启用" value="active" />
            <el-option label="停用" value="disabled" />
          </el-select>
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
import { glApi, type GlAccount, type GlAccountInput } from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

interface AccountNode extends GlAccount {
  children?: AccountNode[]
}

const authStore = useAuthStore()
const canWrite = computed(() => authStore.hasPermission('finance.account.write'))

const accounts = ref<GlAccount[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const editingId = ref('')
const form = reactive({
  code: '',
  name: '',
  type: 'asset',
  direction: 'debit',
  parent_id: '',
  status: 'active'
})

const rows = computed<AccountNode[]>(() => {
  const map = new Map<string, AccountNode>()
  accounts.value.forEach((account) => map.set(account.id, { ...account }))
  const roots: AccountNode[] = []
  map.forEach((node) => {
    const parent = node.parent_id ? map.get(node.parent_id) : undefined
    if (parent) {
      parent.children = parent.children ?? []
      parent.children.push(node)
    } else {
      roots.push(node)
    }
  })
  return roots
})

const parentOptions = computed(() => accounts.value)

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

async function load() {
  loading.value = true
  try {
    accounts.value = await glApi.accounts.list()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载科目失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = ''
  Object.assign(form, {
    code: '',
    name: '',
    type: 'asset',
    direction: 'debit',
    parent_id: '',
    status: 'active'
  })
  dialogVisible.value = true
}

function openEdit(account: GlAccount) {
  editingId.value = account.id
  Object.assign(form, {
    code: account.code,
    name: account.name,
    type: account.type,
    direction: account.direction,
    parent_id: account.parent_id,
    status: account.status
  })
  dialogVisible.value = true
}

async function save() {
  if (!form.code || !form.name) {
    ElMessage.warning('请填写科目编码和名称')
    return
  }
  saving.value = true
  try {
    const input: GlAccountInput = {
      code: form.code,
      name: form.name,
      type: form.type,
      direction: form.direction,
      parent_id: form.parent_id || undefined,
      status: form.status
    }
    if (editingId.value) {
      await glApi.accounts.update(editingId.value, input)
      ElMessage.success('科目已更新')
    } else {
      await glApi.accounts.create(input)
      ElMessage.success('科目已创建')
    }
    dialogVisible.value = false
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存科目失败')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
