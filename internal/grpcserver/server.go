package grpcserver

import (
	"context"
	"errors"
	neturl "net/url"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/voidrunr/go-url-shortener/api/shortener"
	"github.com/voidrunr/go-url-shortener/internal/audit"
	"github.com/voidrunr/go-url-shortener/internal/auth"
	"github.com/voidrunr/go-url-shortener/internal/model"
	"github.com/voidrunr/go-url-shortener/internal/repository"
)

type Shortener interface {
	Shorten(originalURL, userID string) (string, error)
	Resolve(code string) (string, error)
	ListByUser(ctx context.Context, userID string) ([]model.URL, error)
}

type Server struct {
	pb.UnimplementedShortenerServiceServer

	svc      Shortener
	verifier *auth.Verifier
	baseURL  string
	audit    *audit.Emitter
	logger   zerolog.Logger
}

var _ pb.ShortenerServiceServer = (*Server)(nil)

func New(svc Shortener, verifier *auth.Verifier, baseURL string, emitter *audit.Emitter, logger zerolog.Logger) *Server {
	return &Server{
		svc:      svc,
		verifier: verifier,
		baseURL:  baseURL,
		audit:    emitter,
		logger:   logger,
	}
}

func (s *Server) ShortenURL(ctx context.Context, req *pb.URLShortenRequest) (*pb.URLShortenResponse, error) {
	if req.GetUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "url is required")
	}

	userID, _ := s.userFromContext(ctx)
	originalURL := req.GetUrl()
	shortURL, err := s.svc.Shorten(originalURL, userID)
	if err != nil {
		if errors.Is(err, repository.ErrURLAlreadyExists) {
			s.emit(audit.ActionShorten, userID, originalURL)
			return nil, alreadyExistsError(shortURL)
		}
		return nil, s.internalError("ShortenURL", err)
	}

	s.emit(audit.ActionShorten, userID, originalURL)
	return &pb.URLShortenResponse{Result: shortURL}, nil
}

func (s *Server) ExpandURL(ctx context.Context, req *pb.URLExpandRequest) (*pb.URLExpandResponse, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	originalURL, err := s.svc.Resolve(req.GetId())
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return nil, status.Error(codes.NotFound, "url not found")
		case errors.Is(err, repository.ErrGone):
			return nil, status.Error(codes.FailedPrecondition, "url is deleted")
		default:
			return nil, s.internalError("ExpandURL", err)
		}
	}

	userID, _ := s.userFromContext(ctx)
	s.emit(audit.ActionFollow, userID, originalURL)
	return &pb.URLExpandResponse{Result: originalURL}, nil
}

func (s *Server) ListUserURLs(ctx context.Context, _ *emptypb.Empty) (*pb.UserURLsResponse, error) {
	userID, ok := s.userFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "invalid token subject")
	}

	urls, err := s.svc.ListByUser(ctx, userID)
	if err != nil {
		return nil, s.internalError("ListUserURLs", err)
	}

	resp := &pb.UserURLsResponse{Url: make([]*pb.URLData, 0, len(urls))}
	for _, item := range urls {
		shortURL, err := neturl.JoinPath(s.baseURL, item.Code)
		if err != nil {
			return nil, s.internalError("ListUserURLs", err)
		}
		resp.Url = append(resp.Url, &pb.URLData{
			ShortUrl:    shortURL,
			OriginalUrl: item.Original,
		})
	}
	return resp, nil
}

func (s *Server) internalError(method string, err error) error {
	s.logger.Error().Err(err).Str("method", method).Msg("grpc handler failed")
	return status.Error(codes.Internal, "internal error")
}

func (s *Server) emit(action audit.Action, userID, originalURL string) {
	if s.audit == nil {
		return
	}
	s.audit.Notify(audit.Event{
		Action: action,
		UserID: userID,
		URL:    originalURL,
		TS:     time.Now().Unix(),
	})
}

func alreadyExistsError(shortURL string) error {
	st := status.New(codes.AlreadyExists, "url already exists: "+shortURL)
	withDetails, err := st.WithDetails(&errdetails.ErrorInfo{
		Reason:   "URL_ALREADY_EXISTS",
		Domain:   "shortener",
		Metadata: map[string]string{"result": shortURL},
	})
	if err != nil {
		return st.Err()
	}
	return withDetails.Err()
}
