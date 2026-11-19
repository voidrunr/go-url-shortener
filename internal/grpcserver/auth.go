package grpcserver

import (
	"context"
	"strings"

	"google.golang.org/grpc/metadata"

	"github.com/voidrunr/go-url-shortener/internal/auth"
)

func (s *Server) userFromContext(ctx context.Context) (string, bool) {
	var token string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get("authorization"); len(values) > 0 {
			token = strings.TrimSpace(values[0])
		}
	}

	if len(token) >= 7 && strings.EqualFold(token[:7], "Bearer ") {
		token = strings.TrimSpace(token[7:])
	}

	if token != "" && s.verifier != nil {
		userID, err := s.verifier.VerifyToken(token)
		if err == nil {
			if userID == "" {
				return "", false
			}
			return userID, true
		}
	}

	userID, err := auth.NewUserID()
	if err != nil {
		return "", false
	}
	return userID, true
}
