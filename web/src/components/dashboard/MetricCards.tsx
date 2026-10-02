import React from 'react'
import type { MonthlySummary } from '../../api/types'
import { formatVND } from '../../utils/format'
import { 
  ArrowUpRight, 
  ArrowDownLeft, 
  PiggyBank, 
  Loader2,
  TrendingUp,
  TrendingDown
} from 'lucide-react'

interface MetricCardsProps {
  summary: MonthlySummary | undefined
  isLoading: boolean
}

export const MetricCards: React.FC<MetricCardsProps> = ({ summary, isLoading }) => {
  if (isLoading) {
    return (
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {[1, 2, 3, 4].map((i) => (
          <div key={i} className="h-32 rounded-2xl bg-white border border-[#E5EAF1] p-5 flex items-center justify-center shadow-xs">
            <Loader2 className="w-5 h-5 text-[#5BC8A4] animate-spin" />
          </div>
        ))}
      </div>
    )
  }

  const income = summary?.total_income || 0
  const expense = summary?.total_expense || 0
  const netSavings = summary?.net_savings || 0
  const budget = summary?.budget_status
  const totalBudget = budget?.total_budget || 0
  const totalSpent = budget?.total_spent || 0
  const budgetPct = budget?.percentage || 0

  const isSavingsPositive = netSavings >= 0

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      {/* 1. Total Income */}
      <div className="rounded-2xl bg-white border border-[#E5EAF1] p-5 shadow-xs hover:border-emerald-300 transition-all">
        <div className="flex items-center justify-between mb-3">
          <span className="text-xs font-semibold text-[#718096] uppercase tracking-wider">Total Income</span>
          <div className="w-8 h-8 rounded-xl bg-emerald-50 border border-emerald-200/60 flex items-center justify-center text-[#2F8F78]">
            <ArrowUpRight className="w-4 h-4" />
          </div>
        </div>
        <div className="text-2xl font-bold text-[#243044] font-mono tracking-tight">
          {formatVND(income)}
        </div>
        <div className="mt-2 text-xs text-[#718096] flex items-center gap-1.5">
          <span className="text-[#2F8F78] font-medium">Inflows</span>
          <span>•</span>
          <span>This month</span>
        </div>
      </div>

      {/* 2. Total Expense */}
      <div className="rounded-2xl bg-white border border-[#E5EAF1] p-5 shadow-xs hover:border-rose-300 transition-all">
        <div className="flex items-center justify-between mb-3">
          <span className="text-xs font-semibold text-[#718096] uppercase tracking-wider">Total Expenses</span>
          <div className="w-8 h-8 rounded-xl bg-rose-50 border border-rose-200/60 flex items-center justify-center text-[#E98C8C]">
            <ArrowDownLeft className="w-4 h-4" />
          </div>
        </div>
        <div className="text-2xl font-bold text-[#243044] font-mono tracking-tight">
          {formatVND(expense)}
        </div>
        <div className="mt-2 text-xs text-[#718096] flex items-center gap-1.5">
          <span className="text-[#E98C8C] font-medium">Outflows</span>
          <span>•</span>
          <span>This month</span>
        </div>
      </div>

      {/* 3. Net Savings */}
      <div className="rounded-2xl bg-white border border-[#E5EAF1] p-5 shadow-xs hover:border-teal-300 transition-all">
        <div className="flex items-center justify-between mb-3">
          <span className="text-xs font-semibold text-[#718096] uppercase tracking-wider">Net Savings</span>
          <div className={`w-8 h-8 rounded-xl flex items-center justify-center ${
            isSavingsPositive 
              ? 'bg-emerald-50 border border-emerald-200/60 text-[#2F8F78]' 
              : 'bg-rose-50 border border-rose-200/60 text-[#E98C8C]'
          }`}>
            {isSavingsPositive ? <TrendingUp className="w-4 h-4" /> : <TrendingDown className="w-4 h-4" />}
          </div>
        </div>
        <div className="text-2xl font-bold font-mono tracking-tight text-[#243044]">
          {isSavingsPositive ? '+' : ''}{formatVND(netSavings)}
        </div>
        <div className="mt-2 text-xs text-[#718096] flex items-center gap-1.5">
          <span className={isSavingsPositive ? 'text-[#2F8F78] font-medium' : 'text-[#E98C8C] font-medium'}>
            {isSavingsPositive ? 'Surplus' : 'Deficit'}
          </span>
          <span>•</span>
          <span>Income - Expense</span>
        </div>
      </div>

      {/* 4. Budget Usage */}
      <div className="rounded-2xl bg-white border border-[#E5EAF1] p-5 shadow-xs hover:border-amber-300 transition-all">
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-semibold text-[#718096] uppercase tracking-wider">Budget Usage</span>
          <div className="w-8 h-8 rounded-xl bg-amber-50 border border-amber-200/60 flex items-center justify-center text-[#D97706]">
            <PiggyBank className="w-4 h-4" />
          </div>
        </div>

        {totalBudget > 0 ? (
          <div>
            <div className="flex items-baseline justify-between mb-1.5">
              <span className="text-xl font-bold text-[#243044] font-mono">{budgetPct}%</span>
              <span className="text-xs text-[#718096]">
                {formatVND(totalSpent)} / {formatVND(totalBudget)}
              </span>
            </div>
            {/* Progress bar */}
            <div className="w-full h-2 rounded-full bg-[#E5EAF1] overflow-hidden">
              <div 
                className={`h-full rounded-full transition-all duration-500 ${
                  budgetPct > 100 
                    ? 'bg-[#E98C8C]' 
                    : budgetPct >= 80 
                    ? 'bg-[#E9C36A]' 
                    : 'bg-[#5BC8A4]'
                }`}
                style={{ width: `${Math.min(budgetPct, 100)}%` }}
              />
            </div>
          </div>
        ) : (
          <div>
            <div className="text-sm font-semibold text-[#243044] mt-2">No budgets set</div>
            <p className="text-xs text-[#718096] mt-0.5">Set monthly category caps below</p>
          </div>
        )}
      </div>
    </div>
  )
}
