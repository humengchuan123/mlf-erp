import { api } from './client'
import type { Paged } from './system'

export interface Partner {
  id: string
  tenant_id: string
  org_id: string
  code: string
  name: string
  type: string
  category: string
  contact: string
  phone: string
  email: string
  address: string
  tax_no: string
  status: string
  created_by: string
  updated_by: string
  created_at: string
  updated_at: string
}

export interface PartnerInput {
  code?: string
  name: string
  type?: string
  category?: string
  contact?: string
  phone?: string
  email?: string
  address?: string
  tax_no?: string
}

export interface Material {
  id: string
  tenant_id: string
  org_id: string
  code: string
  name: string
  spec: string
  category: string
  unit_id: string
  tax_rate_id: string
  barcode: string
  status: string
  created_by: string
  updated_by: string
  created_at: string
  updated_at: string
}

export interface MaterialInput {
  code?: string
  name: string
  spec?: string
  category?: string
  unit_id?: string
  tax_rate_id?: string
  barcode?: string
}

export interface Warehouse {
  id: string
  tenant_id: string
  org_id: string
  code: string
  name: string
  type: string
  address: string
  status: string
  created_by: string
  updated_by: string
  created_at: string
  updated_at: string
}

export interface WarehouseInput {
  code?: string
  name: string
  type?: string
  address?: string
}

export interface Unit {
  id: string
  tenant_id: string
  code: string
  name: string
  precision: number
  status: string
  created_at: string
  updated_at: string
}

export interface UnitInput {
  code: string
  name: string
  precision: number
}

export interface Currency {
  id: string
  tenant_id: string
  code: string
  name: string
  symbol: string
  decimal_places: number
  status: string
  created_at: string
  updated_at: string
}

export interface CurrencyInput {
  code: string
  name: string
  symbol: string
  decimal_places: number
}

export interface TaxRate {
  id: string
  tenant_id: string
  code: string
  name: string
  rate: string
  type: string
  status: string
  created_at: string
  updated_at: string
}

export interface TaxRateInput {
  code: string
  name: string
  rate: string
  type: string
}

export interface CrudApi<T, I> {
  list(params?: { keyword?: string; page?: number; page_size?: number }): Promise<Paged<T>>
  get(id: string): Promise<T>
  create(input: I): Promise<T>
  update(id: string, input: I): Promise<T>
  remove(id: string): Promise<{ id: string; status: string }>
  enable(id: string): Promise<{ id: string; status: string }>
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

function createCrud<T, I>(basePath: string): CrudApi<T, I> {
  return {
    list(params = {}) {
      return api.get<Paged<T>>(`${basePath}${queryString(params)}`)
    },
    get(id) {
      return api.get<T>(`${basePath}/${id}`)
    },
    create(input) {
      return api.post<T>(basePath, input)
    },
    update(id, input) {
      return api.put<T>(`${basePath}/${id}`, input)
    },
    remove(id) {
      return api.delete<{ id: string; status: string }>(`${basePath}/${id}`)
    },
    enable(id) {
      return api.post<{ id: string; status: string }>(`${basePath}/${id}/enable`)
    }
  }
}

export const masterDataApi = {
  partners: createCrud<Partner, PartnerInput>('/masterdata/partners'),
  materials: createCrud<Material, MaterialInput>('/masterdata/materials'),
  warehouses: createCrud<Warehouse, WarehouseInput>('/masterdata/warehouses'),
  units: createCrud<Unit, UnitInput>('/masterdata/units'),
  currencies: createCrud<Currency, CurrencyInput>('/masterdata/currencies'),
  taxRates: createCrud<TaxRate, TaxRateInput>('/masterdata/tax-rates')
}
