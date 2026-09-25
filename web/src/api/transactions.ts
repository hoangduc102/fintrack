import { apiClient } from './client'
import type { 
  CreateTransactionInput, 
  ListTransactionsFilter, 
  PaginatedTransactionsOutput, 
  Transaction, 
  UpdateTransactionInput 
} from './types'

export async function getTransactions(filter: ListTransactionsFilter = {}): Promise<PaginatedTransactionsOutput> {
  const params: Record<string, string | number | boolean | undefined | null> = {
    page: filter.page,
    limit: filter.limit,
    start_date: filter.start_date,
    end_date: filter.end_date,
    type: filter.type,
    category_id: filter.category_id,
    uncategorized: filter.uncategorized ? true : undefined,
  }

  return apiClient<PaginatedTransactionsOutput>('/api/v1/transactions', {
    method: 'GET',
    params,
  })
}

export async function createTransaction(input: CreateTransactionInput): Promise<Transaction> {
  return apiClient<Transaction>('/api/v1/transactions', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export async function updateTransaction(id: string, input: UpdateTransactionInput): Promise<Transaction> {
  return apiClient<Transaction>(`/api/v1/transactions/${id}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export async function deleteTransaction(id: string): Promise<void> {
  return apiClient<void>(`/api/v1/transactions/${id}`, {
    method: 'DELETE',
  })
}
