package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/voidrunr/go-url-shortener/internal/audit"
	"github.com/voidrunr/go-url-shortener/internal/auth"
	"github.com/voidrunr/go-url-shortener/internal/config"
	"github.com/voidrunr/go-url-shortener/internal/database"
	"github.com/voidrunr/go-url-shortener/internal/database/migrations"
	"github.com/voidrunr/go-url-shortener/internal/handler"
	"github.com/voidrunr/go-url-shortener/internal/repository"
	"github.com/voidrunr/go-url-shortener/internal/service"
	"github.com/voidrunr/go-url-shortener/internal/tlscert"
)

var buildVersion string
var buildDate string
var buildCommit string

const shutdownTimeout = 10 * time.Second

func main() {
	fmt.Printf("Build version: %s\n", orNA(buildVersion))
	fmt.Printf("Build date: %s\n", orNA(buildDate))
	fmt.Printf("Build commit: %s\n", orNA(buildCommit))

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	cfg, err := config.Parse()
	if err != nil {
		log.Fatal().Err(err).Msg("invalid configuration")
	}

	signer, err := auth.NewSigner(cfg.SecretKey, cfg.TokenTTL)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid auth configuration")
	}

	repo, db, err := buildRepository(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create repository")
	}

	svc := service.New(repo, cfg.BaseURL, cfg.CollisionRetries)

	emitter := audit.NewEmitter()
	if cfg.AuditFile != "" {
		emitter.Subscribe(audit.NewFileSink(cfg.AuditFile))
	}
	if cfg.AuditURL != "" {
		emitter.Subscribe(audit.NewHTTPSink(cfg.AuditURL))
	}

	opts := []handler.Option{
		handler.WithAuth(signer),
		handler.WithBaseURL(cfg.BaseURL),
		handler.WithLogger(log.Logger),
		handler.WithAudit(emitter),
	}
	if cfg.TrustedSubnet != "" {
		trustedSubnet, err := netip.ParsePrefix(cfg.TrustedSubnet)
		if err != nil {
			log.Fatal().Err(err).Msg("invalid trusted subnet configuration")
		}
		opts = append(opts, handler.WithTrustedSubnet(trustedSubnet))
	}
	if db != nil {
		opts = append(opts, handler.WithPinger(db))
	}
	hlr := handler.New(svc, opts...)

	var pprofSrv *http.Server
	if cfg.PprofAddress != "" {
		pprofSrv = &http.Server{Addr: cfg.PprofAddress}
		log.Info().Str("address", cfg.PprofAddress).Msg("pprof server")
		go func() {
			if listenErr := pprofSrv.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
				log.Error().Err(listenErr).Msg("pprof server error")
			}
		}()
	}

	srv := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: hlr.Router(),
	}
	if cfg.EnableHTTPS {
		cert, err := tlscert.SelfSignedCertificate()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to generate self-signed certificate")
		}
		srv.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	}

	serveErr := make(chan error, 1)
	go func() {
		if cfg.EnableHTTPS {
			log.Info().Str("address", cfg.ServerAddress).Msg("serving https")
			serveErr <- srv.ListenAndServeTLS("", "")
			return
		}
		serveErr <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	select {
	case sig := <-quit:
		log.Info().Str("signal", sig.String()).Msg("shutting down")
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
		log.Fatal().Msg("server stopped unexpectedly")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown timed out, forcing close")
		_ = srv.Close()
	}
	if pprofSrv != nil {
		_ = pprofSrv.Shutdown(context.Background())
	}
	if err := <-serveErr; err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("server error")
	}

	svc.Close()
	if closeErr := emitter.Close(); closeErr != nil {
		log.Error().Err(closeErr).Msg("failed to close audit emitter")
	}
	if db != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close database")
		}
	}

	log.Info().Msg("serving stopped")
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

func orNA(v string) string {
	if v == "" {
		return "N/A"
	}
	return v
}
