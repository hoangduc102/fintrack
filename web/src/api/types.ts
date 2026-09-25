export interface User {
  id: string
  email: string
  display_name: string | null
  created_at: string
  updated_at: string
}

export interface LoginResponse {
  token: string
}

export interface RegisterRequest {
  email: string
  password: string
  display_name?: string
}

export interface LoginRequest {
  email: string
  password: string
}

export interface ApiErrorResponse {
  error: {
    code: string
    message: string
    fields?: Record<string, string>
  }
}
