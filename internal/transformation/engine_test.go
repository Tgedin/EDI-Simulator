package transformation

import (
	"strings"
	"testing"

	"github.com/theo-gedin/edi-simulator/internal/transformation/canonical"
)

// ---- shared sample data ----------------------------------------------------

// sampleX12 is a minimal but realistic X12 850 Purchase Order.
const sampleX12 = `ISA*00*          *00*          *ZZ*SENDER         *ZZ*RECEIVER       *260215*1200*U*00401*000000001*0*T*>~
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

// sampleEDIFACT is a minimal but realistic EDIFACT ORDERS D.96A message.
const sampleEDIFACT = `UNA:+.? '
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
UNT+13+1'
UNZ+1+000000001'`

// ---- engine tests -----------------------------------------------------------

func newEngine() *TransformationEngine {
	return NewTransformationEngine()
}

func TestEngineTransform_X12ToEDIFACT(t *testing.T) {
	engine := newEngine()
	result, err := engine.Transform("x12", "edifact", sampleX12)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Structural presence
	if !strings.Contains(result.Output, "UNB") {
		t.Error("output missing UNB segment")
	}
	if !strings.Contains(result.Output, "BGM") {
		t.Error("output missing BGM segment")
	}
	if !strings.Contains(result.Output, "UNZ") {
		t.Error("output missing UNZ segment")
	}

	// Field-level: document number must be preserved
	if !strings.Contains(result.Output, "PO-12345") {
		t.Errorf("DocumentNumber not preserved; output:\n%s", result.Output)
	}

	// Canonical snapshot must be populated
	if result.Canonical == nil {
		t.Fatal("Canonical must not be nil")
	}
	if result.Canonical.Body.DocumentNumber != "PO-12345" {
		t.Errorf("canonical DocumentNumber = %q, want %q",
			result.Canonical.Body.DocumentNumber, "PO-12345")
	}
	if result.Canonical.Body.BuyerParty.ID != "BUYER01" {
		t.Errorf("canonical BuyerParty.ID = %q, want BUYER01",
			result.Canonical.Body.BuyerParty.ID)
	}
}

func TestEngineTransform_EDIFACTToX12(t *testing.T) {
	engine := newEngine()
	result, err := engine.Transform("edifact", "x12", sampleEDIFACT)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Output, "ISA") {
		t.Error("output missing ISA segment")
	}
	if !strings.Contains(result.Output, "BEG") {
		t.Error("output missing BEG segment")
	}
	if !strings.Contains(result.Output, "IEA") {
		t.Error("output missing IEA segment")
	}
	if !strings.Contains(result.Output, "PO-12345") {
		t.Errorf("DocumentNumber not preserved; output:\n%s", result.Output)
	}
	if result.Canonical.Body.BuyerParty.ID != "BUYER01" {
		t.Errorf("canonical BuyerParty.ID = %q, want BUYER01",
			result.Canonical.Body.BuyerParty.ID)
	}
}

func TestEngineTransform_X12ToXML(t *testing.T) {
	engine := newEngine()
	result, err := engine.Transform("x12", "xml", sampleX12)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "<Document>") {
		t.Error("expected <Document> root in XML output")
	}
	if !strings.Contains(result.Output, "PO-12345") {
		t.Error("DocumentNumber not preserved in XML output")
	}
}

// TestEngineTransform_RoundTrip is the critical round-trip test:
// X12 → canonical → EDIFACT → canonical → X12
// All business fields must survive both legs unchanged.
func TestEngineTransform_RoundTrip(t *testing.T) {
	engine := newEngine()

	// Leg 1: X12 → EDIFACT
	leg1, err := engine.Transform("x12", "edifact", sampleX12)
	if err != nil {
		t.Fatalf("leg 1 error: %v", err)
	}
	canon1 := leg1.Canonical

	// Leg 2: EDIFACT → X12
	leg2, err := engine.Transform("edifact", "x12", leg1.Output)
	if err != nil {
		t.Fatalf("leg 2 error (output was):\n%s\nerr: %v", leg1.Output, err)
	}
	canon2 := leg2.Canonical

	// DocumentNumber must be identical through both legs
	if canon1.Body.DocumentNumber != canon2.Body.DocumentNumber {
		t.Errorf("DocumentNumber: leg1=%q leg2=%q",
			canon1.Body.DocumentNumber, canon2.Body.DocumentNumber)
	}
	// BuyerParty.ID
	if canon1.Body.BuyerParty.ID != canon2.Body.BuyerParty.ID {
		t.Errorf("BuyerParty.ID: leg1=%q leg2=%q",
			canon1.Body.BuyerParty.ID, canon2.Body.BuyerParty.ID)
	}
	// SellerParty.ID
	if canon1.Body.SellerParty.ID != canon2.Body.SellerParty.ID {
		t.Errorf("SellerParty.ID: leg1=%q leg2=%q",
			canon1.Body.SellerParty.ID, canon2.Body.SellerParty.ID)
	}
	// LineItem[0].Quantity
	if len(canon1.Body.LineItems) == 0 || len(canon2.Body.LineItems) == 0 {
		t.Fatal("line items missing from canonical after round-trip")
	}
	if canon1.Body.LineItems[0].Quantity != canon2.Body.LineItems[0].Quantity {
		t.Errorf("LineItem[0].Quantity: leg1=%v leg2=%v",
			canon1.Body.LineItems[0].Quantity, canon2.Body.LineItems[0].Quantity)
	}
	if canon1.Body.LineItems[0].ProductID != canon2.Body.LineItems[0].ProductID {
		t.Errorf("LineItem[0].ProductID: leg1=%q leg2=%q",
			canon1.Body.LineItems[0].ProductID, canon2.Body.LineItems[0].ProductID)
	}

	// Final X12 output must contain correct document number
	if !strings.Contains(leg2.Output, "PO-12345") {
		t.Errorf("DocumentNumber missing from final X12 output:\n%s", leg2.Output)
	}
}

func TestEngineTransform_UnknownSourceFormat(t *testing.T) {
	engine := newEngine()
	_, err := engine.Transform("csv", "x12", "some,csv,data")
	if err == nil {
		t.Error("expected error for unknown source format")
	}
}

func TestEngineTransform_UnknownTargetFormat(t *testing.T) {
	engine := newEngine()
	_, err := engine.Transform("x12", "csv", sampleX12)
	if err == nil {
		t.Error("expected error for unknown target format")
	}
}

func TestEngineTransform_EmptyContent(t *testing.T) {
	engine := newEngine()
	_, err := engine.Transform("x12", "edifact", "")
	if err == nil {
		t.Error("expected error for empty content")
	}
}

// TestEngineRegisterCodec_Extensibility verifies that registering a new codec
// requires ZERO changes to engine.go — only one new file.
func TestEngineRegisterCodec_Extensibility(t *testing.T) {
	engine := newEngine()
	mock := &mockCodec{
		format: "mock",
		output: "MOCK_OUTPUT",
	}
	engine.RegisterCodec(mock)

	result, err := engine.Transform("mock", "x12", "anything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "ISA") {
		t.Errorf("expected X12 output, got: %s", result.Output)
	}

	// Reverse: x12 → mock
	result2, err := engine.Transform("x12", "mock", sampleX12)
	if err != nil {
		t.Fatalf("unexpected error for x12->mock: %v", err)
	}
	if result2.Output != "MOCK_OUTPUT" {
		t.Errorf("expected MOCK_OUTPUT, got %q", result2.Output)
	}
}

func TestEngineSupportedFormats(t *testing.T) {
	engine := newEngine()
	formats := engine.SupportedFormats()
	want := map[string]bool{"edifact": true, "x12": true, "xml": true}
	for _, f := range formats {
		if !want[f] {
			t.Errorf("unexpected format %q in SupportedFormats", f)
		}
		delete(want, f)
	}
	for missing := range want {
		t.Errorf("format %q missing from SupportedFormats", missing)
	}
}

func TestEngineSupportedTransformations(t *testing.T) {
	engine := newEngine()
	pairs := engine.SupportedTransformations()
	// 3 formats → 3×2 = 6 ordered pairs
	if len(pairs) != 6 {
		t.Errorf("expected 6 transformation pairs, got %d", len(pairs))
	}
	// All pairs must have non-empty "from" and "to" and they must differ
	for _, p := range pairs {
		if p["from"] == "" || p["to"] == "" {
			t.Errorf("pair has empty key: %v", p)
		}
		if p["from"] == p["to"] {
			t.Errorf("self-transformation pair: %v", p)
		}
	}
}

func TestCanonicalXML(t *testing.T) {
	engine := newEngine()
	result, err := engine.Transform("x12", "edifact", sampleX12)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	xmlStr, err := CanonicalXML(result.Canonical)
	if err != nil {
		t.Fatalf("CanonicalXML error: %v", err)
	}
	if !strings.Contains(xmlStr, "<Document>") {
		t.Error("expected <Document> in canonical XML")
	}
	if !strings.Contains(xmlStr, "PO-12345") {
		t.Error("DocumentNumber missing from canonical XML")
	}
}

// ---- mock codec (extensibility test) ----------------------------------------

type mockCodec struct {
	format string
	output string
}

func (m *mockCodec) Format() string { return m.format }

func (m *mockCodec) Decode(_ string) (*canonical.CanonicalDocument, error) {
	return &canonical.CanonicalDocument{}, nil
}

func (m *mockCodec) Encode(_ *canonical.CanonicalDocument) (string, error) {
	return m.output, nil
}
