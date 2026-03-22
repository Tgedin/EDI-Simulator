package storage

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/theo-gedin/edi-simulator/internal/models"
)

// --- Tests ---

// TestMockMessageRepositoryStore tests storing a message
func TestMockMessageRepositoryStore(t *testing.T) {
	repo := NewMockMessageRepository()
	ctx := context.Background()

	msg := &models.Message{
		ID:        "msg-1",
		Format:    "x12",
		Content:   "ISA*...",
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := repo.Store(ctx, msg)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify it was stored
	retrieved, err := repo.GetByID(ctx, "msg-1")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if retrieved.ID != "msg-1" {
		t.Errorf("Expected ID 'msg-1', got '%s'", retrieved.ID)
	}
}

// TestMockMessageRepositoryStoreInvalidMessage tests storing an invalid message
func TestMockMessageRepositoryStoreInvalidMessage(t *testing.T) {
	repo := NewMockMessageRepository()
	ctx := context.Background()

	msg := &models.Message{
		ID:     "", // Invalid: empty ID
		Format: "x12",
	}

	err := repo.Store(ctx, msg)
	if err == nil {
		t.Error("Expected error for invalid message, got none")
	}
}

// TestMockMessageRepositoryGetByID tests retrieving a message by ID
func TestMockMessageRepositoryGetByID(t *testing.T) {
	repo := NewMockMessageRepository()
	ctx := context.Background()

	msg := &models.Message{
		ID:        "msg-1",
		Format:    "x12",
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	repo.Store(ctx, msg)

	retrieved, err := repo.GetByID(ctx, "msg-1")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if retrieved.Format != "x12" {
		t.Errorf("Expected format 'x12', got '%s'", retrieved.Format)
	}
}

// TestMockMessageRepositoryGetByIDNotFound tests GetByID with non-existent ID
func TestMockMessageRepositoryGetByIDNotFound(t *testing.T) {
	repo := NewMockMessageRepository()
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent message, got none")
	}
}

// TestMockMessageRepositoryListAll tests listing all messages
func TestMockMessageRepositoryListAll(t *testing.T) {
	repo := NewMockMessageRepository()
	ctx := context.Background()

	// Add multiple messages
	for i := 1; i <= 3; i++ {
		msg := &models.Message{
			ID:        models.Message{}.ID,
			Format:    "x12",
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		msg.ID = "msg-" + string(rune(i))
		repo.Store(ctx, msg)
	}

	messages, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}
}

// TestMockMessageRepositoryGetByStatus tests filtering by status
func TestMockMessageRepositoryGetByStatus(t *testing.T) {
	repo := NewMockMessageRepository()
	ctx := context.Background()

	// Add messages with different statuses
	statuses := []string{models.StatusPending, models.StatusSent, models.StatusPending}
	for i, status := range statuses {
		msg := &models.Message{
			ID:        "msg-" + string(rune(i+1)),
			Format:    "x12",
			Status:    status,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		repo.Store(ctx, msg)
	}

	// Filter by pending status
	pending, err := repo.GetByStatus(ctx, models.StatusPending)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(pending) != 2 {
		t.Errorf("Expected 2 pending messages, got %d", len(pending))
	}

	// Filter by sent status
	sent, err := repo.GetByStatus(ctx, models.StatusSent)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(sent) != 1 {
		t.Errorf("Expected 1 sent message, got %d", len(sent))
	}
}

// TestMockMessageRepositoryUpdateStatus tests updating message status
func TestMockMessageRepositoryUpdateStatus(t *testing.T) {
	repo := NewMockMessageRepository()
	ctx := context.Background()

	msg := &models.Message{
		ID:        "msg-1",
		Format:    "x12",
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	repo.Store(ctx, msg)

	// Update status
	err := repo.UpdateStatus(ctx, "msg-1", models.StatusSent)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify status was updated
	updated, _ := repo.GetByID(ctx, "msg-1")
	if updated.Status != models.StatusSent {
		t.Errorf("Expected status '%s', got '%s'", models.StatusSent, updated.Status)
	}
}

// TestMockMessageRepositoryUpdateStatusNotFound tests updating non-existent message
func TestMockMessageRepositoryUpdateStatusNotFound(t *testing.T) {
	repo := NewMockMessageRepository()
	ctx := context.Background()

	err := repo.UpdateStatus(ctx, "non-existent", models.StatusSent)
	if err == nil {
		t.Error("Expected error for non-existent message, got none")
	}
}

// TestMockTransactionRepositoryRecord tests recording a transaction
func TestMockTransactionRepositoryRecord(t *testing.T) {
	repo := NewMockTransactionRepository()
	ctx := context.Background()

	tx := &models.Transaction{
		ID:        "tx-1",
		MessageID: "msg-1",
		Event:     "message_created",
		Details:   json.RawMessage(`{"format":"x12"}`),
		Timestamp: time.Now(),
	}

	err := repo.Record(ctx, tx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// TestMockTransactionRepositoryRecordInvalid tests recording invalid transaction
func TestMockTransactionRepositoryRecordInvalid(t *testing.T) {
	repo := NewMockTransactionRepository()
	ctx := context.Background()

	tx := &models.Transaction{
		ID:        "", // Invalid: empty ID
		MessageID: "msg-1",
		Event:     "message_created",
	}

	err := repo.Record(ctx, tx)
	if err == nil {
		t.Error("Expected error for invalid transaction, got none")
	}
}

// TestMockTransactionRepositoryGetByMessageID tests retrieving transactions by message ID
func TestMockTransactionRepositoryGetByMessageID(t *testing.T) {
	repo := NewMockTransactionRepository()
	ctx := context.Background()

	messageID := "msg-1"

	// Record multiple transactions
	for i := 1; i <= 3; i++ {
		tx := &models.Transaction{
			ID:        "tx-" + string(rune(i)),
			MessageID: messageID,
			Event:     "test_event",
			Details:   json.RawMessage(`{}`),
			Timestamp: time.Now(),
		}
		repo.Record(ctx, tx)
	}

	// Retrieve transactions
	txs, err := repo.GetByMessageID(ctx, messageID)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(txs) != 3 {
		t.Errorf("Expected 3 transactions, got %d", len(txs))
	}
}

// TestMockTransactionRepositoryGetByMessageIDNotFound tests with non-existent message
func TestMockTransactionRepositoryGetByMessageIDNotFound(t *testing.T) {
	repo := NewMockTransactionRepository()
	ctx := context.Background()

	txs, err := repo.GetByMessageID(ctx, "non-existent")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(txs) != 0 {
		t.Errorf("Expected 0 transactions, got %d", len(txs))
	}
}

// TestRepositoryIntegration tests message and transaction repositories together
func TestRepositoryIntegration(t *testing.T) {
	msgRepo := NewMockMessageRepository()
	txRepo := NewMockTransactionRepository()
	ctx := context.Background()

	// Create a message
	msg := &models.Message{
		ID:        "msg-1",
		Format:    "x12",
		Content:   "ISA*...",
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	msgRepo.Store(ctx, msg)

	// Record creation transaction
	tx := &models.Transaction{
		ID:        "tx-1",
		MessageID: "msg-1",
		Event:     "message_created",
		Details:   json.RawMessage(`{"format":"x12"}`),
		Timestamp: time.Now(),
	}

	txRepo.Record(ctx, tx)

	// Update message status
	msgRepo.UpdateStatus(ctx, "msg-1", models.StatusSent)

	// Record sent transaction
	tx2 := &models.Transaction{
		ID:        "tx-2",
		MessageID: "msg-1",
		Event:     "message_sent",
		Details:   json.RawMessage(`{"status":"sent"}`),
		Timestamp: time.Now(),
	}

	txRepo.Record(ctx, tx2)

	// Verify final state
	finalMsg, _ := msgRepo.GetByID(ctx, "msg-1")
	if finalMsg.Status != models.StatusSent {
		t.Errorf("Expected final status '%s', got '%s'", models.StatusSent, finalMsg.Status)
	}

	txs, _ := txRepo.GetByMessageID(ctx, "msg-1")
	if len(txs) != 2 {
		t.Errorf("Expected 2 transaction events, got %d", len(txs))
	}
}

func TestMockMessageRepository_UpdateMetadata(t *testing.T) {
	msgRepo := NewMockMessageRepository()
	ctx := context.Background()

	msg := &models.Message{
		ID:        "meta-msg-1",
		Format:    "x12",
		Content:   "ISA*...",
		Status:   models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = msgRepo.Store(ctx, msg)

	err := msgRepo.UpdateMetadata(ctx, "meta-msg-1", "<Document/>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Metadata must be non-empty after update
	updated, _ := msgRepo.GetByID(ctx, "meta-msg-1")
	if len(updated.Metadata) == 0 {
		t.Error("Metadata should be set after UpdateMetadata")
	}
}

func TestMockMessageRepository_UpdateMetadata_NotFound(t *testing.T) {
	msgRepo := NewMockMessageRepository()
	err := msgRepo.UpdateMetadata(context.Background(), "nonexistent", "<Document/>")
	if err != ErrMessageNotFound {
		t.Errorf("expected ErrMessageNotFound, got %v", err)
	}
}
