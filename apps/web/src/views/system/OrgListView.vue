<template>
  <div>
    <PageHeader title="组织管理" description="维护集团、公司、工厂、部门等组织树。">
      <el-button v-if="canWrite" type="primary" @click="openCreate()">新建根组织</el-button>
    </PageHeader>

    <el-card shadow="never">
      <el-tree
        v-loading="loading"
        :data="orgTree"
        node-key="id"
        default-expand-all
        :expand-on-click-node="false"
        :props="{ label: 'name', children: 'children' }"
        class="org-tree"
      >
        <template #default="{ data }">
          <span class="org-node">
            <span class="org-node__label">
              <strong>{{ data.name }}</strong>
              <el-tag size="small" effect="plain">{{ data.type }}</el-tag>
              <small>{{ data.code }}</small>
              <el-tag v-if="data.status !== 'active'" type="info" size="small">禁用</el-tag>
            </span>
            <span v-if="canWrite" class="org-node__actions">
              <el-button link type="primary" @click.stop="openCreate(data)">新增子组织</el-button>
              <el-button link type="primary" @click.stop="openEdit(data)">编辑</el-button>
              <el-button link type="danger" @click.stop="removeOrg(data)">删除</el-button>
            </span>
          </span>
        </template>
      </el-tree>
      <el-empty v-if="!loading && orgTree.length === 0" description="暂无组织，请新建根组织。" />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingOrg ? '编辑组织' : '新建组织'" width="520px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="上级组织">
          <el-input :model-value="parentName" disabled placeholder="无（根组织）" />
        </el-form-item>
        <el-form-item label="组织类型">
          <el-select v-model="form.type" style="width: 100%">
            <el-option label="集团" value="group" />
            <el-option label="公司" value="company" />
            <el-option label="工厂" value="factory" />
            <el-option label="部门" value="dept" />
            <el-option label="其他" value="other" />
          </el-select>
        </el-form-item>
        <el-form-item label="组织编码" required>
          <el-input v-model="form.code" placeholder="例如 WH-001" />
        </el-form-item>
        <el-form-item label="组织名称" required>
          <el-input v-model="form.name" placeholder="例如 华东仓库" />
        </el-form-item>
        <el-form-item v-if="editingOrg" label="状态">
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
import { ElMessage, ElMessageBox } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import { systemApi, type Organization } from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const canWrite = computed(() => authStore.hasPermission('system.org.write'))

const orgTree = ref<Organization[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const editingOrg = ref<Organization | null>(null)
const parentName = ref('')

const form = reactive({
  parentId: '',
  type: 'dept',
  code: '',
  name: '',
  status: 'active'
})

async function loadTree() {
  loading.value = true
  try {
    orgTree.value = await systemApi.orgs.list()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载组织失败')
  } finally {
    loading.value = false
  }
}

function openCreate(parent?: Organization) {
  editingOrg.value = null
  form.parentId = parent?.id ?? ''
  form.type = parent ? 'dept' : 'company'
  form.code = ''
  form.name = ''
  form.status = 'active'
  parentName.value = parent ? `${parent.name}（${parent.code}）` : '无（根组织）'
  dialogVisible.value = true
}

function openEdit(org: Organization) {
  editingOrg.value = org
  form.parentId = org.parent_id
  form.type = org.type
  form.code = org.code
  form.name = org.name
  form.status = org.status
  parentName.value = org.parent_id || '无（根组织）'
  dialogVisible.value = true
}

async function submit() {
  if (!form.code.trim() || !form.name.trim()) {
    ElMessage.warning('组织编码和名称不能为空')
    return
  }

  saving.value = true
  try {
    if (editingOrg.value) {
      await systemApi.orgs.update(editingOrg.value.id, {
        parent_id: form.parentId,
        type: form.type,
        code: form.code.trim(),
        name: form.name.trim(),
        status: form.status
      })
      ElMessage.success('组织已更新')
    } else {
      await systemApi.orgs.create({
        parent_id: form.parentId,
        type: form.type,
        code: form.code.trim(),
        name: form.name.trim()
      })
      ElMessage.success('组织已创建')
    }
    dialogVisible.value = false
    await loadTree()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存组织失败')
  } finally {
    saving.value = false
  }
}

async function removeOrg(org: Organization) {
  try {
    await ElMessageBox.confirm(`确认删除组织 ${org.name}？`, '删除组织', { type: 'warning' })
  } catch {
    return
  }

  try {
    await systemApi.orgs.remove(org.id)
    ElMessage.success('组织已删除')
    await loadTree()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '删除组织失败')
  }
}

onMounted(loadTree)
</script>

<style scoped>
.org-tree {
  padding: 4px 0;
}

.org-node {
  display: flex;
  flex: 1;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-right: 8px;
}

.org-node__label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.org-node__label small {
  color: #94a3b8;
}

.org-node__actions {
  flex-shrink: 0;
}
</style>
