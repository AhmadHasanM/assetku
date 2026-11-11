package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/db/gen"
	"assetku/api/internal/httpx"
)

const (
	sessionTTL     = 30 * 24 * time.Hour
	sessionRenewAt = 15 * 24 * time.Hour // renew when less than this remains

	SubjectUser  = "user"
	SubjectAdmin = "super_admin"
)

type (
	Principal      = httpx.Principal
	AdminPrincipal = httpx.AdminPrincipal
)

// FromContext returns the Principal placed by httpx.RequireUser.
func FromContext(ctx context.Context) Principal { return httpx.PrincipalFrom(ctx) }

// Service owns sessions and the auth/profile endpoints.
type Service struct {
	pool         *pgxpool.Pool
	q            *gen.Queries
	cookieSecure bool
	limiter      *RateLimiter
	now          func() time.Time
}

func NewService(pool *pgxpool.Pool, cookieSecure bool) *Service {
	return &Service{
		pool:         pool,
		q:            gen.New(pool),
		cookieSecure: cookieSecure,
		limiter:      NewRateLimiter(10, 15*time.Minute),
		now:          time.Now,
	}
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// CreateSession stores a new session for the subject and returns the raw token.
func (s *Service) CreateSession(ctx context.Context, subjectType string, subjectID uuid.UUID) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	err := s.q.CreateSession(ctx, gen.CreateSessionParams{
		TokenHash:   hashToken(token),
		SubjectType: subjectType,
		SubjectID:   subjectID,
		ExpiresAt:   s.now().Add(sessionTTL),
	})
	return token, err
}

// DestroySession deletes the session identified by the raw token.
func (s *Service) DestroySession(ctx context.Context, token string) error {
	return s.q.DeleteSession(ctx, hashToken(token))
}

// SessionCookie builds the session cookie for token.
func (s *Service) SessionCookie(token string) *http.Cookie {
	return &http.Cookie{
		Name:     httpx.SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cookieSecure,
	}
}

func (s *Service) ClearCookie() *http.Cookie {
	c := s.SessionCookie("")
	c.MaxAge = -1
	return c
}

// lookup finds the live session of the given subject type and renews it when
// less than sessionRenewAt remains.
func (s *Service) lookup(ctx context.Context, token, subjectType string) (uuid.UUID, bool, error) {
	h := hashToken(token)
	row, err := s.q.GetSession(ctx, h)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, httpx.ErrUnauthenticated
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	if row.SubjectType != subjectType {
		return uuid.Nil, false, httpx.ErrUnauthenticated
	}
	refreshed := false
	if row.ExpiresAt.Sub(s.now()) < sessionRenewAt {
		if err := s.q.ExtendSession(ctx, gen.ExtendSessionParams{TokenHash: h, ExpiresAt: s.now().Add(sessionTTL)}); err != nil {
			return uuid.Nil, false, err
		}
		refreshed = true
	}
	return row.SubjectID, refreshed, nil
}

// ResolveUser implements httpx.SessionResolver.
func (s *Service) ResolveUser(ctx context.Context, token string) (Principal, bool, error) {
	id, refreshed, err := s.lookup(ctx, token, SubjectUser)
	if err != nil {
		return Principal{}, false, err
	}
	u, err := s.q.GetPrincipalUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) { // user or tenant deactivated
		return Principal{}, false, httpx.ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, false, err
	}
	return Principal{UserID: u.ID, TenantID: u.TenantID, Name: u.Name, Position: u.Position, Locale: u.Locale, Theme: u.Theme}, refreshed, nil
}

// ResolveAdmin implements httpx.SessionResolver.
func (s *Service) ResolveAdmin(ctx context.Context, token string) (AdminPrincipal, bool, error) {
	id, refreshed, err := s.lookup(ctx, token, SubjectAdmin)
	if err != nil {
		return AdminPrincipal{}, false, err
	}
	if _, err := s.q.GetPrincipalAdmin(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return AdminPrincipal{}, false, httpx.ErrUnauthenticated
	} else if err != nil {
		return AdminPrincipal{}, false, err
	}
	return AdminPrincipal{ID: id}, refreshed, nil
}

// Limiter exposes the login rate limiter so other login endpoints (admin) share it.
func (s *Service) Limiter() *RateLimiter { return s.limiter }
