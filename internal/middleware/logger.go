package middleware

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/voidrunr/go-url-shortener/internal/pool"
)

// loggerPool recycles responseWriter wrappers, which are allocated
// once per request with no other purpose.
var loggerPool = pool.New(func() *responseWriter {
	return &responseWriter{statusCode: http.StatusOK}
})

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) Reset() {
	rw.ResponseWriter = nil
	rw.statusCode = http.StatusOK
	rw.size = 0
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := loggerPool.Get()
		rw.ResponseWriter = w
		defer loggerPool.Put(rw)

		next.ServeHTTP(rw, r)

		log.Info().
			Str("uri", r.RequestURI).
			Str("method", r.Method).
			Dur("duration", time.Since(start)).
			Int("status", rw.statusCode).
			Int("size", rw.size).
			Msg("request")
	})
}
