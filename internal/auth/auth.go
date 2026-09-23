package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const CookieName = "token"

var (
	ErrNoCookie    = errors.New("no cookie")
	ErrInvalid     = errors.New("invalid cookie")
	ErrEmptySecret = errors.New("auth secret must not be empty")
	ErrInvalidTTL  = errors.New("auth token ttl must be positive")
)

type contextKey string

const (
	userIDKey   contextKey = "userID"
	noUserIDKey contextKey = "noUserID"
)

type State int

const (
	StateNew State = iota
	StateUnauthorized
	StateOK
)

type tokenClaims struct {
	jwt.RegisteredClaims
}

type Signer struct {
	secret   []byte
	verifier *Verifier
	ttl      time.Duration
}

func NewSigner(secret string, ttl time.Duration) (*Signer, error) {
	return newSigner([]byte(secret), ttl)
}

type Verifier struct {
	secret []byte
}

func NewVerifier(secret string) (*Verifier, error) {
	if secret == "" {
		return nil, ErrEmptySecret
	}
	return &Verifier{secret: []byte(secret)}, nil
}

func newSigner(key []byte, ttl time.Duration) (*Signer, error) {
	if len(key) == 0 {
		return nil, ErrEmptySecret
	}
	if ttl <= 0 {
		return nil, ErrInvalidTTL
	}
	return &Signer{secret: key, verifier: newVerifier(key), ttl: ttl}, nil
}

func newVerifier(key []byte) *Verifier {
	return &Verifier{secret: key}
}

func NewUserID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Signer) Cookie(userID string) (*http.Cookie, error) {
	value, err := s.sign(userID)
	if err != nil {
		return nil, err
	}
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
	}, nil
}

func (s *Signer) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, state := s.verifier.Resolve(r)

		ctx := r.Context()
		switch state {
		case StateNew:
			if cookie, err := s.Cookie(userID); err == nil {
				http.SetCookie(w, cookie)
			}
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

func (v *Verifier) Resolve(r *http.Request) (string, State) {
	c, err := r.Cookie(CookieName)
	if errors.Is(err, http.ErrNoCookie) {
		id, err := NewUserID()
		if err != nil {
			return "", StateUnauthorized
		}
		return id, StateNew
	}

	id, err := v.verify(c.Value)
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

func UserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}

func HasNoUserID(ctx context.Context) bool {
	v, _ := ctx.Value(noUserIDKey).(bool)
	return v
}

func (s *Signer) sign(userID string) (string, error) {
	now := time.Now()
	claims := tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (v *Verifier) verify(value string) (string, error) {
	claims := &tokenClaims{}
	_, err := jwt.ParseWithClaims(
		value,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, ErrInvalid
			}
			return v.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return "", ErrInvalid
	}
	return claims.Subject, nil
}
