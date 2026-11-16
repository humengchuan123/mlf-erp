import { api } from './client'

export interface RuntimeConfig {
  app: string
  env: string
  delivery_mode: 'saas' | 'onprem'
  capabilities: string[]
}

export const runtimeApi = {
  get(): Promise<RuntimeConfig> {
    return api.get<RuntimeConfig>('/runtime-config')
  }
}
