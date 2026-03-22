package storage

import (
	"context"
	"encoding/json"
	"time"

	"github.com/theo-gedin/edi-simulator/internal/models"
)

// MockMessageRepository is an in-memory implementation of MessageRepository for testing
type MockMessageRepository struct {
	Messages map[string]*models.Message
}

// NewMockMessageRepository creates a new mock message repository
func NewMockMessageRepository() *MockMessageRepository {
	return &MockMessageRepository{
		Messages: make(map[string]*models.Message),
	}
}

// Store saves a new message
func (m *MockMessageRepository) Store(ctx context.Context, msg *models.Message) error {
	if msg.ID == "" {
		return ErrInvalidMessage
	}
	m.Messages[msg.ID] = msg
	return nil
}

// GetByID retrieves a message by ID
func (m *MockMessageRepository) GetByID(ctx context.Context, id string) (*models.Message, error) {
	if msg, ok := m.Messages[id]; ok {
		return msg, nil
	}
	return nil, ErrMessageNotFound
}

// ListAll retrieves all messages
func (m *MockMessageRepository) ListAll(ctx context.Context) ([]models.Message, error) {
	messages := make([]models.Message, 0, len(m.Messages))
	for _, msg := range m.Messages {
		messages = append(messages, *msg)
	}
	return messages, nil
}

// GetByStatus retrieves messages filtered by status
func (m *MockMessageRepository) GetByStatus(ctx context.Context, status string) ([]models.Message, error) {
	var filtered []models.Message
	for _, msg := range m.Messages {
		if msg.Status == status {
			filtered = append(filtered, *msg)
		}
	}
	return filtered, nil
}

// UpdateStatus updates message status
func (m *MockMessageRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	msg, ok := m.Messages[id]
	if !ok {
		return ErrMessageNotFound
	}
	msg.Status = status
	msg.UpdatedAt = time.Now()
	return nil
}

// UpdateMetadata stores the canonical XML snapshot in Message.Metadata.
func (m *MockMessageRepository) UpdateMetadata(_ context.Context, id string, xmlSnapshot string) error {
	msg, ok := m.Messages[id]
	if !ok {
		return ErrMessageNotFound
	}
	msg.Metadata = json.RawMessage(`"` + xmlSnapshot + `"`)
	msg.UpdatedAt = time.Now()
	return nil
}

// Close closes the repository (no-op for mock)
func (m *MockMessageRepository) Close() error {
	return nil
}

// MockTransactionRepository is an in-memory implementation of TransactionRepository for testing
type MockTransactionRepository struct {
	Transactions map[string][]*models.Transaction
}

// NewMockTransactionRepository creates a new mock transaction repository
func NewMockTransactionRepository() *MockTransactionRepository {
	return &MockTransactionRepository{
		Transactions: make(map[string][]*models.Transaction),
	}
}

// Record stores a new transaction event
func (m *MockTransactionRepository) Record(ctx context.Context, tx *models.Transaction) error {
	if tx.ID == "" || tx.MessageID == "" {
		return ErrInvalidTransaction
	}
	m.Transactions[tx.MessageID] = append(m.Transactions[tx.MessageID], tx)
	return nil
}

// GetByMessageID retrieves all transactions for a message
func (m *MockTransactionRepository) GetByMessageID(ctx context.Context, messageID string) ([]models.Transaction, error) {
	txs := m.Transactions[messageID]
	if txs == nil {
		return []models.Transaction{}, nil
	}
	result := make([]models.Transaction, len(txs))
	for i, tx := range txs {
		result[i] = *tx
	}
	return result, nil
}

// Close closes the repository (no-op for mock)
func (m *MockTransactionRepository) Close() error {
	return nil
}
