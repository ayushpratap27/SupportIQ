import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { ticketService } from '../../services/ticketService'
import Toast, { useToast } from '../../components/Toast'

function CreateTicket() {
  const navigate = useNavigate()
  const { toast, showToast } = useToast()

  const [form, setForm] = useState({
    subject: '',
    description: '',
    customerName: '',
    customerEmail: '',
  })
  const [errors, setErrors] = useState({})
  const [submitting, setSubmitting] = useState(false)

  const handleChange = (e) => {
    setForm((prev) => ({ ...prev, [e.target.name]: e.target.value }))
    setErrors((prev) => ({ ...prev, [e.target.name]: '' }))
  }

  const validate = () => {
    const next = {}
    if (!form.subject.trim() || form.subject.trim().length < 5)
      next.subject = 'Subject must be at least 5 characters.'
    if (form.subject.length > 150)
      next.subject = 'Subject must not exceed 150 characters.'
    if (!form.description.trim())
      next.description = 'Description is required.'
    if (!form.customerName.trim())
      next.customerName = 'Customer name is required.'
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.customerEmail))
      next.customerEmail = 'Please enter a valid email address.'
    return next
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    const fieldErrors = validate()
    if (Object.keys(fieldErrors).length > 0) {
      setErrors(fieldErrors)
      return
    }

    setSubmitting(true)
    try {
      const res = await ticketService.createTicket(form)
      const ticket = res.data.data
      showToast(`Ticket ${ticket.ticket_number} created successfully`)
      setTimeout(() => navigate(`/tickets/${ticket.id}`), 1200)
    } catch (err) {
      showToast(err.response?.data?.message ?? 'Failed to create ticket', 'error')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <Toast toast={toast} />

      <header className="bg-white border-b border-gray-100 px-6 py-4 flex items-center gap-4">
        <Link to="/tickets" className="text-sm text-gray-500 hover:text-gray-700">← Tickets</Link>
        <h1 className="font-bold text-gray-800">New Ticket</h1>
      </header>

      <main className="max-w-2xl mx-auto px-6 py-8">
        <div className="bg-white rounded-2xl border border-gray-100 p-8">
          <form onSubmit={handleSubmit} className="space-y-5">
            {/* Subject */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1.5">Subject <span className="text-red-500">*</span></label>
              <input
                name="subject"
                value={form.subject}
                onChange={handleChange}
                placeholder="Brief description of the issue"
                className={`w-full px-4 py-2.5 border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition ${
                  errors.subject ? 'border-red-400' : 'border-gray-300'
                }`}
              />
              {errors.subject && <p className="mt-1 text-xs text-red-500">{errors.subject}</p>}
            </div>

            {/* Description */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1.5">Description <span className="text-red-500">*</span></label>
              <textarea
                name="description"
                value={form.description}
                onChange={handleChange}
                rows={4}
                placeholder="Detailed description of the issue"
                className={`w-full px-4 py-2.5 border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition resize-none ${
                  errors.description ? 'border-red-400' : 'border-gray-300'
                }`}
              />
              {errors.description && <p className="mt-1 text-xs text-red-500">{errors.description}</p>}
            </div>

            <div className="grid grid-cols-2 gap-4">
              {/* Customer Name */}
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1.5">Customer Name <span className="text-red-500">*</span></label>
                <input
                  name="customerName"
                  value={form.customerName}
                  onChange={handleChange}
                  placeholder="Rahul"
                  className={`w-full px-4 py-2.5 border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition ${
                    errors.customerName ? 'border-red-400' : 'border-gray-300'
                  }`}
                />
                {errors.customerName && <p className="mt-1 text-xs text-red-500">{errors.customerName}</p>}
              </div>

              {/* Customer Email */}
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1.5">Customer Email <span className="text-red-500">*</span></label>
                <input
                  name="customerEmail"
                  type="email"
                  value={form.customerEmail}
                  onChange={handleChange}
                  placeholder="rahul@example.com"
                  className={`w-full px-4 py-2.5 border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition ${
                    errors.customerEmail ? 'border-red-400' : 'border-gray-300'
                  }`}
                />
                {errors.customerEmail && <p className="mt-1 text-xs text-red-500">{errors.customerEmail}</p>}
              </div>
            </div>

            <div className="flex gap-3 pt-2">
              <button
                type="submit"
                disabled={submitting}
                className="flex-1 py-2.5 bg-blue-600 text-white font-semibold rounded-lg hover:bg-blue-700 transition disabled:opacity-50 disabled:cursor-not-allowed text-sm"
              >
                {submitting ? 'Creating…' : 'Create Ticket'}
              </button>
              <Link
                to="/tickets"
                className="px-5 py-2.5 border border-gray-300 text-gray-600 font-medium rounded-lg hover:bg-gray-50 transition text-sm text-center"
              >
                Cancel
              </Link>
            </div>
          </form>
        </div>
      </main>
    </div>
  )
}

export default CreateTicket
