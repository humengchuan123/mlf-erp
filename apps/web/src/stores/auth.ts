import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  authApi,
  clearAuthTokens,
  setAuthTokens,
  type CurrentUser,
  type LoginResponse
} from '@mlf-erp/sdk'

const ACCESS_TOKEN_KEY = 'mlf_access_token'
const REFRESH_TOKEN_KEY = 'mlf_refresh_token'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<CurrentUser | null>(null)
  const roles = ref<string[]>([])
  const permissions = ref<string[]>([])
  const accessToken = ref(localStorage.getItem(ACCESS_TOKEN_KEY) ?? '')
  const refreshToken = ref(localStorage.getItem(REFRESH_TOKEN_KEY) ?? '')

  if (accessToken.value && refreshToken.value) {
    setAuthTokens(accessToken.value, refreshToken.value)
  }

  const hasToken = computed(() => Boolean(accessToken.value))
  const isAuthenticated = computed(() => Boolean(user.value))
  const isSuperAdmin = computed(() => user.value?.is_super_admin === true)

  function hasPermission(code: string): boolean {
    return isSuperAdmin.value || permissions.value.includes(code)
  }

  async function login(username: string, password: string): Promise<void> {
    const result = await authApi.login({ username, password })
    applySession(result)
  }

  async function loadMe(): Promise<void> {
    if (!accessToken.value) {
      return
    }
    const result = await authApi.me()
    user.value = result.user
    roles.value = result.roles
    permissions.value = result.permissions
  }

  async function logout(): Promise<void> {
    const currentRefreshToken = refreshToken.value
    if (currentRefreshToken) {
      try {
        await authApi.logout(currentRefreshToken)
      } catch {
        // Local logout should still succeed when the token is already invalid.
      }
    }
    clear()
  }

  function clear(): void {
    user.value = null
    roles.value = []
    permissions.value = []
    accessToken.value = ''
    refreshToken.value = ''
    clearAuthTokens()
    localStorage.removeItem(ACCESS_TOKEN_KEY)
    localStorage.removeItem(REFRESH_TOKEN_KEY)
  }

  function applySession(result: LoginResponse): void {
    accessToken.value = result.access_token
    refreshToken.value = result.refresh_token
    user.value = result.user
    roles.value = result.roles
    permissions.value = result.permissions
    setAuthTokens(result.access_token, result.refresh_token)
    localStorage.setItem(ACCESS_TOKEN_KEY, result.access_token)
    localStorage.setItem(REFRESH_TOKEN_KEY, result.refresh_token)
  }

  return {
    user,
    roles,
    permissions,
    accessToken,
    refreshToken,
    hasToken,
    isAuthenticated,
    isSuperAdmin,
    hasPermission,
    login,
    loadMe,
    logout,
    clear
  }
})
