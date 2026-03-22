package llm

import (
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// TestAllTools verifies the tool registry returns all expected tools.
func TestAllTools(t *testing.T) {
	tools := AllTools()

	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}

	wantNames := []string{"get_message", "get_recent_transactions", "get_partner", "get_queue_stats"}
	for i, want := range wantNames {
		if tools[i].Function.Name != want {
			t.Errorf("tool[%d] name: got %q, want %q", i, tools[i].Function.Name, want)
		}
	}

	// Every tool must have type="function"
	for _, tool := range tools {
		if tool.Type != "function" {
			t.Errorf("tool %q has type %q, want %q", tool.Function.Name, tool.Type, "function")
		}
	}
}

// TestToolRegistry_Execute_UnknownTool checks that an unknown tool name returns an error.
func TestToolRegistry_Execute_UnknownTool(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	reg := NewToolRegistry(ToolDeps{DB: db})
	_, execErr := reg.Execute("nonexistent_tool", nil)
	if execErr == nil {
		t.Error("expected error for unknown tool, got nil")
	}
}

// TestExecGetMessage returns the expected JSON from a mocked DB row.
func TestExecGetMessage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "format", "status", "sender", "receiver", "content"}).
		AddRow("msg-1", "X12", "failed", "SENDER_001", "RECEIVER_002", "ISA*00...")

	mock.ExpectQuery(`SELECT id, format, status, sender, receiver`).
		WithArgs("msg-1").
		WillReturnRows(rows)

	reg := NewToolRegistry(ToolDeps{DB: db})
	result, err := reg.Execute("get_message", map[string]any{"id": "msg-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]string
	if jsonErr := json.Unmarshal([]byte(result), &got); jsonErr != nil {
		t.Fatalf("result is not valid JSON: %v — raw: %s", jsonErr, result)
	}
	if got["id"] != "msg-1" {
		t.Errorf("id: got %q, want %q", got["id"], "msg-1")
	}
	if got["format"] != "X12" {
		t.Errorf("format: got %q, want %q", got["format"], "X12")
	}
	if got["status"] != "failed" {
		t.Errorf("status: got %q, want %q", got["status"], "failed")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestExecGetMessage_NotFound verifies that a missing message returns an error.
func TestExecGetMessage_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT id, format, status, sender, receiver`).
		WithArgs("missing-id").
		WillReturnError(sql.ErrNoRows)

	reg := NewToolRegistry(ToolDeps{DB: db})
	_, execErr := reg.Execute("get_message", map[string]any{"id": "missing-id"})
	if execErr == nil {
		t.Error("expected error for missing message, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestExecGetQueueStats returns a non-empty JSON result (aggregated status counts).
func TestExecGetQueueStats(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"status", "count"}).
		AddRow("pending", 12).
		AddRow("failed", 3).
		AddRow("processed", 47)

	mock.ExpectQuery(`SELECT status, COUNT`).WillReturnRows(rows)

	reg := NewToolRegistry(ToolDeps{DB: db})
	result, err := reg.Execute("get_queue_stats", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result from get_queue_stats")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
