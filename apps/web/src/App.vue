<template>
  <router-view />
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const appStore = useAppStore()
const authStore = useAuthStore()

onMounted(async () => {
  try {
    if (authStore.hasToken) {
      await authStore.loadMe()
    }
  } catch {
    authStore.clear()
  }

  try {
    await appStore.loadRuntimeConfig()
  } catch {
    // Runtime config is best-effort; the API may not be ready yet.
  }
})
</script>
