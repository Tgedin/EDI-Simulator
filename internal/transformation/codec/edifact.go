package codec

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/theo-gedin/edi-simulator/internal/transformation/canonical"
)

// EDIFACTCodec implements FormatCodec for UN/EDIFACT ORDERS D.96A.
// Segment terminator: '    Element separator: +    Component separator: :
type EDIFACTCodec struct{}

// Format returns the format identifier for this codec.
func (c *EDIFACTCodec) Format() string { return "edifact" }

// Decode parses an EDIFACT ORDERS message into a CanonicalDocument.
func (c *EDIFACTCodec) Decode(raw string) (*canonical.CanonicalDocument, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("edifact: content cannot be empty")
	}

	// Normalise newlines and split by ' (segment terminator)
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	segments := splitEDIFACTSegments(raw)

	if len(segments) == 0 {
		return nil, fmt.Errorf("edifact: no segments found")
	}

	// Validate that the message starts with UNA or UNB
	first := strings.TrimSpace(segments[0])
	if !strings.HasPrefix(first, "UNA") && !strings.HasPrefix(first, "UNB") {
		return nil, fmt.Errorf("edifact: message must begin with UNA or UNB segment")
	}

	doc := &canonical.CanonicalDocument{}
	doc.Envelope.CreatedAt = time.Now()

	var lineItems []canonical.LineItem
	var currentLine *canonical.LineItem

	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}

		elems := strings.Split(seg, "+")
		tag := strings.TrimSpace(elems[0])

		switch tag {
		case "UNA":
			// Service string advice – no business data

		case "UNB":
			// UNB+SYNTAX:VER+SENDER+RECEIVER+DATE:TIME+CTRL
			doc.Envelope.Sender = getE(elems, 2)
			doc.Envelope.Receiver = getE(elems, 3)
			doc.Envelope.ControlNumber = getE(elems, 5)

		case "BGM":
			// BGM+MSGTYPE+DOCNUM+FUNCTION
			doc.Body.DocumentNumber = getE(elems, 2)

		case "DTM":
			// DTM+QUALIFIER:VALUE:FORMAT
			combo := getE(elems, 1)
			parts := strings.SplitN(combo, ":", 3)
			if len(parts) >= 2 && getComp(parts, 0) == "137" {
				doc.Body.OrderDate = parseEDIFACTDate(getComp(parts, 1), getComp(parts, 2))
			}

		case "NAD":
			// NAD+QUALIFIER+ID::QUAL++NAME
			qual := getE(elems, 1)
			idComposite := getE(elems, 2)
			idParts := strings.Split(idComposite, ":")
			id := getComp(idParts, 0)
			name := getE(elems, 4)
			if name == "" {
				name = getE(elems, 3) // some dialects put the name in NAD03
			}
			switch qual {
			case canonical.NADQualBY:
				doc.Body.BuyerParty = canonical.Party{ID: id, Name: name}
			case canonical.NADQualSE, canonical.NADQualSU:
				doc.Body.SellerParty = canonical.Party{ID: id, Name: name}
			}

		case "LIN":
			// LIN+LINENUM++ITEMID:QUALIFIER
			if currentLine != nil {
				lineItems = append(lineItems, *currentLine)
			}
			lineNum, _ := strconv.Atoi(getE(elems, 1))
			itemComposite := getE(elems, 3)
			itemParts := strings.Split(itemComposite, ":")
			productID := getComp(itemParts, 0)
			currentLine = &canonical.LineItem{
				LineNumber: lineNum,
				ProductID:  productID,
			}

		case "QTY":
			// QTY+QUALIFIER:AMOUNT:UOM
			if currentLine == nil {
				break
			}
			combo := getE(elems, 1)
			parts := strings.Split(combo, ":")
			qty, _ := strconv.ParseFloat(getComp(parts, 1), 64)
			uom := getComp(parts, 2)
			currentLine.Quantity = qty
			currentLine.UnitOfMeasure = uom

		case "PRI":
			// PRI+QUALIFIER:PRICE
			if currentLine == nil {
				break
			}
			combo := getE(elems, 1)
			parts := strings.Split(combo, ":")
			price, _ := strconv.ParseFloat(getComp(parts, 1), 64)
			currentLine.UnitPrice = price
		}
	}

	// Flush last line item
	if currentLine != nil {
		lineItems = append(lineItems, *currentLine)
	}
	doc.Body.LineItems = lineItems

	return doc, nil
}

// Encode maps a CanonicalDocument to EDIFACT ORDERS D.96A wire format.
func (c *EDIFACTCodec) Encode(doc *canonical.CanonicalDocument) (string, error) {
	if doc == nil {
		return "", fmt.Errorf("edifact: document cannot be nil")
	}

	ctrl := doc.Envelope.ControlNumber
	if ctrl == "" {
		ctrl = "1"
	}

	now := doc.Envelope.CreatedAt
	if now.IsZero() {
		now = time.Now()
	}
	dateStr := now.Format("060102")
	timeStr := now.Format("1504")

	orderDateStr := ""
	if !doc.Body.OrderDate.IsZero() {
		orderDateStr = doc.Body.OrderDate.Format("20060102")
	}

	var sb strings.Builder
	sb.WriteString("UNA:+.? '\n")
	sb.WriteString(fmt.Sprintf("UNB+IATB:1+%s+%s+%s:%s+%s'\n",
		doc.Envelope.Sender, doc.Envelope.Receiver, dateStr, timeStr, ctrl))
	sb.WriteString(fmt.Sprintf("UNH+1+ORDERS:D:96A:UN'\n"))
	sb.WriteString(fmt.Sprintf("BGM+220+%s+9'\n", doc.Body.DocumentNumber))

	if orderDateStr != "" {
		sb.WriteString(fmt.Sprintf("DTM+137:%s:102'\n", orderDateStr))
	}

	// Party segments
	if doc.Body.BuyerParty.ID != "" || doc.Body.BuyerParty.Name != "" {
		sb.WriteString(fmt.Sprintf("NAD+BY+%s::ZZ++%s'\n",
			doc.Body.BuyerParty.ID, doc.Body.BuyerParty.Name))
	}
	if doc.Body.SellerParty.ID != "" || doc.Body.SellerParty.Name != "" {
		sb.WriteString(fmt.Sprintf("NAD+SE+%s::ZZ++%s'\n",
			doc.Body.SellerParty.ID, doc.Body.SellerParty.Name))
	}

	// Line items
	for _, li := range doc.Body.LineItems {
		sb.WriteString(fmt.Sprintf("LIN+%d++%s:VP'\n", li.LineNumber, li.ProductID))
		sb.WriteString(fmt.Sprintf("QTY+21:%g:%s'\n", li.Quantity, li.UnitOfMeasure))
		sb.WriteString(fmt.Sprintf("PRI+AAA:%g'\n", li.UnitPrice))
	}

	// UNT counts segments from UNH to UNT inclusive
	// UNH + BGM + DTM(opt) + NADs + 3*lines + UNT
	segCount := 2 // UNH + BGM
	if orderDateStr != "" {
		segCount++
	}
	segCount += countEDIFACTNADSegments(doc)
	segCount += 3 * len(doc.Body.LineItems)
	segCount++ // UNT itself

	sb.WriteString(fmt.Sprintf("UNT+%d+1'\n", segCount))
	sb.WriteString(fmt.Sprintf("UNZ+1+%s'\n", ctrl))

	return sb.String(), nil
}

// ---- helpers ----------------------------------------------------------------

// splitEDIFACTSegments splits a raw EDIFACT document by ' treating
// release character ? as an escape (matched character is not a terminator).
// Newlines inside segments are normalised but do not act as terminators.
func splitEDIFACTSegments(raw string) []string {
	var segments []string
	var current strings.Builder
	runes := []rune(raw)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if ch == '?' && i+1 < len(runes) {
			// Release character – next character is literal
			i++
			current.WriteRune(runes[i])
			continue
		}
		if ch == '\'' {
			seg := strings.TrimSpace(current.String())
			if seg != "" {
				segments = append(segments, seg)
			}
			current.Reset()
			continue
		}
		if ch == '\n' {
			// Newlines within a segment are fine (just skip)
			continue
		}
		current.WriteRune(ch)
	}
	// Handle last segment if not terminated
	if seg := strings.TrimSpace(current.String()); seg != "" {
		segments = append(segments, seg)
	}
	return segments
}

// getE returns element at 1-based index from an EDIFACT element slice,
// or "" if out of range.
func getE(elems []string, idx int) string {
	if idx < len(elems) {
		return strings.TrimSpace(elems[idx])
	}
	return ""
}

// getComp returns component at 0-based index from a composite element,
// or "" if out of range.
func getComp(parts []string, idx int) string {
	if idx < len(parts) {
		return strings.TrimSpace(parts[idx])
	}
	return ""
}

// parseEDIFACTDate parses a date value given its format qualifier.
// Common qualifiers: 102 = CCYYMMDD, 101 = CCYYMMDD, 203 = CCYYMMDDHHMM
func parseEDIFACTDate(value, formatQual string) time.Time {
	switch formatQual {
	case "102", "101":
		t, err := time.Parse("20060102", value)
		if err == nil {
			return t
		}
	case "203":
		t, err := time.Parse("200601021504", value)
		if err == nil {
			return t
		}
	}
	// Fallback: try common formats
	for _, layout := range []string{"20060102", "060102", "2006-01-02"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

func countEDIFACTNADSegments(doc *canonical.CanonicalDocument) int {
	count := 0
	if doc.Body.BuyerParty.ID != "" || doc.Body.BuyerParty.Name != "" {
		count++
	}
	if doc.Body.SellerParty.ID != "" || doc.Body.SellerParty.Name != "" {
		count++
	}
	return count
}
