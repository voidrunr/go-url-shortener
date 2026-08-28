package main

import (
	"database/sql"
	"net/http"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/voidrunr/go-url-shortener/internal/auth"
	"github.com/voidrunr/go-url-shortener/internal/config"
	"github.com/voidrunr/go-url-shortener/internal/database"
	"github.com/voidrunr/go-url-shortener/internal/database/migrations"
	"github.com/voidrunr/go-url-shortener/internal/handler"
	"github.com/voidrunr/go-url-shortener/internal/repository"
	"github.com/voidrunr/go-url-shortener/internal/service"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	cfg := config.Parse()

	repo, db, err := buildRepository(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create repository")
	}
	if db != nil {
		defer db.Close()
	}

	svc := service.New(repo, cfg.BaseURL, cfg.CollisionRetries)

	opts := []handler.Option{
		handler.WithAuth(auth.New(cfg.SecretKey)),
		handler.WithBaseURL(cfg.BaseURL),
		handler.WithLogger(log.Logger),
	}
	if db != nil {
		opts = append(opts, handler.WithPinger(db))
	}
	hlr := handler.New(svc, opts...)

	log.Info().Str("address", cfg.ServerAddress).Msg("serving")

	err = http.ListenAndServe(cfg.ServerAddress, hlr.Router())
	if err != nil {
		log.Fatal().Err(err).Msg("server error")
	}
}

func buildRepository(cfg config.Config) (repository.Repository, *sql.DB, error) {
	switch {
	case cfg.DatabaseDSN != "":
		db, err := database.Open(cfg.DatabaseDSN)
		if err != nil {
			return nil, nil, err
		}

		if err := migrations.Up(db); err != nil {
			db.Close()
			return nil, nil, err
		}

		return repository.NewPostgres(db), db, nil

	case cfg.FileStoragePath != "":
		repo, err := repository.NewFile(cfg.FileStoragePath)
		if err != nil {
			return nil, nil, err
		}
		return repo, nil, nil

	default:
		return repository.NewMemory(), nil, nil
	}
}
