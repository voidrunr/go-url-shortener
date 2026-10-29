package audit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

const defaultHTTPTimeout = 3 * time.Second

type HTTPSink struct {
	url    string
	client *http.Client
}

func NewHTTPSink(url string) *HTTPSink {
	return &HTTPSink{
		url:    url,
		client: &http.Client{Timeout: defaultHTTPTimeout},
	}
}

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
