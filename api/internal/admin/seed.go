package admin

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/auth"
	"assetku/api/internal/db/gen"
	"assetku/api/internal/masterdata"
)

// Demo master data created by SeedDemo. The e2e golden path moves an asset to
// "Gudang Bekasi".
var (
	demoLocations     = []string{"Kantor Pusat Jakarta", "Gudang Bekasi", "Cabang Surabaya"}
	demoOrganizations = []string{"Kantor Pusat"}
)

// DemoSeed is the input of SeedDemo.
type DemoSeed struct {
	Tenant, Email, Password string
	// Name of the user; "Demo User" when empty.
	Name string
}

// SeedDemo creates (or brings up to date) a demo tenant with one user, three
// locations and one organization. It is idempotent: the tenant is matched by
// name and the user by email (both case-insensitive); a re-run resets the
// user's name, password and preferences (Indonesian, dark) and reactivates
// both, so every demo or e2e run starts from the same state. An email that belongs to a
// user of another tenant is an error.
func SeedDemo(ctx context.Context, pool *pgxpool.Pool, in DemoSeed) error {
	tenant := strings.TrimSpace(in.Tenant)
	email := strings.TrimSpace(in.Email)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "Demo User"
	}
	if tenant == "" {
		return errors.New("tenant is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return errors.New("invalid email")
	}
	if p := auth.PasswordProblem(in.Password); p != "" {
		return errors.New("password " + p + " (min 8 characters, max 72 bytes)")
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Two concurrent runs (e.g. compose restarts) must not create two tenants.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('assetku.seed-demo'))`); err != nil {
		return err
	}

	var tenantID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM tenants WHERE lower(btrim(name)) = lower($1) ORDER BY created_at, id LIMIT 1`, tenant).Scan(&tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		tenantID, err = gen.New(tx).CreateTenant(ctx, tenant)
	} else if err == nil {
		_, err = tx.Exec(ctx, `UPDATE tenants SET active = true, updated_at = now() WHERE id = $1 AND NOT active`, tenantID)
	}
	if err != nil {
		return fmt.Errorf("tenant: %w", err)
	}

	var userID, userTenant uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id, tenant_id FROM users WHERE lower(email) = lower($1)`, email).Scan(&userID, &userTenant)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		_, err = tx.Exec(ctx, `INSERT INTO users (tenant_id, name, email, password_hash) VALUES ($1, $2, $3, $4)`,
			tenantID, name, email, hash)
	case err == nil && userTenant != tenantID:
		return fmt.Errorf("email %s already belongs to a user of another tenant", email)
	case err == nil:
		_, err = tx.Exec(ctx, `UPDATE users SET name = $2, password_hash = $3, active = true, locale = 'id', theme = 'dark', updated_at = now() WHERE id = $1`,
			userID, name, hash)
		if err == nil {
			err = gen.New(tx).DeleteSessionsForSubject(ctx, gen.DeleteSessionsForSubjectParams{SubjectType: auth.SubjectUser, SubjectID: userID})
		}
	}
	if err != nil {
		return fmt.Errorf("user: %w", err)
	}

	md := masterdata.NewService(pool, nil)
	for _, kind := range []struct {
		k     masterdata.Kind
		names []string
	}{{masterdata.Locations, demoLocations}, {masterdata.Organizations, demoOrganizations}} {
		for _, n := range kind.names {
			if _, _, err := md.FindOrCreateByName(ctx, tx, tenantID, kind.k, n); err != nil {
				return fmt.Errorf("%s %q: %w", kind.k, n, err)
			}
		}
	}
	return tx.Commit(ctx)
}
