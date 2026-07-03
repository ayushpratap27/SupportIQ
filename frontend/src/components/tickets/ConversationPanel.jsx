import { useState, useEffect, useCallback } from 'react'
import { commentService } from '../../services/commentService'
import { formatDate } from '../../utils/format'

export default function ConversationPanel({ ticketId }) {
  const [comments, setComments] = useState([])
  const [loading, setLoading] = useState(true)
  const [message, setMessage] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(() => {
    commentService
      .list(ticketId)
      .then((r) => setComments(r.data.data || []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [ticketId])

  useEffect(() => { load() }, [load])

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!message.trim()) return
    setSubmitting(true)
    setError('')
    try {
      await commentService.create(ticketId, { message: message.trim() })
      setMessage('')
      load()
    } catch (err) {
      setError(err.response?.data?.message || 'Failed to send comment')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="p-4 space-y-6">
      {/* Comments list */}
      <div className="space-y-3">
        {loading ? (
          <p className="text-sm text-gray-400 dark:text-gray-500 animate-pulse">Loading conversation…</p>
        ) : comments.length === 0 ? (
          <p className="text-sm text-gray-400 dark:text-gray-500">No comments yet. Start the conversation.</p>
        ) : (
          comments.map((c) => (
            <div key={c.id} className="rounded-lg border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 shadow-sm">
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-medium text-gray-700 dark:text-gray-200">
                  {c.user?.name || 'Unknown'}
                </span>
                <span className="text-xs text-gray-400 dark:text-gray-500">{formatDate(c.created_at)}</span>
              </div>
              <p className="text-sm text-gray-800 dark:text-gray-100 whitespace-pre-wrap">{c.message}</p>
            </div>
          ))
        )}
      </div>

      {/* Reply form */}
      <form onSubmit={handleSubmit} className="space-y-2">
        <label className="block text-xs font-semibold text-gray-500 dark:text-gray-400 dark:text-gray-500 uppercase tracking-wide">
          Add Comment
        </label>
        <textarea
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          placeholder="Write a comment…"
          rows={4}
          className="w-full rounded-lg border border-gray-200 dark:border-gray-600 px-3 py-2 text-sm focus:border-blue-400 focus:outline-none focus:ring-1 focus:ring-blue-400"
        />
        {error && <p className="text-xs text-red-500">{error}</p>}
        <button
          type="submit"
          disabled={submitting || !message.trim()}
          className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50 transition"
        >
          {submitting ? 'Sending…' : 'Send'}
        </button>
      </form>
    </div>
  )
}
