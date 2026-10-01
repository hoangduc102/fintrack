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
          <div key={i} className="h-32 rounded-2xl bg-slate-900/50 border border-slate-800 p-5 flex items-center justify-center">
            <Loader2 className="w-5 h-5 text-emerald-400 animate-spin" />
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
      <div className="relative overflow-hidden rounded-2xl bg-slate-900/60 border border-slate-800 p-5 shadow-lg group hover:border-emerald-500/40 transition-all">
        <div className="absolute top-0 right-0 w-24 h-24 bg-emerald-500/5 rounded-full blur-xl pointer-events-none" />
        <div className="flex items-center justify-between mb-3">
          <span className="text-xs font-semibold text-slate-400 uppercase tracking-wider">Total Income</span>
          <div className="w-8 h-8 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
            <ArrowUpRight className="w-4 h-4" />
          </div>
        </div>
        <div className="text-2xl font-bold text-white font-mono tracking-tight">
          {formatVND(income)}
        </div>
        <div className="mt-2 text-xs text-slate-400 flex items-center gap-1.5">
          <span className="text-emerald-400 font-medium">Inflows</span>
          <span>•</span>
          <span>This month</span>
        </div>
      </div>

      {/* 2. Total Expense */}
      <div className="relative overflow-hidden rounded-2xl bg-slate-900/60 border border-slate-800 p-5 shadow-lg group hover:border-rose-500/40 transition-all">
        <div className="absolute top-0 right-0 w-24 h-24 bg-rose-500/5 rounded-full blur-xl pointer-events-none" />
        <div className="flex items-center justify-between mb-3">
          <span className="text-xs font-semibold text-slate-400 uppercase tracking-wider">Total Expenses</span>
          <div className="w-8 h-8 rounded-xl bg-rose-500/10 border border-rose-500/20 flex items-center justify-center text-rose-400">
            <ArrowDownLeft className="w-4 h-4" />
          </div>
        </div>
        <div className="text-2xl font-bold text-white font-mono tracking-tight">
          {formatVND(expense)}
        </div>
        <div className="mt-2 text-xs text-slate-400 flex items-center gap-1.5">
          <span className="text-rose-400 font-medium">Outflows</span>
          <span>•</span>
          <span>This month</span>
        </div>
      </div>

      {/* 3. Net Savings */}
      <div className="relative overflow-hidden rounded-2xl bg-slate-900/60 border border-slate-800 p-5 shadow-lg group hover:border-teal-500/40 transition-all">
        <div className="absolute top-0 right-0 w-24 h-24 bg-teal-500/5 rounded-full blur-xl pointer-events-none" />
        <div className="flex items-center justify-between mb-3">
          <span className="text-xs font-semibold text-slate-400 uppercase tracking-wider">Net Savings</span>
          <div className={`w-8 h-8 rounded-xl flex items-center justify-center ${
            isSavingsPositive 
              ? 'bg-teal-500/10 border border-teal-500/20 text-teal-400' 
              : 'bg-rose-500/10 border border-rose-500/20 text-rose-400'
          }`}>
            {isSavingsPositive ? <TrendingUp className="w-4 h-4" /> : <TrendingDown className="w-4 h-4" />}
          </div>
        </div>
        <div className={`text-2xl font-bold font-mono tracking-tight ${
          isSavingsPositive ? 'text-teal-300' : 'text-rose-400'
        }`}>
          {isSavingsPositive ? '+' : ''}{formatVND(netSavings)}
        </div>
        <div className="mt-2 text-xs text-slate-400 flex items-center gap-1.5">
          <span className={isSavingsPositive ? 'text-teal-400 font-medium' : 'text-rose-400 font-medium'}>
            {isSavingsPositive ? 'Surplus' : 'Deficit'}
          </span>
          <span>•</span>
          <span>Income - Expense</span>
        </div>
      </div>

      {/* 4. Budget Usage */}
      <div className="relative overflow-hidden rounded-2xl bg-slate-900/60 border border-slate-800 p-5 shadow-lg group hover:border-amber-500/40 transition-all">
        <div className="absolute top-0 right-0 w-24 h-24 bg-amber-500/5 rounded-full blur-xl pointer-events-none" />
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-semibold text-slate-400 uppercase tracking-wider">Budget Usage</span>
          <div className="w-8 h-8 rounded-xl bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-400">
            <PiggyBank className="w-4 h-4" />
          </div>
        </div>

        {totalBudget > 0 ? (
          <div>
            <div className="flex items-baseline justify-between mb-1.5">
              <span className="text-xl font-bold text-white font-mono">{budgetPct}%</span>
              <span className="text-xs text-slate-400">
                {formatVND(totalSpent)} / {formatVND(totalBudget)}
              </span>
            </div>
            {/* Progress bar */}
            <div className="w-full h-2 rounded-full bg-slate-800 overflow-hidden">
              <div 
                className={`h-full rounded-full transition-all duration-500 ${
                  budgetPct > 100 
                    ? 'bg-rose-500' 
                    : budgetPct >= 80 
                    ? 'bg-amber-400' 
                    : 'bg-emerald-400'
                }`}
                style={{ width: `${Math.min(budgetPct, 100)}%` }}
              />
            </div>
          </div>
        ) : (
          <div>
            <div className="text-sm font-semibold text-slate-300 mt-2">No budgets set</div>
            <p className="text-xs text-slate-400 mt-1">Set monthly category caps below</p>
          </div>
        )}
      </div>
    </div>
  )
}
