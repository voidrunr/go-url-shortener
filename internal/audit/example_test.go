package audit_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/voidrunr/go-url-shortener/internal/audit"
)

type exampleSink struct{}

func (exampleSink) Write(ev audit.Event) error {
	fmt.Println(ev.Action)
	return nil
}

func ExampleNewEmitter() {
	emitter := audit.NewEmitter()
	emitter.Subscribe(exampleSink{})

	emitter.Notify(audit.Event{
		TS:     1,
		Action: audit.ActionShorten,
		UserID: "user-1",
		URL:    "https://example.com",
	})

	if err := emitter.Close(); err != nil {
		panic(err)
	}
	// Output:
	// shorten
}

func ExampleNewFileSink() {
	dir, err := os.MkdirTemp("", "audit-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "audit.log")
	sink := audit.NewFileSink(path)

	err = sink.Write(audit.Event{
		TS:     1,
		Action: audit.ActionShorten,
		UserID: "user-1",
		URL:    "https://example.com",
	})
	if err != nil {
		panic(err)
	}
	err = sink.Close()
	if err != nil {
		panic(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(data))
	// Output:
	// {"action":"shorten","user_id":"user-1","url":"https://example.com","ts":1}
}

func ExampleNewHTTPSink() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ev audit.Event
		if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Println(ev.Action)
	}))
	defer server.Close()

	sink := audit.NewHTTPSink(server.URL)
	if err := sink.Write(audit.Event{
		TS:     1,
		Action: audit.ActionFollow,
		URL:    "https://example.com",
	}); err != nil {
		panic(err)
	}
	if err := sink.Close(); err != nil {
		panic(err)
	}
	// Output:
	// follow
}
