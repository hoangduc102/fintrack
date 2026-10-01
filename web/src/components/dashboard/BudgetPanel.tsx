import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getBudgets, upsertBudget, deleteBudget } from '../../api/budgets'
import { getCategories } from '../../api/categories'
import type { Budget, UpsertBudgetInput } from '../../api/types'
import { formatVND } from '../../utils/format'
import { 
  Target, 
  Plus, 
  Trash2, 
  Edit3, 
  AlertCircle, 
  CheckCircle2, 
  AlertTriangle,
  Loader2,
  X
} from 'lucide-react'

interface BudgetPanelProps {
  year: number
  month: number
}

export const BudgetPanel: React.FC<BudgetPanelProps> = ({ year, month }) => {
  const queryClient = useQueryClient()
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [editingBudget, setEditingBudget] = useState<Budget | null>(null)
  const [deletingId, setDeletingId] = useState<string | null>(null)

  // Form state
  const [selectedCategoryId, setSelectedCategoryId] = useState('')
  const [amountInput, setAmountInput] = useState('')
  const [formError, setFormError] = useState<string | null>(null)

  const monthStr = `${year}-${String(month).padStart(2, '0')}`

  // Fetch budgets for this month
  const { data: budgets = [], isLoading: isLoadingBudgets } = useQuery({
    queryKey: ['budgets', year, month],
    queryFn: () => getBudgets(year, month),
  })

  // Fetch expense categories
  const { data: categories = [], isLoading: isLoadingCategories } = useQuery({
    queryKey: ['categories', 'EXPENSE'],
    queryFn: () => getCategories('EXPENSE'),
  })

  // Upsert mutation
  const upsertMutation = useMutation({
    mutationFn: (input: UpsertBudgetInput) => upsertBudget(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['budgets', year, month] })
      queryClient.invalidateQueries({ queryKey: ['analytics-summary', monthStr] })
      handleCloseModal()
    },
    onError: (err: any) => {
      setFormError(err?.message || 'Failed to save budget')
    },
  })

  // Delete mutation
  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteBudget(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['budgets', year, month] })
      queryClient.invalidateQueries({ queryKey: ['analytics-summary', monthStr] })
      setDeletingId(null)
    },
  })

  const handleOpenAdd = () => {
    setEditingBudget(null)
    setSelectedCategoryId(categories[0]?.id || '')
    setAmountInput('')
    setFormError(null)
    setIsModalOpen(true)
  }

  const handleOpenEdit = (budget: Budget) => {
    setEditingBudget(budget)
    setSelectedCategoryId(budget.category_id)
    setAmountInput(budget.amount.toString())
    setFormError(null)
    setIsModalOpen(true)
  }

  const handleCloseModal = () => {
    setIsModalOpen(false)
    setEditingBudget(null)
    setSelectedCategoryId('')
    setAmountInput('')
    setFormError(null)
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)

    const effectiveCatId = selectedCategoryId || categories[0]?.id || ''
    if (!effectiveCatId) {
      setFormError('Please select a category')
      return
    }

    const numAmount = parseInt(amountInput.replace(/[^0-9]/g, ''), 10)
    if (isNaN(numAmount) || numAmount <= 0) {
      setFormError('Budget amount must be greater than 0 VND')
      return
    }

    upsertMutation.mutate({
      category_id: effectiveCatId,
      amount: numAmount,
      year,
      month,
    })
  }

  // Calculate totals across all budgets for the header
  const totalBudgeted = budgets.reduce((sum, b) => sum + b.amount, 0)
  const totalSpent = budgets.reduce((sum, b) => sum + b.spent, 0)
  const overallPercentage = totalBudgeted > 0 ? (totalSpent / totalBudgeted) * 100 : 0

  return (
    <div className="rounded-2xl bg-slate-900/60 border border-slate-800 p-6 flex flex-col">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-5 border-b border-slate-800">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-lg bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400">
            <Target className="w-4 h-4" />
          </div>
          <div>
            <h3 className="text-base font-bold text-white leading-tight">Monthly Budgets</h3>
            <span className="text-xs text-slate-400">
              Track category spending limits for {String(month).padStart(2, '0')}/{year}
            </span>
          </div>
        </div>

        <button
          onClick={handleOpenAdd}
          className="inline-flex items-center gap-2 px-3.5 py-2 text-xs font-semibold text-white bg-emerald-600 hover:bg-emerald-500 active:bg-emerald-700 rounded-xl transition shadow-lg shadow-emerald-950/40 w-fit"
        >
          <Plus className="w-4 h-4" />
          <span>Set Budget</span>
        </button>
      </div>

      {/* Overall Progress if budgets exist */}
      {budgets.length > 0 && (
        <div className="mt-4 p-4 rounded-xl bg-slate-800/40 border border-slate-800">
          <div className="flex items-center justify-between text-xs mb-2">
            <span className="text-slate-400 font-medium">Total Budget Allocation</span>
            <span className="font-semibold text-white">
              {formatVND(totalSpent)} / <span className="text-slate-400">{formatVND(totalBudgeted)}</span>
              <span className={`ml-2 font-bold ${
                overallPercentage > 100 ? 'text-rose-400' : overallPercentage >= 80 ? 'text-amber-400' : 'text-emerald-400'
              }`}>
                ({overallPercentage.toFixed(1)}%)
              </span>
            </span>
          </div>
          <div className="w-full bg-slate-800 h-2.5 rounded-full overflow-hidden">
            <div
              className={`h-full transition-all duration-500 rounded-full ${
                overallPercentage > 100
                  ? 'bg-rose-500'
                  : overallPercentage >= 80
                  ? 'bg-amber-400'
                  : 'bg-emerald-500'
              }`}
              style={{ width: `${Math.min(overallPercentage, 100)}%` }}
            />
          </div>
        </div>
      )}

      {/* Content */}
      <div className="mt-4">
        {isLoadingBudgets ? (
          <div className="h-48 flex flex-col items-center justify-center text-slate-400">
            <Loader2 className="w-7 h-7 text-emerald-400 animate-spin mb-2" />
            <p className="text-xs">Loading monthly budgets...</p>
          </div>
        ) : budgets.length === 0 ? (
          <div className="py-12 flex flex-col items-center justify-center text-center">
            <div className="w-12 h-12 rounded-2xl bg-slate-800/80 border border-slate-700/60 flex items-center justify-center text-slate-400 mb-3">
              <Target className="w-6 h-6" />
            </div>
            <p className="text-sm font-semibold text-slate-300">No budgets set for this month</p>
            <p className="text-xs text-slate-500 mt-1 max-w-sm">
              Establish category spending goals to prevent overspending and grow your net savings.
            </p>
            <button
              onClick={handleOpenAdd}
              className="mt-4 inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 hover:bg-emerald-500/20 rounded-lg transition"
            >
              <Plus className="w-3.5 h-3.5" />
              <span>Create First Budget</span>
            </button>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3.5">
            {budgets.map((b) => {
              const percentage = b.amount > 0 ? (b.spent / b.amount) * 100 : 0
              const isOver = b.spent > b.amount
              const isWarning = !isOver && percentage >= 80
              const remaining = b.amount - b.spent

              return (
                <div
                  key={b.id}
                  className="p-4 rounded-xl bg-slate-800/30 hover:bg-slate-800/60 border border-slate-800/80 transition flex flex-col justify-between"
                >
                  <div>
                    <div className="flex items-start justify-between gap-2 mb-2">
                      <div className="flex items-center gap-2">
                        <span className="font-semibold text-sm text-white">
                          {b.category_name || 'Uncategorized'}
                        </span>
                        {isOver ? (
                          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-rose-500/10 text-rose-400 border border-rose-500/20">
                            <AlertCircle className="w-3 h-3" />
                            Overbudget
                          </span>
                        ) : isWarning ? (
                          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-amber-500/10 text-amber-400 border border-amber-500/20">
                            <AlertTriangle className="w-3 h-3" />
                            Near Limit
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                            <CheckCircle2 className="w-3 h-3" />
                            Healthy
                          </span>
                        )}
                      </div>

                      <div className="flex items-center gap-1">
                        <button
                          onClick={() => handleOpenEdit(b)}
                          className="p-1 text-slate-400 hover:text-white hover:bg-slate-700/60 rounded-lg transition"
                          title="Edit budget"
                        >
                          <Edit3 className="w-3.5 h-3.5" />
                        </button>
                        <button
                          onClick={() => setDeletingId(b.id)}
                          className="p-1 text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 rounded-lg transition"
                          title="Delete budget"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </div>

                    {/* Progress Bar */}
                    <div className="w-full bg-slate-800 h-2 rounded-full overflow-hidden mt-3 mb-2.5">
                      <div
                        className={`h-full transition-all duration-500 rounded-full ${
                          isOver ? 'bg-rose-500' : isWarning ? 'bg-amber-400' : 'bg-emerald-500'
                        }`}
                        style={{ width: `${Math.min(percentage, 100)}%` }}
                      />
                    </div>
                  </div>

                  {/* Metrics Footer */}
                  <div className="flex items-center justify-between text-xs pt-1 border-t border-slate-800/60">
                    <div>
                      <span className="text-slate-400 text-[11px]">Spent: </span>
                      <span className="font-semibold text-slate-200">{formatVND(b.spent)}</span>
                    </div>
                    <div className="text-right">
                      <span className="text-slate-400 text-[11px]">
                        {isOver ? 'Exceeded by: ' : 'Remaining: '}
                      </span>
                      <span
                        className={`font-semibold ${
                          isOver ? 'text-rose-400' : 'text-emerald-400'
                        }`}
                      >
                        {formatVND(Math.abs(remaining))}
                      </span>
                      <span className="text-slate-500 text-[10px] ml-1">
                        ({percentage.toFixed(0)}%)
                      </span>
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Delete Confirmation Modal */}
      {deletingId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-in fade-in duration-150">
          <div className="w-full max-w-sm bg-slate-900 border border-slate-800 p-5 rounded-2xl shadow-2xl">
            <h4 className="text-base font-bold text-white mb-2">Delete Budget?</h4>
            <p className="text-xs text-slate-400 mb-5 leading-relaxed">
              Are you sure you want to remove this budget? Your transaction history will remain intact.
            </p>
            <div className="flex justify-end gap-2.5">
              <button
                type="button"
                onClick={() => setDeletingId(null)}
                className="px-3 py-1.5 text-xs font-semibold text-slate-300 hover:text-white bg-slate-800 hover:bg-slate-700 rounded-xl transition"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => deleteMutation.mutate(deletingId)}
                disabled={deleteMutation.isPending}
                className="px-3 py-1.5 text-xs font-semibold text-white bg-rose-600 hover:bg-rose-500 rounded-xl transition flex items-center gap-1.5"
              >
                {deleteMutation.isPending && <Loader2 className="w-3 h-3 animate-spin" />}
                <span>Delete</span>
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Upsert Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-in fade-in duration-150">
          <div className="w-full max-w-md bg-slate-900 border border-slate-800 p-6 rounded-2xl shadow-2xl">
            <div className="flex items-center justify-between pb-4 border-b border-slate-800 mb-4">
              <h4 className="text-base font-bold text-white">
                {editingBudget ? 'Edit Monthly Budget' : 'Set Monthly Budget'}
              </h4>
              <button
                onClick={handleCloseModal}
                className="p-1 text-slate-400 hover:text-white rounded-lg transition"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {formError && (
              <div className="mb-4 p-3 rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs flex items-center gap-2">
                <AlertCircle className="w-4 h-4 shrink-0" />
                <span>{formError}</span>
              </div>
            )}

            <form onSubmit={handleSubmit} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1.5">
                  Category (Expenses Only)
                </label>
                {isLoadingCategories ? (
                  <div className="text-xs text-slate-400 py-2">Loading categories...</div>
                ) : categories.length === 0 ? (
                  <div className="text-xs text-amber-400 py-2">
                    No expense categories found. Please create an expense category first.
                  </div>
                ) : (
                  <select
                    value={selectedCategoryId || categories[0]?.id || ''}
                    onChange={(e) => setSelectedCategoryId(e.target.value)}
                    disabled={!!editingBudget}
                    className="w-full bg-slate-800/80 border border-slate-700/80 rounded-xl px-3.5 py-2 text-sm text-white focus:outline-none focus:border-emerald-500 transition disabled:opacity-60 disabled:cursor-not-allowed"
                  >
                    {categories.map((cat) => (
                      <option key={cat.id} value={cat.id}>
                        {cat.name}
                      </option>
                    ))}
                  </select>
                )}
                {editingBudget && (
                  <p className="text-[11px] text-slate-500 mt-1">
                    Category cannot be changed when editing an existing budget.
                  </p>
                )}
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1.5">
                  Monthly Limit (VND)
                </label>
                <input
                  type="text"
                  placeholder="e.g. 2,000,000"
                  value={amountInput ? Number(amountInput.replace(/[^0-9]/g, '')).toLocaleString('en-US') : ''}
                  onChange={(e) => setAmountInput(e.target.value.replace(/[^0-9]/g, ''))}
                  className="w-full bg-slate-800/80 border border-slate-700/80 rounded-xl px-3.5 py-2 text-sm text-white focus:outline-none focus:border-emerald-500 transition font-mono"
                  required
                />
              </div>

              <div className="text-[11px] text-slate-400 bg-slate-800/30 p-2.5 rounded-lg border border-slate-800">
                Target Period: <span className="font-semibold text-white">{String(month).padStart(2, '0')}/{year}</span>
              </div>

              <div className="flex items-center justify-end gap-2.5 pt-3">
                <button
                  type="button"
                  onClick={handleCloseModal}
                  className="px-4 py-2 text-xs font-semibold text-slate-300 hover:text-white bg-slate-800 hover:bg-slate-700 rounded-xl transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={upsertMutation.isPending || categories.length === 0}
                  className="px-4 py-2 text-xs font-semibold text-white bg-emerald-600 hover:bg-emerald-500 active:bg-emerald-700 rounded-xl transition flex items-center gap-1.5 disabled:opacity-50"
                >
                  {upsertMutation.isPending && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                  <span>{editingBudget ? 'Update Budget' : 'Save Budget'}</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
