package transformation

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"

	"github.com/theo-gedin/edi-simulator/internal/transformation/canonical"
	"github.com/theo-gedin/edi-simulator/internal/transformation/codec"
)

// TransformResult carries the transformed wire output and the intermediate
// canonical document produced during the transformation. The canonical field
// is stored in Message.Metadata JSONB as an audit snapshot.
type TransformResult struct {
	// Output is the wire-format string in the target format.
	Output string
	// Canonical is the intermediate canonical document. Use xml.Marshal to
	// store it in Message.Metadata.
	Canonical *canonical.CanonicalDocument
}

// TransformationEngine is the central orchestrator for all format conversions.
// It maintains a registry of FormatCodec implementations keyed by their format
// identifier. Adding a new format costs exactly one file; no engine changes.
type TransformationEngine struct {
	codecs map[string]codec.FormatCodec
}

// NewTransformationEngine creates an engine pre-registered with the X12,
// EDIFACT, and XML codecs.
func NewTransformationEngine() *TransformationEngine {
	e := &TransformationEngine{
		codecs: make(map[string]codec.FormatCodec),
	}
	e.RegisterCodec(&codec.X12Codec{})
	e.RegisterCodec(&codec.EDIFACTCodec{})
	e.RegisterCodec(&codec.XMLCodec{})
	return e
}

// RegisterCodec adds or replaces the codec for the given format key.
// The key is taken from codec.Format() and normalised to lowercase.
func (e *TransformationEngine) RegisterCodec(c codec.FormatCodec) {
	e.codecs[strings.ToLower(c.Format())] = c
}

// Transform is the single entry point for format conversion.
//
// It:
//  1. Looks up the source codec and calls Decode(rawContent) → CanonicalDocument
//  2. Looks up the target codec and calls Encode(canonical) → wire output
//  3. Returns a TransformResult with both the output and the canonical snapshot
//
// Both source and target format strings are case-insensitive.
func (e *TransformationEngine) Transform(
	srcFormat, tgtFormat, rawContent string,
) (*TransformResult, error) {
	src := strings.ToLower(strings.TrimSpace(srcFormat))
	tgt := strings.ToLower(strings.TrimSpace(tgtFormat))

	if rawContent == "" {
		return nil, fmt.Errorf("transform: content cannot be empty")
	}

	srcCodec, ok := e.codecs[src]
	if !ok {
		return nil, fmt.Errorf("transform: unsupported source format %q", srcFormat)
	}
	tgtCodec, ok := e.codecs[tgt]
	if !ok {
		return nil, fmt.Errorf("transform: unsupported target format %q", tgtFormat)
	}

	// Decode: wire → canonical
	doc, err := srcCodec.Decode(rawContent)
	if err != nil {
		return nil, fmt.Errorf("transform: decode from %s failed: %w", srcFormat, err)
	}

	// Encode: canonical → wire
	output, err := tgtCodec.Encode(doc)
	if err != nil {
		return nil, fmt.Errorf("transform: encode to %s failed: %w", tgtFormat, err)
	}

	return &TransformResult{
		Output:    output,
		Canonical: doc,
	}, nil
}

// CanonicalXML serialises the canonical document to an XML string suitable for
// storage in Message.Metadata JSONB.
func CanonicalXML(doc *canonical.CanonicalDocument) (string, error) {
	if doc == nil {
		return "", fmt.Errorf("canonical: document cannot be nil")
	}
	b, err := xml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("canonical: marshal failed: %w", err)
	}
	return string(b), nil
}

// SupportedFormats returns a sorted slice of all registered format keys.
func (e *TransformationEngine) SupportedFormats() []string {
	keys := make([]string, 0, len(e.codecs))
	for k := range e.codecs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SupportedTransformations returns all possible ordered format pairs as
// {"from": srcFormat, "to": tgtFormat} maps. Every format can be decoded;
// every format can be encoded — so the set is all ordered pairs where src ≠ tgt.
func (e *TransformationEngine) SupportedTransformations() []map[string]string {
	formats := e.SupportedFormats()
	var pairs []map[string]string
	for _, src := range formats {
		for _, tgt := range formats {
			if src != tgt {
				pairs = append(pairs, map[string]string{"from": src, "to": tgt})
			}
		}
	}
	return pairs
}

// CountMappedFields returns the number of non-zero business fields present in a
// CanonicalDocument. Used by the preview API to report how many fields were
// successfully extracted from the source message.
func CountMappedFields(doc *canonical.CanonicalDocument) int {
	if doc == nil {
		return 0
	}
	count := 0
	// Envelope
	if doc.Envelope.Sender != "" {
		count++
	}
	if doc.Envelope.Receiver != "" {
		count++
	}
	if doc.Envelope.ControlNumber != "" {
		count++
	}
	if !doc.Envelope.CreatedAt.IsZero() {
		count++
	}
	// Body
	if doc.Body.DocumentNumber != "" {
		count++
	}
	if !doc.Body.OrderDate.IsZero() {
		count++
	}
	if doc.Body.Currency != "" {
		count++
	}
	if doc.Body.BuyerParty.ID != "" {
		count++
	}
	if doc.Body.BuyerParty.Name != "" {
		count++
	}
	if doc.Body.SellerParty.ID != "" {
		count++
	}
	if doc.Body.SellerParty.Name != "" {
		count++
	}
	// Line items
	for _, li := range doc.Body.LineItems {
		if li.ProductID != "" {
			count++
		}
		if li.Quantity != 0 {
			count++
		}
		if li.UnitPrice != 0 {
			count++
		}
		if li.UnitOfMeasure != "" {
			count++
		}
	}
	return count
}
