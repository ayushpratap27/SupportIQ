import { BrowserRouter } from 'react-router-dom'
import { AuthProvider } from './contexts/AuthContext'
import { WebSocketProvider } from './contexts/WebSocketContext'
import RealtimeToast from './components/RealtimeToast'
import AppRoutes from './routes'

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <WebSocketProvider>
          <AppRoutes />
          <RealtimeToast />
        </WebSocketProvider>
      </AuthProvider>
    </BrowserRouter>
  )
}

export default App
