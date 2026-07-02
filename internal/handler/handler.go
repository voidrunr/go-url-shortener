package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/voidrunr/go-url-shortener/internal/middleware"
	"github.com/voidrunr/go-url-shortener/internal/repository"
)

type Shortener interface {
	Shorten(string) (string, error)
	Resolve(string) (string, error)
}

type URLHandler struct {
	svc Shortener
}

func New(svc Shortener) *URLHandler {
	return &URLHandler{
		svc: svc,
	}
}

func (hlr *URLHandler) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logging)
	r.Use(middleware.Gzip)
	r.Post("/", hlr.handleShorten)
	r.Post("/api/shorten", hlr.handleAPIShorten)
	r.Get("/", hlr.handleEmptyCode)
	r.Get("/{code}", hlr.handleResolve)
	return r
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
	shortURL, err := hlr.svc.Shorten(originalURL)
	if err != nil {
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

func (hlr *URLHandler) handleAPIShorten(w http.ResponseWriter, r *http.Request) {
	var req shortenRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil || req.URL == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	shortURL, err := hlr.svc.Shorten(req.URL)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	resp := shortenResponse{Result: shortURL}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (hlr *URLHandler) handleResolve(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	originalURL, err := hlr.svc.Resolve(code)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, originalURL, http.StatusTemporaryRedirect)
}
