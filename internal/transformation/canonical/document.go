package canonical

import (
	"encoding/xml"
	"time"
)

// CanonicalDocument is the shared in-memory representation of any EDI document.
// It is the XML superset: every field from every supported format lives here.
// Missing fields from a given source stay zero-valued; no conditional logic is
// needed in the engine or codecs.
type CanonicalDocument struct {
	XMLName  xml.Name          `xml:"Document"`
	Envelope CanonicalEnvelope `xml:"Envelope"`
	Body     PurchaseOrder     `xml:"Body"`
}

// CanonicalEnvelope holds interchange-level routing and control information
// present in every supported format.
type CanonicalEnvelope struct {
	Sender        string    `xml:"Sender"`
	Receiver      string    `xml:"Receiver"`
	ControlNumber string    `xml:"ControlNumber"`
	CreatedAt     time.Time `xml:"CreatedAt"`
}

// PurchaseOrder covers X12 850, EDIFACT ORDERS, and any XML purchase-order
// equivalent. Phase 5 scope — ShipNotice and Invoice structs follow the same
// pattern and are deferred.
type PurchaseOrder struct {
	DocumentNumber string     `xml:"DocumentNumber"`
	OrderDate      time.Time  `xml:"OrderDate"`
	Currency       string     `xml:"Currency"`
	BuyerParty     Party      `xml:"BuyerParty"`
	SellerParty    Party      `xml:"SellerParty"`
	LineItems      []LineItem `xml:"LineItems>LineItem"`
}

// Party represents a trading partner (buyer or seller) extracted from any
// format's party segment.
type Party struct {
	ID      string `xml:"ID"`
	Name    string `xml:"Name"`
	Address string `xml:"Address"`
}

// LineItem represents a single order line extracted from PO1 (X12), LIN
// (EDIFACT), or the equivalent XML element.
type LineItem struct {
	LineNumber    int     `xml:"LineNumber"`
	ProductID     string  `xml:"ProductID"`
	Description   string  `xml:"Description"`
	Quantity      float64 `xml:"Quantity"`
	UnitPrice     float64 `xml:"UnitPrice"`
	UnitOfMeasure string  `xml:"UnitOfMeasure"`
}
