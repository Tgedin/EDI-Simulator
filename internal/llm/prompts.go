package llm

import "fmt"

// BuildMessages returns the system + user chat messages for a given job type.
// inputRef is a message UUID for classify_failure and draft_communication; empty for health_insight.
func BuildMessages(jobType, inputRef string) []OllamaMessage {
	switch jobType {
	case "classify_failure":
		return []OllamaMessage{
			{
				Role: "system",
				Content: "You are an EDI error analyst. Use the get_message and get_recent_transactions tools " +
					"to retrieve details about the failed message. " +
					"Classify the root cause as EXACTLY ONE of: malformed_content, duplicate, schema_mismatch, partner_config, unknown. " +
					`Respond ONLY with valid JSON in this exact format: {"category":"...","confidence":"high|medium|low","explanation":"one sentence"}. ` +
					"No other text outside the JSON.",
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Classify the failure for message ID: %s", inputRef),
			},
		}

	case "health_insight":
		return []OllamaMessage{
			{
				Role: "system",
				Content: "You are a system health analyst for an EDI processing pipeline. " +
					"Use the get_queue_stats tool to retrieve current message counts. " +
					"Respond in exactly 2 sentences: the first describes what the current metrics suggest " +
					"about the system's health, the second names one specific thing worth investigating.",
			},
			{
				Role:    "user",
				Content: "Give me a health insight for the EDI system right now.",
			},
		}

	case "draft_communication":
		return []OllamaMessage{
			{
				Role: "system",
				Content: "You are a professional EDI coordinator. " +
					"Use the get_message and get_partner tools to understand the issue and identify the trading partner. " +
					"Draft a 3-sentence professional email body (no subject line) to the trading partner explaining the problem. " +
					"Output only the email body, nothing else.",
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Draft a communication for message ID: %s", inputRef),
			},
		}

	case "metrics_analysis":
		return []OllamaMessage{
			{
				Role: "system",
				Content: "You are a system performance analyst for an EDI processing pipeline. " +
					"Use the query_prometheus tool to fetch these four metrics: " +
					"(1) edi_messages_processed_total — total message counts by status, " +
					"(2) rate(edi_messages_processed_total[5m]) — current throughput per second by status, " +
					"(3) edi_queue_depth — current queue depth gauge, " +
					"(4) histogram_quantile(0.95, rate(edi_message_processing_duration_seconds_bucket[5m])) — p95 processing latency. " +
					"After fetching all four, respond in exactly 3 sentences: " +
					"the first summarizes overall throughput and error rate from the data, " +
					"the second describes queue health and p95 latency, " +
					"the third names the single most important thing to investigate based on these numbers.",
			},
			{
				Role:    "user",
				Content: "Analyze the current EDI pipeline metrics from Prometheus.",
			},
		}

	default:
		return []OllamaMessage{
			{Role: "user", Content: inputRef},
		}
	}
}