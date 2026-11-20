package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"

	pb "github.com/voidrunr/go-url-shortener/api/shortener"
	"github.com/voidrunr/go-url-shortener/internal/audit"
	"github.com/voidrunr/go-url-shortener/internal/auth"
	"github.com/voidrunr/go-url-shortener/internal/config"
	"github.com/voidrunr/go-url-shortener/internal/database"
	"github.com/voidrunr/go-url-shortener/internal/database/migrations"
	"github.com/voidrunr/go-url-shortener/internal/grpcserver"
	"github.com/voidrunr/go-url-shortener/internal/handler"
	"github.com/voidrunr/go-url-shortener/internal/repository"
	"github.com/voidrunr/go-url-shortener/internal/service"
	"github.com/voidrunr/go-url-shortener/internal/tlscert"
)

var buildVersion = "N/A"
var buildDate = "N/A"
var buildCommit = "N/A"

const (
	shutdownTimeout   = 10 * time.Second
	readHeaderTimeout = 5 * time.Second
	idleTimeout       = 60 * time.Second
)

func main() {
	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	cfg, err := config.Parse()
	if err != nil {
		log.Fatal().Err(err).Msg("invalid configuration")
	}

	signer, err := auth.NewSigner(cfg.SecretKey, cfg.TokenTTL)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid auth configuration")
	}
	verifier, err := auth.NewVerifier(cfg.SecretKey)
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
		pprofSrv = &http.Server{
			Addr:              cfg.PprofAddress,
			ReadHeaderTimeout: readHeaderTimeout,
			IdleTimeout:       idleTimeout,
		}
		log.Info().Str("address", cfg.PprofAddress).Msg("pprof server")
		go func() {
			if listenErr := pprofSrv.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
				log.Error().Err(listenErr).Msg("pprof server error")
			}
		}()
	}

	srv := &http.Server{
		Addr:              cfg.ServerAddress,
		Handler:           hlr.Router(),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}

	var tlsConfig *tls.Config
	if cfg.EnableHTTPS {
		cert, err := tlscert.SelfSignedCertificate()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to generate self-signed certificate")
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"h2", "http/1.1"},
		}
	}

	listener, err := net.Listen("tcp", cfg.ServerAddress)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen")
	}
	var serveListener net.Listener = listener
	if tlsConfig != nil {
		serveListener = tls.NewListener(listener, tlsConfig)
	}

	mux := cmux.New(serveListener)
	grpcListener := mux.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
	httpListener := mux.Match(cmux.HTTP1Fast(), cmux.HTTP2())

	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		grpcserver.UnaryLogger(log.Logger),
		grpcserver.UnaryRecovery(log.Logger),
	))
	pb.RegisterShortenerServiceServer(grpcServer, grpcserver.New(svc, verifier, cfg.BaseURL, emitter, log.Logger))

	serveErr := make(chan error, 3)
	go func() {
		if cfg.EnableHTTPS {
			log.Info().Str("address", cfg.ServerAddress).Msg("serving https")
		} else {
			log.Info().Str("address", cfg.ServerAddress).Msg("serving http")
		}
		serveErr <- mux.Serve()
	}()
	go func() {
		serveErr <- srv.Serve(httpListener)
	}()
	go func() {
		serveErr <- grpcServer.Serve(grpcListener)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	shutdown := make(chan os.Signal, 1)
	go func() {
		shutdown <- <-quit
		forceExit(<-quit)
	}()

	received := 0
	var serveFailure error
	select {
	case sig := <-shutdown:
		log.Info().Str("signal", sig.String()).Msg("shutting down")
	case err := <-serveErr:
		received = 1
		if !expectedServeError(err) {
			serveFailure = err
		}
	}

	mux.Close()
	_ = listener.Close()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown timed out, forcing close")
		_ = srv.Close()
	}
	grpcServer.GracefulStop()
	for i := 0; i < 3-received; i++ {
		if err := <-serveErr; !expectedServeError(err) {
			log.Error().Err(err).Msg("server error")
		}
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
	if pprofSrv != nil {
		pprofCtx, cancelPprof := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancelPprof()
		if err := pprofSrv.Shutdown(pprofCtx); err != nil {
			log.Error().Err(err).Msg("pprof server shutdown timed out, forcing close")
			_ = pprofSrv.Close()
		}
	}
	if serveFailure != nil {
		log.Fatal().Err(serveFailure).Msg("server error")
	}

	log.Info().Msg("serving stopped")
}

func expectedServeError(err error) bool {
	return err == nil ||
		errors.Is(err, http.ErrServerClosed) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, cmux.ErrServerClosed) ||
		errors.Is(err, cmux.ErrListenerClosed) ||
		errors.Is(err, grpc.ErrServerStopped)
}

func forceExit(sig os.Signal) {
	log.Warn().Str("signal", sig.String()).Msg("second signal, forcing exit")
	signum, ok := sig.(syscall.Signal)
	if !ok {
		signum = syscall.SIGTERM
	}
	os.Exit(128 + int(signum))
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
