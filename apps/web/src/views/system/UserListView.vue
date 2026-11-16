<template>
  <div>
    <PageHeader title="用户管理" description="维护用户、状态与角色分配。">
      <el-button v-if="canWrite" type="primary" @click="openCreate">新建用户</el-button>
    </PageHeader>

    <el-card shadow="never">
      <div class="toolbar">
        <el-input
          v-model="keyword"
          placeholder="搜索用户名、姓名或邮箱"
          clearable
          class="toolbar__search"
          @keyup.enter="search"
        />
        <el-button type="primary" plain @click="search">查询</el-button>
        <el-button @click="resetSearch">重置</el-button>
      </div>

      <el-table v-loading="loading" :data="users" border stripe>
        <el-table-column prop="username" label="用户名" min-width="120" />
        <el-table-column prop="display_name" label="姓名" min-width="120" />
        <el-table-column prop="email" label="邮箱" min-width="180" />
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">
              {{ row.status === 'active' ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="is_super_admin" label="超级管理员" width="110">
          <template #default="{ row }">
            <el-tag v-if="row.is_super_admin" type="warning" size="small">是</el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="last_login_at" label="最后登录" min-width="170">
          <template #default="{ row }">{{ formatDate(row.last_login_at) }}</template>
        </el-table-column>
        <el-table-column v-if="canWrite" label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="!row.is_super_admin || authStore.isSuperAdmin"
              link
              type="primary"
              @click="openEdit(row)"
            >
              编辑
            </el-button>
            <el-button
              v-if="canReadRoles && (!row.is_super_admin || authStore.isSuperAdmin)"
              link
              type="primary"
              @click="openAssignRoles(row)"
            >
              分配角色
            </el-button>
            <el-button
              v-if="row.status === 'active' && row.username !== 'admin' && (!row.is_super_admin || authStore.isSuperAdmin)"
              link
              type="danger"
              @click="disableUser(row)"
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
        @current-change="loadUsers"
        @size-change="handleSizeChange"
      />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingUser ? '编辑用户' : '新建用户'" width="560px">
      <el-form :model="form" label-width="100px">
        <el-form-item label="用户名" required>
          <el-input v-model="form.username" :disabled="Boolean(editingUser)" placeholder="登录用户名" />
        </el-form-item>
        <el-form-item label="姓名" required>
          <el-input v-model="form.displayName" placeholder="显示名称" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="form.email" placeholder="name@example.com" />
        </el-form-item>
        <el-form-item v-if="canReadOrgs" label="组织">
          <el-tree-select
            v-model="form.orgId"
            :data="orgTree"
            :props="{ label: 'name', value: 'id', children: 'children' }"
            check-strictly
            clearable
            placeholder="选择主组织"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item v-if="!editingUser" label="密码" required>
          <el-input v-model="form.password" type="password" show-password placeholder="至少 8 位" />
        </el-form-item>
        <el-form-item v-else label="重置密码">
          <el-input v-model="form.password" type="password" show-password placeholder="留空则不修改" />
        </el-form-item>
        <el-form-item v-if="editingUser" label="状态">
          <el-select v-model="form.status" style="width: 100%">
            <el-option label="启用" value="active" />
            <el-option label="禁用" value="disabled" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="authStore.isSuperAdmin" label="超级管理员">
          <el-switch v-model="form.isSuperAdmin" />
        </el-form-item>
        <el-form-item v-if="!editingUser && canReadRoles" label="角色">
          <el-select v-model="form.roleIds" multiple style="width: 100%" placeholder="选择角色">
            <el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="assignVisible" title="分配角色" width="480px">
      <p class="assign-user">{{ assignUser?.display_name }}（{{ assignUser?.username }}）</p>
      <el-select v-model="assignRoleIds" multiple style="width: 100%" placeholder="选择角色">
        <el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" />
      </el-select>
      <template #footer>
        <el-button @click="assignVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveRoles">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { PageHeader } from '@mlf-erp/ui'
import {
  systemApi,
  type Organization,
  type SystemRole,
  type SystemUser
} from '@mlf-erp/sdk'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const canWrite = computed(() => authStore.hasPermission('system.user.write'))
const canReadRoles = computed(() => authStore.hasPermission('system.role.read'))
const canReadOrgs = computed(() => authStore.hasPermission('system.org.read'))

const users = ref<SystemUser[]>([])
const roles = ref<SystemRole[]>([])
const orgTree = ref<Organization[]>([])
const loading = ref(false)
const saving = ref(false)
const keyword = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const dialogVisible = ref(false)
const editingUser = ref<SystemUser | null>(null)
const form = reactive({
  username: '',
  displayName: '',
  email: '',
  password: '',
  orgId: '',
  status: 'active',
  isSuperAdmin: false,
  roleIds: [] as string[]
})

const assignVisible = ref(false)
const assignUser = ref<SystemUser | null>(null)
const assignRoleIds = ref<string[]>([])

async function loadUsers() {
  loading.value = true
  try {
    const result = await systemApi.users.list({
      keyword: keyword.value,
      page: page.value,
      page_size: pageSize.value
    })
    users.value = result.items
    total.value = result.total
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载用户失败')
  } finally {
    loading.value = false
  }
}

async function loadOptions() {
  const tasks: Promise<void>[] = []

  if (canReadRoles.value) {
    tasks.push(
      systemApi.roles
        .list()
        .then((roleList) => {
          roles.value = roleList
        })
        .catch((error) => {
          ElMessage.error(error instanceof Error ? error.message : '加载角色失败')
        })
    )
  }

  if (canReadOrgs.value) {
    tasks.push(
      systemApi.orgs
        .list()
        .then((orgList) => {
          orgTree.value = orgList
        })
        .catch((error) => {
          ElMessage.error(error instanceof Error ? error.message : '加载组织失败')
        })
    )
  }

  await Promise.all(tasks)
}

function search() {
  page.value = 1
  loadUsers()
}

function resetSearch() {
  keyword.value = ''
  page.value = 1
  loadUsers()
}

function handleSizeChange() {
  page.value = 1
  loadUsers()
}

function openCreate() {
  editingUser.value = null
  Object.assign(form, {
    username: '',
    displayName: '',
    email: '',
    password: '',
    orgId: authStore.user?.org_id ?? '',
    status: 'active',
    isSuperAdmin: false,
    roleIds: []
  })
  dialogVisible.value = true
}

function openEdit(user: SystemUser) {
  editingUser.value = user
  Object.assign(form, {
    username: user.username,
    displayName: user.display_name,
    email: user.email,
    password: '',
    orgId: user.org_id,
    status: user.status,
    isSuperAdmin: user.is_super_admin,
    roleIds: []
  })
  dialogVisible.value = true
}

async function submit() {
  if (!form.username.trim() || !form.displayName.trim()) {
    ElMessage.warning('用户名和姓名不能为空')
    return
  }
  if (!editingUser.value && form.password.length < 8) {
    ElMessage.warning('密码至少 8 位')
    return
  }

  saving.value = true
  try {
    if (editingUser.value) {
      await systemApi.users.update(editingUser.value.id, {
        display_name: form.displayName.trim(),
        email: form.email.trim(),
        org_id: form.orgId,
        status: form.status,
        is_super_admin: form.isSuperAdmin,
        password: form.password
      })
      ElMessage.success('用户已更新')
    } else {
      await systemApi.users.create({
        username: form.username.trim(),
        display_name: form.displayName.trim(),
        email: form.email.trim(),
        password: form.password,
        org_id: form.orgId,
        is_super_admin: form.isSuperAdmin,
        role_ids: form.roleIds
      })
      ElMessage.success('用户已创建')
    }
    dialogVisible.value = false
    await loadUsers()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存用户失败')
  } finally {
    saving.value = false
  }
}

async function disableUser(user: SystemUser) {
  try {
    await ElMessageBox.confirm(`确认禁用用户 ${user.username}？`, '禁用用户', { type: 'warning' })
  } catch {
    return
  }

  try {
    await systemApi.users.remove(user.id)
    ElMessage.success('用户已禁用')
    await loadUsers()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '禁用用户失败')
  }
}

async function openAssignRoles(user: SystemUser) {
  assignUser.value = user
  assignVisible.value = true
  try {
    const result = await systemApi.users.getRoles(user.id)
    assignRoleIds.value = result.role_ids
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载用户角色失败')
  }
}

async function saveRoles() {
  if (!assignUser.value) {
    return
  }
  saving.value = true
  try {
    await systemApi.users.setRoles(assignUser.value.id, assignRoleIds.value)
    ElMessage.success('角色已更新')
    assignVisible.value = false
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存角色失败')
  } finally {
    saving.value = false
  }
}

function formatDate(value?: string) {
  return value ? new Date(value).toLocaleString() : '-'
}

onMounted(async () => {
  await Promise.all([loadUsers(), loadOptions()])
})
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

.assign-user {
  margin: 0 0 12px;
  color: #475569;
}
</style>
