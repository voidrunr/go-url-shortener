package handler

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

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
	r.Post("/", hlr.handleShorten)
	r.Get("/", hlr.handleEmptyCode)
	r.Get("/{code}", hlr.handleResolve)
	return r
}

func (hlr *URLHandler) handleEmptyCode(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Bad Request", http.StatusBadRequest)
}

func (hlr *URLHandler) handleShorten(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(strings.TrimSpace(string(body))) == 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	originalURL := strings.TrimSpace(string(body))
	shortURL, err := hlr.svc.Shorten(originalURL)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

func (hlr *URLHandler) handleResolve(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	originalUrl, err := hlr.svc.Resolve(code)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			http.Error(w, "Not Found", http.StatusBadRequest)
			return
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, originalUrl, http.StatusTemporaryRedirect)
}
