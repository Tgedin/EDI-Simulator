package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/theo-gedin/edi-simulator/internal/config"
	"github.com/theo-gedin/edi-simulator/internal/logger"
	"github.com/theo-gedin/edi-simulator/internal/metrics"
)

// paused controls whether the simulator skips sending messages (0 = running, 1 = paused).
var paused int32

// partnersResponse matches the {"partners": [...]} envelope from GET /api/v1/partners.
type partnersResponse struct {
	Partners []partner `json:"partners"`
}

// partner is a minimal representation of what the API returns for /api/v1/partners.
type partner struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Format string `json:"preferred_format"`
}

// sendRequest mirrors the POST /api/v1/messages body.
type sendRequest struct {
	Format   string `json:"format"`
	Content  string `json:"content"`
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
}

// ── EDI content templates ────────────────────────────────────────────────────

func x12Order(docNum int, qty int, price float64) string {
	return fmt.Sprintf(
		"ISA*00*          *00*          *ZZ*SENDER         *ZZ*RECEIVER       *"+
			"%s*%s*X*00501*%09d*0*P*>~\n"+
			"GS*PO*SENDER*RECEIVER*%s*%s*%d*X*005010~\n"+
			"ST*850*0001~\n"+
			"BEG*00*SA*PO-%06d**%s~\n"+
			"PO1*1*%d*EA*%.2f**IN*ITEM001~\n"+
			"SE*4*0001~\n"+
			"GE*1*%d~\n"+
			"IEA*1*%09d~",
		time.Now().Format("060102"), time.Now().Format("1504"),
		docNum,
		time.Now().Format("20060102"), time.Now().Format("150405"), docNum,
		docNum, time.Now().Format("20060102"),
		qty, price,
		docNum, docNum,
	)
}

func edifactOrder(docNum int, qty int, price float64) string {
	return fmt.Sprintf(
		"UNB+UNOA:3+SENDER+RECEIVER+%s:%s+%d'\n"+
			"UNH+1+ORDERS:D:96A:UN:EAN008'\n"+
			"BGM+220+PO-%06d+9'\n"+
			"DTM+137:%s:102'\n"+
			"LIN+1++ITEM001:IN'\n"+
			"QTY+21:%d'\n"+
			"PRI+AAA:%.2f'\n"+
			"UNT+7+1'\n"+
			"UNZ+1+%d'",
		time.Now().Format("060102"), time.Now().Format("1504"), docNum,
		docNum, time.Now().Format("20060102"),
		qty, price, docNum,
	)
}

func xmlOrder(docNum int, qty int, price float64) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<PurchaseOrder>
  <PONumber>PO-%06d</PONumber>
  <OrderDate>%s</OrderDate>
  <LineItems>
    <LineItem>
      <ItemID>ITEM001</ItemID>
      <Quantity>%d</Quantity>
      <UnitPrice>%.2f</UnitPrice>
    </LineItem>
  </LineItems>
</PurchaseOrder>`, docNum, time.Now().Format("2006-01-02"), qty, price)
}

// generateContent returns EDI content for the given format.
// errorType is one of "", "malformed", or "schema_mismatch".
func generateContent(format string, docNum int, errorType string) string {
	if errorType == "schema_mismatch" {
		return generateSchemaMismatch(format, docNum)
	}

	qty := rand.Intn(90) + 10
	price := float64(rand.Intn(9000)+1000) / 100.0

	var content string
	switch strings.ToLower(format) {
	case "x12":
		content = x12Order(docNum, qty, price)
	case "edifact":
		content = edifactOrder(docNum, qty, price)
	default:
		content = xmlOrder(docNum, qty, price)
	}

	if errorType == "malformed" {
		// Corrupt the content at a random position with a null byte.
		b := []byte(content)
		if len(b) > 10 {
			pos := rand.Intn(len(b)-5) + 5
			b[pos] = 0x00
			content = string(b[:pos]) + "<<<CORRUPT>>>" + string(b[pos:])
		}
	}
	return content
}

// generateSchemaMismatch produces structurally valid EDI envelopes where
// numeric fields (quantity, price) contain invalid string literals.
// This simulates a schema-mismatch failure distinct from binary corruption.
func generateSchemaMismatch(format string, docNum int) string {
	badQty := "PRICE_MISSING"
	badPrice := "QTY_INVALID"
	todayDate := time.Now().Format("20060102")
	todayTS := time.Now().Format("150405")
	switch strings.ToLower(format) {
	case "x12":
		return fmt.Sprintf(
			"ISA*00*          *00*          *ZZ*SENDER         *ZZ*RECEIVER       *"+
				"%s*%s*X*00501*%09d*0*P*>~\n"+
				"GS*PO*SENDER*RECEIVER*%s*%s*%d*X*005010~\n"+
				"ST*850*0001~\n"+
				"BEG*00*SA*PO-%06d**%s~\n"+
				"PO1*1*%s*EA*%s**IN*ITEM001~\n"+
				"SE*4*0001~\n"+
				"GE*1*%d~\n"+
				"IEA*1*%09d~",
			time.Now().Format("060102"), time.Now().Format("1504"),
			docNum,
			todayDate, todayTS, docNum,
			docNum, todayDate,
			badQty, badPrice,
			docNum, docNum,
		)
	case "edifact":
		return fmt.Sprintf(
			"UNB+UNOA:3+SENDER+RECEIVER+%s:%s+%d'\n"+
				"UNH+1+ORDERS:D:96A:UN:EAN008'\n"+
				"BGM+220+PO-%06d+9'\n"+
				"DTM+137:%s:102'\n"+
				"LIN+1++ITEM001:IN'\n"+
				"QTY+21:%s'\n"+
				"PRI+AAA:%s'\n"+
				"UNT+7+1'\n"+
				"UNZ+1+%d'",
			time.Now().Format("060102"), time.Now().Format("1504"), docNum,
			docNum, todayDate,
			badQty, badPrice, docNum,
		)
	default: // XML
		return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<PurchaseOrder>
  <PONumber>PO-%06d</PONumber>
  <OrderDate>%s</OrderDate>
  <LineItems>
    <LineItem>
      <ItemID>ITEM001</ItemID>
      <Quantity>%s</Quantity>
      <UnitPrice>%s</UnitPrice>
    </LineItem>
  </LineItems>
</PurchaseOrder>`, docNum, todayDate, badQty, badPrice)
	}
}

// ── Main ─────────────────────────────────────────────────────────────────────

func main() {
	cfg := config.Load()
	log := logger.New("simulator", cfg.LogLevel)

	if !cfg.SimulatorEnabled {
		log.Info("simulator disabled, exiting")
		return
	}

	// Prometheus metrics server + control endpoints
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())

		mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]bool{"paused": atomic.LoadInt32(&paused) == 1})
		})

		mux.HandleFunc("POST /control", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Action string `json:"action"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
				return
			}
			switch req.Action {
			case "pause":
				atomic.StoreInt32(&paused, 1)
				log.Info("simulator paused")
			case "resume":
				atomic.StoreInt32(&paused, 0)
				log.Info("simulator resumed")
			default:
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "action must be 'pause' or 'resume'"})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]bool{"paused": atomic.LoadInt32(&paused) == 1})
		})

		if err := http.ListenAndServe(cfg.MetricsPort, mux); err != nil {
			log.Error("metrics server failed", "error", err)
		}
	}()

	// Fetch partner list with retry
	var partners []partner
	apiURL := cfg.SimulatorAPIURL
	for attempt := 1; attempt <= 10; attempt++ {
		resp, err := http.Get(apiURL + "/api/v1/partners") //nolint:gosec // URL is internal config
		if err == nil && resp.StatusCode == http.StatusOK {
			var envelope partnersResponse
			if jsonErr := json.NewDecoder(resp.Body).Decode(&envelope); jsonErr == nil && len(envelope.Partners) >= 2 {
				resp.Body.Close()
				partners = envelope.Partners
				break
			}
			resp.Body.Close()
		}
		log.Warn("waiting for api-gateway", "attempt", attempt)
		time.Sleep(5 * time.Second)
	}

	if len(partners) < 2 {
		log.Error("could not fetch at least 2 partners after retries, exiting")
		os.Exit(1)
	}

	log.Info("simulator started", "partners", len(partners), "rate_per_min", cfg.SimulatorRate, "error_rate", cfg.SimulatorErrorRate)

	interval := time.Duration(60.0/float64(cfg.SimulatorRate)*float64(time.Second))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	docNum := rand.Intn(900000) + 100000
	formats := []string{"X12", "EDIFACT", "XML"}

	for {
		select {
		case <-sigChan:
			log.Info("simulator shutting down")
			return

		case <-ticker.C:
			if atomic.LoadInt32(&paused) == 1 {
				continue
			}
			docNum++
			// Split error budget: 50% binary-corrupt, 50% schema-mismatch.
			errorType := ""
			if rand.Float64() < cfg.SimulatorErrorRate {
				if rand.Intn(2) == 0 {
					errorType = "malformed"
				} else {
					errorType = "schema_mismatch"
				}
			}

			// Pick random sender/receiver pair (must be different)
			senderIdx := rand.Intn(len(partners))
			receiverIdx := rand.Intn(len(partners) - 1)
			if receiverIdx >= senderIdx {
				receiverIdx++
			}
			sender := partners[senderIdx]
			receiver := partners[receiverIdx]

			// Use sender's preferred format if set, otherwise random
			format := sender.Format
			if format == "" {
				format = formats[rand.Intn(len(formats))]
			}

			content := generateContent(format, docNum, errorType)

			body, _ := json.Marshal(sendRequest{
				Format:   format,
				Content:  content,
				Sender:   sender.ID,
				Receiver: receiver.ID,
			})

			resp, err := http.Post(apiURL+"/api/v1/messages", "application/json", bytes.NewReader(body)) //nolint:gosec
			hasError := errorType != ""
			malformedStr := "false"
			if hasError {
				malformedStr = "true"
			}
			if err != nil || resp.StatusCode >= 400 {
				statusCode := 0
				if resp != nil {
					statusCode = resp.StatusCode
					resp.Body.Close()
				}
				log.Warn("failed to send simulated message", "format", format, "error_type", errorType, "status", statusCode, "error", err)
			} else {
				resp.Body.Close()
				log.Info("sent simulated message", "format", format, "error_type", errorType, "doc_num", docNum, "sender", sender.Name, "receiver", receiver.Name)
			}

			metrics.SimulatorMessagesTotal.WithLabelValues(format, malformedStr).Inc()
		}
	}
}
