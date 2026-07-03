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

function AppRoutes() {
  return (
    <Routes>
      {/* Public routes wrapped in the shared page shell */}
      <Route element={<MainLayout />}>
        <Route path="/" element={<Home />} />
      </Route>

      {/* Auth pages — manage their own full-screen layout */}
      <Route path="/login" element={<Login />} />
      <Route path="/register" element={<Register />} />

      {/* Protected routes — redirect to /login if unauthenticated */}
      <Route element={<ProtectedRoute />}>
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/my-tickets" element={<MyTickets />} />
        <Route path="/tickets/unassigned" element={<UnassignedTickets />} />
        <Route path="/tickets" element={<TicketList />} />
        <Route path="/tickets/new" element={<CreateTicket />} />
        <Route path="/tickets/:id" element={<TicketDetails />} />
        <Route path="/tickets/:id/edit" element={<EditTicket />} />
      </Route>

      {/* Catch-all */}
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export default AppRoutes
