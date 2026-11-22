package grpcserver

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func UnaryLogger(logger zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := status.Code(err)

		event := logger.Info()
		if code != codes.OK {
			event = logger.Warn()
		}
		event.
			Str("method", info.FullMethod).
			Dur("duration", time.Since(start)).
			Str("code", code.String()).
			Msg("grpc request")

		return resp, err
	}
}

func UnaryRecovery(logger zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error().
					Str("method", info.FullMethod).
					Interface("panic", recovered).
					Msg("grpc panic recovered")
				resp = nil
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}
