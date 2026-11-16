<template>
  <div>
    <PageHeader :title="title" :description="description">
      <el-button v-if="canWrite" type="primary" @click="openCreate">新建</el-button>
    </PageHeader>

    <el-card shadow="never">
      <div class="toolbar">
        <el-input
          v-model="keyword"
          :placeholder="searchPlaceholder ?? '输入编码或名称搜索'"
          clearable
          class="toolbar__search"
          @keyup.enter="search"
        />
        <el-button type="primary" plain @click="search">查询</el-button>
        <el-button @click="resetSearch">重置</el-button>
      </div>

      <el-table v-loading="loading" :data="rows" border stripe>
        <el-table-column
          v-for="column in columns"
          :key="column.key"
          :prop="column.key"
          :label="column.label"
          :width="column.width"
          :min-width="column.minWidth"
          show-overflow-tooltip
        >
          <template v-if="column.formatter" #default="{ row }">
            {{ column.formatter(row) }}
          </template>
        </el-table-column>
        <el-table-column v-if="canWrite" label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button
              v-if="row.status !== 'disabled'"
              link
              type="danger"
              @click="disableRow(row)"
            >
              禁用
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        class="pagination"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        @current-change="load"
        @size-change="handleSizeChange"
      />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑' : '新建'" width="560px">
      <el-form :model="form" label-width="100px">
        <el-form-item
          v-for="field in fields"
          :key="field.key"
          :label="field.label"
          :required="field.required"
        >
          <el-input
            v-if="!field.type || field.type === 'input'"
            v-model="form[field.key]"
            :placeholder="field.placeholder"
            :disabled="Boolean(editing && field.disabledOnEdit)"
          />
          <el-input
            v-else-if="field.type === 'textarea'"
            v-model="form[field.key]"
            type="textarea"
            :rows="3"
            :placeholder="field.placeholder"
            :disabled="Boolean(editing && field.disabledOnEdit)"
          />
          <el-select
            v-else-if="field.type === 'select'"
            v-model="form[field.key]"
            style="width: 100%"
            :placeholder="field.placeholder ?? '请选择'"
            :disabled="Boolean(editing && field.disabledOnEdit)"
          >
            <el-option
              v-for="option in field.options ?? []"
              :key="option.value"
              :label="option.label"
              :value="option.value"
            />
          </el-select>
          <el-input-number
            v-else-if="field.type === 'number'"
            v-model="form[field.key]"
            :min="field.min"
            :max="field.max"
            :step="field.step ?? 1"
            :disabled="Boolean(editing && field.disabledOnEdit)"
            style="width: 100%"
          />
          <el-switch
            v-else-if="field.type === 'switch'"
            v-model="form[field.key]"
            :disabled="Boolean(editing && field.disabledOnEdit)"
          />
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
import { ElMessage, ElMessageBox } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import { useAuthStore } from '@/stores/auth'
import type { ColumnConfig, CrudApi, FieldConfig } from './masterdata-types'

const props = defineProps<{
  title: string
  description?: string
  permission: string
  api: CrudApi
  columns: ColumnConfig[]
  fields: FieldConfig[]
  defaultForm: Record<string, any>
  searchPlaceholder?: string
}>()

const authStore = useAuthStore()
const canWrite = computed(() => authStore.hasPermission(`${props.permission}.write`))

const rows = ref<Record<string, any>[]>([])
const loading = ref(false)
const saving = ref(false)
const keyword = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const dialogVisible = ref(false)
const editing = ref<Record<string, any> | null>(null)
const form = reactive<Record<string, any>>({})

function resetForm() {
  for (const key of Object.keys(form)) {
    delete form[key]
  }
  Object.assign(form, JSON.parse(JSON.stringify(props.defaultForm)))
}

async function load() {
  loading.value = true
  try {
    const result = await props.api.list({
      keyword: keyword.value,
      page: page.value,
      page_size: pageSize.value
    })
    rows.value = result.items
    total.value = result.total
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载数据失败')
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function resetSearch() {
  keyword.value = ''
  page.value = 1
  load()
}

function handleSizeChange() {
  page.value = 1
  load()
}

function openCreate() {
  editing.value = null
  resetForm()
  dialogVisible.value = true
}

function openEdit(row: Record<string, any>) {
  editing.value = row
  resetForm()
  for (const field of props.fields) {
    if (row[field.key] !== undefined && row[field.key] !== null) {
      form[field.key] = row[field.key]
    }
  }
  dialogVisible.value = true
}

async function submit() {
  for (const field of props.fields) {
    if (field.required && (form[field.key] === undefined || form[field.key] === '' || form[field.key] === null)) {
      ElMessage.warning(`${field.label}不能为空`)
      return
    }
  }

  saving.value = true
  try {
    if (editing.value) {
      await props.api.update(String(editing.value.id), { ...form })
      ElMessage.success('已更新')
    } else {
      await props.api.create({ ...form })
      ElMessage.success('已创建')
    }
    dialogVisible.value = false
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存失败')
  } finally {
    saving.value = false
  }
}

async function disableRow(row: Record<string, any>) {
  try {
    await ElMessageBox.confirm(`确认禁用 ${row.name ?? row.code}？`, '禁用', { type: 'warning' })
  } catch {
    return
  }

  try {
    await props.api.remove(String(row.id))
    ElMessage.success('已禁用')
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '禁用失败')
  }
}

onMounted(load)
</script>

<style scoped>
.toolbar {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}

.toolbar__search {
  width: 300px;
}

.pagination {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>
