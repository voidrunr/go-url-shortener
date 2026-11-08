package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type logEntry struct {
	URI      string  `json:"uri"`
	Method   string  `json:"method"`
	Status   int     `json:"status"`
	Size     int     `json:"size"`
	Duration float64 `json:"duration"`
	Msg      string  `json:"message"`
}

func captureLogs(t *testing.T, next http.Handler, method, target string) logEntry {
	t.Helper()

	var buf bytes.Buffer
	prev := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = prev })

	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	Logging(next).ServeHTTP(rec, req)

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected a log line, got empty output")
	}

	var entry logEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("log line is not valid JSON: %v (line: %s)", err, line)
	}
	return entry
}

func TestLoggingRecordsExplicitStatusAndSize(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("hello"))
	})

	entry := captureLogs(t, next, http.MethodPost, "/api/shorten")

	if entry.Status != http.StatusCreated {
		t.Errorf("status = %d, want %d", entry.Status, http.StatusCreated)
	}
	if entry.Size != 5 {
		t.Errorf("size = %d, want 5", entry.Size)
	}
	if entry.Method != http.MethodPost {
		t.Errorf("method = %q, want %q", entry.Method, http.MethodPost)
	}
	if entry.URI != "/api/shorten" {
		t.Errorf("uri = %q, want %q", entry.URI, "/api/shorten")
	}
	if entry.Duration <= 0 {
		t.Errorf("duration = %v, want > 0", entry.Duration)
	}
	if entry.Msg != "request" {
		t.Errorf("msg = %q, want %q", entry.Msg, "request")
	}
}

func TestLoggingDefaultsToStatusOK(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	entry := captureLogs(t, next, http.MethodGet, "/ping")

	if entry.Status != http.StatusOK {
		t.Errorf("status = %d, want %d (implicit write must be recorded as 200)", entry.Status, http.StatusOK)
	}
	if entry.Size != 2 {
		t.Errorf("size = %d, want 2", entry.Size)
	}
}

func TestLoggingDelegatesWriteHeader(t *testing.T) {
	var got int
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		got = w.(*responseWriter).statusCode
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	Logging(next).ServeHTTP(rec, req)

	if got != http.StatusTeapot {
		t.Errorf("wrapped statusCode = %d, want %d", got, http.StatusTeapot)
	}
	if rec.Code != http.StatusTeapot {
		t.Errorf("recorder code = %d, want %d", rec.Code, http.StatusTeapot)
	}
}

func TestLoggingSumsMultipleWrites(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("abc"))
		w.Write([]byte("de"))
	})

	entry := captureLogs(t, next, http.MethodGet, "/stream")

	if entry.Size != 5 {
		t.Errorf("size = %d, want 5 (sum of two writes)", entry.Size)
	}
}
