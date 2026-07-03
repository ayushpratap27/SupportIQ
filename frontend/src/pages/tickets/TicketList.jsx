import { useState, useEffect, useCallback } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { ticketService } from '../../services/ticketService'
import StatusBadge from '../../components/tickets/StatusBadge'
import PriorityBadge from '../../components/tickets/PriorityBadge'
import Toast, { useToast } from '../../components/Toast'
import { formatDate } from '../../utils/format'

const STATUSES = ['OPEN', 'IN_PROGRESS', 'RESOLVED', 'CLOSED']
const PRIORITIES = ['LOW', 'MEDIUM', 'HIGH', 'URGENT']

function TicketList() {
  const navigate = useNavigate()
  const { toast, showToast } = useToast()

  const [tickets, setTickets] = useState([])
  const [meta, setMeta] = useState({ total_count: 0, current_page: 1, total_pages: 1 })
  const [loading, setLoading] = useState(true)

  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [priorityFilter, setPriorityFilter] = useState('')
  const [page, setPage] = useState(1)
  const limit = 20

  const fetchTickets = useCallback(async () => {
    setLoading(true)
    try {
      const res = await ticketService.getTickets({
        page,
        limit,
        search: search || undefined,
        status: statusFilter || undefined,
        priority: priorityFilter || undefined,
      })
      const data = res.data.data
      setTickets(data.items ?? [])
      setMeta(data)
    } catch {
      showToast('Failed to load tickets', 'error')
    } finally {
      setLoading(false)
    }
  }, [page, search, statusFilter, priorityFilter]) // eslint-disable-line

  useEffect(() => { fetchTickets() }, [fetchTickets])

  const handleSearchSubmit = (e) => {
    e.preventDefault()
    setPage(1)
    fetchTickets()
  }

  const handleFilterChange = (setter) => (e) => {
    setter(e.target.value)
    setPage(1)
  }

  return (
    <>
      <Toast toast={toast} />
      <main className="max-w-6xl mx-auto px-6 py-6 space-y-4">
        <div className="flex items-center justify-between">
          <h1 className="font-bold text-gray-800 dark:text-gray-100 text-lg">All Tickets</h1>
          <Link
            to="/tickets/new"
            className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 transition"
          >
            + New Ticket
          </Link>
        </div>
        {/* Filters */}
        <div className="flex flex-wrap gap-3">
          <form onSubmit={handleSearchSubmit} className="flex gap-2">
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search tickets…"
              className="rounded-lg border border-gray-200 dark:border-gray-600 px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-blue-400 w-56"
            />
            <button type="submit" className="rounded-lg bg-blue-600 px-3 py-2 text-sm text-white hover:bg-blue-700 transition">Search</button>
          </form>

          <select
            value={statusFilter}
            onChange={handleFilterChange(setStatusFilter)}
            className="select-field"
          >
            <option value="">All Statuses</option>
            {STATUSES.map((s) => <option key={s} value={s}>{s.replace('_', ' ')}</option>)}
          </select>

          <select
            value={priorityFilter}
            onChange={handleFilterChange(setPriorityFilter)}
            className="select-field"
          >
            <option value="">All Priorities</option>
            {PRIORITIES.map((p) => <option key={p} value={p}>{p}</option>)}
          </select>

          {(search || statusFilter || priorityFilter) && (
            <button
              onClick={() => { setSearch(''); setStatusFilter(''); setPriorityFilter(''); setPage(1) }}
              className="text-sm text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:text-gray-300 underline"
            >
              Clear
            </button>
          )}
        </div>

        {/* Table */}
        <div className="rounded-xl border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-sm overflow-hidden">
          {loading ? (
            <p className="p-6 text-sm text-gray-400 dark:text-gray-500 animate-pulse text-center">Loading…</p>
          ) : tickets.length === 0 ? (
            <p className="p-6 text-sm text-gray-400 dark:text-gray-500 text-center">No tickets found.</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 text-left text-xs font-semibold text-gray-500 dark:text-gray-400 dark:text-gray-500 uppercase tracking-wide">
                    <th className="px-4 py-3">Ticket #</th>
                    <th className="px-4 py-3">Subject</th>
                    <th className="px-4 py-3">Customer</th>
                    <th className="px-4 py-3">Priority</th>
                    <th className="px-4 py-3">Status</th>
                    <th className="px-4 py-3">Assigned To</th>
                    <th className="px-4 py-3">Created</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-50">
                  {tickets.map((t) => (
                    <tr
                      key={t.id}
                      onClick={() => navigate(`/tickets/${t.id}`)}
                      className="hover:bg-gray-50 dark:bg-gray-900 cursor-pointer transition"
                    >
                      <td className="px-4 py-3 font-mono text-xs text-blue-600 font-medium">{t.ticket_number}</td>
                      <td className="px-4 py-3 max-w-[200px] truncate text-gray-800 dark:text-gray-100 font-medium">{t.subject}</td>
                      <td className="px-4 py-3 text-gray-600 dark:text-gray-300 dark:text-gray-600">
                        <div>{t.customer_name}</div>
                        <div className="text-xs text-gray-400 dark:text-gray-500">{t.customer_email}</div>
                      </td>
                      <td className="px-4 py-3"><PriorityBadge priority={t.priority} /></td>
                      <td className="px-4 py-3"><StatusBadge status={t.status} /></td>
                      <td className="px-4 py-3 text-gray-500 dark:text-gray-400 dark:text-gray-500 text-xs">{t.assignee?.name ?? '—'}</td>
                      <td className="px-4 py-3 text-gray-400 dark:text-gray-500 text-xs">{formatDate(t.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Pagination */}
        {meta.total_pages > 1 && (
          <div className="flex items-center justify-between">
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1}
              className="rounded-lg border border-gray-200 dark:border-gray-600 px-3 py-1.5 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800 disabled:opacity-40 transition"
            >
              ← Prev
            </button>
            <span className="text-sm text-gray-400 dark:text-gray-500">Page {page} of {meta.total_pages}</span>
            <button
              onClick={() => setPage((p) => Math.min(meta.total_pages, p + 1))}
              disabled={page === meta.total_pages}
              className="rounded-lg border border-gray-200 dark:border-gray-600 px-3 py-1.5 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800 disabled:opacity-40 transition"
            >
              Next →
            </button>
          </div>
        )}
      </main>
    </>
  )
}

export default TicketList
