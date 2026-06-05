package handler

import (
	"errors"
	"io"
	"net/http"
	"strings"

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

func (hlr *URLHandler) Router() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", hlr.route)

	return mux
}

func (hlr *URLHandler) route(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if r.URL.Path != "/" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		hlr.handleShorten(w, r)
	case http.MethodGet:
		code := strings.TrimPrefix(r.URL.Path, "/")
		if code == "" || strings.Contains(code, "/") {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		hlr.handleResolve(w, r, code)

	default:
		http.Error(w, "Bad Request", http.StatusBadRequest)
	}
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

func (hlr *URLHandler) handleResolve(w http.ResponseWriter, r *http.Request, code string) {
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
