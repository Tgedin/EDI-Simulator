package storage

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	_ "github.com/lib/pq"

	"github.com/theo-gedin/edi-simulator/internal/models"
)

// PostgresMessageRepository implements MessageRepository for PostgreSQL
type PostgresMessageRepository struct {
	db *sql.DB
}

// PostgresTransactionRepository implements TransactionRepository for PostgreSQL
type PostgresTransactionRepository struct {
	db *sql.DB
}

// NewPostgresMessageRepository creates a new PostgreSQL message repository
func NewPostgresMessageRepository(db *sql.DB) *PostgresMessageRepository {
	return &PostgresMessageRepository{db: db}
}

// NewPostgresTransactionRepository creates a new PostgreSQL transaction repository
func NewPostgresTransactionRepository(db *sql.DB) *PostgresTransactionRepository {
	return &PostgresTransactionRepository{db: db}
}

// ConnectPostgres establishes a connection to PostgreSQL
func ConnectPostgres(host, port, user, password, dbname, sslmode string) (*sql.DB, error) {
	psqlInfo := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	log.Println("Connected to PostgreSQL")
	return db, nil
}

// Store saves a new message
func (r *PostgresMessageRepository) Store(ctx context.Context, msg *models.Message) error {
	query := `
		INSERT INTO messages (id, format, content, metadata, sender, receiver, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecContext(ctx, query,
		msg.ID, msg.Format, msg.Content, msg.Metadata,
		msg.Sender, msg.Receiver, msg.Status,
		msg.CreatedAt, msg.UpdatedAt,
	)

	return err
}

// GetByID retrieves a message by ID
func (r *PostgresMessageRepository) GetByID(ctx context.Context, id string) (*models.Message, error) {
	query := `
		SELECT id, format, content, metadata, sender, receiver, status, created_at, updated_at
		FROM messages
		WHERE id = $1
	`

	msg := &models.Message{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&msg.ID, &msg.Format, &msg.Content, &msg.Metadata,
		&msg.Sender, &msg.Receiver, &msg.Status,
		&msg.CreatedAt, &msg.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("message not found")
	}

	return msg, err
}

// ListAll retrieves all messages
func (r *PostgresMessageRepository) ListAll(ctx context.Context) ([]models.Message, error) {
	query := `
		SELECT id, format, content, metadata, sender, receiver, status, created_at, updated_at
		FROM messages
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		msg := models.Message{}
		err := rows.Scan(
			&msg.ID, &msg.Format, &msg.Content, &msg.Metadata,
			&msg.Sender, &msg.Receiver, &msg.Status,
			&msg.CreatedAt, &msg.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// GetByStatus retrieves messages filtered by status
func (r *PostgresMessageRepository) GetByStatus(ctx context.Context, status string) ([]models.Message, error) {
	query := `
		SELECT id, format, content, metadata, sender, receiver, status, created_at, updated_at
		FROM messages
		WHERE status = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		msg := models.Message{}
		err := rows.Scan(
			&msg.ID, &msg.Format, &msg.Content, &msg.Metadata,
			&msg.Sender, &msg.Receiver, &msg.Status,
			&msg.CreatedAt, &msg.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// ErrMessageNotFound is defined in errors.go and reused by Postgres implementation

// UpdateStatus updates message status
func (r *PostgresMessageRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	query := `
		UPDATE messages
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`

	res, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return err
	}
	if ra, _ := res.RowsAffected(); ra == 0 {
		return ErrMessageNotFound
	}
	return nil
}

// UpdateMetadata stores a canonical XML snapshot into Message.Metadata JSONB.
func (r *PostgresMessageRepository) UpdateMetadata(ctx context.Context, id string, xmlSnapshot string) error {
	query := `
		UPDATE messages
		SET metadata = $1::jsonb, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`
	// Encode as a JSON string without HTML-escaping so < > & are stored verbatim.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(xmlSnapshot); err != nil {
		return fmt.Errorf("failed to marshal XML snapshot: %w", err)
	}
	// json.Encoder.Encode appends a trailing newline — trim it.
	jsonVal := strings.TrimRight(buf.String(), "\n")
	res, err := r.db.ExecContext(ctx, query, jsonVal, id)
	if err != nil {
		return err
	}
	if ra, _ := res.RowsAffected(); ra == 0 {
		return ErrMessageNotFound
	}
	return nil
}

// Close closes the database connection
func (r *PostgresMessageRepository) Close() error {
	return r.db.Close()
}

// Record stores a new transaction event
func (r *PostgresTransactionRepository) Record(ctx context.Context, tx *models.Transaction) error {
	query := `
		INSERT INTO transactions (id, message_id, event, details, timestamp)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.db.ExecContext(ctx, query,
		tx.ID, tx.MessageID, tx.Event, tx.Details, tx.Timestamp,
	)

	return err
}

// GetByMessageID retrieves all transactions for a message
func (r *PostgresTransactionRepository) GetByMessageID(ctx context.Context, messageID string) ([]models.Transaction, error) {
	query := `
		SELECT id, message_id, event, details, timestamp
		FROM transactions
		WHERE message_id = $1
		ORDER BY timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []models.Transaction
	for rows.Next() {
		tx := models.Transaction{}
		err := rows.Scan(&tx.ID, &tx.MessageID, &tx.Event, &tx.Details, &tx.Timestamp)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}

	return transactions, rows.Err()
}

// Close closes the database connection
func (r *PostgresTransactionRepository) Close() error {
	return r.db.Close()
}
