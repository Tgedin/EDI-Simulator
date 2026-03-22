package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/theo-gedin/edi-simulator/internal/config"
	"github.com/theo-gedin/edi-simulator/internal/logger"
	"github.com/theo-gedin/edi-simulator/internal/messaging"
	"github.com/theo-gedin/edi-simulator/internal/metrics"
	"github.com/theo-gedin/edi-simulator/internal/models"
	"github.com/theo-gedin/edi-simulator/internal/storage"
	"github.com/theo-gedin/edi-simulator/internal/tracing"
	"github.com/theo-gedin/edi-simulator/internal/validation"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	cfg := config.Load()
	log := logger.New("sender", cfg.LogLevel)

	shutdownTracing := tracing.InitProvider("sender", cfg.OTELEndpoint)
	defer shutdownTracing()

	// Expose Prometheus metrics on a dedicated port.
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(cfg.MetricsPort, mux); err != nil {
			log.Error("metrics server failed", "error", err)
		}
	}()

	// Database
	db, err := storage.ConnectPostgres(
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPass, cfg.DBName, cfg.DBSSLMode,
	)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	msgRepo := storage.NewPostgresMessageRepository(db)
	txnRepo := storage.NewPostgresTransactionRepository(db)

	// RabbitMQ
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

	dlqName := "messages." + cfg.RabbitMQSendQueue + ".dlq"
	dlqRouting := cfg.RabbitMQSendQueue + ".dlq"
	if err := rabbitmq.DeclareDeadLetterQueue(dlqName, cfg.RabbitMQExchange, dlqRouting, cfg.DLQTTLMs); err != nil {
		log.Error("failed to declare send DLQ", "error", err)
		os.Exit(1)
	}
	if err := rabbitmq.BindQueue(dlqName, cfg.RabbitMQExchange, dlqRouting); err != nil {
		log.Error("failed to bind send DLQ", "error", err)
		os.Exit(1)
	}

	if err := rabbitmq.DeclareQueue("messages." + cfg.RabbitMQReceiveQueue); err != nil {
		log.Error("failed to declare receive queue", "error", err)
		os.Exit(1)
	}
	if err := rabbitmq.BindQueue("messages."+cfg.RabbitMQReceiveQueue, cfg.RabbitMQExchange, cfg.RabbitMQReceiveQueue); err != nil {
		log.Error("failed to bind receive queue", "error", err)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		ctx := context.Background()
		msgs, err := rabbitmq.Consume(ctx, "messages."+cfg.RabbitMQSendQueue)
		if err != nil {
			log.Error("failed to consume messages", "error", err)
			os.Exit(1)
		}

		tracer := otel.Tracer("sender")

		for msg := range msgs {
			start := time.Now()

			var message models.Message
			if err := json.Unmarshal(msg.Body, &message); err != nil {
				log.Warn("failed to parse message body", "error", err)
				msg.Nack(false, false)
				continue
			}

			// Continue the trace from AMQP headers, then start a child span.
			msgCtx := messaging.ExtractTraceContext(ctx, msg.Headers)
			msgCtx, span := tracer.Start(msgCtx, "sender.process",
				trace.WithAttributes(
					attribute.String("message.id", message.ID),
					attribute.String("message.format", message.Format),
				),
			)
			_ = msgCtx // propagated downstream via PublishWithRetry

			retryCount := 0
			if h, ok := msg.Headers["x-retry-count"]; ok {
				if c, ok := h.(int32); ok {
					retryCount = int(c)
				}
			}

			log.Info("processing message", "id", message.ID, "format", message.Format, "retry", retryCount)

			if err := validation.Validate(message.Format, message.Content); err != nil {
				log.Warn("validation failed", "id", message.ID, "error", err)

				txDetails := json.RawMessage(`{"error":"` + err.Error() + `"}`)
				txnRepo.Record(ctx, &models.Transaction{
					ID:        message.ID + "-validation-failed",
					MessageID: message.ID,
					Event:     "validation_failed",
					Details:   txDetails,
					Timestamp: time.Now(),
				})

				msgRepo.UpdateStatus(ctx, message.ID, models.StatusFailed)
				metrics.MessagesProcessedTotal.WithLabelValues("sender", message.Format, "failed").Inc()
				msg.Ack(false)
				continue
			}

			if err := msgRepo.UpdateStatus(ctx, message.ID, models.StatusSent); err != nil {
				if err == storage.ErrMessageNotFound {
					log.Warn("message not found in DB, dropping", "id", message.ID)
					msg.Ack(false)
					continue
				}
				log.Warn("failed to update status", "id", message.ID, "error", err)

				if retryCount < cfg.MaxRetries {
					backoff := cfg.InitialBackoff * time.Duration(1<<uint(retryCount))
					if backoff > cfg.MaxBackoff {
						backoff = cfg.MaxBackoff
					}
					log.Info("retrying", "id", message.ID, "attempt", retryCount+1, "backoff_ms", backoff.Milliseconds())
					time.Sleep(backoff)
					msg.Nack(false, true)
				} else {
					log.Warn("max retries exceeded, sending to DLQ", "id", message.ID)
					msgBytes, _ := json.Marshal(message)
					rabbitmq.SendToDLQ(ctx, dlqName, cfg.RabbitMQExchange, dlqRouting, msgBytes)
					metrics.DLQMessagesTotal.WithLabelValues("sender", dlqName).Inc()
					msg.Ack(false)
				}
				continue
			}

			txnRepo.Record(ctx, &models.Transaction{
				ID:        message.ID + "-sent",
				MessageID: message.ID,
				Event:     "message_sent",
				Details:   json.RawMessage(`{"format":"` + message.Format + `","retry":` + strconv.Itoa(retryCount) + `}`),
				Timestamp: time.Now(),
			})

			msgBytes, _ := json.Marshal(message)
			if err := rabbitmq.PublishWithRetry(msgCtx, cfg.RabbitMQExchange, cfg.RabbitMQReceiveQueue, msgBytes, 0); err != nil {
				log.Warn("failed to publish to receiver queue", "id", message.ID, "error", err)

				if retryCount < cfg.MaxRetries {
					backoff := cfg.InitialBackoff * time.Duration(1<<uint(retryCount))
					if backoff > cfg.MaxBackoff {
						backoff = cfg.MaxBackoff
					}
					time.Sleep(backoff)
					msg.Nack(false, true)
				} else {
					rabbitmq.SendToDLQ(ctx, dlqName, cfg.RabbitMQExchange, dlqRouting, msgBytes)
					metrics.DLQMessagesTotal.WithLabelValues("sender", dlqName).Inc()
					msg.Ack(false)
				}
				continue
			}

			elapsed := time.Since(start)
			metrics.MessagesProcessedTotal.WithLabelValues("sender", message.Format, "sent").Inc()
			metrics.MessageProcessingDuration.WithLabelValues("sender").Observe(elapsed.Seconds())
			log.Info("message sent to receiver", "id", message.ID, "duration_ms", elapsed.Milliseconds())
			span.End()
			msg.Ack(false)
		}
	}()

	<-sigChan
	log.Info("shutting down sender")
}

