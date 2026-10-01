import React, { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useAuth } from '../context/AuthContext'
import { getMonthlySummary, getCategoryExpenses, getMonthlyTrend } from '../api/analytics'
import { MetricCards } from '../components/dashboard/MetricCards'
import { TrendChart } from '../components/dashboard/TrendChart'
import { CategoryDistributionChart } from '../components/dashboard/CategoryDistributionChart'
import { BudgetPanel } from '../components/dashboard/BudgetPanel'
import { CsvImportModal } from '../components/dashboard/CsvImportModal'
import {
  UploadCloud,
  PlusCircle,
  ChevronLeft,
  ChevronRight,
  Calendar,
  Sparkles,
  ReceiptText,
  Tags,
  ShieldCheck,
  CheckCircle2
} from 'lucide-react'

export const DashboardPage: React.FC = () => {
  const { user } = useAuth()

  // Default to current month: "YYYY-MM"
  const now = new Date()
  const currentMonthStr = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
  const [selectedMonth, setSelectedMonth] = useState(currentMonthStr)
  const [isImportModalOpen, setIsImportModalOpen] = useState(false)

  // Scroll to anchor on load if hash is present
  useEffect(() => {
    if (window.location.hash) {
      const element = document.querySelector(window.location.hash)
      if (element) {
        setTimeout(() => {
          element.scrollIntoView({ behavior: 'smooth' })
        }, 100)
      }
    }
  }, [])

  // Parse year and month
  const [yearStr, monthStr] = selectedMonth.split('-')
  const currentYear = parseInt(yearStr, 10) || now.getFullYear()
  const currentMonth = parseInt(monthStr, 10) || now.getMonth() + 1

  // Month navigation handlers
  const handlePrevMonth = () => {
    let y = currentYear
    let m = currentMonth - 1
    if (m < 1) {
      m = 12
      y -= 1
    }
    setSelectedMonth(`${y}-${String(m).padStart(2, '0')}`)
  }

  const handleNextMonth = () => {
    let y = currentYear
    let m = currentMonth + 1
    if (m > 12) {
      m = 1
      y += 1
    }
    setSelectedMonth(`${y}-${String(m).padStart(2, '0')}`)
  }

  const handleResetToCurrentMonth = () => {
    setSelectedMonth(currentMonthStr)
  }

  // Format month for display e.g. "March 2026"
  const dateObj = new Date(currentYear, currentMonth - 1, 1)
  const monthDisplayTitle = dateObj.toLocaleDateString('en-US', {
    month: 'long',
    year: 'numeric',
  })

  // Queries
  const { data: summary, isLoading: isLoadingSummary } = useQuery({
    queryKey: ['analytics-summary', selectedMonth],
    queryFn: () => getMonthlySummary(selectedMonth),
  })

  const { data: categoriesShare = [], isLoading: isLoadingCategoriesShare } = useQuery({
    queryKey: ['analytics-categories', selectedMonth],
    queryFn: () => getCategoryExpenses(selectedMonth),
  })

  const { data: trendData = [], isLoading: isLoadingTrend } = useQuery({
    queryKey: ['analytics-trend', 6],
    queryFn: () => getMonthlyTrend(6),
  })

  const displayName = user?.display_name || user?.email?.split('@')[0] || 'User'

  return (
    <div className="space-y-8 animate-in fade-in duration-500 pb-12">
      {/* Welcome & Period Header */}
      <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-6 pb-2 border-b border-slate-800">
        <div>
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-500/10 border border-emerald-500/20 text-xs font-semibold text-emerald-400 mb-2">
            <Sparkles className="w-3.5 h-3.5" />
            <span>Financial Overview</span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-white tracking-tight">
            Welcome back, <span className="text-emerald-400">{displayName}</span>
          </h1>
          <p className="text-xs sm:text-sm text-slate-400 mt-1">
            Real-time monthly summaries, category allocations, budgets, and historical cash flow.
          </p>
        </div>

        {/* Action Controls & Month Picker */}
        <div className="flex flex-wrap items-center gap-3">
          {/* Month Selector */}
          <div className="flex items-center bg-slate-900 border border-slate-800 rounded-2xl p-1 shadow-lg">
            <button
              onClick={handlePrevMonth}
              title="Previous Month"
              className="p-2 text-slate-400 hover:text-white hover:bg-slate-800 rounded-xl transition"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>

            <div className="flex items-center gap-2 px-3">
              <Calendar className="w-4 h-4 text-emerald-400" />
              <input
                type="month"
                value={selectedMonth}
                onChange={(e) => {
                  if (e.target.value) setSelectedMonth(e.target.value)
                }}
                className="bg-transparent text-sm font-semibold text-white focus:outline-none cursor-pointer"
              />
            </div>

            <button
              onClick={handleNextMonth}
              title="Next Month"
              className="p-2 text-slate-400 hover:text-white hover:bg-slate-800 rounded-xl transition"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>

          {selectedMonth !== currentMonthStr && (
            <button
              onClick={handleResetToCurrentMonth}
              className="text-xs font-medium text-emerald-400 hover:text-emerald-300 px-3 py-2 rounded-xl bg-emerald-500/10 border border-emerald-500/20 transition"
            >
              Current Month
            </button>
          )}

          {/* CSV Import Button */}
          <button
            onClick={() => setIsImportModalOpen(true)}
            className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-slate-800/80 hover:bg-slate-800 text-slate-200 hover:text-white border border-slate-700 text-xs font-semibold shadow-sm transition"
          >
            <UploadCloud className="w-4 h-4 text-emerald-400" />
            <span>Import CSV</span>
          </button>

          {/* Add Transaction Button */}
          <Link
            to="/transactions"
            className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-emerald-600 hover:bg-emerald-500 active:bg-emerald-700 text-white text-xs font-semibold shadow-lg shadow-emerald-950/50 transition"
          >
            <PlusCircle className="w-4 h-4" />
            <span>New Transaction</span>
          </Link>
        </div>
      </div>

      {/* Selected Month Status Indicator */}
      <div className="flex items-center justify-between text-xs text-slate-400 bg-slate-900/40 border border-slate-800/80 px-4 py-2.5 rounded-xl">
        <span className="flex items-center gap-2">
          <span className="w-2 h-2 rounded-full bg-emerald-400" />
          Showing metrics for: <strong className="text-white">{monthDisplayTitle}</strong>
        </span>
        <span className="hidden sm:inline text-slate-500">
          All calculations performed in exact VND integer values
        </span>
      </div>

      {/* Metric Cards (Total Income, Total Expenses, Net Savings, Budget Usage) */}
      <MetricCards summary={summary} isLoading={isLoadingSummary} />

      {/* Visual Charts Grid (6-Month Trend & Categorical Expense Share) */}
      <div id="analytics" className="grid grid-cols-1 lg:grid-cols-2 gap-6 scroll-mt-20">
        {/* 6-Month Income vs Expense Trend */}
        <TrendChart data={trendData} isLoading={isLoadingTrend} />

        {/* Category Expense Distribution Donut */}
        <CategoryDistributionChart
          data={categoriesShare}
          isLoading={isLoadingCategoriesShare}
        />
      </div>

      {/* Monthly Budget Management Panel */}
      <div id="budgets" className="scroll-mt-20">
        <BudgetPanel year={currentYear} month={currentMonth} />
      </div>

      {/* Quick Navigation & Capabilities Bar */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pt-4 border-t border-slate-800/60">
        <Link
          to="/transactions"
          className="p-4 rounded-xl bg-slate-900/40 hover:bg-slate-900/80 border border-slate-800 hover:border-slate-700 transition flex items-center justify-between group"
        >
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-lg bg-teal-500/10 border border-teal-500/20 flex items-center justify-center text-teal-400">
              <ReceiptText className="w-4 h-4" />
            </div>
            <div>
              <h4 className="text-sm font-semibold text-white group-hover:text-emerald-400 transition-colors">
                Transaction History
              </h4>
              <p className="text-xs text-slate-400">Filter, search & inspect records</p>
            </div>
          </div>
          <ChevronRight className="w-4 h-4 text-slate-500 group-hover:translate-x-0.5 transition-transform" />
        </Link>

        <Link
          to="/categories"
          className="p-4 rounded-xl bg-slate-900/40 hover:bg-slate-900/80 border border-slate-800 hover:border-slate-700 transition flex items-center justify-between group"
        >
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
              <Tags className="w-4 h-4" />
            </div>
            <div>
              <h4 className="text-sm font-semibold text-white group-hover:text-emerald-400 transition-colors">
                Custom Categories
              </h4>
              <p className="text-xs text-slate-400">Manage income & expense tags</p>
            </div>
          </div>
          <ChevronRight className="w-4 h-4 text-slate-500 group-hover:translate-x-0.5 transition-transform" />
        </Link>

        <div className="p-4 rounded-xl bg-slate-900/40 border border-slate-800 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-lg bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400">
              <ShieldCheck className="w-4 h-4" />
            </div>
            <div>
              <h4 className="text-sm font-semibold text-white">Security & Precision</h4>
              <p className="text-xs text-slate-400">Worker pool & int64 exact math</p>
            </div>
          </div>
          <CheckCircle2 className="w-4 h-4 text-emerald-400" />
        </div>
      </div>

      {/* CSV Import Modal */}
      <CsvImportModal
        isOpen={isImportModalOpen}
        onClose={() => setIsImportModalOpen(false)}
      />
    </div>
  )
}
