import { useState, useEffect, useRef } from 'react'
import './LLMInsight.css'

const BADGE_COLORS = {
  malformed_content: '#ff9800',
  schema_mismatch: '#f44336',
  duplicate: '#2196f3',
  partner_config: '#9c27b0',
  unknown: '#757575',
}

function LLMInsight({ type, inputRef, onDone }) {
  const [job, setJob] = useState(null)
  const [error, setError] = useState(null)
  const pollRef = useRef(null)
  const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

  useEffect(() => {
    createJob()
    return () => {
      if (pollRef.current) clearInterval(pollRef.current)
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const createJob = async () => {
    try {
      const body = { type }
      if (inputRef) body.input_ref = inputRef
      const res = await fetch(`${API_URL}/api/v1/llm/jobs`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data = await res.json()
      fetchJob(data.job_id)
      pollRef.current = setInterval(() => fetchJob(data.job_id), 2000)
    } catch (err) {
      setError(err.message)
    }
  }

  const fetchJob = async (id) => {
    try {
      const res = await fetch(`${API_URL}/api/v1/llm/jobs/${id}`)
      if (!res.ok) return
      const data = await res.json()
      setJob(data)
      if (['done', 'error', 'timeout'].includes(data.status)) {
        clearInterval(pollRef.current)
        if (data.status === 'done' && onDone) onDone(data)
      }
    } catch (err) {
      console.error('LLM poll error', err)
    }
  }

  if (error) {
    return <div className="llm-insight llm-insight--error">AI analysis unavailable</div>
  }

  if (!job || job.status === 'pending' || job.status === 'running') {
    return (
      <div className="llm-insight llm-insight--loading">
        <span className="llm-spinner" />
        <span>Analyzing…</span>
      </div>
    )
  }

  if (job.status === 'error' || job.status === 'timeout') {
    return <div className="llm-insight llm-insight--error">AI analysis unavailable</div>
  }

  // ── done ─────────────────────────────────────────────────────────────────
  if (type === 'classify_failure' && job.result) {
    const { category = 'unknown', confidence = 'low', explanation = '' } = job.result
    const color = BADGE_COLORS[category] || BADGE_COLORS.unknown
    return (
      <div className="llm-insight llm-insight--classify">
        <span className="llm-badge" style={{ background: color }}>
          {category.replace(/_/g, ' ')}
        </span>
        <span className="llm-confidence" data-level={confidence}>{confidence} confidence</span>
        <span className="llm-explanation">{explanation}</span>
      </div>
    )
  }

  if (job.result?.text) {
    return (
      <div className={`llm-insight llm-insight--${type === 'draft_communication' ? 'draft' : 'insight'}`}>
        {type === 'draft_communication'
          ? <pre className="llm-draft">{job.result.text}</pre>
          : <p className="llm-text">{job.result.text}</p>
        }
      </div>
    )
  }

  return <div className="llm-insight llm-insight--error">AI analysis unavailable</div>
}

export default LLMInsight
