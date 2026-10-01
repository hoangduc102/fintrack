import { apiClient } from './client'
import type { CategoryExpenseShare, MonthlySummary, MonthlyTrendPoint } from './types'

export async function getMonthlySummary(month: string): Promise<MonthlySummary> {
  return apiClient<MonthlySummary>('/api/v1/analytics/summary', {
    method: 'GET',
    params: { month },
  })
}

export async function getCategoryExpenses(month: string): Promise<CategoryExpenseShare[]> {
  return apiClient<CategoryExpenseShare[]>('/api/v1/analytics/categories', {
    method: 'GET',
    params: { month },
  })
}

export async function getMonthlyTrend(months = 6): Promise<MonthlyTrendPoint[]> {
  return apiClient<MonthlyTrendPoint[]>('/api/v1/analytics/trend', {
    method: 'GET',
    params: { months },
  })
}
