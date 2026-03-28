import { useState, useEffect } from 'react'
import './MessageDetail.css'
import LLMInsight from './LLMInsight'
import './LLMInsight.css'
import { buildMessagePdf } from '../utils/pdfMessage'

function MessageDetail({ message, onBack }) {
  const [transactions, setTransactions] = useState([])
  const [loading, setLoading] = useState(true)
  const [showClassify, setShowClassify] = useState(false)
  const [classifyDone, setClassifyDone] = useState(false)
  const [showDraft, setShowDraft] = useState(false)
  const [isPdfOpen, setIsPdfOpen] = useState(false)
  const [pdfPreviewUrl, setPdfPreviewUrl] = useState('')
  const [pdfState, setPdfState] = useState('idle')
  const [pdfError, setPdfError] = useState('')
  const [previewTruncated, setPreviewTruncated] = useState(false)

  const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

  useEffect(() => {
    fetchTransactions()
    const interval = setInterval(fetchTransactions, 3000)
    return () => clearInterval(interval)
  }, [message.id])

  useEffect(() => {
    return () => {
      if (pdfPreviewUrl) {
        URL.revokeObjectURL(pdfPreviewUrl)
      }
    }
  }, [pdfPreviewUrl])

  useEffect(() => {
    setIsPdfOpen(false)
    setPdfState('idle')
    setPdfError('')
    setPreviewTruncated(false)

    if (pdfPreviewUrl) {
      URL.revokeObjectURL(pdfPreviewUrl)
      setPdfPreviewUrl('')
    }
  }, [message.id])

  const fetchTransactions = async () => {
    try {
      const response = await fetch(`${API_URL}/api/v1/messages/${message.id}/transactions`)
      if (response.ok) {
        const data = await response.json()
        setTransactions(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch transactions:', err)
    } finally {
      setLoading(false)
    }
  }

  const formatDate = (dateString) => {
    const date = new Date(dateString)
    return date.toLocaleString()
  }

  const getStatusColor = (status) => {
    const colors = {
      pending: '#ff9800',
      sent: '#2196f3',
      received: '#4caf50',
      processed: '#8bc34a',
      failed: '#f44336',
    }
    return colors[status] || '#999'
  }

  const openPdfPreview = () => {
    setIsPdfOpen(true)
    setPdfError('')

    if (pdfPreviewUrl) {
      setPdfState('ready')
      return
    }

    setPdfState('generating')

    try {
      const { blob, truncated } = buildMessagePdf(message, { forPreview: true })
      const url = URL.createObjectURL(blob)
      setPdfPreviewUrl(url)
      setPreviewTruncated(truncated)
      setPdfState('ready')
    } catch (err) {
      setPdfState('error')
      setPdfError('Failed to generate preview PDF')
      console.error('Failed to generate preview PDF:', err)
    }
  }

  const closePdfPreview = () => {
    setIsPdfOpen(false)
  }

  const downloadPdf = () => {
    setPdfError('')

    try {
      const { blob, fileName } = buildMessagePdf(message)
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = fileName
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      URL.revokeObjectURL(url)
    } catch (err) {
      setPdfState('error')
      setPdfError('Failed to create PDF download')
      console.error('Failed to create PDF download:', err)
    }
  }

  return (
    <div className="message-detail-container">
      <button className="back-button" onClick={onBack}>
        ← Back to Messages
      </button>

      <div className="detail-header">
        <h2>Message Details</h2>
        <span className="message-status-badge" style={{ backgroundColor: getStatusColor(message.status) }}>
          {message.status}
        </span>
      </div>

      <div className="detail-grid">
        <div className="detail-card">
          <h3>Message Information</h3>
          <table className="detail-table">
            <tbody>
              <tr>
                <td className="label">ID</td>
                <td className="value mono">{message.id}</td>
              </tr>
              <tr>
                <td className="label">Format</td>
                <td className="value">{message.format}</td>
              </tr>
              <tr>
                <td className="label">Status</td>
                <td className="value">{message.status}</td>
              </tr>
              <tr>
                <td className="label">Sender</td>
                <td className="value">{message.sender || '-'}</td>
              </tr>
              <tr>
                <td className="label">Receiver</td>
                <td className="value">{message.receiver || '-'}</td>
              </tr>
              <tr>
                <td className="label">Created</td>
                <td className="value">{formatDate(message.created_at)}</td>
              </tr>
              <tr>
                <td className="label">Updated</td>
                <td className="value">{formatDate(message.updated_at)}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div className="detail-card full-width">
          <h3>Message Content</h3>
          <div className="pdf-actions" role="group" aria-label="PDF actions">
            <button
              className="ai-btn ai-btn--primary"
              onClick={openPdfPreview}
              disabled={pdfState === 'generating'}
            >
              {pdfState === 'generating' ? 'Generating PDF...' : 'Preview PDF'}
            </button>
            <button className="ai-btn" onClick={downloadPdf}>
              Download PDF
            </button>
            {isPdfOpen && (
              <button className="ai-btn" onClick={closePdfPreview}>
                Hide Preview
              </button>
            )}
          </div>
          {pdfError && <p className="pdf-error">{pdfError}</p>}
          {isPdfOpen && pdfState === 'ready' && previewTruncated && (
            <p className="pdf-note">Preview is truncated for speed. Use Download PDF for full content.</p>
          )}
          {isPdfOpen && pdfState === 'ready' && pdfPreviewUrl && (
            <div className="pdf-preview-wrapper">
              <iframe title="EDI Message PDF Preview" src={pdfPreviewUrl} className="pdf-preview-frame" />
            </div>
          )}
          <div className="code-block">
            <pre>{message.content}</pre>
          </div>
        </div>

        <div className="detail-card full-width">
          <h3>Audit Trail ({transactions.length})</h3>
          {loading && <p>Loading transactions...</p>}
          {!loading && transactions.length === 0 && (
            <p style={{ color: '#999' }}>No audit events yet</p>
          )}
          {!loading && transactions.length > 0 && (
            <div className="transaction-list">
              {transactions.map(tx => (
                <div key={tx.id} className="transaction-item">
                  <div className="tx-header">
                    <span className="tx-event">{tx.event}</span>
                    <span className="tx-time">{formatDate(tx.timestamp)}</span>
                  </div>
                  {tx.details && (
                    <div className="tx-details">
                      <code>{JSON.stringify(tx.details, null, 2)}</code>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        {message.status === 'failed' && (
          <div className="detail-card full-width">
            <div className="ai-section">
              <p className="ai-section-title">🧠 AI Analysis</p>
              <div className="ai-actions">
                {!showClassify && (
                  <button className="ai-btn ai-btn--primary" onClick={() => setShowClassify(true)}>
                    🤖 Ask AI
                  </button>
                )}
                {classifyDone && !showDraft && (
                  <button className="ai-btn" onClick={() => setShowDraft(true)}>
                    📧 Draft Communication
                  </button>
                )}
              </div>
              {showClassify && (
                <LLMInsight
                  type="classify_failure"
                  inputRef={message.id}
                  onDone={() => setClassifyDone(true)}
                />
              )}
              {showDraft && (
                <LLMInsight type="draft_communication" inputRef={message.id} />
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

export default MessageDetail
