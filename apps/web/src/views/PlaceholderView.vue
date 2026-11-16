<template>
  <div>
    <PageHeader :title="title" :description="description" />
    <el-card shadow="never">
      <el-empty :description="`${title}模块骨架已创建，待接入领域模型与 API。`" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { PageHeader } from '@mlf-erp/ui'

const route = useRoute()
const title = computed(() => (route.meta.title as string) ?? '模块')
const description = computed(
  () => (route.meta.description as string) ?? '模块骨架，待接入业务功能。'
)
</script>
