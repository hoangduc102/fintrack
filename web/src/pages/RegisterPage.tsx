import React, { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { ApiError } from '../api/client'
import { Wallet, Mail, Lock, User as UserIcon, Eye, EyeOff, ArrowRight, AlertCircle, Loader2, CheckCircle2 } from 'lucide-react'

export const RegisterPage: React.FC = () => {
  const { register, login } = useAuth()
  const navigate = useNavigate()

  const [email, setEmail] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)

  const [generalError, setGeneralError] = useState<string | null>(null)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [isLoading, setIsLoading] = useState(false)
  const [isSuccess, setIsSuccess] = useState(false)

  const validateForm = (): boolean => {
    const errors: Record<string, string> = {}
    const trimmedEmail = email.trim()

    if (!trimmedEmail) {
      errors.email = 'Email address is required'
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmedEmail)) {
      errors.email = 'Please enter a valid email address'
    }

    if (!password) {
      errors.password = 'Password is required'
    } else if (password.length < 8) {
      errors.password = 'Password must be at least 8 characters'
    } else if (new TextEncoder().encode(password).length > 72) {
      errors.password = 'Password cannot exceed 72 bytes'
    }

    if (password !== confirmPassword) {
      errors.confirmPassword = 'Passwords do not match'
    }

    setFieldErrors(errors)
    return Object.keys(errors).length === 0
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setGeneralError(null)
    setFieldErrors({})

    if (!validateForm()) {
      return
    }

    setIsLoading(true)
    try {
      await register({
        email: email.trim().toLowerCase(),
        password,
        display_name: displayName.trim() || undefined,
      })

      setIsSuccess(true)

      // Automatically sign in the user for seamless onboarding
      try {
        await login({ email: email.trim().toLowerCase(), password })
        navigate('/', { replace: true })
      } catch {
        navigate('/login', { replace: true })
      }
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 409 || err.code === 'CONFLICT') {
          setGeneralError('An account with this email address already exists. Please sign in instead.')
        } else if (err.fields) {
          setFieldErrors(err.fields)
        } else {
          setGeneralError(err.message || 'Registration failed. Please check your inputs.')
        }
      } else {
        setGeneralError('Network error. Unable to connect to the FinTrack backend.')
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
          <h2 className="text-2xl font-bold text-[#243044] tracking-tight">Create an account</h2>
          <p className="mt-1 text-sm text-[#718096]">
            Start tracking expenses and managing monthly budgets
          </p>
        </div>

        {/* Card Form */}
        <div className="mt-8 sm:mx-auto sm:w-full sm:max-w-md">
          <div className="bg-white py-8 px-6 shadow-xs sm:rounded-2xl sm:px-10 border border-[#E5EAF1]">
            {generalError && (
              <div className="mb-6 rounded-xl bg-rose-50 border border-rose-200 p-4 flex items-start gap-3 text-[#C53030]">
                <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5 text-[#E98C8C]" />
                <div className="text-sm font-medium leading-relaxed">{generalError}</div>
              </div>
            )}

            {isSuccess && (
              <div className="mb-6 rounded-xl bg-emerald-50 border border-emerald-200 p-4 flex items-start gap-3 text-[#2F8F78]">
                <CheckCircle2 className="w-5 h-5 flex-shrink-0 mt-0.5 text-[#5BC8A4]" />
                <div className="text-sm font-medium leading-relaxed">
                  Account created successfully! Default categories initialized. Redirecting...
                </div>
              </div>
            )}

            <form className="space-y-4" onSubmit={handleSubmit}>
              <div>
                <label htmlFor="email" className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Email address <span className="text-[#5BC8A4]">*</span>
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
                    className={`block w-full pl-10 pr-4 py-2.5 bg-white border rounded-xl text-[#243044] placeholder-slate-400 focus:outline-none focus:ring-2 text-sm transition ${
                      fieldErrors.email
                        ? 'border-rose-400 focus:ring-rose-100 focus:border-rose-400'
                        : 'border-[#E5EAF1] focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4]'
                    }`}
                  />
                </div>
                {fieldErrors.email && (
                  <p className="mt-1.5 text-xs text-[#C53030] font-medium">{fieldErrors.email}</p>
                )}
              </div>

              <div>
                <label htmlFor="displayName" className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Display name <span className="text-[#718096] font-normal">(optional)</span>
                </label>
                <div className="relative rounded-xl shadow-xs">
                  <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#718096]">
                    <UserIcon className="h-4 w-4" />
                  </div>
                  <input
                    id="displayName"
                    name="displayName"
                    type="text"
                    value={displayName}
                    onChange={(e) => setDisplayName(e.target.value)}
                    placeholder="Duc Hoang"
                    className="block w-full pl-10 pr-4 py-2.5 bg-white border border-[#E5EAF1] rounded-xl text-[#243044] placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4] text-sm transition"
                  />
                </div>
              </div>

              <div>
                <label htmlFor="password" className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Password <span className="text-[#5BC8A4]">*</span>
                </label>
                <div className="relative rounded-xl shadow-xs">
                  <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#718096]">
                    <Lock className="h-4 w-4" />
                  </div>
                  <input
                    id="password"
                    name="password"
                    type={showPassword ? 'text' : 'password'}
                    autoComplete="new-password"
                    required
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="Min. 8 characters"
                    className={`block w-full pl-10 pr-10 py-2.5 bg-white border rounded-xl text-[#243044] placeholder-slate-400 focus:outline-none focus:ring-2 text-sm transition ${
                      fieldErrors.password
                        ? 'border-rose-400 focus:ring-rose-100 focus:border-rose-400'
                        : 'border-[#E5EAF1] focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4]'
                    }`}
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute inset-y-0 right-0 pr-3.5 flex items-center text-[#718096] hover:text-[#243044] transition-colors cursor-pointer"
                  >
                    {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
                {fieldErrors.password && (
                  <p className="mt-1.5 text-xs text-[#C53030] font-medium">{fieldErrors.password}</p>
                )}
              </div>

              <div>
                <label htmlFor="confirmPassword" className="block text-xs font-semibold text-[#718096] uppercase tracking-wider mb-1.5">
                  Confirm password <span className="text-[#5BC8A4]">*</span>
                </label>
                <div className="relative rounded-xl shadow-xs">
                  <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#718096]">
                    <Lock className="h-4 w-4" />
                  </div>
                  <input
                    id="confirmPassword"
                    name="confirmPassword"
                    type={showPassword ? 'text' : 'password'}
                    autoComplete="new-password"
                    required
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    placeholder="Re-enter password"
                    className={`block w-full pl-10 pr-4 py-2.5 bg-white border rounded-xl text-[#243044] placeholder-slate-400 focus:outline-none focus:ring-2 text-sm transition ${
                      fieldErrors.confirmPassword
                        ? 'border-rose-400 focus:ring-rose-100 focus:border-rose-400'
                        : 'border-[#E5EAF1] focus:ring-[#5BC8A4]/20 focus:border-[#5BC8A4]'
                    }`}
                  />
                </div>
                {fieldErrors.confirmPassword && (
                  <p className="mt-1.5 text-xs text-[#C53030] font-medium">{fieldErrors.confirmPassword}</p>
                )}
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
                      Creating account...
                    </>
                  ) : (
                    <>
                      Create account
                      <ArrowRight className="w-4 h-4" />
                    </>
                  )}
                </button>
              </div>
            </form>

            <div className="mt-8 pt-6 border-t border-[#E5EAF1] text-center">
              <p className="text-sm text-[#718096]">
                Already have an account?{' '}
                <Link
                  to="/login"
                  className="font-semibold text-[#2F8F78] hover:text-[#5BC8A4] transition-colors"
                >
                  Sign in
                </Link>
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
