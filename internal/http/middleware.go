package http

import (
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/theo-gedin/edi-simulator/internal/metrics"
)

// responseRecorder wraps http.ResponseWriter to capture the written status code.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// uuidRE matches UUID-shaped path segments so they can be normalised to {id}.
var uuidRE = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func normaliseMetricsPath(path string) string {
	return uuidRE.ReplaceAllString(path, "{id}")
}

// CORSMiddleware enables CORS for frontend requests.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs each HTTP request with structured fields using slog.Default().
// Because logger.New() calls slog.SetDefault(), the service-specific logger is used.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rec, r)

		elapsed := time.Since(start)
		path := normaliseMetricsPath(r.URL.Path)

		slog.Default().Info("http request",
			"method", r.Method,
			"path", path,
			"status", rec.statusCode,
			"duration_ms", elapsed.Milliseconds(),
		)

		// Prometheus metrics
		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(rec.statusCode)).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(elapsed.Seconds())
	})
}

// JSONResponseMiddleware adds the JSON content-type header.
func JSONResponseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
