package llm

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ToolDeps holds dependencies needed by tool executors.
type ToolDeps struct {
	DB            *sql.DB
	PrometheusURL string // e.g. "http://prometheus:9090"
}

// ExecutorFunc is a function that executes a tool call.
type ExecutorFunc func(args map[string]any, deps ToolDeps) (string, error)

// ToolRegistry maps tool names to their executor functions and holds shared deps.
type ToolRegistry struct {
	executors map[string]ExecutorFunc
	deps      ToolDeps
}

// NewToolRegistry creates a registry pre-loaded with all four EDI tools.
func NewToolRegistry(deps ToolDeps) *ToolRegistry {
	r := &ToolRegistry{
		executors: make(map[string]ExecutorFunc),
		deps:      deps,
	}
	r.executors["get_message"] = execGetMessage
	r.executors["get_recent_transactions"] = execGetRecentTransactions
	r.executors["get_partner"] = execGetPartner
	r.executors["get_queue_stats"] = execGetQueueStats
	r.executors["query_prometheus"] = execQueryPrometheus
	return r
}

// Execute runs the named tool and returns its string result.
func (r *ToolRegistry) Execute(name string, args map[string]any) (string, error) {
	fn, ok := r.executors[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return fn(args, r.deps)
}

// AllTools returns the Ollama Tool definitions for all registered tools.
func AllTools() []Tool {
	return []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_message",
				Description: "Retrieve an EDI message by ID, including its format, status, sender, receiver, and content preview.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{
							"type":        "string",
							"description": "The UUID of the message",
						},
					},
					"required": []string{"id"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_recent_transactions",
				Description: "Get the most recent processing events for a given message (e.g. message_created, validation_failed, dlq_retry_triggered).",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"message_id": map[string]any{
							"type":        "string",
							"description": "The UUID of the message",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Maximum number of events to return (default 5, max 20)",
						},
					},
					"required": []string{"message_id"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_partner",
				Description: "Retrieve a trading partner by ID, including their name and preferred EDI format.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{
							"type":        "string",
							"description": "The UUID of the trading partner",
						},
					},
					"required": []string{"id"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_queue_stats",
				Description: "Get message counts by status for the last hour, showing the overall health of the EDI processing pipeline.",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
					"required":   []string{},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "query_prometheus",
				Description: "Execute a PromQL query against Prometheus and return the result. Use this to get real-time metrics: message throughput rates, error rates, queue depths, and processing latency percentiles.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "A valid PromQL expression, e.g. 'edi_queue_depth' or 'rate(edi_messages_processed_total[5m])'",
						},
					},
					"required": []string{"query"},
				},
			},
		},
	}
}

// ─── Tool executors ───────────────────────────────────────────────────────────

func execGetMessage(args map[string]any, deps ToolDeps) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	var msgID, format, status, sender, receiver, content string
	err := deps.DB.QueryRow(`
		SELECT id, format, status, sender, receiver, LEFT(content, 500)
		FROM messages WHERE id = $1`, id).
		Scan(&msgID, &format, &status, &sender, &receiver, &content)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("message %s not found", id)
	}
	if err != nil {
		return "", err
	}

	result, _ := json.Marshal(map[string]string{
		"id":       msgID,
		"format":   format,
		"status":   status,
		"sender":   sender,
		"receiver": receiver,
		"content":  content,
	})
	return string(result), nil
}

func execGetRecentTransactions(args map[string]any, deps ToolDeps) (string, error) {
	messageID, _ := args["message_id"].(string)
	if messageID == "" {
		return "", fmt.Errorf("message_id is required")
	}
	limit := 5
	if l, ok := args["limit"]; ok {
		switch v := l.(type) {
		case float64:
			limit = int(v)
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				limit = n
			}
		}
	}
	if limit <= 0 || limit > 20 {
		limit = 5
	}

	rows, err := deps.DB.Query(`
		SELECT event, details, timestamp
		FROM transactions
		WHERE message_id = $1
		ORDER BY timestamp DESC
		LIMIT $2`, messageID, limit)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	type txnRow struct {
		Event     string    `json:"event"`
		Details   string    `json:"details"`
		Timestamp time.Time `json:"timestamp"`
	}
	var txns []txnRow
	for rows.Next() {
		var event string
		var detailsBytes []byte
		var ts time.Time
		if err := rows.Scan(&event, &detailsBytes, &ts); err != nil {
			continue
		}
		txns = append(txns, txnRow{
			Event:     event,
			Details:   string(detailsBytes),
			Timestamp: ts,
		})
	}
	result, _ := json.Marshal(txns)
	return string(result), nil
}

func execGetPartner(args map[string]any, deps ToolDeps) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	var partnerID, name, format string
	err := deps.DB.QueryRow(`
		SELECT id, name, preferred_format
		FROM trading_partners WHERE id = $1`, id).
		Scan(&partnerID, &name, &format)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("partner %s not found", id)
	}
	if err != nil {
		return "", err
	}

	result, _ := json.Marshal(map[string]string{
		"id":               partnerID,
		"name":             name,
		"preferred_format": format,
	})
	return string(result), nil
}

func execGetQueueStats(args map[string]any, deps ToolDeps) (string, error) {
	rows, err := deps.DB.Query(`
		SELECT status, COUNT(*) AS count
		FROM messages
		WHERE created_at > now() - interval '1 hour'
		GROUP BY status`)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		stats[status] = count
	}
	total := 0
	for _, v := range stats {
		total += v
	}
	stats["total_last_hour"] = total

	result, _ := json.Marshal(stats)
	return string(result), nil
}

func execQueryPrometheus(args map[string]any, deps ToolDeps) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	if deps.PrometheusURL == "" {
		return "", fmt.Errorf("prometheus not configured")
	}

	params := url.Values{}
	params.Set("query", query)
	fullURL := deps.PrometheusURL + "/api/v1/query?" + params.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fullURL) //nolint:noctx
	if err != nil {
		return "", fmt.Errorf("prometheus request failed: %w", err)
	}
	defer resp.Body.Close()

	var promResp struct {
		Status string         `json:"status"`
		Data   map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&promResp); err != nil {
		return "", fmt.Errorf("failed to decode prometheus response: %w", err)
	}
	if promResp.Status != "success" {
		return "", fmt.Errorf("prometheus returned status: %s", promResp.Status)
	}

	out, _ := json.Marshal(promResp.Data)
	return string(out), nil
}
