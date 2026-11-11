package maintenance

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"assetku/api/internal/httpx"
)

var errNotFound = httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "not found"}

// Register mounts the maintenance routes; all require a user session.
func (s *Service) Register(mux *http.ServeMux) {
	user := httpx.RequireUser(s.auth)
	mux.Handle("GET /api/assets/{id}/maintenance", user(httpx.H(s.list)))
	mux.Handle("POST /api/assets/{id}/maintenance", user(httpx.H(s.add)))
	mux.Handle("DELETE /api/maintenance/{id}", user(httpx.H(s.remove)))
}

func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, errNotFound
	}
	return id, nil
}

func apiErr(err error) error {
	var ve *ValidationError
	switch {
	case errors.Is(err, ErrNotFound):
		return errNotFound
	case errors.As(err, &ve):
		return httpx.Error{Status: http.StatusUnprocessableEntity, Code: "validation", Message: "validation failed", Fields: ve.Fields}
	}
	return err
}

func (s *Service) list(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	out, err := s.List(r.Context(), httpx.PrincipalFrom(r.Context()).TenantID, id)
	if err != nil {
		return apiErr(err)
	}
	httpx.JSON(w, http.StatusOK, out)
	return nil
}

func (s *Service) add(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	rec, err := s.Add(r.Context(), httpx.PrincipalFrom(r.Context()), id, in)
	if err != nil {
		return apiErr(err)
	}
	httpx.JSON(w, http.StatusCreated, rec)
	return nil
}

func (s *Service) remove(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := s.Delete(r.Context(), httpx.PrincipalFrom(r.Context()), id); err != nil {
		return apiErr(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
