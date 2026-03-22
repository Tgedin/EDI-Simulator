import { useState, useEffect } from 'react'
import './MessageList.css'

const PAGE_SIZE = 5

function MessageList({ messages, onSelectMessage, onRefresh, filterStatus, onFilterChange, partnerMap = {} }) {
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE)

  // Reset to 5 whenever the active filter changes so you always see the 5 most recent.
  useEffect(() => {
    setVisibleCount(PAGE_SIZE)
  }, [filterStatus])
  const getStatusColor = (status) => {
    const colors = {
      pending: '#ff9800',
      sent: '#2196f3',
      received: '#4caf50',
      transformed: '#00bcd4',
      processed: '#8bc34a',
      failed: '#f44336',
    }
    return colors[status] || '#999'
  }

  const formatDate = (dateString) => {
    const date = new Date(dateString)
    const now = new Date()
    const diffMs = now - date
    const diffSec = Math.floor(diffMs / 1000)
    const diffMin = Math.floor(diffMs / 60000)

    if (diffSec < 60) return `${diffSec}s ago`
    if (diffMin < 60) return `${diffMin}m ago`
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }

  const sortedMessages = [...messages].sort(
    (a, b) => new Date(b.created_at) - new Date(a.created_at)
  )
  const filtered = filterStatus
    ? sortedMessages.filter(m => m.status === filterStatus)
    : sortedMessages

  // Reset visible count when filter changes so the user always starts from the top.
  const displayed = filtered.slice(0, visibleCount)
  const remaining = filtered.length - visibleCount

  return (
    <div className="message-feed-container">
      <div className="feed-header">
        <h2>Message Feed</h2>
        <button className="refresh-button" onClick={onRefresh} title="Refresh">
          ↻
        </button>
      </div>

      {filterStatus && (
        <div
          className="filter-banner"
          style={{ '--filter-color': getStatusColor(filterStatus) }}
        >
          <span>Showing: <strong>{filterStatus}</strong></span>
          <button
            className="filter-banner-clear"
            onClick={() => { if (onFilterChange) { onFilterChange(null); setVisibleCount(PAGE_SIZE) } }}
            title="Clear filter"
          >
            ✕ clear
          </button>
        </div>
      )}

      {displayed.length === 0 ? (
        <div className="empty-feed">
          {filterStatus
            ? <p>No <strong>{filterStatus}</strong> messages</p>
            : <p>No messages yet</p>
          }
          {!filterStatus && <p className="empty-hint">Create a message to get started</p>}
        </div>
      ) : (
        <div className="feed-items">
          {displayed.map(message => (
            <div
              key={message.id}
              className="feed-item"
              onClick={() => onSelectMessage(message)}
            >
              <div className="feed-item-header">
                <span className="feed-time">
                  {formatDate(message.created_at)}
                </span>
                <span
                  className="feed-status"
                  style={{ backgroundColor: getStatusColor(message.status) }}
                >
                  {message.status}
                </span>
              </div>
              <div className="feed-item-body">
                <div className="feed-id">{message.id.substring(0, 12)}...</div>
                <div className="feed-details">
                  <span className="feed-format">{message.format}</span>
                  {message.sender && (
                    <span className="feed-route">
                      {partnerMap[message.sender] || message.sender.substring(0, 8) + '…'}
                      {' → '}
                      {message.receiver
                        ? (partnerMap[message.receiver] || message.receiver.substring(0, 8) + '…')
                        : 'unknown'}
                    </span>
                  )}
                </div>
              </div>
            </div>
          ))}
          {remaining > 0 && (
            <button
              className="feed-show-more"
              onClick={() => setVisibleCount(c => c + PAGE_SIZE)}
            >
              Show {Math.min(remaining, PAGE_SIZE)} more
              <span className="feed-show-more-count">{remaining} remaining</span>
            </button>
          )}
          {visibleCount > PAGE_SIZE && remaining === 0 && (
            <button
              className="feed-show-more feed-show-more--collapse"
              onClick={() => setVisibleCount(PAGE_SIZE)}
            >
              Show less
            </button>
          )}
        </div>
      )}
    </div>
  )
}

export default MessageList
