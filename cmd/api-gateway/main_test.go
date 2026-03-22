package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/theo-gedin/edi-simulator/internal/models"
	"github.com/theo-gedin/edi-simulator/internal/storage"
	"github.com/theo-gedin/edi-simulator/internal/transformation"
	"github.com/theo-gedin/edi-simulator/internal/validation"
)

// setupTestServer creates a test HTTP server with all routes
func setupTestServer() (*http.ServeMux, *storage.MockMessageRepository, *storage.MockTransactionRepository) {
	msgRepo := storage.NewMockMessageRepository()
	txnRepo := storage.NewMockTransactionRepository()
	mappingRepo := storage.NewMockMappingRepository()
	partnerRepo := storage.NewMockPartnerRepository()
	engine := transformation.NewTransformationEngine()

	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// GET /api/v1/partners - List all active trading partners
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

	// GET /api/v1/partners/{id}
	mux.HandleFunc("GET /api/v1/partners/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		partner, err := partnerRepo.GetByID(r.Context(), id)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "partner not found"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(partner)
	})

	// POST /api/v1/messages - Create message
	mux.HandleFunc("POST /api/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Format   string          `json:"format"`
			Content  string          `json:"content"`
			Metadata json.RawMessage `json:"metadata"`
			Sender   string          `json:"sender"`
			Receiver string          `json:"receiver"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request format"})
			return
		}

		// Validate format
		if err := validation.Validate(req.Format, req.Content); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		msg := &models.Message{
			ID:        "msg-" + time.Now().Format("20060102150405"),
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
			ID:        "tx-" + time.Now().Format("20060102150405"),
			MessageID: msg.ID,
			Event:     "message_created",
			Details:   json.RawMessage(`{"format":"` + req.Format + `"}`),
			Timestamp: time.Now(),
		}
		txnRepo.Record(r.Context(), tx)

		w.Header().Set("Location", "/api/v1/messages/"+msg.ID)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(msg)
	})

	// GET /api/v1/messages - List messages
	mux.HandleFunc("GET /api/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		messages, err := msgRepo.ListAll(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
	})

	// GET /api/v1/messages/{id}/transactions - Get transaction history
	mux.HandleFunc("GET /api/v1/messages/{id}/transactions", func(w http.ResponseWriter, r *http.Request) {
		messageID := r.PathValue("id")

		txs, err := txnRepo.GetByMessageID(r.Context(), messageID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(txs)
	})

	// POST /api/v1/transform - Transform message (stateful)
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
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		if canonXML, xmlErr := transformation.CanonicalXML(result.Canonical); xmlErr == nil {
			msgRepo.UpdateMetadata(r.Context(), msg.ID, canonXML)
		}

		// Advance status to transformed
		msgRepo.UpdateStatus(r.Context(), msg.ID, models.StatusTransformed)

		txDetails := json.RawMessage(`{"source":"` + req.SourceFormat + `","target":"` + req.TargetFormat + `"}`)
		tx := &models.Transaction{
			ID:        "tx-transform-" + req.MessageID,
			MessageID: msg.ID,
			Event:     "transformation_complete",
			Details:   txDetails,
			Timestamp: time.Now(),
		}
		txnRepo.Record(r.Context(), tx)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message_id":       req.MessageID,
			"source_format":    req.SourceFormat,
			"target_format":    req.TargetFormat,
			"result":           result.Output,
			"canonical_stored": true,
		})
	})
	// GET /api/v1/formats - List formats
	mux.HandleFunc("GET /api/v1/formats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"formats":         engine.SupportedFormats(),
			"transformations": engine.SupportedTransformations(),
		})
	})

	// GET /api/v1/transform/formats
	mux.HandleFunc("GET /api/v1/transform/formats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"formats": engine.SupportedFormats(),
		})
	})

	// GET /api/v1/transform/mappings
	mux.HandleFunc("GET /api/v1/transform/mappings", func(w http.ResponseWriter, r *http.Request) {
		mappings, _ := mappingRepo.ListActive(r.Context())
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mappings": mappings,
		})
	})

	// POST /api/v1/transform/mappings - Create mapping
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
			Name: req.Name, SourceFormat: req.SourceFormat,
			TargetFormat: req.TargetFormat, Description: req.Description,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)
	})

	// GET /api/v1/transform/mappings/{id}
	mux.HandleFunc("GET /api/v1/transform/mappings/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		m, err := mappingRepo.GetByID(r.Context(), id)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "mapping not found"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(m)
	})

	// PUT /api/v1/transform/mappings/{id}
	mux.HandleFunc("PUT /api/v1/transform/mappings/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Active      *bool  `json:"active"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		existing, err := mappingRepo.GetByID(r.Context(), id)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "mapping not found"})
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

	// DELETE /api/v1/transform/mappings/{id}
	mux.HandleFunc("DELETE /api/v1/transform/mappings/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := mappingRepo.Delete(r.Context(), id); err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "mapping not found"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// POST /api/v1/transform/preview
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
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"source_format": req.SourceFormat,
			"target_format": req.TargetFormat,
			"input":         req.Content,
			"canonical":     canonXML,
			"output":        result.Output,
			"fields_mapped": transformation.CountMappedFields(result.Canonical),
		})
	})

	return mux, msgRepo, txnRepo
}

// --- Tests ---

// TestHealthEndpoint tests the health check endpoint
func TestHealthEndpoint(t *testing.T) {
	mux, _, _ := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response["status"])
	}
}

// TestCreateMessageX12 tests creating an X12 message
func TestCreateMessageX12(t *testing.T) {
	mux, _, _ := setupTestServer()

	x12Content := `ISA*00*          *00*          *01*9876543210     *01*1234567890     *220215*1430*^*00501*000000001*0*T*:
GS*PO*9876543210*1234567890*20220215*143000*1*X*005010
ST*850*000001
SE*2*000001
GE*1*1
IEA*1*000000001`

	payload := map[string]interface{}{
		"format":  "x12",
		"content": x12Content,
		"metadata": map[string]string{
			"order_number": "ORD-123",
		},
	}

	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewReader(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response models.Message
	json.Unmarshal(w.Body.Bytes(), &response)

	if response.Format != "x12" {
		t.Errorf("Expected format 'x12', got '%s'", response.Format)
	}

	if response.Status != models.StatusPending {
		t.Errorf("Expected status 'pending', got '%s'", response.Status)
	}
}

// TestCreateMessageEDIFACT tests creating an EDIFACT message
func TestCreateMessageEDIFACT(t *testing.T) {
	mux, _, _ := setupTestServer()

	edifactContent := `UNA:+.? '
UNB+IATB:1+1SNDPROC+2RCVPROC+2602151200:1234+1+ORDERS'
UNH+1+ORDERS:D:96A:UN
UNT+1+1
UNZ+1+1`

	payload := map[string]interface{}{
		"format":  "edifact",
		"content": edifactContent,
	}

	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewReader(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response models.Message
	json.Unmarshal(w.Body.Bytes(), &response)

	if response.Format != "edifact" {
		t.Errorf("Expected format 'edifact', got '%s'", response.Format)
	}
}

// TestCreateMessageInvalidFormat tests creating message with invalid format
func TestCreateMessageInvalidFormat(t *testing.T) {
	mux, _, _ := setupTestServer()

	payload := map[string]interface{}{
		"format":  "json",
		"content": "{}",
	}

	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewReader(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestCreateMessageInvalidX12 tests creating message with invalid X12 content
func TestCreateMessageInvalidX12(t *testing.T) {
	mux, _, _ := setupTestServer()

	payload := map[string]interface{}{
		"format":  "x12",
		"content": "This is not valid X12",
	}

	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewReader(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestListMessages tests listing all messages
func TestListMessages(t *testing.T) {
	mux, msgRepo, _ := setupTestServer()

	// Store a test message
	msg := &models.Message{
		ID:        "msg-1",
		Format:    "x12",
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	msgRepo.Store(context.Background(), msg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var messages []models.Message
	json.Unmarshal(w.Body.Bytes(), &messages)

	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
}

// TestGetTransactions tests retrieving message transactions
func TestGetTransactions(t *testing.T) {
	mux, msgRepo, txnRepo := setupTestServer()

	// Store test message and transaction
	msg := &models.Message{
		ID:        "msg-1",
		Format:    "x12",
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	msgRepo.Store(context.Background(), msg)

	tx := &models.Transaction{
		ID:        "tx-1",
		MessageID: "msg-1",
		Event:     "message_created",
		Details:   json.RawMessage(`{"format":"x12"}`),
		Timestamp: time.Now(),
	}
	txnRepo.Record(context.Background(), tx)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/msg-1/transactions", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var transactions []models.Transaction
	json.Unmarshal(w.Body.Bytes(), &transactions)

	if len(transactions) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(transactions))
	}

	if transactions[0].Event != "message_created" {
		t.Errorf("Expected event 'message_created', got '%s'", transactions[0].Event)
	}
}

// TestTransformMessage tests message transformation
func TestTransformMessage(t *testing.T) {
	mux, msgRepo, _ := setupTestServer()

	// Store test message
	x12Content := `ISA*00*          *00*          *01*9876543210     *01*1234567890     *220215*1430*^*00501*000000001*0*T*:
GS*PO*9876543210*1234567890*20220215*143000*1*X*005010
ST*850*000001
SE*2*000001
GE*1*1
IEA*1*000000001`

	msg := &models.Message{
		ID:        "msg-1",
		Format:    "x12",
		Content:   x12Content,
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	msgRepo.Store(context.Background(), msg)

	payload := map[string]interface{}{
		"message_id":    "msg-1",
		"source_format": "x12",
		"target_format": "edifact",
	}

	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transform", bytes.NewReader(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["target_format"] != "edifact" {
		t.Error("Expected target_format 'edifact' in response")
	}

	if _, ok := response["result"]; !ok {
		t.Error("Expected result in response")
	}
}

// TestTransformMessageNotFound tests transform for non-existent message
func TestTransformMessageNotFound(t *testing.T) {
	mux, _, _ := setupTestServer()

	payload := map[string]interface{}{
		"message_id":    "non-existent",
		"source_format": "x12",
		"target_format": "edifact",
	}

	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transform", bytes.NewReader(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestListFormats tests the formats endpoint
func TestListFormats(t *testing.T) {
	mux, _, _ := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/formats", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if formats, ok := response["formats"]; !ok || len(formats.([]interface{})) == 0 {
		t.Error("Expected formats in response")
	}

	if transformations, ok := response["transformations"]; !ok || len(transformations.([]interface{})) == 0 {
		t.Error("Expected transformations in response")
	}
}

// TestCreateAndListMessageFlow tests end-to-end message creation and listing
func TestCreateAndListMessageFlow(t *testing.T) {
	mux, _, _ := setupTestServer()

	// Create message
	x12Content := `ISA*00*          *00*          *01*9876543210     *01*1234567890     *220215*1430*^*00501*000000001*0*T*:
GS*PO*9876543210*1234567890*20220215*143000*1*X*005010
ST*850*000001
SE*2*000001
GE*1*1
IEA*1*000000001`

	payload := map[string]interface{}{
		"format":  "x12",
		"content": x12Content,
	}

	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewReader(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Create failed with status %d", w.Code)
		return
	}

	// List messages
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
	w2 := httptest.NewRecorder()

	mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("List failed with status %d", w2.Code)
		return
	}

	var messages []models.Message
	json.Unmarshal(w2.Body.Bytes(), &messages)

	if len(messages) != 1 {
		t.Errorf("Expected 1 message after creation, got %d", len(messages))
	}

	if messages[0].Format != "x12" {
		t.Errorf("Expected format 'x12', got '%s'", messages[0].Format)
	}
}

// TestInvalidJSON tests endpoint with invalid JSON payload
func TestInvalidJSON(t *testing.T) {
	mux, _, _ := setupTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestMissingContentType tests POST with missing content
func TestMissingContent(t *testing.T) {
	mux, _, _ := setupTestServer()

	payload := map[string]interface{}{
		"format": "x12",
		// Missing content
	}

	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewReader(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

const testX12Sample = "ISA*00*          *00*          *ZZ*SENDER         *ZZ*RECEIVER       *260215*1200*U*00401*000000001*0*T*>~\nGS*OE*SENDER*RECEIVER*20260215*1200*1*X*004010~\nST*850*0001~\nBEG*00*SA*PO-12345**20260215~\nN1*BY*Buyer Corp*ZZ*BUYER01~\nN1*SE*Seller Inc*ZZ*SELLER01~\nPO1*1*10*EA*9.99**VP*WIDGET-A~\nSE*5*0001~\nGE*1*1~\nIEA*1*000000001~\n"

// TestGetTransformFormats tests GET /api/v1/transform/formats
func TestGetTransformFormats(t *testing.T) {
	mux, _, _ := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transform/formats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	formats, ok := resp["formats"].([]interface{})
	if !ok || len(formats) == 0 {
		t.Errorf("Expected non-empty formats slice, got %v", resp["formats"])
	}

	expectedFormats := map[string]bool{"x12": false, "edifact": false, "xml": false}
	for _, f := range formats {
		if s, ok := f.(string); ok {
			expectedFormats[s] = true
		}
	}
	for name, found := range expectedFormats {
		if !found {
			t.Errorf("Expected format '%s' not found in response", name)
		}
	}
}

// TestGetTransformMappings tests GET /api/v1/transform/mappings
func TestGetTransformMappings(t *testing.T) {
	mux, _, _ := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transform/mappings", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	mappings, ok := resp["mappings"].([]interface{})
	if !ok {
		t.Fatalf("Expected mappings array, got %T", resp["mappings"])
	}
	if len(mappings) < 4 {
		t.Errorf("Expected at least 4 mappings, got %d", len(mappings))
	}
}

// TestTransformPreviewEndpoint tests POST /api/v1/transform/preview with valid X12 input
func TestTransformPreviewEndpoint(t *testing.T) {
	mux, _, _ := setupTestServer()

	payload := map[string]string{
		"source_format": "x12",
		"target_format": "edifact",
		"content":       testX12Sample,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transform/preview", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	requiredKeys := []string{"source_format", "target_format", "input", "canonical", "output", "fields_mapped"}
	for _, key := range requiredKeys {
		if _, ok := resp[key]; !ok {
			t.Errorf("Expected key '%s' in response", key)
		}
	}

	if resp["source_format"] != "x12" {
		t.Errorf("Expected source_format 'x12', got %v", resp["source_format"])
	}
	if resp["target_format"] != "edifact" {
		t.Errorf("Expected target_format 'edifact', got %v", resp["target_format"])
	}

	fieldsRaw, _ := resp["fields_mapped"].(float64)
	if int(fieldsRaw) <= 0 {
		t.Errorf("Expected fields_mapped > 0, got %v", resp["fields_mapped"])
	}

	output, _ := resp["output"].(string)
	if output == "" {
		t.Errorf("Expected non-empty output")
	}

	canonical, _ := resp["canonical"].(string)
	if canonical == "" {
		t.Errorf("Expected non-empty canonical XML")
	}
}

// TestTransformPreviewEndpoint_BadFormat tests preview with unknown format
func TestTransformPreviewEndpoint_BadFormat(t *testing.T) {
	mux, _, _ := setupTestServer()

	payload := map[string]string{
		"source_format": "unknown",
		"target_format": "edifact",
		"content":       testX12Sample,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transform/preview", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for unknown format, got %d", w.Code)
	}
}

// TestTransformPreviewEndpoint_EmptyContent tests preview with empty content
func TestTransformPreviewEndpoint_EmptyContent(t *testing.T) {
	mux, _, _ := setupTestServer()

	payload := map[string]string{
		"source_format": "x12",
		"target_format": "edifact",
		"content":       "",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transform/preview", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty content, got %d", w.Code)
	}
}

// TestTransformMessage_StoresCanonical tests that a transform request stores canonical XML in metadata
func TestTransformMessage_StoresCanonical(t *testing.T) {
	mux, msgRepo, _ := setupTestServer()

	// Store a message to transform
	createPayload := map[string]string{
		"format":  "x12",
		"content": testX12Sample,
	}
	createBody, _ := json.Marshal(createPayload)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewReader(createBody))
	createW := httptest.NewRecorder()
	mux.ServeHTTP(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("Expected 201 on create, got %d", createW.Code)
	}

	var createResp map[string]interface{}
	json.Unmarshal(createW.Body.Bytes(), &createResp)
	msgID, _ := createResp["id"].(string)

	// Transform it
	transformPayload := map[string]string{
		"message_id":    msgID,
		"source_format": "x12",
		"target_format": "edifact",
	}
	transformBody, _ := json.Marshal(transformPayload)
	transformReq := httptest.NewRequest(http.MethodPost, "/api/v1/transform", bytes.NewReader(transformBody))
	transformW := httptest.NewRecorder()
	mux.ServeHTTP(transformW, transformReq)

	if transformW.Code != http.StatusOK {
		t.Fatalf("Expected 200 on transform, got %d: %s", transformW.Code, transformW.Body.String())
	}

	// Verify that canonical XML was stored in metadata
	storedMsg, err := msgRepo.GetByID(context.Background(), msgID)
	if err != nil {
		t.Fatalf("Failed to retrieve message after transform: %v", err)
	}
	if len(storedMsg.Metadata) == 0 {
		t.Errorf("Expected non-empty metadata after transform (canonical XML should be stored)")
	}

	var resp map[string]interface{}
	json.Unmarshal(transformW.Body.Bytes(), &resp)
	if resp["canonical_stored"] != true {
		t.Errorf("Expected canonical_stored: true in transform response, got %v", resp["canonical_stored"])
	}
}

// ---- Phase 5: Transformation Engine tests ----

// TestTransformMessage_UpdatesStatus verifies that POST /api/v1/transform advances status to "transformed"
func TestTransformMessage_UpdatesStatus(t *testing.T) {
	mux, msgRepo, _ := setupTestServer()

	msg := &models.Message{
		ID:        "msg-status-test",
		Format:    "x12",
		Content:   testX12Sample,
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	msgRepo.Store(context.Background(), msg)

	payload := map[string]string{
		"message_id":    "msg-status-test",
		"source_format": "x12",
		"target_format": "edifact",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transform", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	updated, err := msgRepo.GetByID(context.Background(), "msg-status-test")
	if err != nil {
		t.Fatalf("Failed to retrieve message: %v", err)
	}
	if updated.Status != models.StatusTransformed {
		t.Errorf("Expected status %q after transform, got %q", models.StatusTransformed, updated.Status)
	}
}

// TestTransformMessage_RecordsAuditEvent verifies that transformation_complete event is stored
func TestTransformMessage_RecordsAuditEvent(t *testing.T) {
	mux, msgRepo, txnRepo := setupTestServer()

	msg := &models.Message{
		ID:        "msg-audit-test",
		Format:    "x12",
		Content:   testX12Sample,
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	msgRepo.Store(context.Background(), msg)

	payload := map[string]string{
		"message_id":    "msg-audit-test",
		"source_format": "x12",
		"target_format": "xml",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transform", bytes.NewReader(body))
	mux.ServeHTTP(httptest.NewRecorder(), req)

	txs, err := txnRepo.GetByMessageID(context.Background(), "msg-audit-test")
	if err != nil {
		t.Fatalf("Failed to retrieve transactions: %v", err)
	}

	var found bool
	for _, tx := range txs {
		if tx.Event == "transformation_complete" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'transformation_complete' audit event, got %+v", txs)
	}
}

// TestCreateMessageXML tests creating an XML-format message
func TestCreateMessageXML(t *testing.T) {
	mux, _, _ := setupTestServer()

	payload := map[string]string{
		"format":   "xml",
		"content":  "<order><id>ORD-001</id><amount>250</amount></order>",
		"sender":   "SenderA",
		"receiver": "ReceiverB",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.Message
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Format != "xml" {
		t.Errorf("Expected format 'xml', got %q", resp.Format)
	}
	if resp.Status != models.StatusPending {
		t.Errorf("Expected status 'pending', got %q", resp.Status)
	}
}

// TestTransformAllPairs verifies all 6 supported transformation pairs succeed via preview
func TestTransformAllPairs(t *testing.T) {
	mux, _, _ := setupTestServer()

	testEDIFACT := "UNA:+.? '\nUNB+IATB:1+SENDER+RECEIVER+260301:1200+1+ORDERS'\nUNH+1+ORDERS:D:96A:UN\nUNT+1+1\nUNZ+1+1"
	testXML := "<Document><PurchaseOrder><Number>PO-001</Number><Buyer>BuyerCo</Buyer><Seller>SellerInc</Seller></PurchaseOrder></Document>"

	pairs := []struct {
		from, to, content string
	}{
		{"x12", "edifact", testX12Sample},
		{"x12", "xml", testX12Sample},
		{"edifact", "x12", testEDIFACT},
		{"edifact", "xml", testEDIFACT},
		{"xml", "x12", testXML},
		{"xml", "edifact", testXML},
	}

	for _, p := range pairs {
		t.Run(p.from+"->"+p.to, func(t *testing.T) {
			payload := map[string]string{
				"source_format": p.from,
				"target_format": p.to,
				"content":       p.content,
			}
			body, _ := json.Marshal(payload)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/transform/preview", bytes.NewReader(body))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Pair %s->%s: expected 200, got %d: %s", p.from, p.to, w.Code, w.Body.String())
				return
			}

			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)

			if output, _ := resp["output"].(string); output == "" {
				t.Errorf("Pair %s->%s: expected non-empty output", p.from, p.to)
			}
			if canonical, _ := resp["canonical"].(string); canonical == "" {
				t.Errorf("Pair %s->%s: expected non-empty canonical XML", p.from, p.to)
			}
		})
	}
}

// --- Partner tests ---

// TestListPartners tests GET /api/v1/partners returns 8 seeded companies
func TestListPartners(t *testing.T) {
	mux, _, _ := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/partners", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Partners []struct {
			ID              string `json:"id"`
			Name            string `json:"name"`
			PreferredFormat string `json:"preferred_format"`
			Country         string `json:"country"`
		} `json:"partners"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Partners) != 8 {
		t.Errorf("expected 8 partners, got %d", len(resp.Partners))
	}
}

// TestGetPartner tests GET /api/v1/partners/{id} for an existing partner
func TestGetPartner(t *testing.T) {
	mux, _, _ := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/partners/p1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var partner struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		PreferredFormat string `json:"preferred_format"`
	}
	json.Unmarshal(w.Body.Bytes(), &partner)

	if partner.ID != "p1" {
		t.Errorf("expected partner id 'p1', got %q", partner.ID)
	}
	if partner.PreferredFormat != "x12" {
		t.Errorf("expected preferred_format 'x12', got %q", partner.PreferredFormat)
	}
}

// TestGetPartner_NotFound tests 404 for unknown partner ID
func TestGetPartner_NotFound(t *testing.T) {
	mux, _, _ := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/partners/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- Mapping CRUD tests ---

// TestCreateMapping tests POST /api/v1/transform/mappings
func TestCreateMapping(t *testing.T) {
	mux, _, _ := setupTestServer()

	payload := map[string]string{
		"name":          "JSON -> X12",
		"source_format": "json",
		"target_format": "x12",
		"description":   "Test mapping",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transform/mappings", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created storage.TransformationMapping
	json.Unmarshal(w.Body.Bytes(), &created)

	if created.Name != "JSON -> X12" {
		t.Errorf("expected name 'JSON -> X12', got %q", created.Name)
	}
	if created.SourceFormat != "json" {
		t.Errorf("expected source_format 'json', got %q", created.SourceFormat)
	}
}

// TestCreateMapping_MissingFields tests 400 when required fields are absent
func TestCreateMapping_MissingFields(t *testing.T) {
	mux, _, _ := setupTestServer()

	payload := map[string]string{"name": "incomplete"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transform/mappings", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestGetMappingByID tests GET /api/v1/transform/mappings/{id}
func TestGetMappingByID(t *testing.T) {
	mux, _, _ := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transform/mappings/m1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var m storage.TransformationMapping
	json.Unmarshal(w.Body.Bytes(), &m)

	if m.ID != "m1" {
		t.Errorf("expected id 'm1', got %q", m.ID)
	}
}

// TestGetMappingByID_NotFound tests 404 for unknown mapping ID
func TestGetMappingByID_NotFound(t *testing.T) {
	mux, _, _ := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transform/mappings/no-such-id", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestUpdateMapping tests PUT /api/v1/transform/mappings/{id}
func TestUpdateMapping(t *testing.T) {
	mux, _, _ := setupTestServer()

	payload := map[string]string{"name": "Updated Name", "description": "New desc"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/transform/mappings/m1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated storage.TransformationMapping
	json.Unmarshal(w.Body.Bytes(), &updated)

	if updated.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got %q", updated.Name)
	}
	if updated.Description != "New desc" {
		t.Errorf("expected description 'New desc', got %q", updated.Description)
	}
}

// TestDeleteMapping tests DELETE /api/v1/transform/mappings/{id} soft-deletes a mapping
func TestDeleteMapping(t *testing.T) {
	mux, _, _ := setupTestServer()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/transform/mappings/m1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	// Confirm it no longer appears in active list
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/transform/mappings", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	var resp struct {
		Mappings []storage.TransformationMapping `json:"mappings"`
	}
	json.Unmarshal(w2.Body.Bytes(), &resp)

	for _, m := range resp.Mappings {
		if m.ID == "m1" {
			t.Error("deleted mapping m1 still appears in active list")
		}
	}
}

// TestDeleteMapping_NotFound tests 404 for unknown mapping ID
func TestDeleteMapping_NotFound(t *testing.T) {
	mux, _, _ := setupTestServer()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/transform/mappings/no-such-id", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── LLM proxy tests ───────────────────────────────────────────────────────────

// addLLMProxyRoutes registers the same LLM proxy handlers used in main.go
// but with an injectable llmServiceURL so tests can point to a mock server.
func addLLMProxyRoutes(mux *http.ServeMux, llmServiceURL string) {
	mux.HandleFunc("POST /api/v1/llm/jobs", func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Post(llmServiceURL+"/jobs", "application/json", r.Body) //nolint:gosec
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": "llm service unreachable"})
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		json.NewEncoder(w).Encode(json.RawMessage(mustReadBody(resp))) //nolint:errcheck
	})

	mux.HandleFunc("GET /api/v1/llm/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		resp, err := http.Get(llmServiceURL + "/jobs/" + id) //nolint:gosec
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": "llm service unreachable"})
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		json.NewEncoder(w).Encode(json.RawMessage(mustReadBody(resp))) //nolint:errcheck
	})
}

func mustReadBody(resp *http.Response) []byte {
	b := make([]byte, 0, 512)
	buf := bytes.NewBuffer(b)
	buf.ReadFrom(resp.Body) //nolint:errcheck
	return buf.Bytes()
}

// TestLLMJobCreate verifies that POST /api/v1/llm/jobs proxies to the LLM service
// and returns the job_id from the upstream response.
func TestLLMJobCreate(t *testing.T) {
	// Mock llm-service that returns 201 with a job ID.
	llmMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/jobs" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"job_id": "test-job-001"})
	}))
	defer llmMock.Close()

	mux := http.NewServeMux()
	addLLMProxyRoutes(mux, llmMock.URL)

	body, _ := json.Marshal(map[string]string{"type": "health_insight"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/llm/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["job_id"] != "test-job-001" {
		t.Errorf("expected job_id=test-job-001, got %v", resp["job_id"])
	}
}

// TestLLMJobGet verifies that GET /api/v1/llm/jobs/{id} proxies the job response.
func TestLLMJobGet(t *testing.T) {
	llmMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/jobs/test-job-001" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":     "test-job-001",
			"type":   "health_insight",
			"status": "done",
			"result": map[string]string{"text": "System looks healthy."},
		})
	}))
	defer llmMock.Close()

	mux := http.NewServeMux()
	addLLMProxyRoutes(mux, llmMock.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/llm/jobs/test-job-001", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "done" {
		t.Errorf("expected status=done, got %v", resp["status"])
	}
}

// TestLLMServiceUnreachable verifies that a 502 is returned when the LLM service is down.
func TestLLMServiceUnreachable(t *testing.T) {
	// Use a port that is definitely not listening.
	mux := http.NewServeMux()
	addLLMProxyRoutes(mux, "http://127.0.0.1:19999")

	body, _ := json.Marshal(map[string]string{"type": "health_insight"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/llm/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] == "" {
		t.Error("expected error message in body, got empty")
	}
}
