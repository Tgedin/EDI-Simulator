package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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
	"github.com/theo-gedin/edi-simulator/internal/transformation"
	"github.com/theo-gedin/edi-simulator/internal/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// transformRequest is the schema for messages published to the transform queue.
// Producers (e.g. the API gateway or a future orchestrator) publish this JSON.
type transformRequest struct {
	MessageID    string `json:"message_id"`
	SourceFormat string `json:"source_format"`
	TargetFormat string `json:"target_format"`
}

func main() {
	cfg := config.Load()
	log := logger.New("transformer", cfg.LogLevel)

	shutdownTracing := tracing.InitProvider("transformer", cfg.OTELEndpoint)
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

	// Transformation engine – registers X12, EDIFACT, XML codecs automatically.
	engine := transformation.NewTransformationEngine()

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

	transformRoutingKey := cfg.RabbitMQTransformQueue
	transformQueue := "messages." + transformRoutingKey
	if err := rabbitmq.DeclareQueue(transformQueue); err != nil {
		log.Error("failed to declare transform queue", "error", err)
		os.Exit(1)
	}
	if err := rabbitmq.BindQueue(transformQueue, cfg.RabbitMQExchange, transformRoutingKey); err != nil {
		log.Error("failed to bind transform queue", "error", err)
		os.Exit(1)
	}

	sendRoutingKey := cfg.RabbitMQSendQueue

	ctx := context.Background()

	deliveries, err := rabbitmq.Consume(ctx, transformQueue)
	if err != nil {
		log.Error("failed to start consuming", "error", err)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info("transformer started", "queue", transformQueue)

	tracer := otel.Tracer("transformer")

	go func() {
		for delivery := range deliveries {
			start := time.Now()

			var req transformRequest
			if err := json.Unmarshal(delivery.Body, &req); err != nil {
				log.Warn("failed to unmarshal delivery", "error", err)
				delivery.Nack(false, false)
				continue
			}

			// Continue distributed trace from AMQP headers.
			msgCtx := messaging.ExtractTraceContext(ctx, delivery.Headers)
			msgCtx, span := tracer.Start(msgCtx, "transformer.process",
				trace.WithAttributes(
					attribute.String("message.id", req.MessageID),
					attribute.String("source_format", req.SourceFormat),
					attribute.String("target_format", req.TargetFormat),
				),
			)

			log.Info("processing transform", "id", req.MessageID, "source", req.SourceFormat, "target", req.TargetFormat)

			msg, err := msgRepo.GetByID(ctx, req.MessageID)
			if err != nil {
				log.Warn("message not found", "id", req.MessageID, "error", err)
				span.End()
				delivery.Nack(false, false)
				continue
			}

			result, err := engine.Transform(req.SourceFormat, req.TargetFormat, msg.Content)
			if err != nil {
				log.Warn("transform failed", "id", req.MessageID, "error", err)
				metrics.TransformationsTotal.WithLabelValues(req.SourceFormat, req.TargetFormat, "error").Inc()
				recordFailure(ctx, txRepo, req.MessageID, err.Error(), log)
				span.End()
				delivery.Nack(false, false)
				continue
			}

			if canonXML, xmlErr := transformation.CanonicalXML(result.Canonical); xmlErr == nil {
				if updateErr := msgRepo.UpdateMetadata(ctx, req.MessageID, canonXML); updateErr != nil {
					log.Warn("failed to store canonical", "id", req.MessageID, "error", updateErr)
				}
			}

			if err := msgRepo.UpdateStatus(ctx, req.MessageID, models.StatusProcessed); err != nil {
				log.Warn("failed to update status", "id", req.MessageID, "error", err)
			}

			if err := txRepo.Record(ctx, &models.Transaction{
				ID:        uuid.New().String(),
				MessageID: req.MessageID,
				Event:     "transformation_complete",
				Details:   json.RawMessage(`{"source":"` + req.SourceFormat + `","target":"` + req.TargetFormat + `"}`),
				Timestamp: time.Now(),
			}); err != nil {
				log.Warn("failed to record transaction", "id", req.MessageID, "error", err)
			}

			// Publish transformed output back to sender queue.
			outMsg := &models.Message{
				ID:        uuid.New().String(),
				Format:    req.TargetFormat,
				Content:   result.Output,
				Sender:    msg.Sender,
				Receiver:  msg.Receiver,
				Status:    models.StatusPending,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			outBytes, err := json.Marshal(outMsg)
			if err != nil {
				log.Warn("failed to marshal output message", "id", req.MessageID, "error", err)
				span.End()
				delivery.Ack(false)
				continue
			}
			if err := rabbitmq.PublishWithRetry(msgCtx, cfg.RabbitMQExchange, sendRoutingKey, outBytes, 0); err != nil {
				log.Warn("failed to publish transformed message", "id", req.MessageID, "error", err)
			}

			elapsed := time.Since(start)
			metrics.TransformationsTotal.WithLabelValues(req.SourceFormat, req.TargetFormat, "success").Inc()
			metrics.MessageProcessingDuration.WithLabelValues("transformer").Observe(elapsed.Seconds())
			log.Info("transform complete", "id", req.MessageID, "routing_key", sendRoutingKey, "duration_ms", elapsed.Milliseconds())

			span.End()
			delivery.Ack(false)
		}
	}()

	<-sigChan
	log.Info("transformer shutting down")
}

// recordFailure writes a transformation_failed transaction to the audit log.
func recordFailure(ctx context.Context, txRepo storage.TransactionRepository, messageID, errMsg string, log *slog.Logger) {
	tx := &models.Transaction{
		ID:        uuid.New().String(),
		MessageID: messageID,
		Event:     "transformation_failed",
		Details:   json.RawMessage(`{"error":"` + errMsg + `"}`),
		Timestamp: time.Now(),
	}
	if err := txRepo.Record(ctx, tx); err != nil {
		log.Warn("failed to record failure transaction", "id", messageID, "error", err)
	}
}
