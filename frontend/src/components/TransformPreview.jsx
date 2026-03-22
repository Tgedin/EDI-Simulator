import { useState, useEffect } from 'react'
import './TransformPreview.css'

const SAMPLE_X12 = `ISA*00*          *00*          *ZZ*SENDER         *ZZ*RECEIVER       *260215*1200*U*00401*000000001*0*T*>~
GS*OE*SENDER*RECEIVER*20260215*1200*1*X*004010~
ST*850*0001~
BEG*00*SA*PO-12345**20260215~
N1*BY*Buyer Corp*ZZ*BUYER01~
N1*SE*Seller Inc*ZZ*SELLER01~
PO1*1*10*EA*9.99**VP*WIDGET-A~
SE*5*0001~
GE*1*1~
IEA*1*000000001~`

const SAMPLE_EDIFACT = `UNA:+.? '
UNB+IATB:1+SENDER+RECEIVER+260215:1200+1+ORDERS'
UNH+1+ORDERS:D:96A:UN'
BGM+220+PO-12345+9'
DTM+137:20260215:102'
NAD+BY+BUYER01::ZZ++Buyer Corp'
NAD+SE+SELLER01::ZZ++Seller Inc'
LIN+1++WIDGET-A:VP'
QTY+21:10:EA'
PRI+AAA:9.99'
UNT+9+1'
UNZ+1+1'`

function TransformPreview() {
  const [formats, setFormats] = useState([])
  const [sourceFormat, setSourceFormat] = useState('x12')
  const [targetFormat, setTargetFormat] = useState('edifact')
  const [inputContent, setInputContent] = useState(SAMPLE_X12)
  const [canonical, setCanonical] = useState('')
  const [output, setOutput] = useState('')
  const [fieldsMapped, setFieldsMapped] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)

  const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

  useEffect(() => {
    fetch(`${API_URL}/api/v1/transform/formats`)
      .then(r => r.json())
      .then(data => {
        if (data.formats && data.formats.length > 0) {
          setFormats(data.formats)
        }
      })
      .catch(err => console.error('Failed to fetch formats:', err))
  }, [API_URL])

  // Reset output when inputs change
  useEffect(() => {
    setCanonical('')
    setOutput('')
    setFieldsMapped(null)
    setError(null)
  }, [sourceFormat, targetFormat, inputContent])

  // Load a sample when the source format dropdown changes
  useEffect(() => {
    if (sourceFormat === 'x12') setInputContent(SAMPLE_X12)
    else if (sourceFormat === 'edifact') setInputContent(SAMPLE_EDIFACT)
    else setInputContent('')
  }, [sourceFormat])

  const handleTransform = async () => {
    if (!inputContent.trim()) {
      setError('Input content cannot be empty')
      return
    }
    setLoading(true)
    setError(null)
    setCanonical('')
    setOutput('')
    setFieldsMapped(null)

    try {
      const response = await fetch(`${API_URL}/api/v1/transform/preview`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          source_format: sourceFormat,
          target_format: targetFormat,
          content: inputContent,
        }),
      })

      const data = await response.json()

      if (!response.ok) {
        setError(data.error || 'Transformation failed')
        return
      }

      setCanonical(data.canonical || '')
      setOutput(data.output || '')
      setFieldsMapped(data.fields_mapped ?? null)
    } catch (err) {
      setError('Network error: ' + err.message)
    } finally {
      setLoading(false)
    }
  }

  const formatLabel = f => f ? f.toUpperCase() : ''

  return (
    <div className="transform-preview">
      <div className="transform-toolbar">
        <div className="transform-route">
          <div className="format-select-group">
            <label className="format-label">Source</label>
            <select
              className="format-select"
              value={sourceFormat}
              onChange={e => setSourceFormat(e.target.value)}
              disabled={loading}
            >
              {formats.length > 0
                ? formats.map(f => (
                    <option key={f} value={f}>
                      {formatLabel(f)}
                    </option>
                  ))
                : <>
                    <option value="x12">X12</option>
                    <option value="edifact">EDIFACT</option>
                    <option value="xml">XML</option>
                  </>
              }
            </select>
          </div>

          <span className="route-arrow">→</span>

          <div className="format-select-group">
            <label className="format-label">Target</label>
            <select
              className="format-select"
              value={targetFormat}
              onChange={e => setTargetFormat(e.target.value)}
              disabled={loading}
            >
              {formats.length > 0
                ? formats.map(f => (
                    <option key={f} value={f}>
                      {formatLabel(f)}
                    </option>
                  ))
                : <>
                    <option value="x12">X12</option>
                    <option value="edifact">EDIFACT</option>
                    <option value="xml">XML</option>
                  </>
              }
            </select>
          </div>
        </div>

        <button
          className={`transform-btn ${loading ? 'loading' : ''}`}
          onClick={handleTransform}
          disabled={loading}
        >
          {loading ? 'Transforming…' : 'Transform'}
        </button>
      </div>

      {error && <div className="transform-error">{error}</div>}

      <div className="transform-panels">
        {/* Panel 1 — raw input */}
        <div className="transform-panel">
          <div className="panel-header">
            <span className="panel-title">Raw Input</span>
            <span className="panel-badge panel-badge--source">{formatLabel(sourceFormat)}</span>
          </div>
          <textarea
            className="panel-textarea panel-textarea--editable"
            value={inputContent}
            onChange={e => setInputContent(e.target.value)}
            placeholder={`Paste your ${formatLabel(sourceFormat)} message here…`}
            spellCheck={false}
            disabled={loading}
          />
        </div>

        {/* Panel 2 — canonical XML */}
        <div className="transform-panel">
          <div className="panel-header">
            <span className="panel-title">Canonical Model</span>
            <span className="panel-badge panel-badge--canonical">XML</span>
          </div>
          <textarea
            className="panel-textarea panel-textarea--readonly"
            value={canonical}
            readOnly
            placeholder="The shared in-memory representation will appear here after transformation."
            spellCheck={false}
          />
        </div>

        {/* Panel 3 — output */}
        <div className="transform-panel">
          <div className="panel-header">
            <span className="panel-title">Transformed Output</span>
            <span className="panel-badge panel-badge--target">{formatLabel(targetFormat)}</span>
          </div>
          <textarea
            className="panel-textarea panel-textarea--readonly"
            value={output}
            readOnly
            placeholder="The transformed wire-format output will appear here."
            spellCheck={false}
          />
        </div>
      </div>

      <div className="transform-footer">
        {fieldsMapped !== null ? (
          <>
            <span className="footer-fields">{fieldsMapped} element{fieldsMapped !== 1 ? 's' : ''} mapped</span>
            <span className="footer-sep">·</span>
            <span className="footer-mapping">
              {formatLabel(sourceFormat)} → {formatLabel(targetFormat)}
            </span>
          </>
        ) : (
          <span className="footer-idle">
            Paste a message, choose formats, then click Transform.
          </span>
        )}
      </div>
    </div>
  )
}

export default TransformPreview
