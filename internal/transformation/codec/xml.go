package codec

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/theo-gedin/edi-simulator/internal/transformation/canonical"
)

// XMLCodec implements FormatCodec for the canonical XML wire format.
// The input is expected to be a fully-structured CanonicalDocument XML;
// no manual segment parsing is required – encoding/xml does the work.
type XMLCodec struct{}

// Format returns the format identifier for this codec.
func (c *XMLCodec) Format() string { return "xml" }

// Decode unmarshals a canonical XML document into a CanonicalDocument struct.
func (c *XMLCodec) Decode(raw string) (*canonical.CanonicalDocument, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("xml: content cannot be empty")
	}

	doc := &canonical.CanonicalDocument{}
	if err := xml.Unmarshal([]byte(raw), doc); err != nil {
		return nil, fmt.Errorf("xml: failed to unmarshal document: %w", err)
	}

	// Minimal validation: a canonical XML document must have a <Document> root
	if doc.XMLName.Local != "Document" {
		return nil, fmt.Errorf("xml: root element must be <Document>, got <%s>", doc.XMLName.Local)
	}

	return doc, nil
}

// Encode marshals a CanonicalDocument to indented canonical XML.
func (c *XMLCodec) Encode(doc *canonical.CanonicalDocument) (string, error) {
	if doc == nil {
		return "", fmt.Errorf("xml: document cannot be nil")
	}

	// Ensure the XMLName is set correctly for serialisation
	doc.XMLName = xml.Name{Local: "Document"}

	b, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("xml: failed to marshal document: %w", err)
	}

	return xml.Header + string(b), nil
}
