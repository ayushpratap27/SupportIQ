import { useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ticketService } from '../../services/ticketService'
import PriorityBadge from '../../components/tickets/PriorityBadge'
import { formatDate } from '../../utils/format'
import DarkModeToggle from '../../components/DarkModeToggle'

export default function UnassignedTickets() {
  const navigate = useNavigate()
  const [tickets, setTickets] = useState([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [takingId, setTakingId] = useState(null)
  const [message, setMessage] = useState('')

  const load = () => {
    setLoading(true)
    ticketService
      .getUnassigned({ limit: 50 })
      .then((r) => {
        const d = r.data.data
        setTickets(d.items || [])
        setTotal(d.total_count || 0)
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }

  useEffect(() => { load() }, [])

  const handleTakeOwnership = async (id, e) => {
    e.stopPropagation()
    setTakingId(id)
    setMessage('')
    try {
      await ticketService.takeOwnership(id)
      setMessage('Ticket assigned to you!')
      load()
    } catch (err) {
      setMessage(err.response?.data?.message || 'Failed to take ownership')
    } finally {
      setTakingId(null)
    }
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <header className="bg-white dark:bg-gray-800 border-b border-gray-100 dark:border-gray-700 px-6 py-4 flex justify-between items-center">
        <div className="flex items-center gap-3">
          <Link to="/dashboard" className="text-sm text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:text-gray-300 dark:text-gray-600">← Dashboard</Link>
          <span className="text-gray-300 dark:text-gray-600">/</span>
          <h1 className="text-sm font-semibold text-gray-700 dark:text-gray-200">Unassigned Tickets</h1>
        </div>
        <div className="flex items-center gap-3">
          {message && (
            <span className={`text-xs font-medium px-2 py-1 rounded-full ${
              message.includes('Failed') ? 'bg-red-100 text-red-600' : 'bg-green-100 text-green-600'
            }`}>
              {message}
            </span>
          )}
          <span className="text-xs text-gray-400 dark:text-gray-500">{total} unassigned</span>
          <button
            onClick={load}
            className="rounded-lg border border-gray-200 dark:border-gray-600 px-3 py-1.5 text-sm text-gray-600 dark:text-gray-300 dark:text-gray-600 hover:bg-gray-50 dark:bg-gray-900 transition"
          >
            Refresh
          </button>
        </div>
        <DarkModeToggle />
      </header>

      <main className="max-w-6xl mx-auto px-6 py-6">
        <div className="rounded-xl border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-sm overflow-hidden">
          {loading ? (
            <p className="p-6 text-sm text-gray-400 dark:text-gray-500 animate-pulse text-center">Loading…</p>
          ) : tickets.length === 0 ? (
            <div className="p-12 text-center">
              <p className="text-2xl mb-2">🎉</p>
              <p className="text-sm text-gray-400 dark:text-gray-500">No unassigned tickets!</p>
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-gray-50 dark:bg-gray-900 border-b border-gray-100 dark:border-gray-700">
                <tr>
                  {['Ticket #', 'Subject', 'Customer', 'Priority', 'Created', 'Action'].map((h) => (
                    <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-gray-500 dark:text-gray-400 dark:text-gray-500 uppercase tracking-wide">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-50">
                {tickets.map((t) => (
                  <tr
                    key={t.id}
                    onClick={() => navigate(`/tickets/${t.id}`)}
                    className="hover:bg-gray-50 dark:bg-gray-900 cursor-pointer transition"
                  >
                    <td className="px-4 py-3 font-mono text-xs text-blue-600">{t.ticket_number}</td>
                    <td className="px-4 py-3 font-medium text-gray-800 dark:text-gray-100 max-w-xs truncate">{t.subject}</td>
                    <td className="px-4 py-3 text-gray-500 dark:text-gray-400 dark:text-gray-500">{t.customer_name}</td>
                    <td className="px-4 py-3"><PriorityBadge priority={t.priority} /></td>
                    <td className="px-4 py-3 text-gray-400 dark:text-gray-500 text-xs">{formatDate(t.created_at)}</td>
                    <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                      <button
                        onClick={(e) => handleTakeOwnership(t.id, e)}
                        disabled={takingId === t.id}
                        className="rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-indigo-700 disabled:opacity-50 transition"
                      >
                        {takingId === t.id ? 'Taking…' : 'Take Ownership'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </main>
    </div>
  )
}
