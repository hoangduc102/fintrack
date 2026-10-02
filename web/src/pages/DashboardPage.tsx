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
import { MonthPicker } from '../components/dashboard/MonthPicker'
import {
  UploadCloud,
  PlusCircle,
  Sparkles,
  ReceiptText,
  Tags,
  ShieldCheck,
  CheckCircle2,
  ChevronRight
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
    <div className="space-y-6 sm:space-y-8 animate-in fade-in duration-300 pb-12">
      {/* Welcome & Period Header */}
      <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4 pb-4 border-b border-[#E5EAF1]">
        <div>
          <div className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-emerald-50 border border-emerald-200/60 text-xs font-semibold text-[#2F8F78] mb-1.5 shadow-xs">
            <Sparkles className="w-3.5 h-3.5" />
            <span>Financial Overview</span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-[#243044] tracking-tight">
            Welcome back, <span className="text-[#2F8F78]">{displayName}</span>
          </h1>
          <p className="text-xs sm:text-sm text-[#718096] mt-0.5">
            Real-time monthly summaries, category allocations, budgets, and historical cash flow.
          </p>
        </div>

        {/* Action Controls & Month Picker Toolbar */}
        <div className="flex items-center gap-2 sm:gap-2.5 flex-wrap sm:flex-nowrap shrink-0">
          {/* Custom Styled Month/Year Selector */}
          <MonthPicker
            value={selectedMonth}
            onChange={setSelectedMonth}
            onPrevMonth={handlePrevMonth}
            onNextMonth={handleNextMonth}
          />

          {selectedMonth !== currentMonthStr && (
            <button
              type="button"
              onClick={handleResetToCurrentMonth}
              className="shrink-0 text-xs font-semibold text-[#2F8F78] hover:text-[#243044] px-3 py-2 rounded-xl bg-emerald-50/80 border border-emerald-200/60 hover:bg-emerald-100/60 transition shadow-xs cursor-pointer"
            >
              Current Month
            </button>
          )}

          {/* CSV Import Button */}
          <button
            type="button"
            onClick={() => setIsImportModalOpen(true)}
            className="shrink-0 inline-flex items-center gap-1.5 sm:gap-2 px-3 sm:px-3.5 py-2 rounded-xl bg-white hover:bg-slate-50 text-[#243044] border border-[#E5EAF1] text-xs font-semibold shadow-xs transition cursor-pointer"
          >
            <UploadCloud className="w-4 h-4 text-[#5BC8A4]" />
            <span>Import CSV</span>
          </button>

          {/* Add Transaction Button */}
          <Link
            to="/transactions"
            className="shrink-0 inline-flex items-center gap-1.5 sm:gap-2 px-3.5 sm:px-4 py-2 rounded-xl bg-[#5BC8A4] hover:bg-[#4db995] text-white text-xs font-semibold shadow-xs transition"
          >
            <PlusCircle className="w-4 h-4" />
            <span>New Transaction</span>
          </Link>
        </div>
      </div>

      {/* Selected Month Status Indicator */}
      <div className="flex items-center justify-between text-xs text-[#718096] bg-white border border-[#E5EAF1] px-4 py-2.5 rounded-xl shadow-xs">
        <span className="flex items-center gap-2">
          <span className="w-2 h-2 rounded-full bg-[#5BC8A4]" />
          Showing metrics for: <strong className="text-[#243044] font-semibold">{monthDisplayTitle}</strong>
        </span>
        <span className="hidden sm:inline text-[#718096]">
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
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pt-4 border-t border-[#E5EAF1]">
        <Link
          to="/transactions"
          className="p-4 rounded-2xl bg-white hover:bg-[#F7F9FC] border border-[#E5EAF1] transition flex items-center justify-between group shadow-xs"
        >
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-xl bg-[#5BC8A4]/15 border border-[#5BC8A4]/30 flex items-center justify-center text-[#2F8F78]">
              <ReceiptText className="w-4 h-4" />
            </div>
            <div>
              <h4 className="text-sm font-semibold text-[#243044] group-hover:text-[#2F8F78] transition-colors">
                Transaction History
              </h4>
              <p className="text-xs text-[#718096]">Filter, search & inspect records</p>
            </div>
          </div>
          <ChevronRight className="w-4 h-4 text-[#718096] group-hover:translate-x-0.5 transition-transform" />
        </Link>

        <Link
          to="/categories"
          className="p-4 rounded-2xl bg-white hover:bg-[#F7F9FC] border border-[#E5EAF1] transition flex items-center justify-between group shadow-xs"
        >
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-xl bg-[#8FB7E8]/20 border border-[#8FB7E8]/40 flex items-center justify-center text-[#3B82F6]">
              <Tags className="w-4 h-4" />
            </div>
            <div>
              <h4 className="text-sm font-semibold text-[#243044] group-hover:text-[#3B82F6] transition-colors">
                Custom Categories
              </h4>
              <p className="text-xs text-[#718096]">Manage income & expense tags</p>
            </div>
          </div>
          <ChevronRight className="w-4 h-4 text-[#718096] group-hover:translate-x-0.5 transition-transform" />
        </Link>

        <div className="p-4 rounded-2xl bg-white border border-[#E5EAF1] flex items-center justify-between shadow-xs">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-xl bg-[#B8A7E8]/20 border border-[#B8A7E8]/40 flex items-center justify-center text-[#7C3AED]">
              <ShieldCheck className="w-4 h-4" />
            </div>
            <div>
              <h4 className="text-sm font-semibold text-[#243044]">Security & Precision</h4>
              <p className="text-xs text-[#718096]">Worker pool & int64 exact math</p>
            </div>
          </div>
          <CheckCircle2 className="w-4 h-4 text-[#5BC8A4]" />
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
