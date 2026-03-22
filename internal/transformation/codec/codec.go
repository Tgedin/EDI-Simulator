package codec

import "github.com/theo-gedin/edi-simulator/internal/transformation/canonical"

// FormatCodec owns the full decode/encode responsibility for one EDI format.
// It handles both syntax tokenisation and field mapping to/from the canonical
// document. Adding a new format costs exactly one file implementing this
// interface; no existing code changes.
type FormatCodec interface {
	// Decode parses raw wire content and maps fields into a CanonicalDocument.
	Decode(raw string) (*canonical.CanonicalDocument, error)

	// Encode maps a CanonicalDocument back to wire format.
	Encode(doc *canonical.CanonicalDocument) (string, error)

	// Format returns the lowercase format identifier: "x12", "edifact", "xml".
	Format() string
}
