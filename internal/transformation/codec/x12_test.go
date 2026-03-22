package codec

import (
	"strings"
	"testing"
)

const x12Sample = `ISA*00*          *00*          *ZZ*SENDER         *ZZ*RECEIVER       *260215*1200*U*00401*000000001*0*T*>~
GS*OE*SENDER*RECEIVER*20260215*1200*1*X*004010~
ST*850*0001~
BEG*00*SA*PO-12345**20260215~
N1*BY*Buyer Corp*ZZ*BUYER01~
N1*SE*Seller Inc*ZZ*SELLER01~
PO1*1*10*EA*9.99**VP*WIDGET-A~
PO1*2*5*EA*19.99**VP*WIDGET-B~
SE*8*0001~
GE*1*1~
IEA*1*000000001~`

func TestX12CodecFormat(t *testing.T) {
	c := &X12Codec{}
	if c.Format() != "x12" {
		t.Errorf("Format() = %q, want %q", c.Format(), "x12")
	}
}

func TestX12CodecDecode_PurchaseOrder(t *testing.T) {
	c := &X12Codec{}
	doc, err := c.Decode(x12Sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if doc.Envelope.Sender != "SENDER" {
		t.Errorf("Sender = %q, want %q", doc.Envelope.Sender, "SENDER")
	}
	if doc.Envelope.Receiver != "RECEIVER" {
		t.Errorf("Receiver = %q, want %q", doc.Envelope.Receiver, "RECEIVER")
	}
	if doc.Envelope.ControlNumber != "000000001" {
		t.Errorf("ControlNumber = %q, want %q", doc.Envelope.ControlNumber, "000000001")
	}
	if doc.Body.DocumentNumber != "PO-12345" {
		t.Errorf("DocumentNumber = %q, want %q", doc.Body.DocumentNumber, "PO-12345")
	}
	if doc.Body.BuyerParty.ID != "BUYER01" {
		t.Errorf("BuyerParty.ID = %q, want %q", doc.Body.BuyerParty.ID, "BUYER01")
	}
	if doc.Body.BuyerParty.Name != "Buyer Corp" {
		t.Errorf("BuyerParty.Name = %q, want %q", doc.Body.BuyerParty.Name, "Buyer Corp")
	}
	if doc.Body.SellerParty.ID != "SELLER01" {
		t.Errorf("SellerParty.ID = %q, want %q", doc.Body.SellerParty.ID, "SELLER01")
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
}

func TestX12CodecDecode_MissingISA(t *testing.T) {
	c := &X12Codec{}
	_, err := c.Decode("GS*OE*SENDER*RECEIVER*20260215*1200*1*X*004010~")
	if err == nil {
		t.Error("expected error for non-X12 input (no ISA)")
	}
}

func TestX12CodecDecode_Empty(t *testing.T) {
	c := &X12Codec{}
	_, err := c.Decode("")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestX12CodecEncode_RoundTrip(t *testing.T) {
	c := &X12Codec{}
	doc, err := c.Decode(x12Sample)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	out, err := c.Encode(doc)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	for _, want := range []string{"ISA", "GS", "ST", "BEG", "IEA", "PO-12345"} {
		if !strings.Contains(out, want) {
			t.Errorf("encoded output missing %q\noutput:\n%s", want, out)
		}
	}

	// Re-decode the encoded output – must give same document number
	doc2, err := c.Decode(out)
	if err != nil {
		t.Fatalf("re-decode error: %v\noutput:\n%s", err, out)
	}
	if doc2.Body.DocumentNumber != doc.Body.DocumentNumber {
		t.Errorf("DocumentNumber: original=%q re-decoded=%q",
			doc.Body.DocumentNumber, doc2.Body.DocumentNumber)
	}
	if len(doc2.Body.LineItems) != len(doc.Body.LineItems) {
		t.Errorf("LineItems: original=%d re-decoded=%d",
			len(doc.Body.LineItems), len(doc2.Body.LineItems))
	}
}

func TestX12CodecEncode_NilDoc(t *testing.T) {
	c := &X12Codec{}
	_, err := c.Encode(nil)
	if err == nil {
		t.Error("expected error for nil document")
	}
}
