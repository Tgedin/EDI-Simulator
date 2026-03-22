package codec

import (
	"strings"
	"testing"
)

const edifactSample = `UNA:+.? '
UNB+IATB:1+SENDER+RECEIVER+260215:1200+000000001'
UNH+1+ORDERS:D:96A:UN'
BGM+220+PO-12345+9'
DTM+137:20260215:102'
NAD+BY+BUYER01::ZZ++Buyer Corp'
NAD+SE+SELLER01::ZZ++Seller Inc'
LIN+1++WIDGET-A:VP'
QTY+21:10:EA'
PRI+AAA:9.99'
LIN+2++WIDGET-B:VP'
QTY+21:5:EA'
PRI+AAA:19.99'
UNT+12+1'
UNZ+1+000000001'`

func TestEDIFACTCodecFormat(t *testing.T) {
	c := &EDIFACTCodec{}
	if c.Format() != "edifact" {
		t.Errorf("Format() = %q, want %q", c.Format(), "edifact")
	}
}

func TestEDIFACTCodecDecode_PurchaseOrder(t *testing.T) {
	c := &EDIFACTCodec{}
	doc, err := c.Decode(edifactSample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if doc.Envelope.Sender != "SENDER" {
		t.Errorf("Sender = %q, want SENDER", doc.Envelope.Sender)
	}
	if doc.Envelope.Receiver != "RECEIVER" {
		t.Errorf("Receiver = %q, want RECEIVER", doc.Envelope.Receiver)
	}
	if doc.Envelope.ControlNumber != "000000001" {
		t.Errorf("ControlNumber = %q, want 000000001", doc.Envelope.ControlNumber)
	}
	if doc.Body.DocumentNumber != "PO-12345" {
		t.Errorf("DocumentNumber = %q, want PO-12345", doc.Body.DocumentNumber)
	}
	if doc.Body.BuyerParty.ID != "BUYER01" {
		t.Errorf("BuyerParty.ID = %q, want BUYER01", doc.Body.BuyerParty.ID)
	}
	if doc.Body.BuyerParty.Name != "Buyer Corp" {
		t.Errorf("BuyerParty.Name = %q, want %q", doc.Body.BuyerParty.Name, "Buyer Corp")
	}
	if doc.Body.SellerParty.ID != "SELLER01" {
		t.Errorf("SellerParty.ID = %q, want SELLER01", doc.Body.SellerParty.ID)
	}
	if len(doc.Body.LineItems) != 2 {
		t.Fatalf("LineItems count = %d, want 2", len(doc.Body.LineItems))
	}

	li := doc.Body.LineItems[0]
	if li.ProductID != "WIDGET-A" {
		t.Errorf("LineItem[0].ProductID = %q, want WIDGET-A", li.ProductID)
	}
	if li.Quantity != 10 {
		t.Errorf("LineItem[0].Quantity = %v, want 10", li.Quantity)
	}
	if li.UnitPrice != 9.99 {
		t.Errorf("LineItem[0].UnitPrice = %v, want 9.99", li.UnitPrice)
	}
	if li.UnitOfMeasure != "EA" {
		t.Errorf("LineItem[0].UnitOfMeasure = %q, want EA", li.UnitOfMeasure)
	}

	li2 := doc.Body.LineItems[1]
	if li2.ProductID != "WIDGET-B" {
		t.Errorf("LineItem[1].ProductID = %q, want WIDGET-B", li2.ProductID)
	}
	if li2.Quantity != 5 {
		t.Errorf("LineItem[1].Quantity = %v, want 5", li2.Quantity)
	}
}

func TestEDIFACTCodecDecode_MissingUNB(t *testing.T) {
	c := &EDIFACTCodec{}
	_, err := c.Decode("BGM+220+PO-12345+9'")
	if err == nil {
		t.Error("expected error for input not starting with UNA/UNB")
	}
}

func TestEDIFACTCodecDecode_Empty(t *testing.T) {
	c := &EDIFACTCodec{}
	_, err := c.Decode("")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestEDIFACTCodecEncode_RoundTrip(t *testing.T) {
	c := &EDIFACTCodec{}
	doc, err := c.Decode(edifactSample)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	out, err := c.Encode(doc)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	for _, want := range []string{"UNB", "UNH", "BGM", "UNT", "UNZ", "PO-12345"} {
		if !strings.Contains(out, want) {
			t.Errorf("encoded output missing %q\noutput:\n%s", want, out)
		}
	}

	// Re-decode and verify fields survive
	doc2, err := c.Decode(out)
	if err != nil {
		t.Fatalf("re-decode error: %v\noutput:\n%s", err, out)
	}
	if doc2.Body.DocumentNumber != doc.Body.DocumentNumber {
		t.Errorf("DocumentNumber: original=%q re-decoded=%q",
			doc.Body.DocumentNumber, doc2.Body.DocumentNumber)
	}
	if doc2.Body.BuyerParty.ID != doc.Body.BuyerParty.ID {
		t.Errorf("BuyerParty.ID: original=%q re-decoded=%q",
			doc.Body.BuyerParty.ID, doc2.Body.BuyerParty.ID)
	}
	if len(doc2.Body.LineItems) != len(doc.Body.LineItems) {
		t.Errorf("LineItems: original=%d re-decoded=%d",
			len(doc.Body.LineItems), len(doc2.Body.LineItems))
	}
	if len(doc2.Body.LineItems) > 0 {
		if doc2.Body.LineItems[0].Quantity != doc.Body.LineItems[0].Quantity {
			t.Errorf("LineItem[0].Quantity: original=%v re-decoded=%v",
				doc.Body.LineItems[0].Quantity, doc2.Body.LineItems[0].Quantity)
		}
	}
}

func TestEDIFACTCodecEncode_NilDoc(t *testing.T) {
	c := &EDIFACTCodec{}
	_, err := c.Encode(nil)
	if err == nil {
		t.Error("expected error for nil document")
	}
}
