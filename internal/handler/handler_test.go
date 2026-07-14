package handler_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/voidrunr/go-url-shortener/internal/handler"
	"github.com/voidrunr/go-url-shortener/internal/handler/mocks"
	"github.com/voidrunr/go-url-shortener/internal/repository"
)

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
		name  string
		body  string
		setup func(*mocks.Shortener)
		want  want
	}{
		{
			name: "valid URL returns 201 and short URL",
			body: "https://practicum.yandex.ru/",
			setup: func(m *mocks.Shortener) {
				m.EXPECT().Shorten("https://practicum.yandex.ru/").Return("http://localhost:8080/EwHXdJfB", nil)
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
			want: want{code: http.StatusBadRequest},
		},
		{
			name: "whitespace-only body returns 400",
			body: "   \n",
			want: want{code: http.StatusBadRequest},
		},
		{
			name: "service error returns 500",
			body: "https://example.com",
			setup: func(m *mocks.Shortener) {
				m.EXPECT().Shorten("https://example.com").Return("", io.ErrUnexpectedEOF)
			},
			want: want{code: http.StatusInternalServerError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewShortener(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "text/plain")
			w := httptest.NewRecorder()

			newHandler(svc).ServeHTTP(w, req)

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

// --- POST /api/shorten ---

func TestHandleAPIShorten(t *testing.T) {
	type want struct {
		code        int
		result      string
		contentType string
	}
	tests := []struct {
		name    string
		body    string
		setup   func(*mocks.Shortener)
		want    want
		wantErr bool
	}{
		{
			name: "valid URL returns 201 and JSON with result",
			body: `{"url":"https://practicum.yandex.ru/"}`,
			setup: func(m *mocks.Shortener) {
				m.EXPECT().Shorten("https://practicum.yandex.ru/").Return("http://localhost:8080/EwHXdJfB", nil)
			},
			want: want{
				code:        http.StatusCreated,
				result:      "http://localhost:8080/EwHXdJfB",
				contentType: "application/json",
			},
		},
		{
			name: "empty JSON body returns 400",
			body: "",
			want: want{code: http.StatusBadRequest},
		},
		{
			name: "invalid JSON returns 400",
			body: `not json`,
			want: want{code: http.StatusBadRequest},
		},
		{
			name: "missing url field returns 400",
			body: `{"other":"value"}`,
			want: want{code: http.StatusBadRequest},
		},
		{
			name: "empty url value returns 400",
			body: `{"url":""}`,
			want: want{code: http.StatusBadRequest},
		},
		{
			name: "service error returns 500",
			body: `{"url":"https://example.com"}`,
			setup: func(m *mocks.Shortener) {
				m.EXPECT().Shorten("https://example.com").Return("", io.ErrUnexpectedEOF)
			},
			want: want{code: http.StatusInternalServerError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewShortener(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			newHandler(svc).ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.code, res.StatusCode)

			if tt.want.result != "" {
				var resp struct {
					Result string `json:"result"`
				}
				err := json.NewDecoder(res.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, tt.want.result, resp.Result)
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

			newHandler(mocks.NewShortener(t)).ServeHTTP(w, req)

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
		name  string
		path  string
		setup func(*mocks.Shortener)
		want  want
	}{
		{
			name: "known ID returns 307 with Location header",
			path: "/EwHXdJfB",
			setup: func(m *mocks.Shortener) {
				m.EXPECT().Resolve("EwHXdJfB").Return("https://practicum.yandex.ru/", nil)
			},
			want: want{
				code:     http.StatusTemporaryRedirect,
				location: "https://practicum.yandex.ru/",
			},
		},
		{
			name: "unknown ID returns 404",
			path: "/unknown",
			setup: func(m *mocks.Shortener) {
				m.EXPECT().Resolve("unknown").Return("", repository.ErrNotFound)
			},
			want: want{code: http.StatusNotFound},
		},
		{
			name: "empty ID (GET /) returns 400",
			path: "/",
			want: want{code: http.StatusBadRequest},
		},
		{
			name: "nested path returns 404",
			path: "/a/b",
			want: want{code: http.StatusNotFound},
		},
		{
			name: "service error returns 500",
			path: "/EwHXdJfB",
			setup: func(m *mocks.Shortener) {
				m.EXPECT().Resolve("EwHXdJfB").Return("", io.ErrUnexpectedEOF)
			},
			want: want{code: http.StatusInternalServerError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewShortener(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			newHandler(svc).ServeHTTP(w, req)

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

			newHandler(mocks.NewShortener(t)).ServeHTTP(w, req)

			assert.Equal(t, tt.want.code, w.Code)
		})
	}
}
