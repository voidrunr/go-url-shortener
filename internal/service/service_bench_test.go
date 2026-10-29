package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/voidrunr/go-url-shortener/internal/model"
	"github.com/voidrunr/go-url-shortener/internal/repository"
)

func benchService(b *testing.B) *URLService {
	return New(repository.NewMemory(), "http://localhost:8080", 5)
}

func benchResolveRepo(b *testing.B) (*URLService, string) {
	repo := repository.NewMemory()
	svc := New(repo, "http://localhost:8080", 5)

	shortURL, err := svc.Shorten("https://long.example.com/a/very/long/path", "bench-user")
	if err != nil {
		b.Fatal(err)
	}

	return svc, strings.TrimPrefix(shortURL, "http://localhost:8080/")
}

func BenchmarkShorten(b *testing.B) {
	svc := benchService(b)
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.Shorten(fmt.Sprintf("https://example.com/some/long/path/%d", i), "bench-user")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkShortenBatch(b *testing.B) {
	svc := benchService(b)
	items := []model.BatchItem{
		{CorrelationID: "1", OriginalURL: "https://one.example/long/path"},
		{CorrelationID: "2", OriginalURL: "https://two.example/long/path"},
	}
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.ShortenBatch(items, "bench-user"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResolve(b *testing.B) {
	svc, code := benchResolveRepo(b)
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Resolve(code); err != nil {
			b.Fatal(err)
		}
	}
}
