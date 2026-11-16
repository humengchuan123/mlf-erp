import { api } from './client'

export interface PurchaseOrder {
  id: string
  code: string
  supplier_id: string
  amount: string
  currency: string
  status: string
  created_at: string
}

export interface CreatePurchaseOrderInput {
  code: string
  supplier_id?: string
  amount: string
  currency?: string
}

export const purchaseApi = {
  list(): Promise<PurchaseOrder[]> {
    return api.get<PurchaseOrder[]>('/purchase/orders')
  },
  create(input: CreatePurchaseOrderInput): Promise<PurchaseOrder> {
    return api.post<PurchaseOrder>('/purchase/orders', input)
  }
}
