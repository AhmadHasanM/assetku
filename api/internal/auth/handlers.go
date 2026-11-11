package auth

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"assetku/api/internal/db/gen"
	"assetku/api/internal/httpx"
)

type tenantRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Me is the current-user payload.
type Me struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Position string    `json:"position"`
	Locale   string    `json:"locale"`
	Theme    string    `json:"theme"`
	Tenant   tenantRef `json:"tenant"`
}

func meFromRow(r gen.GetMeRow) Me {
	return Me{ID: r.ID, Name: r.Name, Email: r.Email, Position: r.Position, Locale: r.Locale, Theme: r.Theme,
		Tenant: tenantRef{ID: r.TenantID, Name: r.TenantName}}
}

var (
	errInvalidCreds = httpx.Error{Status: http.StatusUnauthorized, Code: "invalid_credentials", Message: "invalid email or password"}
	errRateLimited  = httpx.Error{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: "too many failed attempts, try again later"}
)

func validation(fields map[string]string) httpx.Error {
	return httpx.Error{Status: http.StatusUnprocessableEntity, Code: "validation", Message: "validation failed", Fields: fields}
}

// Register mounts the auth and profile routes.
func (s *Service) Register(mux *http.ServeMux) {
	user := httpx.RequireUser(s)
	mux.HandleFunc("POST /api/auth/login", httpx.H(s.login))
	mux.HandleFunc("POST /api/auth/logout", httpx.H(s.logout))
	mux.Handle("GET /api/auth/me", user(httpx.H(s.me)))
	mux.Handle("PATCH /api/me", user(httpx.H(s.updateProfile)))
	mux.Handle("POST /api/me/password", user(httpx.H(s.changePassword)))
}

var (
	dummyOnce sync.Once
	dummyHash string
)

// burnPasswordCheck equalises timing for unknown emails.
func burnPasswordCheck(pw string) {
	dummyOnce.Do(func() { dummyHash, _ = HashPassword("dummy-password") })
	CheckPassword(dummyHash, pw)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Service) login(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	email := strings.TrimSpace(in.Email)
	key := LoginKey(clientIP(r), email)
	if !s.limiter.Allowed(key) {
		return errRateLimited
	}
	fail := func() error { s.limiter.Fail(key); return errInvalidCreds }

	u, err := s.q.GetUserForLogin(r.Context(), email)
	if errors.Is(err, pgx.ErrNoRows) {
		burnPasswordCheck(in.Password)
		return fail()
	}
	if err != nil {
		return err
	}
	if !CheckPassword(u.PasswordHash, in.Password) || !u.UserActive || !u.TenantActive {
		return fail()
	}
	s.limiter.Reset(key)
	token, err := s.CreateSession(r.Context(), subjectUser, u.ID)
	if err != nil {
		return err
	}
	me, err := s.q.GetMe(r.Context(), u.ID)
	if err != nil {
		return err
	}
	http.SetCookie(w, s.SessionCookie(token))
	httpx.JSON(w, http.StatusOK, map[string]any{"user": meFromRow(me)})
	return nil
}

func (s *Service) logout(w http.ResponseWriter, r *http.Request) error {
	if ck, err := r.Cookie(httpx.SessionCookieName); err == nil && ck.Value != "" {
		if err := s.DestroySession(r.Context(), ck.Value); err != nil {
			return err
		}
	}
	http.SetCookie(w, s.clearCookie())
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Service) me(w http.ResponseWriter, r *http.Request) error {
	me, err := s.q.GetMe(r.Context(), FromContext(r.Context()).UserID)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"user": meFromRow(me)})
	return nil
}

func (s *Service) updateProfile(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name     *string `json:"name"`
		Position *string `json:"position"`
		Locale   *string `json:"locale"`
		Theme    *string `json:"theme"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	fields := map[string]string{}
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		in.Name = &n
		if n == "" {
			fields["name"] = "required"
		}
	}
	if in.Position != nil {
		p := strings.TrimSpace(*in.Position)
		in.Position = &p
	}
	if in.Locale != nil && *in.Locale != "id" && *in.Locale != "en" {
		fields["locale"] = "invalid"
	}
	if in.Theme != nil && *in.Theme != "dark" && *in.Theme != "light" {
		fields["theme"] = "invalid"
	}
	if len(fields) > 0 {
		return validation(fields)
	}
	id := FromContext(r.Context()).UserID
	if err := s.q.UpdateProfile(r.Context(), gen.UpdateProfileParams{
		ID: id, Name: in.Name, Position: in.Position, Locale: in.Locale, Theme: in.Theme,
	}); err != nil {
		return err
	}
	me, err := s.q.GetMe(r.Context(), id)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"user": meFromRow(me)})
	return nil
}

func (s *Service) changePassword(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	id := FromContext(r.Context()).UserID
	me, err := s.q.GetMe(r.Context(), id)
	if err != nil {
		return err
	}
	fields := map[string]string{}
	if !CheckPassword(me.PasswordHash, in.CurrentPassword) {
		fields["current_password"] = "invalid"
	}
	if p := PasswordProblem(in.NewPassword); p != "" {
		fields["new_password"] = p
	}
	if len(fields) > 0 {
		return validation(fields)
	}
	hash, err := HashPassword(in.NewPassword)
	if err != nil {
		return err
	}
	if err := s.q.UpdatePassword(r.Context(), gen.UpdatePasswordParams{ID: id, PasswordHash: hash}); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
