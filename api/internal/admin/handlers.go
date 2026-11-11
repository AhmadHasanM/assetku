package admin

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"assetku/api/internal/auth"
	"assetku/api/internal/db/gen"
	"assetku/api/internal/httpx"
)

var (
	errInvalidCreds = httpx.Error{Status: http.StatusUnauthorized, Code: "invalid_credentials", Message: "invalid email or password"}
	errRateLimited  = httpx.Error{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: "too many failed attempts, try again later"}
	errNotFound     = httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "not found"}
)

func validation(fields map[string]string) httpx.Error {
	return httpx.Error{Status: http.StatusUnprocessableEntity, Code: "validation", Message: "validation failed", Fields: fields}
}

type adminMe struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
}

type tenantJSON struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Active     bool      `json:"active"`
	UserCount  int64     `json:"user_count"`
	AssetCount int64     `json:"asset_count"`
	CreatedAt  time.Time `json:"created_at"`
}

type userJSON struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Position string    `json:"position"`
	Active   bool      `json:"active"`
}

// Register mounts the admin routes. Everything but login requires a super admin session.
func (s *Service) Register(mux *http.ServeMux) {
	adm := httpx.RequireSuperAdmin(s.auth)
	mux.HandleFunc("POST /api/admin/auth/login", httpx.H(s.login))
	mux.HandleFunc("POST /api/admin/auth/logout", httpx.H(s.logout))
	mux.Handle("GET /api/admin/auth/me", adm(httpx.H(s.me)))
	mux.Handle("GET /api/admin/tenants", adm(httpx.H(s.listTenants)))
	mux.Handle("POST /api/admin/tenants", adm(httpx.H(s.createTenant)))
	mux.Handle("PATCH /api/admin/tenants/{id}", adm(httpx.H(s.updateTenant)))
	mux.Handle("GET /api/admin/tenants/{id}/users", adm(httpx.H(s.listUsers)))
	mux.Handle("POST /api/admin/tenants/{id}/users", adm(httpx.H(s.createUser)))
	mux.Handle("PATCH /api/admin/users/{id}", adm(httpx.H(s.updateUser)))
	mux.Handle("POST /api/admin/users/{id}/reset-password", adm(httpx.H(s.resetPassword)))
}

func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, errNotFound
	}
	return id, nil
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
	lim := s.auth.Limiter()
	key := "admin|" + auth.LoginKey(auth.ClientIP(r), email)
	if !lim.Allowed(key) {
		return errRateLimited
	}
	fail := func() error { lim.Fail(key); return errInvalidCreds }
	a, err := s.q.GetSuperAdminForLogin(r.Context(), email)
	if errors.Is(err, pgx.ErrNoRows) {
		auth.BurnPasswordCheck(in.Password)
		return fail()
	}
	if err != nil {
		return err
	}
	if !auth.CheckPassword(a.PasswordHash, in.Password) {
		return fail()
	}
	lim.Reset(key)
	token, err := s.auth.CreateSession(r.Context(), auth.SubjectAdmin, a.ID)
	if err != nil {
		return err
	}
	http.SetCookie(w, s.auth.SessionCookie(token))
	httpx.JSON(w, http.StatusOK, map[string]any{"admin": adminMe{a.ID, a.Name, a.Email}})
	return nil
}

func (s *Service) logout(w http.ResponseWriter, r *http.Request) error {
	if ck, err := r.Cookie(httpx.SessionCookieName); err == nil && ck.Value != "" {
		if err := s.auth.DestroySession(r.Context(), ck.Value); err != nil {
			return err
		}
	}
	http.SetCookie(w, s.auth.ClearCookie())
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Service) me(w http.ResponseWriter, r *http.Request) error {
	a, err := s.q.GetSuperAdmin(r.Context(), httpx.AdminFrom(r.Context()).ID)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"admin": adminMe{a.ID, a.Name, a.Email}})
	return nil
}

func (s *Service) listTenants(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.q.ListTenantsAdmin(r.Context())
	if err != nil {
		return err
	}
	out := make([]tenantJSON, 0, len(rows))
	for _, t := range rows {
		out = append(out, tenantJSON{t.ID, t.Name, t.Active, t.UserCount, t.AssetCount, t.CreatedAt})
	}
	httpx.JSON(w, http.StatusOK, out)
	return nil
}

func (s *Service) writeTenant(w http.ResponseWriter, r *http.Request, status int, id uuid.UUID) error {
	t, err := s.q.GetTenantAdmin(r.Context(), id)
	if isNotFound(err) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	httpx.JSON(w, status, tenantJSON{t.ID, t.Name, t.Active, t.UserCount, t.AssetCount, t.CreatedAt})
	return nil
}

func (s *Service) createTenant(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return validation(map[string]string{"name": "required"})
	}
	id, err := s.q.CreateTenant(r.Context(), name)
	if err != nil {
		return err
	}
	return s.writeTenant(w, r, http.StatusCreated, id)
}

func (s *Service) updateTenant(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var in struct {
		Name   *string `json:"name"`
		Active *bool   `json:"active"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" {
			return validation(map[string]string{"name": "required"})
		}
		in.Name = &n
	}
	n, err := s.q.UpdateTenant(r.Context(), gen.UpdateTenantParams{ID: id, Name: in.Name, Active: in.Active})
	if err != nil {
		return err
	}
	if n == 0 {
		return errNotFound
	}
	return s.writeTenant(w, r, http.StatusOK, id)
}

func (s *Service) listUsers(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if _, err := s.q.GetTenantAdmin(r.Context(), id); isNotFound(err) {
		return errNotFound
	} else if err != nil {
		return err
	}
	rows, err := s.q.ListTenantUsers(r.Context(), id)
	if err != nil {
		return err
	}
	out := make([]userJSON, 0, len(rows))
	for _, u := range rows {
		out = append(out, userJSON{u.ID, u.Name, u.Email, u.Position, u.Active})
	}
	httpx.JSON(w, http.StatusOK, out)
	return nil
}

func (s *Service) createUser(w http.ResponseWriter, r *http.Request) error {
	tid, err := pathID(r)
	if err != nil {
		return err
	}
	var in struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Position string `json:"position"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	name, email, position := strings.TrimSpace(in.Name), strings.TrimSpace(in.Email), strings.TrimSpace(in.Position)
	fields := map[string]string{}
	if name == "" {
		fields["name"] = "required"
	}
	if email == "" {
		fields["email"] = "required"
	} else if a, err := mail.ParseAddress(email); err != nil || a.Address != email {
		fields["email"] = "invalid"
	}
	if p := auth.PasswordProblem(in.Password); p != "" {
		fields["password"] = p
	}
	if len(fields) > 0 {
		return validation(fields)
	}
	if _, err := s.q.GetTenantAdmin(r.Context(), tid); isNotFound(err) {
		return errNotFound
	} else if err != nil {
		return err
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return err
	}
	u, err := s.q.CreateUserAdmin(r.Context(), gen.CreateUserAdminParams{
		TenantID: tid, Name: name, Email: email, Position: position, PasswordHash: hash,
	})
	if isUniqueViolation(err) {
		return validation(map[string]string{"email": "taken"})
	}
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, userJSON{u.ID, u.Name, u.Email, u.Position, u.Active})
	return nil
}

func (s *Service) updateUser(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var in struct {
		Name     *string `json:"name"`
		Position *string `json:"position"`
		Active   *bool   `json:"active"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" {
			return validation(map[string]string{"name": "required"})
		}
		in.Name = &n
	}
	if in.Position != nil {
		p := strings.TrimSpace(*in.Position)
		in.Position = &p
	}
	u, err := s.q.UpdateUserAdmin(r.Context(), gen.UpdateUserAdminParams{ID: id, Name: in.Name, Position: in.Position, Active: in.Active})
	if isNotFound(err) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, userJSON{u.ID, u.Name, u.Email, u.Position, u.Active})
	return nil
}

func (s *Service) resetPassword(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var in struct {
		Password string `json:"password"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if p := auth.PasswordProblem(in.Password); p != "" {
		return validation(map[string]string{"password": p})
	}
	if ok, err := s.q.UserExists(r.Context(), id); err != nil {
		return err
	} else if !ok {
		return errNotFound
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	if err := q.UpdatePassword(r.Context(), gen.UpdatePasswordParams{ID: id, PasswordHash: hash}); err != nil {
		return err
	}
	if err := q.DeleteSessionsForSubject(r.Context(), gen.DeleteSessionsForSubjectParams{SubjectType: auth.SubjectUser, SubjectID: id}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
