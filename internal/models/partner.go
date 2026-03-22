package models

import "time"

// TradingPartner represents a simulated company that sends or receives EDI messages.
type TradingPartner struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Country         string    `json:"country"`
	PreferredFormat string    `json:"preferred_format"` // x12, edifact, xml
	EDIQualifier    string    `json:"edi_qualifier"`    // e.g. "01", "ZZ"
	EDIID           string    `json:"edi_id"`           // e.g. "AUTOPARTS01"
	Active          bool      `json:"active"`
	CreatedAt       time.Time `json:"created_at"`
}
