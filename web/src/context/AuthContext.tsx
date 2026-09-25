import React, { createContext, useContext, useEffect, useState } from 'react'
import { getMe, login as apiLogin, register as apiRegister } from '../api/auth'
import { getToken, removeToken, setToken } from '../api/client'
import type { LoginRequest, RegisterRequest, User } from '../api/types'

interface AuthContextType {
  user: User | null
  token: string | null
  isLoading: boolean
  isAuthenticated: boolean
  login: (input: LoginRequest) => Promise<void>
  register: (input: RegisterRequest) => Promise<User>
  logout: () => void
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null)
  const [token, setTokenState] = useState<string | null>(() => getToken())
  const [isLoading, setIsLoading] = useState<boolean>(true)

  const logout = () => {
    removeToken()
    setTokenState(null)
    setUser(null)
  }

  // Session restoration on boot
  useEffect(() => {
    let isMounted = true

    async function restoreSession() {
      const storedToken = getToken()
      if (!storedToken) {
        if (isMounted) {
          setIsLoading(false)
        }
        return
      }

      try {
        const userProfile = await getMe()
        if (isMounted) {
          setUser(userProfile)
          setTokenState(storedToken)
        }
      } catch (err) {
        console.error('Session restoration failed:', err)
        if (isMounted) {
          logout()
        }
      } finally {
        if (isMounted) {
          setIsLoading(false)
        }
      }
    }

    restoreSession()

    const handleUnauthorized = () => {
      logout()
    }
    window.addEventListener('fintrack:unauthorized', handleUnauthorized)

    return () => {
      isMounted = false
      window.removeEventListener('fintrack:unauthorized', handleUnauthorized)
    }
  }, [])

  const login = async (input: LoginRequest) => {
    const res = await apiLogin(input)
    setToken(res.token)
    setTokenState(res.token)
    // Fetch profile after login
    const userProfile = await getMe()
    setUser(userProfile)
  }

  const register = async (input: RegisterRequest): Promise<User> => {
    const newUser = await apiRegister(input)
    return newUser
  }

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        isLoading,
        isAuthenticated: !!user && !!token,
        login,
        register,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthContextType {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
