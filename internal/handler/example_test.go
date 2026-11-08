package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/voidrunr/go-url-shortener/internal/audit"
	"github.com/voidrunr/go-url-shortener/internal/handler"
	"github.com/voidrunr/go-url-shortener/internal/model"
)

type exampleShortener struct{}

func (exampleShortener) Shorten(string, string) (string, error) {
	return "http://short.example/abc123", nil
}

func (exampleShortener) ShortenBatch(items []model.BatchItem, _ string) ([]model.BatchItem, error) {
	return items, nil
}

func (exampleShortener) Resolve(string) (string, error) {
	return "https://example.com", nil
}

func (exampleShortener) ListByUser(context.Context, string) ([]model.URL, error) {
	return nil, nil
}

func (exampleShortener) Delete(string, []string) error {
	return nil
}

type exampleAuditSink struct{}

func (exampleAuditSink) Write(ev audit.Event) error {
	fmt.Println(ev.Action)
	return nil
}

func ExampleWithAudit() {
	emitter := audit.NewEmitter()
	emitter.Subscribe(exampleAuditSink{})

	router := handler.New(exampleShortener{}, handler.WithAudit(emitter)).Router()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if err := emitter.Close(); err != nil {
		panic(err)
	}

	fmt.Println(rec.Code)
	// Output:
	// shorten
	// 201
}
