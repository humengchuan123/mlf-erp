export interface ApiResponse<T> {
  code: string
  message: string
  data: T
  request_id?: string
}

export class ApiError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly status: number
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

export interface RequestOptions {
  idempotencyKey?: string
}

let accessToken: string | null = null
let refreshToken: string | null = null
let unauthorizedHandler: (() => void) | null = null
let refreshHandler: (() => Promise<string | null>) | null = null

export function setAuthTokens(access: string | null, refresh: string | null): void {
  accessToken = access || null
  refreshToken = refresh || null
}

export function getAccessToken(): string | null {
  return accessToken
}

export function getRefreshToken(): string | null {
  return refreshToken
}

export function clearAuthTokens(): void {
  accessToken = null
  refreshToken = null
}

export function setUnauthorizedHandler(handler: (() => void) | null): void {
  unauthorizedHandler = handler
}

export function setRefreshHandler(handler: (() => Promise<string | null>) | null): void {
  refreshHandler = handler
}

export class ApiClient {
  constructor(private readonly baseUrl = '/api/v1') {}

  async request<T>(path: string, init: RequestInit = {}, retried = false): Promise<T> {
    const headers = new Headers(init.headers)
    if (!headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json')
    }
    if (accessToken) {
      headers.set('Authorization', `Bearer ${accessToken}`)
    }

    const response = await fetch(`${this.baseUrl}${path}`, {
      ...init,
      headers
    })

    const isPublicAuthPath = path.startsWith('/auth/login') || path.startsWith('/auth/refresh')

    if (response.status === 401 && !retried && !isPublicAuthPath && refreshHandler) {
      const newAccessToken = await refreshHandler()
      if (newAccessToken) {
        return this.request<T>(path, init, true)
      }
    }

    if (response.status === 401 && !isPublicAuthPath && !retried) {
      unauthorizedHandler?.()
    }

    const body = (await response.json()) as ApiResponse<T>
    if (!response.ok) {
      throw new ApiError(body.code ?? 'HTTP_ERROR', body.message ?? response.statusText, response.status)
    }
    return body.data
  }

  get<T>(path: string): Promise<T> {
    return this.request<T>(path)
  }

  post<T>(path: string, data?: unknown, options?: RequestOptions): Promise<T> {
    const headers: Record<string, string> = {}
    if (options?.idempotencyKey) {
      headers['Idempotency-Key'] = options.idempotencyKey
    }
    return this.request<T>(path, {
      method: 'POST',
      headers,
      body: data === undefined ? undefined : JSON.stringify(data)
    })
  }

  put<T>(path: string, data?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'PUT',
      body: data === undefined ? undefined : JSON.stringify(data)
    })
  }

  delete<T>(path: string): Promise<T> {
    return this.request<T>(path, { method: 'DELETE' })
  }
}

export const api = new ApiClient()
