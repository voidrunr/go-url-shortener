package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	a := New("test-secret")

	c := a.Cookie("user-123")
	require.NotEmpty(t, c.Value)

	id, err := a.verify(c.Value)
	require.NoError(t, err)
	assert.Equal(t, "user-123", id)
}

func TestVerifyRejectsTamperedCookie(t *testing.T) {
	a := New("test-secret")

	c := a.Cookie("user-123")
	tampered := c.Value[:len(c.Value)-4] + "beef"

	_, err := a.verify(tampered)
	assert.Error(t, err)
}

func TestVerifyRejectsMalformedCookie(t *testing.T) {
	a := New("test-secret")

	_, err := a.verify("no-dot-separator")
	assert.Error(t, err)
	_, err = a.verify("")
	assert.Error(t, err)
}

func TestVerifyRejectsSignatureFromDifferentSecret(t *testing.T) {
	a := New("secret-a")
	b := New("secret-b")

	_, err := b.verify(a.Cookie("user-123").Value)
	assert.Error(t, err)
}

func TestResolveNoCookieCreatesNewUser(t *testing.T) {
	a := New("test-secret")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	id, state := a.Resolve(req)
	require.Equal(t, StateNew, state)
	assert.NotEmpty(t, id)

	_, err := a.verify(a.Cookie(id).Value)
	require.NoError(t, err)
}

func TestResolveValidCookie(t *testing.T) {
	a := New("test-secret")

	c := a.Cookie("user-456")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)

	id, state := a.Resolve(req)
	assert.Equal(t, StateOK, state)
	assert.Equal(t, "user-456", id)
}

func TestResolveInvalidCookieCreatesNewUser(t *testing.T) {
	a := New("test-secret")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "garbage"})

	id, state := a.Resolve(req)
	assert.Equal(t, StateNew, state)
	assert.NotEmpty(t, id)
}

func TestResolveCookieWithoutUserIDIsUnauthorized(t *testing.T) {
	a := New("test-secret")

	c := a.Cookie("")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)

	id, state := a.Resolve(req)
	assert.Equal(t, StateUnauthorized, state)
	assert.Empty(t, id)
}

func TestMiddlewareSetsCookieForNewUser(t *testing.T) {
	a := New("test-secret")

	var gotUserID string
	var gotNoUserID bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
		gotNoUserID = HasNoUserID(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	a.Handler(next).ServeHTTP(w, req)

	res := w.Result()
	cookies := res.Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, CookieName, cookies[0].Name)
	assert.False(t, gotNoUserID)
	assert.NotEmpty(t, gotUserID)
}

func TestMiddlewarePropagatesExistingUser(t *testing.T) {
	a := New("test-secret")

	var gotUserID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(a.Cookie("user-789"))
	w := httptest.NewRecorder()

	a.Handler(next).ServeHTTP(w, req)

	res := w.Result()
	assert.Empty(t, res.Cookies())
	assert.Equal(t, "user-789", gotUserID)
}

func TestMiddlewareFlagsCookieWithoutUserID(t *testing.T) {
	a := New("test-secret")

	var gotUserID string
	var gotNoUserID bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
		gotNoUserID = HasNoUserID(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(a.Cookie(""))
	w := httptest.NewRecorder()

	a.Handler(next).ServeHTTP(w, req)

	assert.True(t, gotNoUserID)
	assert.Empty(t, gotUserID)
}
