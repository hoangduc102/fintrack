import React from 'react'
import { Navbar } from './Navbar'

export const Layout: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  return (
    <div className="min-h-screen bg-[#F7F9FC] text-[#243044] flex flex-col">
      <Navbar />

      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {children}
      </main>

      <footer className="border-t border-[#E5EAF1] bg-white py-6 text-center text-xs text-[#718096]">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row items-center justify-between gap-4">
          <p>© 2026 FinTrack. All rights reserved.</p>
          <div className="flex items-center gap-6 text-[#718096]">
            <span>Secure Go Backend (API v1)</span>
            <span>•</span>
            <span>React + Vite Client</span>
          </div>
        </div>
      </footer>
    </div>
  )
}
