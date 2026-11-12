package notify

import (
	"net/http"
	"strconv"

	"assetku/api/internal/httpx"
)

var errNotFound = httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "not found"}

// Register mounts the notification routes; all require a user session.
func (s *Service) Register(mux *http.ServeMux) {
	user := httpx.RequireUser(s.auth)
	mux.Handle("GET /api/notifications", user(httpx.H(s.list)))
	mux.Handle("POST /api/notifications/read-all", user(httpx.H(s.readAll)))
	mux.Handle("POST /api/notifications/{id}/read", user(httpx.H(s.read)))
}

func (s *Service) list(w http.ResponseWriter, r *http.Request) error {
	p := httpx.PrincipalFrom(r.Context())
	out, err := s.List(r.Context(), p.UserID, p.Locale)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, out)
	return nil
}

func (s *Service) read(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return errNotFound
	}
	p := httpx.PrincipalFrom(r.Context())
	ok, err := s.MarkRead(r.Context(), p.TenantID, p.UserID, id)
	if err != nil {
		return err
	}
	if !ok {
		return errNotFound
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Service) readAll(w http.ResponseWriter, r *http.Request) error {
	if err := s.MarkAllRead(r.Context(), httpx.PrincipalFrom(r.Context()).UserID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
