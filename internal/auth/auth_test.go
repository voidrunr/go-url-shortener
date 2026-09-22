package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSignerRejectsEmptySecret(t *testing.T) {
	_, err := NewSigner("", time.Minute)
	assert.ErrorIs(t, err, ErrEmptySecret)
}

func TestNewSignerRejectsNonPositiveTTL(t *testing.T) {
	_, err := NewSigner("test-secret", 0)
	assert.ErrorIs(t, err, ErrInvalidTTL)
	_, err = NewSigner("test-secret", -time.Hour)
	assert.ErrorIs(t, err, ErrInvalidTTL)
}

func TestNewVerifierRejectsEmptySecret(t *testing.T) {
	_, err := NewVerifier("")
	assert.ErrorIs(t, err, ErrEmptySecret)
}

func TestSignVerifyRoundTrip(t *testing.T) {
	s, err := NewSigner("test-secret", time.Hour)
	require.NoError(t, err)
	v, err := NewVerifier("test-secret")
	require.NoError(t, err)

	c, err := s.Cookie("user-123")
	require.NoError(t, err)
	require.NotEmpty(t, c.Value)

	id, err := v.verify(c.Value)
	require.NoError(t, err)
	assert.Equal(t, "user-123", id)
}

func TestVerifyRejectsTamperedCookie(t *testing.T) {
	s, err := NewSigner("test-secret", time.Hour)
	require.NoError(t, err)
	v, err := NewVerifier("test-secret")
	require.NoError(t, err)

	c, err := s.Cookie("user-123")
	require.NoError(t, err)
	tampered := c.Value[:len(c.Value)-4] + "beef"

	_, err = v.verify(tampered)
	assert.Error(t, err)
}

func TestVerifyRejectsMalformedCookie(t *testing.T) {
	v, err := NewVerifier("test-secret")
	require.NoError(t, err)

	_, err = v.verify("no-dot-separator")
	assert.Error(t, err)
	_, err = v.verify("")
	assert.Error(t, err)
}

func TestVerifyRejectsSignatureFromDifferentSecret(t *testing.T) {
	s, err := NewSigner("secret-a", time.Hour)
	require.NoError(t, err)
	v, err := NewVerifier("secret-b")
	require.NoError(t, err)

	c, err := s.Cookie("user-123")
	require.NoError(t, err)
	_, err = v.verify(c.Value)
	assert.Error(t, err)
}

func TestClaimsRoundTrip(t *testing.T) {
	ttl := 90 * time.Minute
	s, err := NewSigner("test-secret", ttl)
	require.NoError(t, err)

	c, err := s.Cookie("user-claims")
	require.NoError(t, err)

	token, err := jwt.Parse(c.Value, func(token *jwt.Token) (any, error) {
		assert.Equal(t, jwt.SigningMethodHS256.Alg(), token.Method.Alg())
		return []byte("test-secret"), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	require.NoError(t, err)

	claims, ok := token.Claims.(jwt.MapClaims)
	require.True(t, ok)
	assert.Equal(t, "user-claims", claims["sub"])
	assert.InDelta(t, ttl.Seconds(), claims["exp"].(float64)-claims["iat"].(float64), 1)
}

func TestResolveNoCookieCreatesNewUser(t *testing.T) {
	v, err := NewVerifier("test-secret")
	require.NoError(t, err)
	s, err := NewSigner("test-secret", time.Hour)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	id, state := v.Resolve(req)
	require.Equal(t, StateNew, state)
	assert.NotEmpty(t, id)

	c, err := s.Cookie(id)
	require.NoError(t, err)
	check, err := NewVerifier("test-secret")
	require.NoError(t, err)
	_, err = check.verify(c.Value)
	require.NoError(t, err)
}

func TestResolveValidCookie(t *testing.T) {
	s, err := NewSigner("test-secret", time.Hour)
	require.NoError(t, err)
	v, err := NewVerifier("test-secret")
	require.NoError(t, err)

	c, err := s.Cookie("user-456")
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)

	id, state := v.Resolve(req)
	assert.Equal(t, StateOK, state)
	assert.Equal(t, "user-456", id)
}

func TestResolveInvalidCookieCreatesNewUser(t *testing.T) {
	v, err := NewVerifier("test-secret")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "garbage"})

	id, state := v.Resolve(req)
	assert.Equal(t, StateNew, state)
	assert.NotEmpty(t, id)
}

func TestResolveCookieWithoutUserIDIsUnauthorized(t *testing.T) {
	s, err := NewSigner("test-secret", time.Hour)
	require.NoError(t, err)
	v, err := NewVerifier("test-secret")
	require.NoError(t, err)

	c, err := s.Cookie("")
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)

	id, state := v.Resolve(req)
	assert.Equal(t, StateUnauthorized, state)
	assert.Empty(t, id)
}

func TestResolveExpiredTokenCreatesNewUser(t *testing.T) {
	v, err := NewVerifier("test-secret")
	require.NoError(t, err)

	now := time.Now()
	expired := tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-old",
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
		},
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expired).SignedString([]byte("test-secret"))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: value})

	id, state := v.Resolve(req)
	assert.Equal(t, StateNew, state)
	assert.NotEmpty(t, id)
}

func TestMiddlewareSetsCookieForNewUser(t *testing.T) {
	s, err := NewSigner("test-secret", time.Hour)
	require.NoError(t, err)

	var gotUserID string
	var gotNoUserID bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
		gotNoUserID = HasNoUserID(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	s.Handler(next).ServeHTTP(w, req)

	res := w.Result()
	cookies := res.Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, CookieName, cookies[0].Name)
	assert.False(t, gotNoUserID)
	assert.NotEmpty(t, gotUserID)
}

func TestMiddlewarePropagatesExistingUser(t *testing.T) {
	s, err := NewSigner("test-secret", time.Hour)
	require.NoError(t, err)

	var gotUserID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
	})

	c, err := s.Cookie("user-789")
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)
	w := httptest.NewRecorder()

	s.Handler(next).ServeHTTP(w, req)

	res := w.Result()
	assert.Empty(t, res.Cookies())
	assert.Equal(t, "user-789", gotUserID)
}

func TestMiddlewareFlagsCookieWithoutUserID(t *testing.T) {
	s, err := NewSigner("test-secret", time.Hour)
	require.NoError(t, err)

	var gotUserID string
	var gotNoUserID bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
		gotNoUserID = HasNoUserID(r.Context())
	})

	c, err := s.Cookie("")
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)
	w := httptest.NewRecorder()

	s.Handler(next).ServeHTTP(w, req)

	assert.True(t, gotNoUserID)
	assert.Empty(t, gotUserID)
}