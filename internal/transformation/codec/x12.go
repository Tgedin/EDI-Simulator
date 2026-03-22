package codec

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/theo-gedin/edi-simulator/internal/transformation/canonical"
)

// X12Codec implements FormatCodec for the ASC X12 format.
// It handles segment-level parsing of the 850 Purchase Order transaction set.
type X12Codec struct{}

// Format returns the format identifier for this codec.
func (c *X12Codec) Format() string { return "x12" }

// Decode parses an X12 850 message and maps fields into a CanonicalDocument.
// Segment terminator: ~   Element separator: *
func (c *X12Codec) Decode(raw string) (*canonical.CanonicalDocument, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("x12: content cannot be empty")
	}

	// Normalise: accept both ~ and newline as segment terminators
	raw = strings.ReplaceAll(raw, "~\n", "~")
	raw = strings.ReplaceAll(raw, "~\r\n", "~")
	segments := splitSegments(raw, '~')

	if len(segments) == 0 {
		return nil, fmt.Errorf("x12: no segments found")
	}
	if !strings.HasPrefix(segments[0], "ISA") {
		return nil, fmt.Errorf("x12: message must begin with ISA segment")
	}

	doc := &canonical.CanonicalDocument{}
	doc.Envelope.CreatedAt = time.Now()

	// State for multi-segment line item accumulation
	var lineItems []canonical.LineItem
	var currentLine *canonical.LineItem
	inBEGFound := false

	for _, seg := range segments {
		if strings.TrimSpace(seg) == "" {
			continue
		}
		elems := strings.Split(seg, "*")
		tag := strings.TrimSpace(elems[0])

		switch tag {
		case canonical.SegISA:
			doc.Envelope.Sender = get(elems, canonical.ISASenderIdx)
			doc.Envelope.Receiver = get(elems, canonical.ISAReceiverIdx)
			doc.Envelope.ControlNumber = get(elems, canonical.ISAControlIdx)

		case canonical.SegBEG:
			inBEGFound = true
			doc.Body.DocumentNumber = get(elems, canonical.BEGDocNumIdx)
			if raw := get(elems, canonical.BEGOrderDateIdx); raw != "" {
				doc.Body.OrderDate = parseX12Date(raw)
			}

		case canonical.SegN1:
			qual := get(elems, 1)
			name := get(elems, canonical.N1NameIdx)
			id := get(elems, canonical.N1IDIdx)
			switch qual {
			case canonical.N1QualBY:
				doc.Body.BuyerParty = canonical.Party{ID: id, Name: name}
			case canonical.N1QualSE:
				doc.Body.SellerParty = canonical.Party{ID: id, Name: name}
			}

		case canonical.SegPO1:
			// Save previous line item if any
			if currentLine != nil {
				lineItems = append(lineItems, *currentLine)
			}
			lineNum, _ := strconv.Atoi(get(elems, canonical.PO1LineNumIdx))
			qty, _ := strconv.ParseFloat(get(elems, canonical.PO1QtyIdx), 64)
			uom := get(elems, canonical.PO1UoMIdx)
			price, _ := strconv.ParseFloat(get(elems, canonical.PO1PriceIdx), 64)

			// Extract product ID from qualifier/value pairs starting at index 6
			productID := ""
			for i := canonical.PO1CodeStart; i+1 < len(elems); i += 2 {
				qual := get(elems, i)
				val := get(elems, i+1)
				if qual == canonical.QualVP || qual == canonical.QualBP || qual == canonical.QualUP {
					productID = val
					break
				}
				if productID == "" && val != "" {
					productID = val // fallback: take first non-empty value
				}
			}

			currentLine = &canonical.LineItem{
				LineNumber:    lineNum,
				ProductID:     productID,
				Quantity:      qty,
				UnitPrice:     price,
				UnitOfMeasure: uom,
			}
		}
	}

	// Flush last line item
	if currentLine != nil {
		lineItems = append(lineItems, *currentLine)
	}
	doc.Body.LineItems = lineItems

	if !inBEGFound {
		// Tolerate missing BEG for non-850 transactions fed into the engine
		if doc.Envelope.Sender == "" {
			return nil, fmt.Errorf("x12: missing ISA segment")
		}
	}

	return doc, nil
}

// Encode maps a CanonicalDocument back to X12 850 wire format.
func (c *X12Codec) Encode(doc *canonical.CanonicalDocument) (string, error) {
	if doc == nil {
		return "", fmt.Errorf("x12: document cannot be nil")
	}

	ctrl := doc.Envelope.ControlNumber
	if ctrl == "" {
		ctrl = "000000001"
	}
	// Pad control number to 9 digits for IEA
	ctrlPadded := fmt.Sprintf("%09s", ctrl)

	sender := padRight(doc.Envelope.Sender, 15)
	receiver := padRight(doc.Envelope.Receiver, 15)

	dateStr := "000101"
	timeStr := "0000"
	if !doc.Envelope.CreatedAt.IsZero() {
		dateStr = doc.Envelope.CreatedAt.Format("060102")
		timeStr = doc.Envelope.CreatedAt.Format("1504")
	}
	orderDateStr := ""
	if !doc.Body.OrderDate.IsZero() {
		orderDateStr = doc.Body.OrderDate.Format("20060102")
	}

	var sb strings.Builder
	// ISA – fixed-width fields
	sb.WriteString(fmt.Sprintf(
		"ISA*00*          *00*          *ZZ*%s*ZZ*%s*%s*%s*U*00401*%s*0*T*>~\n",
		sender, receiver, dateStr, timeStr, ctrlPadded,
	))
	sb.WriteString(fmt.Sprintf("GS*OE*%s*%s*%s*%s*1*X*004010~\n",
		strings.TrimSpace(doc.Envelope.Sender),
		strings.TrimSpace(doc.Envelope.Receiver),
		orderDateStr,
		timeStr,
	))
	sb.WriteString("ST*850*0001~\n")
	sb.WriteString(fmt.Sprintf("BEG*00*SA*%s**%s~\n", doc.Body.DocumentNumber, orderDateStr))

	// Party segments
	if doc.Body.BuyerParty.ID != "" || doc.Body.BuyerParty.Name != "" {
		sb.WriteString(fmt.Sprintf("N1*BY*%s*ZZ*%s~\n",
			doc.Body.BuyerParty.Name, doc.Body.BuyerParty.ID))
	}
	if doc.Body.SellerParty.ID != "" || doc.Body.SellerParty.Name != "" {
		sb.WriteString(fmt.Sprintf("N1*SE*%s*ZZ*%s~\n",
			doc.Body.SellerParty.Name, doc.Body.SellerParty.ID))
	}

	// Line items
	for _, li := range doc.Body.LineItems {
		sb.WriteString(fmt.Sprintf("PO1*%d*%g*%s*%g**VP*%s~\n",
			li.LineNumber,
			li.Quantity,
			li.UnitOfMeasure,
			li.UnitPrice,
			li.ProductID,
		))
	}

	segCount := 4 + countN1Segments(doc) + len(doc.Body.LineItems)
	sb.WriteString(fmt.Sprintf("SE*%d*0001~\n", segCount))
	sb.WriteString("GE*1*1~\n")
	sb.WriteString(fmt.Sprintf("IEA*1*%s~\n", ctrlPadded))

	return sb.String(), nil
}

// ---- helpers ----------------------------------------------------------------

// splitSegments splits an X12 document by a separator character, stripping
// trailing whitespace from each segment.
func splitSegments(raw string, sep rune) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == sep })
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// get returns the element at 1-based index idx, or "" if out of range.
func get(elems []string, idx int) string {
	if idx < len(elems) {
		return strings.TrimSpace(elems[idx])
	}
	return ""
}

// parseX12Date parses YYYYMMDD or YYMMDD date strings.
func parseX12Date(s string) time.Time {
	if len(s) == 8 {
		t, err := time.Parse("20060102", s)
		if err == nil {
			return t
		}
	}
	if len(s) == 6 {
		t, err := time.Parse("060102", s)
		if err == nil {
			return t
		}
	}
	return time.Time{}
}

// padRight pads or truncates a string to exactly n characters.
func padRight(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}

// countN1Segments returns how many N1 segments will be emitted.
func countN1Segments(doc *canonical.CanonicalDocument) int {
	count := 0
	if doc.Body.BuyerParty.ID != "" || doc.Body.BuyerParty.Name != "" {
		count++
	}
	if doc.Body.SellerParty.ID != "" || doc.Body.SellerParty.Name != "" {
		count++
	}
	return count
}
