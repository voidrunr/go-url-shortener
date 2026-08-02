package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/voidrunr/go-url-shortener/internal/auth"
	"github.com/voidrunr/go-url-shortener/internal/handler"
	"github.com/voidrunr/go-url-shortener/internal/handler/mocks"
	"github.com/voidrunr/go-url-shortener/internal/model"
	"github.com/voidrunr/go-url-shortener/internal/repository"
)

func newHandler(svc handler.Shortener) http.Handler {
	return handler.New(svc).Router()
}

func newHandlerWithPinger(svc handler.Shortener, p handler.Pinger) http.Handler {
	return handler.New(svc, handler.WithPinger(p)).Router()
}

func newHandlerWithAuth(svc handler.Shortener, a *auth.Auth) http.Handler {
	return handler.New(svc, handler.WithAuth(a), handler.WithBaseURL("http://localhost:8080")).Router()
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
				m.EXPECT().Shorten("https://practicum.yandex.ru/", "").Return("http://localhost:8080/EwHXdJfB", nil)
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
				m.EXPECT().Shorten("https://example.com", "").Return("", io.ErrUnexpectedEOF)
			},
			want: want{code: http.StatusInternalServerError},
		},
		{
			name: "duplicate URL returns 409 with existing short URL",
			body: "https://practicum.yandex.ru/",
			setup: func(m *mocks.Shortener) {
				m.EXPECT().Shorten("https://practicum.yandex.ru/", "").Return("http://localhost:8080/EwHXdJfB", repository.ErrURLAlreadyExists)
			},
			want: want{
				code:        http.StatusConflict,
				body:        "http://localhost:8080/EwHXdJfB",
				contentType: "text/plain",
			},
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
				m.EXPECT().Shorten("https://practicum.yandex.ru/", "").Return("http://localhost:8080/EwHXdJfB", nil)
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
				m.EXPECT().Shorten("https://example.com", "").Return("", io.ErrUnexpectedEOF)
			},
			want: want{code: http.StatusInternalServerError},
		},
		{
			name: "duplicate URL returns 409 with existing short URL",
			body: `{"url":"https://practicum.yandex.ru/"}`,
			setup: func(m *mocks.Shortener) {
				m.EXPECT().Shorten("https://practicum.yandex.ru/", "").Return("http://localhost:8080/EwHXdJfB", repository.ErrURLAlreadyExists)
			},
			want: want{
				code:        http.StatusConflict,
				result:      "http://localhost:8080/EwHXdJfB",
				contentType: "application/json",
			},
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

// --- POST /api/shorten/batch ---

func TestHandleAPIShortenBatch(t *testing.T) {
	type want struct {
		code        int
		contentType string
	}
	tests := []struct {
		name  string
		body  string
		setup func(*mocks.Shortener)
		want  want
	}{
		{
			name: "valid batch returns 201 and JSON results",
			body: `[{"correlation_id":"1","original_url":"https://one.example"},{"correlation_id":"2","original_url":"https://two.example"}]`,
			setup: func(m *mocks.Shortener) {
				m.EXPECT().ShortenBatch([]model.BatchItem{
					{CorrelationID: "1", OriginalURL: "https://one.example"},
					{CorrelationID: "2", OriginalURL: "https://two.example"},
				}, "").Return([]model.BatchItem{
					{CorrelationID: "1", ShortURL: "http://localhost:8080/a"},
					{CorrelationID: "2", ShortURL: "http://localhost:8080/b"},
				}, nil)
			},
			want: want{code: http.StatusCreated, contentType: "application/json"},
		},
		{
			name: "empty batch returns 400",
			body: `[]`,
			want: want{code: http.StatusBadRequest},
		},
		{
			name: "invalid JSON returns 400",
			body: `not json`,
			want: want{code: http.StatusBadRequest},
		},
		{
			name: "empty original_url returns 400",
			body: `[{"correlation_id":"1","original_url":""}]`,
			want: want{code: http.StatusBadRequest},
		},
		{
			name: "service error returns 500",
			body: `[{"correlation_id":"1","original_url":"https://one.example"}]`,
			setup: func(m *mocks.Shortener) {
				m.EXPECT().ShortenBatch([]model.BatchItem{
					{CorrelationID: "1", OriginalURL: "https://one.example"},
				}, "").Return(nil, io.ErrUnexpectedEOF)
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

			req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			newHandler(svc).ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.code, res.StatusCode)

			if tt.want.contentType != "" {
				assert.Equal(t, tt.want.contentType, res.Header.Get("Content-Type"))
			}

			if tt.want.code == http.StatusCreated {
				var resp []struct {
					CorrelationID string `json:"correlation_id"`
					ShortURL      string `json:"short_url"`
				}
				err := json.NewDecoder(res.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Len(t, resp, 2)
				assert.Equal(t, "1", resp[0].CorrelationID)
				assert.Equal(t, "http://localhost:8080/a", resp[0].ShortURL)
				assert.Equal(t, "2", resp[1].CorrelationID)
				assert.Equal(t, "http://localhost:8080/b", resp[1].ShortURL)
			}
		})
	}
}

// --- GET /ping ---

type pingerStub struct {
	err error
}

func (p pingerStub) PingContext(context.Context) error {
	return p.err
}

func TestHandlePing(t *testing.T) {
	tests := []struct {
		name   string
		pinger handler.Pinger
		want   int
	}{
		{
			name:   "database reachable returns 200",
			pinger: pingerStub{},
			want:   http.StatusOK,
		},
		{
			name:   "database unreachable returns 500",
			pinger: pingerStub{err: errors.New("connection refused")},
			want:   http.StatusInternalServerError,
		},
		{
			name:   "no database configured returns 500",
			pinger: nil,
			want:   http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			w := httptest.NewRecorder()

			newHandlerWithPinger(mocks.NewShortener(t), tt.pinger).ServeHTTP(w, req)

			assert.Equal(t, tt.want, w.Code)
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

// --- GET /api/user/urls ---

func TestHandleUserURLs(t *testing.T) {
	tests := []struct {
		name        string
		sendCookie  bool
		cookieUser  string
		setup       func(*mocks.Shortener)
		wantCode    int
		wantNoBody  bool
		wantResults []map[string]string
	}{
		{
			name:       "no cookie creates user and returns 204 for empty list",
			wantCode:   http.StatusNoContent,
			sendCookie: false,
			setup: func(m *mocks.Shortener) {
				m.EXPECT().ListByUser(mock.AnythingOfType("string")).Return(nil, nil)
			},
		},
		{
			name:       "cookie without user id returns 401",
			sendCookie: true,
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:       "empty list for existing user returns 204",
			sendCookie: true,
			cookieUser: "user-1",
			setup: func(m *mocks.Shortener) {
				m.EXPECT().ListByUser("user-1").Return(nil, nil)
			},
			wantCode:   http.StatusNoContent,
			wantNoBody: true,
		},
		{
			name:       "user with URLs returns JSON list",
			sendCookie: true,
			cookieUser: "user-1",
			setup: func(m *mocks.Shortener) {
				m.EXPECT().ListByUser("user-1").Return([]model.URL{
					{Code: "abc123", Original: "https://one.example"},
					{Code: "def456", Original: "https://two.example"},
				}, nil)
			},
			wantCode: http.StatusOK,
			wantResults: []map[string]string{
				{"short_url": "http://localhost:8080/abc123", "original_url": "https://one.example"},
				{"short_url": "http://localhost:8080/def456", "original_url": "https://two.example"},
			},
		},
		{
			name:       "service error returns 500",
			sendCookie: true,
			cookieUser: "user-1",
			setup: func(m *mocks.Shortener) {
				m.EXPECT().ListByUser("user-1").Return(nil, io.ErrUnexpectedEOF)
			},
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewShortener(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
			if tt.sendCookie {
				c := auth.New("test-secret").Cookie(tt.cookieUser)
				req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: c.Value})
			}
			w := httptest.NewRecorder()

			newHandlerWithAuth(svc, auth.New("test-secret")).ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.wantCode, res.StatusCode)

			if tt.wantResults != nil {
				var resp []map[string]string
				err := json.NewDecoder(res.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, tt.wantResults, resp)
			}

			if tt.wantNoBody {
				body, err := io.ReadAll(res.Body)
				require.NoError(t, err)
				assert.Empty(t, body)
			}
		})
	}
}

func TestHandleUserURLs_SetsCookieOnFirstVisit(t *testing.T) {
	svc := mocks.NewShortener(t)
	svc.EXPECT().ListByUser(mock.AnythingOfType("string")).Return(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	w := httptest.NewRecorder()

	newHandlerWithAuth(svc, auth.New("test-secret")).ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusNoContent, res.StatusCode)
	assert.Len(t, res.Cookies(), 1)
	assert.Equal(t, auth.CookieName, res.Cookies()[0].Name)
}
