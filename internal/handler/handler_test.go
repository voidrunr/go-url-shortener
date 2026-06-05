package handler_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/voidrunr/go-url-shortener/internal/handler"
	"github.com/voidrunr/go-url-shortener/internal/repository"
)

// --- mock ---

type mockShortener struct {
	shortenFn func(string) (string, error)
	resolveFn func(string) (string, error)
}

func (m *mockShortener) Shorten(url string) (string, error) { return m.shortenFn(url) }
func (m *mockShortener) Resolve(id string) (string, error)  { return m.resolveFn(id) }

func newHandler(svc handler.Shortener) http.Handler {
	return handler.New(svc).Router()
}

// --- POST / ---

func TestHandleShorten(t *testing.T) {
	type want struct {
		code        int
		body        string
		contentType string
	}
	tests := []struct {
		name string
		body string
		svc  handler.Shortener
		want want
	}{
		{
			name: "valid URL returns 201 and short URL",
			body: "https://practicum.yandex.ru/",
			svc: &mockShortener{
				shortenFn: func(string) (string, error) {
					return "http://localhost:8080/EwHXdJfB", nil
				},
			},
			want: want{
				code:        http.StatusCreated,
				body:        "http://localhost:8080/EwHXdJfB",
				contentType: "text/plain",
			},
		},
		{
			name: "empty body returns 400",
			body: "",
			svc:  &mockShortener{},
			want: want{code: http.StatusBadRequest},
		},
		{
			name: "whitespace-only body returns 400",
			body: "   \n",
			svc:  &mockShortener{},
			want: want{code: http.StatusBadRequest},
		},
		{
			name: "service error returns 500",
			body: "https://example.com",
			svc: &mockShortener{
				shortenFn: func(string) (string, error) {
					return "", io.ErrUnexpectedEOF
				},
			},
			want: want{code: http.StatusInternalServerError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "text/plain")
			w := httptest.NewRecorder()

			newHandler(tt.svc).ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.code, res.StatusCode)

			if tt.want.body != "" {
				resBody, err := io.ReadAll(res.Body)
				require.NoError(t, err)
				assert.Equal(t, tt.want.body, string(resBody))
			}

			if tt.want.contentType != "" {
				assert.Equal(t, tt.want.contentType, res.Header.Get("Content-Type"))
			}
		})
	}
}

func TestHandleShorten_WrongPath(t *testing.T) {
	type want struct {
		code int
	}
	tests := []struct {
		name   string
		method string
		path   string
		want   want
	}{
		{
			name:   "POST to non-root path returns 405",
			method: http.MethodPost,
			path:   "/something",
			want:   want{code: http.StatusMethodNotAllowed},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader("https://example.com"))
			w := httptest.NewRecorder()

			newHandler(&mockShortener{}).ServeHTTP(w, req)

			assert.Equal(t, tt.want.code, w.Code)
		})
	}
}

// --- GET /{id} ---

func TestHandleResolve(t *testing.T) {
	type want struct {
		code     int
		location string
	}
	tests := []struct {
		name string
		path string
		svc  handler.Shortener
		want want
	}{
		{
			name: "known ID returns 307 with Location header",
			path: "/EwHXdJfB",
			svc: &mockShortener{
				resolveFn: func(id string) (string, error) {
					if id == "EwHXdJfB" {
						return "https://practicum.yandex.ru/", nil
					}
					return "", errors.New("Url not found")
				},
			},
			want: want{
				code:     http.StatusTemporaryRedirect,
				location: "https://practicum.yandex.ru/",
			},
		},
		{
			name: "unknown ID returns 400",
			path: "/unknown",
			svc: &mockShortener{
				resolveFn: func(string) (string, error) {
					return "", repository.ErrNotFound
				},
			},
			want: want{code: http.StatusBadRequest},
		},
		{
			name: "empty ID (GET /) returns 400",
			path: "/",
			svc:  &mockShortener{},
			want: want{code: http.StatusBadRequest},
		},
		{
			name: "nested path returns 404",
			path: "/a/b",
			svc:  &mockShortener{},
			want: want{code: http.StatusNotFound},
		},
		{
			name: "service error returns 500",
			path: "/EwHXdJfB",
			svc: &mockShortener{
				resolveFn: func(string) (string, error) {
					return "", io.ErrUnexpectedEOF
				},
			},
			want: want{code: http.StatusInternalServerError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			newHandler(tt.svc).ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.code, res.StatusCode)

			if tt.want.location != "" {
				assert.Equal(t, tt.want.location, res.Header.Get("Location"))
			}
		})
	}
}

// --- unsupported methods ---

func TestRoute_UnsupportedMethods(t *testing.T) {
	type want struct {
		code int
	}
	tests := []struct {
		name   string
		method string
		want   want
	}{
		{
			name:   "PUT returns 405",
			method: http.MethodPut,
			want:   want{code: http.StatusMethodNotAllowed},
		},
		{
			name:   "PATCH returns 405",
			method: http.MethodPatch,
			want:   want{code: http.StatusMethodNotAllowed},
		},
		{
			name:   "DELETE returns 405",
			method: http.MethodDelete,
			want:   want{code: http.StatusMethodNotAllowed},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/", nil)
			w := httptest.NewRecorder()

			newHandler(&mockShortener{}).ServeHTTP(w, req)

			assert.Equal(t, tt.want.code, w.Code)
		})
	}
}
