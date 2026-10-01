import { apiClient } from './client'
import type { Budget, UpsertBudgetInput } from './types'

export async function getBudgets(year: number, month: number): Promise<Budget[]> {
  const monthStr = `${year}-${String(month).padStart(2, '0')}`
  return apiClient<Budget[]>('/api/v1/budgets', {
    method: 'GET',
    params: { month: monthStr },
  })
}

export async function upsertBudget(input: UpsertBudgetInput): Promise<Budget> {
  return apiClient<Budget>('/api/v1/budgets', {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export async function deleteBudget(id: string): Promise<void> {
  return apiClient<void>(`/api/v1/budgets/${id}`, {
    method: 'DELETE',
  })
}
