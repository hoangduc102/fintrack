import { apiClient } from './client'
import type { Category, CategoryType, CreateCategoryInput, UpdateCategoryInput } from './types'

export async function getCategories(type?: CategoryType): Promise<Category[]> {
  return apiClient<Category[]>('/api/v1/categories', {
    method: 'GET',
    params: type ? { type } : undefined,
  })
}

export async function createCategory(input: CreateCategoryInput): Promise<Category> {
  return apiClient<Category>('/api/v1/categories', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export async function updateCategory(id: string, input: UpdateCategoryInput): Promise<Category> {
  return apiClient<Category>(`/api/v1/categories/${id}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export async function deleteCategory(id: string): Promise<void> {
  return apiClient<void>(`/api/v1/categories/${id}`, {
    method: 'DELETE',
  })
}
