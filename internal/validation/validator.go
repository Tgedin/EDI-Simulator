package validation

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// Validator interface for different format validators
type Validator interface {
	Validate(content string) error
	Format() string
}

// X12Validator validates X12 EDI messages
type X12Validator struct{}

// ValidateX12 checks if content is valid X12 format
func (v *X12Validator) Validate(content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("X12 content cannot be empty")
	}

	// Check for ISA segment (must start with ISA)
	if !strings.HasPrefix(content, "ISA") {
		return fmt.Errorf("X12 message must start with ISA segment")
	}

	// Check for GS segment
	if !strings.Contains(content, "GS") {
		return fmt.Errorf("X12 message must contain GS segment")
	}

	// Check for ST segment
	if !strings.Contains(content, "ST") {
		return fmt.Errorf("X12 message must contain ST segment")
	}

	// Check for SE segment (transaction set end)
	if !strings.Contains(content, "SE") {
		return fmt.Errorf("X12 message must contain SE segment")
	}

	// Check for GE segment (functional group end)
	if !strings.Contains(content, "GE") {
		return fmt.Errorf("X12 message must contain GE segment")
	}

	// Check for IEA segment (interchange end)
	if !strings.Contains(content, "IEA") {
		return fmt.Errorf("X12 message must contain IEA segment")
	}

	return nil
}

// Format returns the format name
func (v *X12Validator) Format() string {
	return "x12"
}

// EDIFACTValidator validates EDIFACT messages
type EDIFACTValidator struct{}

// ValidateEDIFACT checks if content is valid EDIFACT format
func (v *EDIFACTValidator) Validate(content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("EDIFACT content cannot be empty")
	}

	// Check for UNA segment (service string advice, optional but common)
	// or must have UNB (interchange header)
	if !strings.HasPrefix(content, "UNA") && !strings.HasPrefix(content, "UNB") {
		return fmt.Errorf("EDIFACT message must start with UNA or UNB segment")
	}

	// Check for UNB segment (interchange header)
	if !strings.Contains(content, "UNB") {
		return fmt.Errorf("EDIFACT message must contain UNB segment")
	}

	// Check for UNZ segment (interchange trailer)
	if !strings.Contains(content, "UNZ") {
		return fmt.Errorf("EDIFACT message must contain UNZ segment")
	}

	return nil
}

// Format returns the format name
func (v *EDIFACTValidator) Format() string {
	return "edifact"
}

// XMLValidator validates XML messages
type XMLValidator struct{}

// ValidateXML checks if content is valid XML format
func (v *XMLValidator) Validate(content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("XML content cannot be empty")
	}

	// Check basic XML structure
	if !strings.HasPrefix(content, "<") {
		return fmt.Errorf("XML must start with '<'")
	}

	if !strings.HasSuffix(content, ">") {
		return fmt.Errorf("XML must end with '>'")
	}

	// Try to parse as XML to validate well-formedness
	var xmlNode interface{}
	err := xml.Unmarshal([]byte(content), &xmlNode)
	if err != nil {
		return fmt.Errorf("XML malformed: %w", err)
	}

	return nil
}

// Format returns the format name
func (v *XMLValidator) Format() string {
	return "xml"
}

// GetValidator returns the appropriate validator for a format
func GetValidator(format string) (Validator, error) {
	switch strings.ToLower(format) {
	case "x12", "edi":
		return &X12Validator{}, nil
	case "edifact", "un":
		return &EDIFACTValidator{}, nil
	case "xml":
		return &XMLValidator{}, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// Validate is a convenience function for validating content in a specific format
func Validate(format string, content string) error {
	validator, err := GetValidator(format)
	if err != nil {
		return err
	}
	return validator.Validate(content)
}

// SupportedFormats returns list of supported formats
func SupportedFormats() []string {
	return []string{"x12", "edifact", "xml"}
}
