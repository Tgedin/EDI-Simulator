// Package pipeline provides integration tests for the full message pipeline.
// It verifies that a message transitions correctly through pending → sent → received
// using in-memory mocks for storage and a simple channel-based broker simulation.
package pipeline

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/theo-gedin/edi-simulator/internal/models"
	"github.com/theo-gedin/edi-simulator/internal/storage"
	"github.com/theo-gedin/edi-simulator/internal/validation"
)

// ---------- minimal fake broker -------------------------------------------------

type fakeMessage struct {
	body    []byte
	headers map[string]interface{}
	acked   bool
	nacked  bool
	requeue bool
}

func (f *fakeMessage) ack()            { f.acked = true }
func (f *fakeMessage) nack(requeue bool) { f.nacked = true; f.requeue = requeue }

type fakeBroker struct {
	queues map[string]chan *fakeMessage
}

func newFakeBroker() *fakeBroker {
	return &fakeBroker{queues: make(map[string]chan *fakeMessage)}
}

func (b *fakeBroker) publish(queue string, body []byte, retryCount int) {
	if b.queues[queue] == nil {
		b.queues[queue] = make(chan *fakeMessage, 100)
	}
	b.queues[queue] <- &fakeMessage{
		body:    body,
		headers: map[string]interface{}{"x-retry-count": int32(retryCount)},
	}
}

func (b *fakeBroker) consume(queue string) *fakeMessage {
	if b.queues[queue] == nil {
		return nil
	}
	select {
	case m := <-b.queues[queue]:
		return m
	default:
		return nil
	}
}

// ---------- worker simulation ---------------------------------------------------

// simulateWorker mimics the worker: finds pending unpublished messages and publishes them.
func simulateWorker(msgRepo *storage.MockMessageRepository, broker *fakeBroker) error {
	ctx := context.Background()
	pending, err := msgRepo.GetByStatus(ctx, models.StatusPending)
	if err != nil {
		return err
	}
	for _, msg := range pending {
		if msg.PublishedAt != nil {
			continue
		}
		body, _ := json.Marshal(msg)
		broker.publish("messages.send", body, 0)
		// mark published
		now := time.Now()
		msg.PublishedAt = &now
		msgRepo.Messages[msg.ID] = &msg
	}
	return nil
}

// ---------- sender simulation ---------------------------------------------------

// simulateSender mimics the sender: consumes from send queue, validates, updates status,
// then publishes to the receive queue.
func simulateSender(msgRepo *storage.MockMessageRepository, txRepo *storage.MockTransactionRepository, broker *fakeBroker) error {
	ctx := context.Background()
	m := broker.consume("messages.send")
	if m == nil {
		return nil
	}

	var msg models.Message
	if err := json.Unmarshal(m.body, &msg); err != nil {
		m.nack(false)
		return err
	}

	retryCount := 0
	if h, ok := m.headers["x-retry-count"]; ok {
		if c, ok := h.(int32); ok {
			retryCount = int(c)
		}
	}

	// Validate
	if err := validation.Validate(msg.Format, msg.Content); err != nil {
		txRepo.Record(ctx, &models.Transaction{
			ID: uuid.New().String(), MessageID: msg.ID, Event: "validation_failed",
			Details: json.RawMessage(`{"error":"` + err.Error() + `"}`), Timestamp: time.Now(),
		})
		msgRepo.UpdateStatus(ctx, msg.ID, models.StatusFailed)
		m.ack()
		return nil
	}

	// Update to sent
	if err := msgRepo.UpdateStatus(ctx, msg.ID, models.StatusSent); err != nil {
		m.nack(true)
		return err
	}

	txRepo.Record(ctx, &models.Transaction{
		ID: msg.ID + "-sent", MessageID: msg.ID, Event: "message_sent",
		Details:   json.RawMessage(`{"format":"` + msg.Format + `","retry":` + strconv.Itoa(retryCount) + `}`),
		Timestamp: time.Now(),
	})

	body, _ := json.Marshal(msg)
	broker.publish("messages.receive", body, 0)
	m.ack()
	return nil
}

// ---------- receiver simulation -------------------------------------------------

// simulateReceiver mimics the receiver: consumes from receive queue, validates,
// updates status to received, records transaction.
func simulateReceiver(msgRepo *storage.MockMessageRepository, txRepo *storage.MockTransactionRepository, broker *fakeBroker) error {
	ctx := context.Background()
	m := broker.consume("messages.receive")
	if m == nil {
		return nil
	}

	var msg models.Message
	if err := json.Unmarshal(m.body, &msg); err != nil {
		m.nack(false)
		return err
	}

	retryCount := 0
	if h, ok := m.headers["x-retry-count"]; ok {
		if c, ok := h.(int32); ok {
			retryCount = int(c)
		}
	}

	// Validate
	if err := validation.Validate(msg.Format, msg.Content); err != nil {
		txRepo.Record(ctx, &models.Transaction{
			ID: uuid.New().String(), MessageID: msg.ID, Event: "validation_failed",
			Details: json.RawMessage(`{"error":"` + err.Error() + `"}`), Timestamp: time.Now(),
		})
		m.ack()
		return nil
	}

	// Update to received — handles ErrMessageNotFound
	if err := msgRepo.UpdateStatus(ctx, msg.ID, models.StatusReceived); err != nil {
		if err == storage.ErrMessageNotFound {
			m.ack()
			return nil
		}
		m.nack(true)
		return err
	}

	txRepo.Record(ctx, &models.Transaction{
		ID: uuid.New().String(), MessageID: msg.ID, Event: "message_received",
		Details:   json.RawMessage(`{"status":"received","retry":` + strconv.Itoa(retryCount) + `}`),
		Timestamp: time.Now(),
	})
	m.ack()
	return nil
}

// validX12 is a complete X12 message satisfying all required segments.
const validX12 = `ISA*00*          *00*          *01*9876543210     *01*1234567890     *220215*1430*^*00501*000000001*0*T*:
GS*PO*9876543210*1234567890*20220215*143000*1*X*005010
ST*850*000001
BEG*00*NE*ORDER123**220215
SE*4*000001
GE*1*1
IEA*1*000000001`

// TestFullPipeline verifies the happy path: pending → sent → received.
func TestFullPipeline(t *testing.T) {
	msgRepo := storage.NewMockMessageRepository()
	txRepo := storage.NewMockTransactionRepository()
	broker := newFakeBroker()
	ctx := context.Background()

	// Create a valid X12 message in pending state
	msg := &models.Message{
		ID:        uuid.New().String(),
		Format:    "x12",
		Content:   validX12,

		Receiver:  "RECEIVER",
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	msgRepo.Store(ctx, msg)

	// Step 1: worker picks up pending message and publishes it
	if err := simulateWorker(msgRepo, broker); err != nil {
		t.Fatalf("worker failed: %v", err)
	}

	// Step 2: sender processes from send queue
	if err := simulateSender(msgRepo, txRepo, broker); err != nil {
		t.Fatalf("sender failed: %v", err)
	}

	sent, _ := msgRepo.GetByID(ctx, msg.ID)
	if sent.Status != models.StatusSent {
		t.Errorf("expected status=sent after sender, got %s", sent.Status)
	}

	// Step 3: receiver processes from receive queue
	if err := simulateReceiver(msgRepo, txRepo, broker); err != nil {
		t.Fatalf("receiver failed: %v", err)
	}

	received, _ := msgRepo.GetByID(ctx, msg.ID)
	if received.Status != models.StatusReceived {
		t.Errorf("expected status=received after receiver, got %s", received.Status)
	}

	// Verify transaction trail
	txs, _ := txRepo.GetByMessageID(ctx, msg.ID)
	events := make(map[string]bool)
	for _, tx := range txs {
		events[tx.Event] = true
	}
	if !events["message_sent"] {
		t.Error("expected message_sent transaction event")
	}
	if !events["message_received"] {
		t.Error("expected message_received transaction event")
	}
}

// TestReceiverHandlesMissingRow verifies that if a message is not found in the DB,
// the receiver acks and discards rather than crashing or requeueing.
func TestReceiverHandlesMissingRow(t *testing.T) {
	msgRepo := storage.NewMockMessageRepository()
	txRepo := storage.NewMockTransactionRepository()
	broker := newFakeBroker()

	// Publish a message to the receive queue that doesn't exist in DB
	ghost := &models.Message{
		ID:      uuid.New().String(),
		Format:  "x12",
		Content: validX12,

	}
	body, _ := json.Marshal(ghost)
	broker.publish("messages.receive", body, 0)

	// simulateReceiver should handle ErrMessageNotFound gracefully
	err := simulateReceiver(msgRepo, txRepo, broker)
	if err != nil {
		t.Errorf("receiver returned error for missing row, expected nil: %v", err)
	}
}

// TestSenderValidationFailure verifies that invalid messages are marked failed and not forwarded.
func TestSenderValidationFailure(t *testing.T) {
	msgRepo := storage.NewMockMessageRepository()
	txRepo := storage.NewMockTransactionRepository()
	broker := newFakeBroker()
	ctx := context.Background()

	msg := &models.Message{
		ID:        uuid.New().String(),
		Format:    "x12",
		Content:   "INVALID CONTENT",
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	msgRepo.Store(ctx, msg)

	simulateWorker(msgRepo, broker)
	simulateSender(msgRepo, txRepo, broker)

	updated, _ := msgRepo.GetByID(ctx, msg.ID)
	if updated.Status != models.StatusFailed {
		t.Errorf("expected status=failed for invalid message, got %s", updated.Status)
	}

	// Nothing should be on the receive queue
	if m := broker.consume("messages.receive"); m != nil {
		t.Error("expected no message on receive queue after validation failure")
	}
}
