import React from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import { 
  Wallet, 
  LogOut, 
  LayoutDashboard,
  Tags,
  ReceiptText,
  PiggyBank,
  BarChart3
} from 'lucide-react'

export const Navbar: React.FC = () => {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const displayName = user?.display_name || user?.email?.split('@')[0] || 'User'
  const initials = displayName.slice(0, 2).toUpperCase()

  const isActive = (path: string) => {
    if (path === '/') return location.pathname === '/'
    return location.pathname.startsWith(path)
  }

  return (
    <header className="sticky top-0 z-50 backdrop-blur-md bg-white/90 border-b border-[#E5EAF1] shadow-xs">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          {/* Brand Logo */}
          <div className="flex items-center gap-8">
            <Link to="/" className="flex items-center gap-3 group">
              <div className="w-10 h-10 rounded-xl bg-[#5BC8A4]/15 border border-[#5BC8A4]/30 flex items-center justify-center text-[#2F8F78] shadow-xs group-hover:bg-[#5BC8A4]/25 transition-all">
                <Wallet className="w-5 h-5 text-[#2F8F78] group-hover:scale-105 transition-transform" />
              </div>
              <div>
                <span className="text-xl font-bold tracking-tight text-[#243044]">
                  Fin<span className="text-[#2F8F78]">Track</span>
                </span>
                <span className="block text-[10px] uppercase font-semibold tracking-wider text-[#718096]">Finance Manager</span>
              </div>
            </Link>

            {/* Navigation Links */}
            <nav className="hidden md:flex items-center gap-1.5">
              <Link
                to="/"
                className={`flex items-center gap-2 px-3 py-2 rounded-xl text-sm font-medium transition-colors ${
                  isActive('/')
                    ? 'bg-emerald-50 text-[#2F8F78] border border-emerald-200/60 font-semibold shadow-xs'
                    : 'text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC]'
                }`}
              >
                <LayoutDashboard className="w-4 h-4 text-[#2F8F78]" />
                Dashboard
              </Link>
              <Link
                to="/categories"
                className={`flex items-center gap-2 px-3 py-2 rounded-xl text-sm font-medium transition-colors ${
                  isActive('/categories')
                    ? 'bg-emerald-50 text-[#2F8F78] border border-emerald-200/60 font-semibold shadow-xs'
                    : 'text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC]'
                }`}
              >
                <Tags className="w-4 h-4 text-[#2F8F78]" />
                Categories
              </Link>
              <Link
                to="/transactions"
                className={`flex items-center gap-2 px-3 py-2 rounded-xl text-sm font-medium transition-colors ${
                  isActive('/transactions')
                    ? 'bg-emerald-50 text-[#2F8F78] border border-emerald-200/60 font-semibold shadow-xs'
                    : 'text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC]'
                }`}
              >
                <ReceiptText className="w-4 h-4 text-[#2F8F78]" />
                Transactions
              </Link>
              <a
                href="/#budgets"
                onClick={(e) => {
                  if (location.pathname === '/') {
                    e.preventDefault()
                    document.getElementById('budgets')?.scrollIntoView({ behavior: 'smooth' })
                  }
                }}
                className="flex items-center gap-2 px-3 py-2 rounded-xl text-sm font-medium text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC] transition-colors"
              >
                <PiggyBank className="w-4 h-4 text-[#2F8F78]" />
                Budgets
              </a>
              <a
                href="/#analytics"
                onClick={(e) => {
                  if (location.pathname === '/') {
                    e.preventDefault()
                    document.getElementById('analytics')?.scrollIntoView({ behavior: 'smooth' })
                  }
                }}
                className="flex items-center gap-2 px-3 py-2 rounded-xl text-sm font-medium text-[#718096] hover:text-[#243044] hover:bg-[#F7F9FC] transition-colors"
              >
                <BarChart3 className="w-4 h-4 text-[#2F8F78]" />
                Analytics
              </a>
            </nav>
          </div>

          {/* User profile & Logout */}
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2.5 pl-3 border-l border-[#E5EAF1]">
              <div className="w-8 h-8 rounded-full bg-[#5BC8A4]/15 border border-[#5BC8A4]/30 flex items-center justify-center text-xs font-bold text-[#2F8F78]">
                {initials}
              </div>
              <div className="hidden sm:block text-left">
                <div className="text-xs font-semibold text-[#243044] leading-tight">
                  {displayName}
                </div>
                <div className="text-[11px] text-[#718096] leading-tight truncate max-w-[160px]">
                  {user?.email}
                </div>
              </div>
            </div>

            <button
              onClick={handleLogout}
              title="Sign out of your account"
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-medium text-[#718096] hover:text-rose-600 hover:bg-rose-50 border border-transparent hover:border-rose-200 transition-all cursor-pointer"
            >
              <LogOut className="w-4 h-4" />
              <span className="hidden sm:inline">Logout</span>
            </button>
          </div>
        </div>
      </div>
    </header>
  )
}
