package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
)

const CookieName = "token"

var (
	ErrNoCookie = errors.New("no cookie")
	ErrInvalid  = errors.New("invalid cookie")
)

type contextKey string

const (
	userIDKey   contextKey = "userID"
	noUserIDKey contextKey = "noUserID"
)

// State describes how a request user was resolved.
type State int

const (
	// StateNew means the request had no valid cookie, a fresh user was created
	// and a new cookie must be set in the response.
	StateNew State = iota
	// StateUnauthorized means the request carried a cookie that failed
	// authentication (e.g. it did not contain a user ID).
	StateUnauthorized
	// StateOK means the request carried a valid cookie with a user ID.
	StateOK
)

type Auth struct {
	secret []byte
}

// New creates an Auth signer. An empty secret results in a random per-process
// key.
func New(secret string) *Auth {
	key := []byte(secret)
	if len(key) == 0 {
		key = make([]byte, 32)
		_, _ = rand.Read(key)
	}
	return &Auth{secret: key}
}

// NewUserID generates a new unique user identifier.
func NewUserID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Cookie builds a signed cookie carrying the given user ID.
func (a *Auth) Cookie(userID string) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    a.sign(userID),
		Path:     "/",
		HttpOnly: true,
	}
}

// Handler resolves the request user, sets a new signed cookie when needed and
// stores the result in the request context.
func (a *Auth) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, state := a.Resolve(r)

		ctx := r.Context()
		switch state {
		case StateNew:
			http.SetCookie(w, a.Cookie(userID))
			ctx = context.WithValue(ctx, userIDKey, userID)
		case StateUnauthorized:
			ctx = context.WithValue(ctx, noUserIDKey, true)
			ctx = context.WithValue(ctx, userIDKey, "")
		case StateOK:
			ctx = context.WithValue(ctx, userIDKey, userID)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Resolve returns the user ID for the request and the resolution state.
func (a *Auth) Resolve(r *http.Request) (string, State) {
	c, err := r.Cookie(CookieName)
	if errors.Is(err, http.ErrNoCookie) {
		id, err := NewUserID()
		if err != nil {
			return "", StateUnauthorized
		}
		return id, StateNew
	}

	id, err := a.verify(c.Value)
	if err != nil {
		id, genErr := NewUserID()
		if genErr != nil {
			return "", StateUnauthorized
		}
		return id, StateNew
	}
	if id == "" {
		return "", StateUnauthorized
	}

	return id, StateOK
}

// UserIDFromContext returns the user ID stored by the auth handler, if any.
func UserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}

// HasNoUserID reports whether the request carried a cookie that did not
// contain a user ID.
func HasNoUserID(ctx context.Context) bool {
	v, _ := ctx.Value(noUserIDKey).(bool)
	return v
}

func (a *Auth) sign(userID string) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(userID))

	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))

	return payload + "." + sig
}

func (a *Auth) verify(value string) (string, error) {
	payload, sig, ok := strings.Cut(value, ".")
	if !ok {
		return "", ErrInvalid
	}

	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", ErrInvalid
	}

	data, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", ErrInvalid
	}

	return string(data), nil
}
