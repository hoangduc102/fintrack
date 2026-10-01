import { apiClient } from './client'
import type { ImportResult } from './types'

export async function importTransactionsCSV(file: File): Promise<ImportResult> {
  const formData = new FormData()
  formData.append('file', file)

  return apiClient<ImportResult>('/api/v1/transactions/import', {
    method: 'POST',
    body: formData,
  })
}
