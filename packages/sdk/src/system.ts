import { api } from './client'

export interface Paged<T> {
  items: T[]
  total: number
}

export interface SystemUser {
  id: string
  tenant_id: string
  org_id: string
  username: string
  display_name: string
  email: string
  status: string
  is_super_admin: boolean
  last_login_at?: string
  created_at: string
  updated_at: string
}

export interface SystemRole {
  id: string
  tenant_id: string
  code: string
  name: string
  is_system: boolean
  permissions: string[]
  created_at: string
}

export interface Permission {
  code: string
  name: string
  module: string
  type: string
  description: string
}

export interface Organization {
  id: string
  tenant_id: string
  parent_id: string
  type: string
  code: string
  name: string
  path: string
  status: string
  created_at: string
  updated_at: string
  children?: Organization[]
}

export interface AuditLog {
  id: string
  tenant_id: string
  org_id: string
  user_id: string
  action: string
  resource_type: string
  resource_id: string
  request_id: string
  ip: string
  user_agent: string
  before_data: unknown
  after_data: unknown
  created_at: string
}

export interface CreateUserInput {
  username: string
  display_name: string
  email?: string
  password: string
  org_id?: string
  is_super_admin?: boolean
  role_ids?: string[]
}

export interface UpdateUserInput {
  display_name?: string
  email?: string
  org_id?: string
  status?: string
  is_super_admin?: boolean
  password?: string
}

export interface CreateRoleInput {
  code: string
  name: string
}

export interface UpdateRoleInput {
  code: string
  name: string
}

export interface CreateOrgInput {
  parent_id?: string
  type?: string
  code: string
  name: string
}

export interface UpdateOrgInput {
  parent_id?: string
  type?: string
  code: string
  name: string
  status?: string
}

function queryString(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') {
      search.set(key, String(value))
    }
  }
  const query = search.toString()
  return query ? `?${query}` : ''
}

export const systemApi = {
  users: {
    list(params: { keyword?: string; page?: number; page_size?: number } = {}): Promise<Paged<SystemUser>> {
      return api.get<Paged<SystemUser>>(`/system/users${queryString(params)}`)
    },
    create(input: CreateUserInput): Promise<SystemUser> {
      return api.post<SystemUser>('/system/users', input)
    },
    update(id: string, input: UpdateUserInput): Promise<SystemUser> {
      return api.put<SystemUser>(`/system/users/${id}`, input)
    },
    remove(id: string): Promise<{ id: string; status: string }> {
      return api.delete<{ id: string; status: string }>(`/system/users/${id}`)
    },
    getRoles(id: string): Promise<{ user_id: string; role_ids: string[] }> {
      return api.get<{ user_id: string; role_ids: string[] }>(`/system/users/${id}/roles`)
    },
    setRoles(id: string, roleIds: string[]): Promise<{ user_id: string; role_ids: string[] }> {
      return api.post<{ user_id: string; role_ids: string[] }>(`/system/users/${id}/roles`, {
        role_ids: roleIds
      })
    }
  },
  roles: {
    list(): Promise<SystemRole[]> {
      return api.get<SystemRole[]>('/system/roles')
    },
    create(input: CreateRoleInput): Promise<SystemRole> {
      return api.post<SystemRole>('/system/roles', input)
    },
    update(id: string, input: UpdateRoleInput): Promise<SystemRole> {
      return api.put<SystemRole>(`/system/roles/${id}`, input)
    },
    remove(id: string): Promise<{ id: string; deleted: boolean }> {
      return api.delete<{ id: string; deleted: boolean }>(`/system/roles/${id}`)
    },
    setPermissions(id: string, permissionCodes: string[]): Promise<{ role_id: string; permission_codes: string[] }> {
      return api.put<{ role_id: string; permission_codes: string[] }>(`/system/roles/${id}/permissions`, {
        permission_codes: permissionCodes
      })
    }
  },
  permissions: {
    list(): Promise<Permission[]> {
      return api.get<Permission[]>('/system/permissions')
    }
  },
  orgs: {
    list(): Promise<Organization[]> {
      return api.get<Organization[]>('/system/orgs')
    },
    create(input: CreateOrgInput): Promise<Organization> {
      return api.post<Organization>('/system/orgs', input)
    },
    update(id: string, input: UpdateOrgInput): Promise<Organization> {
      return api.put<Organization>(`/system/orgs/${id}`, input)
    },
    remove(id: string): Promise<{ id: string; deleted: boolean }> {
      return api.delete<{ id: string; deleted: boolean }>(`/system/orgs/${id}`)
    }
  },
  auditLogs: {
    list(params: { page?: number; page_size?: number } = {}): Promise<Paged<AuditLog>> {
      return api.get<Paged<AuditLog>>(`/system/audit-logs${queryString(params)}`)
    }
  }
}
