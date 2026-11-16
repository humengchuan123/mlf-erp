<template>
  <div>
    <PageHeader title="编码规则" description="配置单据和主数据的自动编码规则。">
      <el-button v-if="canWrite" type="primary" @click="openCreate">新建规则</el-button>
    </PageHeader>

    <el-card shadow="never">
      <el-table v-loading="loading" :data="rules" border stripe>
        <el-table-column prop="entity" label="实体" min-width="150" />
        <el-table-column prop="prefix" label="前缀" width="100" />
        <el-table-column prop="date_pattern" label="日期模式" width="120" />
        <el-table-column prop="separator" label="分隔符" width="90" />
        <el-table-column prop="sequence_length" label="流水号长度" width="110" />
        <el-table-column prop="current_value" label="当前流水" width="100" />
        <el-table-column prop="reset_policy" label="重置策略" width="110" />
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">
              {{ row.status === 'active' ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column v-if="canWrite" label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑编码规则' : '新建编码规则'" width="560px">
      <el-form :model="form" label-width="110px">
        <el-form-item label="实体" required>
          <el-input
            v-model="form.entity"
            :disabled="Boolean(editing)"
            placeholder="例如 partner / material / purchase_order"
          />
        </el-form-item>
        <el-form-item label="前缀" required>
          <el-input v-model="form.prefix" placeholder="例如 CUS" />
        </el-form-item>
        <el-form-item label="日期模式">
          <el-select v-model="form.date_pattern" style="width: 100%">
            <el-option label="无日期" value="none" />
            <el-option label="年 yyyy" value="yyyy" />
            <el-option label="年月 yyyyMM" value="yyyyMM" />
            <el-option label="年月日 yyyyMMdd" value="yyyyMMdd" />
          </el-select>
        </el-form-item>
        <el-form-item label="分隔符">
          <el-input v-model="form.separator" placeholder="-" />
        </el-form-item>
        <el-form-item label="流水号长度">
          <el-input-number v-model="form.sequence_length" :min="1" :max="10" style="width: 100%" />
        </el-form-item>
        <el-form-item label="重置策略">
          <el-select v-model="form.reset_policy" style="width: 100%">
            <el-option label="不重置" value="never" />
            <el-option label="按年" value="year" />
            <el-option label="按月" value="month" />
            <el-option label="按日" value="day" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="form.status" style="width: 100%">
            <el-option label="启用" value="active" />
            <el-option label="禁用" value="disabled" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import { systemApi, type CodeRule } from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const canWrite = computed(() => authStore.hasPermission('system.code_rule.write'))

const rules = ref<CodeRule[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const editing = ref<CodeRule | null>(null)

const form = reactive({
  entity: '',
  prefix: '',
  date_pattern: 'yyyyMMdd',
  separator: '-',
  sequence_length: 4,
  reset_policy: 'day',
  status: 'active'
})

async function load() {
  loading.value = true
  try {
    rules.value = await systemApi.codeRules.list()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载编码规则失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  Object.assign(form, {
    entity: '',
    prefix: '',
    date_pattern: 'yyyyMMdd',
    separator: '-',
    sequence_length: 4,
    reset_policy: 'day',
    status: 'active'
  })
  dialogVisible.value = true
}

function openEdit(rule: CodeRule) {
  editing.value = rule
  Object.assign(form, {
    entity: rule.entity,
    prefix: rule.prefix,
    date_pattern: rule.date_pattern,
    separator: rule.separator,
    sequence_length: rule.sequence_length,
    reset_policy: rule.reset_policy,
    status: rule.status
  })
  dialogVisible.value = true
}

async function submit() {
  if (!form.entity.trim() || !form.prefix.trim()) {
    ElMessage.warning('实体和前缀不能为空')
    return
  }

  saving.value = true
  try {
    if (editing.value) {
      await systemApi.codeRules.update(editing.value.id, { ...form })
      ElMessage.success('编码规则已更新')
    } else {
      await systemApi.codeRules.create({ ...form })
      ElMessage.success('编码规则已创建')
    }
    dialogVisible.value = false
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存编码规则失败')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
