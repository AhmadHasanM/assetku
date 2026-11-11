package asset

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"assetku/api/internal/httpx"
)

var errNotFound = httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "not found"}

// Register mounts /api/assets routes; all require a user session.
func (s *Service) Register(mux *http.ServeMux) {
	user := httpx.RequireUser(s.auth)
	mux.Handle("POST /api/assets", user(httpx.H(s.create)))
	mux.Handle("GET /api/assets", user(httpx.H(s.list)))
	mux.Handle("GET /api/assets/summary", user(httpx.H(s.summary)))
	mux.Handle("GET /api/assets/assignees", user(httpx.H(s.assignees)))
	mux.Handle("GET /api/assets/{id}", user(httpx.H(s.get)))
	mux.Handle("PUT /api/assets/{id}", user(httpx.H(s.update)))
	mux.Handle("DELETE /api/assets/{id}", user(httpx.H(s.remove)))
}

func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, errNotFound
	}
	return id, nil
}

// apiErr maps service errors to API errors.
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

func (s *Service) create(w http.ResponseWriter, r *http.Request) error {
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	a, err := s.Create(r.Context(), httpx.PrincipalFrom(r.Context()), in)
	if err != nil {
		return apiErr(err)
	}
	httpx.JSON(w, http.StatusCreated, a)
	return nil
}

func (s *Service) get(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	a, err := s.Get(r.Context(), httpx.PrincipalFrom(r.Context()).TenantID, id)
	if err != nil {
		return apiErr(err)
	}
	httpx.JSON(w, http.StatusOK, a)
	return nil
}

func (s *Service) update(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	a, err := s.Update(r.Context(), httpx.PrincipalFrom(r.Context()), id, in)
	if err != nil {
		return apiErr(err)
	}
	httpx.JSON(w, http.StatusOK, a)
	return nil
}

func (s *Service) remove(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := s.Delete(r.Context(), httpx.PrincipalFrom(r.Context()).TenantID, id); err != nil {
		return apiErr(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Service) assignees(w http.ResponseWriter, r *http.Request) error {
	names, err := s.AssigneeSuggestions(r.Context(), httpx.PrincipalFrom(r.Context()).TenantID, r.URL.Query().Get("q"))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, names)
	return nil
}
