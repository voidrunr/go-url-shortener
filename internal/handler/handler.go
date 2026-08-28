package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/voidrunr/go-url-shortener/internal/auth"
	"github.com/voidrunr/go-url-shortener/internal/middleware"
	"github.com/voidrunr/go-url-shortener/internal/model"
	"github.com/voidrunr/go-url-shortener/internal/repository"
)

type Shortener interface {
	Shorten(string, string) (string, error)
	ShortenBatch([]model.BatchItem, string) ([]model.BatchItem, error)
	Resolve(string) (string, error)
	ListByUser(string) ([]model.URL, error)
	Delete(string, []string) error
}

type Pinger interface {
	PingContext(context.Context) error
}

type Option func(*URLHandler)

func WithPinger(p Pinger) Option {
	return func(hlr *URLHandler) {
		hlr.pinger = p
	}
}

func WithAuth(a *auth.Auth) Option {
	return func(hlr *URLHandler) {
		hlr.auth = a
	}
}

func WithBaseURL(baseURL string) Option {
	return func(hlr *URLHandler) {
		hlr.baseURL = baseURL
	}
}

func WithLogger(l zerolog.Logger) Option {
	return func(hlr *URLHandler) {
		hlr.logger = l
	}
}

type URLHandler struct {
	svc     Shortener
	pinger  Pinger
	auth    *auth.Auth
	baseURL string
	logger  zerolog.Logger
}

func New(svc Shortener, opts ...Option) *URLHandler {
	hlr := &URLHandler{
		svc:    svc,
		logger: log.Logger,
	}
	for _, opt := range opts {
		opt(hlr)
	}
	return hlr
}

func (hlr *URLHandler) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logging)
	r.Use(middleware.Gzip)
	if hlr.auth != nil {
		r.Use(hlr.auth.Handler)
	}
	r.Post("/", hlr.handleShorten)
	r.Post("/api/shorten", hlr.handleAPIShorten)
	r.Post("/api/shorten/batch", hlr.handleAPIShortenBatch)
	r.Get("/api/user/urls", hlr.handleUserURLs)
	r.Delete("/api/user/urls", hlr.handleDeleteUserURLs)
	r.Get("/", hlr.handleEmptyCode)
	r.Get("/ping", hlr.handlePing)
	r.Get("/{code}", hlr.handleResolve)
	return r
}

func (hlr *URLHandler) handlePing(w http.ResponseWriter, r *http.Request) {
	if hlr.pinger == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := hlr.pinger.PingContext(ctx); err != nil {
		hlr.logger.Error().Err(err).Msg("database ping failed")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (hlr *URLHandler) handleEmptyCode(w http.ResponseWriter, r *http.Request) {
	http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
}

func (hlr *URLHandler) handleShorten(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(strings.TrimSpace(string(body))) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	originalURL := strings.TrimSpace(string(body))
	userID := auth.UserIDFromContext(r.Context())
	shortURL, err := hlr.svc.Shorten(originalURL, userID)
	if err != nil {
		if errors.Is(err, repository.ErrURLAlreadyExists) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(shortURL))
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	Result string `json:"result"`
}

type batchShortenRequest struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type batchShortenResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

func (hlr *URLHandler) handleAPIShorten(w http.ResponseWriter, r *http.Request) {
	var req shortenRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil || req.URL == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	userID := auth.UserIDFromContext(r.Context())
	shortURL, err := hlr.svc.Shorten(req.URL, userID)
	if err != nil {
		if errors.Is(err, repository.ErrURLAlreadyExists) {
			resp := shortenResponse{Result: shortURL}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				hlr.logger.Error().Err(err).Msg("failed to encode response")
			}
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	resp := shortenResponse{Result: shortURL}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		hlr.logger.Error().Err(err).Msg("failed to encode response")
	}
}

func (hlr *URLHandler) handleAPIShortenBatch(w http.ResponseWriter, r *http.Request) {
	var reqs []batchShortenRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&reqs); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if len(reqs) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	items := make([]model.BatchItem, 0, len(reqs))
	for _, req := range reqs {
		if strings.TrimSpace(req.OriginalURL) == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		items = append(items, model.BatchItem{
			CorrelationID: req.CorrelationID,
			OriginalURL:   req.OriginalURL,
		})
	}

	userID := auth.UserIDFromContext(r.Context())
	results, err := hlr.svc.ShortenBatch(items, userID)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	resp := make([]batchShortenResponse, 0, len(results))
	for _, res := range results {
		resp = append(resp, batchShortenResponse{
			CorrelationID: res.CorrelationID,
			ShortURL:      res.ShortURL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		hlr.logger.Error().Err(err).Msg("failed to encode response")
	}
}

func (hlr *URLHandler) handleResolve(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	originalURL, err := hlr.svc.Resolve(code)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		if errors.Is(err, repository.ErrGone) {
			http.Error(w, http.StatusText(http.StatusGone), http.StatusGone)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, originalURL, http.StatusTemporaryRedirect)
}

func (hlr *URLHandler) handleDeleteUserURLs(w http.ResponseWriter, r *http.Request) {
	if auth.HasNoUserID(r.Context()) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var codes []string
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&codes); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	userID := auth.UserIDFromContext(r.Context())
	if err := hlr.svc.Delete(userID, codes); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

type userURLResponse struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

func (hlr *URLHandler) handleUserURLs(w http.ResponseWriter, r *http.Request) {
	if auth.HasNoUserID(r.Context()) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	userID := auth.UserIDFromContext(r.Context())
	urls, err := hlr.svc.ListByUser(userID)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if len(urls) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	resp := make([]userURLResponse, 0, len(urls))
	for _, u := range urls {
		shortURL, err := url.JoinPath(hlr.baseURL, u.Code)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		resp = append(resp, userURLResponse{
			ShortURL:    shortURL,
			OriginalURL: u.Original,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error().Err(err).Msg("failed to encode response")
	}
}
