package models

import (
	"encoding/json"
	"time"
)

// Message status constants
const (
	StatusPending     = "pending"
	StatusSent        = "sent"
	StatusReceived    = "received"
	StatusProcessed   = "processed"
	StatusFailed      = "failed"
	StatusTransformed = "transformed"
)

// Message represents a generic EDI message with flexible metadata
type Message struct {
	ID          string          `json:"id"`
	Format      string          `json:"format"`    // X12, EDIFACT, XML, JSON
	Content     string          `json:"content"`   // Raw message body
	Metadata    json.RawMessage `json:"metadata"`  // Dynamic fields per message type
	Sender      string          `json:"sender"`
	Receiver    string          `json:"receiver"`
	Status      string          `json:"status"`    // created, sent, received, validated, failed
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	PublishedAt *time.Time      `json:"published_at"` // When message was published to queue
}

// Transaction represents an audit event for a message
type Transaction struct {
	ID        string          `json:"id"`
	MessageID string          `json:"message_id"`
	Event     string          `json:"event"`
	Details   json.RawMessage `json:"details"`
	Timestamp time.Time       `json:"timestamp"`
}
