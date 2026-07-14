package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type compressResponseWriter struct {
	http.ResponseWriter
	writer *gzip.Writer
}

func (crw *compressResponseWriter) gzip() bool {
	ct := crw.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") && !strings.Contains(ct, "text/html") {
		return false
	}
	crw.writer = gzip.NewWriter(crw.ResponseWriter)
	crw.Header().Del("Content-Length")
	crw.Header().Set("Content-Encoding", "gzip")
	return true
}

func (crw *compressResponseWriter) Write(p []byte) (int, error) {
	if crw.writer == nil {
		crw.gzip()
	}
	if crw.writer != nil {
		return crw.writer.Write(p)
	}
	return crw.ResponseWriter.Write(p)
}

func (crw *compressResponseWriter) WriteHeader(code int) {
	crw.gzip()
	crw.ResponseWriter.WriteHeader(code)
}

func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gzReader, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
			defer gzReader.Close()
			r.Body = io.NopCloser(gzReader)
		}

		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			crw := &compressResponseWriter{ResponseWriter: w}
			defer func() {
				if crw.writer != nil {
					crw.writer.Close()
				}
			}()
			next.ServeHTTP(crw, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
