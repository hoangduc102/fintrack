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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/40 backdrop-blur-xs animate-in fade-in duration-200">
      <div className="w-full max-w-xl bg-white border border-[#E5EAF1] rounded-2xl shadow-xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Modal Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-[#E5EAF1]">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-xl bg-emerald-50 border border-emerald-200/60 flex items-center justify-center text-[#2F8F78]">
              <FileSpreadsheet className="w-4 h-4" />
            </div>
            <div>
              <h3 className="text-base font-bold text-[#243044]">Import Transactions</h3>
              <p className="text-xs text-[#718096]">Batch import from a CSV file (up to 5MB)</p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 text-[#718096] hover:text-[#243044] rounded-lg transition cursor-pointer"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Modal Body */}
        <div className="p-6 overflow-y-auto space-y-5">
          {errorMessage && (
            <div className="p-3.5 rounded-xl bg-rose-50 border border-rose-200 text-[#C53030] text-xs flex items-center gap-2.5">
              <AlertTriangle className="w-4 h-4 shrink-0" />
              <span>{errorMessage}</span>
            </div>
          )}

          {/* Result View */}
          {result ? (
            <div className="space-y-5">
              <div className="p-4 rounded-xl bg-[#F7F9FC] border border-[#E5EAF1] flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div
                    className={`w-10 h-10 rounded-xl flex items-center justify-center ${
                      result.failed === 0
                        ? 'bg-emerald-50 text-[#2F8F78] border border-emerald-200'
                        : 'bg-amber-50 text-[#B45309] border border-amber-200'
                    }`}
                  >
                    {result.failed === 0 ? (
                      <CheckCircle2 className="w-5 h-5" />
                    ) : (
                      <AlertTriangle className="w-5 h-5" />
                    )}
                  </div>
                  <div>
                    <h4 className="text-sm font-bold text-[#243044]">
                      {result.failed === 0
                        ? 'Import Completed Successfully!'
                        : 'Import Completed with Partial Errors'}
                    </h4>
                    <p className="text-xs text-[#718096]">
                      Processed {result.total} total rows from your CSV file.
                    </p>
                  </div>
                </div>
              </div>

              {/* Metrics cards */}
              <div className="grid grid-cols-3 gap-3 text-center">
                <div className="p-3 bg-[#F7F9FC] border border-[#E5EAF1] rounded-xl">
                  <span className="text-[11px] font-medium text-[#718096] block mb-0.5">Total Rows</span>
                  <span className="text-lg font-bold text-[#243044]">{result.total}</span>
                </div>
                <div className="p-3 bg-emerald-50 border border-emerald-200 rounded-xl">
                  <span className="text-[11px] font-medium text-[#2F8F78] block mb-0.5">Imported</span>
                  <span className="text-lg font-bold text-[#2F8F78]">{result.imported}</span>
                </div>
                <div className="p-3 bg-rose-50 border border-rose-200 rounded-xl">
                  <span className="text-[11px] font-medium text-[#C53030] block mb-0.5">Failed</span>
                  <span className="text-lg font-bold text-[#C53030]">{result.failed}</span>
                </div>
              </div>

              {/* Errors list if any */}
              {result.errors && result.errors.length > 0 && (
                <div className="border border-[#E5EAF1] rounded-xl overflow-hidden">
                  <div className="px-4 py-2.5 bg-[#F7F9FC] border-b border-[#E5EAF1] flex items-center justify-between">
                    <span className="text-xs font-semibold text-[#C53030]">
                      Row Error Details ({result.errors.length})
                    </span>
                    <span className="text-[11px] text-[#718096]">Rows below were skipped</span>
                  </div>
                  <div className="max-h-48 overflow-y-auto divide-y divide-[#E5EAF1]">
                    {result.errors.map((err, idx) => (
                      <div key={idx} className="px-4 py-2 text-xs flex items-center justify-between gap-4">
                        <span className="font-mono text-[#718096] bg-[#F7F9FC] px-2 py-0.5 rounded text-[11px] border border-[#E5EAF1]">
                          Row {err.row}
                        </span>
                        <span className="text-[#C53030] text-right font-medium">{err.error}</span>
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
                    ? 'border-[#5BC8A4] bg-emerald-50/60 scale-[1.01]'
                    : selectedFile
                    ? 'border-[#5BC8A4]/60 bg-emerald-50/30'
                    : 'border-[#E5EAF1] hover:border-emerald-300 bg-[#F7F9FC]'
                }`}
              >
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".csv"
                  className="hidden"
                  onChange={handleFileChange}
                />

                <div className="w-12 h-12 rounded-2xl bg-emerald-50 border border-emerald-200/60 flex items-center justify-center text-[#2F8F78] mb-3">
                  <UploadCloud className="w-6 h-6" />
                </div>

                {selectedFile ? (
                  <div>
                    <p className="text-sm font-semibold text-[#243044]">{selectedFile.name}</p>
                    <p className="text-xs text-[#718096] mt-1">
                      {(selectedFile.size / 1024).toFixed(1)} KB — Click or drop to change
                    </p>
                  </div>
                ) : (
                  <div>
                    <p className="text-sm font-semibold text-[#243044]">
                      Drag & drop your CSV file here, or{' '}
                      <span className="text-[#2F8F78] underline underline-offset-2">browse</span>
                    </p>
                    <p className="text-xs text-[#718096] mt-1">Accepts standard .csv files up to 5MB</p>
                  </div>
                )}
              </div>

              {/* CSV Format Reference */}
              <div className="p-3.5 rounded-xl bg-[#F7F9FC] border border-[#E5EAF1] flex items-center justify-between text-xs">
                <div>
                  <span className="text-[#718096]">Expected CSV Header:</span>
                  <code className="ml-2 font-mono text-[11px] text-[#2F8F78] bg-emerald-50 border border-emerald-200/60 px-1.5 py-0.5 rounded">
                    date,type,amount,category,description
                  </code>
                </div>
                <button
                  type="button"
                  onClick={downloadSampleCsv}
                  className="inline-flex items-center gap-1.5 text-xs text-[#718096] hover:text-[#2F8F78] transition cursor-pointer"
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
        <div className="flex items-center justify-between px-6 py-4 border-t border-[#E5EAF1] bg-[#F7F9FC]">
          {result ? (
            <div className="flex items-center justify-between w-full">
              <button
                type="button"
                onClick={handleReset}
                className="inline-flex items-center gap-1.5 px-3 py-2 text-xs font-semibold text-[#718096] hover:text-[#243044] bg-white border border-[#E5EAF1] hover:bg-slate-50 rounded-xl transition cursor-pointer shadow-xs"
              >
                <RotateCcw className="w-3.5 h-3.5" />
                <span>Import Another</span>
              </button>
              <button
                type="button"
                onClick={onClose}
                className="px-4 py-2 text-xs font-semibold text-white bg-[#5BC8A4] hover:bg-[#4db995] rounded-xl transition cursor-pointer shadow-xs"
              >
                Done
              </button>
            </div>
          ) : (
            <div className="flex items-center justify-end gap-3 w-full">
              <button
                type="button"
                onClick={onClose}
                className="px-4 py-2 text-xs font-semibold text-[#718096] hover:text-[#243044] bg-white border border-[#E5EAF1] hover:bg-slate-50 rounded-xl transition cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="button"
                disabled={!selectedFile || importMutation.isPending}
                onClick={handleUpload}
                className="inline-flex items-center gap-2 px-4 py-2 text-xs font-semibold text-white bg-[#5BC8A4] hover:bg-[#4db995] rounded-xl transition disabled:opacity-50 cursor-pointer shadow-xs"
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
