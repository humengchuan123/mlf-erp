import { api } from './client'
import type { Paged } from './system'

export type ApprovalDocumentType = 'sales_order' | 'purchase_order'
export type ApprovalConditionType = 'always' | 'amount_gte'
export type ApprovalApproverType = 'role' | 'user'
export type ApprovalRequestStatus = 'pending' | 'approved' | 'rejected' | 'cancelled'
export type ApprovalActionType = 'approve' | 'reject' | 'cancel'

export interface ApprovalFlowStep {
  id: string
  step_no: number
  name: string
  approver_type: ApprovalApproverType
  approver_id: string
  created_at?: string
}

export interface ApprovalFlow {
  id: string
  tenant_id: string
  /** Empty string means a tenant-level flow; otherwise the organization scope. */
  org_id: string
  name: string
  document_type: ApprovalDocumentType
  condition_type: ApprovalConditionType
  threshold_amount: string
  priority: number
  active: boolean
  created_by: string
  created_at: string
  updated_at: string
  steps: ApprovalFlowStep[]
}

export interface ApprovalFlowStepInput {
  step_no: number
  name: string
  approver_type: ApprovalApproverType
  approver_id: string
}

export interface CreateApprovalFlowInput {
  name: string
  document_type: ApprovalDocumentType
  org_id?: string
  condition_type: ApprovalConditionType
  threshold_amount: string
  priority: number
  active: boolean
  steps: ApprovalFlowStepInput[]
}

export interface ApprovalAction {
  id: string
  step_no: number
  action: ApprovalActionType
  actor_id: string
  comment: string
  created_at: string
}

export interface ApprovalRequest {
  id: string
  tenant_id: string
  org_id: string
  document_type: ApprovalDocumentType
  document_id: string
  document_code: string
  flow_id: string
  status: ApprovalRequestStatus
  current_step: number
  requested_by: string
  requested_at: string
  completed_at?: string | null
  amount: string
  currency: string
  base_amount: string
  steps: ApprovalFlowStep[]
  actions: ApprovalAction[]
}

export type ApprovalRequestListParams = {
  document_type?: string
  document_id?: string
  status?: string
  page?: number
  page_size?: number
}

function queryString(params: Record<string, string | number | boolean | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') {
      search.set(key, typeof value === 'boolean' ? (value ? 'true' : 'false') : String(value))
    }
  }
  const query = search.toString()
  return query ? `?${query}` : ''
}

export const approvalApi = {
  flows(params: { document_type?: string; active?: boolean } = {}): Promise<{ items: ApprovalFlow[] }> {
    return api.get<{ items: ApprovalFlow[] }>(`/system/approval-flows${queryString(params)}`)
  },
  flow(id: string): Promise<ApprovalFlow> {
    return api.get<ApprovalFlow>(`/system/approval-flows/${id}`)
  },
  createFlow(input: CreateApprovalFlowInput): Promise<ApprovalFlow> {
    return api.post<ApprovalFlow>('/system/approval-flows', input)
  },
  updateFlow(id: string, input: CreateApprovalFlowInput): Promise<ApprovalFlow> {
    return api.put<ApprovalFlow>(`/system/approval-flows/${id}`, input)
  },
  removeFlow(id: string): Promise<unknown> {
    return api.delete(`/system/approval-flows/${id}`)
  },
  setFlowActive(id: string, active: boolean): Promise<ApprovalFlow> {
    return api.post<ApprovalFlow>(`/system/approval-flows/${id}/active`, { active })
  },
  pending(): Promise<{ items: ApprovalRequest[] }> {
    return api.get<{ items: ApprovalRequest[] }>('/approvals/pending')
  },
  requests(params: ApprovalRequestListParams = {}): Promise<Paged<ApprovalRequest>> {
    return api.get<Paged<ApprovalRequest>>(`/approvals/requests${queryString(params)}`)
  },
  request(id: string): Promise<ApprovalRequest> {
    return api.get<ApprovalRequest>(`/approvals/requests/${id}`)
  }
}
