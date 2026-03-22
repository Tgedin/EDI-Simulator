import { useState, useEffect } from 'react'
import './SimulatorControl.css'

function SimulatorControl() {
  const [paused, setPaused] = useState(null) // null = loading / unreachable
  const [busy, setBusy] = useState(false)

  const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

  useEffect(() => {
    fetchStatus()
    const interval = setInterval(fetchStatus, 3000)
    return () => clearInterval(interval)
  }, [])

  const fetchStatus = async () => {
    try {
      const res = await fetch(`${API_URL}/api/v1/simulator/status`)
      if (res.ok) {
        const data = await res.json()
        setPaused(data.paused)
      }
    } catch {
      setPaused(null)
    }
  }

  const handleToggle = async () => {
    if (busy || paused === null) return
    setBusy(true)
    try {
      const action = paused ? 'resume' : 'pause'
      const res = await fetch(`${API_URL}/api/v1/simulator/control`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action }),
      })
      if (res.ok) {
        const data = await res.json()
        setPaused(data.paused)
      }
    } catch {
      // silently fail; next poll will correct state
    } finally {
      setBusy(false)
    }
  }

  if (paused === null) {
    return (
      <div className="sim-control sim-control--offline" title="Simulator unreachable">
        <span className="sim-dot sim-dot--offline" />
        <span className="sim-label">Simulator</span>
      </div>
    )
  }

  return (
    <button
      className={`sim-control ${paused ? 'sim-control--paused' : 'sim-control--running'}`}
      onClick={handleToggle}
      disabled={busy}
      title={paused ? 'Click to resume the simulator' : 'Click to pause the simulator'}
    >
      <span className={`sim-dot ${paused ? 'sim-dot--paused' : 'sim-dot--running'}`} />
      <span className="sim-label">{paused ? 'Resume' : 'Pause'}</span>
    </button>
  )
}

export default SimulatorControl
