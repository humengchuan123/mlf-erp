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
  const accessToken = ref('')
  const refreshToken = ref('')
  let refreshPromise: Promise<string | null> | null = null
  let sessionEpoch = 0

  const hasToken = computed(() => Boolean(accessToken.value))
  const isAuthenticated = computed(() => Boolean(user.value))
  const isSuperAdmin = computed(() => user.value?.is_super_admin === true)

  function hasPermission(code: string): boolean {
    return isSuperAdmin.value || permissions.value.includes(code)
  }

  async function login(username: string, password: string, tenantId?: string): Promise<void> {
    const result = await authApi.login({
      username,
      password,
      ...(tenantId ? { tenant_id: tenantId } : {})
    })
    sessionEpoch += 1
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

  async function refreshSession(): Promise<string | null> {
    if (!refreshToken.value) {
      return null
    }

    if (!refreshPromise) {
      const currentRefreshToken = refreshToken.value
      const epoch = sessionEpoch
      refreshPromise = (async () => {
        try {
          const response = await fetch('/api/v1/auth/refresh', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ refresh_token: currentRefreshToken })
          })
          if (!response.ok) {
            if (sessionEpoch !== epoch) {
              return null
            }
            const storedRefreshToken = localStorage.getItem(REFRESH_TOKEN_KEY)
            if (storedRefreshToken && storedRefreshToken !== currentRefreshToken) {
              restoreFromStorage()
              return accessToken.value || null
            }
            clear()
            return null
          }
          if (sessionEpoch !== epoch) {
            return null
          }
          const body = (await response.json()) as { data: LoginResponse }
          applySession(body.data)
          return body.data.access_token
        } catch {
          if (sessionEpoch === epoch) {
            clear()
          }
          return null
        } finally {
          refreshPromise = null
        }
      })()
    }

    return refreshPromise
  }

  async function logout(): Promise<void> {
    const currentRefreshToken = refreshToken.value
    const currentAccessToken = accessToken.value

    if (currentRefreshToken) {
      try {
        await fetch('/api/v1/auth/logout', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(currentAccessToken ? { Authorization: `Bearer ${currentAccessToken}` } : {})
          },
          body: JSON.stringify({ refresh_token: currentRefreshToken })
        })
      } catch {
        // Local logout should still succeed when the server is unavailable.
      }
    }

    clear()
  }

  function restoreFromStorage(): void {
    sessionEpoch += 1
    refreshPromise = null
    const nextAccessToken = localStorage.getItem(ACCESS_TOKEN_KEY) ?? ''
    const nextRefreshToken = localStorage.getItem(REFRESH_TOKEN_KEY) ?? ''

    accessToken.value = nextAccessToken
    refreshToken.value = nextRefreshToken

    if (nextAccessToken && nextRefreshToken) {
      setAuthTokens(nextAccessToken, nextRefreshToken)
      return
    }

    clearAuthTokens()
    user.value = null
    roles.value = []
    permissions.value = []
  }

  function clear(): void {
    sessionEpoch += 1
    refreshPromise = null
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

  restoreFromStorage()

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
    refreshSession,
    logout,
    restoreFromStorage,
    clear
  }
})
