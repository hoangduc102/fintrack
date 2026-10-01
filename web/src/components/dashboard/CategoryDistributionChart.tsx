import React from 'react'
import { 
  ResponsiveContainer, 
  PieChart, 
  Pie, 
  Cell, 
  Tooltip, 
  Legend 
} from 'recharts'
import type { CategoryExpenseShare } from '../../api/types'
import { formatVND } from '../../utils/format'
import { PieChart as PieIcon, Loader2 } from 'lucide-react'

interface CategoryDistributionChartProps {
  data: CategoryExpenseShare[]
  isLoading: boolean
}

const COLORS = [
  '#10b981', // emerald
  '#06b6d4', // cyan
  '#3b82f6', // blue
  '#8b5cf6', // violet
  '#f59e0b', // amber
  '#ec4899', // pink
  '#f97316', // orange
  '#14b8a6', // teal
  '#64748b', // slate
]

export const CategoryDistributionChart: React.FC<CategoryDistributionChartProps> = ({ data, isLoading }) => {
  if (isLoading) {
    return (
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800 p-6 h-96 flex flex-col items-center justify-center text-slate-400">
        <Loader2 className="w-8 h-8 text-emerald-400 animate-spin mb-3" />
        <p className="text-sm">Loading expense distribution...</p>
      </div>
    )
  }

  const chartData = data.map((item) => ({
    name: item.category_name,
    value: item.amount,
    percentage: item.percentage,
  }))

  const hasData = chartData.length > 0 && chartData.some((d) => d.value > 0)

  return (
    <div className="rounded-2xl bg-slate-900/60 border border-slate-800 p-6 flex flex-col justify-between">
      <div className="flex items-center justify-between pb-4 border-b border-slate-800 mb-4">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-lg bg-teal-500/10 border border-teal-500/20 flex items-center justify-center text-teal-400">
            <PieIcon className="w-4 h-4" />
          </div>
          <div>
            <h3 className="text-base font-bold text-white leading-tight">Expense Distribution</h3>
            <span className="text-xs text-slate-400">Percentage share by category</span>
          </div>
        </div>
      </div>

      {!hasData ? (
        <div className="h-72 flex flex-col items-center justify-center text-slate-400">
          <PieIcon className="w-10 h-10 mb-2 opacity-30 text-slate-400" />
          <p className="text-sm font-medium">No expenses recorded for this month</p>
          <span className="text-xs text-slate-400 mt-1">Expenses logged will show category shares here</span>
        </div>
      ) : (
        <div className="h-72 w-full">
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={chartData}
                cx="50%"
                cy="50%"
                innerRadius={60}
                outerRadius={85}
                paddingAngle={4}
                dataKey="value"
              >
                {chartData.map((_, index) => (
                  <Cell 
                    key={`cell-${index}`} 
                    fill={COLORS[index % COLORS.length]} 
                    stroke="rgba(15, 23, 42, 0.8)"
                    strokeWidth={2}
                  />
                ))}
              </Pie>
              <Tooltip
                content={({ active, payload }) => {
                  if (active && payload && payload.length) {
                    const item = payload[0].payload as { name: string; value: number; percentage: number }
                    return (
                      <div className="rounded-xl bg-slate-950/95 border border-slate-700/80 p-3 shadow-xl backdrop-blur-md text-xs">
                        <div className="font-semibold text-white mb-1">{item.name}</div>
                        <div className="text-emerald-400 font-mono font-bold text-sm">
                          {formatVND(item.value)}
                        </div>
                        <div className="text-slate-400 mt-0.5">{item.percentage}% of total expenses</div>
                      </div>
                    )
                  }
                  return null
                }}
              />
              <Legend
                formatter={(value) => <span className="text-xs text-slate-300 mr-2">{value}</span>}
                iconType="circle"
                iconSize={8}
              />
            </PieChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  )
}
