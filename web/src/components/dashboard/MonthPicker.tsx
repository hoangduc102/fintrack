import React, { useState, useRef, useEffect } from 'react'
import { Calendar, ChevronLeft, ChevronRight, ChevronDown } from 'lucide-react'

interface MonthPickerProps {
  value: string // 'YYYY-MM'
  onChange: (monthStr: string) => void
  onPrevMonth: () => void
  onNextMonth: () => void
}

const MONTH_NAMES = [
  'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
  'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'
]

export const MonthPicker: React.FC<MonthPickerProps> = ({
  value,
  onChange,
  onPrevMonth,
  onNextMonth,
}) => {
  const [isOpen, setIsOpen] = useState(false)
  const [navYearOverride, setNavYearOverride] = useState<number | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  // Parse selected YYYY-MM
  const [selectedYearStr, selectedMonthStr] = value.split('-')
  const selectedYear = parseInt(selectedYearStr, 10) || new Date().getFullYear()
  const selectedMonthIndex = (parseInt(selectedMonthStr, 10) || 1) - 1

  // Popover calendar navigation year (derived, reset when popover closes)
  const viewYear = navYearOverride ?? selectedYear

  // Current system date for "This Month"
  const now = new Date()
  const currentYear = now.getFullYear()
  const currentMonthIndex = now.getMonth()

  // Display label e.g. "September 2026"
  const displayDate = new Date(selectedYear, selectedMonthIndex, 1)
  const displayLabel = displayDate.toLocaleDateString('en-US', {
    month: 'long',
    year: 'numeric',
  })

  // Close popover on click outside or Escape
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setIsOpen(false)
        setNavYearOverride(null)
      }
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsOpen(false)
        setNavYearOverride(null)
      }
    }

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside)
      document.addEventListener('keydown', handleKeyDown)
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [isOpen])

  const handleToggle = () => {
    setIsOpen((prev) => {
      if (prev) {
        setNavYearOverride(null)
      }
      return !prev
    })
  }

  const handleSelectMonth = (monthIndex: number) => {
    const formattedMonth = String(monthIndex + 1).padStart(2, '0')
    onChange(`${viewYear}-${formattedMonth}`)
    setNavYearOverride(null)
    setIsOpen(false)
  }

  const handleSelectToday = () => {
    const formattedMonth = String(currentMonthIndex + 1).padStart(2, '0')
    onChange(`${currentYear}-${formattedMonth}`)
    setNavYearOverride(null)
    setIsOpen(false)
  }

  return (
    <div className="relative inline-flex items-center" ref={containerRef}>
      {/* Navigator Bar */}
      <div className="flex items-center bg-white border border-[#E5EAF1] rounded-2xl p-1 shadow-xs transition-colors">
        <button
          type="button"
          onClick={onPrevMonth}
          title="Previous Month"
          aria-label="Previous Month"
          className="p-2 text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC] rounded-xl transition cursor-pointer"
        >
          <ChevronLeft className="w-4 h-4" />
        </button>

        {/* Center Trigger: Stable width prevents header layout shifting */}
        <button
          type="button"
          onClick={handleToggle}
          aria-expanded={isOpen}
          aria-haspopup="dialog"
          title="Open Month Picker"
          className="flex items-center justify-center gap-2 px-3 py-1.5 text-xs sm:text-sm font-semibold text-[#243044] hover:bg-[#F7F9FC] rounded-xl transition cursor-pointer min-w-[170px] sm:min-w-[195px] whitespace-nowrap"
        >
          <Calendar className="w-4 h-4 text-[#5BC8A4] shrink-0" />
          <span className="truncate">{displayLabel}</span>
          <ChevronDown
            className={`w-3.5 h-3.5 text-[#718096] transition-transform duration-200 shrink-0 ${
              isOpen ? 'rotate-180 text-[#2F8F78]' : ''
            }`}
          />
        </button>

        <button
          type="button"
          onClick={onNextMonth}
          title="Next Month"
          aria-label="Next Month"
          className="p-2 text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC] rounded-xl transition cursor-pointer"
        >
          <ChevronRight className="w-4 h-4" />
        </button>
      </div>

      {/* Styled Popover */}
      {isOpen && (
        <div
          role="dialog"
          aria-label="Select Month and Year"
          className="absolute left-0 top-full mt-2 z-50 bg-white border border-[#E5EAF1] rounded-2xl p-4 shadow-xl w-72 sm:w-80 animate-in fade-in zoom-in-95 duration-150"
        >
          {/* Year Navigator */}
          <div className="flex items-center justify-between pb-3 border-b border-[#E5EAF1] mb-3">
            <button
              type="button"
              onClick={() => setNavYearOverride(viewYear - 1)}
              title="Previous Year"
              aria-label="Previous Year"
              className="p-1.5 text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC] rounded-lg transition cursor-pointer"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <span className="text-sm font-bold text-[#243044] tracking-tight">{viewYear}</span>
            <button
              type="button"
              onClick={() => setNavYearOverride(viewYear + 1)}
              title="Next Year"
              aria-label="Next Year"
              className="p-1.5 text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC] rounded-lg transition cursor-pointer"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>

          {/* 12-Month Grid */}
          <div className="grid grid-cols-4 gap-2 mb-3">
            {MONTH_NAMES.map((mName, mIndex) => {
              const isSelected = viewYear === selectedYear && mIndex === selectedMonthIndex
              const isCurrent = viewYear === currentYear && mIndex === currentMonthIndex

              return (
                <button
                  key={mName}
                  type="button"
                  onClick={() => handleSelectMonth(mIndex)}
                  className={`py-2 px-1 text-xs font-semibold rounded-xl transition cursor-pointer text-center ${
                    isSelected
                      ? 'bg-[#5BC8A4] text-white shadow-sm'
                      : isCurrent
                      ? 'bg-[#5BC8A4]/15 text-[#2F8F78] font-bold border border-[#5BC8A4]/30 hover:bg-[#5BC8A4]/25'
                      : 'text-[#243044] hover:bg-[#F7F9FC] hover:text-[#2F8F78]'
                  }`}
                >
                  {mName}
                </button>
              )
            })}
          </div>

          {/* Quick Action Footer */}
          <div className="pt-2 border-t border-[#E5EAF1] flex items-center justify-between">
            <button
              type="button"
              onClick={handleSelectToday}
              className="text-xs font-semibold text-[#2F8F78] hover:text-[#5BC8A4] px-2.5 py-1 rounded-lg hover:bg-emerald-50/60 transition cursor-pointer"
            >
              This Month
            </button>
            <button
              type="button"
              onClick={() => {
                setIsOpen(false)
                setNavYearOverride(null)
              }}
              className="text-xs font-medium text-[#718096] hover:text-[#243044] px-2.5 py-1 rounded-lg hover:bg-slate-100 transition cursor-pointer"
            >
              Close
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
