package masterdata

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"assetku/api/internal/db"
	"assetku/api/internal/httpx"
)

var errNotFound = httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "not found"}

func validation(fields map[string]string) httpx.Error {
	return httpx.Error{Status: http.StatusUnprocessableEntity, Code: "validation", Message: "validation failed", Fields: fields}
}

// Register mounts /api/{locations,organizations,suppliers}; all require a user session.
func (s *Service) Register(mux *http.ServeMux) {
	user := httpx.RequireUser(s.auth)
	for k := range kindFields {
		base := "/api/" + string(k)
		kind := k
		mux.Handle("GET "+base, user(httpx.H(func(w http.ResponseWriter, r *http.Request) error { return s.list(w, r, kind) })))
		mux.Handle("POST "+base, user(httpx.H(func(w http.ResponseWriter, r *http.Request) error { return s.create(w, r, kind) })))
		mux.Handle("PATCH "+base+"/{id}", user(httpx.H(func(w http.ResponseWriter, r *http.Request) error { return s.update(w, r, kind) })))
		mux.Handle("DELETE "+base+"/{id}", user(httpx.H(func(w http.ResponseWriter, r *http.Request) error { return s.remove(w, r, kind) })))
	}
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
	switch {
	case errors.Is(err, ErrNotFound):
		return errNotFound
	case errors.Is(err, ErrNameTaken):
		return validation(map[string]string{"name": "taken"})
	case errors.Is(err, ErrNameRequired):
		return validation(map[string]string{"name": "required"})
	}
	return err
}

// decodeValues reads the body and keeps only the kind's whitelisted fields.
func decodeValues(r *http.Request, k Kind) (map[string]string, error) {
	var raw map[string]*string
	if err := httpx.Decode(r, &raw); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, c := range k.columns() {
		if v := raw[c]; v != nil {
			out[c] = *v
		}
	}
	return out, nil
}

func (s *Service) list(w http.ResponseWriter, r *http.Request, k Kind) error {
	items, err := s.List(r.Context(), httpx.PrincipalFrom(r.Context()).TenantID, k, r.URL.Query().Get("q"))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, items)
	return nil
}

func (s *Service) create(w http.ResponseWriter, r *http.Request, k Kind) error {
	vals, err := decodeValues(r, k)
	if err != nil {
		return err
	}
	it, err := s.Create(r.Context(), httpx.PrincipalFrom(r.Context()).TenantID, k, vals)
	if err != nil {
		return apiErr(err)
	}
	httpx.JSON(w, http.StatusCreated, it)
	return nil
}

func (s *Service) update(w http.ResponseWriter, r *http.Request, k Kind) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	vals, err := decodeValues(r, k)
	if err != nil {
		return err
	}
	it, err := s.Update(r.Context(), httpx.PrincipalFrom(r.Context()).TenantID, id, k, vals)
	if err != nil {
		return apiErr(err)
	}
	httpx.JSON(w, http.StatusOK, it)
	return nil
}

func (s *Service) remove(w http.ResponseWriter, r *http.Request, k Kind) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	tenantID := httpx.PrincipalFrom(r.Context()).TenantID
	err = s.Delete(r.Context(), tenantID, id, k)
	if db.IsFKViolation(err) {
		counts, cerr := s.AssetCounts(r.Context(), tenantID, k)
		if cerr != nil {
			return cerr
		}
		return httpx.Error{Status: http.StatusConflict, Code: "in_use",
			Message: "still referenced by assets", Fields: map[string]string{"count": strconv.FormatInt(counts[id], 10)}}
	}
	if err != nil {
		return apiErr(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
