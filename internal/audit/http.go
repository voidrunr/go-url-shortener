package audit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

const defaultHTTPTimeout = 3 * time.Second

// HTTPSink delivers audit events to an HTTP endpoint.
type HTTPSink struct {
	url    string
	client *http.Client
}

// NewHTTPSink returns an HTTPSink that posts JSON events to url. Requests time
// out after 3 seconds.
func NewHTTPSink(url string) *HTTPSink {
	return &HTTPSink{
		url:    url,
		client: &http.Client{Timeout: defaultHTTPTimeout},
	}
}

// Write posts ev as a JSON body. Write does not retry failed deliveries.
func (h *HTTPSink) Write(ev Event) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	resp, err := h.client.Post(h.url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Close releases idle keep-alive connections held by the sink.
func (h *HTTPSink) Close() error {
	h.client.CloseIdleConnections()
	return nil
}
