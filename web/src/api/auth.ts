import { apiClient } from './client'
import type { LoginRequest, LoginResponse, RegisterRequest, User } from './types'

export async function login(input: LoginRequest): Promise<LoginResponse> {
  return apiClient<LoginResponse>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export async function register(input: RegisterRequest): Promise<User> {
  return apiClient<User>('/api/v1/auth/register', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export async function getMe(): Promise<User> {
  return apiClient<User>('/api/v1/auth/me', {
    method: 'GET',
  })
}
