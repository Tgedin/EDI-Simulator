package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// API Gateway
	APIGatewayPort string
	APIGatewayHost string

	// PostgreSQL
	DBHost   string
	DBPort   string
	DBUser   string
	DBPass   string
	DBName   string
	DBSSLMode string

	// RabbitMQ
	RabbitMQURL             string
	RabbitMQExchange        string
	RabbitMQSendQueue       string // routing key for sender input queue
	RabbitMQReceiveQueue    string // routing key for receiver input queue
	RabbitMQTransformQueue  string // routing key for transformer input queue

	// Queue Reliability
	MaxRetries       int
	InitialBackoff   time.Duration
	MaxBackoff       time.Duration
	DLQTTLMs         int64 // Dead letter queue TTL in milliseconds

	// Observability
	OTELEndpoint string // e.g. "jaeger:4318"; empty disables tracing
	MetricsPort  string // e.g. ":9091"

	// Application
	LogLevel    string
	Environment string

	// Simulator
	SimulatorEnabled   bool
	SimulatorRate      int     // messages per minute
	SimulatorErrorRate float64 // fraction of malformed messages (0.0–1.0)
	SimulatorAPIURL    string
	SimulatorURL       string  // base URL of simulator metrics/control server (e.g. http://simulator:9094)

	// LLM / AI
	OllamaURL      string // base URL of Ollama API (e.g. http://ollama:11434)
	LLMServiceURL  string // base URL of llm-service (e.g. http://llm-service:9095)
	LLMServicePort string // port the llm-service listens on
	PrometheusURL  string // base URL of Prometheus (e.g. http://prometheus:9090)
}

func Load() *Config {
	return &Config{
		APIGatewayPort: getEnv("API_GATEWAY_PORT", "8080"),
		APIGatewayHost: getEnv("API_GATEWAY_HOST", "0.0.0.0"),

		DBHost:    getEnv("DB_HOST", "localhost"),
		DBPort:    getEnv("DB_PORT", "5432"),
		DBUser:    getEnv("DB_USER", "edi_user"),
		DBPass:    getEnv("DB_PASSWORD", "edi_password"),
		DBName:    getEnv("DB_NAME", "edi_simulator"),
		DBSSLMode: getEnv("DB_SSL_MODE", "disable"),

		RabbitMQURL:            getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RabbitMQExchange:       getEnv("RABBITMQ_EXCHANGE", "edi.messages"),
		RabbitMQSendQueue:      getEnv("RABBITMQ_SENDER_QUEUE", "sender"),
		RabbitMQReceiveQueue:   getEnv("RABBITMQ_RECEIVER_QUEUE", "receiver"),
		RabbitMQTransformQueue: getEnv("RABBITMQ_TRANSFORM_QUEUE", "transform"),

		MaxRetries:     getEnvInt("MAX_RETRIES", 3),
		InitialBackoff: time.Duration(getEnvInt("INITIAL_BACKOFF_MS", 1000)) * time.Millisecond,
		MaxBackoff:     time.Duration(getEnvInt("MAX_BACKOFF_MS", 4000)) * time.Millisecond,
		DLQTTLMs:       int64(getEnvInt("DLQ_TTL_MS", 3600000)), // 1 hour

		OTELEndpoint: getEnv("OTEL_ENDPOINT", ""),
		MetricsPort:  getEnv("METRICS_PORT", ":9091"),

		LogLevel:    getEnv("LOG_LEVEL", "info"),
		Environment: getEnv("ENVIRONMENT", "development"),

		SimulatorEnabled:   getEnvBool("SIMULATOR_ENABLED", true),
		SimulatorRate:      getEnvInt("SIMULATOR_RATE", 10),
		SimulatorErrorRate: getEnvFloat("SIMULATOR_ERROR_RATE", 0.15),
		SimulatorAPIURL:    getEnv("SIMULATOR_API_URL", "http://localhost:8080"),
		SimulatorURL:       getEnv("SIMULATOR_URL", "http://simulator:9094"),

		OllamaURL:      getEnv("OLLAMA_URL", "http://ollama:11434"),
		LLMServiceURL:  getEnv("LLM_SERVICE_URL", "http://llm-service:9095"),
		LLMServicePort: getEnv("LLM_SERVICE_PORT", "9095"),
		PrometheusURL:  getEnv("PROMETHEUS_URL", "http://prometheus:9090"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		switch value {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	}
	return defaultValue
}