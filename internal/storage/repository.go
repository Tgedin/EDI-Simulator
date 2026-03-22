package storage

import (
	"context"

	"github.com/theo-gedin/edi-simulator/internal/models"
)

// MessageRepository defines the interface for message storage operations
type MessageRepository interface {
	// Store saves a new message
	Store(ctx context.Context, msg *models.Message) error

	// GetByID retrieves a message by ID
	GetByID(ctx context.Context, id string) (*models.Message, error)

	// ListAll retrieves all messages
	ListAll(ctx context.Context) ([]models.Message, error)

	// GetByStatus retrieves messages filtered by status
	GetByStatus(ctx context.Context, status string) ([]models.Message, error)

	// UpdateStatus updates message status
	UpdateStatus(ctx context.Context, id string, status string) error

	// UpdateMetadata stores a canonical XML snapshot in Message.Metadata JSONB.
	// The xmlSnapshot is the string produced by xml.Marshal(CanonicalDocument).
	UpdateMetadata(ctx context.Context, id string, xmlSnapshot string) error

	// Close closes database connection
	Close() error
}

// TransactionRepository defines the interface for audit log operations
type TransactionRepository interface {
	// Record stores a new transaction event
	Record(ctx context.Context, tx *models.Transaction) error

	// GetByMessageID retrieves all transactions for a message
	GetByMessageID(ctx context.Context, messageID string) ([]models.Transaction, error)

	// Close closes database connection
	Close() error
}
