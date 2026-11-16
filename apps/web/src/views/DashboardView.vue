<template>
  <div>
    <PageHeader
      title="工作台"
      description="ERP 骨架已就绪：平台底座、采购示例模块、运行时能力配置。"
    />

    <el-row :gutter="16">
      <el-col v-for="card in cards" :key="card.label" :xs="24" :sm="12" :lg="6">
        <el-card shadow="never" class="metric">
          <p class="metric__label">{{ card.label }}</p>
          <p class="metric__value">{{ card.value }}</p>
          <p class="metric__hint">{{ card.hint }}</p>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never" class="next">
      <template #header>下一步</template>
      <ul class="next__list">
        <li>接入 JWT 登录与 RBAC 数据权限。</li>
        <li>将采购订单内存仓储替换为 PostgreSQL 仓储。</li>
        <li>接入 Redis、MinIO、RabbitMQ 适配器。</li>
        <li>补齐销售、库存、财务模块的领域模型。</li>
      </ul>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { PageHeader } from '@mlf-erp/ui'

const cards = [
  { label: '本月订单', value: '0', hint: '骨架数据' },
  { label: '库存准确率', value: '--', hint: '待接入库存模块' },
  { label: '待审批', value: '0', hint: '待接入审批流' },
  { label: '交付模式', value: 'SaaS / 私有化', hint: '同一套核心代码' }
]
</script>

<style scoped>
.metric {
  margin-bottom: 16px;
}

.metric__label {
  margin: 0;
  color: #64748b;
  font-size: 13px;
}

.metric__value {
  margin: 10px 0 4px;
  color: #0f172a;
  font-size: 26px;
  font-weight: 600;
}

.metric__hint {
  margin: 0;
  color: #94a3b8;
  font-size: 12px;
}

.next {
  margin-top: 8px;
}

.next__list {
  margin: 0;
  padding-left: 20px;
  color: #475569;
  line-height: 2;
}
</style>
