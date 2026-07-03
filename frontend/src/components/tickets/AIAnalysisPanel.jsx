import { useState, useEffect, useRef } from 'react'
import { aiService } from '../../services/aiService'

// ─── Utility badges ──────────────────────────────────────────────────────────

const PRIORITY_COLOR = {
  Urgent: 'bg-red-100 text-red-700 border-red-200',
  High:   'bg-orange-100 text-orange-700 border-orange-200',
  Medium: 'bg-amber-100 text-amber-700 border-amber-200',
  Low:    'bg-green-100 text-green-700 border-green-200',
}

const SENTIMENT_COLOR = {
  Positive:   'bg-green-100 text-green-700',
  Neutral:    'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-300 dark:text-gray-600',
  Frustrated: 'bg-orange-100 text-orange-700',
  Angry:      'bg-red-100 text-red-600',
  Confused:   'bg-purple-100 text-purple-700',
}

function Badge({ label, colorClass }) {
  return (
    <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium border ${colorClass}`}>
      {label}
    </span>
  )
}

function ConfidenceBar({ value }) {
  const color = value >= 85 ? 'bg-green-500' : value >= 60 ? 'bg-amber-500' : 'bg-red-500'
  return (
    <div className="space-y-1">
      <div className="flex justify-between text-xs text-gray-500 dark:text-gray-400 dark:text-gray-500">
        <span>Confidence</span>
        <span className="font-semibold text-gray-700 dark:text-gray-200">{value}%</span>
      </div>
      <div className="h-2 w-full rounded-full bg-gray-100 dark:bg-gray-800 overflow-hidden">
        <div
          className={`h-full rounded-full transition-all duration-500 ${color}`}
          style={{ width: `${value}%` }}
        />
      </div>
    </div>
  )
}

// ─── Main component ──────────────────────────────────────────────────────────

const POLL_INTERVAL_MS = 3500

export default function AIAnalysisPanel({ ticketId }) {
  const [analysis, setAnalysis] = useState(null)
  const [loading, setLoading] = useState(true)
  const [retrying, setRetrying] = useState(false)
  const intervalRef = useRef(null)

  const fetch = async () => {
    try {
      const r = await aiService.getAnalysis(ticketId)
      const data = r.data.data
      setAnalysis(data)
      // Stop polling once we have a terminal status
      if (data.processing_status === 'COMPLETED' || data.processing_status === 'FAILED') {
        clearInterval(intervalRef.current)
      }
    } catch {
      // Silently ignore network errors during polling
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetch()
    // Start polling; will clear itself on COMPLETED / FAILED
    intervalRef.current = setInterval(fetch, POLL_INTERVAL_MS)
    return () => clearInterval(intervalRef.current)
  }, [ticketId])

  const handleRetry = async () => {
    setRetrying(true)
    try {
      await aiService.retryAnalysis(ticketId)
      setAnalysis((prev) => ({ ...prev, processing_status: 'PROCESSING' }))
      // Restart polling
      clearInterval(intervalRef.current)
      intervalRef.current = setInterval(fetch, POLL_INTERVAL_MS)
    } catch {
      // ignore
    } finally {
      setRetrying(false)
    }
  }

  if (loading) {
    return <p className="p-4 text-sm text-gray-400 dark:text-gray-500 animate-pulse">Loading AI analysis…</p>
  }

  const status = analysis?.processing_status

  // ── Pending / Processing ──────────────────────────────────────────────────
  if (status === 'PENDING' || status === 'PROCESSING') {
    return (
      <div className="p-6 flex flex-col items-center justify-center gap-4 text-center">
        <div className="relative w-12 h-12">
          <div className="absolute inset-0 rounded-full border-4 border-blue-100" />
          <div className="absolute inset-0 rounded-full border-4 border-blue-500 border-t-transparent animate-spin" />
        </div>
        <div>
          <p className="text-sm font-semibold text-gray-700 dark:text-gray-200">AI is analyzing this ticket…</p>
          <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">This usually takes a few seconds</p>
        </div>
      </div>
    )
  }

  // ── Failed ────────────────────────────────────────────────────────────────
  if (status === 'FAILED') {
    return (
      <div className="p-6 flex flex-col items-center justify-center gap-4 text-center">
        <span className="text-3xl">⚠️</span>
        <div>
          <p className="text-sm font-semibold text-gray-700 dark:text-gray-200">AI analysis failed</p>
          <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">
            The AI provider was unable to process this ticket.
          </p>
        </div>
        <button
          onClick={handleRetry}
          disabled={retrying}
          className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50 transition"
        >
          {retrying ? 'Queuing retry…' : 'Retry Analysis'}
        </button>
      </div>
    )
  }

  // ── Completed ─────────────────────────────────────────────────────────────
  if (status !== 'COMPLETED') {
    return <p className="p-4 text-sm text-gray-400 dark:text-gray-500">No AI analysis available.</p>
  }

  return (
    <div className="p-5 space-y-6">
      {/* Summary */}
      <div className="rounded-xl bg-blue-50 border border-blue-100 p-4">
        <p className="text-xs font-semibold text-blue-500 uppercase tracking-wide mb-1">AI Summary</p>
        <p className="text-sm text-gray-800 dark:text-gray-100 leading-relaxed">{analysis.summary}</p>
      </div>

      {/* Confidence */}
      {analysis.confidence != null && (
        <ConfidenceBar value={analysis.confidence} />
      )}

      {/* Classification grid */}
      <div className="grid grid-cols-2 gap-4">
        <div className="rounded-xl border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 shadow-sm">
          <p className="text-xs text-gray-400 dark:text-gray-500 mb-2">Detected Category</p>
          <Badge
            label={analysis.category}
            colorClass="bg-indigo-50 text-indigo-700 border-indigo-100"
          />
        </div>

        <div className="rounded-xl border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 shadow-sm">
          <p className="text-xs text-gray-400 dark:text-gray-500 mb-2">Suggested Priority</p>
          <Badge
            label={analysis.priority}
            colorClass={PRIORITY_COLOR[analysis.priority] || 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-300 dark:text-gray-600 border-gray-200 dark:border-gray-600'}
          />
        </div>

        <div className="rounded-xl border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 shadow-sm">
          <p className="text-xs text-gray-400 dark:text-gray-500 mb-2">Customer Sentiment</p>
          <Badge
            label={analysis.sentiment}
            colorClass={`${SENTIMENT_COLOR[analysis.sentiment] || 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-300 dark:text-gray-600'} border border-transparent`}
          />
        </div>

        <div className="rounded-xl border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 shadow-sm">
          <p className="text-xs text-gray-400 dark:text-gray-500 mb-2">Recommended Team</p>
          <Badge
            label={analysis.recommended_team}
            colorClass="bg-teal-50 text-teal-700 border-teal-100"
          />
        </div>
      </div>

      {/* Tags */}
      {analysis.tags && analysis.tags.length > 0 && (
        <div>
          <p className="text-xs text-gray-400 dark:text-gray-500 mb-2">Tags</p>
          <div className="flex flex-wrap gap-2">
            {analysis.tags.map((tag) => (
              <span
                key={tag}
                className="rounded-full bg-gray-100 dark:bg-gray-800 px-3 py-1 text-xs font-medium text-gray-600 dark:text-gray-300 dark:text-gray-600"
              >
                #{tag}
              </span>
            ))}
          </div>
        </div>
      )}

      {/* Footer */}
      <div className="flex items-center justify-between pt-2 border-t border-gray-100 dark:border-gray-700">
        <p className="text-xs text-gray-400 dark:text-gray-500">
          {analysis.processed_at
            ? `Analyzed at ${new Date(analysis.processed_at).toLocaleString()}`
            : ''}
        </p>
        <button
          onClick={handleRetry}
          disabled={retrying}
          className="text-xs text-gray-400 dark:text-gray-500 hover:text-blue-600 underline disabled:opacity-50 transition"
        >
          {retrying ? 'Queuing…' : 'Re-analyze'}
        </button>
      </div>
    </div>
  )
}
