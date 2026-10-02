import React from 'react'
import {
  ResponsiveContainer,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
} from 'recharts'
import type { MonthlyTrendPoint } from '../../api/types'
import { formatVND } from '../../utils/format'
import { TrendingUp, Loader2 } from 'lucide-react'

interface TrendChartProps {
  data: MonthlyTrendPoint[]
  isLoading: boolean
}

interface CustomTooltipProps {
  active?: boolean
  payload?: Array<{
    name: string
    value: number
    color: string
    dataKey: string
  }>
  label?: string
}

const CustomTooltip: React.FC<CustomTooltipProps> = ({ active, payload, label }) => {
  if (active && payload && payload.length) {
    const income = payload.find((p) => p.dataKey === 'income')?.value ?? 0
    const expense = payload.find((p) => p.dataKey === 'expense')?.value ?? 0
    const net = income - expense

    return (
      <div className="bg-white border border-[#E5EAF1] p-3.5 rounded-xl shadow-lg min-w-[200px]">
        <p className="text-xs font-semibold text-[#718096] mb-2 border-b border-[#E5EAF1] pb-1.5">
          Month: {label}
        </p>
        <div className="space-y-1.5 text-xs">
          <div className="flex items-center justify-between text-[#2F8F78]">
            <span className="flex items-center gap-1.5">
              <span className="w-2 h-2 rounded-full bg-[#5BBF9A]" />
              Income:
            </span>
            <span className="font-semibold">{formatVND(income)}</span>
          </div>
          <div className="flex items-center justify-between text-[#E98C8C]">
            <span className="flex items-center gap-1.5">
              <span className="w-2 h-2 rounded-full bg-[#E98C8C]" />
              Expense:
            </span>
            <span className="font-semibold">{formatVND(expense)}</span>
          </div>
          <div className="flex items-center justify-between pt-1.5 border-t border-[#E5EAF1] text-[#243044]">
            <span className="font-medium text-[#718096]">Net Savings:</span>
            <span
              className={`font-bold ${
                net > 0 ? 'text-[#2F8F78]' : net < 0 ? 'text-[#E98C8C]' : 'text-[#718096]'
              }`}
            >
              {formatVND(net)}
            </span>
          </div>
        </div>
      </div>
    )
  }
  return null
}

export const TrendChart: React.FC<TrendChartProps> = ({ data, isLoading }) => {
  if (isLoading) {
    return (
      <div className="rounded-2xl bg-white border border-[#E5EAF1] p-6 h-96 flex flex-col items-center justify-center text-[#718096] shadow-xs">
        <Loader2 className="w-8 h-8 text-[#5BC8A4] animate-spin mb-3" />
        <p className="text-sm">Loading 6-month historical trend...</p>
      </div>
    )
  }

  // Format month labels for readability e.g. "2026-03" -> "03/2026"
  const formattedData = data.map((item) => {
    const parts = item.month.split('-')
    const displayMonth = parts.length === 2 ? `${parts[1]}/${parts[0]}` : item.month
    return {
      ...item,
      displayMonth,
    }
  })

  const hasData = formattedData.some((d) => d.income > 0 || d.expense > 0)

  // Format VND in millions for compact Y axis
  const formatYAxis = (val: number) => {
    if (val === 0) return '0'
    if (Math.abs(val) >= 1_000_000) {
      return `${(val / 1_000_000).toFixed(0)}M`
    }
    if (Math.abs(val) >= 1_000) {
      return `${(val / 1_000).toFixed(0)}K`
    }
    return `${val}`
  }

  return (
    <div className="rounded-2xl bg-white border border-[#E5EAF1] p-6 shadow-xs flex flex-col justify-between">
      <div className="flex items-center justify-between pb-4 border-b border-[#E5EAF1] mb-4">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-xl bg-emerald-50 border border-emerald-200/60 flex items-center justify-center text-[#2F8F78]">
            <TrendingUp className="w-4 h-4" />
          </div>
          <div>
            <h3 className="text-base font-bold text-[#243044] leading-tight">6-Month Financial Trend</h3>
            <span className="text-xs text-[#718096]">Monthly income vs. expense comparison</span>
          </div>
        </div>
      </div>

      {!hasData ? (
        <div className="h-72 flex flex-col items-center justify-center text-[#718096]">
          <p className="text-sm">No transaction history in the past 6 months</p>
        </div>
      ) : (
        <div className="h-72 w-full">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart
              data={formattedData}
              margin={{ top: 10, right: 10, left: 10, bottom: 5 }}
            >
              <CartesianGrid strokeDasharray="3 3" stroke="#E5EAF1" vertical={false} />
              <XAxis
                dataKey="displayMonth"
                stroke="#718096"
                tick={{ fill: '#718096', fontSize: 12 }}
                axisLine={{ stroke: '#E5EAF1' }}
              />
              <YAxis
                stroke="#718096"
                tick={{ fill: '#718096', fontSize: 12 }}
                tickFormatter={formatYAxis}
                axisLine={{ stroke: '#E5EAF1' }}
              />
              <Tooltip content={<CustomTooltip />} />
              <Legend
                wrapperStyle={{ paddingTop: '12px' }}
                formatter={(value) => (
                  <span className="text-xs font-medium text-[#718096] capitalize">
                    {value === 'income' ? 'Income' : 'Expense'}
                  </span>
                )}
              />
              <Bar dataKey="income" fill="#5BBF9A" radius={[4, 4, 0, 0]} maxBarSize={36} />
              <Bar dataKey="expense" fill="#E98C8C" radius={[4, 4, 0, 0]} maxBarSize={36} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  )
}
