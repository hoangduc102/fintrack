import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getCategories, createCategory, updateCategory, deleteCategory } from '../api/categories'
import { ApiError } from '../api/client'
import type { Category, CategoryType } from '../api/types'
import { 
  Tags, 
  Plus, 
  ArrowDownLeft, 
  ArrowUpRight, 
  Pencil, 
  Trash2, 
  AlertCircle, 
  Loader2, 
  X, 
  CheckCircle2,
  Info
} from 'lucide-react'

export const CategoriesPage: React.FC = () => {
  const queryClient = useQueryClient()

  // Modals state
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [editingCategory, setEditingCategory] = useState<Category | null>(null)
  const [deletingCategory, setDeletingCategory] = useState<Category | null>(null)

  // Form states
  const [name, setName] = useState('')
  const [type, setType] = useState<CategoryType>('EXPENSE')
  const [formError, setFormError] = useState<string | null>(null)
  const [feedback, setFeedback] = useState<{ message: string; type: 'success' | 'error' } | null>(null)

  // Fetch categories query
  const { data: categories = [], isLoading, error: queryError } = useQuery({
    queryKey: ['categories'],
    queryFn: () => getCategories(),
  })

  const invalidateCategoryQueries = () => {
    queryClient.invalidateQueries({ queryKey: ['categories'] })
    queryClient.invalidateQueries({ queryKey: ['transactions'] })
    queryClient.invalidateQueries({ queryKey: ['analytics-categories'] })
    queryClient.invalidateQueries({ queryKey: ['budgets'] })
  }

  // Create mutation
  const createMutation = useMutation({
    mutationFn: (data: { name: string; type: CategoryType }) => createCategory(data),
    onSuccess: (newCat) => {
      invalidateCategoryQueries()
      setIsCreateOpen(false)
      setName('')
      setFormError(null)
      showFeedback(`Category "${newCat.name}" created successfully.`)
    },
    onError: (err: unknown) => {
      if (err instanceof ApiError) {
        if (err.status === 409 || err.code === 'CONFLICT') {
          setFormError(`A category named "${name.trim()}" (${type}) already exists.`)
        } else {
          setFormError(err.message || 'Failed to create category')
        }
      } else {
        setFormError('Network error while creating category')
      }
    },
  })

  // Update mutation
  const updateMutation = useMutation({
    mutationFn: ({ id, name }: { id: string; name: string }) => updateCategory(id, { name }),
    onSuccess: (updatedCat) => {
      invalidateCategoryQueries()
      setEditingCategory(null)
      setName('')
      setFormError(null)
      showFeedback(`Category updated to "${updatedCat.name}".`)
    },
    onError: (err: unknown) => {
      if (err instanceof ApiError) {
        if (err.status === 409 || err.code === 'CONFLICT') {
          setFormError(`A category with this name already exists.`)
        } else {
          setFormError(err.message || 'Failed to update category')
        }
      } else {
        setFormError('Network error while updating category')
      }
    },
  })

  // Delete mutation
  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteCategory(id),
    onSuccess: () => {
      invalidateCategoryQueries()
      const deletedName = deletingCategory?.name
      setDeletingCategory(null)
      showFeedback(`Category "${deletedName}" deleted. Associated transactions are now uncategorized.`)
    },
    onError: (err: unknown) => {
      if (err instanceof ApiError) {
        setFormError(err.message || 'Failed to delete category')
      } else {
        setFormError('Network error while deleting category')
      }
    },
  })

  const showFeedback = (message: string, type: 'success' | 'error' = 'success') => {
    setFeedback({ message, type })
    setTimeout(() => {
      setFeedback(null)
    }, 4000)
  }

  // Open modals handlers
  const handleOpenCreate = (defaultType: CategoryType = 'EXPENSE') => {
    setName('')
    setType(defaultType)
    setFormError(null)
    setIsCreateOpen(true)
  }

  const handleOpenEdit = (cat: Category) => {
    setEditingCategory(cat)
    setName(cat.name)
    setFormError(null)
  }

  const handleOpenDelete = (cat: Category) => {
    setDeletingCategory(cat)
    setFormError(null)
  }

  const handleSaveCreate = (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    const trimmed = name.trim()
    if (!trimmed) {
      setFormError('Category name is required')
      return
    }
    if (trimmed.length > 50) {
      setFormError('Category name must not exceed 50 characters')
      return
    }
    createMutation.mutate({ name: trimmed, type })
  }

  const handleSaveEdit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingCategory) return
    setFormError(null)
    const trimmed = name.trim()
    if (!trimmed) {
      setFormError('Category name is required')
      return
    }
    if (trimmed.length > 50) {
      setFormError('Category name must not exceed 50 characters')
      return
    }
    updateMutation.mutate({ id: editingCategory.id, name: trimmed })
  }

  // Filter groups
  const expenseCategories = categories.filter((c) => c.type === 'EXPENSE')
  const incomeCategories = categories.filter((c) => c.type === 'INCOME')

  return (
    <div className="space-y-6 sm:space-y-8 animate-in fade-in duration-300">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-[#243044] tracking-tight flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-emerald-50 border border-emerald-200/60 flex items-center justify-center text-[#2F8F78]">
              <Tags className="w-5 h-5" />
            </div>
            Categories
          </h1>
          <p className="mt-1 text-sm text-[#718096]">
            Organize and classify your income and expenses for accurate tracking and budgeting
          </p>
        </div>

        <button
          type="button"
          onClick={() => handleOpenCreate('EXPENSE')}
          className="inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl text-sm font-semibold text-white bg-[#5BC8A4] hover:bg-[#4db995] shadow-xs transition-all cursor-pointer"
        >
          <Plus className="w-4 h-4" />
          Add Category
        </button>
      </div>

      {/* Floating feedback alert */}
      {feedback && (
        <div className="rounded-xl bg-emerald-50 border border-emerald-200 p-4 flex items-center gap-3 text-[#2F8F78] animate-in fade-in slide-in-from-top-2">
          <CheckCircle2 className="w-5 h-5 text-[#5BC8A4] shrink-0" />
          <span className="text-sm font-medium">{feedback.message}</span>
        </div>
      )}

      {queryError && (
        <div className="rounded-xl bg-rose-50 border border-rose-200 p-4 flex items-center gap-3 text-[#C53030]">
          <AlertCircle className="w-5 h-5 text-[#E98C8C] shrink-0" />
          <span className="text-sm font-medium">Failed to load categories. Please check your backend connection.</span>
        </div>
      )}

      {/* Categories Content Columns */}
      {isLoading ? (
        <div className="flex flex-col items-center justify-center py-20 text-[#718096]">
          <Loader2 className="w-8 h-8 text-[#5BC8A4] animate-spin mb-3" />
          <p className="text-sm">Loading categories...</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
          {/* Expense Categories Column */}
          <div className="rounded-2xl bg-white border border-[#E5EAF1] p-6 flex flex-col shadow-xs">
            <div className="flex items-center justify-between pb-4 border-b border-[#E5EAF1] mb-6">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-xl bg-rose-50 border border-rose-200 flex items-center justify-center text-[#C53030]">
                  <ArrowDownLeft className="w-4 h-4" />
                </div>
                <div>
                  <h2 className="text-base font-bold text-[#243044] leading-tight">Expense Categories</h2>
                  <span className="text-xs text-[#718096]">{expenseCategories.length} categories configured</span>
                </div>
              </div>

              <button
                type="button"
                onClick={() => handleOpenCreate('EXPENSE')}
                className="text-xs font-semibold text-[#C53030] hover:text-rose-700 hover:bg-rose-50 px-2.5 py-1.5 rounded-lg border border-rose-200 transition-colors cursor-pointer"
              >
                + Add Expense
              </button>
            </div>

            {expenseCategories.length === 0 ? (
              <div className="py-12 text-center text-[#718096] border border-dashed border-[#E5EAF1] rounded-xl bg-[#F7F9FC]">
                <p className="text-sm">No expense categories yet.</p>
              </div>
            ) : (
              <div className="space-y-3">
                {expenseCategories.map((cat) => (
                  <div
                    key={cat.id}
                    className="flex items-center justify-between p-3.5 rounded-xl bg-[#F7F9FC] border border-[#E5EAF1] hover:border-emerald-300 transition-all group"
                  >
                    <div className="flex items-center gap-3">
                      <div className="w-2.5 h-2.5 rounded-full bg-[#E98C8C]" />
                      <div>
                        <span className="text-sm font-semibold text-[#243044]">{cat.name}</span>
                        <span className="block text-[11px] text-[#718096]">Expense</span>
                      </div>
                    </div>

                    <div className="flex items-center gap-1 opacity-90 group-hover:opacity-100 transition-opacity">
                      <button
                        type="button"
                        onClick={() => handleOpenEdit(cat)}
                        className="p-1.5 rounded-lg text-[#718096] hover:text-[#243044] hover:bg-white transition-colors cursor-pointer"
                        title="Edit name"
                      >
                        <Pencil className="w-3.5 h-3.5" />
                      </button>
                      <button
                        type="button"
                        onClick={() => handleOpenDelete(cat)}
                        className="p-1.5 rounded-lg text-[#718096] hover:text-rose-600 hover:bg-rose-50 transition-colors cursor-pointer"
                        title="Delete category"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Income Categories Column */}
          <div className="rounded-2xl bg-white border border-[#E5EAF1] p-6 flex flex-col shadow-xs">
            <div className="flex items-center justify-between pb-4 border-b border-[#E5EAF1] mb-6">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-xl bg-emerald-50 border border-emerald-200 flex items-center justify-center text-[#2F8F78]">
                  <ArrowUpRight className="w-4 h-4" />
                </div>
                <div>
                  <h2 className="text-base font-bold text-[#243044] leading-tight">Income Categories</h2>
                  <span className="text-xs text-[#718096]">{incomeCategories.length} categories configured</span>
                </div>
              </div>

              <button
                type="button"
                onClick={() => handleOpenCreate('INCOME')}
                className="text-xs font-semibold text-[#2F8F78] hover:text-emerald-800 hover:bg-emerald-50 px-2.5 py-1.5 rounded-lg border border-emerald-200 transition-colors cursor-pointer"
              >
                + Add Income
              </button>
            </div>

            {incomeCategories.length === 0 ? (
              <div className="py-12 text-center text-[#718096] border border-dashed border-[#E5EAF1] rounded-xl bg-[#F7F9FC]">
                <p className="text-sm">No income categories yet.</p>
              </div>
            ) : (
              <div className="space-y-3">
                {incomeCategories.map((cat) => (
                  <div
                    key={cat.id}
                    className="flex items-center justify-between p-3.5 rounded-xl bg-[#F7F9FC] border border-[#E5EAF1] hover:border-emerald-300 transition-all group"
                  >
                    <div className="flex items-center gap-3">
                      <div className="w-2.5 h-2.5 rounded-full bg-[#5BC8A4]" />
                      <div>
                        <span className="text-sm font-semibold text-[#243044]">{cat.name}</span>
                        <span className="block text-[11px] text-[#718096]">Income</span>
                      </div>
                    </div>

                    <div className="flex items-center gap-1 opacity-90 group-hover:opacity-100 transition-opacity">
                      <button
                        type="button"
                        onClick={() => handleOpenEdit(cat)}
                        className="p-1.5 rounded-lg text-[#718096] hover:text-[#243044] hover:bg-white transition-colors cursor-pointer"
                        title="Edit name"
                      >
                        <Pencil className="w-3.5 h-3.5" />
                      </button>
                      <button
                        type="button"
                        onClick={() => handleOpenDelete(cat)}
                        className="p-1.5 rounded-lg text-[#718096] hover:text-rose-600 hover:bg-rose-50 transition-colors cursor-pointer"
                        title="Delete category"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* CREATE MODAL */}
      {isCreateOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/40 backdrop-blur-xs animate-in fade-in">
          <div className="bg-white border border-[#E5EAF1] rounded-2xl w-full max-w-md p-6 shadow-xl relative text-[#243044]">
            <div className="flex items-center justify-between pb-4 border-b border-[#E5EAF1]">
              <h3 className="text-lg font-bold text-[#243044]">Create New Category</h3>
              <button
                type="button"
                onClick={() => setIsCreateOpen(false)}
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

            <form onSubmit={handleSaveCreate} className="mt-4 space-y-4">
              <div>
                <label className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Category Name
                </label>
                <input
                  type="text"
                  required
                  maxLength={50}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. Coffee & Snacks"
                  className="w-full px-3.5 py-2.5 rounded-xl bg-white border border-[#E5EAF1] text-[#243044] text-sm focus:outline-none focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4]"
                />
                <div className="flex justify-end mt-1 text-[11px] text-[#718096]">
                  {name.length}/50 characters
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Category Type
                </label>
                <div className="grid grid-cols-2 gap-3">
                  <button
                    type="button"
                    onClick={() => setType('EXPENSE')}
                    className={`py-2 px-3 rounded-xl text-xs font-semibold border flex items-center justify-center gap-2 transition-all cursor-pointer ${
                      type === 'EXPENSE'
                        ? 'bg-rose-50 border-rose-200 text-[#C53030] shadow-xs'
                        : 'bg-white border-[#E5EAF1] text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC]'
                    }`}
                  >
                    <ArrowDownLeft className="w-4 h-4" />
                    Expense
                  </button>
                  <button
                    type="button"
                    onClick={() => setType('INCOME')}
                    className={`py-2 px-3 rounded-xl text-xs font-semibold border flex items-center justify-center gap-2 transition-all cursor-pointer ${
                      type === 'INCOME'
                        ? 'bg-emerald-50 border-emerald-200 text-[#2F8F78] shadow-xs'
                        : 'bg-white border-[#E5EAF1] text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC]'
                    }`}
                  >
                    <ArrowUpRight className="w-4 h-4" />
                    Income
                  </button>
                </div>
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
                  Create Category
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* EDIT MODAL */}
      {editingCategory && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/40 backdrop-blur-xs animate-in fade-in">
          <div className="bg-white border border-[#E5EAF1] rounded-2xl w-full max-w-md p-6 shadow-xl relative text-[#243044]">
            <div className="flex items-center justify-between pb-4 border-b border-[#E5EAF1]">
              <h3 className="text-lg font-bold text-[#243044]">Edit Category</h3>
              <button
                type="button"
                onClick={() => setEditingCategory(null)}
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
              <div>
                <label className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Category Name
                </label>
                <input
                  type="text"
                  required
                  maxLength={50}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="w-full px-3.5 py-2.5 rounded-xl bg-white border border-[#E5EAF1] text-[#243044] text-sm focus:outline-none focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4]"
                />
                <div className="flex justify-between mt-1 text-[11px] text-[#718096]">
                  <span>Type: <strong className="text-[#243044]">{editingCategory.type}</strong> (fixed)</span>
                  <span>{name.length}/50 characters</span>
                </div>
              </div>

              <div className="pt-4 flex items-center justify-end gap-3 border-t border-[#E5EAF1]">
                <button
                  type="button"
                  onClick={() => setEditingCategory(null)}
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
      {deletingCategory && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/40 backdrop-blur-xs animate-in fade-in">
          <div className="bg-white border border-[#E5EAF1] rounded-2xl w-full max-w-md p-6 shadow-xl relative text-[#243044]">
            <div className="flex items-center gap-3 text-[#C53030] mb-4">
              <div className="w-10 h-10 rounded-xl bg-rose-50 border border-rose-200 flex items-center justify-center">
                <Trash2 className="w-5 h-5 text-[#C53030]" />
              </div>
              <h3 className="text-lg font-bold text-[#243044]">Delete Category</h3>
            </div>

            <p className="text-sm text-[#718096] mb-4">
              Are you sure you want to delete <strong className="text-[#243044]">"{deletingCategory.name}"</strong>?
            </p>

            <div className="p-3 rounded-xl bg-amber-50 border border-amber-200 text-xs text-[#B45309] flex items-start gap-2.5 mb-6">
              <Info className="w-4 h-4 text-[#D97706] shrink-0 mt-0.5" />
              <span>
                <strong>Data Preservation:</strong> Existing transactions in this category will NOT be deleted. They will remain in your history and be marked as <strong>Uncategorized</strong>.
              </span>
            </div>

            <div className="flex items-center justify-end gap-3 pt-4 border-t border-[#E5EAF1]">
              <button
                type="button"
                onClick={() => setDeletingCategory(null)}
                className="px-4 py-2 rounded-xl text-sm font-medium text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC] cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="button"
                disabled={deleteMutation.isPending}
                onClick={() => deleteMutation.mutate(deletingCategory.id)}
                className="px-4 py-2 rounded-xl text-sm font-semibold text-white bg-rose-600 hover:bg-rose-500 transition-colors flex items-center gap-2 cursor-pointer disabled:opacity-50 shadow-xs"
              >
                {deleteMutation.isPending && <Loader2 className="w-4 h-4 animate-spin" />}
                Delete Category
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
