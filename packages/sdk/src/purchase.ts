import { api } from './client'
import type { Paged } from './system'

export interface PurchaseOrderLine {
  id: string
  line_no: number
  material_id: string
  material_name: string
  quantity: string
  unit_price: string
  tax_rate_id: string
  tax_rate: string
  amount: string
  tax_amount: string
  gross_amount: string
  base_amount: string
  base_tax_amount: string
  base_gross_amount: string
  received_qty: string
  remaining_qty: string
}

export interface PurchaseOrder {
  id: string
  code: string
  supplier_id: string
  supplier_name: string
  warehouse_id: string
  warehouse_name: string
  order_date: string
  currency: string
  exchange_rate: string
  status: string
  price_includes_tax: boolean
  net_amount: string
  tax_amount: string
  total_amount: string
  base_net_amount: string
  base_tax_amount: string
  base_total_amount: string
  remark: string
  created_at: string
  updated_at: string
  lines: PurchaseOrderLine[]
}

export interface PurchaseOrderLineInput {
  material_id: string
  quantity: string
  unit_price: string
  tax_rate_id?: string
}

export interface PurchaseOrderInput {
  supplier_id: string
  warehouse_id: string
  order_date: string
  currency: string
  exchange_rate?: string
  price_includes_tax?: boolean
  remark?: string
  lines: PurchaseOrderLineInput[]
}

export interface PurchaseReceiptLine {
  order_line_id: string
  material_id: string
  material_name: string
  quantity: string
  unit_price: string
  tax_rate_id: string
  tax_rate: string
  amount: string
  tax_amount: string
  gross_amount: string
  base_amount: string
  base_tax_amount: string
  base_gross_amount: string
}

export interface PurchaseReceipt {
  id: string
  code: string
  order_id: string
  order_code: string
  supplier_name: string
  warehouse_id: string
  warehouse_name: string
  receipt_date: string
  status: string
  price_includes_tax: boolean
  exchange_rate: string
  net_amount: string
  tax_amount: string
  total_amount: string
  base_net_amount: string
  base_tax_amount: string
  base_total_amount: string
  remark: string
  payable_id: string
  payable_code: string
  payable_due_date: string
  voucher_code: string
  created_at: string
  lines: PurchaseReceiptLine[]
}

export interface ReceiveLineInput {
  order_line_id: string
  quantity: string
}

export interface ReceiveInput {
  receipt_date: string
  exchange_rate?: string
  remark?: string
  lines: ReceiveLineInput[]
}

function queryString(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') {
      search.set(key, String(value))
    }
  }
  const query = search.toString()
  return query ? `?${query}` : ''
}

export const purchaseApi = {
  list(params: { status?: string; keyword?: string; page?: number; page_size?: number } = {}): Promise<Paged<PurchaseOrder>> {
    return api.get<Paged<PurchaseOrder>>(`/purchase/orders${queryString(params)}`)
  },
  get(id: string): Promise<PurchaseOrder> {
    return api.get<PurchaseOrder>(`/purchase/orders/${id}`)
  },
  create(input: PurchaseOrderInput): Promise<PurchaseOrder> {
    return api.post<PurchaseOrder>('/purchase/orders', input)
  },
  update(id: string, input: PurchaseOrderInput): Promise<PurchaseOrder> {
    return api.put<PurchaseOrder>(`/purchase/orders/${id}`, input)
  },
  submit(id: string): Promise<unknown> {
    return api.post(`/purchase/orders/${id}/submit`)
  },
  approve(id: string): Promise<unknown> {
    return api.post(`/purchase/orders/${id}/approve`)
  },
  cancel(id: string): Promise<unknown> {
    return api.post(`/purchase/orders/${id}/cancel`)
  },
  receive(id: string, input: ReceiveInput): Promise<PurchaseReceipt> {
    return api.post<PurchaseReceipt>(`/purchase/orders/${id}/receive`, input)
  },
  receipts(params: { order_id?: string; page?: number; page_size?: number } = {}): Promise<Paged<PurchaseReceipt>> {
    return api.get<Paged<PurchaseReceipt>>(`/purchase/receipts${queryString(params)}`)
  }
}
