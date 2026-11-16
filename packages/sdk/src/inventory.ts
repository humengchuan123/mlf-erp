import { api } from './client'
import type { Paged } from './system'

export interface InventoryBalance {
  tenant_id: string
  org_id: string
  warehouse_id: string
  warehouse_name: string
  material_id: string
  material_name: string
  on_hand: string
  reserved: string
  available: string
  updated_at: string
}

export interface InventoryTransaction {
  id: string
  tenant_id: string
  org_id: string
  warehouse_id: string
  warehouse_name: string
  material_id: string
  material_name: string
  direction: string
  biz_type: string
  biz_id: string
  biz_code: string
  quantity: string
  unit_cost: string
  amount: string
  occurred_at: string
  created_by: string
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

export const inventoryApi = {
  balances(params: { warehouse_id?: string; material_id?: string; keyword?: string; page?: number; page_size?: number } = {}): Promise<Paged<InventoryBalance>> {
    return api.get<Paged<InventoryBalance>>(`/inventory/balances${queryString(params)}`)
  },
  transactions(params: { warehouse_id?: string; material_id?: string; keyword?: string; page?: number; page_size?: number } = {}): Promise<Paged<InventoryTransaction>> {
    return api.get<Paged<InventoryTransaction>>(`/inventory/transactions${queryString(params)}`)
  }
}
