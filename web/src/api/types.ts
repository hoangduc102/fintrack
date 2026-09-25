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

// Categories
export type CategoryType = 'INCOME' | 'EXPENSE'

export interface Category {
  id: string
  user_id: string
  name: string
  type: CategoryType
  created_at: string
  updated_at: string
}

export interface CreateCategoryInput {
  name: string
  type: CategoryType
}

export interface UpdateCategoryInput {
  name: string
}

// Transactions
export type TransactionType = 'INCOME' | 'EXPENSE'

export interface Transaction {
  id: string
  user_id: string
  amount: number
  type: TransactionType
  category_id: string | null
  transaction_date: string // YYYY-MM-DD
  description: string | null
  created_at: string
  updated_at: string
}

export interface ListTransactionsFilter {
  page?: number
  limit?: number
  start_date?: string
  end_date?: string
  type?: TransactionType
  category_id?: string
  uncategorized?: boolean
}

export interface PaginationInfo {
  page: number
  limit: number
  total_items: number
  total_pages: number
}

export interface PaginatedTransactionsOutput {
  items: Transaction[]
  pagination: PaginationInfo
}

export interface CreateTransactionInput {
  amount: number
  type: TransactionType
  category_id?: string | null
  transaction_date: string // YYYY-MM-DD
  description?: string | null
}

export interface UpdateTransactionInput {
  amount: number
  type: TransactionType
  category_id?: string | null
  transaction_date: string // YYYY-MM-DD
  description?: string | null
}
