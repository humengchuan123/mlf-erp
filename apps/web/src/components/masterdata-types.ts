export interface FieldOption {
  label: string
  value: string
}

export interface FieldConfig {
  key: string
  label: string
  type?: 'input' | 'textarea' | 'select' | 'number' | 'switch'
  required?: boolean
  placeholder?: string
  options?: FieldOption[]
  min?: number
  max?: number
  step?: number
  disabledOnEdit?: boolean
}

export interface ColumnConfig {
  key: string
  label: string
  width?: number
  minWidth?: number
  formatter?: (row: Record<string, any>) => string
}

export interface CrudApi {
  list: (params: { keyword?: string; page?: number; page_size?: number }) => Promise<{
    items: Record<string, any>[]
    total: number
  }>
  create: (input: Record<string, any>) => Promise<unknown>
  update: (id: string, input: Record<string, any>) => Promise<unknown>
  remove: (id: string) => Promise<unknown>
  enable: (id: string) => Promise<unknown>
}
