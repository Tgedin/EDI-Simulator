package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const maxToolIterations = 3

// ChatMessage is a message in the Ollama chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolFunction describes an Ollama tool function.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Tool is an Ollama tool definition.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolCall is a tool invocation requested by the model.
type ToolCall struct {
	Function struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function"`
}

// OllamaMessage is the wire format Ollama uses (extends ChatMessage with tool_calls).
type OllamaMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ChatResponse is the top-level Ollama /api/chat response.
type ChatResponse struct {
	Message OllamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

type chatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Tools    []Tool          `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
}

// ToolExecutor is a function that executes a named tool and returns its result as a string.
type ToolExecutor func(name string, args map[string]any) (string, error)

// PostChat sends a single chat request to Ollama and returns the response.
func PostChat(ctx context.Context, baseURL, model string, messages []OllamaMessage, tools []Tool) (ChatResponse, error) {
	body := chatRequest{
		Model:    model,
		Messages: messages,
		Tools:    tools,
		Stream:   false,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/chat", bytes.NewReader(b))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ChatResponse{}, fmt.Errorf("ollama returned HTTP %d", resp.StatusCode)
	}

	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ChatResponse{}, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

// RunWithTools executes a tool-call loop: sends messages, executes any requested tool
// calls, appends results, and repeats up to maxToolIterations times, then returns the
// final assistant text and the list of tool names that were called.
func RunWithTools(ctx context.Context, baseURL, model string, messages []OllamaMessage, tools []Tool, executor ToolExecutor) (string, []string, error) {
	called := make([]string, 0)

	for i := 0; i < maxToolIterations; i++ {
		resp, err := PostChat(ctx, baseURL, model, messages, tools)
		if err != nil {
			return "", called, err
		}

		msg := resp.Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			return msg.Content, called, nil
		}

		// Execute each requested tool and append its result as a tool message.
		for _, tc := range msg.ToolCalls {
			called = append(called, tc.Function.Name)
			result, err := executor(tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("error: %s", err.Error())
			}
			messages = append(messages, OllamaMessage{
				Role:    "tool",
				Content: result,
			})
		}
	}

	// One final call with no tools to force a text response.
	resp, err := PostChat(ctx, baseURL, model, messages, nil)
	if err != nil {
		return "", called, err
	}
	return resp.Message.Content, called, nil
}
