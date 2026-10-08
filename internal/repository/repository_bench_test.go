package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/voidrunr/go-url-shortener/internal/model"
)

func BenchmarkMemoryRepositoryWrite(b *testing.B) {
	repo := NewMemory()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		url := model.URL{
			UUID:      "uuid",
			Original:  fmt.Sprintf("https://example.com/%d", i),
			Code:      fmt.Sprintf("code-%d", i),
			UserID:    "user",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := repo.Write(ctx, url); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryRepositoryGet(b *testing.B) {
	repo := NewMemory()
	ctx := context.Background()
	if err := repo.Write(ctx, testURL("abc123", "https://example.com")); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := repo.Get(ctx, "abc123"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFileRepositoryWrite(b *testing.B) {
	path := filepath.Join(b.TempDir(), "data.json")
	repo, err := NewFile(path)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		url := model.URL{
			UUID:      "uuid",
			Original:  fmt.Sprintf("https://example.com/%d", i),
			Code:      fmt.Sprintf("code-%d", i),
			UserID:    "user",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := repo.Write(ctx, url); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFileRepositoryWriteBatch(b *testing.B) {
	path := filepath.Join(b.TempDir(), "data.json")
	repo, err := NewFile(path)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		urls := []model.URL{
			{UUID: "uuid", Original: fmt.Sprintf("https://one.example/%d", i), Code: fmt.Sprintf("one-%d", i), UserID: "user"},
			{UUID: "uuid", Original: fmt.Sprintf("https://two.example/%d", i), Code: fmt.Sprintf("two-%d", i), UserID: "user"},
		}
		if err := repo.WriteBatch(ctx, urls); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFileRepositoryRead(b *testing.B) {
	path := filepath.Join(b.TempDir(), "data.json")
	repo, err := NewFile(path)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		if err := repo.Write(ctx, model.URL{
			UUID:      "uuid",
			Original:  fmt.Sprintf("https://example.com/%d", i),
			Code:      fmt.Sprintf("code-%d", i),
			UserID:    "user",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}); err != nil {
			b.Fatal(err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reopened, err := NewFile(path)
		if err != nil {
			b.Fatal(err)
		}
		_ = reopened
	}
}
