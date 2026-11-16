import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { DEFAULT_CAPABILITIES, type Capability, type DeliveryMode } from '@mlf-erp/shared'
import { runtimeApi } from '@mlf-erp/sdk'

export const useAppStore = defineStore('app', () => {
  const token = ref(localStorage.getItem('mlf_token') ?? '')
  const userName = ref(localStorage.getItem('mlf_user') ?? 'Demo User')
  const appName = ref('MLF ERP')
  const deliveryMode = ref<DeliveryMode>('saas')
  const capabilities = ref<Capability[]>([...DEFAULT_CAPABILITIES])

  const deliveryModeLabel = computed(() => (deliveryMode.value === 'saas' ? 'SaaS' : '私有化'))

  function login(user: string) {
    userName.value = user
    token.value = `dev-${user}`
    localStorage.setItem('mlf_token', token.value)
    localStorage.setItem('mlf_user', userName.value)
  }

  function logout() {
    token.value = ''
    localStorage.removeItem('mlf_token')
  }

  function hasCapability(capability: Capability) {
    return capabilities.value.includes(capability)
  }

  async function loadRuntimeConfig() {
    const config = await runtimeApi.get()
    appName.value = config.app
    deliveryMode.value = config.delivery_mode
    capabilities.value = config.capabilities as Capability[]
  }

  return {
    token,
    userName,
    appName,
    deliveryMode,
    deliveryModeLabel,
    capabilities,
    login,
    logout,
    hasCapability,
    loadRuntimeConfig
  }
})
