export const CAPABILITIES = {
  PLATFORM_ORG: 'platform.org',
  PLATFORM_RBAC: 'platform.rbac',
  PURCHASE: 'purchase',
  SALES: 'sales',
  INVENTORY: 'inventory',
  FINANCE: 'finance',
  MANUFACTURING: 'manufacturing',
  QUALITY: 'quality',
  PROJECT: 'project',
  REPORTING: 'reporting'
} as const

export type Capability = (typeof CAPABILITIES)[keyof typeof CAPABILITIES]

export const DEFAULT_CAPABILITIES: Capability[] = Object.values(CAPABILITIES)

export type DeliveryMode = 'saas' | 'onprem'
