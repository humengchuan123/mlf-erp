<template>
  <router-view />
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()

onMounted(async () => {
  try {
    await appStore.loadRuntimeConfig()
  } catch {
    // The frontend can run without the API during early development.
  }
})
</script>
