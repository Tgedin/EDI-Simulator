package validation

import (
	"testing"
)

// TestX12ValidatorValid tests X12 validation with valid messages
func TestX12ValidatorValid(t *testing.T) {
	validator := &X12Validator{}

	validX12 := `ISA*00*          *00*          *01*9876543210     *01*1234567890     *220215*1430*^*00501*000000001*0*T*:
GS*PO*9876543210*1234567890*20220215*143000*1*X*005010
ST*850*000001
BEG*00*NE*ORDER123**220215
DTM*002*20220215
N1*BY*Buyer Company
N1*SU*Supplier Company
PO1*1*100*EA*10.00*CB*VN*SKUABC123
CTT*1
SE*9*000001
GE*1*1
IEA*1*000000001`

	err := validator.Validate(validX12)
	if err != nil {
		t.Errorf("Expected valid X12 message, got error: %v", err)
	}
}

// TestX12ValidatorMissingISA tests X12 validation without ISA segment
func TestX12ValidatorMissingISA(t *testing.T) {
	validator := &X12Validator{}

	invalidX12 := `GS*PO*9876543210*1234567890*20220215*143000*1*X*005010
ST*850*000001
SE*2*000001
GE*1*1
IEA*1*000000001`

	err := validator.Validate(invalidX12)
	if err == nil {
		t.Error("Expected error for missing ISA segment, got none")
	}

	if err.Error() != "X12 message must start with ISA segment" {
		t.Errorf("Expected 'X12 message must start with ISA segment', got: %v", err)
	}
}

// TestX12ValidatorMissingGS tests X12 validation without GS segment
func TestX12ValidatorMissingGS(t *testing.T) {
	validator := &X12Validator{}

	invalidX12 := `ISA*00*          *00*          *01*9876543210     *01*1234567890     *220215*1430*^*00501*000000001*0*T*:
ST*850*000001
SE*2*000001
GE*1*1
IEA*1*000000001`

	err := validator.Validate(invalidX12)
	if err == nil {
		t.Error("Expected error for missing GS segment, got none")
	}
}

// TestX12ValidatorMissingMandatorySegments tests X12 validation with missing segments
func TestX12ValidatorMissingMandatorySegments(t *testing.T) {
	validator := &X12Validator{}

	tests := []struct {
		name        string
		content     string
		expectedErr string
	}{
		{
			name:        "missing ST segment",
			content:     "ISA*...*GS*...*SE*...*GE*...*IEA*...",
			expectedErr: "X12 message must contain ST segment",
		},
		{
			name:        "missing SE segment",
			content:     "ISA*...*GS*...*ST*...*GE*...*IEA*...",
			expectedErr: "X12 message must contain SE segment",
		},
		{
			name:        "missing GE segment",
			content:     "ISA*...*GS*...*ST*...*SE*...*IEA*...",
			expectedErr: "X12 message must contain GE segment",
		},
		{
			name:        "missing IEA segment",
			content:     "ISA*...*GS*...*ST*...*SE*...*GE*...",
			expectedErr: "X12 message must contain IEA segment",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validator.Validate(tc.content)
			if err == nil {
				t.Errorf("Expected error: %s, got none", tc.expectedErr)
			}
			if err.Error() != tc.expectedErr {
				t.Errorf("Expected: %s, got: %v", tc.expectedErr, err)
			}
		})
	}
}

// TestX12ValidatorEmpty tests X12 validation with empty content
func TestX12ValidatorEmpty(t *testing.T) {
	validator := &X12Validator{}

	err := validator.Validate("")
	if err == nil {
		t.Error("Expected error for empty content, got none")
	}

	if err.Error() != "X12 content cannot be empty" {
		t.Errorf("Expected 'X12 content cannot be empty', got: %v", err)
	}
}

// TestX12ValidatorFormat tests X12 format name
func TestX12ValidatorFormat(t *testing.T) {
	validator := &X12Validator{}
	expected := "x12"
	if fmt := validator.Format(); fmt != expected {
		t.Errorf("Expected format: %s, got: %s", expected, fmt)
	}
}

// TestEDIFACTValidatorValid tests EDIFACT validation with valid messages
func TestEDIFACTValidatorValid(t *testing.T) {
	validator := &EDIFACTValidator{}

	validEDIFACT := `UNA:+.? '
UNB+IATB:1+1SNDPROC+2RCVPROC+2602151200:1234+1+ORDERS'
UNH+1+ORDERS:D:96A:UN
BGM+220+ORDER123+9
DTM+137:20260215:102
UNT+4+1
UNZ+1+1`

	err := validator.Validate(validEDIFACT)
	if err != nil {
		t.Errorf("Expected valid EDIFACT message, got error: %v", err)
	}
}

// TestEDIFACTValidatorWithoutUNA tests EDIFACT validation without UNA segment
func TestEDIFACTValidatorWithoutUNA(t *testing.T) {
	validator := &EDIFACTValidator{}

	validEDIFACT := `UNB+IATB:1+1SNDPROC+2RCVPROC+2602151200:1234+1+ORDERS'
UNH+1+ORDERS:D:96A:UN
BGM+220+ORDER123+9
DTM+137:20260215:102
UNT+4+1
UNZ+1+1`

	err := validator.Validate(validEDIFACT)
	if err != nil {
		t.Errorf("Expected valid EDIFACT message without UNA, got error: %v", err)
	}
}

// TestEDIFACTValidatorMissingUNB tests EDIFACT validation without UNB segment
func TestEDIFACTValidatorMissingUNB(t *testing.T) {
	validator := &EDIFACTValidator{}

	invalidEDIFACT := `UNA:+.? '
UNH+1+ORDERS:D:96A:UN
BGM+220+ORDER123+9
UNT+4+1
UNZ+1+1`

	err := validator.Validate(invalidEDIFACT)
	if err == nil {
		t.Error("Expected error for missing UNB segment, got none")
	}

	if err.Error() != "EDIFACT message must contain UNB segment" {
		t.Errorf("Expected 'EDIFACT message must contain UNB segment', got: %v", err)
	}
}

// TestEDIFACTValidatorMissingUNZ tests EDIFACT validation without UNZ segment
func TestEDIFACTValidatorMissingUNZ(t *testing.T) {
	validator := &EDIFACTValidator{}

	invalidEDIFACT := `UNA:+.? '
UNB+IATB:1+1SNDPROC+2RCVPROC+2602151200:1234+1+ORDERS'
UNH+1+ORDERS:D:96A:UN
BGM+220+ORDER123+9
UNT+4+1`

	err := validator.Validate(invalidEDIFACT)
	if err == nil {
		t.Error("Expected error for missing UNZ segment, got none")
	}

	if err.Error() != "EDIFACT message must contain UNZ segment" {
		t.Errorf("Expected 'EDIFACT message must contain UNZ segment', got: %v", err)
	}
}

// TestEDIFACTValidatorEmpty tests EDIFACT validation with empty content
func TestEDIFACTValidatorEmpty(t *testing.T) {
	validator := &EDIFACTValidator{}

	err := validator.Validate("")
	if err == nil {
		t.Error("Expected error for empty content, got none")
	}

	if err.Error() != "EDIFACT content cannot be empty" {
		t.Errorf("Expected 'EDIFACT content cannot be empty', got: %v", err)
	}
}

// TestEDIFACTValidatorFormat tests EDIFACT format name
func TestEDIFACTValidatorFormat(t *testing.T) {
	validator := &EDIFACTValidator{}
	expected := "edifact"
	if fmt := validator.Format(); fmt != expected {
		t.Errorf("Expected format: %s, got: %s", expected, fmt)
	}
}

// TestXMLValidatorValid tests XML validation with valid XML
func TestXMLValidatorValid(t *testing.T) {
	validator := &XMLValidator{}

	validXML := `<?xml version="1.0" encoding="UTF-8"?>
<Message>
	<Format>x12</Format>
	<Content>ISA*00*...*IEA*1*000000001</Content>
</Message>`

	err := validator.Validate(validXML)
	if err != nil {
		t.Errorf("Expected valid XML, got error: %v", err)
	}
}

// TestXMLValidatorInvalid tests XML validation with invalid XML
func TestXMLValidatorInvalid(t *testing.T) {
	validator := &XMLValidator{}

	invalidXML := `<?xml version="1.0" encoding="UTF-8"?>
<Message>
	<Format>x12</Format>
	<Content>ISA*00*...*IEA*1*000000001</Content>
</Message`

	err := validator.Validate(invalidXML)
	if err == nil {
		t.Error("Expected error for invalid XML, got none")
	}
}

// TestXMLValidatorEmpty tests XML validation with empty content
func TestXMLValidatorEmpty(t *testing.T) {
	validator := &XMLValidator{}

	err := validator.Validate("")
	if err == nil {
		t.Error("Expected error for empty content, got none")
	}

	if err.Error() != "XML content cannot be empty" {
		t.Errorf("Expected 'XML content cannot be empty', got: %v", err)
	}
}

// TestXMLValidatorFormat tests XML format name
func TestXMLValidatorFormat(t *testing.T) {
	validator := &XMLValidator{}
	expected := "xml"
	if fmt := validator.Format(); fmt != expected {
		t.Errorf("Expected format: %s, got: %s", expected, fmt)
	}
}

// TestGetValidator tests the validator factory function
func TestGetValidator(t *testing.T) {
	tests := []struct {
		format        string
		expectedType  string
		shouldBeFound bool
	}{
		{"x12", "X12", true},
		{"X12", "X12", true},
		{"edifact", "EDIFACT", true},
		{"EDIFACT", "EDIFACT", true},
		{"xml", "XML", true},
		{"XML", "XML", true},
		{"json", "", false},
		{"invalid", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			validator, err := GetValidator(tc.format)
			if tc.shouldBeFound {
				if err != nil {
					t.Errorf("Expected to find validator for format '%s', got error: %v", tc.format, err)
				}
				if validator == nil {
					t.Errorf("Expected non-nil validator for format '%s'", tc.format)
				}
				if validator.Format() != tc.format && validator.Format() != tc.format {
					t.Logf("Validator format: %s", validator.Format())
				}
			} else {
				if err == nil {
					t.Errorf("Expected error for unsupported format '%s', got none", tc.format)
				}
			}
		})
	}
}

// TestValidateFunction tests the main Validate function
func TestValidateFunction(t *testing.T) {
	validX12 := `ISA*00*          *00*          *01*9876543210     *01*1234567890     *220215*1430*^*00501*000000001*0*T*:
GS*PO*9876543210*1234567890*20220215*143000*1*X*005010
ST*850*000001
SE*2*000001
GE*1*1
IEA*1*000000001`

	validEDIFACT := `UNA:+.? '
UNB+IATB:1+1SNDPROC+2RCVPROC+2602151200:1234+1+ORDERS'
UNH+1+ORDERS:D:96A:UN
UNT+1+1
UNZ+1+1`

	validXML := `<?xml version="1.0"?><root><item>test</item></root>`

	tests := []struct {
		name        string
		format      string
		content     string
		shouldError bool
	}{
		{"valid x12", "x12", validX12, false},
		{"valid edifact", "edifact", validEDIFACT, false},
		{"valid xml", "xml", validXML, false},
		{"invalid format", "json", "{}", true},
		{"empty content", "x12", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.format, tc.content)
			if tc.shouldError && err == nil {
				t.Error("Expected error, got none")
			}
			if !tc.shouldError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}
