import React, { useState, useRef } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { importTransactionsCSV } from '../../api/import'
import type { ImportResult } from '../../api/types'
import {
  UploadCloud,
  FileSpreadsheet,
  CheckCircle2,
  AlertTriangle,
  X,
  Loader2,
  Download,
  RotateCcw
} from 'lucide-react'

interface CsvImportModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess?: () => void
}

export const CsvImportModal: React.FC<CsvImportModalProps> = ({
  isOpen,
  onClose,
  onSuccess,
}) => {
  const queryClient = useQueryClient()
  const [dragActive, setDragActive] = useState(false)
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [result, setResult] = useState<ImportResult | null>(null)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const importMutation = useMutation({
    mutationFn: (file: File) => importTransactionsCSV(file),
    onSuccess: (data) => {
      setResult(data)
      setErrorMessage(null)
      // Invalidate relevant queries to refresh dashboard and transaction tables
      queryClient.invalidateQueries({ queryKey: ['transactions'] })
      queryClient.invalidateQueries({ queryKey: ['categories'] })
      queryClient.invalidateQueries({ queryKey: ['budgets'] })
      queryClient.invalidateQueries({ queryKey: ['analytics-summary'] })
      queryClient.invalidateQueries({ queryKey: ['analytics-categories'] })
      queryClient.invalidateQueries({ queryKey: ['analytics-trend'] })
      if (onSuccess) {
        onSuccess()
      }
    },
    onError: (err: any) => {
      setErrorMessage(err?.message || 'Failed to import CSV file')
    },
  })

  if (!isOpen) return null

  const handleDrag = (e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setDragActive(true)
    } else if (e.type === 'dragleave') {
      setDragActive(false)
    }
  }

  const validateAndSetFile = (file: File) => {
    if (!file.name.toLowerCase().endsWith('.csv')) {
      setErrorMessage('Please select a valid .csv file')
      return
    }
    if (file.size > 5 * 1024 * 1024) {
      setErrorMessage('File size exceeds the 5MB maximum limit')
      return
    }
    setErrorMessage(null)
    setSelectedFile(file)
  }

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setDragActive(false)
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      validateAndSetFile(e.dataTransfer.files[0])
    }
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      validateAndSetFile(e.target.files[0])
    }
  }

  const handleUpload = () => {
    if (!selectedFile) return
    importMutation.mutate(selectedFile)
  }

  const handleReset = () => {
    setSelectedFile(null)
    setResult(null)
    setErrorMessage(null)
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

  const downloadSampleCsv = () => {
    const csvContent =
      'date,type,amount,category,description\n' +
      '2026-03-01,INCOME,25000000,Salary,Monthly software engineer salary\n' +
      '2026-03-02,EXPENSE,45000,Food & Dining,Morning coffee & breakfast\n' +
      '2026-03-05,EXPENSE,350000,Shopping,Household essentials\n' +
      '2026-03-10,EXPENSE,120000,Utilities,Monthly internet bill\n'

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', 'fintrack_sample_transactions.csv')
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="w-full max-w-xl bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Modal Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
              <FileSpreadsheet className="w-4 h-4" />
            </div>
            <div>
              <h3 className="text-base font-bold text-white">Import Transactions</h3>
              <p className="text-xs text-slate-400">Batch import from a CSV file (up to 5MB)</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 text-slate-400 hover:text-white rounded-lg transition"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Modal Body */}
        <div className="p-6 overflow-y-auto space-y-5">
          {errorMessage && (
            <div className="p-3.5 rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs flex items-center gap-2.5">
              <AlertTriangle className="w-4 h-4 shrink-0" />
              <span>{errorMessage}</span>
            </div>
          )}

          {/* Result View */}
          {result ? (
            <div className="space-y-5">
              <div className="p-4 rounded-xl bg-slate-800/40 border border-slate-800 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div
                    className={`w-10 h-10 rounded-xl flex items-center justify-center ${
                      result.failed === 0
                        ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                        : 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                    }`}
                  >
                    {result.failed === 0 ? (
                      <CheckCircle2 className="w-5 h-5" />
                    ) : (
                      <AlertTriangle className="w-5 h-5" />
                    )}
                  </div>
                  <div>
                    <h4 className="text-sm font-bold text-white">
                      {result.failed === 0
                        ? 'Import Completed Successfully!'
                        : 'Import Completed with Partial Errors'}
                    </h4>
                    <p className="text-xs text-slate-400">
                      Processed {result.total} total rows from your CSV file.
                    </p>
                  </div>
                </div>
              </div>

              {/* Metrics cards */}
              <div className="grid grid-cols-3 gap-3 text-center">
                <div className="p-3 bg-slate-800/30 border border-slate-800 rounded-xl">
                  <span className="text-[11px] font-medium text-slate-400 block mb-0.5">Total Rows</span>
                  <span className="text-lg font-bold text-white">{result.total}</span>
                </div>
                <div className="p-3 bg-emerald-500/5 border border-emerald-500/20 rounded-xl">
                  <span className="text-[11px] font-medium text-emerald-400 block mb-0.5">Imported</span>
                  <span className="text-lg font-bold text-emerald-400">{result.imported}</span>
                </div>
                <div className="p-3 bg-rose-500/5 border border-rose-500/20 rounded-xl">
                  <span className="text-[11px] font-medium text-rose-400 block mb-0.5">Failed</span>
                  <span className="text-lg font-bold text-rose-400">{result.failed}</span>
                </div>
              </div>

              {/* Errors list if any */}
              {result.errors && result.errors.length > 0 && (
                <div className="border border-slate-800 rounded-xl overflow-hidden">
                  <div className="px-4 py-2.5 bg-slate-800/60 border-b border-slate-800 flex items-center justify-between">
                    <span className="text-xs font-semibold text-rose-400">
                      Row Error Details ({result.errors.length})
                    </span>
                    <span className="text-[11px] text-slate-400">Rows below were skipped</span>
                  </div>
                  <div className="max-h-48 overflow-y-auto divide-y divide-slate-800">
                    {result.errors.map((err, idx) => (
                      <div key={idx} className="px-4 py-2 text-xs flex items-center justify-between gap-4">
                        <span className="font-mono text-slate-400 bg-slate-800 px-2 py-0.5 rounded text-[11px]">
                          Row {err.row}
                        </span>
                        <span className="text-rose-400 text-right font-medium">{err.error}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          ) : (
            /* Upload Dropzone View */
            <div className="space-y-4">
              <div
                onDragEnter={handleDrag}
                onDragLeave={handleDrag}
                onDragOver={handleDrag}
                onDrop={handleDrop}
                onClick={() => fileInputRef.current?.click()}
                className={`relative border-2 border-dashed rounded-2xl p-8 flex flex-col items-center justify-center text-center cursor-pointer transition-all ${
                  dragActive
                    ? 'border-emerald-500 bg-emerald-500/10 scale-[1.01]'
                    : selectedFile
                    ? 'border-emerald-500/50 bg-slate-800/40'
                    : 'border-slate-700/80 hover:border-slate-600 bg-slate-800/20'
                }`}
              >
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".csv"
                  className="hidden"
                  onChange={handleFileChange}
                />

                <div className="w-12 h-12 rounded-2xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400 mb-3">
                  <UploadCloud className="w-6 h-6" />
                </div>

                {selectedFile ? (
                  <div>
                    <p className="text-sm font-semibold text-white">{selectedFile.name}</p>
                    <p className="text-xs text-slate-400 mt-1">
                      {(selectedFile.size / 1024).toFixed(1)} KB — Click or drop to change
                    </p>
                  </div>
                ) : (
                  <div>
                    <p className="text-sm font-semibold text-slate-200">
                      Drag & drop your CSV file here, or{' '}
                      <span className="text-emerald-400 underline underline-offset-2">browse</span>
                    </p>
                    <p className="text-xs text-slate-500 mt-1">Accepts standard .csv files up to 5MB</p>
                  </div>
                )}
              </div>

              {/* CSV Format Reference */}
              <div className="p-3.5 rounded-xl bg-slate-800/30 border border-slate-800 flex items-center justify-between text-xs">
                <div>
                  <span className="text-slate-400">Expected CSV Header:</span>
                  <code className="ml-2 font-mono text-[11px] text-emerald-400 bg-emerald-500/10 px-1.5 py-0.5 rounded">
                    date,type,amount,category,description
                  </code>
                </div>
                <button
                  type="button"
                  onClick={downloadSampleCsv}
                  className="inline-flex items-center gap-1.5 text-xs text-slate-400 hover:text-emerald-400 transition"
                  title="Download sample template"
                >
                  <Download className="w-3.5 h-3.5" />
                  <span>Sample CSV</span>
                </button>
              </div>
            </div>
          )}
        </div>

        {/* Modal Footer */}
        <div className="flex items-center justify-between px-6 py-4 border-t border-slate-800 bg-slate-900/50">
          {result ? (
            <div className="flex items-center justify-between w-full">
              <button
                type="button"
                onClick={handleReset}
                className="inline-flex items-center gap-1.5 px-3 py-2 text-xs font-semibold text-slate-300 hover:text-white bg-slate-800 hover:bg-slate-700 rounded-xl transition"
              >
                <RotateCcw className="w-3.5 h-3.5" />
                <span>Import Another</span>
              </button>
              <button
                type="button"
                onClick={onClose}
                className="px-4 py-2 text-xs font-semibold text-white bg-emerald-600 hover:bg-emerald-500 rounded-xl transition"
              >
                Done
              </button>
            </div>
          ) : (
            <div className="flex items-center justify-end gap-3 w-full">
              <button
                type="button"
                onClick={onClose}
                className="px-4 py-2 text-xs font-semibold text-slate-300 hover:text-white bg-slate-800 hover:bg-slate-700 rounded-xl transition"
              >
                Cancel
              </button>
              <button
                type="button"
                disabled={!selectedFile || importMutation.isPending}
                onClick={handleUpload}
                className="inline-flex items-center gap-2 px-4 py-2 text-xs font-semibold text-white bg-emerald-600 hover:bg-emerald-500 active:bg-emerald-700 rounded-xl transition disabled:opacity-50"
              >
                {importMutation.isPending ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    <span>Processing CSV...</span>
                  </>
                ) : (
                  <>
                    <UploadCloud className="w-4 h-4" />
                    <span>Start Import</span>
                  </>
                )}
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
