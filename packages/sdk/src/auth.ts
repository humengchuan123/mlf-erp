import { api } from './client'

export interface CurrentUser {
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

export interface LoginRequest {
  username: string
  password: string
  tenant_id?: string
}

export interface LoginResponse {
  access_token: string
  refresh_token: string
  expires_in: number
  user: CurrentUser
  roles: string[]
  permissions: string[]
}

export interface MeResponse {
  user: CurrentUser
  roles: string[]
  permissions: string[]
}

export const authApi = {
  login(input: LoginRequest): Promise<LoginResponse> {
    return api.post<LoginResponse>('/auth/login', input)
  },
  refresh(refreshToken: string): Promise<LoginResponse> {
    return api.post<LoginResponse>('/auth/refresh', { refresh_token: refreshToken })
  },
  me(): Promise<MeResponse> {
    return api.get<MeResponse>('/auth/me')
  },
  logout(refreshToken: string): Promise<{ logged_out: boolean }> {
    return api.post<{ logged_out: boolean }>('/auth/logout', { refresh_token: refreshToken })
  },
  changePassword(input: { old_password: string; new_password: string }): Promise<{ changed: boolean }> {
    return api.post<{ changed: boolean }>('/auth/change-password', input)
  }
}
