import { useState, useEffect } from 'react'
import './DLQInspector.css'

function DLQInspector({ isOpen, onClose }) {
  const [dlqMessages, setDlqMessages] = useState([])
  const [selectedType, setSelectedType] = useState('send')
  const [loading, setLoading] = useState(false)
  const [retryingId, setRetryingId] = useState(null)

  const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

  useEffect(() => {
    if (isOpen) {
      fetchDLQMessages()
      const interval = setInterval(fetchDLQMessages, 5000)
      return () => clearInterval(interval)
    }
  }, [isOpen, selectedType])

  const fetchDLQMessages = async () => {
    setLoading(true)
    try {
      const response = await fetch(
        `${API_URL}/api/v1/queue/dlq?type=${selectedType}`
      )
      if (response.ok) {
        const data = await response.json()
        setDlqMessages(data.messages || 0)
      }
    } catch (err) {
      console.error('Failed to fetch DLQ messages:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleRetry = async (messageId) => {
    setRetryingId(messageId)
    try {
      const response = await fetch(
        `${API_URL}/api/v1/queue/dlq/${messageId}/retry`,
        { method: 'POST' }
      )

      if (response.ok) {
        setDlqMessages(prev => Math.max(0, prev - 1))
      } else {
        console.error('Failed to retry message')
      }
    } catch (err) {
      console.error('Error retrying message:', err)
    } finally {
      setRetryingId(null)
    }
  }

  if (!isOpen) return null

  return (
    <div className="dlq-inspector-overlay" onClick={onClose}>
      <div className="dlq-inspector-modal" onClick={e => e.stopPropagation()}>
        <div className="dlq-modal-header">
          <h2>Dead Letter Queue Inspector</h2>
          <button className="close-button" onClick={onClose}>×</button>
        </div>

        <div className="dlq-modal-body">
          <div className="type-selector">
            <button
              className={`type-tab ${selectedType === 'send' ? 'active' : ''}`}
              onClick={() => setSelectedType('send')}
            >
              Send DLQ
            </button>
            <button
              className={`type-tab ${selectedType === 'receive' ? 'active' : ''}`}
              onClick={() => setSelectedType('receive')}
            >
              Receive DLQ
            </button>
          </div>

          <div className="dlq-content">
            {loading ? (
              <div className="loading">Fetching DLQ messages...</div>
            ) : dlqMessages === 0 ? (
              <div className="empty-dlq">
                <p>No messages in {selectedType} DLQ</p>
              </div>
            ) : (
              <div className="dlq-list">
                <div className="dlq-message-count">
                  <span>{dlqMessages} message{dlqMessages !== 1 ? 's' : ''} in DLQ</span>
                </div>
                <div className="dlq-info">
                  <p>Messages stuck in the DLQ can be retried automatically.</p>
                  <p>They typically end up here after 3 failed attempts.</p>
                  <p>Use the retry button to republish them to the main queue.</p>
                </div>

                {dlqMessages > 0 && (
                  <button
                    className="retry-all-button"
                    onClick={() => {
                      for (let i = 0; i < dlqMessages; i++) {
                        handleRetry(`dlq-msg-${i}`)
                      }
                    }}
                    disabled={retryingId !== null}
                  >
                    {retryingId ? 'Retrying...' : 'Retry All'}
                  </button>
                )}
              </div>
            )}
          </div>
        </div>

        <div className="dlq-modal-footer">
          <p className="dlq-help-text">
            Auto-refresh every 5 seconds. Messages expire after 1 hour in DLQ.
          </p>
          <button className="close-modal-button" onClick={onClose}>
            Close
          </button>
        </div>
      </div>
    </div>
  )
}

export default DLQInspector
