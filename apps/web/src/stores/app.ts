import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { DEFAULT_CAPABILITIES, type Capability, type DeliveryMode } from '@mlf-erp/shared'
import { runtimeApi } from '@mlf-erp/sdk'

export const useAppStore = defineStore('app', () => {
  const appName = ref('MLF ERP')
  const deliveryMode = ref<DeliveryMode>('saas')
  const capabilities = ref<Capability[]>([...DEFAULT_CAPABILITIES])

  const deliveryModeLabel = computed(() => (deliveryMode.value === 'saas' ? 'SaaS' : '私有化'))

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
    appName,
    deliveryMode,
    deliveryModeLabel,
    capabilities,
    hasCapability,
    loadRuntimeConfig
  }
})
