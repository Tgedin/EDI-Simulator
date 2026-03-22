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

	"github.com/google/uuid"
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
	log := logger.New("receiver", cfg.LogLevel)

	shutdownTracing := tracing.InitProvider("receiver", cfg.OTELEndpoint)
	defer shutdownTracing()

	// Prometheus metrics server
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(cfg.MetricsPort, mux); err != nil {
			log.Error("metrics server failed", "error", err)
		}
	}()

	db, err := storage.ConnectPostgres(
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPass, cfg.DBName, cfg.DBSSLMode,
	)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	msgRepo := storage.NewPostgresMessageRepository(db)
	txRepo := storage.NewPostgresTransactionRepository(db)
	mappingRepo := storage.NewPostgresMappingRepository(db)

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

	if err := rabbitmq.DeclareQueue("messages." + cfg.RabbitMQReceiveQueue); err != nil {
		log.Error("failed to declare receive queue", "error", err)
		os.Exit(1)
	}
	if err := rabbitmq.BindQueue("messages."+cfg.RabbitMQReceiveQueue, cfg.RabbitMQExchange, cfg.RabbitMQReceiveQueue); err != nil {
		log.Error("failed to bind receive queue", "error", err)
		os.Exit(1)
	}

	dlqName := "messages." + cfg.RabbitMQReceiveQueue + ".dlq"
	dlqRouting := cfg.RabbitMQReceiveQueue + ".dlq"
	if err := rabbitmq.DeclareDeadLetterQueue(dlqName, cfg.RabbitMQExchange, dlqRouting, cfg.DLQTTLMs); err != nil {
		log.Error("failed to declare receive DLQ", "error", err)
		os.Exit(1)
	}
	if err := rabbitmq.BindQueue(dlqName, cfg.RabbitMQExchange, dlqRouting); err != nil {
		log.Error("failed to bind receive DLQ", "error", err)
		os.Exit(1)
	}

	transformQueueName := "messages." + cfg.RabbitMQTransformQueue
	if err := rabbitmq.DeclareQueue(transformQueueName); err != nil {
		log.Error("failed to declare transform queue", "error", err)
		os.Exit(1)
	}
	if err := rabbitmq.BindQueue(transformQueueName, cfg.RabbitMQExchange, cfg.RabbitMQTransformQueue); err != nil {
		log.Error("failed to bind transform queue", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	messages, err := rabbitmq.Consume(ctx, "messages."+cfg.RabbitMQReceiveQueue)
	if err != nil {
		log.Error("failed to consume from queue", "error", err)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info("receiver waiting for messages")

	tracer := otel.Tracer("receiver")

	go func() {
		for delivery := range messages {
			start := time.Now()

			msg := &models.Message{}
			if err := json.Unmarshal(delivery.Body, msg); err != nil {
				log.Warn("failed to unmarshal message", "error", err)
				delivery.Nack(false, false)
				continue
			}

			// Continue the distributed trace.
			msgCtx := messaging.ExtractTraceContext(ctx, delivery.Headers)
			msgCtx, span := tracer.Start(msgCtx, "receiver.process",
				trace.WithAttributes(
					attribute.String("message.id", msg.ID),
					attribute.String("message.format", msg.Format),
				),
			)

			retryCount := 0
			if h, ok := delivery.Headers["x-retry-count"]; ok {
				if c, ok := h.(int32); ok {
					retryCount = int(c)
				}
			}

			log.Info("processing message", "id", msg.ID, "format", msg.Format, "retry", retryCount)

			if err := validation.Validate(msg.Format, msg.Content); err != nil {
				log.Warn("validation failed", "id", msg.ID, "error", err)
				txDetails := json.RawMessage(`{"error":"` + err.Error() + `"}`)
				txRepo.Record(ctx, &models.Transaction{
					ID:        uuid.New().String(),
					MessageID: msg.ID,
					Event:     "validation_failed",
					Details:   txDetails,
					Timestamp: time.Now(),
				})
				metrics.MessagesProcessedTotal.WithLabelValues("receiver", msg.Format, "validation_failed").Inc()
				span.End()
				delivery.Ack(false)
				continue
			}

			if err := msgRepo.UpdateStatus(ctx, msg.ID, models.StatusReceived); err != nil {
				if err == storage.ErrMessageNotFound {
					log.Warn("message not found in DB, dropping", "id", msg.ID)
					span.End()
					delivery.Ack(false)
					continue
				}
				log.Warn("failed to update status", "id", msg.ID, "error", err)
				if retryCount < cfg.MaxRetries {
					backoff := cfg.InitialBackoff * time.Duration(1<<uint(retryCount))
					if backoff > cfg.MaxBackoff {
						backoff = cfg.MaxBackoff
					}
					log.Info("retrying", "id", msg.ID, "attempt", retryCount+1)
					time.Sleep(backoff)
					span.End()
					delivery.Nack(false, true)
				} else {
					log.Warn("max retries exceeded, sending to DLQ", "id", msg.ID)
					msgBytes, _ := json.Marshal(msg)
					rabbitmq.SendToDLQ(ctx, dlqName, cfg.RabbitMQExchange, dlqRouting, msgBytes)
					metrics.DLQMessagesTotal.WithLabelValues("receiver", dlqName).Inc()
					span.End()
					delivery.Ack(false)
				}
				continue
			}

			if err := txRepo.Record(ctx, &models.Transaction{
				ID:        uuid.New().String(),
				MessageID: msg.ID,
				Event:     "message_received",
				Details:   json.RawMessage(`{"status":"received","retry":` + strconv.Itoa(retryCount) + `}`),
				Timestamp: time.Now(),
			}); err != nil {
				log.Warn("failed to record transaction", "id", msg.ID, "error", err)
			}

			elapsed := time.Since(start)
			metrics.MessagesProcessedTotal.WithLabelValues("receiver", msg.Format, "received").Inc()
			metrics.MessageProcessingDuration.WithLabelValues("receiver").Observe(elapsed.Seconds())
			log.Info("message received and stored", "id", msg.ID, "format", msg.Format, "duration_ms", elapsed.Milliseconds())

			delivery.Ack(false)

			// Auto-trigger transform if a mapping exists for this format.
			allMappings, err := mappingRepo.ListActive(ctx)
			if err != nil {
				log.Warn("failed to list mappings", "error", err)
				span.End()
				continue
			}
			var chosen *storage.TransformationMapping
			for i, m := range allMappings {
				if m.SourceFormat == msg.Format {
					chosen = &allMappings[i]
					break
				}
			}
			if chosen != nil {
				transformReq, _ := json.Marshal(map[string]string{
					"message_id":    msg.ID,
					"source_format": chosen.SourceFormat,
					"target_format": chosen.TargetFormat,
				})
				pubCtx, cancel := context.WithTimeout(msgCtx, 5*time.Second)
				if pubErr := rabbitmq.Publish(pubCtx, cfg.RabbitMQExchange, cfg.RabbitMQTransformQueue, transformReq); pubErr != nil {
					log.Warn("failed to publish transform request", "id", msg.ID, "error", pubErr)
				} else {
					log.Info("transform queued", "id", msg.ID, "source", chosen.SourceFormat, "target", chosen.TargetFormat)
				}
				cancel()
			} else {
				log.Info("no active mapping found, skipping auto-transform", "id", msg.ID, "format", msg.Format)
			}
			span.End()
		}
	}()

	<-sigChan
	log.Info("shutting down receiver")
}
