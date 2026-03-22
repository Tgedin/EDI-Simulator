package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpMiddleware "github.com/theo-gedin/edi-simulator/internal/http"
	"github.com/theo-gedin/edi-simulator/internal/config"
	"github.com/theo-gedin/edi-simulator/internal/logger"
	"github.com/theo-gedin/edi-simulator/internal/messaging"
	"github.com/theo-gedin/edi-simulator/internal/metrics"
	"github.com/theo-gedin/edi-simulator/internal/models"
	"github.com/theo-gedin/edi-simulator/internal/storage"
	"github.com/theo-gedin/edi-simulator/internal/transformation"
	"github.com/theo-gedin/edi-simulator/internal/tracing"
	"github.com/theo-gedin/edi-simulator/internal/validation"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	cfg := config.Load()
	log := logger.New("api-gateway", cfg.LogLevel)

	// Distributed tracing
	shutdownTracing := tracing.InitProvider("api-gateway", cfg.OTELEndpoint)
	defer shutdownTracing()

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
	mappingRepo := storage.NewPostgresMappingRepository(db)
	partnerRepo := storage.NewPostgresPartnerRepository(db)
	engine := transformation.NewTransformationEngine()

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

	// Background: refresh QueueDepth gauge every 15 s.
	go func() {
		sendQ := "messages." + cfg.RabbitMQSendQueue
		receiveQ := "messages." + cfg.RabbitMQReceiveQueue
		transformQ := "messages." + cfg.RabbitMQTransformQueue
		dlqSend := sendQ + ".dlq"
		dlqReceive := receiveQ + ".dlq"
		queues := []string{sendQ, receiveQ, transformQ, dlqSend, dlqReceive}
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			for _, q := range queues {
				if qs, qErr := rabbitmq.GetQueueStatus(q); qErr == nil {
					metrics.QueueDepth.WithLabelValues(q).Set(float64(qs.Messages))
				}
			}
		}
	}()

	mux := http.NewServeMux()

	// Prometheus /metrics on the same port.
	mux.Handle("GET /metrics", promhttp.Handler())

	// Middleware: otelhttp tracing → CORS → Logging+metrics → JSON content-type
	handler := otelhttp.NewHandler(
		httpMiddleware.CORSMiddleware(
			httpMiddleware.LoggingMiddleware(
				httpMiddleware.JSONResponseMiddleware(mux),
			),
		),
		"api-gateway",
	)

	// ── Health ────────────────────────────────────────────────────────────────

	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// ── Partners ──────────────────────────────────────────────────────────────

	mux.HandleFunc("GET /api/v1/partners", func(w http.ResponseWriter, r *http.Request) {
		partners, err := partnerRepo.ListActive(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"partners": partners})
	})

	mux.HandleFunc("GET /api/v1/partners/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		partner, err := partnerRepo.GetByID(r.Context(), id)
		if err != nil {
			if err == storage.ErrPartnerNotFound {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "partner not found"})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(partner)
	})

	// ── Messages ──────────────────────────────────────────────────────────────

	mux.HandleFunc("GET /api/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		messages, err := msgRepo.ListAll(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(messages)
	})

	mux.HandleFunc("GET /api/v1/messages/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		msg, err := msgRepo.GetByID(r.Context(), id)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(msg)
	})

	mux.HandleFunc("GET /api/v1/messages/{id}/transactions", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		transactions, err := txnRepo.GetByMessageID(r.Context(), id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(transactions)
	})

	mux.HandleFunc("POST /api/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Format   string          `json:"format"`
			Content  string          `json:"content"`
			Metadata json.RawMessage `json:"metadata,omitempty"`
			Sender   string          `json:"sender,omitempty"`
			Receiver string          `json:"receiver,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request format"})
			return
		}

		if err := validation.Validate(req.Format, req.Content); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		if len(req.Metadata) == 0 {
			req.Metadata = json.RawMessage("{}")
		}

		msg := &models.Message{
			ID:        uuid.New().String(),
			Format:    req.Format,
			Content:   req.Content,
			Metadata:  req.Metadata,
			Sender:    req.Sender,
			Receiver:  req.Receiver,
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := msgRepo.Store(r.Context(), msg); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		tx := &models.Transaction{
			ID:        uuid.New().String(),
			MessageID: msg.ID,
			Event:     "message_created",
			Details:   json.RawMessage(`{"format":"` + req.Format + `"}`),
			Timestamp: time.Now(),
		}
		txnRepo.Record(r.Context(), tx)

		// Publish to sender queue with trace context propagated.
		msgBytes, _ := json.Marshal(msg)
		pubCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := rabbitmq.PublishWithRetry(pubCtx, cfg.RabbitMQExchange, cfg.RabbitMQSendQueue, msgBytes, 0); err != nil {
			log.Warn("failed to publish to sender queue", "id", msg.ID, "error", err)
		}

		log.Info("message created", "id", msg.ID, "format", msg.Format, "sender", msg.Sender)

		w.Header().Set("Location", "/api/v1/messages/"+msg.ID)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(msg)
	})

	// ── Transformation ────────────────────────────────────────────────────────

	mux.HandleFunc("POST /api/v1/transform", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MessageID    string `json:"message_id"`
			SourceFormat string `json:"source_format"`
			TargetFormat string `json:"target_format"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request format"})
			return
		}

		msg, err := msgRepo.GetByID(r.Context(), req.MessageID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "message not found"})
			return
		}

		result, err := engine.Transform(req.SourceFormat, req.TargetFormat, msg.Content)
		if err != nil {
			metrics.TransformationsTotal.WithLabelValues(req.SourceFormat, req.TargetFormat, "error").Inc()
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		if canonXML, xmlErr := transformation.CanonicalXML(result.Canonical); xmlErr == nil {
			if metaErr := msgRepo.UpdateMetadata(r.Context(), msg.ID, canonXML); metaErr != nil {
				log.Warn("UpdateMetadata failed", "id", msg.ID, "error", metaErr)
			}
		}

		if statusErr := msgRepo.UpdateStatus(r.Context(), msg.ID, models.StatusTransformed); statusErr != nil {
			log.Warn("UpdateStatus failed", "id", msg.ID, "error", statusErr)
		}

		txDetails := json.RawMessage(`{"source":"` + req.SourceFormat + `","target":"` + req.TargetFormat + `"}`)
		tx := &models.Transaction{
			ID:        uuid.New().String(),
			MessageID: msg.ID,
			Event:     "transformation_complete",
			Details:   txDetails,
			Timestamp: time.Now(),
		}
		txnRepo.Record(r.Context(), tx)

		metrics.TransformationsTotal.WithLabelValues(req.SourceFormat, req.TargetFormat, "success").Inc()
		log.Info("transformation complete", "id", msg.ID, "source", req.SourceFormat, "target", req.TargetFormat)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message_id":       req.MessageID,
			"source_format":    req.SourceFormat,
			"target_format":    req.TargetFormat,
			"result":           result.Output,
			"canonical_stored": true,
		})
	})

	mux.HandleFunc("GET /api/v1/formats", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"formats":         engine.SupportedFormats(),
			"transformations": engine.SupportedTransformations(),
		})
	})

	mux.HandleFunc("GET /api/v1/transform/formats", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"formats": engine.SupportedFormats(),
		})
	})

	mux.HandleFunc("GET /api/v1/transform/mappings", func(w http.ResponseWriter, r *http.Request) {
		mappings, err := mappingRepo.ListActive(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"mappings": mappings})
	})

	mux.HandleFunc("POST /api/v1/transform/mappings", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name         string `json:"name"`
			SourceFormat string `json:"source_format"`
			TargetFormat string `json:"target_format"`
			Description  string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.SourceFormat == "" || req.TargetFormat == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "name, source_format, and target_format are required"})
			return
		}
		created, err := mappingRepo.Create(r.Context(), &storage.TransformationMapping{
			Name:         req.Name,
			SourceFormat: req.SourceFormat,
			TargetFormat: req.TargetFormat,
			Description:  req.Description,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)
	})

	mux.HandleFunc("GET /api/v1/transform/mappings/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		m, err := mappingRepo.GetByID(r.Context(), id)
		if err != nil {
			if err == storage.ErrMappingNotFound {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "mapping not found"})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(m)
	})

	mux.HandleFunc("PUT /api/v1/transform/mappings/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Active      *bool  `json:"active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request format"})
			return
		}
		existing, err := mappingRepo.GetByID(r.Context(), id)
		if err != nil {
			if err == storage.ErrMappingNotFound {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "mapping not found"})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if req.Name != "" {
			existing.Name = req.Name
		}
		if req.Description != "" {
			existing.Description = req.Description
		}
		if req.Active != nil {
			existing.Active = *req.Active
		}
		updated, err := mappingRepo.Update(r.Context(), existing)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(updated)
	})

	mux.HandleFunc("DELETE /api/v1/transform/mappings/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := mappingRepo.Delete(r.Context(), id); err != nil {
			if err == storage.ErrMappingNotFound {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "mapping not found"})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /api/v1/transform/preview", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SourceFormat string `json:"source_format"`
			TargetFormat string `json:"target_format"`
			Content      string `json:"content"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request format"})
			return
		}
		if req.Content == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "content cannot be empty"})
			return
		}

		result, err := engine.Transform(req.SourceFormat, req.TargetFormat, req.Content)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		canonXML, _ := transformation.CanonicalXML(result.Canonical)
		fieldsMapped := transformation.CountMappedFields(result.Canonical)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"source_format": req.SourceFormat,
			"target_format": req.TargetFormat,
			"input":         req.Content,
			"canonical":     canonXML,
			"output":        result.Output,
			"fields_mapped": fieldsMapped,
		})
	})

	// ── Queue ─────────────────────────────────────────────────────────────────

	mux.HandleFunc("GET /api/v1/queue/status", func(w http.ResponseWriter, r *http.Request) {
		sendQ := "messages." + cfg.RabbitMQSendQueue
		receiveQ := "messages." + cfg.RabbitMQReceiveQueue
		dlqSend := sendQ + ".dlq"
		dlqReceive := receiveQ + ".dlq"
		queues := []string{sendQ, receiveQ, dlqSend, dlqReceive}
		status := make([]map[string]interface{}, 0)

		for _, queueName := range queues {
			qStatus, err := rabbitmq.GetQueueStatus(queueName)
			if err != nil {
				log.Warn("failed to get queue status", "queue", queueName, "error", err)
				continue
			}
			metrics.QueueDepth.WithLabelValues(queueName).Set(float64(qStatus.Messages))
			status = append(status, map[string]interface{}{
				"name":      qStatus.Name,
				"messages":  qStatus.Messages,
				"consumers": qStatus.Consumers,
			})
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"queues": status})
	})

	mux.HandleFunc("GET /api/v1/queue/dlq", func(w http.ResponseWriter, r *http.Request) {
		dlqType := r.URL.Query().Get("type")
		if dlqType == "" {
			dlqType = "send"
		}
		queueName := "messages." + dlqType + ".dlq"
		qStatus, err := rabbitmq.GetQueueStatus(queueName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"dlq_name": queueName,
			"messages": qStatus.Messages,
		})
	})

	mux.HandleFunc("POST /api/v1/queue/dlq/{id}/retry", func(w http.ResponseWriter, r *http.Request) {
		messageID := r.PathValue("id")

		msg, err := msgRepo.GetByID(r.Context(), messageID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "message not found"})
			return
		}

		txDetails := json.RawMessage(`{"action":"manual_dlq_retry"}`)
		tx := &models.Transaction{
			ID:        uuid.New().String(),
			MessageID: msg.ID,
			Event:     "dlq_retry_triggered",
			Details:   txDetails,
			Timestamp: time.Now(),
		}
		txnRepo.Record(r.Context(), tx)

		msgBytes, _ := json.Marshal(msg)
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if err := rabbitmq.PublishWithRetry(ctx, cfg.RabbitMQExchange, "send", msgBytes, 0); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		log.Info("DLQ retry triggered", "id", messageID)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message_id": messageID,
			"status":     "retry_queued",
		})
	})

	mux.HandleFunc("POST /api/v1/simulate", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"message": "Simulation triggered"})
	})

	// ── Simulator control (proxy to simulator's internal HTTP server) ──────────

	mux.HandleFunc("GET /api/v1/simulator/status", func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get(cfg.SimulatorURL + "/status") //nolint:gosec
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": "simulator unreachable"})
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body) //nolint:errcheck
	})

	mux.HandleFunc("POST /api/v1/simulator/control", func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Post(cfg.SimulatorURL+"/control", "application/json", r.Body) //nolint:gosec
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": "simulator unreachable"})
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body) //nolint:errcheck
	})

	// ── LLM service proxy ─────────────────────────────────────────────────────

	mux.HandleFunc("POST /api/v1/llm/jobs", func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Post(cfg.LLMServiceURL+"/jobs", "application/json", r.Body) //nolint:gosec
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": "llm service unreachable"})
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body) //nolint:errcheck
	})

	mux.HandleFunc("GET /api/v1/llm/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		resp, err := http.Get(cfg.LLMServiceURL + "/jobs/" + id) //nolint:gosec
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": "llm service unreachable"})
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body) //nolint:errcheck
	})

	// ── Start server ──────────────────────────────────────────────────────────

	address := cfg.APIGatewayHost + ":" + cfg.APIGatewayPort
	server := &http.Server{
		Addr:    address,
		Handler: handler,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		slog.Default().Info("shutting down API gateway")
		server.Close()
	}()

	log.Info("API gateway listening", "address", "http://"+address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("server error", "error", err)
		os.Exit(1)
	}
}

