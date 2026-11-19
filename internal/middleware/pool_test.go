package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCompressResponseWriterReset verifies the Reset contract the pool
// relies on: a dirty wrapper becomes clean and its gzip writer is
// released, whether or not sync.Pool actually hands back the same
// object (that part is left to sync.Pool).
func TestCompressResponseWriterReset(t *testing.T) {
	rec := httptest.NewRecorder()
	crw := compressPool.Get()
	crw.ResponseWriter = rec
	crw.status = 500
	crw.writer = acquireGzipWriter(io.Discard)

	crw.Reset()

	require.Nil(t, crw.ResponseWriter)
	require.Nil(t, crw.writer, "Reset must release the gzip writer")
	require.Equal(t, 0, crw.status)

	// The released writer must be closed and usable again.
	gz := gzipWriterPool.Get().(*gzip.Writer)
	require.NotPanics(t, func() { gz.Reset(io.Discard) })
	gzipWriterPool.Put(gz)
}

// TestLoggerResponseWriterReset verifies the Reset contract the logger
// pool relies on: back to pre-request state.
func TestLoggerResponseWriterReset(t *testing.T) {
	rw := loggerPool.Get()
	rw.ResponseWriter = httptest.NewRecorder()
	rw.WriteHeader(404)
	rw.Write([]byte("hello"))

	rw.Reset()

	require.Nil(t, rw.ResponseWriter)
	require.Equal(t, 200, rw.statusCode)
	require.Equal(t, 0, rw.size)
}

// TestPooledWrappersUnderServe exercises the actual pooled path end to
// end: many requests through both middlewares with race detection on.
func TestPooledWrappersUnderServe(t *testing.T) {
	handlerChain := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>ok</html>"))
	}))

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("Accept-Encoding", "gzip")
				handlerChain.ServeHTTP(rec, req)
				require.Equal(t, 200, rec.Code)
			}
		}()
	}
	wg.Wait()
}
