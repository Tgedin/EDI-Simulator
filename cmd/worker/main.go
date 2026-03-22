package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/theo-gedin/edi-simulator/internal/config"
	"github.com/theo-gedin/edi-simulator/internal/logger"
	"github.com/theo-gedin/edi-simulator/internal/messaging"
	"github.com/theo-gedin/edi-simulator/internal/models"
	"github.com/theo-gedin/edi-simulator/internal/storage"
)

func main() {
	cfg := config.Load()
	log := logger.New("worker", cfg.LogLevel)

	db, err := storage.ConnectPostgres(
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPass, cfg.DBName, cfg.DBSSLMode,
	)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	msgRepo := storage.NewPostgresMessageRepository(db)

	rabbitmq, err := messaging.NewRabbitMQClient(cfg.RabbitMQURL)
	if err != nil {
		log.Error("failed to connect to RabbitMQ", "error", err)
		os.Exit(1)
	}
	defer rabbitmq.Close()

	if err := rabbitmq.DeclareExchange(cfg.RabbitMQExchange); err != nil {
		log.Error("failed to declare exchange", "error", err)
		os.Exit(1)
	}

	if err := rabbitmq.DeclareQueue("messages." + cfg.RabbitMQSendQueue); err != nil {
		log.Error("failed to declare send queue", "error", err)
		os.Exit(1)
	}

	if err := rabbitmq.BindQueue("messages."+cfg.RabbitMQSendQueue, cfg.RabbitMQExchange, cfg.RabbitMQSendQueue); err != nil {
		log.Error("failed to bind send queue", "error", err)
		os.Exit(1)
	}

	log.Info("worker started, polling for pending messages")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			log.Info("worker shutting down")
			return

		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			processMessages(ctx, db, msgRepo, rabbitmq, cfg, log)
			cancel()
		}
	}
}

// processMessages polls for pending unpublished messages and publishes them to the queue
func processMessages(ctx context.Context, db *sql.DB, msgRepo storage.MessageRepository, rabbitmq *messaging.RabbitMQClient, cfg *config.Config, log *slog.Logger) {
	// Query for unpublished pending messages
	rows, err := db.QueryContext(ctx, `
		SELECT id, format, content, metadata, sender, receiver, status, created_at, updated_at
		FROM messages
		WHERE status = 'pending' AND published_at IS NULL
		ORDER BY created_at ASC
		LIMIT 100
	`)
	if err != nil {
		log.Error("error querying pending messages", "error", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var msg models.Message
		var metadata *string

		if err := rows.Scan(
			&msg.ID, &msg.Format, &msg.Content, &metadata,
			&msg.Sender, &msg.Receiver, &msg.Status,
			&msg.CreatedAt, &msg.UpdatedAt,
		); err != nil {
			log.Error("error scanning message", "error", err)
			continue
		}

		if metadata != nil {
			msg.Metadata = json.RawMessage(*metadata)
		}

		msgBytes, err := json.Marshal(msg)
		if err != nil {
			log.Error("error marshaling message", "id", msg.ID, "error", err)
			continue
		}

		if err := rabbitmq.PublishWithRetry(ctx, cfg.RabbitMQExchange, cfg.RabbitMQSendQueue, msgBytes, 0); err != nil {
			log.Error("error publishing message", "id", msg.ID, "error", err)
			continue
		}

		now := time.Now()
		if err := markMessagePublished(ctx, db, msg.ID, now); err != nil {
			log.Error("error marking message published", "id", msg.ID, "error", err)
			continue
		}

		count++
		log.Info("published message to queue", "id", msg.ID, "format", msg.Format)
	}

	if count > 0 {
		log.Info("worker batch complete", "published", count)
	}
}

// markMessagePublished updates the published_at timestamp for a message
func markMessagePublished(ctx context.Context, db *sql.DB, messageID string, timestamp time.Time) error {
	_, err := db.ExecContext(ctx, `
		UPDATE messages
		SET published_at = $1
		WHERE id = $2
	`, timestamp, messageID)
	return err
}
