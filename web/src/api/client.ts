import type { ApiErrorResponse } from './types'

export const TOKEN_STORAGE_KEY = 'fintrack_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_STORAGE_KEY)
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_STORAGE_KEY, token)
}

export function removeToken(): void {
  localStorage.removeItem(TOKEN_STORAGE_KEY)
}

export class ApiError extends Error {
  status: number
  code: string
  fields?: Record<string, string>

  constructor(status: number, message: string, code = 'ERROR', fields?: Record<string, string>) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.fields = fields
  }
}

interface RequestOptions extends RequestInit {
  params?: Record<string, string | number | boolean | undefined | null>
}

export async function apiClient<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
  const { params, headers: customHeaders, ...customConfig } = options

  let url = endpoint
  if (params) {
    const searchParams = new URLSearchParams()
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== null && value !== '') {
        searchParams.append(key, String(value))
      }
    }
    const queryString = searchParams.toString()
    if (queryString) {
      url += (url.includes('?') ? '&' : '?') + queryString
    }
  }

  const token = getToken()
  const headers = new Headers(customHeaders)

  if (!headers.has('Content-Type') && !(customConfig.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }

  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const response = await fetch(url, {
    ...customConfig,
    headers,
  })

  // Handle 204 No Content
  if (response.status === 204) {
    return {} as T
  }

  let data: any
  const contentType = response.headers.get('content-type')
  if (contentType && contentType.includes('application/json')) {
    data = await response.json()
  } else {
    data = await response.text()
  }

  if (!response.ok) {
    const errorData = data as ApiErrorResponse
    const code = errorData?.error?.code || 'HTTP_ERROR'
    const message = errorData?.error?.message || response.statusText || 'An unexpected error occurred'
    const fields = errorData?.error?.fields

    // If 401 Unauthorized, notify or clear stale token if appropriate
    if (response.status === 401 && endpoint !== '/api/v1/auth/login') {
      window.dispatchEvent(new CustomEvent('fintrack:unauthorized'))
    }

    throw new ApiError(response.status, message, code, fields)
  }

  return data as T
}
