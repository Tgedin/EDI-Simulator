import { useState, useEffect } from 'react'
import MessageList from './components/MessageList'
import MessageForm from './components/MessageForm'
import MessageDetail from './components/MessageDetail'
import MetricsPanel from './components/MetricsPanel'
import DLQInspector from './components/DLQInspector'
import TransformPreview from './components/TransformPreview'
import SimulatorControl from './components/SimulatorControl'
import './App.css'

function App() {
  const [currentView, setCurrentView] = useState('dashboard')
  const [messages, setMessages] = useState([])
  const [selectedMessage, setSelectedMessage] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [showDLQInspector, setShowDLQInspector] = useState(false)
  const [filterStatus, setFilterStatus] = useState(null)
  const [partnerMap, setPartnerMap] = useState({})

  // Dark mode: follow OS preference by default, persist manual override in localStorage
  const [darkMode, setDarkMode] = useState(() => {
    const saved = localStorage.getItem('theme')
    if (saved) return saved === 'dark'
    return window.matchMedia('(prefers-color-scheme: dark)').matches
  })

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', darkMode ? 'dark' : 'light')
    localStorage.setItem('theme', darkMode ? 'dark' : 'light')
  }, [darkMode])

  const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

  useEffect(() => {
    fetchMessages()
    fetchPartners()
    // Poll every 3 seconds
    const interval = setInterval(fetchMessages, 3000)
    return () => clearInterval(interval)
  }, [])

  const fetchPartners = async () => {
    try {
      const response = await fetch(`${API_URL}/api/v1/partners`)
      if (response.ok) {
        const data = await response.json()
        const map = {}
        for (const p of (data.partners || [])) {
          map[p.id] = p.name
        }
        setPartnerMap(map)
      }
    } catch (err) {
      console.error('Failed to fetch partners:', err)
    }
  }

  const fetchMessages = async () => {
    try {
      const response = await fetch(`${API_URL}/api/v1/messages`)
      if (response.ok) {
        const data = await response.json()
        setMessages(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch messages:', err)
    }
  }

  const handleMessageSubmit = async (format, content, metadata, sender, receiver) => {
    setLoading(true)
    setError(null)
    try {
      const response = await fetch(`${API_URL}/api/v1/messages`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ format, content, metadata, sender, receiver }),
      })

      if (!response.ok) {
        const err = await response.json()
        throw new Error(err.error || 'Failed to create message')
      }

      const newMessage = await response.json()
      setMessages([newMessage, ...messages])
      setCurrentView('list')
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleSelectMessage = (message) => {
    setSelectedMessage(message)
    setCurrentView('detail')
  }

  const handleBackToDashboard = () => {
    setCurrentView('dashboard')
    setSelectedMessage(null)
    fetchMessages()
  }

  return (
    <div className="app" data-theme={darkMode ? 'dark' : 'light'}>
      <header className="app-header">
        <div className="app-header-top">
          <h1>EDI Transaction Simulator</h1>
          <div className="header-controls">
            <SimulatorControl />
            <button
              className="theme-toggle"
              onClick={() => setDarkMode(d => !d)}
              title={darkMode ? 'Switch to light mode' : 'Switch to dark mode'}
            >
              {darkMode ? '☀️' : '🌙'}
            </button>
          </div>
        </div>
        <nav className="app-nav">
          <button
            className={`nav-button ${currentView === 'dashboard' ? 'active' : ''}`}
            onClick={() => { setCurrentView('dashboard'); setSelectedMessage(null) }}
          >
            Dashboard
          </button>
          <button
            className={`nav-button ${currentView === 'form' ? 'active' : ''}`}
            onClick={() => setCurrentView('form')}
          >
            Create Message
          </button>
          <button
            className="nav-button"
            onClick={() => setShowDLQInspector(true)}
          >
            DLQ Inspector
          </button>
          <button
            className={`nav-button ${currentView === 'transform' ? 'active' : ''}`}
            onClick={() => { setCurrentView('transform'); setSelectedMessage(null) }}
          >
            Transform
          </button>
        </nav>
      </header>

      <main className="app-main">
        {error && <div className="error-banner">{error}</div>}

        {currentView === 'dashboard' && (
          <div className="dashboard-layout">
            <MetricsPanel messages={messages} filterStatus={filterStatus} onFilterChange={setFilterStatus} />
            <MessageList
              messages={messages}
              onSelectMessage={handleSelectMessage}
              onRefresh={fetchMessages}
              filterStatus={filterStatus}
              onFilterChange={setFilterStatus}
              partnerMap={partnerMap}
            />
          </div>
        )}

        {currentView === 'form' && (
          <MessageForm
            onSubmit={handleMessageSubmit}
            loading={loading}
            onCancel={() => setCurrentView('dashboard')}
          />
        )}

        {currentView === 'detail' && selectedMessage && (
          <MessageDetail
            message={selectedMessage}
            onBack={handleBackToDashboard}
          />
        )}

        {currentView === 'transform' && (
          <TransformPreview />
        )}
      </main>

      <DLQInspector
        isOpen={showDLQInspector}
        onClose={() => setShowDLQInspector(false)}
      />
    </div>
  )
}

export default App
