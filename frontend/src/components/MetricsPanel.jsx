import { useState, useEffect, useCallback } from 'react'
import './MetricsPanel.css'
import LLMInsight from './LLMInsight'
import './LLMInsight.css'

function MetricsPanel({ messages, filterStatus, onFilterChange }) {
  const [queueStatus, setQueueStatus] = useState(null)

  const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

  useEffect(() => {
    fetchMetrics()
    const interval = setInterval(fetchMetrics, 3000)
    return () => clearInterval(interval)
  }, [])

  const fetchMetrics = async () => {
    try {
      const res = await fetch(`${API_URL}/api/v1/queue/status`)
      if (res.ok) {
        const data = await res.json()
        setQueueStatus(data.queues || [])
      }
    } catch (err) {
      console.error('Failed to fetch metrics:', err)
    }
  }

  const countByStatus = (status) => messages.filter(m => m.status === status).length

  const getQueueDepth = (queueName) => {
    if (!queueStatus) return '—'
    const q = queueStatus.find(q => q.name === queueName)
    return q ? q.messages : 0
  }

  const failed = countByStatus('failed')
  const [insightOpen, setInsightOpen] = useState(false)
  const [insightKey, setInsightKey] = useState(0)
  const [metricsOpen, setMetricsOpen] = useState(false)
  const [metricsKey, setMetricsKey] = useState(0)

  const openInsight = useCallback(() => {
    setInsightKey(k => k + 1)
    setInsightOpen(true)
  }, [])

  const openMetrics = useCallback(() => {
    setMetricsKey(k => k + 1)
    setMetricsOpen(true)
  }, [])

  const handleFilter = (status) => {
    if (onFilterChange) onFilterChange(filterStatus === status ? null : status)
  }

  return (
    <>
    <div className="stats-bar">
      {/* Message flow */}
      <StatCard label="Total" value={messages.length} />
      <StatCard label="Pending"   value={countByStatus('pending')}   accent="#ff9800" isActive={filterStatus==='pending'}   onFilter={() => handleFilter('pending')} />
      <StatCard label="Sent"      value={countByStatus('sent')}      accent="#2196f3" isActive={filterStatus==='sent'}      onFilter={() => handleFilter('sent')} />
      <StatCard label="Received"  value={countByStatus('received')}  accent="#4caf50" isActive={filterStatus==='received'}  onFilter={() => handleFilter('received')} />
      <StatCard label="Processed" value={countByStatus('processed')} accent="#00bcd4" isActive={filterStatus==='processed'} onFilter={() => handleFilter('processed')} />
      <StatCard label="Failed"    value={failed}                     accent="#f44336" alert={failed > 0}
        isActive={filterStatus==='failed'}
        onFilter={() => handleFilter('failed')}
        onInsight={failed > 0 ? openInsight : null} />

      <div className="stats-divider" />

      {/* Queue depths */}
      <StatCard
        label="Send Q"
        value={getQueueDepth('messages.send')}
        accent="#9c27b0"
        sub={`${getQueueDepth('messages.send.dlq')} DLQ`}
      />
      <StatCard
        label="Receive Q"
        value={getQueueDepth('messages.receive')}
        accent="#9c27b0"
        sub={`${getQueueDepth('messages.receive.dlq')} DLQ`}
      />

      <div className="stats-divider" />

      {/* System health */}
      <div className="stat-card stat-card--health">
        <span className="stat-dot" />
        <span className="stat-label">System OK</span>
      </div>

      {/* Metrics analysis */}
      <button className="stat-analyze-btn" onClick={openMetrics} title="AI metrics analysis">
        📊 Analyze
      </button>
    </div>

    {insightOpen && (
      <div className="stats-insight-float">
        <div className="stats-insight-header">
          <span className="stats-insight-title">System Health Insight</span>
          <button className="stats-insight-close" onClick={() => setInsightOpen(false)}>✕</button>
        </div>
        <LLMInsight key={insightKey} type="health_insight" />
      </div>
    )}

    {metricsOpen && (
      <div className="stats-insight-float stats-insight-float--metrics">
        <div className="stats-insight-header">
          <span className="stats-insight-title" style={{ color: '#7c4dff' }}>📊 Metrics Analysis</span>
          <button className="stats-insight-close" onClick={() => setMetricsOpen(false)}>✕</button>
        </div>
        <LLMInsight key={metricsKey} type="metrics_analysis" />
      </div>
    )}
    </>
  )
}

function StatCard({ label, value, accent, sub, alert, isActive, onFilter, onInsight }) {
  return (
    <div
      className={`stat-card${alert ? ' stat-card--alert' : ''}${isActive ? ' stat-card--active' : ''}${onFilter ? ' stat-card--clickable' : ''}`}
      style={accent ? { '--card-accent': accent } : {}}
      onClick={onFilter || undefined}
      role={onFilter ? 'button' : undefined}
      tabIndex={onFilter ? 0 : undefined}
      onKeyDown={onFilter ? (e) => { if (e.key === 'Enter' || e.key === ' ') onFilter() } : undefined}
    >
      <div className="stat-label">{label}</div>
      <div className="stat-value">{value}</div>
      {sub && <div className="stat-sub">{sub}</div>}
      {onInsight && (
        <button
          className="stat-insight-btn"
          onClick={(e) => { e.stopPropagation(); onInsight() }}
          title="AI health insight"
        >💡</button>
      )}
    </div>
  )
}

export default MetricsPanel
