import { useState, useEffect } from 'react'
import { healthService } from '../services/api'

// Possible values: 'loading' | 'online' | 'offline'
const STATUS = {
  LOADING: 'loading',
  ONLINE: 'online',
  OFFLINE: 'offline',
}

function Home() {
  const [backendStatus, setBackendStatus] = useState(STATUS.LOADING)

  useEffect(() => {
    const checkBackend = async () => {
      try {
        await healthService.check()
        setBackendStatus(STATUS.ONLINE)
      } catch {
        setBackendStatus(STATUS.OFFLINE)
      }
    }

    checkBackend()
  }, [])

  return (
    <div className="flex flex-col items-center justify-center min-h-screen gap-8">
      <h1 className="text-4xl font-bold text-gray-800 tracking-tight">
        AI Support Assistant
      </h1>

      <div className="bg-white rounded-xl shadow-md px-8 py-5 flex items-center gap-3">
        <span className="text-base font-medium text-gray-500">Backend Status</span>
        <span className="text-gray-300">|</span>

        {backendStatus === STATUS.LOADING && (
          <span className="text-gray-400 text-sm animate-pulse">Checking...</span>
        )}

        {backendStatus === STATUS.ONLINE && (
          <span className="text-green-600 font-semibold">🟢 Backend Connected</span>
        )}

        {backendStatus === STATUS.OFFLINE && (
          <span className="text-red-500 font-semibold">🔴 Backend Offline</span>
        )}
      </div>
    </div>
  )
}

export default Home
