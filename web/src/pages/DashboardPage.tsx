import React from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { 
  Sparkles, 
  ShieldCheck, 
  Mail, 
  Fingerprint, 
  Calendar, 
  Tags,
  ReceiptText,
  PiggyBank,
  CheckCircle2,
  Cpu
} from 'lucide-react'

export const DashboardPage: React.FC = () => {
  const { user } = useAuth()

  const displayName = user?.display_name || user?.email?.split('@')[0] || 'User'
  const memberSince = user?.created_at
    ? new Date(user.created_at).toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'long',
        day: 'numeric',
      })
    : 'Recently'

  return (
    <div className="space-y-8 animate-in fade-in duration-500">
      {/* Welcome Banner */}
      <div className="relative overflow-hidden rounded-3xl bg-gradient-to-r from-emerald-950/60 via-slate-900/80 to-slate-900/60 border border-emerald-500/20 p-8 shadow-2xl backdrop-blur-xl">
        <div className="absolute top-0 right-0 -mt-12 -mr-12 w-64 h-64 bg-emerald-500/10 rounded-full blur-3xl pointer-events-none" />
        
        <div className="relative z-10 flex flex-col md:flex-row md:items-center justify-between gap-6">
          <div>
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-500/10 border border-emerald-500/20 text-xs font-semibold text-emerald-400 mb-4">
              <Sparkles className="w-3.5 h-3.5" />
              <span>Session Authenticated</span>
            </div>
            <h1 className="text-3xl sm:text-4xl font-extrabold text-white tracking-tight">
              Hello, <span className="bg-gradient-to-r from-emerald-400 to-teal-300 bg-clip-text text-transparent">{displayName}</span>!
            </h1>
            <p className="mt-2 text-slate-400 text-sm sm:text-base max-w-xl">
              Your FinTrack personal finance workspace is active. Default categories have been seeded into your account.
            </p>
          </div>

          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2 bg-slate-950/60 border border-slate-800 rounded-2xl px-4 py-3">
              <div className="w-3 h-3 rounded-full bg-emerald-400 animate-ping" />
              <div className="text-xs font-medium text-slate-300">
                <span className="text-slate-400">Status:</span> Connected
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Profile Overview Card & Quick Specs */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* Profile Card */}
        <div className="md:col-span-1 rounded-2xl bg-slate-900/50 border border-slate-800 p-6 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between pb-4 border-b border-slate-800/80 mb-4">
              <h3 className="text-sm font-semibold uppercase tracking-wider text-slate-400 flex items-center gap-2">
                <ShieldCheck className="w-4 h-4 text-emerald-400" />
                Account Profile
              </h3>
              <span className="text-[11px] font-bold px-2 py-0.5 rounded-full bg-emerald-500/20 text-emerald-300 border border-emerald-500/30">
                Active
              </span>
            </div>

            <div className="space-y-4">
              <div>
                <span className="text-xs text-slate-400 flex items-center gap-1.5 mb-1">
                  <Mail className="w-3.5 h-3.5" />
                  Email
                </span>
                <p className="text-sm font-medium text-slate-200 truncate">{user?.email}</p>
              </div>

              <div>
                <span className="text-xs text-slate-400 flex items-center gap-1.5 mb-1">
                  <Fingerprint className="w-3.5 h-3.5" />
                  User UUID
                </span>
                <p className="text-xs font-mono text-slate-400 bg-slate-950/80 p-2 rounded-lg border border-slate-800/80 break-all select-all">
                  {user?.id}
                </p>
              </div>

              <div>
                <span className="text-xs text-slate-400 flex items-center gap-1.5 mb-1">
                  <Calendar className="w-3.5 h-3.5" />
                  Member Since
                </span>
                <p className="text-sm font-medium text-slate-300">{memberSince}</p>
              </div>
            </div>
          </div>

          <div className="mt-6 pt-4 border-t border-slate-800/80">
            <div className="flex items-center gap-2 text-xs text-slate-400">
              <CheckCircle2 className="w-4 h-4 text-emerald-400" />
              <span>Session stored in localStorage</span>
            </div>
          </div>
        </div>

        {/* System Capabilities & Seeded State */}
        <div className="md:col-span-2 rounded-2xl bg-slate-900/50 border border-slate-800 p-6 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between pb-4 border-b border-slate-800/80 mb-6">
              <h3 className="text-sm font-semibold uppercase tracking-wider text-slate-400 flex items-center gap-2">
                <Cpu className="w-4 h-4 text-teal-400" />
                Backend Integration Status
              </h3>
              <span className="text-xs text-slate-400">API v1 Ready</span>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div className="rounded-xl bg-slate-950/60 border border-slate-800/80 p-4">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-xs font-medium text-slate-400">Default Categories</span>
                  <Tags className="w-4 h-4 text-emerald-400" />
                </div>
                <div className="text-xl font-bold text-white">6 Seeded</div>
                <p className="text-xs text-slate-400 mt-1">4 Expense & 2 Income categories auto-created</p>
              </div>

              <div className="rounded-xl bg-slate-950/60 border border-slate-800/80 p-4">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-xs font-medium text-slate-400">Money Precision</span>
                  <ReceiptText className="w-4 h-4 text-teal-400" />
                </div>
                <div className="text-xl font-bold text-white">VND int64</div>
                <p className="text-xs text-slate-400 mt-1">100% exact integer math (No float rounding)</p>
              </div>

              <div className="rounded-xl bg-slate-950/60 border border-slate-800/80 p-4">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-xs font-medium text-slate-400">Worker Pool</span>
                  <Cpu className="w-4 h-4 text-indigo-400" />
                </div>
                <div className="text-xl font-bold text-white">4 Workers</div>
                <p className="text-xs text-slate-400 mt-1">Concurrent CSV transaction ingestion</p>
              </div>

              <div className="rounded-xl bg-slate-950/60 border border-slate-800/80 p-4">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-xs font-medium text-slate-400">Monthly Budgets</span>
                  <PiggyBank className="w-4 h-4 text-amber-400" />
                </div>
                <div className="text-xl font-bold text-white">Dynamic SQL</div>
                <p className="text-xs text-slate-400 mt-1">LEFT JOIN spent aggregation & Upsert</p>
              </div>
            </div>
          </div>

          <div className="mt-6 pt-4 border-t border-slate-800/80 flex items-center justify-between text-xs text-slate-400">
            <span>Authentication Token: Valid (24h TTL)</span>
            <span className="text-emerald-400 font-medium">Issue #12 Foundation Complete</span>
          </div>
        </div>
      </div>

      {/* Next Slices Roadmap Banner */}
      <div className="rounded-2xl bg-gradient-to-r from-slate-900/90 to-slate-950 border border-slate-800 p-6">
        <h3 className="text-base font-bold text-white mb-2 flex items-center gap-2">
          <span>Upcoming Client Slices</span>
          <span className="text-xs font-normal text-slate-400">(Already supported by Go Backend)</span>
        </h3>
        <p className="text-xs text-slate-400 mb-4">
          The backend API for Categories, Transactions, Budgets, and Analytics is fully tested and ready.
        </p>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <Link 
            to="/categories" 
            className="p-4 rounded-xl bg-slate-950/60 border border-slate-800/80 hover:border-emerald-500/50 hover:bg-slate-900/60 transition-all group block"
          >
            <div className="flex items-center justify-between text-xs font-semibold text-emerald-400 mb-1">
              <span>Categories</span>
              <span className="text-[10px] uppercase tracking-wider px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-300">Live</span>
            </div>
            <div className="text-sm font-bold text-slate-200 group-hover:text-white transition-colors">Category Management</div>
            <div className="text-xs text-slate-400 mt-1">Create, edit, and delete income/expense tags</div>
          </Link>

          <Link 
            to="/transactions" 
            className="p-4 rounded-xl bg-slate-950/60 border border-slate-800/80 hover:border-emerald-500/50 hover:bg-slate-900/60 transition-all group block"
          >
            <div className="flex items-center justify-between text-xs font-semibold text-emerald-400 mb-1">
              <span>Transactions</span>
              <span className="text-[10px] uppercase tracking-wider px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-300">Live</span>
            </div>
            <div className="text-sm font-bold text-slate-200 group-hover:text-white transition-colors">Transaction History</div>
            <div className="text-xs text-slate-400 mt-1">Multi-criteria filtering, pagination, and forms</div>
          </Link>

          <div className="p-4 rounded-xl bg-slate-950/60 border border-slate-800/80 opacity-60">
            <div className="text-xs font-semibold text-amber-400 mb-1">Issue #14</div>
            <div className="text-sm font-bold text-slate-200">Monthly Budgets</div>
            <div className="text-xs text-slate-400 mt-1">Upsert caps & live spending progress bars</div>
          </div>

          <div className="p-4 rounded-xl bg-slate-950/60 border border-slate-800/80 opacity-60">
            <div className="text-xs font-semibold text-indigo-400 mb-1">Issue #14</div>
            <div className="text-sm font-bold text-slate-200">Financial Analytics</div>
            <div className="text-xs text-slate-400 mt-1">Category shares & 6-month trend charts</div>
          </div>
        </div>
      </div>
    </div>
  )
}
