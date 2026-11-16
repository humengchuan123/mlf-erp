<template>
  <div>
    <PageHeader title="角色权限" description="维护角色及其权限码，权限变更在下次请求时生效。">
      <el-button v-if="canWrite" type="primary" @click="openCreate">新建角色</el-button>
    </PageHeader>

    <el-card shadow="never">
      <el-table v-loading="loading" :data="roles" border stripe>
        <el-table-column prop="code" label="角色编码" min-width="140" />
        <el-table-column prop="name" label="角色名称" min-width="160" />
        <el-table-column prop="is_system" label="系统角色" width="100">
          <template #default="{ row }">
            <el-tag v-if="row.is_system" type="warning" size="small">是</el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="权限数" width="90">
          <template #default="{ row }">{{ row.permissions.length }}</template>
        </el-table-column>
        <el-table-column v-if="canWrite" label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="primary" @click="openPermissions(row)">分配权限</el-button>
            <el-button
              v-if="!row.is_system"
              link
              type="danger"
              @click="removeRole(row)"
            >
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingRole ? '编辑角色' : '新建角色'" width="480px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="角色编码" required>
          <el-input v-model="form.code" placeholder="例如 warehouse_manager" />
        </el-form-item>
        <el-form-item label="角色名称" required>
          <el-input v-model="form.name" placeholder="例如 仓库主管" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="permissionVisible" title="分配权限" width="640px">
      <p class="permission-role">
        {{ selectedRole?.name }}（{{ selectedRole?.code }}）
      </p>
      <div class="permission-list">
        <div v-for="group in permissionGroups" :key="group.module" class="permission-group">
          <h4>{{ group.module }}</h4>
          <el-checkbox-group v-model="selectedCodes">
            <el-checkbox
              v-for="permission in group.items"
              :key="permission.code"
              :value="permission.code"
            >
              {{ permission.name }}（{{ permission.code }}）
            </el-checkbox>
          </el-checkbox-group>
        </div>
      </div>
      <template #footer>
        <el-button @click="permissionVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="savePermissions">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import { systemApi, type Permission, type SystemRole } from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const canWrite = computed(() => authStore.hasPermission('system.role.write'))

const roles = ref<SystemRole[]>([])
const permissions = ref<Permission[]>([])
const loading = ref(false)
const saving = ref(false)

const dialogVisible = ref(false)
const editingRole = ref<SystemRole | null>(null)
const form = reactive({ code: '', name: '' })

const permissionVisible = ref(false)
const selectedRole = ref<SystemRole | null>(null)
const selectedCodes = ref<string[]>([])

const permissionGroups = computed(() => {
  const grouped = new Map<string, Permission[]>()
  for (const permission of permissions.value) {
    const list = grouped.get(permission.module) ?? []
    list.push(permission)
    grouped.set(permission.module, list)
  }
  return Array.from(grouped.entries()).map(([module, items]) => ({ module, items }))
})

async function loadData() {
  loading.value = true
  try {
    const [roleList, permissionList] = await Promise.all([
      systemApi.roles.list(),
      systemApi.permissions.list()
    ])
    roles.value = roleList
    permissions.value = permissionList
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载角色权限失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingRole.value = null
  form.code = ''
  form.name = ''
  dialogVisible.value = true
}

function openEdit(role: SystemRole) {
  editingRole.value = role
  form.code = role.code
  form.name = role.name
  dialogVisible.value = true
}

async function submit() {
  if (!form.code.trim() || !form.name.trim()) {
    ElMessage.warning('角色编码和名称不能为空')
    return
  }

  saving.value = true
  try {
    if (editingRole.value) {
      await systemApi.roles.update(editingRole.value.id, {
        code: form.code.trim(),
        name: form.name.trim()
      })
      ElMessage.success('角色已更新')
    } else {
      await systemApi.roles.create({ code: form.code.trim(), name: form.name.trim() })
      ElMessage.success('角色已创建')
    }
    dialogVisible.value = false
    await loadData()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存角色失败')
  } finally {
    saving.value = false
  }
}

async function removeRole(role: SystemRole) {
  try {
    await ElMessageBox.confirm(`确认删除角色 ${role.name}？`, '删除角色', { type: 'warning' })
  } catch {
    return
  }

  try {
    await systemApi.roles.remove(role.id)
    ElMessage.success('角色已删除')
    await loadData()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '删除角色失败')
  }
}

function openPermissions(role: SystemRole) {
  selectedRole.value = role
  selectedCodes.value = [...role.permissions]
  permissionVisible.value = true
}

async function savePermissions() {
  if (!selectedRole.value) {
    return
  }
  saving.value = true
  try {
    await systemApi.roles.setPermissions(selectedRole.value.id, selectedCodes.value)
    ElMessage.success('权限已更新')
    permissionVisible.value = false
    await loadData()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存权限失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadData)
</script>

<style scoped>
.permission-role {
  margin: 0 0 16px;
  color: #475569;
}

.permission-list {
  max-height: 420px;
  overflow: auto;
  padding-right: 8px;
}

.permission-group {
  margin-bottom: 18px;
}

.permission-group h4 {
  margin: 0 0 8px;
  color: #1f2d3d;
  font-size: 14px;
}

.permission-group :deep(.el-checkbox) {
  display: flex;
  margin-right: 0;
  line-height: 1.8;
}
</style>
