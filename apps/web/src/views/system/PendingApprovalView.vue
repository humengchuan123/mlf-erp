<template>
  <div>
    <PageHeader
      title="待我审批"
      description="查看当前由您审批的销售/采购订单申请；批准或拒绝需要对应订单的审批权限。"
    >
      <el-button plain @click="loadPending">刷新</el-button>
    </PageHeader>

    <el-card shadow="never">
      <el-table v-loading="loading" :data="requests" border stripe>
        <el-table-column label="单据类型" width="110">
          <template #default="{ row }">{{ documentTypeLabel(row.document_type) }}</template>
        </el-table-column>
        <el-table-column prop="document_code" label="单据编号" min-width="160" />
        <el-table-column label="金额" width="160" align="right">
          <template #default="{ row }">{{ row.amount }} {{ row.currency }}</template>
        </el-table-column>
        <el-table-column label="当前步骤" width="170">
          <template #default="{ row }">{{ currentStepLabel(row) }}</template>
        </el-table-column>
        <el-table-column prop="requested_by" label="申请人" min-width="180" />
        <el-table-column label="申请时间" width="180">
          <template #default="{ row }">{{ formatTime(row.requested_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openDetail(row)">详情</el-button>
            <el-button link type="success" @click="openApprove(row)">批准</el-button>
            <el-button link type="danger" @click="openReject(row)">拒绝</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无待办审批" :image-size="80" />
        </template>
      </el-table>
    </el-card>

    <el-dialog v-model="detailVisible" title="审批详情" width="900px">
      <div v-loading="detailLoading" class="detail-body">
        <template v-if="detail">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="单据类型">
              {{ documentTypeLabel(detail.document_type) }}
            </el-descriptions-item>
            <el-descriptions-item label="单据编号">{{ detail.document_code }}</el-descriptions-item>
            <el-descriptions-item label="金额">
              {{ detail.amount }} {{ detail.currency }}
            </el-descriptions-item>
            <el-descriptions-item label="本位币金额">{{ detail.base_amount }}</el-descriptions-item>
            <el-descriptions-item label="状态">
              <el-tag :type="statusTagType(detail.status)" size="small">
                {{ statusLabel(detail.status) }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="当前步骤">
              {{ currentStepLabel(detail) }}
            </el-descriptions-item>
            <el-descriptions-item label="申请人">{{ detail.requested_by }}</el-descriptions-item>
            <el-descriptions-item label="申请时间">
              {{ formatTime(detail.requested_at) }}
            </el-descriptions-item>
          </el-descriptions>

          <el-divider content-position="left">审批步骤</el-divider>
          <el-table :data="detail.steps" border size="small">
            <el-table-column prop="step_no" label="步骤" width="60" />
            <el-table-column prop="name" label="步骤名称" min-width="150" />
            <el-table-column label="审批人类型" width="110">
              <template #default="{ row }">{{ approverTypeLabel(row.approver_type) }}</template>
            </el-table-column>
            <el-table-column prop="approver_id" label="审批人 ID" min-width="220" />
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="stepStateTagType(detail, row.step_no)" size="small">
                  {{ stepState(detail, row.step_no) }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>

          <el-divider content-position="left">审批动作</el-divider>
          <el-table :data="detail.actions" border size="small">
            <el-table-column prop="step_no" label="步骤" width="60" />
            <el-table-column label="动作" width="90">
              <template #default="{ row }">{{ actionLabel(row.action) }}</template>
            </el-table-column>
            <el-table-column prop="actor_id" label="操作人" min-width="220" />
            <el-table-column label="意见" min-width="180">
              <template #default="{ row }">{{ row.comment || '-' }}</template>
            </el-table-column>
            <el-table-column label="时间" width="180">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
          </el-table>
          <el-empty
            v-if="detail.actions.length === 0"
            description="暂无审批动作"
            :image-size="60"
          />
        </template>
        <el-empty v-else-if="!detailLoading" description="暂无审批详情" :image-size="60" />
      </div>
      <template #footer>
        <el-button @click="detailVisible = false">关闭</el-button>
        <template v-if="detail && detail.status === 'pending'">
          <el-button type="danger" plain @click="openReject(detail)">拒绝</el-button>
          <el-button type="success" @click="openApprove(detail)">批准</el-button>
        </template>
      </template>
    </el-dialog>

    <el-dialog v-model="approveVisible" title="批准审批" width="560px">
      <template v-if="actionTarget">
        <p class="dialog-hint">
          {{ documentTypeLabel(actionTarget.document_type) }} {{ actionTarget.document_code }} ·
          {{ currentStepLabel(actionTarget) }} · {{ actionTarget.amount }} {{ actionTarget.currency }}
        </p>
        <el-form label-width="90px">
          <el-form-item label="审批意见">
            <el-input
              v-model="comment"
              type="textarea"
              :rows="4"
              placeholder="可选，填写审批意见"
            />
          </el-form-item>
        </el-form>
      </template>
      <template #footer>
        <el-button @click="approveVisible = false">取消</el-button>
        <el-button type="success" :loading="saving" @click="submitApprove">确认批准</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="rejectVisible" title="拒绝审批" width="560px">
      <template v-if="actionTarget">
        <p class="dialog-hint">
          {{ documentTypeLabel(actionTarget.document_type) }} {{ actionTarget.document_code }} ·
          {{ currentStepLabel(actionTarget) }} · {{ actionTarget.amount }} {{ actionTarget.currency }}
        </p>
        <el-form label-width="90px">
          <el-form-item label="拒绝原因" required>
            <el-input
              v-model="comment"
              type="textarea"
              :rows="4"
              placeholder="请填写拒绝原因（必填）"
            />
          </el-form-item>
        </el-form>
      </template>
      <template #footer>
        <el-button @click="rejectVisible = false">取消</el-button>
        <el-button type="danger" :loading="saving" @click="submitReject">确认拒绝</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import {
  ApiError,
  approvalApi,
  purchaseApi,
  salesApi,
  type ApprovalRequest
} from '@mlf-erp/sdk'

defineOptions({ name: 'PendingApprovalView' })

const requests = ref<ApprovalRequest[]>([])
const loading = ref(false)
const saving = ref(false)

const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<ApprovalRequest | null>(null)

const approveVisible = ref(false)
const rejectVisible = ref(false)
const actionTarget = ref<ApprovalRequest | null>(null)
const comment = ref('')

function documentTypeLabel(value: string) {
  if (value === 'sales_order') return '销售订单'
  if (value === 'purchase_order') return '采购订单'
  return value || '-'
}

function approverTypeLabel(value: string) {
  if (value === 'role') return '角色'
  if (value === 'user') return '用户'
  return value || '-'
}

function statusLabel(value: string) {
  if (value === 'pending') return '审批中'
  if (value === 'approved') return '已通过'
  if (value === 'rejected') return '已拒绝'
  if (value === 'cancelled') return '已取消'
  return value || '-'
}

function statusTagType(value: string) {
  if (value === 'approved') return 'success'
  if (value === 'rejected') return 'danger'
  if (value === 'cancelled') return 'info'
  return 'warning'
}

function actionLabel(value: string) {
  if (value === 'approve') return '批准'
  if (value === 'reject') return '拒绝'
  if (value === 'cancel') return '取消'
  if (value === 'skip') return '已跳过'
  return value || '-'
}

function currentStepLabel(request: ApprovalRequest) {
  const total = request.steps.length
  const step = request.steps.find((item) => item.step_no === request.current_step)
  const progress = total > 0 ? `${request.current_step}/${total}` : String(request.current_step)
  return step ? `${progress} · ${step.name}` : progress
}

function stepState(request: ApprovalRequest, stepNo: number) {
  const action = request.actions.find((item) => item.step_no === stepNo)
  if (action) {
    if (action.action === 'approve') return '已通过'
    if (action.action === 'reject') return '已拒绝'
    if (action.action === 'skip') return '已跳过'
    return '已取消'
  }
  if (request.status === 'pending') {
    if (stepNo === request.current_step) return '待审批'
    if (stepNo < request.current_step) return '已处理'
    return '未开始'
  }
  if (request.status === 'approved') return '已通过'
  return '未开始'
}

function stepStateTagType(request: ApprovalRequest, stepNo: number) {
  const state = stepState(request, stepNo)
  if (state === '已通过') return 'success'
  if (state === '已拒绝') return 'danger'
  if (state === '待审批') return 'warning'
  return 'info'
}

function formatTime(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function approvalErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'SELF_APPROVAL':
        return '不能审批自己提交的申请'
      case 'NOT_APPROVER':
        return '当前用户不是本步骤的审批人'
      case 'ALREADY_ACTED':
        return '该步骤已审批，请刷新后重试'
      case 'INVALID_STATUS':
        return '单据当前状态不允许该操作'
      case 'NO_PENDING_APPROVAL':
        return '该单据没有待审批请求'
      case 'APPROVAL_NO_APPROVER':
        return '审批流程没有可用审批人，请联系管理员'
    }
  }
  return error instanceof Error ? error.message : fallback
}

async function loadPending() {
  loading.value = true
  try {
    const result = await approvalApi.pending()
    requests.value = result.items
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载待办审批失败')
  } finally {
    loading.value = false
  }
}

async function openDetail(row: ApprovalRequest) {
  detail.value = null
  detailVisible.value = true
  detailLoading.value = true
  try {
    detail.value = await approvalApi.request(row.id)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载审批详情失败')
    detailVisible.value = false
  } finally {
    detailLoading.value = false
  }
}

async function refreshDetail(requestID: string) {
  if (!detailVisible.value || detail.value?.id !== requestID) return
  try {
    detail.value = await approvalApi.request(requestID)
  } catch {
    // Keep the last loaded detail if the refresh fails.
  }
}

async function refreshAfterAction(requestID: string) {
  await loadPending()
  await refreshDetail(requestID)
}

function openApprove(row: ApprovalRequest) {
  actionTarget.value = row
  comment.value = ''
  approveVisible.value = true
}

function openReject(row: ApprovalRequest) {
  actionTarget.value = row
  comment.value = ''
  rejectVisible.value = true
}

async function submitApprove() {
  const target = actionTarget.value
  if (!target) return
  saving.value = true
  try {
    const input = comment.value.trim() ? { comment: comment.value.trim() } : undefined
    if (target.document_type === 'sales_order') {
      await salesApi.approve(target.document_id, input)
    } else {
      await purchaseApi.approve(target.document_id, input)
    }
    approveVisible.value = false
    ElMessage.success('审批已提交')
    await refreshAfterAction(target.id)
  } catch (error) {
    ElMessage.error(approvalErrorMessage(error, '审批失败'))
  } finally {
    saving.value = false
  }
}

async function submitReject() {
  const target = actionTarget.value
  if (!target) return
  const reason = comment.value.trim()
  if (!reason) {
    ElMessage.warning('请填写拒绝原因')
    return
  }
  saving.value = true
  try {
    if (target.document_type === 'sales_order') {
      await salesApi.reject(target.document_id, { comment: reason })
    } else {
      await purchaseApi.reject(target.document_id, { comment: reason })
    }
    rejectVisible.value = false
    ElMessage.success('已拒绝审批')
    await refreshAfterAction(target.id)
  } catch (error) {
    ElMessage.error(approvalErrorMessage(error, '拒绝审批失败'))
  } finally {
    saving.value = false
  }
}

onMounted(loadPending)
</script>

<style scoped>
.detail-body {
  min-height: 120px;
}

.dialog-hint {
  margin: 0 0 16px;
  color: #475569;
}
</style>
