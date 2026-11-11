package httpx

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

const (
	SessionCookieName = "session"
	CSRFCookieName    = "csrf_token"
	CSRFHeaderName    = "X-CSRF-Token"
)

// ErrUnauthenticated is returned by a SessionResolver when the session is
// missing, expired, or its subject/tenant is inactive.
var ErrUnauthenticated = errors.New("unauthenticated")

// Principal is the authenticated tenant user (auth.Principal is an alias).
type Principal struct {
	UserID, TenantID uuid.UUID
	Name, Position   string
	Locale, Theme    string
}

// AdminPrincipal is the authenticated super admin (auth.AdminPrincipal is an alias).
type AdminPrincipal struct {
	ID uuid.UUID
}

// SessionResolver maps a session token to a principal. The bool reports that
// the session was extended, so the cookie should be re-issued.
type SessionResolver interface {
	ResolveUser(ctx context.Context, token string) (Principal, bool, error)
	ResolveAdmin(ctx context.Context, token string) (AdminPrincipal, bool, error)
	SessionCookie(token string) *http.Cookie
}

type ctxKey int

const (
	principalKey ctxKey = iota
	adminKey
)

// PrincipalFrom returns the user principal placed by RequireUser (zero value if absent).
func PrincipalFrom(ctx context.Context) Principal {
	p, _ := ctx.Value(principalKey).(Principal)
	return p
}

// AdminFrom returns the principal placed by RequireSuperAdmin (zero value if absent).
func AdminFrom(ctx context.Context) AdminPrincipal {
	p, _ := ctx.Value(adminKey).(AdminPrincipal)
	return p
}

var errUnauthenticated = Error{Status: http.StatusUnauthorized, Code: "unauthenticated", Message: "authentication required"}

func authenticate[P any](res SessionResolver, resolve func(context.Context, string) (P, bool, error), key ctxKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return H(func(w http.ResponseWriter, r *http.Request) error {
			ck, err := r.Cookie(SessionCookieName)
			if err != nil || ck.Value == "" {
				return errUnauthenticated
			}
			p, refreshed, err := resolve(r.Context(), ck.Value)
			if errors.Is(err, ErrUnauthenticated) {
				return errUnauthenticated
			}
			if err != nil {
				return err
			}
			if refreshed {
				http.SetCookie(w, res.SessionCookie(ck.Value))
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key, p)))
			return nil
		})
	}
}

// RequireUser rejects requests without a valid active-user session (401
// unauthenticated) and stores the Principal in the request context.
func RequireUser(res SessionResolver) func(http.Handler) http.Handler {
	return authenticate(res, res.ResolveUser, principalKey)
}

// RequireSuperAdmin is like RequireUser for super admin sessions.
func RequireSuperAdmin(res SessionResolver) func(http.Handler) http.Handler {
	return authenticate(res, res.ResolveAdmin, adminKey)
}

// CSRF implements double-submit-cookie protection: a csrf_token cookie is set
// on any response when absent, and every method other than GET/HEAD/OPTIONS
// must echo it in the X-CSRF-Token header.
func CSRF(secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return H(func(w http.ResponseWriter, r *http.Request) error {
			ck, err := r.Cookie(CSRFCookieName)
			have := err == nil && ck.Value != ""
			if !have {
				b := make([]byte, 32)
				if _, err := rand.Read(b); err != nil {
					return err
				}
				http.SetCookie(w, &http.Cookie{
					Name:     CSRFCookieName,
					Value:    base64.RawURLEncoding.EncodeToString(b),
					Path:     "/",
					MaxAge:   365 * 24 * 3600,
					SameSite: http.SameSiteLaxMode,
					Secure:   secure,
				})
			}
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
			default:
				hdr := r.Header.Get(CSRFHeaderName)
				if !have || subtle.ConstantTimeCompare([]byte(hdr), []byte(ck.Value)) != 1 {
					return Error{Status: http.StatusForbidden, Code: "csrf", Message: "invalid CSRF token"}
				}
			}
			next.ServeHTTP(w, r)
			return nil
		})
	}
}
