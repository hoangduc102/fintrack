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

// Clean harmonious soft pastel palette
const COLORS = [
  '#5BC8A4', // mint
  '#8FB7E8', // soft blue
  '#B8A7E8', // soft lavender
  '#E9C36A', // soft gold/amber
  '#E98C8C', // soft coral/rose
  '#6EE7B7', // light mint
  '#93C5FD', // light blue
  '#FCA5A5', // light rose
  '#A5B4FC', // light indigo
  '#FDE68A', // light yellow
]

export const CategoryDistributionChart: React.FC<CategoryDistributionChartProps> = ({ data, isLoading }) => {
  if (isLoading) {
    return (
      <div className="rounded-2xl bg-white border border-[#E5EAF1] p-6 h-96 flex flex-col items-center justify-center text-[#718096] shadow-xs">
        <Loader2 className="w-8 h-8 text-[#5BC8A4] animate-spin mb-3" />
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
    <div className="rounded-2xl bg-white border border-[#E5EAF1] p-6 shadow-xs flex flex-col justify-between">
      <div className="flex items-center justify-between pb-4 border-b border-[#E5EAF1] mb-4">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-xl bg-teal-50 border border-teal-200/60 flex items-center justify-center text-[#2F8F78]">
            <PieIcon className="w-4 h-4" />
          </div>
          <div>
            <h3 className="text-base font-bold text-[#243044] leading-tight">Expense Distribution</h3>
            <span className="text-xs text-[#718096]">Percentage share by category</span>
          </div>
        </div>
      </div>

      {!hasData ? (
        <div className="h-72 flex flex-col items-center justify-center text-[#718096]">
          <PieIcon className="w-10 h-10 mb-2 opacity-30 text-[#718096]" />
          <p className="text-sm font-medium text-[#243044]">No expenses recorded for this month</p>
          <span className="text-xs text-[#718096] mt-1">Expenses logged will show category shares here</span>
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
                paddingAngle={3}
                dataKey="value"
              >
                {chartData.map((_, index) => (
                  <Cell 
                    key={`cell-${index}`} 
                    fill={COLORS[index % COLORS.length]} 
                    stroke="#FFFFFF"
                    strokeWidth={2}
                  />
                ))}
              </Pie>
              <Tooltip
                content={({ active, payload }) => {
                  if (active && payload && payload.length) {
                    const item = payload[0].payload as { name: string; value: number; percentage: number }
                    return (
                      <div className="rounded-xl bg-white border border-[#E5EAF1] p-3 shadow-lg text-xs">
                        <div className="font-semibold text-[#243044] mb-1">{item.name}</div>
                        <div className="text-[#2F8F78] font-mono font-bold text-sm">
                          {formatVND(item.value)}
                        </div>
                        <div className="text-[#718096] mt-0.5">{item.percentage}% of total expenses</div>
                      </div>
                    )
                  }
                  return null
                }}
              />
              <Legend
                formatter={(value) => <span className="text-xs text-[#718096] mr-2">{value}</span>}
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
