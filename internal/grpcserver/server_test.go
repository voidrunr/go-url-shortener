package grpcserver_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/voidrunr/go-url-shortener/api/shortener"
	"github.com/voidrunr/go-url-shortener/internal/auth"
	"github.com/voidrunr/go-url-shortener/internal/grpcserver"
	"github.com/voidrunr/go-url-shortener/internal/model"
	"github.com/voidrunr/go-url-shortener/internal/repository"
)

const (
	testSecret = "test-secret"
	testBase   = "https://sho.rt"
)

type stubShortener struct {
	shortenFn func(string, string) (string, error)
	resolveFn func(string) (string, error)
	listFn    func(context.Context, string) ([]model.URL, error)
}

func (s *stubShortener) Shorten(originalURL, userID string) (string, error) {
	return s.shortenFn(originalURL, userID)
}

func (s *stubShortener) Resolve(code string) (string, error) {
	return s.resolveFn(code)
}

func (s *stubShortener) ListByUser(ctx context.Context, userID string) ([]model.URL, error) {
	return s.listFn(ctx, userID)
}

func newClient(t *testing.T, svc grpcserver.Shortener) pb.ShortenerServiceClient {
	t.Helper()

	verifier, err := auth.NewVerifier(testSecret)
	require.NoError(t, err)
	logger := zerolog.Nop()

	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		grpcserver.UnaryLogger(logger),
		grpcserver.UnaryRecovery(logger),
	))
	pb.RegisterShortenerServiceServer(grpcServer, grpcserver.New(svc, verifier, testBase, nil, logger))

	listener := bufconn.Listen(1024 * 1024)
	go func() {
		_ = grpcServer.Serve(listener)
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = conn.Close()
		grpcServer.Stop()
		_ = listener.Close()
	})

	return pb.NewShortenerServiceClient(conn)
}

func testToken(t *testing.T, userID string) string {
	t.Helper()

	signer, err := auth.NewSigner(testSecret, time.Hour)
	require.NoError(t, err)
	cookie, err := signer.Cookie(userID)
	require.NoError(t, err)
	return cookie.Value
}

func withToken(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", token)
}

func TestShortenURLAuthForms(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{name: "raw token"},
		{name: "bearer token", prefix: "Bearer "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotUserID string
			svc := &stubShortener{
				shortenFn: func(_ string, userID string) (string, error) {
					gotUserID = userID
					return testBase + "/abc123", nil
				},
			}
			client := newClient(t, svc)
			ctx := withToken(context.Background(), tc.prefix+testToken(t, "user-1"))

			resp, err := client.ShortenURL(ctx, &pb.URLShortenRequest{Url: "https://example.com"})

			require.NoError(t, err)
			require.Equal(t, testBase+"/abc123", resp.GetResult())
			require.Equal(t, "user-1", gotUserID)
		})
	}
}

func TestShortenURLGeneratesUserIDWithoutValidToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{name: "missing token"},
		{name: "invalid token", token: "garbage"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotUserID string
			svc := &stubShortener{
				shortenFn: func(_ string, userID string) (string, error) {
					gotUserID = userID
					return testBase + "/abc123", nil
				},
			}
			client := newClient(t, svc)
			ctx := context.Background()
			if tc.token != "" {
				ctx = withToken(ctx, tc.token)
			}

			_, err := client.ShortenURL(ctx, &pb.URLShortenRequest{Url: "https://example.com"})

			require.NoError(t, err)
			require.NotEmpty(t, gotUserID)
		})
	}
}

func TestShortenURLValidation(t *testing.T) {
	client := newClient(t, &stubShortener{})

	_, err := client.ShortenURL(context.Background(), &pb.URLShortenRequest{})

	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestShortenURLAlreadyExists(t *testing.T) {
	shortURL := testBase + "/abc123"
	tests := []struct {
		name string
		err  error
	}{
		{name: "sentinel", err: repository.ErrURLAlreadyExists},
		{name: "typed", err: &repository.DuplicateURLError{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &stubShortener{
				shortenFn: func(_ string, _ string) (string, error) {
					return shortURL, tc.err
				},
			}
			client := newClient(t, svc)

			_, err := client.ShortenURL(context.Background(), &pb.URLShortenRequest{Url: "https://example.com"})

			st, ok := status.FromError(err)
			require.True(t, ok)
			require.Equal(t, codes.AlreadyExists, st.Code())
			require.Contains(t, st.Message(), shortURL)
			require.Len(t, st.Details(), 1)
			info, ok := st.Details()[0].(*errdetails.ErrorInfo)
			require.True(t, ok)
			require.Equal(t, shortURL, info.GetMetadata()["result"])
		})
	}
}

func TestExpandURL(t *testing.T) {
	svc := &stubShortener{
		resolveFn: func(code string) (string, error) {
			if code != "abc123" {
				return "", repository.ErrNotFound
			}
			return "https://example.com", nil
		},
	}
	client := newClient(t, svc)

	resp, err := client.ExpandURL(context.Background(), &pb.URLExpandRequest{Id: "abc123"})

	require.NoError(t, err)
	require.Equal(t, "https://example.com", resp.GetResult())
}

func TestExpandURLErrors(t *testing.T) {
	tests := []struct {
		name string
		id   string
		err  error
		code codes.Code
	}{
		{name: "empty id", code: codes.InvalidArgument},
		{name: "not found", id: "abc123", err: repository.ErrNotFound, code: codes.NotFound},
		{name: "deleted", id: "abc123", err: repository.ErrGone, code: codes.FailedPrecondition},
		{name: "internal", id: "abc123", err: errors.New("boom"), code: codes.Internal},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &stubShortener{
				resolveFn: func(_ string) (string, error) {
					return "", tc.err
				},
			}
			client := newClient(t, svc)

			_, err := client.ExpandURL(context.Background(), &pb.URLExpandRequest{Id: tc.id})

			require.Equal(t, tc.code, status.Code(err))
		})
	}
}

func TestListUserURLs(t *testing.T) {
	var gotUserID string
	svc := &stubShortener{
		listFn: func(_ context.Context, userID string) ([]model.URL, error) {
			gotUserID = userID
			return []model.URL{
				{Code: "abc123", Original: "https://example.com"},
				{Code: "def456", Original: "https://example.org"},
			}, nil
		},
	}
	client := newClient(t, svc)

	resp, err := client.ListUserURLs(withToken(context.Background(), testToken(t, "user-1")), &emptypb.Empty{})

	require.NoError(t, err)
	require.Equal(t, "user-1", gotUserID)
	require.Len(t, resp.GetUrl(), 2)
	require.Equal(t, testBase+"/abc123", resp.GetUrl()[0].GetShortUrl())
	require.Equal(t, "https://example.com", resp.GetUrl()[0].GetOriginalUrl())
	require.Equal(t, testBase+"/def456", resp.GetUrl()[1].GetShortUrl())
}

func TestListUserURLsAuth(t *testing.T) {
	t.Run("empty subject", func(t *testing.T) {
		client := newClient(t, &stubShortener{})

		_, err := client.ListUserURLs(withToken(context.Background(), testToken(t, "")), &emptypb.Empty{})

		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("missing token generates user", func(t *testing.T) {
		var gotUserID string
		svc := &stubShortener{
			listFn: func(_ context.Context, userID string) ([]model.URL, error) {
				gotUserID = userID
				return nil, nil
			},
		}
		client := newClient(t, svc)

		resp, err := client.ListUserURLs(context.Background(), &emptypb.Empty{})

		require.NoError(t, err)
		require.Empty(t, resp.GetUrl())
		require.NotEmpty(t, gotUserID)
	})
}

func TestListUserURLsInternalError(t *testing.T) {
	svc := &stubShortener{
		listFn: func(_ context.Context, _ string) ([]model.URL, error) {
			return nil, errors.New("boom")
		},
	}
	client := newClient(t, svc)

	_, err := client.ListUserURLs(context.Background(), &emptypb.Empty{})

	require.Equal(t, codes.Internal, status.Code(err))
}

func TestRecoveryInterceptor(t *testing.T) {
	svc := &stubShortener{
		shortenFn: func(_ string, _ string) (string, error) {
			panic("boom")
		},
	}
	client := newClient(t, svc)

	_, err := client.ShortenURL(context.Background(), &pb.URLShortenRequest{Url: "https://example.com"})

	require.Equal(t, codes.Internal, status.Code(err))
}
