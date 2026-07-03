import { useState, useEffect } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useAuth } from '../contexts/AuthContext'
import { authService } from '../services/authService'
import { ticketService } from '../services/ticketService'
import { activityService } from '../services/activityService'
import { formatDate } from '../utils/format'

const ACTIVITY_ICON = {
  CREATE_TICKET: '🎫',
  UPDATE_TICKET: '✏️',
  ASSIGN_TICKET: '👤',
  TAKE_OWNERSHIP: '✋',
  STATUS_CHANGED: '🔄',
  PRIORITY_CHANGED: '⚡',
  CATEGORY_CHANGED: '🏷️',
  COMMENT_ADDED: '💬',
  INTERNAL_NOTE_ADDED: '📝',
  TICKET_CLOSED: '✅',
}

function StatCard({ label, value, color, to }) {
  const inner = (
    <div className={`rounded-xl border p-4 ${color}`}>
      <p className="text-2xl font-bold">{value === null ? '…' : value}</p>
      <p className="text-xs font-medium mt-1 opacity-80">{label}</p>
    </div>
  )
  return to ? <Link to={to}>{inner}</Link> : inner
}

export default function Dashboard() {
  const { user: ctxUser, logout } = useAuth()
  const navigate = useNavigate()
  const [user, setUser] = useState(ctxUser)
  const [stats, setStats] = useState({
    total: null, myTickets: null, open: null, unassigned: null,
    inProgress: null, resolved: null, closed: null,
  })
  const [recentActivity, setRecentActivity] = useState([])
  const [recentMine, setRecentMine] = useState([])

  useEffect(() => {
    authService.getMe().then((r) => setUser(r.data.data)).catch(() => {})

    Promise.all([
      ticketService.getTickets({ limit: 1 }),
      ticketService.getMyTickets({ limit: 5 }),
      ticketService.getTickets({ limit: 1, status: 'OPEN' }),
      ticketService.getUnassigned({ limit: 1 }),
      ticketService.getTickets({ limit: 1, status: 'IN_PROGRESS' }),
      ticketService.getTickets({ limit: 1, status: 'RESOLVED' }),
      ticketService.getTickets({ limit: 1, status: 'CLOSED' }),
      activityService.getRecent(),
    ])
      .then(([total, mine, open, unassigned, inProg, resolved, closed, activity]) => {
        setStats({
          total:      total.data.data.total_count,
          myTickets:  mine.data.data.total_count,
          open:       open.data.data.total_count,
          unassigned: unassigned.data.data.total_count,
          inProgress: inProg.data.data.total_count,
          resolved:   resolved.data.data.total_count,
          closed:     closed.data.data.total_count,
        })
        setRecentActivity(activity.data.data || [])
        setRecentMine(mine.data.data.items || [])
      })
      .catch(() => {})
  }, [])

  const handleLogout = async () => {
    await logout()
    navigate('/login')
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-100 px-6 py-4 flex justify-between items-center">
        <span className="font-bold text-gray-800">AI Support Assistant</span>
        <div className="flex items-center gap-4">
          <Link to="/tickets/unassigned" className="text-sm text-indigo-600 font-medium hover:underline">Unassigned</Link>
          <Link to="/my-tickets" className="text-sm text-purple-600 font-medium hover:underline">My Tickets</Link>
          <Link to="/tickets" className="text-sm text-blue-600 font-medium hover:underline">All Tickets</Link>
          {user?.role === 'Admin' && (
            <>
              <Link to="/knowledge-base" className="text-sm text-emerald-600 font-medium hover:underline">Knowledge Base</Link>
              <Link to="/jobs" className="text-sm text-violet-600 font-medium hover:underline">Job Monitor</Link>
              <Link to="/email/accounts" className="text-sm text-orange-600 font-medium hover:underline">Email Accounts</Link>
              <Link to="/email/monitor" className="text-sm text-cyan-600 font-medium hover:underline">Email Monitor</Link>
            </>
          )}
          <button onClick={handleLogout} className="text-sm text-red-500 hover:text-red-600 font-medium transition">Logout</button>
        </div>
      </header>

      <main className="max-w-6xl mx-auto px-6 py-8 space-y-8">
        {/* Stat cards */}
        <div>
          <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">Overview</h2>
          <div className="grid grid-cols-7 gap-3">
            <StatCard label="Total"       value={stats.total}      color="bg-blue-50   text-blue-700   border-blue-100"   to="/tickets" />
            <StatCard label="My Tickets"  value={stats.myTickets}  color="bg-purple-50 text-purple-700 border-purple-100" to="/my-tickets" />
            <StatCard label="Open"        value={stats.open}       color="bg-amber-50  text-amber-700  border-amber-100"  />
            <StatCard label="Unassigned"  value={stats.unassigned} color="bg-red-50    text-red-600    border-red-100"    to="/tickets/unassigned" />
            <StatCard label="In Progress" value={stats.inProgress} color="bg-violet-50 text-violet-700 border-violet-100" />
            <StatCard label="Resolved"    value={stats.resolved}   color="bg-green-50  text-green-700  border-green-100"  />
            <StatCard label="Closed"      value={stats.closed}     color="bg-gray-50   text-gray-600   border-gray-100"   />
          </div>
        </div>

        <div className="grid grid-cols-3 gap-6">
          {/* Recent Activity */}
          <div className="col-span-2 bg-white rounded-2xl shadow-sm border border-gray-100 p-6">
            <h2 className="text-sm font-bold text-gray-900 mb-4">Recent Activity</h2>
            {recentActivity.length === 0 ? (
              <p className="text-sm text-gray-400">No activity yet.</p>
            ) : (
              <ul className="space-y-3">
                {recentActivity.slice(0, 10).map((a) => (
                  <li key={a.id} className="flex items-start gap-3">
                    <span className="text-base">{ACTIVITY_ICON[a.activity_type] || '•'}</span>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm text-gray-800 truncate">{a.description}</p>
                      <p className="text-xs text-gray-400">{a.user?.name} · {formatDate(a.created_at)}</p>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>

          {/* Right column */}
          <div className="space-y-4">
            {/* User info */}
            <div className="bg-white rounded-2xl shadow-sm border border-gray-100 p-6">
              <h2 className="text-sm font-bold text-gray-900 mb-4">Account</h2>
              <dl className="space-y-3">
                <div className="flex items-center gap-3">
                  <dt className="w-14 text-xs text-gray-400 shrink-0">Name</dt>
                  <dd className="text-sm font-medium text-gray-800 truncate">{user?.name}</dd>
                </div>
                <div className="flex items-center gap-3">
                  <dt className="w-14 text-xs text-gray-400 shrink-0">Role</dt>
                  <dd><span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-700">{user?.role}</span></dd>
                </div>
                <div className="flex items-center gap-3">
                  <dt className="w-14 text-xs text-gray-400 shrink-0">Status</dt>
                  <dd className="text-xs text-green-600 font-medium flex items-center gap-1">
                    <span className="w-1.5 h-1.5 rounded-full bg-green-500 inline-block" />Active
                  </dd>
                </div>
              </dl>
            </div>

            {/* Recent assigned to me */}
            {recentMine.length > 0 && (
              <div className="bg-white rounded-2xl shadow-sm border border-gray-100 p-6">
                <h2 className="text-sm font-bold text-gray-900 mb-4">My Recent Tickets</h2>
                <ul className="space-y-2">
                  {recentMine.slice(0, 5).map((t) => (
                    <li key={t.id}>
                      <Link
                        to={`/tickets/${t.id}`}
                        className="flex items-center justify-between hover:bg-gray-50 rounded-lg p-1 -mx-1 transition"
                      >
                        <span className="font-mono text-xs text-blue-600">{t.ticket_number}</span>
                        <span className="text-xs text-gray-500 truncate max-w-[120px] ml-2">{t.subject}</span>
                      </Link>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        </div>
      </main>
    </div>
  )
}
