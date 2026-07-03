import { useState, useEffect } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useAuth } from '../../contexts/AuthContext'
import { ticketService } from '../../services/ticketService'
import { useToast } from '../../components/Toast'
import StatusBadge from '../../components/tickets/StatusBadge'
import PriorityBadge from '../../components/tickets/PriorityBadge'
import ActivityTimeline from '../../components/tickets/ActivityTimeline'
import NotesPanel from '../../components/tickets/NotesPanel'
import ConversationPanel from '../../components/tickets/ConversationPanel'
import AIAnalysisPanel from '../../components/tickets/AIAnalysisPanel'
import AIReplyPanel from '../../components/tickets/AIReplyPanel'
import { formatDate } from '../../utils/format'

const TABS = ['Overview', 'Conversation', 'Notes', 'Activity', 'AI Analysis', 'AI Reply']

const NEXT_STATUS = {
  OPEN: 'IN_PROGRESS',
  IN_PROGRESS: 'RESOLVED',
  RESOLVED: 'CLOSED',
}

const NEXT_STATUS_LABEL = {
  OPEN: 'Start Progress',
  IN_PROGRESS: 'Mark Resolved',
  RESOLVED: 'Close Ticket',
}

export default function TicketDetails() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const { toast, showToast } = useToast()

  const [ticket, setTicket] = useState(null)
  const [agents, setAgents] = useState([])
  const [loading, setLoading] = useState(true)
  const [activeTab, setActiveTab] = useState('Overview')
  const [statusLoading, setStatusLoading] = useState(false)
  const [assignLoading, setAssignLoading] = useState(false)

  const isAdmin = user?.role === 'Admin'

  const load = () => {
    const calls = [ticketService.getTicket(id)]
    if (isAdmin) calls.push(ticketService.getAgents())

    Promise.all(calls)
      .then(([tRes, aRes]) => {
        setTicket(tRes.data.data)
        if (aRes) setAgents(aRes.data.data || [])
      })
      .catch(() => showToast('Failed to load ticket', 'error'))
      .finally(() => setLoading(false))
  }

  useEffect(() => { load() }, [id])

  const handleStatusUpdate = async () => {
    const next = NEXT_STATUS[ticket.status]
    if (!next) return
    setStatusLoading(true)
    try {
      const r = await ticketService.updateStatus(id, next)
      setTicket(r.data.data)
      showToast(`Status updated to ${next}`)
    } catch (err) {
      showToast(err.response?.data?.message || 'Failed to update status', 'error')
    } finally {
      setStatusLoading(false)
    }
  }

  const handleAssign = async (e) => {
    const agentId = Number(e.target.value)
    if (!agentId) return
    setAssignLoading(true)
    try {
      const r = await ticketService.assignTicket(id, agentId)
      setTicket(r.data.data)
      showToast('Ticket assigned')
    } catch (err) {
      showToast(err.response?.data?.message || 'Failed to assign', 'error')
    } finally {
      setAssignLoading(false)
    }
  }

  const handleDelete = async () => {
    if (!window.confirm('Delete this ticket? This cannot be undone.')) return
    try {
      await ticketService.deleteTicket(id)
      showToast('Ticket deleted')
      setTimeout(() => navigate('/tickets'), 800)
    } catch (err) {
      showToast(err.response?.data?.message || 'Failed to delete', 'error')
    }
  }

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <p className="text-gray-400 animate-pulse">Loading ticket…</p>
      </div>
    )
  }

  if (!ticket) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <p className="text-gray-500">Ticket not found.</p>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="bg-white border-b border-gray-100 px-6 py-4 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Link to="/tickets" className="text-sm text-gray-400 hover:text-gray-600">← Tickets</Link>
          <span className="text-gray-300">/</span>
          <span className="text-sm font-mono font-medium text-gray-700">{ticket.ticket_number}</span>
        </div>
        <div className="flex items-center gap-2">
          <Link
            to={`/tickets/${id}/edit`}
            className="rounded-lg border border-gray-200 px-3 py-1.5 text-sm font-medium text-gray-600 hover:bg-gray-50 transition"
          >
            Edit
          </Link>
          {NEXT_STATUS[ticket.status] && (
            <button
              onClick={handleStatusUpdate}
              disabled={statusLoading}
              className="rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50 transition"
            >
              {statusLoading ? 'Updating…' : NEXT_STATUS_LABEL[ticket.status]}
            </button>
          )}
          {isAdmin && (
            <button
              onClick={handleDelete}
              className="rounded-lg border border-red-200 px-3 py-1.5 text-sm font-medium text-red-500 hover:bg-red-50 transition"
            >
              Delete
            </button>
          )}
        </div>
      </header>

      {/* Title bar */}
      <div className="bg-white border-b border-gray-100 px-6 py-4">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h1 className="text-lg font-bold text-gray-900">{ticket.subject}</h1>
            <div className="mt-1 flex items-center gap-2">
              <StatusBadge status={ticket.status} />
              <PriorityBadge priority={ticket.priority} />
            </div>
          </div>
        </div>

        {/* Tabs */}
        <div className="mt-4 flex gap-1 border-b border-gray-100 -mb-[1px]">
          {TABS.map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition ${
                activeTab === tab
                  ? 'border-blue-600 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700'
              }`}
            >
              {tab}
            </button>
          ))}
        </div>
      </div>

      {/* Tab content */}
      <div className="max-w-5xl mx-auto px-6 py-6">
        {activeTab === 'Overview' && (
          <div className="grid grid-cols-3 gap-6">
            {/* Main content */}
            <div className="col-span-2 space-y-4">
              <div className="rounded-xl border border-gray-100 bg-white p-5 shadow-sm">
                <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">Description</h3>
                <p className="text-sm text-gray-800 whitespace-pre-wrap">{ticket.description}</p>
              </div>

              <div className="rounded-xl border border-gray-100 bg-white p-5 shadow-sm">
                <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">Customer</h3>
                <dl className="space-y-2">
                  <div className="flex gap-3">
                    <dt className="w-16 text-xs text-gray-400 shrink-0">Name</dt>
                    <dd className="text-sm text-gray-800 font-medium">{ticket.customer_name}</dd>
                  </div>
                  <div className="flex gap-3">
                    <dt className="w-16 text-xs text-gray-400 shrink-0">Email</dt>
                    <dd className="text-sm text-gray-800">{ticket.customer_email}</dd>
                  </div>
                </dl>
              </div>
            </div>

            {/* Sidebar */}
            <div className="space-y-4">
              <div className="rounded-xl border border-gray-100 bg-white p-5 shadow-sm space-y-3">
                <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wide">Details</h3>
                <dl className="space-y-2.5 text-sm">
                  <div className="flex justify-between">
                    <dt className="text-gray-400">Priority</dt>
                    <dd><PriorityBadge priority={ticket.priority} /></dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-gray-400">Category</dt>
                    <dd className="text-gray-700 font-medium">{ticket.category}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-gray-400">Source</dt>
                    <dd className="text-gray-700">{ticket.source}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-gray-400">Created by</dt>
                    <dd className="text-gray-700">{ticket.creator?.name || '—'}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-gray-400">Assigned to</dt>
                    <dd className="text-gray-700">{ticket.assignee?.name || 'Unassigned'}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-gray-400">Created</dt>
                    <dd className="text-gray-700">{formatDate(ticket.created_at)}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-gray-400">Updated</dt>
                    <dd className="text-gray-700">{formatDate(ticket.updated_at)}</dd>
                  </div>
                </dl>
              </div>

              {/* Admin assignment */}
              {isAdmin && (
                <div className="rounded-xl border border-gray-100 bg-white p-5 shadow-sm">
                  <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">Assign</h3>
                  <select
                    onChange={handleAssign}
                    disabled={assignLoading}
                    value={ticket.assigned_to || ''}
                    className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-blue-400"
                  >
                    <option value="">Unassigned</option>
                    {agents.map((a) => (
                      <option key={a.id} value={a.id}>{a.name}</option>
                    ))}
                  </select>
                </div>
              )}
            </div>
          </div>
        )}

        {activeTab === 'Conversation' && (
          <div className="max-w-2xl">
            <ConversationPanel ticketId={id} />
          </div>
        )}

        {activeTab === 'Notes' && (
          <div className="max-w-2xl">
            <NotesPanel ticketId={id} />
          </div>
        )}

        {activeTab === 'Activity' && (
          <div className="max-w-2xl">
            <ActivityTimeline ticketId={id} />
          </div>
        )}

        {activeTab === 'AI Analysis' && (
          <div className="max-w-2xl">
            <AIAnalysisPanel ticketId={id} />
          </div>
        )}

        {activeTab === 'AI Reply' && (
          <div className="max-w-2xl">
            <AIReplyPanel ticketId={id} />
          </div>
        )}
      </div>

      {toast && (
        <div className={`fixed top-4 right-4 z-50 rounded-lg px-4 py-3 text-sm font-medium shadow-lg ${
          toast.type === 'error' ? 'bg-red-500 text-white' : 'bg-gray-900 text-white'
        }`}>
          {toast.message}
        </div>
      )}
    </div>
  )
}
