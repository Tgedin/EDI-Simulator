import { jsPDF } from 'jspdf'

// ── Palette ───────────────────────────────────────────────────────────────────
const C = {
  headerBg: [20,  30,  60],
  accent:   [102, 126, 234],
  dark:     [30,  30,  50],
  mid:      [70,  80, 100],
  hint:     [140, 150, 170],
  border:   [215, 220, 235],
  rowAlt:   [245, 247, 255],
  white:    [255, 255, 255],
}

// ── XML parsing ───────────────────────────────────────────────────────────────
const parseCanonical = (metadata) => {
  if (!metadata || typeof metadata !== 'string') return null
  try {
    const xmlDoc = new DOMParser().parseFromString(metadata, 'application/xml')
    if (
      xmlDoc.documentElement.tagName === 'parsererror' ||
      xmlDoc.getElementsByTagName('parsererror').length > 0
    ) return null

    const getChild = (parent, tag) => {
      if (!parent) return ''
      const el = parent.getElementsByTagName(tag)[0]
      return el ? el.textContent.trim() : ''
    }

    const root    = xmlDoc.documentElement
    const envEl   = root.getElementsByTagName('Envelope')[0]
    const buyerEl = root.getElementsByTagName('BuyerParty')[0]
    const sellEl  = root.getElementsByTagName('SellerParty')[0]

    return {
      sender:         getChild(envEl,  'Sender'),
      receiver:       getChild(envEl,  'Receiver'),
      controlNumber:  getChild(envEl,  'ControlNumber'),
      documentNumber: getChild(root,   'DocumentNumber'),
      orderDate:      getChild(root,   'OrderDate'),
      currency:       getChild(root,   'Currency'),
      buyer: {
        id:      getChild(buyerEl, 'ID'),
        name:    getChild(buyerEl, 'Name'),
        address: getChild(buyerEl, 'Address'),
      },
      seller: {
        id:      getChild(sellEl, 'ID'),
        name:    getChild(sellEl, 'Name'),
        address: getChild(sellEl, 'Address'),
      },
      lineItems: Array.from(root.getElementsByTagName('LineItem')).map(el => ({
        lineNumber:    parseInt(getChild(el, 'LineNumber'), 10) || 0,
        productID:     getChild(el, 'ProductID'),
        description:   getChild(el, 'Description'),
        quantity:      parseFloat(getChild(el, 'Quantity'))  || 0,
        unitPrice:     parseFloat(getChild(el, 'UnitPrice')) || 0,
        unitOfMeasure: getChild(el, 'UnitOfMeasure'),
      })),
    }
  } catch {
    return null
  }
}

// ── Date helpers ──────────────────────────────────────────────────────────────
const fmtDate = (s) => {
  if (!s) return '-'
  const d = new Date(s)
  return isNaN(d.getTime()) ? s : d.toLocaleString()
}

const fmtDateShort = (s) => {
  if (!s) return '-'
  const d = new Date(s)
  return isNaN(d.getTime()) ? s : d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

const fmtMoney = (amount, currency = '') => {
  const prefix = currency && currency.length <= 3 ? `${currency} ` : ''
  return `${prefix}${amount.toFixed(2)}`
}

const sanitizeFilePart = (v) => String(v || 'message').replace(/[^a-zA-Z0-9_-]/g, '_')

// ── Section heading ───────────────────────────────────────────────────────────
const drawSectionTitle = (doc, x, y, text) => {
  doc.setFont('helvetica', 'bold')
  doc.setFontSize(9)
  doc.setTextColor(...C.accent)
  doc.text(text, x, y)
}

// ── Party box ─────────────────────────────────────────────────────────────────
const PARTY_BOX_H = 72

const drawPartyBox = (doc, x, y, w, role, party) => {
  doc.setFillColor(252, 252, 255)
  doc.setDrawColor(...C.border)
  doc.setLineWidth(0.5)
  doc.roundedRect(x, y - 6, w, PARTY_BOX_H, 3, 3, 'FD')

  doc.setFont('helvetica', 'bold')
  doc.setFontSize(8)
  doc.setTextColor(...C.accent)
  doc.text(role, x + 8, y + 8)

  let py = y + 22
  ;[['ID', party.id], ['Name', party.name], ['Address', party.address]].forEach(([label, val]) => {
    doc.setFont('helvetica', 'bold')
    doc.setFontSize(7.5)
    doc.setTextColor(...C.hint)
    doc.text(label, x + 8, py)
    doc.setFont('helvetica', 'normal')
    doc.setFontSize(8.5)
    doc.setTextColor(...C.mid)
    const maxChars = Math.max(4, Math.floor((w - 50) / 5.2))
    const display = (val || '-').length > maxChars ? (val || '-').slice(0, maxChars - 1) + '…' : (val || '-')
    doc.text(display, x + 42, py)
    py += 14
  })
}

// ── Table columns (A4 pt, margin=40, CW=515) ──────────────────────────────────
const TABLE_COLS = [
  { label: '#',           xOff:   0, w:  26, align: 'left'  },
  { label: 'Product ID',  xOff:  26, w:  88, align: 'left'  },
  { label: 'Description', xOff: 114, w: 152, align: 'left'  },
  { label: 'Qty',         xOff: 266, w:  48, align: 'right' },
  { label: 'UOM',         xOff: 314, w:  40, align: 'left'  },
  { label: 'Unit Price',  xOff: 354, w:  76, align: 'right' },
  { label: 'Total',       xOff: 430, w:  85, align: 'right' },
]

// ── Main export ───────────────────────────────────────────────────────────────
export const buildMessagePdf = (message, options = {}) => {
  const doc = new jsPDF({ unit: 'pt', format: 'a4' })
  const PW  = doc.internal.pageSize.getWidth()   // 595.28 pt
  const PH  = doc.internal.pageSize.getHeight()  // 841.89 pt
  const M   = 40
  const CW  = PW - M * 2

  const canonical = parseCanonical(message.metadata)
  let y = M
  let truncated = false

  // Ensure enough vertical space, adding a new page if needed
  const need = (h) => {
    if (y + h > PH - M - 24) {
      doc.addPage()
      y = M
    }
  }

  const hline = (yPos, color = C.border, lw = 0.5) => {
    doc.setDrawColor(...color)
    doc.setLineWidth(lw)
    doc.line(M, yPos, M + CW, yPos)
  }

  // ── Header band ─────────────────────────────────────────────────────────────
  doc.setFillColor(...C.headerBg)
  doc.rect(0, 0, PW, 68, 'F')

  doc.setFont('helvetica', 'bold')
  doc.setFontSize(22)
  doc.setTextColor(...C.white)
  doc.text('PURCHASE ORDER', M, 34)

  doc.setFont('helvetica', 'normal')
  doc.setFontSize(8.5)
  doc.setTextColor(170, 180, 210)
  doc.text(
    `EDI Format: ${(message.format || '-').toUpperCase()}   ·   Status: ${(message.status || '-').toUpperCase()}   ·   ID: ${message.id || '-'}`,
    M, 52,
  )

  y = 84

  if (canonical) {
    // ── Summary strip ──────────────────────────────────────────────────────────
    need(50)
    const stripCols = CW / 3
    ;[
      ['PO Number',  canonical.documentNumber || '-'],
      ['Order Date', fmtDateShort(canonical.orderDate)],
      ['Currency',   canonical.currency || '-'],
    ].forEach(([label, val], i) => {
      const x = M + i * stripCols
      doc.setFont('helvetica', 'normal')
      doc.setFontSize(7.5)
      doc.setTextColor(...C.hint)
      doc.text(label.toUpperCase(), x, y)
      doc.setFont('helvetica', 'bold')
      doc.setFontSize(14)
      doc.setTextColor(...C.dark)
      doc.text(val, x, y + 17)
    })
    y += 38
    hline(y)
    y += 18

    // ── Parties ────────────────────────────────────────────────────────────────
    need(PARTY_BOX_H + 30)
    drawSectionTitle(doc, M, y, 'PARTIES')
    y += 20
    const halfW = (CW - 16) / 2
    drawPartyBox(doc, M,             y, halfW, 'BUYER (BILL TO)', canonical.buyer)
    drawPartyBox(doc, M + halfW + 16, y, halfW, 'SELLER (SHIP FROM)', canonical.seller)
    y += PARTY_BOX_H + 18

    // ── Line items ─────────────────────────────────────────────────────────────
    if (canonical.lineItems.length > 0) {
      need(50)
      drawSectionTitle(doc, M, y, `LINE ITEMS  (${canonical.lineItems.length})`)
      y += 20

      // Table header row
      need(22)
      doc.setFillColor(...C.headerBg)
      doc.roundedRect(M, y - 14, CW, 20, 2, 2, 'F')
      doc.setFont('helvetica', 'bold')
      doc.setFontSize(7.5)
      doc.setTextColor(...C.white)
      TABLE_COLS.forEach(c => {
        const xBase = M + c.xOff
        c.align === 'right'
          ? doc.text(c.label, xBase + c.w, y, { align: 'right' })
          : doc.text(c.label, xBase + 2,   y)
      })
      y += 10

      let grandTotal = 0
      canonical.lineItems.forEach((item, idx) => {
        const ROW_H = 17
        need(ROW_H + 4)

        if (idx % 2 === 1) {
          doc.setFillColor(...C.rowAlt)
          doc.rect(M, y - 1, CW, ROW_H + 2, 'F')
        }

        const lineTotal = item.quantity * item.unitPrice
        grandTotal += lineTotal

        doc.setFont('helvetica', 'normal')
        doc.setFontSize(8.5)
        doc.setTextColor(...C.mid)
        ;[
          String(item.lineNumber > 0 ? item.lineNumber : idx + 1),
          item.productID     || '-',
          item.description   || '-',
          item.quantity > 0  ? item.quantity.toLocaleString() : '-',
          item.unitOfMeasure || '-',
          item.unitPrice > 0 ? fmtMoney(item.unitPrice)  : '-',
          lineTotal > 0      ? fmtMoney(lineTotal)        : '-',
        ].forEach((val, ci) => {
          const c = cols(ci)
          const maxChars = Math.max(4, Math.floor(c.w / 5.2))
          const display = val.length > maxChars ? val.slice(0, maxChars - 1) + '…' : val
          c.align === 'right'
            ? doc.text(display, M + c.xOff + c.w, y + ROW_H - 4, { align: 'right' })
            : doc.text(display, M + c.xOff + 2,   y + ROW_H - 4)
        })
        y += ROW_H
      })

      // Grand total row
      need(26)
      hline(y + 4, C.accent, 1)
      y += 12
      doc.setFont('helvetica', 'bold')
      doc.setFontSize(9.5)
      doc.setTextColor(...C.dark)
      doc.text('ORDER TOTAL', M + TABLE_COLS[4].xOff + TABLE_COLS[4].w, y + 12, { align: 'right' })
      doc.text(
        grandTotal > 0 ? fmtMoney(grandTotal, canonical.currency) : '-',
        M + CW, y + 12, { align: 'right' },
      )
      y += 26
    }

    // ── Transmission details ───────────────────────────────────────────────────
    y += 6
    need(110)
    hline(y)
    y += 16
    drawSectionTitle(doc, M, y, 'TRANSMISSION DETAILS')
    y += 18
    ;[
      ['Sender',         canonical.sender        || message.sender   || '-'],
      ['Receiver',       canonical.receiver      || message.receiver || '-'],
      ['Control Number', canonical.controlNumber || '-'],
      ['Created At',     fmtDate(message.created_at)],
      ['Updated At',     fmtDate(message.updated_at)],
      ['Message ID',     message.id || '-'],
    ].forEach(([label, val]) => {
      need(14)
      doc.setFont('helvetica', 'bold')
      doc.setFontSize(7.5)
      doc.setTextColor(...C.hint)
      doc.text(label.toUpperCase(), M, y)
      doc.setFont('helvetica', 'normal')
      doc.setFontSize(9)
      doc.setTextColor(...C.mid)
      doc.text(String(val), M + 110, y)
      y += 14
    })

  } else {
    // ── Fallback: message info + raw EDI ──────────────────────────────────────
    need(80)
    drawSectionTitle(doc, M, y, 'MESSAGE INFORMATION')
    y += 18
    ;[
      ['Format',     (message.format || '-').toUpperCase()],
      ['Status',     message.status     || '-'],
      ['Sender',     message.sender     || '-'],
      ['Receiver',   message.receiver   || '-'],
      ['Created At', fmtDate(message.created_at)],
      ['Updated At', fmtDate(message.updated_at)],
      ['Message ID', message.id         || '-'],
    ].forEach(([label, val]) => {
      need(14)
      doc.setFont('helvetica', 'bold')
      doc.setFontSize(7.5)
      doc.setTextColor(...C.hint)
      doc.text(label.toUpperCase(), M, y)
      doc.setFont('helvetica', 'normal')
      doc.setFontSize(9)
      doc.setTextColor(...C.mid)
      doc.text(String(val), M + 110, y)
      y += 14
    })

    y += 14
    hline(y)
    y += 18
    drawSectionTitle(doc, M, y, 'RAW EDI CONTENT')
    y += 14
    doc.setFont('helvetica', 'italic')
    doc.setFontSize(8)
    doc.setTextColor(...C.hint)
    doc.text('This message has not been transformed yet — showing raw EDI content.', M, y)
    y += 16

    const MAX_RAW = options.forPreview ? 20000 : Infinity
    const raw = message.content || ''
    truncated = raw.length > MAX_RAW
    const display = truncated
      ? raw.slice(0, MAX_RAW) + '\n\n[Content truncated — use Download PDF for full content.]'
      : raw

    doc.setFont('courier', 'normal')
    doc.setFontSize(8)
    doc.setTextColor(...C.mid)
    doc.splitTextToSize(display, CW).forEach(line => {
      need(11)
      doc.text(line, M, y)
      y += 11
    })
  }

  // ── Per-page footers ──────────────────────────────────────────────────────────
  const total = doc.internal.getNumberOfPages()
  for (let p = 1; p <= total; p++) {
    doc.setPage(p)
    doc.setFont('helvetica', 'normal')
    doc.setFontSize(7.5)
    doc.setTextColor(...C.hint)
    doc.text(`Page ${p} of ${total}`, PW - M, PH - 18, { align: 'right' })
    doc.text('Generated by EDI Simulator', M, PH - 18)
  }

  const fileName = `edi-${sanitizeFilePart(message.id)}-${new Date().toISOString().replace(/[:.]/g, '-')}.pdf`
  return { blob: doc.output('blob'), fileName, truncated }
}

// Indexed access helper for TABLE_COLS
const cols = (i) => TABLE_COLS[i]
