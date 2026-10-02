import React, { useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { ApiError } from '../api/client'
import { Wallet, Mail, Lock, Eye, EyeOff, ArrowRight, AlertCircle, Loader2 } from 'lucide-react'

export const LoginPage: React.FC = () => {
  const { login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)

  // Location redirect from protected route if user tried to visit a deep link
  const from = (location.state as { from?: { pathname: string } })?.from?.pathname || '/'

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    const trimmedEmail = email.trim()
    if (!trimmedEmail) {
      setError('Please enter your email address')
      return
    }
    if (!password) {
      setError('Please enter your password')
      return
    }

    setIsLoading(true)
    try {
      await login({ email: trimmedEmail, password })
      navigate(from, { replace: true })
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 401 || err.code === 'UNAUTHORIZED') {
          setError('Invalid email or password. Please check your credentials and try again.')
        } else {
          setError(err.message || 'Failed to login. Please try again.')
        }
      } else {
        setError('Network error. Please make sure the FinTrack backend server is running.')
      }
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-[#F7F9FC] flex flex-col justify-center py-12 sm:px-6 lg:px-8 relative overflow-hidden">
      <div className="sm:mx-auto sm:w-full sm:max-w-md relative z-10">
        {/* Brand Header */}
        <div className="flex flex-col items-center">
          <Link to="/" className="flex items-center gap-3 group mb-2">
            <div className="w-12 h-12 rounded-2xl bg-[#5BC8A4]/15 border border-[#5BC8A4]/30 flex items-center justify-center text-[#2F8F78] shadow-xs group-hover:bg-[#5BC8A4]/25 transition-all">
              <Wallet className="w-6 h-6 text-[#2F8F78] group-hover:scale-105 transition-transform" />
            </div>
            <span className="text-2xl font-extrabold tracking-tight text-[#243044]">
              Fin<span className="text-[#2F8F78]">Track</span>
            </span>
          </Link>
          <h2 className="text-2xl font-bold text-[#243044] tracking-tight">Welcome back</h2>
          <p className="mt-1 text-sm text-[#718096]">
            Sign in to manage your budgets and monitor personal finances
          </p>
        </div>

        {/* Card Form */}
        <div className="mt-8 sm:mx-auto sm:w-full sm:max-w-md">
          <div className="bg-white py-8 px-6 shadow-xs sm:rounded-2xl sm:px-10 border border-[#E5EAF1]">
            {error && (
              <div className="mb-6 rounded-xl bg-rose-50 border border-rose-200 p-4 flex items-start gap-3 text-[#C53030]">
                <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5 text-[#E98C8C]" />
                <div className="text-sm font-medium leading-relaxed">{error}</div>
              </div>
            )}

            <form className="space-y-5" onSubmit={handleSubmit}>
              <div>
                <label htmlFor="email" className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-2">
                  Email address
                </label>
                <div className="relative rounded-xl shadow-xs">
                  <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#718096]">
                    <Mail className="h-4 w-4" />
                  </div>
                  <input
                    id="email"
                    name="email"
                    type="email"
                    autoComplete="email"
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="you@example.com"
                    className="block w-full pl-10 pr-4 py-2.5 bg-white border border-[#E5EAF1] rounded-xl text-[#243044] placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4] text-sm transition"
                  />
                </div>
              </div>

              <div>
                <label htmlFor="password" className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-2">
                  Password
                </label>
                <div className="relative rounded-xl shadow-xs">
                  <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#718096]">
                    <Lock className="h-4 w-4" />
                  </div>
                  <input
                    id="password"
                    name="password"
                    type={showPassword ? 'text' : 'password'}
                    autoComplete="current-password"
                    required
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="••••••••"
                    className="block w-full pl-10 pr-10 py-2.5 bg-white border border-[#E5EAF1] rounded-xl text-[#243044] placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4] text-sm transition"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute inset-y-0 right-0 pr-3.5 flex items-center text-[#718096] hover:text-[#243044] transition-colors cursor-pointer"
                  >
                    {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
              </div>

              <div className="pt-2">
                <button
                  type="submit"
                  disabled={isLoading}
                  className="w-full flex items-center justify-center gap-2 py-3 px-4 rounded-xl text-sm font-semibold text-white bg-[#5BC8A4] hover:bg-[#4db995] focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-[#5BC8A4]/40 shadow-xs transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
                >
                  {isLoading ? (
                    <>
                      <Loader2 className="w-4 h-4 animate-spin" />
                      Signing in...
                    </>
                  ) : (
                    <>
                      Sign in
                      <ArrowRight className="w-4 h-4" />
                    </>
                  )}
                </button>
              </div>
            </form>

            <div className="mt-8 pt-6 border-t border-[#E5EAF1] text-center">
              <p className="text-sm text-[#718096]">
                Don't have an account yet?{' '}
                <Link
                  to="/register"
                  className="font-semibold text-[#2F8F78] hover:text-[#5BC8A4] transition-colors"
                >
                  Create an account
                </Link>
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
