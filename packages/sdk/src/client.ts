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

export class ApiClient {
  constructor(private readonly baseUrl = '/api/v1') {}

  async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const headers = new Headers(init.headers)
    if (!headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json')
    }

    const response = await fetch(`${this.baseUrl}${path}`, {
      ...init,
      headers
    })

    const body = (await response.json()) as ApiResponse<T>
    if (!response.ok) {
      throw new ApiError(body.code ?? 'HTTP_ERROR', body.message ?? response.statusText, response.status)
    }
    return body.data
  }

  get<T>(path: string): Promise<T> {
    return this.request<T>(path)
  }

  post<T>(path: string, data?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'POST',
      body: data === undefined ? undefined : JSON.stringify(data)
    })
  }
}

export const api = new ApiClient()
