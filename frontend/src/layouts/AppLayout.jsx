import { Navigate, Outlet } from 'react-router-dom'
import { useAuth } from '../contexts/AuthContext'
import Sidebar from '../components/Sidebar'
import DarkModeToggle from '../components/DarkModeToggle'

export default function AppLayout() {
  const { user, loading } = useAuth()

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex items-center justify-center">
        <p className="text-sm text-gray-400 dark:text-gray-500 animate-pulse">Loading…</p>
      </div>
    )
  }

  if (!user) return <Navigate to="/login" replace />

  return (
    <div className="flex min-h-screen bg-gray-50 dark:bg-gray-900">
      <Sidebar />
      <div className="flex-1 min-w-0 overflow-auto">
        <div className="fixed top-3 right-4 z-50">
          <DarkModeToggle />
        </div>
        <Outlet />
      </div>
    </div>
  )
}
