import { api } from './client'
import type { Paged } from './system'

export interface SalesOrderLine {
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
  shipped_qty: string
  remaining_qty: string
}

export interface SalesOrder {
  id: string
  code: string
  customer_id: string
  customer_name: string
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
  lines: SalesOrderLine[]
}

export interface SalesOrderLineInput {
  material_id: string
  quantity: string
  unit_price: string
  tax_rate_id?: string
}

export interface SalesOrderInput {
  customer_id: string
  warehouse_id: string
  order_date: string
  currency: string
  exchange_rate?: string
  price_includes_tax?: boolean
  remark?: string
  lines: SalesOrderLineInput[]
}

export interface ShipmentLine {
  id: string
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
  unit_cost: string
  cost_amount: string
  returned_qty: string
  returnable_qty: string
}

export interface Shipment {
  id: string
  code: string
  order_id: string
  order_code: string
  customer_id: string
  customer_name: string
  warehouse_id: string
  warehouse_name: string
  shipment_date: string
  status: string
  price_includes_tax: boolean
  exchange_rate: string
  net_amount: string
  tax_amount: string
  base_net_amount: string
  base_tax_amount: string
  base_total_amount: string
  remark: string
  invoice_id: string
  invoice_code: string
  invoice_due_date: string
  voucher_code: string
  total_amount: string
  total_cost: string
  created_at: string
  lines: ShipmentLine[]
}

export interface ShipLineInput {
  order_line_id: string
  quantity: string
}

export interface ShipInput {
  shipment_date: string
  exchange_rate?: string
  remark?: string
  lines: ShipLineInput[]
}

export interface SalesReturnLine {
  id: string
  line_no: number
  shipment_line_id: string
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
  unit_cost: string
  cost_amount: string
}

export interface SalesReturn {
  id: string
  code: string
  shipment_id: string
  shipment_code: string
  order_id: string
  order_code: string
  customer_id: string
  customer_name: string
  warehouse_id: string
  warehouse_name: string
  return_date: string
  status: string
  currency: string
  exchange_rate: string
  price_includes_tax: boolean
  net_amount: string
  tax_amount: string
  total_amount: string
  base_net_amount: string
  base_tax_amount: string
  base_total_amount: string
  cost_amount: string
  credit_note_id: string
  credit_note_code: string
  credit_note_amount: string
  credit_note_applied_amount: string
  credit_note_remaining_amount: string
  credit_note_status: string
  voucher_code: string
  reason: string
  remark: string
  created_at: string
  lines: SalesReturnLine[]
}

export interface CreateSalesReturnLineInput {
  shipment_line_id: string
  quantity: string
}

export interface CreateSalesReturnInput {
  return_date?: string
  reason?: string
  remark?: string
  lines: CreateSalesReturnLineInput[]
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

export const salesApi = {
  list(params: { status?: string; keyword?: string; page?: number; page_size?: number } = {}): Promise<Paged<SalesOrder>> {
    return api.get<Paged<SalesOrder>>(`/sales/orders${queryString(params)}`)
  },
  get(id: string): Promise<SalesOrder> {
    return api.get<SalesOrder>(`/sales/orders/${id}`)
  },
  create(input: SalesOrderInput): Promise<SalesOrder> {
    return api.post<SalesOrder>('/sales/orders', input)
  },
  update(id: string, input: SalesOrderInput): Promise<SalesOrder> {
    return api.put<SalesOrder>(`/sales/orders/${id}`, input)
  },
  submit(id: string): Promise<unknown> {
    return api.post(`/sales/orders/${id}/submit`)
  },
  approve(id: string): Promise<unknown> {
    return api.post(`/sales/orders/${id}/approve`)
  },
  cancel(id: string): Promise<unknown> {
    return api.post(`/sales/orders/${id}/cancel`)
  },
  ship(id: string, input: ShipInput): Promise<Shipment> {
    return api.post<Shipment>(`/sales/orders/${id}/ship`, input)
  },
  shipments(params: { order_id?: string; page?: number; page_size?: number } = {}): Promise<Paged<Shipment>> {
    return api.get<Paged<Shipment>>(`/sales/shipments${queryString(params)}`)
  },
  returns: {
    list(params: {
      shipment_id?: string
      customer_id?: string
      keyword?: string
      start_date?: string
      end_date?: string
      page?: number
      page_size?: number
    } = {}): Promise<Paged<SalesReturn>> {
      return api.get<Paged<SalesReturn>>(`/sales/returns${queryString(params)}`)
    },
    get(id: string): Promise<SalesReturn> {
      return api.get<SalesReturn>(`/sales/returns/${id}`)
    },
    create(shipmentID: string, input: CreateSalesReturnInput): Promise<SalesReturn> {
      return api.post<SalesReturn>(`/sales/shipments/${shipmentID}/returns`, input)
    }
  }
}
