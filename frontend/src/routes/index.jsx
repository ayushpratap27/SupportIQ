import { Routes, Route, Navigate } from 'react-router-dom'
import MainLayout from '../layouts/MainLayout'
import ProtectedRoute from '../components/ProtectedRoute'
import Home from '../pages/Home'
import Login from '../pages/Login'
import Register from '../pages/Register'
import Dashboard from '../pages/Dashboard'
import TicketList from '../pages/tickets/TicketList'
import CreateTicket from '../pages/tickets/CreateTicket'
import TicketDetails from '../pages/tickets/TicketDetails'
import EditTicket from '../pages/tickets/EditTicket'
import MyTickets from '../pages/tickets/MyTickets'
import UnassignedTickets from '../pages/tickets/UnassignedTickets'
import KnowledgeBase from '../pages/KnowledgeBase'
import JobMonitor from '../pages/JobMonitor'
import EmailAccounts from '../pages/EmailAccounts'
import EmailMonitor from '../pages/EmailMonitor'

function AppRoutes() {
  return (
    <Routes>
      <Route element={<MainLayout />}>
        <Route path="/" element={<Home />} />
      </Route>

      <Route path="/login" element={<Login />} />
      <Route path="/register" element={<Register />} />

      <Route element={<ProtectedRoute />}>
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/my-tickets" element={<MyTickets />} />
        <Route path="/tickets/unassigned" element={<UnassignedTickets />} />
        <Route path="/knowledge-base" element={<KnowledgeBase />} />
        <Route path="/jobs" element={<JobMonitor />} />
        <Route path="/email/accounts" element={<EmailAccounts />} />
        <Route path="/email/monitor" element={<EmailMonitor />} />
        <Route path="/tickets" element={<TicketList />} />
        <Route path="/tickets/new" element={<CreateTicket />} />
        <Route path="/tickets/:id" element={<TicketDetails />} />
        <Route path="/tickets/:id/edit" element={<EditTicket />} />
      </Route>

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export default AppRoutes
