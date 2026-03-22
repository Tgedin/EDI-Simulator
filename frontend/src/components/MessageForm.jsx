import { useState, useEffect } from 'react'
import './MessageForm.css'

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

function MessageForm({ onSubmit, loading, onCancel }) {
  const [format, setFormat] = useState('x12')
  const [content, setContent] = useState('')
  const [sender, setSender] = useState('')
  const [receiver, setReceiver] = useState('')
  const [partners, setPartners] = useState([])

  useEffect(() => {
    fetch(`${API_URL}/api/v1/partners`)
      .then((r) => r.json())
      .then((data) => setPartners(data.partners || []))
      .catch(() => setPartners([]))
  }, [])

  const handleSubmit = (e) => {
    e.preventDefault()
    if (!content.trim()) {
      alert('Please enter message content')
      return
    }
    onSubmit(format, content, {}, sender, receiver)
    setContent('')
    setSender('')
    setReceiver('')
  }

  const sampleMessages = {
    x12: `ISA*00*          *00*          *01*9876543210     *01*1234567890     *220215*1430*^*00501*000000001*0*T*:
GS*PO*9876543210*1234567890*20220215*143000*1*X*005010
ST*850*000001
BEG*00*NE*ORDER123**220215
DTM*002*20220215
N1*BY*Buyer Company
N1*SU*Supplier Company
PO1*1*100*EA*10.00*CB*VN*SKUABC123
CTT*1
SE*9*000001
GE*1*1
IEA*1*000000001`,
    edifact: `UNA:+.? '
UNB+IATB:1+SNDPROC+RCVPROC+200215:1234+1+ORDERS'
UNH+1+ORDERS:D:96A:UN'
BGM+220+ORDER123+9'
DTM+137:20220215:102'
NAD+BY+BUYER123'
NAD+SU+SUPPLIER123'
UNT+7+1'
UNZ+1+1'`,
    xml: `<?xml version="1.0" encoding="UTF-8"?>
<Message>
  <Type>PurchaseOrder</Type>
  <OrderNumber>ORDER123</OrderNumber>
  <Date>2022-02-15</Date>
  <Buyer>Buyer Company</Buyer>
  <Supplier>Supplier Company</Supplier>
  <Items>
    <Item>
      <SKU>SKUABC123</SKU>
      <Quantity>100</Quantity>
      <UnitPrice>10.00</UnitPrice>
    </Item>
  </Items>
</Message>`,
  }

  return (
    <div className="message-form-container">
      <h2>Create New Message</h2>
      <form onSubmit={handleSubmit}>
        <div className="form-row">
          <div className="form-group">
            <label htmlFor="sender">Sender</label>
            <select
              id="sender"
              value={sender}
              onChange={(e) => setSender(e.target.value)}
              disabled={loading}
            >
              <option value="">— select sender —</option>
              {partners.map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.country} · {p.preferred_format.toUpperCase()})
                </option>
              ))}
            </select>
          </div>

          <div className="form-group">
            <label htmlFor="receiver">Receiver</label>
            <select
              id="receiver"
              value={receiver}
              onChange={(e) => setReceiver(e.target.value)}
              disabled={loading}
            >
              <option value="">— select receiver —</option>
              {partners.map((p) => (
                <option key={p.id} value={p.name}>
                  {p.name} ({p.country} · {p.preferred_format.toUpperCase()})
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="form-group">
          <label htmlFor="format">Format</label>
          <select
            id="format"
            value={format}
            onChange={(e) => setFormat(e.target.value)}
            disabled={loading}
          >
            <option value="x12">X12 EDI</option>
            <option value="edifact">EDIFACT</option>
            <option value="xml">XML</option>
          </select>
        </div>

        <div className="form-group">
          <div className="label-with-button">
            <label htmlFor="content">Message Content</label>
            <button
              type="button"
              className="load-sample-button"
              onClick={() => setContent(sampleMessages[format])}
            >
              Load Sample
            </button>
          </div>
          <textarea
            id="content"
            value={content}
            onChange={(e) => setContent(e.target.value)}
            placeholder="Paste your message content here..."
            rows={15}
            disabled={loading}
          />
        </div>

        <div className="form-actions">
          <button type="submit" className="submit-button" disabled={loading}>
            {loading ? 'Creating...' : 'Create Message'}
          </button>
          <button type="button" className="cancel-button" onClick={onCancel} disabled={loading}>
            Cancel
          </button>
        </div>
      </form>
    </div>
  )
}

export default MessageForm
