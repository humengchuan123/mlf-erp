<template>
  <div>
    <PageHeader
      title="审批流程"
      description="配置销售/采购订单的顺序审批流程；已有审批请求的流程不能编辑或删除，请停用后新建。"
    >
      <el-button v-if="canWrite" type="primary" @click="openCreate">新建流程</el-button>
    </PageHeader>

    <el-card shadow="never">
      <div class="toolbar">
        <el-select
          v-model="documentTypeFilter"
          placeholder="全部单据类型"
          clearable
          class="toolbar__filter"
          @change="loadFlows"
        >
          <el-option label="销售订单" value="sales_order" />
          <el-option label="采购订单" value="purchase_order" />
        </el-select>
        <el-select
          v-model="activeFilter"
          placeholder="全部状态"
          clearable
          class="toolbar__filter"
          @change="loadFlows"
        >
          <el-option label="启用" value="true" />
          <el-option label="停用" value="false" />
        </el-select>
        <el-button type="primary" plain @click="loadFlows">查询</el-button>
        <el-button @click="resetFilters">重置</el-button>
      </div>

      <el-table v-loading="loading" :data="flows" border stripe>
        <el-table-column prop="name" label="流程名称" min-width="170" />
        <el-table-column label="单据类型" width="110">
          <template #default="{ row }">{{ documentTypeLabel(row.document_type) }}</template>
        </el-table-column>
        <el-table-column label="组织" min-width="130">
          <template #default="{ row }">{{ orgLabel(row.org_id) }}</template>
        </el-table-column>
        <el-table-column label="条件" width="100">
          <template #default="{ row }">{{ conditionLabel(row.condition_type) }}</template>
        </el-table-column>
        <el-table-column label="阈值" width="130" align="right">
          <template #default="{ row }">
            {{ row.condition_type === 'amount_gte' ? row.threshold_amount : '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="priority" label="优先级" width="90" align="center" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.active ? 'success' : 'info'" size="small">
              {{ row.active ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="步骤" min-width="220">
          <template #default="{ row }">{{ stepsSummary(row) }}</template>
        </el-table-column>
        <el-table-column v-if="canWrite" label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" @click="removeFlow(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无审批流程" :image-size="80" />
        </template>
      </el-table>
    </el-card>

    <el-dialog
      v-model="dialogVisible"
      :title="editingFlow ? '编辑审批流程' : '新建审批流程'"
      width="980px"
    >
      <el-form :model="form" label-width="150px">
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="流程名称" required>
              <el-input v-model="form.name" placeholder="例如 销售订单金额审批" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="单据类型" required>
              <el-select v-model="form.document_type" style="width: 100%">
                <el-option label="销售订单" value="sales_order" />
                <el-option label="采购订单" value="purchase_order" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="组织（留空=租户级）">
              <el-tree-select
                v-model="form.org_id"
                :data="orgs"
                :props="{ label: 'name', value: 'id', children: 'children' }"
                check-strictly
                clearable
                filterable
                :disabled="orgsUnavailable"
                placeholder="留空表示租户级"
                style="width: 100%"
              />
              <div v-if="orgsUnavailable" class="form-hint">
                组织列表加载失败，留空即为租户级。
              </div>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="条件类型" required>
              <el-select v-model="form.condition_type" style="width: 100%">
                <el-option label="始终" value="always" />
                <el-option label="金额≥阈值（本位币）" value="amount_gte" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col v-if="form.condition_type === 'amount_gte'" :span="12">
            <el-form-item label="阈值金额（本位币）" required>
              <el-input v-model="form.threshold_amount" placeholder="0.0000" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="优先级">
              <el-input-number v-model="form.priority" :min="0" :max="9999" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="启用">
              <el-switch v-model="form.active" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">审批步骤</el-divider>
        <p v-if="approverOptionsHint" class="form-hint">{{ approverOptionsHint }}</p>
        <el-table :data="form.steps" border>
          <el-table-column label="步骤" width="70" align="center">
            <template #default="{ $index }">{{ $index + 1 }}</template>
          </el-table-column>
          <el-table-column label="步骤名称" min-width="180">
            <template #default="{ row }">
              <el-input v-model="row.name" placeholder="例如 销售主管" />
            </template>
          </el-table-column>
          <el-table-column label="审批人类型" width="150">
            <template #default="{ row }">
              <el-select v-model="row.approver_type" style="width: 100%" @change="changeApproverType(row)">
                <el-option label="角色" value="role" />
                <el-option label="用户" value="user" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="审批人" min-width="220">
            <template #default="{ row }">
              <el-select
                v-model="row.approver_id"
                filterable
                allow-create
                default-first-option
                style="width: 100%"
                :placeholder="row.approver_type === 'role' ? '选择角色' : '选择用户'"
              >
                <el-option
                  v-for="option in approverOptions(row.approver_type)"
                  :key="option.id"
                  :label="option.label"
                  :value="option.id"
                />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="80" align="center">
            <template #default="{ $index }">
              <el-button
                link
                type="danger"
                :disabled="form.steps.length <= 1"
                @click="removeStep($index)"
              >
                删除
              </el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-button class="add-step" plain @click="addStep">添加步骤</el-button>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveFlow">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import {
  ApiError,
  approvalApi,
  systemApi,
  type ApprovalApproverType,
  type ApprovalConditionType,
  type ApprovalDocumentType,
  type ApprovalFlow,
  type CreateApprovalFlowInput,
  type Organization,
  type SystemRole,
  type SystemUser
} from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

defineOptions({ name: 'ApprovalFlowListView' })

interface StepForm {
  name: string
  approver_type: ApprovalApproverType
  approver_id: string
}

interface ApproverOption {
  id: string
  label: string
}

const authStore = useAuthStore()
const canWrite = computed(() => authStore.hasPermission('system.approval.write'))

const flows = ref<ApprovalFlow[]>([])
const loading = ref(false)
const saving = ref(false)
const documentTypeFilter = ref<ApprovalDocumentType | ''>('')
const activeFilter = ref<'' | 'true' | 'false'>('')

const orgs = ref<Organization[]>([])
const roles = ref<SystemRole[]>([])
const users = ref<SystemUser[]>([])
const orgsUnavailable = ref(false)
const roleLoadFailed = ref(false)
const userLoadFailed = ref(false)

const dialogVisible = ref(false)
const editingFlow = ref<ApprovalFlow | null>(null)
const form = reactive({
  name: '',
  document_type: 'sales_order' as ApprovalDocumentType,
  org_id: '',
  condition_type: 'always' as ApprovalConditionType,
  threshold_amount: '0.0000',
  priority: 0,
  active: true,
  steps: [] as StepForm[]
})

const orgNameByID = computed(() => {
  const result = new Map<string, string>()
  const walk = (items: Organization[]) => {
    for (const item of items) {
      result.set(item.id, item.name)
      if (item.children?.length) walk(item.children)
    }
  }
  walk(orgs.value)
  return result
})

const roleOptions = computed<ApproverOption[]>(() =>
  roles.value.map((role) => ({ id: role.id, label: `${role.name}（${role.code}）` }))
)

const userOptions = computed<ApproverOption[]>(() =>
  users.value.map((user) => ({
    id: user.id,
    label: `${user.display_name || user.username}（${user.username}）`
  }))
)

const approverOptionsHint = computed(() => {
  if (roleLoadFailed.value && userLoadFailed.value) {
    return '角色和用户列表加载失败（可能缺少 system.role.read / system.user.read 权限），可在审批人框中手动输入 ID。'
  }
  if (roleLoadFailed.value) {
    return '角色列表加载失败（可能缺少 system.role.read 权限），可在审批人框中手动输入角色 ID。'
  }
  if (userLoadFailed.value) {
    return '用户列表加载失败（可能缺少 system.user.read 权限），可在审批人框中手动输入用户 ID。'
  }
  return ''
})

function documentTypeLabel(value: string) {
  if (value === 'sales_order') return '销售订单'
  if (value === 'purchase_order') return '采购订单'
  return value || '-'
}

function conditionLabel(value: string) {
  if (value === 'always') return '始终'
  if (value === 'amount_gte') return '金额≥'
  return value || '-'
}

function orgLabel(orgID: string) {
  if (!orgID) return '租户级'
  return orgNameByID.value.get(orgID) ?? orgID
}

function stepsSummary(flow: ApprovalFlow) {
  if (flow.steps.length === 0) return '-'
  return flow.steps.map((step) => `${step.step_no}. ${step.name}`).join(' → ')
}

function approverOptions(type: ApprovalApproverType): ApproverOption[] {
  return type === 'role' ? roleOptions.value : userOptions.value
}

function changeApproverType(step: StepForm) {
  step.approver_id = ''
}

async function loadFlows() {
  loading.value = true
  try {
    const result = await approvalApi.flows({
      document_type: documentTypeFilter.value || undefined,
      active: activeFilter.value ? activeFilter.value === 'true' : undefined
    })
    flows.value = result.items
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载审批流程失败')
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  documentTypeFilter.value = ''
  activeFilter.value = ''
  loadFlows()
}

async function loadOrgs() {
  try {
    orgs.value = await systemApi.orgs.list()
    orgsUnavailable.value = false
  } catch {
    orgs.value = []
    orgsUnavailable.value = true
  }
}

async function loadRoles() {
  try {
    roles.value = await systemApi.roles.list()
    roleLoadFailed.value = false
  } catch {
    roles.value = []
    roleLoadFailed.value = true
  }
}

async function loadUsers() {
  try {
    const result = await systemApi.users.list({ page_size: 200 })
    users.value = result.items
    userLoadFailed.value = false
  } catch {
    users.value = []
    userLoadFailed.value = true
  }
}

function resetForm() {
  Object.assign(form, {
    name: '',
    document_type: 'sales_order' as ApprovalDocumentType,
    org_id: '',
    condition_type: 'always' as ApprovalConditionType,
    threshold_amount: '0.0000',
    priority: 0,
    active: true,
    steps: [{ name: '', approver_type: 'role' as ApprovalApproverType, approver_id: '' }]
  })
}

function openCreate() {
  editingFlow.value = null
  resetForm()
  dialogVisible.value = true
}

function openEdit(flow: ApprovalFlow) {
  editingFlow.value = flow
  Object.assign(form, {
    name: flow.name,
    document_type: flow.document_type,
    org_id: flow.org_id ?? '',
    condition_type: flow.condition_type,
    threshold_amount: flow.threshold_amount,
    priority: flow.priority,
    active: flow.active,
    steps: flow.steps.map((step) => ({
      name: step.name,
      approver_type: step.approver_type,
      approver_id: step.approver_id
    }))
  })
  dialogVisible.value = true
}

function addStep() {
  form.steps.push({ name: '', approver_type: 'role', approver_id: '' })
}

function removeStep(index: number) {
  if (form.steps.length <= 1) return
  form.steps.splice(index, 1)
}

function validateForm() {
  if (!form.name.trim()) {
    ElMessage.warning('请填写流程名称')
    return false
  }
  if (!form.document_type) {
    ElMessage.warning('请选择单据类型')
    return false
  }
  if (!form.condition_type) {
    ElMessage.warning('请选择条件类型')
    return false
  }
  if (form.condition_type === 'amount_gte') {
    const threshold = form.threshold_amount.trim()
    const value = Number(threshold)
    if (!threshold || !Number.isFinite(value) || value < 0) {
      ElMessage.warning('阈值金额必须是大于等于 0 的数字')
      return false
    }
  }
  if (form.steps.length === 0) {
    ElMessage.warning('至少添加一个审批步骤')
    return false
  }
  for (let index = 0; index < form.steps.length; index += 1) {
    const step = form.steps[index]
    if (!step.name.trim()) {
      ElMessage.warning(`请填写第 ${index + 1} 步的名称`)
      return false
    }
    if (!step.approver_id) {
      ElMessage.warning(`请选择第 ${index + 1} 步的审批人`)
      return false
    }
  }
  return true
}

function approvalErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError && error.code === 'FLOW_IN_USE') {
    return '该流程已有审批请求（含历史），不能编辑或删除；请停用后新建。'
  }
  return error instanceof Error ? error.message : fallback
}

async function saveFlow() {
  if (!validateForm()) return

  const input: CreateApprovalFlowInput = {
    name: form.name.trim(),
    document_type: form.document_type,
    org_id: form.org_id || '',
    condition_type: form.condition_type,
    threshold_amount:
      form.condition_type === 'amount_gte' ? form.threshold_amount.trim() : '0',
    priority: form.priority,
    active: form.active,
    steps: form.steps.map((step, index) => ({
      step_no: index + 1,
      name: step.name.trim(),
      approver_type: step.approver_type,
      approver_id: step.approver_id
    }))
  }

  saving.value = true
  try {
    if (editingFlow.value) {
      await approvalApi.updateFlow(editingFlow.value.id, input)
      ElMessage.success('审批流程已更新')
    } else {
      await approvalApi.createFlow(input)
      ElMessage.success('审批流程已创建')
    }
    dialogVisible.value = false
    await loadFlows()
  } catch (error) {
    ElMessage.error(approvalErrorMessage(error, '保存审批流程失败'))
  } finally {
    saving.value = false
  }
}

async function removeFlow(flow: ApprovalFlow) {
  try {
    await ElMessageBox.confirm(`确认删除审批流程 ${flow.name}？`, '删除审批流程', {
      type: 'warning'
    })
  } catch {
    return
  }

  try {
    await approvalApi.removeFlow(flow.id)
    ElMessage.success('审批流程已删除')
    await loadFlows()
  } catch (error) {
    ElMessage.error(approvalErrorMessage(error, '删除审批流程失败'))
  }
}

onMounted(async () => {
  await Promise.all([loadFlows(), loadOrgs(), loadRoles(), loadUsers()])
})
</script>

<style scoped>
.toolbar {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}

.toolbar__filter {
  width: 160px;
}

.form-hint {
  margin: 6px 0 0;
  color: #64748b;
  font-size: 12px;
}

.add-step {
  margin-top: 12px;
}
</style>
