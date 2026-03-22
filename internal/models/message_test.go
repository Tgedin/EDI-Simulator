package models

import (
	"encoding/json"
	"testing"
	"time"
)

// TestMessageStatusConstants tests that all status constants are defined
func TestMessageStatusConstants(t *testing.T) {
	tests := []struct {
		constant string
		expected string
	}{
		{"StatusPending", StatusPending},
		{"StatusSent", StatusSent},
		{"StatusReceived", StatusReceived},
		{"StatusProcessed", StatusProcessed},
		{"StatusFailed", StatusFailed},
	}

	expectedValues := map[string]bool{
		"pending":   true,
		"sent":      true,
		"received":  true,
		"processed": true,
		"failed":    true,
	}

	for _, tt := range tests {
		if !expectedValues[tt.expected] {
			t.Errorf("Status %s has unexpected value: %s", tt.constant, tt.expected)
		}
	}
}

// TestStatusPendingConstant tests the pending status constant
func TestStatusPendingConstant(t *testing.T) {
	if StatusPending != "pending" {
		t.Errorf("Expected 'pending', got '%s'", StatusPending)
	}
}

// TestStatusSentConstant tests the sent status constant
func TestStatusSentConstant(t *testing.T) {
	if StatusSent != "sent" {
		t.Errorf("Expected 'sent', got '%s'", StatusSent)
	}
}

// TestStatusReceivedConstant tests the received status constant
func TestStatusReceivedConstant(t *testing.T) {
	if StatusReceived != "received" {
		t.Errorf("Expected 'received', got '%s'", StatusReceived)
	}
}

// TestStatusProcessedConstant tests the processed status constant
func TestStatusProcessedConstant(t *testing.T) {
	if StatusProcessed != "processed" {
		t.Errorf("Expected 'processed', got '%s'", StatusProcessed)
	}
}

// TestStatusFailedConstant tests the failed status constant
func TestStatusFailedConstant(t *testing.T) {
	if StatusFailed != "failed" {
		t.Errorf("Expected 'failed', got '%s'", StatusFailed)
	}
}

// TestMessageCreation tests creating a message struct
func TestMessageCreation(t *testing.T) {
	now := time.Now()
	metadata := json.RawMessage(`{"order_number":"123"}`)

	msg := &Message{
		ID:        "msg-1",
		Format:    "x12",
		Content:   "ISA*00*...*IEA*1*000000001",
		Metadata:  metadata,
		Sender:    "SENDER",
		Receiver:  "RECEIVER",
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if msg.ID != "msg-1" {
		t.Errorf("Expected ID 'msg-1', got '%s'", msg.ID)
	}

	if msg.Format != "x12" {
		t.Errorf("Expected format 'x12', got '%s'", msg.Format)
	}

	if msg.Status != StatusPending {
		t.Errorf("Expected status '%s', got '%s'", StatusPending, msg.Status)
	}

	if msg.Sender != "SENDER" {
		t.Errorf("Expected sender 'SENDER', got '%s'", msg.Sender)
	}

	if msg.Receiver != "RECEIVER" {
		t.Errorf("Expected receiver 'RECEIVER', got '%s'", msg.Receiver)
	}
}

// TestMessageStatusTransitions tests message status transitions
func TestMessageStatusTransitions(t *testing.T) {
	msg := &Message{
		ID:     "msg-1",
		Status: StatusPending,
	}

	transitions := []string{StatusPending, StatusSent, StatusReceived, StatusProcessed}

	for i, expectedStatus := range transitions {
		if msg.Status != expectedStatus {
			t.Errorf("Step %d: Expected status '%s', got '%s'", i, expectedStatus, msg.Status)
		}
		// Simulate status update
		if i < len(transitions)-1 {
			msg.Status = transitions[i+1]
		}
	}
}

// TestMessageFailedStatus tests message can have failed status
func TestMessageFailedStatus(t *testing.T) {
	msg := &Message{
		ID:     "msg-1",
		Status: StatusPending,
	}

	msg.Status = StatusFailed

	if msg.Status != StatusFailed {
		t.Errorf("Expected status '%s', got '%s'", StatusFailed, msg.Status)
	}
}

// TestMessageMetadata tests message metadata handling
func TestMessageMetadata(t *testing.T) {
	metadata := json.RawMessage(`{"order_number":"ORD-123","priority":"high"}`)

	msg := &Message{
		ID:       "msg-1",
		Metadata: metadata,
	}

	var parsedMetadata map[string]interface{}
	err := json.Unmarshal(msg.Metadata, &parsedMetadata)
	if err != nil {
		t.Errorf("Failed to unmarshal metadata: %v", err)
	}

	if orderNumber, ok := parsedMetadata["order_number"]; !ok || orderNumber != "ORD-123" {
		t.Error("Expected order_number in metadata")
	}

	if priority, ok := parsedMetadata["priority"]; !ok || priority != "high" {
		t.Error("Expected priority in metadata")
	}
}

// TestMessageTimestamps tests message timestamp fields
func TestMessageTimestamps(t *testing.T) {
	now := time.Now()
	msg := &Message{
		ID:        "msg-1",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if msg.CreatedAt != now {
		t.Error("Expected CreatedAt to match")
	}

	if msg.UpdatedAt != now {
		t.Error("Expected UpdatedAt to match")
	}

	// Simulate update
	later := now.Add(time.Second)
	msg.UpdatedAt = later

	if msg.CreatedAt != now {
		t.Error("CreatedAt should not change")
	}

	if msg.UpdatedAt != later {
		t.Error("UpdatedAt should be updated")
	}
}

// TestMessageJSONMarshal tests message JSON serialization
func TestMessageJSONMarshal(t *testing.T) {
	metadata := json.RawMessage(`{"key":"value"}`)
	now := time.Now()
	msg := &Message{
		ID:        "msg-1",
		Format:    "x12",
		Content:   "ISA*...",
		Metadata:  metadata,
		Sender:    "SENDER",
		Receiver:  "RECEIVER",
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	var unmarshaled Message
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if unmarshaled.ID != msg.ID {
		t.Error("ID mismatch after JSON round-trip")
	}

	if unmarshaled.Format != msg.Format {
		t.Error("Format mismatch after JSON round-trip")
	}

	if unmarshaled.Status != msg.Status {
		t.Error("Status mismatch after JSON round-trip")
	}
}

// TestTransactionCreation tests creating a transaction struct
func TestTransactionCreation(t *testing.T) {
	now := time.Now()
	details := json.RawMessage(`{"event_data":"value"}`)

	tx := &Transaction{
		ID:        "tx-1",
		MessageID: "msg-1",
		Event:     "message_created",
		Details:   details,
		Timestamp: now,
	}

	if tx.ID != "tx-1" {
		t.Errorf("Expected ID 'tx-1', got '%s'", tx.ID)
	}

	if tx.MessageID != "msg-1" {
		t.Errorf("Expected MessageID 'msg-1', got '%s'", tx.MessageID)
	}

	if tx.Event != "message_created" {
		t.Errorf("Expected Event 'message_created', got '%s'", tx.Event)
	}

	if tx.Timestamp != now {
		t.Error("Expected Timestamp to match")
	}
}

// TestTransactionEvents tests various transaction events
func TestTransactionEvents(t *testing.T) {
	events := []string{
		"message_created",
		"message_sent",
		"message_received",
		"validation_passed",
		"transformation_requested",
		"message_processed",
		"message_failed",
	}

	for _, event := range events {
		tx := &Transaction{
			ID:    "tx-1",
			Event: event,
		}

		if tx.Event != event {
			t.Errorf("Expected event '%s', got '%s'", event, tx.Event)
		}
	}
}

// TestTransactionDetails tests transaction details handling
func TestTransactionDetails(t *testing.T) {
	details := json.RawMessage(`{"source":"x12","target":"edifact"}`)

	tx := &Transaction{
		ID:      "tx-1",
		Details: details,
	}

	var parsedDetails map[string]interface{}
	err := json.Unmarshal(tx.Details, &parsedDetails)
	if err != nil {
		t.Errorf("Failed to unmarshal details: %v", err)
	}

	if source, ok := parsedDetails["source"]; !ok || source != "x12" {
		t.Error("Expected source in details")
	}

	if target, ok := parsedDetails["target"]; !ok || target != "edifact" {
		t.Error("Expected target in details")
	}
}

// TestTransactionJSONMarshal tests transaction JSON serialization
func TestTransactionJSONMarshal(t *testing.T) {
	details := json.RawMessage(`{"key":"value"}`)
	now := time.Now()
	tx := &Transaction{
		ID:        "tx-1",
		MessageID: "msg-1",
		Event:     "message_created",
		Details:   details,
		Timestamp: now,
	}

	data, err := json.Marshal(tx)
	if err != nil {
		t.Fatalf("Failed to marshal transaction: %v", err)
	}

	var unmarshaled Transaction
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal transaction: %v", err)
	}

	if unmarshaled.ID != tx.ID {
		t.Error("ID mismatch after JSON round-trip")
	}

	if unmarshaled.MessageID != tx.MessageID {
		t.Error("MessageID mismatch after JSON round-trip")
	}

	if unmarshaled.Event != tx.Event {
		t.Error("Event mismatch after JSON round-trip")
	}
}
