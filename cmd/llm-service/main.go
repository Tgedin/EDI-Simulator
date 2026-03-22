package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/theo-gedin/edi-simulator/internal/config"
	"github.com/theo-gedin/edi-simulator/internal/llm"
	"github.com/theo-gedin/edi-simulator/internal/logger"
	"github.com/theo-gedin/edi-simulator/internal/storage"
)

const ollamaModel = "qwen2.5:3b"

func main() {
	cfg := config.Load()
	log := logger.New("llm-service", cfg.LogLevel)

	db, err := storage.ConnectPostgres(
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPass, cfg.DBName, cfg.DBSSLMode,
	)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Background worker: claim and process one pending job per second.
	go runWorker(db, cfg.OllamaURL, cfg.PrometheusURL, log)

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())

	// POST /jobs — create an async LLM job, with dedup for classify/draft.
	mux.HandleFunc("POST /jobs", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Type     string `json:"type"`
			InputRef string `json:"input_ref,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Type == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "type is required"})
			return
		}

		// Dedup: return an existing active or done job if one exists for this (type, input_ref).
		if req.Type != "health_insight" && req.InputRef != "" {
			var existingID string
			dedupErr := db.QueryRowContext(r.Context(),
				`SELECT id FROM llm_jobs
				 WHERE type=$1 AND input_ref=$2 AND status IN ('pending','running','done')
				 ORDER BY created_at DESC LIMIT 1`,
				req.Type, req.InputRef).Scan(&existingID)
			if dedupErr == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"job_id": existingID})
				return
			}
		}

		jobID := uuid.New().String()
		_, err := db.ExecContext(r.Context(),
			`INSERT INTO llm_jobs (id, type, input_ref, status)
			 VALUES ($1, $2, NULLIF($3, '')::uuid, 'pending')`,
			jobID, req.Type, req.InputRef)
		if err != nil {
			log.Error("failed to insert job", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "failed to create job"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"job_id": jobID})
	})

	// GET /jobs/{id} — poll job status and result.
	mux.HandleFunc("GET /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var (
			jobID       string
			jobType     string
			inputRef    sql.NullString
			status      string
			result      []byte
			jobError    sql.NullString
			createdAt   time.Time
			completedAt sql.NullTime
		)
		err := db.QueryRowContext(r.Context(),
			`SELECT id, type, input_ref, status, result, error, created_at, completed_at
			 FROM llm_jobs WHERE id=$1`, id).
			Scan(&jobID, &jobType, &inputRef, &status, &result, &jobError, &createdAt, &completedAt)
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "job not found"})
			return
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		resp := map[string]any{
			"id":         jobID,
			"type":       jobType,
			"status":     status,
			"created_at": createdAt,
		}
		if inputRef.Valid {
			resp["input_ref"] = inputRef.String
		}
		if len(result) > 0 {
			resp["result"] = json.RawMessage(result)
		}
		if jobError.Valid {
			resp["error"] = jobError.String
		}
		if completedAt.Valid {
			resp["completed_at"] = completedAt.Time
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	})

	port := cfg.LLMServicePort
	if port == "" {
		port = "9095"
	}
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx) //nolint:errcheck
	}()

	log.Info("LLM service listening", "port", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("server error", "error", err)
		os.Exit(1)
	}
}

// runWorker polls the DB every second and processes the next pending job.
func runWorker(db *sql.DB, ollamaURL, prometheusURL string, log *slog.Logger) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		processNextJob(db, ollamaURL, prometheusURL, log)
	}
}

func processNextJob(db *sql.DB, ollamaURL, prometheusURL string, log *slog.Logger) {
	ctx := context.Background()

	// Atomically claim the oldest pending job.
	var jobID, jobType string
	var inputRef sql.NullString
	err := db.QueryRowContext(ctx, `
		UPDATE llm_jobs SET status='running'
		WHERE id = (
			SELECT id FROM llm_jobs
			WHERE status='pending'
			ORDER BY created_at
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, type, input_ref`).
		Scan(&jobID, &jobType, &inputRef)
	if err == sql.ErrNoRows {
		return // nothing pending
	}
	if err != nil {
		log.Error("worker: failed to claim job", "error", err)
		return
	}

	ref := ""
	if inputRef.Valid {
		ref = inputRef.String
	}
	log.Info("worker: processing job", "id", jobID, "type", jobType)

	jobCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	result, jobErr := runJob(jobCtx, db, ollamaURL, prometheusURL, jobType, ref, log)
	now := time.Now()

	if jobErr != nil {
		log.Error("worker: job failed", "id", jobID, "error", jobErr)
		finalStatus := "error"
		if jobCtx.Err() == context.DeadlineExceeded {
			finalStatus = "timeout"
		}
		db.ExecContext(ctx, //nolint:errcheck
			`UPDATE llm_jobs SET status=$1, error=$2, completed_at=$3 WHERE id=$4`,
			finalStatus, jobErr.Error(), now, jobID)
		return
	}

	db.ExecContext(ctx, //nolint:errcheck
		`UPDATE llm_jobs SET status='done', result=$1, completed_at=$2 WHERE id=$3`,
		result, now, jobID)
	log.Info("worker: job done", "id", jobID)
}

func runJob(ctx context.Context, db *sql.DB, ollamaURL, prometheusURL, jobType, inputRef string, log *slog.Logger) (json.RawMessage, error) {
	messages := llm.BuildMessages(jobType, inputRef)
	tools := llm.AllTools()
	registry := llm.NewToolRegistry(llm.ToolDeps{DB: db, PrometheusURL: prometheusURL})

	text, toolsCalled, err := llm.RunWithTools(ctx, ollamaURL, ollamaModel, messages, tools, registry.Execute)
	if err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}
	log.Info("worker: inference complete", "tools_called", toolsCalled, "response_len", len(text))

	// For classify_failure the model should return valid JSON directly.
	// Models sometimes wrap it in extra prose, so extract the first {...} block.
	if jobType == "classify_failure" {
		extracted := llm.ExtractJSON(text)
		var parsed map[string]any
		if jsonErr := json.Unmarshal([]byte(extracted), &parsed); jsonErr == nil {
			result, _ := json.Marshal(parsed)
			return result, nil
		}
		// Still unparseable — return as plain explanation so the UI can display something.
		result, _ := json.Marshal(map[string]string{"category": "unknown", "confidence": "low", "explanation": text})
		return result, nil
	}

	result, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return nil, err
	}
	return result, nil
}


