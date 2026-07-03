import { useState, useEffect, useCallback } from 'react'
import { PieChart, Pie, Cell, BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts'
import analyticsService from '../../services/analyticsService'
import useWebSocket from '../../hooks/useWebSocket'
import DarkModeToggle from '../../components/DarkModeToggle'

const STATUS_COLORS = {
  QUEUED: '#F59E0B',
  PROCESSING: '#3B82F6',
  COMPLETED: '#10B981',
  FAILED: '#EF4444',
  DEAD: '#6B7280',
  RETRYING: '#8B5CF6',
}

function StatusBadge({ status, count }) {
  const color = STATUS_COLORS[status] ?? '#6B7280'
  return (
    <div className="rounded-xl border p-4 text-center" style={{ borderColor: color + '40', background: color + '10' }}>
      <p className="text-xs font-semibold uppercase tracking-wide" style={{ color }}>{status}</p>
      <p className="mt-2 text-3xl font-bold" style={{ color }}>{count?.toLocaleString() ?? '0'}</p>
    </div>
  )
}

export default function QueueMonitoring() {
  const [data, setData] = useState(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await analyticsService.getQueueStats()
      setData(res.data.data)
    } catch (e) {
      console.error(e)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  // Auto-refresh every 15 seconds for live queue data
  useEffect(() => {
    const t = setInterval(load, 15_000)
    return () => clearInterval(t)
  }, [load])

  // Also refresh on WebSocket analytics events
  useWebSocket((msg) => {
    if (msg?.type === 'ANALYTICS_REFRESH') load()
  })

  const pieData = data
    ? [
        { name: 'Queued', value: Number(data.total_queued) },
        { name: 'Processing', value: Number(data.total_processing) },
        { name: 'Completed', value: Number(data.total_completed) },
        { name: 'Failed', value: Number(data.total_failed) },
        { name: 'Dead', value: Number(data.total_dead) },
        { name: 'Retrying', value: Number(data.total_retrying) },
      ].filter(d => d.value > 0)
    : []

  const jobTypeData = (data?.by_job_type ?? []).map(j => ({
    name: j.label?.replace(/_/g, ' '),
    count: Number(j.count),
  }))

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Queue Monitoring</h1>
          <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5">Auto-refreshes every 15 seconds</p>
        </div>
        <button
          onClick={load}
          className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
        >
          Refresh Now
        </button>
      </div>

      {loading && !data ? (
        <div className="flex h-64 items-center justify-center">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-blue-600 border-t-transparent" />
        </div>
      ) : (
        <>
          {/* Status badges */}
          <div className="grid grid-cols-3 md:grid-cols-6 gap-3 mb-6">
            <StatusBadge status="QUEUED" count={data?.total_queued} />
            <StatusBadge status="PROCESSING" count={data?.total_processing} />
            <StatusBadge status="COMPLETED" count={data?.total_completed} />
            <StatusBadge status="FAILED" count={data?.total_failed} />
            <StatusBadge status="RETRYING" count={data?.total_retrying} />
            <StatusBadge status="DEAD" count={data?.total_dead} />
          </div>

          {/* Avg queue time */}
          <div className="mb-6 rounded-xl border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 p-5 flex items-center gap-8">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400 dark:text-gray-500">Avg Queue Wait</p>
              <p className="mt-1 text-4xl font-bold text-blue-700">
                {data?.avg_queue_seconds != null ? `${data.avg_queue_seconds}s` : '—'}
              </p>
              <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5">last 24 hours</p>
            </div>

            {(data?.total_failed > 0 || data?.total_dead > 0) && (
              <div className="flex-1 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
                ⚠️ <strong>{data.total_failed}</strong> failed &amp; <strong>{data.total_dead}</strong> dead letter jobs require attention.
                Use the <a href="/jobs" className="underline">Job Monitor</a> to inspect and retry.
              </div>
            )}
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Status distribution */}
            <div className="rounded-xl border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 p-5">
              <h2 className="mb-4 text-sm font-semibold text-gray-700 dark:text-gray-200">Job Status Distribution</h2>
              {pieData.length > 0 ? (
                <ResponsiveContainer width="100%" height={240}>
                  <PieChart>
                    <Pie data={pieData} cx="50%" cy="50%" innerRadius={55} outerRadius={90}
                      dataKey="value" nameKey="name"
                      label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                      labelLine={false} fontSize={11}>
                      {pieData.map((d) => (
                        <Cell key={d.name} fill={STATUS_COLORS[d.name.toUpperCase()] ?? '#6B7280'} />
                      ))}
                    </Pie>
                    <Tooltip />
                    <Legend />
                  </PieChart>
                </ResponsiveContainer>
              ) : <p className="py-16 text-center text-sm text-gray-400 dark:text-gray-500">No jobs found</p>}
            </div>

            {/* Jobs by type */}
            <div className="rounded-xl border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 p-5">
              <h2 className="mb-4 text-sm font-semibold text-gray-700 dark:text-gray-200">Jobs by Type</h2>
              {jobTypeData.length > 0 ? (
                <ResponsiveContainer width="100%" height={240}>
                  <BarChart data={jobTypeData} layout="vertical">
                    <CartesianGrid strokeDasharray="3 3" horizontal={false} />
                    <XAxis type="number" tick={{ fontSize: 11 }} allowDecimals={false} />
                    <YAxis type="category" dataKey="name" tick={{ fontSize: 10 }} width={130} />
                    <Tooltip />
                    <Bar dataKey="count" name="Jobs" fill="#3B82F6" radius={[0, 4, 4, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              ) : <p className="py-16 text-center text-sm text-gray-400 dark:text-gray-500">No job type data</p>}
            </div>
          </div>
        </>
      )}
    </div>
  )
}
