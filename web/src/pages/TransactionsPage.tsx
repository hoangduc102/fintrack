import React, { useState, useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getTransactions, createTransaction, updateTransaction, deleteTransaction } from '../api/transactions'
import { getCategories } from '../api/categories'
import { ApiError } from '../api/client'
import type { 
  Transaction, 
  TransactionType, 
  CreateTransactionInput, 
  UpdateTransactionInput 
} from '../api/types'
import { formatVND, formatDate, getTodayString } from '../utils/format'
import { 
  ReceiptText, 
  Plus, 
  ChevronLeft, 
  ChevronRight, 
  Pencil, 
  Trash2, 
  ArrowDownLeft, 
  ArrowUpRight, 
  X, 
  AlertCircle, 
  Loader2, 
  CheckCircle2, 
  RotateCcw
} from 'lucide-react'

export const TransactionsPage: React.FC = () => {
  const queryClient = useQueryClient()

  // Filter States
  const [page, setPage] = useState(1)
  const [limit, setLimit] = useState(20)
  const [typeFilter, setTypeFilter] = useState<TransactionType | ''>('')
  const [categoryFilter, setCategoryFilter] = useState<string>('')
  const [startDate, setStartDate] = useState<string>('')
  const [endDate, setEndDate] = useState<string>('')
  const [uncategorized, setUncategorized] = useState<boolean>(false)

  // Modals States
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [editingTransaction, setEditingTransaction] = useState<Transaction | null>(null)
  const [deletingTransaction, setDeletingTransaction] = useState<Transaction | null>(null)

  // Form Fields State
  const [amount, setAmount] = useState<number | ''>('')
  const [txType, setTxType] = useState<TransactionType>('EXPENSE')
  const [categoryId, setCategoryId] = useState<string>('')
  const [transactionDate, setTransactionDate] = useState<string>(getTodayString())
  const [description, setDescription] = useState<string>('')
  const [formError, setFormError] = useState<string | null>(null)
  const [feedback, setFeedback] = useState<{ message: string; type: 'success' | 'error' } | null>(null)

  // Date range validation: end date cannot precede start date
  const isDateRangeInvalid = Boolean(startDate && endDate && endDate < startDate)

  // Fetch Categories for Dropdowns
  const { data: categories = [] } = useQuery({
    queryKey: ['categories'],
    queryFn: () => getCategories(),
  })

  // Category Lookup Map
  const categoryMap = useMemo(() => {
    const map = new Map<string, string>()
    for (const c of categories) {
      map.set(c.id, c.name)
    }
    return map
  }, [categories])

  // Filter query parameters
  const queryFilter = useMemo(() => {
    return {
      page,
      limit,
      type: typeFilter ? typeFilter : undefined,
      category_id: !uncategorized && categoryFilter ? categoryFilter : undefined,
      start_date: startDate ? startDate : undefined,
      end_date: endDate ? endDate : undefined,
      uncategorized: uncategorized ? true : undefined,
    }
  }, [page, limit, typeFilter, categoryFilter, startDate, endDate, uncategorized])

  // Fetch Transactions Query - prevent execution if date range is invalid
  const { 
    data: transactionsData, 
    isLoading, 
    isFetching, 
    error: queryError 
  } = useQuery({
    queryKey: ['transactions', queryFilter],
    queryFn: () => getTransactions(queryFilter),
    enabled: !isDateRangeInvalid,
  })

  // Centralized TanStack Query cache invalidator for all affected views
  const invalidateFinancialQueries = () => {
    queryClient.invalidateQueries({ queryKey: ['transactions'] })
    queryClient.invalidateQueries({ queryKey: ['analytics-summary'] })
    queryClient.invalidateQueries({ queryKey: ['analytics-categories'] })
    queryClient.invalidateQueries({ queryKey: ['analytics-trend'] })
    queryClient.invalidateQueries({ queryKey: ['budgets'] })
  }

  // Create Mutation
  const createMutation = useMutation({
    mutationFn: (input: CreateTransactionInput) => createTransaction(input),
    onSuccess: () => {
      invalidateFinancialQueries()
      setIsCreateOpen(false)
      resetForm()
      showFeedback('Transaction recorded successfully.')
    },
    onError: (err: unknown) => {
      if (err instanceof ApiError) {
        setFormError(err.message || 'Failed to create transaction')
      } else {
        setFormError('Network error while saving transaction')
      }
    },
  })

  // Update Mutation
  const updateMutation = useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateTransactionInput }) => updateTransaction(id, input),
    onSuccess: () => {
      invalidateFinancialQueries()
      setEditingTransaction(null)
      resetForm()
      showFeedback('Transaction updated successfully.')
    },
    onError: (err: unknown) => {
      if (err instanceof ApiError) {
        setFormError(err.message || 'Failed to update transaction')
      } else {
        setFormError('Network error while updating transaction')
      }
    },
  })

  // Delete Mutation
  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteTransaction(id),
    onSuccess: () => {
      invalidateFinancialQueries()
      setDeletingTransaction(null)
      showFeedback('Transaction deleted successfully.')
    },
    onError: (err: unknown) => {
      if (err instanceof ApiError) {
        setFormError(err.message || 'Failed to delete transaction')
      } else {
        setFormError('Network error while deleting transaction')
      }
    },
  })

  const showFeedback = (message: string, type: 'success' | 'error' = 'success') => {
    setFeedback({ message, type })
    setTimeout(() => setFeedback(null), 4000)
  }

  const resetForm = () => {
    setAmount('')
    setTxType('EXPENSE')
    setCategoryId('')
    setTransactionDate(getTodayString())
    setDescription('')
    setFormError(null)
  }

  const handleOpenCreate = () => {
    resetForm()
    setIsCreateOpen(true)
  }

  const handleOpenEdit = (tx: Transaction) => {
    setEditingTransaction(tx)
    setAmount(tx.amount)
    setTxType(tx.type)
    setCategoryId(tx.category_id || '')
    setTransactionDate(tx.transaction_date)
    setDescription(tx.description || '')
    setFormError(null)
  }

  const handleOpenDelete = (tx: Transaction) => {
    setDeletingTransaction(tx)
    setFormError(null)
  }

  const handleSaveCreate = (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)

    if (amount === '' || amount <= 0) {
      setFormError('Amount must be greater than 0 VND')
      return
    }
    if (!transactionDate) {
      setFormError('Transaction date is required')
      return
    }

    createMutation.mutate({
      amount: Number(amount),
      type: txType,
      category_id: categoryId || null,
      transaction_date: transactionDate,
      description: description.trim() || null,
    })
  }

  const handleSaveEdit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingTransaction) return
    setFormError(null)

    if (amount === '' || amount <= 0) {
      setFormError('Amount must be greater than 0 VND')
      return
    }
    if (!transactionDate) {
      setFormError('Transaction date is required')
      return
    }

    updateMutation.mutate({
      id: editingTransaction.id,
      input: {
        amount: Number(amount),
        type: txType,
        category_id: categoryId || null,
        transaction_date: transactionDate,
        description: description.trim() || null,
      },
    })
  }

  const handleResetFilters = () => {
    setTypeFilter('')
    setCategoryFilter('')
    setStartDate('')
    setEndDate('')
    setUncategorized(false)
    setPage(1)
  }

  const hasActiveFilters = Boolean(typeFilter || categoryFilter || startDate || endDate || uncategorized)

  // Filter category options in modals according to active modal txType
  const modalCategoryOptions = useMemo(() => {
    return categories.filter((c) => c.type === txType)
  }, [categories, txType])

  const pagination = transactionsData?.pagination
  const items = transactionsData?.items || []

  return (
    <div className="space-y-6 animate-in fade-in duration-300">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-[#243044] tracking-tight flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-[#5BC8A4]/15 border border-[#5BC8A4]/30 flex items-center justify-center text-[#2F8F78]">
              <ReceiptText className="w-5 h-5" />
            </div>
            Transactions
          </h1>
          <p className="mt-1 text-sm text-[#718096]">
            View, filter, and record all financial transactions with exact VND integer precision
          </p>
        </div>

        <button
          type="button"
          onClick={handleOpenCreate}
          className="inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl text-sm font-semibold text-white bg-[#5BC8A4] hover:bg-[#4db995] shadow-xs transition-all cursor-pointer"
        >
          <Plus className="w-4 h-4" />
          Record Transaction
        </button>
      </div>

      {/* Floating feedback alert */}
      {feedback && (
        <div className="rounded-xl bg-emerald-50 border border-emerald-200 p-4 flex items-center gap-3 text-[#2F8F78] animate-in fade-in">
          <CheckCircle2 className="w-5 h-5 text-[#5BC8A4] shrink-0" />
          <span className="text-sm font-medium">{feedback.message}</span>
        </div>
      )}

      {/* Filter Toolbar */}
      <div className="rounded-2xl bg-white border border-[#E5EAF1] p-5 space-y-4 shadow-xs">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          {/* Type Toggle Tabs */}
          <div className="flex items-center p-1 bg-[#F7F9FC] rounded-xl border border-[#E5EAF1] w-fit">
            <button
              type="button"
              onClick={() => { setTypeFilter(''); setPage(1); }}
              className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer ${
                typeFilter === '' ? 'bg-white text-[#243044] shadow-xs font-bold' : 'text-[#718096] hover:text-[#243044]'
              }`}
            >
              All Types
            </button>
            <button
              type="button"
              onClick={() => { setTypeFilter('EXPENSE'); setPage(1); }}
              className={`px-3 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-1.5 transition-all cursor-pointer ${
                typeFilter === 'EXPENSE' ? 'bg-rose-50 text-[#C53030] border border-rose-200' : 'text-[#718096] hover:text-[#243044]'
              }`}
            >
              <ArrowDownLeft className="w-3.5 h-3.5 text-[#E98C8C]" />
              Expenses
            </button>
            <button
              type="button"
              onClick={() => { setTypeFilter('INCOME'); setPage(1); }}
              className={`px-3 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-1.5 transition-all cursor-pointer ${
                typeFilter === 'INCOME' ? 'bg-emerald-50 text-[#2F8F78] border border-emerald-200' : 'text-[#718096] hover:text-[#243044]'
              }`}
            >
              <ArrowUpRight className="w-3.5 h-3.5 text-[#5BC8A4]" />
              Income
            </button>
          </div>

          {/* Reset Filters CTA */}
          {hasActiveFilters && (
            <button
              type="button"
              onClick={handleResetFilters}
              className="inline-flex items-center gap-1.5 text-xs font-semibold text-[#718096] hover:text-rose-600 transition-colors cursor-pointer"
            >
              <RotateCcw className="w-3.5 h-3.5" />
              Reset all filters
            </button>
          )}
        </div>

        {/* Granular Filters Grid */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 pt-2 border-t border-[#E5EAF1]">
          {/* Start Date */}
          <div>
            <label className="block text-[11px] font-semibold text-[#718096] uppercase tracking-wider mb-1">
              Start Date
            </label>
            <div className="relative">
              <input
                type="date"
                value={startDate}
                onChange={(e) => { setStartDate(e.target.value); setPage(1); }}
                className="w-full px-3 py-2 rounded-xl bg-white border border-[#E5EAF1] text-[#243044] text-xs focus:outline-none focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4] transition"
              />
            </div>
          </div>

          {/* End Date */}
          <div>
            <label className="block text-[11px] font-semibold text-[#718096] uppercase tracking-wider mb-1">
              End Date
            </label>
            <div className="relative">
              <input
                type="date"
                value={endDate}
                min={startDate || undefined}
                onChange={(e) => { setEndDate(e.target.value); setPage(1); }}
                className={`w-full px-3 py-2 rounded-xl bg-white border text-xs focus:outline-none transition ${
                  isDateRangeInvalid
                    ? 'border-rose-400 focus:ring-2 focus:ring-rose-100 text-[#C53030]'
                    : 'border-[#E5EAF1] text-[#243044] focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4]'
                }`}
              />
            </div>
            {/* Inline date range validation error message */}
            {isDateRangeInvalid && (
              <div className="flex items-center gap-1.5 text-xs text-[#C53030] font-medium mt-1.5">
                <AlertCircle className="w-3.5 h-3.5 shrink-0" />
                <span>End date must be on or after start date.</span>
              </div>
            )}
          </div>

          {/* Category Dropdown */}
          <div>
            <label className="block text-[11px] font-semibold text-[#718096] uppercase tracking-wider mb-1">
              Category
            </label>
            <select
              disabled={uncategorized}
              value={categoryFilter}
              onChange={(e) => { setCategoryFilter(e.target.value); setPage(1); }}
              className={`w-full px-3 py-2 rounded-xl bg-white border border-[#E5EAF1] text-[#243044] text-xs focus:outline-none focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4] transition ${
                uncategorized ? 'opacity-40 cursor-not-allowed bg-slate-50' : ''
              }`}
            >
              <option value="">All Categories</option>
              {categories.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name} ({c.type})
                </option>
              ))}
            </select>
          </div>

          {/* Uncategorized Checkbox */}
          <div className="flex items-end pb-2">
            <label className="flex items-center gap-2.5 cursor-pointer text-xs font-medium text-[#718096] hover:text-[#243044] select-none">
              <input
                type="checkbox"
                checked={uncategorized}
                onChange={(e) => {
                  setUncategorized(e.target.checked)
                  if (e.target.checked) {
                    setCategoryFilter('')
                  }
                  setPage(1)
                }}
                className="w-4 h-4 rounded border-[#E5EAF1] text-[#5BC8A4] focus:ring-[#5BC8A4]/40"
              />
              <span>Uncategorized only</span>
            </label>
          </div>
        </div>
      </div>

      {/* Transactions Table Card */}
      <div className="rounded-2xl bg-white border border-[#E5EAF1] overflow-hidden shadow-xs">
        {isDateRangeInvalid ? (
          <div className="py-16 text-center text-[#718096]">
            <div className="w-12 h-12 rounded-2xl bg-amber-50 border border-amber-200 flex items-center justify-center mx-auto mb-3 text-[#D97706]">
              <AlertCircle className="w-6 h-6" />
            </div>
            <h3 className="text-base font-semibold text-[#243044]">Invalid Date Filter Range</h3>
            <p className="text-xs text-[#718096] mt-1 max-w-sm mx-auto">
              End date must be on or after start date. Please adjust your date range above.
            </p>
          </div>
        ) : queryError ? (
          <div className="p-8 text-center text-rose-500">
            <AlertCircle className="w-8 h-8 mx-auto mb-2 text-[#E98C8C]" />
            <p className="text-sm font-medium">Failed to load transactions.</p>
          </div>
        ) : isLoading ? (
          <div className="py-20 flex flex-col items-center justify-center text-[#718096]">
            <Loader2 className="w-8 h-8 text-[#5BC8A4] animate-spin mb-3" />
            <p className="text-sm">Fetching transactions...</p>
          </div>
        ) : items.length === 0 ? (
          <div className="py-20 text-center text-[#718096]">
            <div className="w-12 h-12 rounded-2xl bg-[#F7F9FC] border border-[#E5EAF1] flex items-center justify-center mx-auto mb-3 text-[#718096]">
              <ReceiptText className="w-6 h-6" />
            </div>
            <h3 className="text-base font-semibold text-[#243044]">No transactions found</h3>
            <p className="text-xs text-[#718096] mt-1 max-w-sm mx-auto">
              {hasActiveFilters
                ? 'Try adjusting or resetting your filter criteria above.'
                : 'Click "Record Transaction" above to log your first income or expense.'}
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="bg-[#F7F9FC] text-[11px] font-semibold text-[#718096] uppercase tracking-wider border-b border-[#E5EAF1]">
                <tr>
                  <th className="py-3.5 px-4 sm:px-6">Date</th>
                  <th className="py-3.5 px-4">Description</th>
                  <th className="py-3.5 px-4">Category</th>
                  <th className="py-3.5 px-4">Type</th>
                  <th className="py-3.5 px-4 text-right">Amount (VND)</th>
                  <th className="py-3.5 px-4 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[#E5EAF1]">
                {items.map((tx) => {
                  const isIncome = tx.type === 'INCOME'
                  const catName = tx.category_id ? categoryMap.get(tx.category_id) || 'Unknown' : 'Uncategorized'

                  return (
                    <tr key={tx.id} className="hover:bg-[#F7F9FC]/70 transition-colors group">
                      <td className="py-3.5 px-4 sm:px-6 whitespace-nowrap text-xs font-medium text-[#718096]">
                        {formatDate(tx.transaction_date)}
                      </td>
                      <td className="py-3.5 px-4 max-w-xs truncate text-xs text-[#243044]">
                        {tx.description ? tx.description : <span className="text-slate-400 italic">No description</span>}
                      </td>
                      <td className="py-3.5 px-4 whitespace-nowrap">
                        <span
                          className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-[11px] font-medium ${
                            tx.category_id
                              ? 'bg-[#F7F9FC] text-[#243044] border border-[#E5EAF1]'
                              : 'bg-slate-100 text-[#718096] border border-slate-200 italic'
                          }`}
                        >
                          {catName}
                        </span>
                      </td>
                      <td className="py-3.5 px-4 whitespace-nowrap">
                        <span
                          className={`inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[11px] font-semibold ${
                            isIncome
                              ? 'bg-emerald-50 text-[#2F8F78] border border-emerald-200'
                              : 'bg-rose-50 text-[#C53030] border border-rose-200'
                          }`}
                        >
                          {isIncome ? <ArrowUpRight className="w-3 h-3" /> : <ArrowDownLeft className="w-3 h-3" />}
                          {tx.type}
                        </span>
                      </td>
                      <td className="py-3.5 px-4 text-right whitespace-nowrap font-mono font-bold text-sm">
                        <span className={isIncome ? 'text-[#2F8F78]' : 'text-[#243044]'}>
                          {isIncome ? '+' : '-'} {formatVND(tx.amount)}
                        </span>
                      </td>
                      <td className="py-3.5 px-4 text-right whitespace-nowrap">
                        <div className="flex items-center justify-end gap-1 opacity-80 group-hover:opacity-100 transition-opacity">
                          <button
                            type="button"
                            onClick={() => handleOpenEdit(tx)}
                            className="p-1.5 rounded-lg text-[#718096] hover:text-[#243044] hover:bg-slate-100 transition-colors cursor-pointer"
                            title="Edit transaction"
                          >
                            <Pencil className="w-3.5 h-3.5" />
                          </button>
                          <button
                            type="button"
                            onClick={() => handleOpenDelete(tx)}
                            className="p-1.5 rounded-lg text-[#718096] hover:text-rose-600 hover:bg-rose-50 transition-colors cursor-pointer"
                            title="Delete transaction"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination Toolbar */}
        {pagination && pagination.total_items > 0 && !isDateRangeInvalid && (
          <div className="p-4 bg-[#F7F9FC] border-t border-[#E5EAF1] flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-[#718096]">
            <div className="flex items-center gap-4">
              <span>
                Showing <strong className="text-[#243044]">{(pagination.page - 1) * pagination.limit + 1}</strong> to{' '}
                <strong className="text-[#243044]">
                  {Math.min(pagination.page * pagination.limit, pagination.total_items)}
                </strong>{' '}
                of <strong className="text-[#243044]">{pagination.total_items}</strong> records
              </span>

              {/* Limit Selector */}
              <div className="flex items-center gap-1.5">
                <span>Per page:</span>
                <select
                  value={limit}
                  onChange={(e) => { setLimit(Number(e.target.value)); setPage(1); }}
                  className="px-2 py-1 rounded-lg bg-white border border-[#E5EAF1] text-[#243044] text-xs focus:outline-none"
                >
                  <option value={10}>10</option>
                  <option value={20}>20</option>
                  <option value={50}>50</option>
                  <option value={100}>100</option>
                </select>
              </div>
            </div>

            {/* Navigation Buttons */}
            <div className="flex items-center gap-2">
              <button
                type="button"
                disabled={page <= 1 || isFetching}
                onClick={() => setPage((p) => Math.max(p - 1, 1))}
                className="px-3 py-1.5 rounded-xl border border-[#E5EAF1] bg-white text-[#243044] hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed flex items-center gap-1 transition-colors cursor-pointer shadow-xs"
              >
                <ChevronLeft className="w-3.5 h-3.5" />
                Previous
              </button>

              <span className="px-3 py-1 font-medium text-[#243044]">
                Page {pagination.page} of {pagination.total_pages || 1}
              </span>

              <button
                type="button"
                disabled={page >= pagination.total_pages || isFetching}
                onClick={() => setPage((p) => p + 1)}
                className="px-3 py-1.5 rounded-xl border border-[#E5EAF1] bg-white text-[#243044] hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed flex items-center gap-1 transition-colors cursor-pointer shadow-xs"
              >
                Next
                <ChevronRight className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        )}
      </div>

      {/* CREATE MODAL */}
      {isCreateOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/40 backdrop-blur-xs animate-in fade-in">
          <div className="bg-white border border-[#E5EAF1] rounded-2xl w-full max-w-lg p-6 shadow-xl relative text-[#243044]">
            <div className="flex items-center justify-between pb-4 border-b border-[#E5EAF1]">
              <h3 className="text-lg font-bold text-[#243044]">Record Transaction</h3>
              <button
                type="button"
                onClick={() => setIsCreateOpen(false)}
                className="text-[#718096] hover:text-[#243044] p-1 rounded-lg hover:bg-slate-100 cursor-pointer"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {formError && (
              <div className="mt-4 p-3 rounded-xl bg-rose-50 border border-rose-200 text-xs font-medium text-[#C53030] flex items-center gap-2">
                <AlertCircle className="w-4 h-4 shrink-0 text-[#E98C8C]" />
                <span>{formError}</span>
              </div>
            )}

            <form onSubmit={handleSaveCreate} className="mt-4 space-y-4">
              {/* Type Switcher */}
              <div>
                <label className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Transaction Type
                </label>
                <div className="grid grid-cols-2 gap-3">
                  <button
                    type="button"
                    onClick={() => { setTxType('EXPENSE'); setCategoryId(''); }}
                    className={`py-2 px-3 rounded-xl text-xs font-semibold border flex items-center justify-center gap-2 transition-all cursor-pointer ${
                      txType === 'EXPENSE'
                        ? 'bg-rose-50 border-rose-200 text-[#C53030] shadow-xs'
                        : 'bg-white border-[#E5EAF1] text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC]'
                    }`}
                  >
                    <ArrowDownLeft className="w-4 h-4" />
                    Expense
                  </button>
                  <button
                    type="button"
                    onClick={() => { setTxType('INCOME'); setCategoryId(''); }}
                    className={`py-2 px-3 rounded-xl text-xs font-semibold border flex items-center justify-center gap-2 transition-all cursor-pointer ${
                      txType === 'INCOME'
                        ? 'bg-emerald-50 border-emerald-200 text-[#2F8F78] shadow-xs'
                        : 'bg-white border-[#E5EAF1] text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC]'
                    }`}
                  >
                    <ArrowUpRight className="w-4 h-4" />
                    Income
                  </button>
                </div>
              </div>

              {/* Amount (VND) */}
              <div>
                <label className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Amount (VND) <span className="text-[#5BC8A4]">*</span>
                </label>
                <div className="relative">
                  <input
                    type="number"
                    min="1"
                    step="1"
                    required
                    value={amount}
                    onChange={(e) => setAmount(e.target.value === '' ? '' : parseInt(e.target.value, 10))}
                    placeholder="e.g. 50000"
                    className="w-full pl-3.5 pr-14 py-2.5 rounded-xl bg-white border border-[#E5EAF1] text-[#243044] text-sm focus:outline-none focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4] font-mono [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                  />
                  <div className="absolute inset-y-0 right-0 pr-3.5 flex items-center pointer-events-none text-xs font-semibold text-[#718096] select-none">
                    VND
                  </div>
                </div>
                {typeof amount === 'number' && amount > 0 && (
                  <p className="mt-1 text-xs text-[#2F8F78] font-medium">
                    Formatted: {formatVND(amount)}
                  </p>
                )}
              </div>

              {/* Date */}
              <div>
                <label className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Date <span className="text-[#5BC8A4]">*</span>
                </label>
                <input
                  type="date"
                  required
                  value={transactionDate}
                  onChange={(e) => setTransactionDate(e.target.value)}
                  className="w-full px-3.5 py-2.5 rounded-xl bg-white border border-[#E5EAF1] text-[#243044] text-sm focus:outline-none focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4]"
                />
              </div>

              {/* Category */}
              <div>
                <label className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Category <span className="text-[#718096] font-normal">(optional)</span>
                </label>
                <select
                  value={categoryId}
                  onChange={(e) => setCategoryId(e.target.value)}
                  className="w-full px-3.5 py-2.5 rounded-xl bg-white border border-[#E5EAF1] text-[#243044] text-sm focus:outline-none focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4]"
                >
                  <option value="">None (Uncategorized)</option>
                  {modalCategoryOptions.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
                <span className="block mt-1 text-[11px] text-[#718096]">
                  Showing {txType.toLowerCase()} categories only
                </span>
              </div>

              {/* Description */}
              <div>
                <label className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Description <span className="text-[#718096] font-normal">(optional)</span>
                </label>
                <input
                  type="text"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="e.g. Lunch with team, Freelance invoice #4"
                  className="w-full px-3.5 py-2.5 rounded-xl bg-white border border-[#E5EAF1] text-[#243044] text-sm focus:outline-none focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4]"
                />
              </div>

              <div className="pt-4 flex items-center justify-end gap-3 border-t border-[#E5EAF1]">
                <button
                  type="button"
                  onClick={() => setIsCreateOpen(false)}
                  className="px-4 py-2 rounded-xl text-sm font-medium text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC] cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={createMutation.isPending}
                  className="px-4 py-2 rounded-xl text-sm font-semibold text-white bg-[#5BC8A4] hover:bg-[#4db995] transition-colors flex items-center gap-2 cursor-pointer disabled:opacity-50 shadow-xs"
                >
                  {createMutation.isPending && <Loader2 className="w-4 h-4 animate-spin" />}
                  Save Transaction
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* EDIT MODAL */}
      {editingTransaction && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/40 backdrop-blur-xs animate-in fade-in">
          <div className="bg-white border border-[#E5EAF1] rounded-2xl w-full max-w-lg p-6 shadow-xl relative text-[#243044]">
            <div className="flex items-center justify-between pb-4 border-b border-[#E5EAF1]">
              <h3 className="text-lg font-bold text-[#243044]">Edit Transaction</h3>
              <button
                type="button"
                onClick={() => setEditingTransaction(null)}
                className="text-[#718096] hover:text-[#243044] p-1 rounded-lg hover:bg-[#F7F9FC] cursor-pointer"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {formError && (
              <div className="mt-4 p-3 rounded-xl bg-rose-50 border border-rose-200 text-xs font-medium text-[#C53030] flex items-center gap-2">
                <AlertCircle className="w-4 h-4 shrink-0 text-[#E98C8C]" />
                <span>{formError}</span>
              </div>
            )}

            <form onSubmit={handleSaveEdit} className="mt-4 space-y-4">
              {/* Type Switcher */}
              <div>
                <label className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Transaction Type
                </label>
                <div className="grid grid-cols-2 gap-3">
                  <button
                    type="button"
                    onClick={() => { setTxType('EXPENSE'); setCategoryId(''); }}
                    className={`py-2 px-3 rounded-xl text-xs font-semibold border flex items-center justify-center gap-2 transition-all cursor-pointer ${
                      txType === 'EXPENSE'
                        ? 'bg-rose-50 border-rose-200 text-[#C53030] shadow-xs'
                        : 'bg-white border-[#E5EAF1] text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC]'
                    }`}
                  >
                    <ArrowDownLeft className="w-4 h-4" />
                    Expense
                  </button>
                  <button
                    type="button"
                    onClick={() => { setTxType('INCOME'); setCategoryId(''); }}
                    className={`py-2 px-3 rounded-xl text-xs font-semibold border flex items-center justify-center gap-2 transition-all cursor-pointer ${
                      txType === 'INCOME'
                        ? 'bg-emerald-50 border-emerald-200 text-[#2F8F78] shadow-xs'
                        : 'bg-white border-[#E5EAF1] text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC]'
                    }`}
                  >
                    <ArrowUpRight className="w-4 h-4" />
                    Income
                  </button>
                </div>
              </div>

              {/* Amount (VND) */}
              <div>
                <label className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Amount (VND) <span className="text-[#5BC8A4]">*</span>
                </label>
                <div className="relative">
                  <input
                    type="number"
                    min="1"
                    step="1"
                    required
                    value={amount}
                    onChange={(e) => setAmount(e.target.value === '' ? '' : parseInt(e.target.value, 10))}
                    className="w-full pl-3.5 pr-14 py-2.5 rounded-xl bg-white border border-[#E5EAF1] text-[#243044] text-sm focus:outline-none focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4] font-mono [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                  />
                  <div className="absolute inset-y-0 right-0 pr-3.5 flex items-center pointer-events-none text-xs font-semibold text-[#718096] select-none">
                    VND
                  </div>
                </div>
                {typeof amount === 'number' && amount > 0 && (
                  <p className="mt-1 text-xs text-[#2F8F78] font-medium">
                    Formatted: {formatVND(amount)}
                  </p>
                )}
              </div>

              {/* Date */}
              <div>
                <label className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Date <span className="text-[#5BC8A4]">*</span>
                </label>
                <input
                  type="date"
                  required
                  value={transactionDate}
                  onChange={(e) => setTransactionDate(e.target.value)}
                  className="w-full px-3.5 py-2.5 rounded-xl bg-white border border-[#E5EAF1] text-[#243044] text-sm focus:outline-none focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4]"
                />
              </div>

              {/* Category */}
              <div>
                <label className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Category <span className="text-[#718096] font-normal">(optional)</span>
                </label>
                <select
                  value={categoryId}
                  onChange={(e) => setCategoryId(e.target.value)}
                  className="w-full px-3.5 py-2.5 rounded-xl bg-white border border-[#E5EAF1] text-[#243044] text-sm focus:outline-none focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4]"
                >
                  <option value="">None (Uncategorized)</option>
                  {modalCategoryOptions.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
              </div>

              {/* Description */}
              <div>
                <label className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Description <span className="text-[#718096] font-normal">(optional)</span>
                </label>
                <input
                  type="text"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  className="w-full px-3.5 py-2.5 rounded-xl bg-white border border-[#E5EAF1] text-[#243044] text-sm focus:outline-none focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4]"
                />
              </div>

              <div className="pt-4 flex items-center justify-end gap-3 border-t border-[#E5EAF1]">
                <button
                  type="button"
                  onClick={() => setEditingTransaction(null)}
                  className="px-4 py-2 rounded-xl text-sm font-medium text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC] cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={updateMutation.isPending}
                  className="px-4 py-2 rounded-xl text-sm font-semibold text-white bg-[#5BC8A4] hover:bg-[#4db995] transition-colors flex items-center gap-2 cursor-pointer disabled:opacity-50 shadow-xs"
                >
                  {updateMutation.isPending && <Loader2 className="w-4 h-4 animate-spin" />}
                  Save Changes
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* DELETE CONFIRMATION MODAL */}
      {deletingTransaction && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/40 backdrop-blur-xs animate-in fade-in">
          <div className="bg-white border border-[#E5EAF1] rounded-2xl w-full max-w-md p-6 shadow-xl relative text-[#243044]">
            <div className="flex items-center gap-3 text-rose-600 mb-4">
              <div className="w-10 h-10 rounded-xl bg-rose-50 border border-rose-200 flex items-center justify-center">
                <Trash2 className="w-5 h-5 text-[#C53030]" />
              </div>
              <h3 className="text-lg font-bold text-[#243044]">Delete Transaction</h3>
            </div>

            <p className="text-sm text-[#718096] mb-4">
              Are you sure you want to delete this {deletingTransaction.type.toLowerCase()} of{' '}
              <strong className="text-[#243044] font-semibold">{formatVND(deletingTransaction.amount)}</strong> on{' '}
              <strong className="text-[#243044] font-semibold">{formatDate(deletingTransaction.transaction_date)}</strong>?
            </p>

            <div className="flex items-center justify-end gap-3 pt-4 border-t border-[#E5EAF1]">
              <button
                type="button"
                onClick={() => setDeletingTransaction(null)}
                className="px-4 py-2 rounded-xl text-sm font-medium text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC] cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="button"
                disabled={deleteMutation.isPending}
                onClick={() => deleteMutation.mutate(deletingTransaction.id)}
                className="px-4 py-2 rounded-xl text-sm font-semibold text-white bg-rose-600 hover:bg-rose-500 transition-colors flex items-center gap-2 cursor-pointer disabled:opacity-50 shadow-xs"
              >
                {deleteMutation.isPending && <Loader2 className="w-4 h-4 animate-spin" />}
                Delete Record
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
