import React from 'react'
import { Link, useNavigate } from 'react-router-dom'
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

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const displayName = user?.display_name || user?.email?.split('@')[0] || 'User'
  const initials = displayName.slice(0, 2).toUpperCase()

  return (
    <header className="sticky top-0 z-50 backdrop-blur-xl bg-slate-950/80 border-b border-slate-800/80">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          {/* Brand Logo */}
          <div className="flex items-center gap-8">
            <Link to="/" className="flex items-center gap-3 group">
              <div className="w-10 h-10 rounded-xl bg-gradient-to-tr from-emerald-500 to-teal-400 p-0.5 shadow-lg shadow-emerald-500/20 group-hover:shadow-emerald-500/40 transition-all duration-300">
                <div className="w-full h-full bg-slate-950 rounded-[10px] flex items-center justify-center">
                  <Wallet className="w-5 h-5 text-emerald-400 group-hover:scale-110 transition-transform duration-300" />
                </div>
              </div>
              <div>
                <span className="text-xl font-bold bg-gradient-to-r from-white via-slate-100 to-slate-400 bg-clip-text text-transparent">
                  Fin<span className="text-emerald-400">Track</span>
                </span>
                <span className="block text-[10px] uppercase font-semibold tracking-wider text-slate-500">Finance Manager</span>
              </div>
            </Link>

            {/* Navigation Links */}
            <nav className="hidden md:flex items-center gap-1">
              <Link
                to="/"
                className="flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium text-slate-300 hover:text-white hover:bg-slate-900 transition-colors"
              >
                <LayoutDashboard className="w-4 h-4 text-emerald-400" />
                Dashboard
              </Link>
              <span className="flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium text-slate-500 cursor-not-allowed">
                <Tags className="w-4 h-4" />
                Categories
              </span>
              <span className="flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium text-slate-500 cursor-not-allowed">
                <ReceiptText className="w-4 h-4" />
                Transactions
              </span>
              <span className="flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium text-slate-500 cursor-not-allowed">
                <PiggyBank className="w-4 h-4" />
                Budgets
              </span>
              <span className="flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium text-slate-500 cursor-not-allowed">
                <BarChart3 className="w-4 h-4" />
                Analytics
              </span>
            </nav>
          </div>

          {/* User profile & Logout */}
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-3 pl-4 border-l border-slate-800">
              <div className="w-9 h-9 rounded-full bg-gradient-to-br from-emerald-500/20 to-teal-500/10 border border-emerald-500/30 flex items-center justify-center text-xs font-bold text-emerald-400 shadow-inner">
                {initials}
              </div>
              <div className="hidden sm:block text-left">
                <div className="text-sm font-semibold text-slate-200 leading-tight">
                  {displayName}
                </div>
                <div className="text-xs text-slate-400 leading-tight truncate max-w-[180px]">
                  {user?.email}
                </div>
              </div>
            </div>

            <button
              onClick={handleLogout}
              title="Sign out of your account"
              className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 border border-transparent hover:border-rose-500/20 transition-all duration-200 cursor-pointer"
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
