package codec

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/theo-gedin/edi-simulator/internal/transformation/canonical"
)

func sampleCanonicalDoc() *canonical.CanonicalDocument {
	return &canonical.CanonicalDocument{
		XMLName: xml.Name{Local: "Document"},
		Envelope: canonical.CanonicalEnvelope{
			Sender:        "SENDER",
			Receiver:      "RECEIVER",
			ControlNumber: "000000001",
			CreatedAt:     time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC),
		},
		Body: canonical.PurchaseOrder{
			DocumentNumber: "PO-12345",
			OrderDate:      time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
			BuyerParty:     canonical.Party{ID: "BUYER01", Name: "Buyer Corp"},
			SellerParty:    canonical.Party{ID: "SELLER01", Name: "Seller Inc"},
			LineItems: []canonical.LineItem{
				{LineNumber: 1, ProductID: "WIDGET-A", Quantity: 10, UnitPrice: 9.99, UnitOfMeasure: "EA"},
				{LineNumber: 2, ProductID: "WIDGET-B", Quantity: 5, UnitPrice: 19.99, UnitOfMeasure: "EA"},
			},
		},
	}
}

func TestXMLCodecFormat(t *testing.T) {
	c := &XMLCodec{}
	if c.Format() != "xml" {
		t.Errorf("Format() = %q, want xml", c.Format())
	}
}

func TestXMLCodecEncode_ValidDocument(t *testing.T) {
	c := &XMLCodec{}
	doc := sampleCanonicalDoc()
	out, err := c.Encode(doc)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	for _, want := range []string{
		"<?xml", "<Document>", "</Document>",
		"PO-12345", "BUYER01", "Buyer Corp", "WIDGET-A",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("encoded XML missing %q\noutput:\n%s", want, out)
		}
	}
}

func TestXMLCodecDecode_ValidDocument(t *testing.T) {
	c := &XMLCodec{}
	// First encode, then decode
	doc := sampleCanonicalDoc()
	encoded, err := c.Encode(doc)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if decoded.Body.DocumentNumber != "PO-12345" {
		t.Errorf("DocumentNumber = %q, want PO-12345", decoded.Body.DocumentNumber)
	}
	if decoded.Body.BuyerParty.ID != "BUYER01" {
		t.Errorf("BuyerParty.ID = %q, want BUYER01", decoded.Body.BuyerParty.ID)
	}
	if decoded.Body.SellerParty.ID != "SELLER01" {
		t.Errorf("SellerParty.ID = %q, want SELLER01", decoded.Body.SellerParty.ID)
	}
	if len(decoded.Body.LineItems) != 2 {
		t.Fatalf("LineItems = %d, want 2", len(decoded.Body.LineItems))
	}
	if decoded.Body.LineItems[0].Quantity != 10 {
		t.Errorf("LineItem[0].Quantity = %v, want 10", decoded.Body.LineItems[0].Quantity)
	}
	if decoded.Body.LineItems[0].UnitPrice != 9.99 {
		t.Errorf("LineItem[0].UnitPrice = %v, want 9.99", decoded.Body.LineItems[0].UnitPrice)
	}
}

func TestXMLCodecDecode_Malformed(t *testing.T) {
	c := &XMLCodec{}
	_, err := c.Decode("this is not xml at all <<<")
	if err == nil {
		t.Error("expected error for malformed XML")
	}
}

func TestXMLCodecDecode_WrongRoot(t *testing.T) {
	c := &XMLCodec{}
	_, err := c.Decode(`<?xml version="1.0"?><Message><Body/></Message>`)
	if err == nil {
		t.Error("expected error for wrong root element")
	}
}

func TestXMLCodecDecode_Empty(t *testing.T) {
	c := &XMLCodec{}
	_, err := c.Decode("")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestXMLCodecEncode_NilDoc(t *testing.T) {
	c := &XMLCodec{}
	_, err := c.Encode(nil)
	if err == nil {
		t.Error("expected error for nil document")
	}
}

func TestXMLCodecRoundTrip_AllFields(t *testing.T) {
	c := &XMLCodec{}
	original := sampleCanonicalDoc()

	encoded, err := c.Encode(original)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	// Envelope
	if decoded.Envelope.Sender != original.Envelope.Sender {
		t.Errorf("Sender: %q vs %q", decoded.Envelope.Sender, original.Envelope.Sender)
	}
	if decoded.Envelope.ControlNumber != original.Envelope.ControlNumber {
		t.Errorf("ControlNumber: %q vs %q",
			decoded.Envelope.ControlNumber, original.Envelope.ControlNumber)
	}

	// Body
	if decoded.Body.DocumentNumber != original.Body.DocumentNumber {
		t.Errorf("DocumentNumber: %q vs %q",
			decoded.Body.DocumentNumber, original.Body.DocumentNumber)
	}
	if decoded.Body.BuyerParty.Name != original.Body.BuyerParty.Name {
		t.Errorf("BuyerParty.Name: %q vs %q",
			decoded.Body.BuyerParty.Name, original.Body.BuyerParty.Name)
	}

	// LineItems
	if len(decoded.Body.LineItems) != len(original.Body.LineItems) {
		t.Fatalf("LineItems length: %d vs %d",
			len(decoded.Body.LineItems), len(original.Body.LineItems))
	}
	for i, li := range original.Body.LineItems {
		got := decoded.Body.LineItems[i]
		if got.ProductID != li.ProductID {
			t.Errorf("LineItem[%d].ProductID: %q vs %q", i, got.ProductID, li.ProductID)
		}
		if got.Quantity != li.Quantity {
			t.Errorf("LineItem[%d].Quantity: %v vs %v", i, got.Quantity, li.Quantity)
		}
		if got.UnitPrice != li.UnitPrice {
			t.Errorf("LineItem[%d].UnitPrice: %v vs %v", i, got.UnitPrice, li.UnitPrice)
		}
	}
}
