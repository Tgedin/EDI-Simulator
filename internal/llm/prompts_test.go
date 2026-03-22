package llm

import (
	"strings"
	"testing"
)

func TestBuildMessages_ClassifyFailure(t *testing.T) {
	refID := "test-uuid-1234"
	msgs := BuildMessages("classify_failure", refID)

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("first message role: got %q, want %q", msgs[0].Role, "system")
	}
	if msgs[1].Role != "user" {
		t.Errorf("second message role: got %q, want %q", msgs[1].Role, "user")
	}
	if !strings.Contains(msgs[1].Content, refID) {
		t.Errorf("user message should contain refID %q; got %q", refID, msgs[1].Content)
	}
	// System prompt should instruct JSON-only output.
	if !strings.Contains(msgs[0].Content, "JSON") {
		t.Errorf("classify_failure system prompt should mention JSON format")
	}
	// System prompt should list at least one valid category.
	if !strings.Contains(msgs[0].Content, "malformed_content") {
		t.Errorf("classify_failure system prompt should list categories")
	}
}

func TestBuildMessages_HealthInsight(t *testing.T) {
	msgs := BuildMessages("health_insight", "")

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("first message role: got %q, want %q", msgs[0].Role, "system")
	}
	if msgs[1].Role != "user" {
		t.Errorf("second message role: got %q, want %q", msgs[1].Role, "user")
	}
	// InputRef is empty for health_insight — user message should not contain "".
	if strings.Contains(msgs[1].Content, "message ID: ") {
		t.Errorf("health_insight user message should not reference a message ID")
	}
}

func TestBuildMessages_DraftCommunication(t *testing.T) {
	refID := "draft-ref-5678"
	msgs := BuildMessages("draft_communication", refID)

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("first message role: got %q, want %q", msgs[0].Role, "system")
	}
	if !strings.Contains(msgs[1].Content, refID) {
		t.Errorf("user message should contain refID %q; got %q", refID, msgs[1].Content)
	}
	if !strings.Contains(msgs[0].Content, "email") {
		t.Errorf("draft_communication system prompt should mention email")
	}
}

func TestBuildMessages_UnknownType(t *testing.T) {
	msgs := BuildMessages("nonexistent_type", "some-ref")
	// Unknown type falls back to a single user message with the inputRef as content.
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message for unknown type, got %d", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("fallback message role: got %q, want %q", msgs[0].Role, "user")
	}
}
