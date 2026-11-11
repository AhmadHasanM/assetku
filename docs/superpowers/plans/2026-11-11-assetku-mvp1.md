# AssetKu MVP 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Membangun aplikasi SaaS asset management multi-tenant (AssetKu MVP 1) sesuai PRD. Aplikasi siap di-deploy ke VPS dengan Docker Compose.

**Architecture:**
- **Monorepo:**
  - `api/`: Go REST JSON API. Berisi semua logika bisnis, otorisasi, PDF, import/export, dan scheduler reminder.
  - `web/`: Next.js App Router. Hanya UI, dengan fetch client-side ke `/api/*` di origin yang sama.
  - `deploy/`: konfigurasi nginx.
- **Isolasi tenant:** setiap query bisnis difilter `tenant_id` dari sesi. Akses ke resource tenant lain menghasilkan 404.
- **Sesi:** cookie httpOnly, dengan CSRF double-submit.

**Tech Stack:**
- **Go 1.25:**
  - routing: `chi` v5;
  - database: `pgx` v5, `sqlc`, `golang-migrate`;
  - auth: `golang.org/x/crypto/bcrypt`;
  - spreadsheet: `excelize` v2, `github.com/extrame/xls`;
  - PDF dan QR: `github.com/go-pdf/fpdf`, `github.com/skip2/go-qrcode`.
- **PostgreSQL 17**, dengan extension `pg_trgm` dan `pgcrypto`.
- **Node 24 LTS dan Next.js** (versi stabil terbaru):
  - TypeScript dan Tailwind CSS v4;
  - shadcn/ui dengan ikon `lucide-react`;
  - `next-intl` (tanpa i18n routing) dan `next-themes`;
  - `@tanstack/react-query`, `react-hook-form` + `zod`;
  - `qrcode.react` dan `recharts` (via shadcn chart).
- **Test:**
  - Go: `go test` terhadap Postgres sungguhan;
  - frontend: Vitest + Testing Library;
  - end-to-end: Playwright.

**Spec:** `docs/superpowers/specs/2026-11-11-asset-management-mvp1-design.md`. Baca spec ini sebelum mengerjakan task mana pun. Nomor bagian "§n" di plan merujuk ke spec.

## Global Constraints

**Produk dan tampilan**
- Nama produk "AssetKu" (placeholder). Taruh di satu konstanta per app: `api/internal/config/brand.go` `ProductName` dan `web/src/lib/brand.ts` `PRODUCT_NAME`.
- **Tidak ada emoji** di UI, PDF, pesan API, maupun seed data. Ikon UI hanya `lucide-react`. Ikon di PDF berupa gambar QR saja.
- Desain StarChart persis (§3):
  - Font Fredoka untuk heading, DM Sans untuk body, Space Mono untuk Asset Tag, serial, dan nomor BAST.
  - Tombol pill (9999px), kartu radius 16px, input radius 12px.
  - Tap target ≥ 44px. Body ≥ 14px; satu-satunya pengecualian adalah "tiny" 12px.
  - Star `#FDE047` hanya untuk CTA utama (Add Asset, tombol simpan/submit).
  - Satu elemen tidak boleh memakai star-glow dan nebula-glow sekaligus.
- **Tema:** `dark` (default) dan `light`. Palet persis dari §3 dan diimplementasikan sebagai CSS variable shadcn.
- **Bahasa:** `id` (default) dan `en`, disimpan per user. PDF BAST selalu berbahasa Indonesia. Label QR tidak bergantung bahasa.
- **Format:** tanggal di UI `dd MMM yyyy` sesuai locale. Uang `Rp 12.500.000` (pemisah ribuan titik, tanpa desimal).
- **Responsif:** semua halaman dipakai di lebar 360px, 768px, dan ≥ 1280px.

**Aturan data**
- Nilai enum di DB dan API berupa kode Inggris:
  - kategori: `it_equipment|vehicle|building_facility|furniture|electronics|other`;
  - status: `available|in_use|maintenance|damaged|lost|archived`;
  - jenis maintenance: `preventive|corrective|inspection|other`;
  - kondisi BAST: `good|new_sealed|used_normal`.
- **Asset Tag otomatis:** `AST-` + 8 karakter dari `ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789` (crypto/rand). Tag unik per tenant tanpa membedakan huruf besar-kecil.
- **Waktu:** "hari ini" untuk reminder, BAST, dan garansi dihitung di zona `Asia/Jakarta`.
- **Pagination:** daftar asset 20 baris per halaman. History 20 per halaman. Lonceng notifikasi menampilkan 20 terakhir.
- **Nomor BAST:** `BAST-<tag>-<YYYYMMDD>`.
- **Label PDF:** halaman 90×50 mm. BAST memakai A4 portrait, export PDF memakai A4 landscape.
- **Import:** maksimal 10 MB, 10.000 baris data, sheet pertama, header dicari di 10 baris teratas.
- **Reminder:** default H-7 bila tanggal berikutnya diisi. Job berjalan setiap hari pukul 07:00 WIB dan sekali saat start.
- **Error API:** selalu `{"error":{"code":"<snake_case>","message":"<en text>","fields":{"<field>":"<code>"}}}`. Frontend menerjemahkan berdasarkan `code`.

## Review Focus

1. **Isolasi tenant lewat ID langsung.** User tenant A memanggil `GET/PUT/DELETE` asset, maintenance, movement, history, label, BAST, master data, notifikasi, atau import job milik tenant B. Harus selalu 404, dan data B tidak berubah. Ditest di Task 2 (helper), 5, 7, 8, 10, 13, dan 15.
2. **Spreadsheet "berantakan" yang wajar.**
   - Ada judul dan baris kosong di atas header.
   - CSV dengan BOM dan pemisah `;` (Excel locale Indonesia).
   - Tanggal berupa serial Excel atau teks `dd/mm/yyyy`.
   - Harga "Rp 12.500.000".
   - Sel dengan spasi di ujung, dan header dengan huruf besar atau garis bawah campur.
   - Semua kasus ini harus ter-import tanpa error fatal. Ditest di Task 11 dan 12.
3. **Asset Tag beda huruf besar-kecil.** `ast-001` dan `AST-001` dianggap sama: create kedua gagal dengan `fields.asset_tag=taken`, dan import memperbarui asset yang sama. Ditest di Task 5 dan 12.
4. **Teks panjang atau Unicode di PDF.** Nama 200 karakter, `&`, `é`, dan tanda kutip tidak membuat label atau BAST error. Teks yang terlalu panjang dipotong dengan "…" dan tidak keluar dari label 90×50 mm. Ditest di Task 10.
5. **Scan QR saat belum login.** User diarahkan ke `/login?next=/assets/<id>`, lalu kembali ke asset setelah login. `next` yang bukan path relatif (misal `//evil.com` atau `https://…`) diabaikan dan user diarahkan ke `/dashboard`. Ditest di Task 14.

---

## File Structure

```
/                          docker-compose.yml, docker-compose.dev.yml, .env.example, .gitignore, README.md
deploy/nginx/nginx.conf    reverse proxy: /api/* -> api:8080, lainnya -> web:3000
api/
  cmd/server/main.go       start HTTP + migrate + scheduler; subcommand `create-superadmin`
  internal/config/         env config, brand constant
  internal/db/migrations/  NNNN_name.up.sql / .down.sql
  internal/db/queries/     *.sql untuk sqlc; internal/db/gen/ hasil generate
  internal/db/db.go        pool, Migrate(), IsUniqueViolation(), IsFKViolation()
  internal/httpx/          router, JSON helpers, error type, middleware (auth, csrf, tenant)
  internal/auth/           password, sessions, ratelimit, handlers login/logout/me/profile
  internal/admin/          super admin handlers (tenants, users)
  internal/masterdata/     locations, organizations, suppliers (satu service generik)
  internal/asset/          tag, warranty, service, list filter, handlers, assignees
  internal/history/        Record(), List(), event types
  internal/maintenance/    service + handlers
  internal/movement/       service + handlers
  internal/notify/         notifications service, reminder job, scheduler
  internal/dashboard/      handler
  internal/pdf/            fonts (embed), label.go, bast.go, terbilang.go, assetlist.go
  internal/export/         csv, xlsx, pdf writers
  internal/importer/       headers.go, values.go, reader.go, job.go, apply.go, report.go, template.go
  internal/testutil/       NewDB(t), fixtures (tenant, user, session client)
web/
  src/app/(auth)/login/              halaman login
  src/app/(app)/layout.tsx           shell: sidebar, header, lonceng, menu user
  src/app/(app)/dashboard/
  src/app/(app)/assets/  [id]/  [id]/edit/  new/
  src/app/(app)/locations/ organizations/ suppliers/ profile/
  src/app/admin/(auth)/login/  src/app/admin/(panel)/tenants/ [id]/
  src/components/ui/                 komponen shadcn (di-theme StarChart)
  src/components/                    komponen aplikasi per fitur (assets/, master/, import/, ...)
  src/lib/api.ts types.ts format.ts filters.ts brand.ts
  src/messages/id.json en.json
  src/i18n/request.ts                locale dari cookie `locale`
  middleware.ts                      redirect ke /login bila tidak ada cookie sesi
  e2e/                               Playwright
```

---

### Task 1: Scaffold repo, database, dan server Go minimal

**Files:**
- Create:
  - root: `.gitignore` (berisi `.superpowers/`, `node_modules`, `.env`, `web/.next`), `docker-compose.dev.yml` (service `db` postgres:17 di port 5432), `.env.example`;
  - `api/go.mod` (module `assetku/api`);
  - `api/cmd/server/main.go`;
  - `api/internal/config/config.go`, `api/internal/config/brand.go`;
  - `api/internal/db/db.go`, `api/internal/db/migrations/0001_init.up.sql` / `.down.sql`;
  - `api/internal/httpx/{router.go,json.go,errors.go}`;
  - `api/internal/testutil/db.go`;
  - `api/sqlc.yaml`.
- Test: `api/internal/httpx/router_test.go`, `api/internal/db/db_test.go`

**Interfaces:**
- Produces:
  - `config.Load() (Config, error)`. Field `Config`: `DatabaseURL`, `AppBaseURL`, `Addr` (default `:8080`), `CookieSecure bool`, `Timezone *time.Location` (Asia/Jakarta).
  - Fungsi DB:
    - `db.Open(ctx, url) (*pgxpool.Pool, error)`;
    - `db.Migrate(url string) error` (migrasi di-embed lewat `embed.FS`);
    - `db.IsUniqueViolation(err) (constraint string, ok bool)`;
    - `db.IsFKViolation(err) bool`.
  - `httpx.Error{Status int; Code, Message string; Fields map[string]string}` yang mengimplementasikan `error`.
  - `httpx.Handler func(w, r) error`, dibungkus `httpx.H(h)`. `H` menulis error sesuai format global, dan error non-`httpx.Error` menjadi 500 `internal`.
  - Helper JSON: `httpx.JSON(w, status, v)`, `httpx.Decode(r, &v) error` (400 `invalid_json`).
  - `httpx.NewRouter(deps Deps) http.Handler`. Ada `GET /api/health` yang mengembalikan `{"ok":true}`.
  - `testutil.NewDB(t) *pgxpool.Pool`:
    - membuat database baru `t_<rand>` di `TEST_DATABASE_URL` (default `postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable`);
    - menjalankan `db.Migrate`;
    - drop database lewat `t.Cleanup`.
  - Migration 0001 berisi `CREATE EXTENSION IF NOT EXISTS pg_trgm; CREATE EXTENSION IF NOT EXISTS pgcrypto;`.

- [ ] **Step 1:** Jalankan `git init` di root, lalu buat file root dan `docker-compose.dev.yml`. Jalankan `docker compose -f docker-compose.dev.yml up -d db`. Expected: `docker compose ps` menunjukkan db `healthy`.
- [ ] **Step 2: Write failing tests**

```go
// router_test.go
func TestHealth(t *testing.T) {
    srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{}))
    res, _ := http.Get(srv.URL + "/api/health")
    // assert 200 and body {"ok":true}
}
func TestErrorFormat(t *testing.T) {
    // handler returning httpx.Error{Status:422, Code:"validation", Fields:{"name":"required"}}
    // via httpx.H -> status 422, body {"error":{"code":"validation","message":...,"fields":{"name":"required"}}}
    // handler returning errors.New("boom") -> 500, code "internal", message does NOT contain "boom"
}
// db_test.go
func TestMigrateAndExtensions(t *testing.T) {
    pool := testutil.NewDB(t)
    // SELECT count(*) FROM pg_extension WHERE extname IN ('pg_trgm','pgcrypto') == 2
}
```

- [ ] **Step 3:** Jalankan `cd api && go test ./...`. Expected: FAIL (paket belum ada).
- [ ] **Step 4:** Implementasikan interface di atas. `main.go` menjalankan `config.Load`, `db.Migrate`, `db.Open`, lalu `http.ListenAndServe`.
- [ ] **Step 5:** Jalankan `cd api && go test ./...`. Expected: PASS. Jalankan juga `go run ./cmd/server` lalu `curl localhost:8080/api/health`. Expected: `{"ok":true}`.
- [ ] **Step 6:** Commit `chore: scaffold repo, go api skeleton, migrations`.

---

### Task 2: Tenant, user, sesi, login, CSRF, dan profil

**Files:**
- Create:
  - `api/internal/db/migrations/0002_auth.{up,down}.sql`;
  - `api/internal/db/queries/auth.sql`;
  - `api/internal/auth/{password.go,session.go,ratelimit.go,handlers.go}`;
  - `api/internal/httpx/middleware.go`;
  - `api/internal/testutil/fixtures.go`.
- Test:
  - `api/internal/auth/auth_test.go`;
  - `api/internal/httpx/middleware_test.go`.

**Interfaces:**
- **Schema:**
  - `tenants(id uuid pk default gen_random_uuid(), name text not null, active bool not null default true, created_at, updated_at)`.
  - `users`, berisi kolom:
    - `id uuid pk`, `tenant_id uuid not null fk`;
    - `name text not null`, `email text not null`, `position text not null default ''`;
    - `password_hash text not null`, `active bool default true`;
    - `locale text not null default 'id' check in ('id','en')`;
    - `theme text not null default 'dark' check in ('dark','light')`;
    - timestamps.
    - Unique index pada `lower(email)` (global, karena login tanpa memilih tenant).
  - `super_admins(id uuid pk, name, email unique lower, password_hash, created_at)`.
  - `sessions`:
    - `token_hash bytea pk`;
    - `subject_type text check in ('user','super_admin')`, `subject_id uuid`;
    - `expires_at timestamptz`, `created_at`.
- **Sesi:**
  - Token acak 32 byte, base64url, disimpan di cookie `session` (httpOnly, SameSite=Lax, Secure=`cfg.CookieSecure`, Path=/).
  - Yang disimpan di DB adalah sha256 token.
  - TTL 30 hari, sliding: diperpanjang bila sisa < 15 hari.
- **CSRF:**
  - Cookie `csrf_token` (tidak httpOnly) di-set pada respons apa pun bila belum ada.
  - Semua method non-GET/HEAD wajib header `X-CSRF-Token` yang sama dengan cookie. Kalau tidak, 403 `csrf`.
- **Middleware:**
  - `httpx.RequireUser`: menaruh `auth.Principal{UserID, TenantID uuid.UUID; Name, Position, Locale, Theme string}` di context. Kalau tidak ada sesi, user nonaktif, atau tenant nonaktif: 401 `unauthenticated`.
  - Helper: `auth.FromContext(ctx) auth.Principal`.
  - `httpx.RequireSuperAdmin`: menaruh `auth.AdminPrincipal{ID}`.
- **Fungsi:**
  - `auth.HashPassword(string) (string, error)` dengan bcrypt cost 12.
  - `auth.CheckPassword(hash, pw) bool`.
  - Password minimal 8 karakter. Kalau kurang: 422 `fields.password=too_short`.
  - `auth.RateLimiter`: maksimal 10 percobaan gagal per (IP, lower(email)) dalam 15 menit. Kalau lewat: 429 `rate_limited`. Disimpan in-memory.
- **Endpoint:**
  - `POST /api/auth/login {email,password}` → 200 `{user: Me}`. Kalau gagal: 401 `invalid_credentials`. User atau tenant nonaktif juga menghasilkan `invalid_credentials`.
  - `POST /api/auth/logout` → 204.
  - `GET /api/auth/me` → `Me{id,name,email,position,locale,theme,tenant:{id,name}}`.
  - `PATCH /api/me {name?,position?,locale?,theme?}` → `Me`.
  - `POST /api/me/password {current_password,new_password}` → 204. Kalau password lama salah: 422 `fields.current_password=invalid`.
- **Fixtures:**
  - `testutil.CreateTenant(t, pool, name) uuid.UUID`;
  - `testutil.CreateUser(t, pool, tenantID, email, password) uuid.UUID`;
  - `testutil.Client(t, srvURL)` yang mengembalikan `*testutil.APIClient`. Client ini punya cookie jar, otomatis mengirim CSRF, dan punya method `Login(email, pw)`, `Do(method, path string, body any) *http.Response`, `JSON(res, &v)`.

- [ ] **Step 1: Write failing tests** di `auth_test.go`:
  - `TestLoginSuccessSetsSessionAndMe`: setelah login, `GET /api/auth/me` mengembalikan email, `tenant.name`, `locale=="id"`, `theme=="dark"`.
  - `TestLoginWrongPassword401`.
  - `TestLoginInactiveTenant401`.
  - `TestRateLimitAfter10Failures`: percobaan ke-11 menghasilkan 429 walaupun password benar.
  - `TestPostWithoutCSRF403`: `PATCH /api/me` tanpa header menghasilkan 403 `csrf`.
  - `TestUpdateProfileLocaleTheme`: `PATCH {locale:"en",theme:"light"}`, lalu `/me` mencerminkan perubahan. `locale:"fr"` menghasilkan 422 `fields.locale=invalid`.
  - `TestChangePassword`: password lama salah menghasilkan 422. Password benar menghasilkan 204, lalu login dengan password baru berhasil.
  - `TestLogoutInvalidatesSession`: setelah logout, `/me` menghasilkan 401.
- [ ] **Step 2:** Jalankan `go test ./internal/auth/...`. Expected: FAIL.
- [ ] **Step 3:** Implementasikan schema, sqlc queries (`sqlc generate`), service, middleware, dan handler sesuai Interfaces.
- [ ] **Step 4:** Jalankan `go test ./...`. Expected: PASS.
- [ ] **Step 5:** Commit `feat(api): auth, sessions, csrf, profile`.

---

### Task 3: Super admin (seed dan panel API)

**Files:**
- Create:
  - `api/internal/admin/{handlers.go,service.go}`;
  - `api/internal/db/queries/admin.sql`.
- Modify: `api/cmd/server/main.go` (subcommand), `api/internal/httpx/router.go`.
- Test: `api/internal/admin/admin_test.go`

**Interfaces:**
- **Seed:** subcommand `server create-superadmin --email E --name N --password P`. Idempoten: kalau email sudah ada, password-nya di-update.
- **Endpoint** (semua di bawah `RequireSuperAdmin`, kecuali login):
  - **Sesi admin:**
    - `POST /api/admin/auth/login`;
    - `POST /api/admin/auth/logout`;
    - `GET /api/admin/auth/me`.
  - **Tenant:**
    - `GET /api/admin/tenants` → `[{id,name,active,user_count,asset_count,created_at}]`;
    - `POST /api/admin/tenants {name}`;
    - `PATCH /api/admin/tenants/{id} {name?,active?}`.
  - **User:**
    - `GET /api/admin/tenants/{id}/users` → `[{id,name,email,position,active}]`;
    - `POST /api/admin/tenants/{id}/users {name,email,position,password}`. Kalau email sudah dipakai: 422 `fields.email=taken`;
    - `PATCH /api/admin/users/{id} {name?,position?,active?}`;
    - `POST /api/admin/users/{id}/reset-password {password}` → 204. Semua sesi user itu dihapus.
- **Isolasi admin:** sesi `user` tidak bisa mengakses `/api/admin/*` (401). Sesi admin tidak bisa mengakses API tenant (401).
- **Data asset:** endpoint admin tidak pernah mengembalikan data asset, hanya jumlah.
- **Catatan:** `asset_count` baru tersedia setelah Task 5. Sampai saat itu, kembalikan 0 dari query yang aman. Task 5 menambahkan join ke tabel `assets`.

- [ ] **Step 1: Write failing tests:**
  - `TestCreateTenantAndUserThenUserCanLogin`.
  - `TestDuplicateEmail422`.
  - `TestDeactivateTenantBlocksLogin`.
  - `TestResetPasswordKillsSessions`: client user yang sudah login mendapat 401 setelah reset.
  - `TestUserSessionCannotAccessAdmin`.
  - `TestCreateSuperadminCommandIdempotent`: memanggil fungsi `admin.UpsertSuperAdmin` dua kali menghasilkan satu baris.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan `go test ./...`. Expected: PASS.
- [ ] **Step 5:** Commit `feat(api): super admin tenants and users`.

---

### Task 4: Master data (Location, Organization, Supplier)

**Files:**
- Create:
  - `api/internal/db/migrations/0003_master_data.{up,down}.sql`;
  - `api/internal/db/queries/masterdata.sql`;
  - `api/internal/masterdata/{service.go,handlers.go}`.
- Test: `api/internal/masterdata/masterdata_test.go`

**Interfaces:**
- **Schema:**
  - `locations(id, tenant_id, name, description text default '', timestamps)`.
  - `organizations(id, tenant_id, name, timestamps)`.
  - `suppliers(id, tenant_id, name, contact_name, phone, email, address — text default '', timestamps)`.
  - Unique index `(tenant_id, lower(btrim(name)))` di ketiganya.
- **Kind dan field:**
  - Tipe `masterdata.Kind` bernilai `"locations"|"organizations"|"suppliers"`, dengan whitelist field per kind.
  - Service `masterdata.Service` punya `FindOrCreateByName(ctx, tx, tenantID, kind, name) (uuid.UUID, created bool, error)` dan `FindByName(ctx, tx, tenantID, kind, name) (uuid.UUID, bool, error)`. Keduanya dipakai importer. Nama di-trim dan dicocokkan tanpa membedakan huruf besar-kecil.
- **Endpoint** (untuk setiap kind):
  - `GET /api/{kind}?q=` → `[{id,name,...,asset_count}]`, urut nama. Sampai Task 5, `asset_count` selalu 0. Task 5 mengganti nilai ini dengan hitungan dari tabel `assets`;
  - `POST /api/{kind}`;
  - `PATCH /api/{kind}/{id}`;
  - `DELETE /api/{kind}/{id}`.
- **Validasi dan error:**
  - Nama kosong: 422 `fields.name=required`.
  - Nama sudah dipakai: 422 `fields.name=taken`.
  - Hapus yang masih dipakai: 409 `{"error":{"code":"in_use","message":...,"fields":{"count":"<n>"}}}`. Logika ini dilengkapi di Task 5 setelah tabel assets ada. Di task ini, handler delete sudah memetakan `db.IsFKViolation` ke 409 `in_use` beserta count dari query hitung.
  - Tenant lain: 404 `not_found`.

- [ ] **Step 1: Write failing tests:**
  - `TestCRUDLocations`.
  - `TestNameUniqueCaseInsensitivePerTenant`: " Gudang " dan "gudang" ditolak di tenant yang sama, tetapi boleh di tenant lain.
  - `TestCrossTenantPatchDelete404`.
  - `TestFindOrCreateByName`: panggilan kedua dengan "  KANTOR pusat" mengembalikan id yang sama dan `created=false`.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan test. Expected: PASS.
- [ ] **Step 5:** Commit `feat(api): master data locations/organizations/suppliers`.

---

### Task 5: Asset CRUD, tag, garansi, dan History

**Files:**
- Create:
  - `api/internal/db/migrations/0004_assets_history.{up,down}.sql`;
  - `api/internal/db/queries/{assets.sql,history.sql}`;
  - `api/internal/asset/{tag.go,warranty.go,model.go,service.go,handlers.go}`;
  - `api/internal/history/{history.go,handlers.go}`.
- Modify: `api/internal/admin/service.go` (`asset_count`), `api/internal/masterdata/service.go` (`asset_count` di list).
- Test:
  - `api/internal/asset/tag_test.go`;
  - `api/internal/asset/warranty_test.go`;
  - `api/internal/asset/asset_test.go`;
  - `api/internal/history/history_test.go`.

**Interfaces:**
- **Schema `assets`:**
  - Kolom identitas: `id`, `tenant_id`, `name text not null`, `asset_tag text not null`, `category text check(enum)`.
  - Kolom teks dengan default `''`: `manufacturer`, `model`, `serial_number`, `assigned_to`, `order_number`, `notes`.
  - Kolom status: `status text check(enum) default 'available'`.
  - Relasi master:
    - `location_id`, `organization_id`, `supplier_id` sebagai uuid null;
    - FK ke master data dengan `ON DELETE RESTRICT`.
  - Kolom pembelian: `purchase_date date null`, `purchase_cost numeric(16,2) null check >= 0`, `warranty_months int null check >= 0`.
  - Timestamps.
  - Index:
    - unique `(tenant_id, lower(asset_tag))`;
    - index `(tenant_id, status)` dan `(tenant_id, category)`;
    - GIN trigram pada `lower(asset_tag||' '||name||' '||serial_number||' '||model||' '||assigned_to)`.
- **Schema `asset_history`:**
  - `id bigserial`, `tenant_id`, `asset_id fk ON DELETE CASCADE`;
  - `event text not null`;
  - `actor_id uuid null`, `actor_name text not null`;
  - `detail jsonb not null default '{}'`;
  - `created_at`;
  - index `(asset_id, id desc)`.
- **Fungsi tag dan garansi:**
  - `asset.GenerateTag() string`.
  - `asset.NormalizeTag(s) string`: hanya trim.
  - `asset.WarrantyExpires(purchase *civil.Date, months *int) *civil.Date`: hasilnya `purchase.AddMonths(months)`. Kalau salah satu nil, hasilnya nil. Tanggal akhir bulan di-clamp ke hari terakhir bulan, misalnya 31 Jan + 1 bulan = 28/29 Feb. Pakai `cloud.google.com/go/civil` `civil.Date` di semua paket (asset, maintenance, notify, pdf, importer). Untuk clamp akhir bulan, buat helper `asset.AddMonthsClamped`.
- **Tipe `asset.Input`:**
  - Semua field yang bisa diedit.
  - FK dikirim sebagai `*uuid.UUID`.
  - `PurchaseCost` bertipe `*decimal`. Pakai `github.com/shopspring/decimal`, dan serialisasikan di JSON sebagai string `"12500000.00"`.
- **Tipe `asset.Asset`** berisi:
  - Input;
  - `ID`;
  - `Location`, `Organization`, `Supplier` sebagai `*Ref{ID,Name}`;
  - `WarrantyExpires *Date`;
  - `WarrantyExpired bool`, dibandingkan dengan hari ini di Jakarta;
  - timestamps.
- **Service:**
  - `asset.Service.Create(ctx, p auth.Principal, in Input) (Asset, error)`;
  - `Update(ctx, p, id, in Input) (Asset, error)`, berupa replace penuh;
  - `Get(ctx, tenantID, id)`;
  - `Delete(ctx, tenantID, id)`.
  - Validasi:
    - name dan category wajib;
    - status wajib dan harus enum;
    - FK harus milik tenant yang sama. Kalau tidak, 422 `fields.location_id=invalid`.
  - Tag kosong diisi `GenerateTag`, dengan retry sampai 5 kali bila bentrok. Tag dari user yang bentrok menghasilkan 422 `fields.asset_tag=taken`.
- **History:**
  - `history.Record(ctx, q db.Querier, e history.Entry) error`, dengan `Entry{TenantID, AssetID uuid.UUID; Event string; ActorID *uuid.UUID; ActorName string; Detail map[string]any}`.
  - Konstanta event:
    - siklus asset: `asset_created`, `asset_imported_created`, `asset_imported_updated`;
    - perubahan field: `status_changed`, `location_changed`, `assignee_changed`, `details_updated`;
    - cetak: `label_printed`, `bast_printed`;
    - catatan: `maintenance_added`, `maintenance_deleted`, `movement_added`, `movement_deleted`.
  - Aturan saat `Update`, dijalankan dalam satu transaksi:
    - status berbeda: `status_changed {from,to}`;
    - lokasi berbeda: `location_changed {from,to}`, berisi nama lokasi;
    - assigned_to berbeda: `assignee_changed {from,to}`;
    - field lain berubah: satu `details_updated {fields:[...nama field json...]}`;
    - tidak ada yang berubah: tidak ada entri.
  - `Create` menghasilkan `asset_created`.
- **Endpoint asset:**
  - `POST /api/assets`;
  - `GET /api/assets/{id}`;
  - `PUT /api/assets/{id}`;
  - `DELETE /api/assets/{id}` (204, hard delete, cascade).
- **Endpoint lain:**
  - `GET /api/assets/{id}/history?before=<id>` → `{items:[{id,event,actor_name,detail,created_at}], next_before:<id|null>}`. Urut `id desc`, 20 per halaman.
  - `GET /api/assets/assignees?q=` → `[]string`. Maksimal 10 nama distinct non-empty yang mengandung q (tanpa membedakan huruf besar-kecil), urut berdasarkan frekuensi.

- [ ] **Step 1: Write failing tests**
  - `tag_test.go`:
    - `TestGenerateTagFormat`: regex `^AST-[A-Z0-9]{8}$`, dicek 1000 kali.
  - `warranty_test.go`:
    - 2026-01-15 + 12 = 2027-01-15;
    - 2026-01-31 + 1 = 2026-02-28;
    - nil months menghasilkan nil.
  - `asset_test.go`:
    - `TestCreateAutoTag`.
    - `TestCreateRequiredFields422`: fields name, category, status.
    - `TestTagUniqueCaseInsensitive`: `AST-001` lalu `ast-001` menghasilkan 422 `asset_tag=taken`. Di tenant lain OK.
    - `TestFKFromOtherTenant422`.
    - `TestUpdateRecordsHistory`: satu PUT yang mengubah status, lokasi, assigned_to, dan model menghasilkan tepat 4 entri dengan event yang benar, dan `details_updated.fields == ["model"]`.
    - `TestUpdateNoChangeNoHistory`.
    - `TestCrossTenantGetPutDelete404`.
    - `TestDeleteCascadesHistory`.
    - `TestMasterDeleteInUse409`: hapus lokasi yang dipakai 2 asset menghasilkan 409 `in_use` dengan `count=2`.
    - `TestAssigneeSuggestions`.
    - `TestWarrantyExpiredFlag`.
  - `history_test.go`:
    - `TestHistoryPaginationAndCrossTenant404`.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan `go test ./...`. Expected: PASS.
- [ ] **Step 5:** Commit `feat(api): assets crud, tags, warranty, history`.

---

### Task 6: Daftar asset (search, filter, sort, pagination) dan summary

**Files:**
- Create: `api/internal/asset/{listfilter.go,list.go}`
- Test: `api/internal/asset/list_test.go`

**Interfaces:**
- **`asset.ListFilter`:** `{Q string; Categories, Statuses []string; LocationID, OrganizationID *uuid.UUID; Sort string; Dir string; Page int}`.
- **`asset.ParseListFilter(url.Values) (ListFilter, error)`.** Dipakai ulang oleh export.
  - Query param:
    - `q`;
    - `category` dan `status` (dipisah koma);
    - `location_id`, `organization_id`;
    - `sort` ∈ `asset_tag|name|category|model|serial_number|status|created_at` (default `created_at`);
    - `dir` ∈ `asc|desc` (default `desc`);
    - `page` (default 1).
  - Nilai tidak valid menghasilkan 400 `invalid_filter`.
- **Fungsi list:**
  - `Service.List(ctx, tenantID, f ListFilter, limit int) ([]ListItem, total int, error)`. Kalau `limit<=0`, semua baris dikembalikan (untuk export). Kalau tidak, 20 baris dengan offset dari page.
  - `ListItem{ID, AssetTag, Name, Category, Model, SerialNumber, Status, AssignedTo, LocationName, OrganizationName}`.
- **Pencarian:** `q` di-trim. Kalau tidak kosong, pakai `lower(<expr trigram>) LIKE '%'||lower(q)||'%'` (escape `%` dan `_`).
- **Urutan:** selalu ada tie-breaker `id` supaya pagination stabil.
- **Endpoint list:** `GET /api/assets` → `{items, total, page, page_size:20}`.
- **Endpoint summary:** `GET /api/assets/summary` → `{total, available, in_use, needs_attention}`.
  - `needs_attention` = maintenance + damaged + lost.
  - Mengabaikan filter.
  - `archived` hanya dihitung di total.

- [ ] **Step 1: Write failing tests:**
  - `TestSearchMatchesTagNameSerialModelAssignee`: setiap field ditemukan lewat substring huruf kecil.
  - `TestSearchEscapesWildcards`: `q="%"` hanya cocok dengan asset yang namanya mengandung `%`.
  - `TestFilterMultiStatusAndCategory`.
  - `TestFilterLocationOrganization`.
  - `TestSortEachColumnBothDirs`.
  - `TestPagination20`: 45 asset menghasilkan page 3 dengan 5 item dan total 45.
  - `TestInvalidSort400`.
  - `TestSummaryCounts`: memakai status campuran termasuk archived.
  - `TestListCrossTenantNotVisible`.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan. Query dibangun dengan placeholder parameter. Kolom sort dipetakan dari whitelist dan tidak pernah diinterpolasi dari input.
- [ ] **Step 4:** Jalankan test. Expected: PASS.
- [ ] **Step 5:** Commit `feat(api): asset list search/filter/sort and summary`.

---

### Task 7: Maintenance dan Movements

**Files:**
- Create:
  - `api/internal/db/migrations/0005_maintenance_movements.{up,down}.sql`;
  - `api/internal/maintenance/{service.go,handlers.go}`;
  - `api/internal/movement/{service.go,handlers.go}`.
- Test:
  - `api/internal/maintenance/maintenance_test.go`;
  - `api/internal/movement/movement_test.go`.

**Interfaces:**
- **Schema `maintenance_records`:**
  - `id`, `tenant_id`, `asset_id fk cascade`;
  - `date date not null`;
  - `type text check in (preventive,corrective,inspection,other)`;
  - `performed_by text default ''`;
  - `cost numeric(16,2) null check >= 0`;
  - `next_date date null`, `reminder_days int null check >= 0`;
  - `notes text default ''`;
  - `reminder_sent_at timestamptz null`;
  - `created_by_name text`, `created_at`.
- **Schema `movements`:**
  - `id`, `tenant_id`, `asset_id fk cascade`;
  - `from_location_id uuid null fk ON DELETE SET NULL`, `from_location_name text default ''`;
  - `to_location_id uuid not null fk RESTRICT`;
  - `date date not null`;
  - `moved_by text not null`;
  - `reason text default ''`;
  - `created_at`.
- **Endpoint maintenance:**
  - `GET /api/assets/{id}/maintenance` → `{items (urut date desc, created_at desc), next_due:{date,record_id}|null}`. `next_due` diambil dari record terbaru per asset (lihat Task 8) yang punya `next_date`.
  - `POST /api/assets/{id}/maintenance {date,type,performed_by,cost,next_date,reminder_days,notes}`:
    - `reminder_days` default 7 bila `next_date` diisi dan `reminder_days` null;
    - `next_date` < `date` menghasilkan 422 `fields.next_date=before_date`;
    - history `maintenance_added {record_id,type,date}`.
  - `DELETE /api/maintenance/{id}` → 204, dengan history `maintenance_deleted {type,date}`.
  - Tidak ada endpoint edit.
- **Endpoint movement:**
  - `GET /api/assets/{id}/movements` (urut date desc).
  - `POST /api/assets/{id}/movements {to_location_id,date,moved_by,reason}`, dalam satu transaksi:
    - `from` = lokasi asset saat ini (id dan nama snapshot);
    - mengubah `assets.location_id = to`;
    - history `movement_added {from,to,reason}` (nama lokasi). Tidak ada entri `location_changed` terpisah;
    - `to_location_id` sama dengan lokasi saat ini menghasilkan 422 `fields.to_location_id=same_as_current`;
    - `moved_by` kosong diisi `p.Name`.
  - `DELETE /api/movements/{id}` → 204. Lokasi asset tidak berubah. History `movement_deleted`.

- [ ] **Step 1: Write failing tests:**
  - `TestAddMaintenanceDefaultsReminder7`.
  - `TestNextDateBeforeDate422`.
  - `TestDeleteMaintenanceHistory`.
  - `TestMovementUpdatesLocationAndHistory`.
  - `TestDeleteMovementKeepsLocation`.
  - `TestMovementSameLocation422`.
  - `TestCrossTenantMaintenanceMovementDelete404`.
  - `TestMovementToOtherTenantLocation422`.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan test. Expected: PASS.
- [ ] **Step 5:** Commit `feat(api): maintenance records and movements`.

---

### Task 8: Notifikasi dan job reminder

**Files:**
- Create:
  - `api/internal/db/migrations/0006_notifications.{up,down}.sql`;
  - `api/internal/notify/{service.go,reminder.go,scheduler.go,handlers.go}`.
- Modify: `api/cmd/server/main.go` (menjalankan scheduler).
- Test: `api/internal/notify/notify_test.go`

**Interfaces:**
- **Schema `notifications`:**
  - `id bigserial`, `tenant_id`, `user_id fk cascade`;
  - `type text`, `title text`, `body text`;
  - `asset_id uuid null fk ON DELETE CASCADE`;
  - `read_at timestamptz null`, `created_at`;
  - index `(user_id, id desc)`.
- **Bahasa notifikasi:** `title`/`body` disimpan dalam bahasa Indonesia dan Inggris sekaligus sebagai `title_id, title_en, body_id, body_en`. API mengembalikan versi sesuai `p.Locale`.
  - `body_id`: `Maintenance {name} ({tag}) jatuh tempo {dd MMM yyyy}`.
  - `body_en`: `Maintenance for {name} ({tag}) is due {dd MMM yyyy}`.
  - Judul: "Pengingat maintenance" / "Maintenance reminder".
- **`notify.RunReminders(ctx, pool, today civil.Date) (sent int, err error)`.** Untuk setiap maintenance record, ambil record terbaru per asset: `DISTINCT ON (asset_id) ORDER BY asset_id, date desc, created_at desc`. Record dikirimi reminder bila memenuhi semua syarat berikut:
  - `next_date` tidak null dan `reminder_days` tidak null;
  - `reminder_sent_at` masih null;
  - `today >= next_date - reminder_days`.
  - Untuk record yang memenuhi syarat:
    - insert satu notifikasi per user aktif di tenant tersebut;
    - set `reminder_sent_at = now()`, dalam transaksi yang sama.
  - Tenant nonaktif dilewati.
- **`notify.Scheduler{Pool, Loc, Clock func() time.Time}.Start(ctx)`:**
  - menjalankan `RunReminders` sekali saat start;
  - lalu menjalankannya setiap hari pukul 07:00 di `Asia/Jakarta`.
- **Endpoint:**
  - `GET /api/notifications` → `{unread:int, items:[{id,type,title,body,asset_id,read_at,created_at}]}`, 20 terakhir milik user;
  - `POST /api/notifications/{id}/read` → 204. Milik user lain: 404;
  - `POST /api/notifications/read-all` → 204.

- [ ] **Step 1: Write failing tests:**
  - `TestReminderFiresOnDayWindowStart`: next 2026-11-18, H-7, today 2026-11-11 → 1 notifikasi per user aktif, dan user nonaktif tidak dapat.
  - `TestReminderNotBeforeWindow`: today 2026-11-10 → 0.
  - `TestReminderOnlyOnce`: dijalankan dua kali → total tetap sama.
  - `TestReminderOnlyLatestRecordPerAsset`: record lama dengan next_date yang sudah masuk window, ditambah record yang lebih baru tanpa next_date → 0.
  - `TestReminderOverdueStillFiresOnce`: today > next_date dan belum terkirim → terkirim.
  - `TestNotificationLocale`: user dengan locale `en` mendapat `body_en`.
  - `TestMarkReadOtherUser404`.
  - `TestSchedulerNextRun`: fungsi murni `nextRun(now, loc) time.Time`.
    - 06:59 WIB → hari yang sama 07:00.
    - 07:00 WIB → besok 07:00.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan test. Expected: PASS.
- [ ] **Step 5:** Commit `feat(api): in-app notifications and maintenance reminder job`.

---

### Task 9: Endpoint Dashboard

**Files:**
- Create: `api/internal/dashboard/handlers.go`
- Test: `api/internal/dashboard/dashboard_test.go`

**Interfaces:**
- `GET /api/dashboard` mengembalikan:
  - `summary`: sama dengan `/api/assets/summary`.
  - `attention`: `[{id,asset_tag,name,status,location_name,updated_at}]`. Berisi asset dengan status maintenance, damaged, atau lost, urut `updated_at desc`, maksimal 50.
  - `upcoming_maintenance`: `[{asset_id,asset_tag,name,next_date,overdue:bool}]`. Diambil dari record terbaru per asset dengan `next_date <= today+30`, urut `next_date asc`, maksimal 50.
  - `by_category`: `[{key,count}]`, berisi semua 6 kategori termasuk yang 0.
  - `by_status`: `[{key,count}]`, berisi semua 6 status.

- [ ] **Step 1: Write failing test:** `TestDashboardShape`.
  - Fixture: 1 asset available, 1 damaged, 1 lost, 1 archived, dan 1 maintenance record dengan next_date today+5.
  - `attention` memuat 2 asset. `upcoming_maintenance` memuat 1 dengan `overdue=false`.
  - Panjang `by_category` 6.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan test. Expected: PASS.
- [ ] **Step 5:** Commit `feat(api): dashboard endpoint`.

---

### Task 10: PDF Label dan BAST

**Files:**
- Create:
  - `api/internal/pdf/fonts/` (DMSans-Regular/Bold.ttf, SpaceMono-Regular/Bold.ttf, lisensi OFL; di-embed);
  - `api/internal/pdf/{fonts.go,label.go,bast.go,terbilang.go}`;
  - `api/internal/asset/print_handlers.go`.
- Test:
  - `api/internal/pdf/terbilang_test.go`;
  - `api/internal/pdf/pdf_test.go`;
  - `api/internal/asset/print_test.go`.

**Interfaces:**
- **Label:**
  - `pdf.Label(w io.Writer, d pdf.LabelData) error`, dengan `LabelData{QRURL, Tag, Name, Model, Serial, TenantName string}`.
  - Halaman 90×50 mm, latar putih, satu halaman.
  - QR di kiri, sekitar 38 mm.
  - Kolom kanan: Tag (Space Mono Bold), Name, Model, Serial, dan TenantName kecil di bawah.
  - Setiap baris teks dipotong agar muat lebar kolom dengan akhiran "…", diukur memakai `GetStringWidth`.
- **Tanggal dalam kata:** `pdf.Terbilang(n int) string` dan `pdf.TanggalKata(d civil.Date) string`.
  - Contoh 2026-11-11: `"Rabu, tanggal sebelas bulan November tahun dua ribu dua puluh enam"`.
- **BAST:**
  - `pdf.BAST(w io.Writer, d pdf.BASTData) error`.
  - `BASTData` berisi:
    - `Number, TenantName string`;
    - `Date civil.Date`;
    - Pihak Pertama: `FirstName, FirstPosition`;
    - Pihak Kedua: `SecondName, SecondPosition, SecondDepartment`;
    - barang: `Tag, AssetName, Category (label ID), BrandModel, Serial`;
    - `Condition (label ID), Warranty, Notes string`.
  - A4 portrait, isi sesuai §7 Print BAST, seluruhnya berbahasa Indonesia.
  - Label kondisi:
    - `good` → "Baik dan Layak Digunakan";
    - `new_sealed` → "Baru (Segel)";
    - `used_normal` → "Bekas (Normal)".
  - Label kategori ID:
    - `it_equipment` → Peralatan IT;
    - `vehicle` → Kendaraan;
    - `building_facility` → Fasilitas Gedung;
    - `furniture` → Furnitur;
    - `electronics` → Elektronik;
    - `other` → Lainnya.
- **Endpoint:**
  - `POST /api/assets/{id}/label` → `application/pdf`. QR URL = `cfg.AppBaseURL + "/assets/" + id`. History `label_printed`.
  - `GET /api/assets/{id}/bast/defaults` → `{number, date, first_name, first_position, second_name, warranty}`.
    - `warranty` = `"s.d. dd MMMM yyyy"` (bulan Indonesia) bila ada tanggal garansi berakhir. Kalau tidak ada, `""`.
  - `POST /api/assets/{id}/bast {date,first_name,first_position,second_name,second_position,second_department,condition,warranty,notes}` → pdf.
    - `second_name` wajib (422).
    - `condition` harus enum.
    - Nomor BAST memakai `date`.
    - History `bast_printed {number}`.
  - `Content-Disposition` memakai nama file `label-<tag>.pdf` / `<number>.pdf`.

- [ ] **Step 1: Write failing tests:**
  - **Terbilang:** 0 "nol", 11 "sebelas", 21 "dua puluh satu", 100 "seratus", 1000 "seribu", 2026 "dua ribu dua puluh enam".
  - `TanggalKata(2026-11-11)` sesuai contoh.
  - `TestLabelPageSize90x50`: parse output dan cek `/MediaBox` ≈ [0 0 255.12 141.73] pt.
  - `TestLabelLongUnicodeName`: nama 200 karakter dengan `é&"` tidak error, PDF valid (diawali `%PDF`), tetap satu halaman.
  - `TestBASTRequiresSecondName422`.
  - `TestBASTNumberAndHistory`: `detail.number == "BAST-AST-8F2K1Q9Z-20261111"`.
  - `TestLabelCrossTenant404`.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan. Font TTF di-embed lewat `go:embed` dan didaftarkan dengan `AddUTF8FontFromBytes`.
- [ ] **Step 4:** Jalankan test. Expected: PASS. Simpan contoh output ke `/tmp/label.pdf` dan `/tmp/bast.pdf`, lalu periksa secara visual dengan membaca PDF tersebut.
- [ ] **Step 5:** Commit `feat(api): label and BAST pdf`.

---

### Task 11: Importer, bagian 1: header, nilai, dan pembaca file

**Files:**
- Create: `api/internal/importer/{headers.go,values.go,reader.go}`
- Test:
  - `api/internal/importer/headers_test.go`;
  - `api/internal/importer/values_test.go`;
  - `api/internal/importer/reader_test.go`;
  - `testdata/` (dibuat test lewat excelize dan string CSV, tanpa file biner yang di-commit kecuali `sample.xls`).

**Interfaces:**
- **Header:**
  - `importer.NormalizeHeader(s string) string`: lowercase, hapus semua karakter non huruf dan angka. Contoh: "Nama_Asset " → "namaasset".
  - `importer.FieldAliases map[string][]string`: alias dari §10, ditambah header Snipe-IT. Diambil dari spec persis, plus `Asset Name`, `Name`, `Checked Out To`, `Company`, `Serial`, `Warranty`, `Purchase Cost`, `Purchase Date`, `Order Number`, `Default Location`, `Location`.
  - `importer.MapHeaders(row []string) (cols map[string]int, unknown []string)`. Kalau satu field cocok dengan dua kolom, ambil kolom pertama dan kolom kedua masuk `unknown`.
  - `importer.FindHeaderRow(rows [][]string) (idx int, ok bool)`: baris pertama dalam 10 teratas yang memiliki ≥ 1 header dikenali, dengan prioritas baris yang memiliki `name`.
- **Nilai:**
  - `importer.ParseCategory(s) (code string, recognized bool)`. Yang tidak dikenali menjadi `"other", false`. Alias ID dan EN minimal:
    - **IT:** peralatan it, it, it equipment, komputer, laptop, computer;
    - **vehicle:** kendaraan, mobil, motor, vehicle;
    - **building:** fasilitas gedung, gedung, bangunan, building facility, building;
    - **furniture:** furnitur, furniture, mebel, perabot;
    - **elektronik:** elektronik, electronics, electronic;
    - **lainnya:** lainnya, lain-lain, other.
  - `importer.ParseStatus(s) (code string, recognized bool)`. String kosong menjadi `"available", true`. Yang tidak dikenali menjadi `"available", false`. Alias:
    - **available:** available, tersedia, ready to deploy, ready, siap;
    - **in_use:** in use, digunakan, dipakai, terpakai, deployed;
    - **maintenance:** maintenance, perbaikan, servis, service, pending;
    - **damaged:** damaged, rusak, broken, un-deployable, undeployable;
    - **lost:** lost, hilang, stolen, dicuri;
    - **archived:** archived, diarsipkan, arsip.
  - `importer.ParseDate(s string) (*civil.Date, error)`:
    - serial Excel (angka 1–80000, epoch 1899-12-30);
    - format `dd/mm/yyyy`, `dd-mm-yyyy`, `d/m/yyyy`, `yyyy-mm-dd`;
    - "11 November 2026", "11 Nov 2026", "11 Agustus 2026", "11 Agu 2026";
    - string kosong menjadi nil.
  - `importer.ParseMoney(s) (*decimal.Decimal, error)`:
    - "Rp 12.500.000" → 12500000;
    - "12,500,000.00" → 12500000;
    - "12.500.000,50" → 12500000.50;
    - "12500000" → 12500000;
    - "Rp. 1.500" → 1500.
    - Aturan: kalau ada `,` dan `.`, pemisah desimal adalah yang terakhir muncul. Kalau hanya ada `.` dan ada grup 3 digit setelahnya, berarti pemisah ribuan.
  - `importer.ParseWarrantyMonths(s) (*int, error)`: "12" → 12, "12 bulan", "12 months" → 12, "1 tahun"/"2 years" → 12/24.
- **Pembaca file:**
  - `importer.ReadFile(name string, r io.Reader) ([][]string, error)`, dipilih berdasarkan ekstensi (`.csv|.xlsx|.xls`, case-insensitive). Ekstensi lain menghasilkan `ErrUnsupportedFormat`.
  - CSV:
    - strip BOM;
    - pemisah dideteksi dari baris pertama yang tidak kosong: jumlah `;` > `,` berarti `;`;
    - `LazyQuotes`, `FieldsPerRecord=-1`.
  - xlsx: sheet pertama, nilai terformat (`GetRows`). Tanggal yang diformat sebagai tanggal diambil dalam bentuk mentah lewat `excelize.Options{RawCellValue:true}` supaya `ParseDate` menerima serial.
  - Semua sel di-`TrimSpace`.

- [ ] **Step 1: Write failing tests** (table-driven):
  - Semua contoh di atas.
  - `FindHeaderRow`: baris 0 "LAPORAN ASSET PT X", baris 1 kosong, baris 2 header → idx 2.
  - `MapHeaders`: "Harga Pembelian" → purchase_cost; "Kolom Aneh" → unknown.
  - `ReadFile`:
    - CSV dengan BOM dan `;`;
    - xlsx buatan excelize dengan tanggal bertipe date;
    - `.txt` → `ErrUnsupportedFormat`.
- [ ] **Step 2:** Jalankan `go test ./internal/importer/...`. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan test. Expected: PASS.
- [ ] **Step 5:** Commit `feat(api): import header and value parsing`.

---

### Task 12: Importer, bagian 2: job, penerapan baris, hasil, laporan, dan template

**Files:**
- Create:
  - `api/internal/db/migrations/0007_imports.{up,down}.sql`;
  - `api/internal/importer/{job.go,apply.go,report.go,template.go,handlers.go}`.
- Test: `api/internal/importer/apply_test.go`, `api/internal/importer/job_test.go`

**Interfaces:**
- **Schema `imports`:**
  - `id uuid`, `tenant_id`, `user_id`, `file_name`;
  - `status text check in (queued,running,done,failed)`;
  - `total_rows int`, `processed_rows int`;
  - `result jsonb`, `error text default ''`;
  - `created_at`, `finished_at`.
  - Saat server start, `running|queued` diubah menjadi `failed` dengan `error='server_restarted'`.
- **`importer.Result`:**
  - jumlah: `Created, Updated, Unchanged, Skipped int`;
  - `UnknownColumns []string`;
  - `UnmatchedOrganizations []NameCount{Name string; Rows int}`;
  - `UnrecognizedValues []ValueIssue{Field, Value string; Rows int}`, untuk status dan kategori;
  - `SkippedRows []RowIssue{Row int; Reason string}`, dengan `Reason` ∈ `missing_name|duplicate_tag_in_file|invalid_value:<field>`;
  - `CreatedSuppliers []string`, `CreatedLocations []string`.
  - JSON memakai snake_case.
- **`importer.ApplyRow(ctx, tx, p auth.Principal, cols map[string]int, row []string, rowNum int, res *Result, seenTags map[string]int) error`.** Aturan §10:
  - **Lewati baris:**
    - baris yang semua selnya kosong tidak dihitung;
    - name kosong menjadi `missing_name`;
    - tag (lowercase) sudah ada di `seenTags` menjadi `duplicate_tag_in_file`;
    - tanggal, uang, atau garansi tidak bisa di-parse menjadi `invalid_value:<field>`, dan baris dilewati.
  - **Tag ada di DB** (tanpa membedakan huruf besar-kecil):
    - update hanya kolom yang ada di `cols` dan berisi nilai;
    - kalau tidak ada nilai yang berbeda, `Unchanged++`;
    - kalau ada yang berbeda, `Updated++` dan history `asset_imported_updated {fields:[...]}`.
  - **Tag tidak ada atau kosong:**
    - buat asset baru, dengan tag dari `GenerateTag` bila kosong;
    - `Created++`;
    - history `asset_imported_created`.
  - **Master data:**
    - Organization dicocokkan dengan `FindByName`. Kalau tidak ditemukan, dikosongkan dan masuk `UnmatchedOrganizations`.
    - Supplier dan Location memakai `FindOrCreateByName`. Nama baru dicatat di `Created*`.
  - Kategori dan status yang tidak dikenali masuk `UnrecognizedValues`, dan baris tetap masuk.
  - Kategori kosong pada asset baru menjadi `other`, dan dicatat sebagai tidak dikenali dengan value `""`.
  - Setiap baris memakai savepoint sendiri, sehingga error satu baris tidak membatalkan import.
- **`importer.Run(ctx, pool, jobID)`:**
  - membaca baris yang sudah di-parse dan disimpan di memori job;
  - update `processed_rows` setiap 25 baris;
  - status akhir `done` dengan `result`.
- **Endpoint:**
  - `POST /api/imports` (multipart field `file`):
    - lebih dari 10 MB menghasilkan 413 `file_too_large`;
    - ekstensi selain csv, xlsx, atau xls menghasilkan 422 `unsupported_format`;
    - header tidak ditemukan menghasilkan 422 `header_not_found`;
    - lebih dari 10.000 baris data menghasilkan 422 `too_many_rows`;
    - kalau valid: 202 `{id}`, lalu job berjalan di goroutine.
  - `GET /api/imports/{id}` → `{id,status,total_rows,processed_rows,result,error}`. Tenant lain: 404.
  - `GET /api/imports/{id}/report.csv` → CSV berisi kolom `row,reason` untuk skipped, diikuti bagian unmatched dan unrecognized. Header bagian berbahasa Inggris.
  - `GET /api/imports/template.xlsx` → satu sheet. Header kanonik berbahasa Inggris (urutan field §5) dan satu baris contoh.
- **Header kanonik** `importer.CanonicalHeaders []string`: `Asset Tag, Asset Name, Category, Manufacturer, Model, Serial Number, Status, Location, Assigned To, Organization, Supplier, Order Number, Purchase Date, Purchase Cost, Warranty Months, Notes`. Konstanta ini dipakai juga oleh export.

- [ ] **Step 1: Write failing tests** (`apply_test.go` memakai DB):
  - `TestImportCreatesUpdatesUnchanged`: import 3 baris dua kali; run kedua menghasilkan `Unchanged=3`. Mengubah 1 sel menghasilkan `Updated=1`.
  - `TestEmptyCellDoesNotOverwrite`.
  - `TestTagCaseInsensitiveUpdate`: DB berisi `AST-001`, file berisi `ast-001` → Updated, bukan Created.
  - `TestDuplicateTagInFileSkipped`.
  - `TestMissingNameSkippedAndBlankRowsIgnored`.
  - `TestUnknownOrganizationBlankedAndReported`: `Rows` dihitung per nama.
  - `TestSupplierAndLocationAutoCreated`.
  - `TestUnknownStatusBecomesAvailableAndReported`.
  - `TestSnipeITExport`: header Snipe-IT, status "Ready to Deploy" dan "Deployed", "Checked Out To" → `assigned_to`.
  - `TestMessyIndonesianSpreadsheet`: CSV `;` dengan BOM, judul di baris 1, header "Nama Asset;Kode Aset;Kategori;Lokasi;Harga Pembelian;Tanggal Beli", harga "Rp 12.500.000", tanggal "11/11/2026" → Created=1 dengan nilai benar.
  - `TestImportCrossTenantStatus404` (job_test).
  - `TestUploadLimits`: 11 MB → 413; 10.001 baris → 422 `too_many_rows`; `.pdf` → 422.
  - `TestJobProgressReachesDone`: poll sampai `done`, dengan `processed_rows == total_rows`.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan test. Expected: PASS.
- [ ] **Step 5:** Commit `feat(api): import jobs, results, report and template`.

---

### Task 13: Export CSV, Excel, dan PDF

**Files:**
- Create: `api/internal/export/{csv.go,xlsx.go,pdf.go,handlers.go}`, `api/internal/pdf/assetlist.go`
- Test: `api/internal/export/export_test.go`

**Interfaces:**
- **Endpoint:** `GET /api/assets/export?format=csv|xlsx|pdf&<filter Task 6>`.
  - Memakai `asset.ParseListFilter` dan `Service.List(…, limit=0)`, dengan urutan yang sama.
  - Nama file `assets-YYYYMMDD.<ext>`.
  - Format lain menghasilkan 400 `invalid_format`.
- **CSV/XLSX:**
  - Header `importer.CanonicalHeaders`.
  - Nilai enum ditulis sebagai label bahasa Inggris (misal "IT Equipment", "In Use") supaya bisa di-import ulang.
  - Tanggal `yyyy-mm-dd`. Harga berupa angka tanpa format.
  - Location, Organization, dan Supplier ditulis sebagai nama.
  - CSV memakai UTF-8 dengan BOM dan pemisah `,`.
- **PDF:**
  - `pdf.AssetList(w, d pdf.AssetListData{TenantName, PrintedAt, FilterSummary string; Rows []asset.ListItem; Locale string})`.
  - A4 landscape.
  - Kolom Tag, Name, Category, Model, Serial, Status, Location, Assigned To. Header tabel diulang di setiap halaman.
  - Label mengikuti locale user.

- [ ] **Step 1: Write failing tests:**
  - `TestExportRespectsFilterAllPages`: 45 asset, 30 di antaranya status `in_use`; export `status=in_use` menghasilkan 30 baris data, bukan 20.
  - `TestCSVRoundTripImport`: export lalu import ke tenant baru menghasilkan `Created` == jumlah, dan field-nya sama.
  - `TestXLSXHeaders`.
  - `TestPDFValid`.
  - `TestExportCrossTenantExcluded`.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan test. Expected: PASS.
- [ ] **Step 5:** Commit `feat(api): export csv/xlsx/pdf`.

---

### Task 14: Scaffold web, theme StarChart, i18n, login, dan app shell

**Files:**
- Create:
  - project `web/` (create-next-app TS, Tailwind v4, src dir), lalu `npx shadcn@latest init` dan tambahkan komponen:
    - aksi dan form: button, input, label, select, checkbox, radio-group, textarea, form, popover, command, calendar;
    - overlay: dialog, alert-dialog, sheet, dropdown-menu, tooltip, sonner;
    - tampilan data: card, badge, tabs, table, skeleton, progress, pagination, separator.
  - `web/src/app/globals.css`: token kedua tema dari §3.
  - `web/src/app/layout.tsx`: font Fredoka, DM Sans, dan Space Mono via `next/font/google`; ThemeProvider; NextIntlClientProvider; QueryClientProvider.
  - `web/src/i18n/request.ts`, `web/src/messages/{id,en}.json`.
  - `web/src/lib/{api.ts,types.ts,format.ts,brand.ts,safe-next.ts}`.
  - `web/middleware.ts`.
  - `web/src/app/(auth)/login/page.tsx`.
  - `web/src/app/(app)/layout.tsx`, `web/src/components/shell/{sidebar.tsx,user-menu.tsx,mobile-nav.tsx}`.
  - `web/src/app/(app)/profile/page.tsx`.
  - `web/next.config.ts`: rewrite `/api/:path*` → `http://localhost:8080/api/:path*` hanya saat dev.
- Test:
  - `web/src/lib/format.test.ts`;
  - `web/src/lib/safe-next.test.ts`;
  - `web/src/lib/api.test.ts`.

**Interfaces:**
- **Klien API:**
  - `api<T>(path, init?)`: menambahkan `X-CSRF-Token` dari cookie `csrf_token` pada method non-GET. Bila respons 401, redirect ke `/login?next=<path sekarang>`. Bila error, melempar `ApiError{status, code, fields}`.
  - `apiBlob(path, init?) → Blob`: dipakai untuk PDF dan file.
- **Format:**
  - `formatRupiah(v: string|number|null) → "Rp 12.500.000" | "—"`;
  - `formatDate(iso, locale) → "11 Nov 2026"` (id) / `"11 Nov 2026"` (en). Nama bulan Indonesia memakai "Agu", "Okt", "Des".
- **`safeNext(next: string|null) → string`:** hanya menerima path yang diawali `/` tetapi bukan `//`, dan tanpa `\`. Selain itu, mengembalikan `"/dashboard"`.
- **Middleware Next.js:** route `(app)` tanpa cookie `session` di-redirect ke `/login?next=<pathname+search>`. Route `/admin/(panel)` tanpa cookie sesi di-redirect ke `/admin/login`.
- **Locale:** diambil dari cookie `locale` (`id` default). Saat login dan saat user mengganti bahasa, frontend menulis cookie ini dari `Me.locale` lalu menjalankan `router.refresh()`. Tema diatur `next-themes` dari `Me.theme`. Kedua perubahan juga di-`PATCH /api/me`.
- **Shell:**
  - **Sidebar desktop (≥ 1024px):** logo bintang lucide `Star` dan teks "AssetKu", menu Dashboard, Assets, Locations, Organizations, Suppliers dengan ikon lucide `LayoutDashboard, Package, MapPin, Building2, Truck`.
  - **Layar < 1024px:** sidebar diganti Sheet yang dibuka dari tombol `Menu`.
  - **Header:** tempat lonceng (diisi Task 20) dan menu user. Menu user berisi nama, Profil, Ganti password, Bahasa (Indonesia/English), Tema (Gelap/Terang), dan Keluar.
- **Pesan:** semua teks UI lewat `useTranslations`. Tidak ada teks hard-coded di komponen. Key dikelompokkan per fitur, misalnya `nav.*`, `auth.*`, `assets.*`, `status.*`, `category.*`, `errors.<code>`.
- **Login:** form email dan password dengan tombol Primary (Star). Setelah sukses, diarahkan ke `safeNext(next)`. Error `invalid_credentials` dan `rate_limited` ditampilkan sebagai teks terjemahan.

- [ ] **Step 1:** Scaffold project, init shadcn, dan tulis token tema di `globals.css`. Map token shadcn ke StarChart:
  - **Gelap:**
    - `--background` #1E1B4B, `--card` #2E2A6E, `--popover` #3D3890, `--input` #141136, `--border` #3D3890;
    - `--primary` #FDE047 dengan `--primary-foreground` #1E1B4B;
    - `--secondary` / `--ring` #A78BFA;
    - `--destructive` #F87171.
  - **Terang:** nilai §3. Radius dasar `--radius: 1rem`.
  - **Utility:** `glow-nebula-{sm,md,lg}` dan `glow-star-{sm,md,lg}` sebagai utility Tailwind `@utility` sesuai spesifikasi StarChart.
  - **Variant Button:** di-override menjadi Primary (Star), Secondary (Nebula outline 2px), Ghost, dan Destructive. Semua pill, dengan size sm/md/lg sesuai spesifikasi.
- [ ] **Step 2: Write failing tests** (Vitest):
  - `formatRupiah("12500000.00") === "Rp 12.500.000"`, dan `formatRupiah(null) === "—"`.
  - `formatDate("2026-08-11","id") === "11 Agu 2026"`.
  - `safeNext("/assets/x?y=1") === "/assets/x?y=1"`. `safeNext("//evil.com")`, `safeNext("https://evil.com")`, dan `safeNext("/\\evil")` semuanya `"/dashboard"`.
  - `api` mengirim header CSRF pada POST.
  - `api` me-redirect ke `/login?next=%2Fassets%2F1` saat 401 (mock `window.location`).
- [ ] **Step 3:** Jalankan `cd web && npx vitest run`. Expected: FAIL.
- [ ] **Step 4:** Implementasikan lib, middleware, login, shell, dan halaman profil. Profil berisi edit nama dan jabatan, serta form ganti password.
- [ ] **Step 5:** Jalankan `npx vitest run && npm run build`. Expected: PASS. Lalu jalankan API dan web dev. Buat tenant dan user lewat `go test`-style seed, atau lewat curl admin setelah Task 3. Pastikan:
  - login berhasil;
  - ganti bahasa mengubah teks;
  - ganti tema berpindah gelap/terang;
  - di lebar 360px, sidebar menjadi Sheet.
- [ ] **Step 6:** Commit `feat(web): scaffold, StarChart theme, i18n, auth, shell`.

---

### Task 15: Halaman master data (Locations, Organizations, Suppliers)

**Files:**
- Create:
  - `web/src/components/master/{master-page.tsx,master-form-dialog.tsx}`;
  - `web/src/app/(app)/{locations,organizations,suppliers}/page.tsx`.
- Test: `web/src/components/master/master-page.test.tsx`

**Interfaces:**
- **Komponen:** `<MasterPage kind="locations"|"organizations"|"suppliers" />` dipakai ketiga halaman. Field per kind:
  - locations: name, description;
  - organizations: name;
  - suppliers: name, contact_name, phone, email, address.
- **Daftar:**
  - Desktop: tabel dengan kolom field utama, `asset_count`, dan menu aksi Edit/Hapus.
  - HP: list.
  - Ada kotak cari.
- **Tambah dan Edit:** form di Dialog.
- **Hapus:** memakai AlertDialog. Error 409 `in_use` ditampilkan sebagai toast `errors.in_use` dengan count, misalnya "Masih dipakai oleh 2 asset".
- **Error field:** `fields.name=taken` ditampilkan di bawah input.

- [ ] **Step 1: Write failing tests** (mock `api`):
  - render daftar;
  - submit tambah memanggil POST;
  - respons `taken` menampilkan pesan error di field;
  - hapus dengan respons 409 menampilkan toast count.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan `npx vitest run`. Expected: PASS. Lalu cek manual ketiga halaman di 360px dan desktop.
- [ ] **Step 5:** Commit `feat(web): master data pages`.

---

### Task 16: Halaman Daftar Asset

**Files:**
- Create:
  - `web/src/lib/filters.ts`;
  - `web/src/app/(app)/assets/page.tsx`;
  - `web/src/components/assets/{summary-cards.tsx,asset-filters.tsx,asset-table.tsx,asset-card-list.tsx,status-chip.tsx,row-actions.tsx,delete-asset-dialog.tsx}`.
- Test:
  - `web/src/lib/filters.test.ts`;
  - `web/src/components/assets/summary-cards.test.tsx`.

**Interfaces:**
- **State filter (`filters.ts`):**
  - Tipe `AssetFilters{q, category: string[], status: string[], location_id?, organization_id?, sort, dir, page}`.
  - Fungsi `parseFilters(URLSearchParams)`, `toSearchParams(f)` (hanya menyertakan nilai non-default), dan `cardFromStatuses(status[]) → "total"|"available"|"in_use"|"attention"|null`.
  - Konstanta `ATTENTION_STATUSES = ["maintenance","damaged","lost"]`.
  - State disimpan di URL lewat `router.replace`. Mengubah filter mereset `page` ke 1.
- **`<StatusChip status>`:** warna sesuai tabel §3, berbentuk pill dengan fill 20%.
- **Tata letak halaman:**
  - **Header:** judul, tombol Import dan Export (Secondary; diisi Task 19), dan Add Asset (Primary Star, ikon `Plus`). Di bawah 768px, Import dan Export pindah ke DropdownMenu `MoreHorizontal`.
  - **SummaryCards:** grid 4 kolom, menjadi 2×2 di bawah 768px. Kartu aktif mendapat border nebula dan `glow-nebula-md`. Klik kartu mengatur `status`, dan klik Total menghapus filter status.
  - **Filter:**
    - Input cari dengan ikon `Search` dan debounce 300 ms;
    - Category dan Status multi-select (Popover + Command dengan checkbox);
    - Location dan Organization berupa select;
    - tombol Reset (Ghost).
  - **Tabel (≥ 768px):**
    - kolom sesuai §6;
    - header bisa diklik untuk sort dengan ikon `ArrowUpDown/ArrowUp/ArrowDown`;
    - klik baris membuka `/assets/{id}`;
    - kolom aksi berisi DropdownMenu (View Details, Edit, Delete). Kolom aksi menghentikan propagasi klik.
  - **Daftar kartu (< 768px):** tag mono, nama, status chip, dan menu aksi.
  - **Pagination:** teks "1–20 dari 1.248" / "1–20 of 1,248", dengan tombol prev/next dan nomor halaman.
  - **Keadaan kosong (§6):**
    - total = 0: tampil ajakan Import dan Add;
    - hasil filter kosong: tampil pesan dan tombol Reset.

- [ ] **Step 1: Write failing tests:**
  - `filters.test.ts`:
    - round-trip parse dan toSearchParams;
    - `cardFromStatuses(["lost","maintenance","damaged"]) === "attention"`;
    - mengubah filter mereset page.
  - `summary-cards.test.tsx`: klik "Needs Attention" memanggil `onChange` dengan 3 status. Kartu aktif memiliki `aria-pressed="true"`.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan test. Expected: PASS. Cek manual dengan 45 asset seed:
  - sort, filter, dan pagination berjalan;
  - URL bisa di-bookmark;
  - tombol back mengembalikan filter sebelumnya;
  - tampilan di 360px benar.
- [ ] **Step 5:** Commit `feat(web): asset list page`.

---

### Task 17: Form Tambah dan Edit Asset

**Files:**
- Create:
  - `web/src/components/assets/{asset-form.tsx,master-combobox.tsx,assignee-input.tsx,asset-schema.ts}`;
  - `web/src/app/(app)/assets/new/page.tsx`, `web/src/app/(app)/assets/[id]/edit/page.tsx`.
- Test:
  - `web/src/components/assets/asset-schema.test.ts`;
  - `web/src/components/assets/asset-form.test.tsx`.

**Interfaces:**
- **Skema validasi:** `assetSchema` (zod) mencerminkan validasi API. Wajib: `name`, `category`, `status` (default `available`). Field angka ≥ 0.
- **Fungsi:** `warrantyExpiresPreview(purchase_date?: string, months?: number) → string|null`, dengan aturan clamp akhir bulan yang sama seperti backend.
- **`<MasterCombobox kind value onChange>`:** Popover + Command dengan pencarian ke `GET /api/{kind}?q=`. Ada item terakhir "+ Tambah \"{q}\"" yang memanggil POST lalu langsung memilih hasilnya.
- **`<AssigneeInput>`:** input dengan saran dari `GET /api/assets/assignees?q=` (debounce 200 ms). Tetap menerima teks bebas.
- **Tata letak form:**
  - 4 Card bagian sesuai §9;
  - satu kolom di bawah 768px, dua kolom di atasnya;
  - tombol Simpan (Primary Star) dan Batal (Ghost);
  - error field dari API (`fields.*`) dipetakan ke field form.
- **Peringatan keluar:** `beforeunload` dan konfirmasi saat navigasi internal bila form `isDirty`.
- **Setelah simpan:** diarahkan ke `/assets/{id}` dengan toast sukses.

- [ ] **Step 1: Write failing tests:**
  - schema menolak name kosong;
  - `warrantyExpiresPreview("2026-01-31", 1) === "2026-02-28"`;
  - form menampilkan error `asset_tag=taken` dari API di bawah field Asset Tag;
  - submit tanpa tag tidak mengirim `asset_tag` (atau mengirim string kosong).
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan test. Expected: PASS. Cek manual alur tambah dan edit, termasuk "+ Tambah" lokasi baru dari combobox.
- [ ] **Step 5:** Commit `feat(web): asset add/edit form`.

---

### Task 18: Halaman Detail Asset

**Files:**
- Create:
  - `web/src/app/(app)/assets/[id]/page.tsx`;
  - `web/src/components/assets/detail/` berisi:
    - `asset-header.tsx`, `info-tab.tsx`, `history-tab.tsx`, `history-text.ts`;
    - `maintenance-tab.tsx`, `maintenance-dialog.tsx`;
    - `movements-tab.tsx`, `movement-dialog.tsx`;
    - `bast-dialog.tsx`, `download.ts`.
- Test:
  - `web/src/components/assets/detail/history-text.test.ts`;
  - `web/src/components/assets/detail/bast-dialog.test.tsx`.

**Interfaces:**
- **Header:**
  - QR (`qrcode.react` `QRCodeSVG`, value `window.location.origin + "/assets/" + id`) dengan latar putih dan padding.
  - Nama (Fredoka h2), StatusChip, tag mono, kategori, serial, manufacturer, model, lokasi, dan pemegang.
- **Tombol header:**
  - Generate Label (`Tag`), Print BAST (`FileText`), Edit (`Pencil`), Delete (`Trash2`, Destructive). Di bawah 768px, semua tombol masuk DropdownMenu.
  - `download.ts` `downloadBlob(blob, filename)` dipakai untuk PDF.
  - Setelah label atau BAST dibuat, query history di-invalidate.
- **Tab** (shadcn Tabs; di HP bisa di-scroll horizontal):
  - **Information / Informasi:** 4 bagian.
    - Garansi berakhir ditampilkan dengan badge "Expired"/"Kedaluwarsa" bila `warranty_expired`.
    - Harga memakai `formatRupiah`.
  - **History / Riwayat:**
    - `historyText(entry, t) → string` membuat kalimat yang mudah dibaca per event, memakai key `history.<event>` dengan parameter `{actor, from, to, fields, number, type, date, reason}`.
    - Nama field di `details_updated` diterjemahkan.
    - Ikon per event dari lucide.
    - Tombol "Muat lagi" memakai `next_before`.
  - **Maintenance:**
    - Kartu "Maintenance berikutnya" bila ada `next_due`.
    - Daftar maintenance dan dialog tambah (field §5, `reminder_days` default 7 muncul saat `next_date` diisi).
    - Hapus memakai AlertDialog.
  - **Movements / Perpindahan:**
    - Daftar movement dan dialog "Log Movement": `to_location_id` lewat MasterCombobox locations, tanggal default hari ini, `moved_by` default nama user, dan alasan.
    - Setelah simpan, query asset di-invalidate supaya lokasi di header ikut berubah.
    - Hapus memakai AlertDialog dengan teks bahwa lokasi asset tidak dikembalikan.
- **BAST Dialog:**
  - Prefill dari `GET /bast/defaults`.
  - Nomor read-only dan diperbarui saat tanggal diganti, dengan format `BAST-${tag}-${yyyymmdd}`.
  - Kondisi berupa RadioGroup 3 opsi.
  - Submit mengunduh PDF.
- **Delete:** AlertDialog yang menyebut nama dan tag. Setelah sukses, diarahkan ke `/assets`.
- **Asset tidak ditemukan:** 404 atau asset milik tenant lain menampilkan state "Asset tidak ditemukan".

- [ ] **Step 1: Write failing tests:**
  - `historyText`:
    - `status_changed {from:"available",to:"in_use"}` dengan locale id → `"Budi mengubah status dari Tersedia ke Digunakan"`;
    - `movement_added` → mengandung nama kedua lokasi.
  - `bast-dialog`: mengganti tanggal ke 2026-12-01 membuat nomor menjadi `BAST-AST-8F2K1Q9Z-20261201`, dan submit tanpa nama Pihak Kedua menampilkan error.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan test. Expected: PASS. Cek manual:
  - unduh label, lalu periksa PDF-nya;
  - unduh BAST;
  - tambah dan hapus maintenance;
  - log movement mengubah lokasi di header;
  - History terisi.
- [ ] **Step 5:** Commit `feat(web): asset detail page with tabs, label and BAST`.

---

### Task 19: Dialog Import dan menu Export

**Files:**
- Create: `web/src/components/import/{import-dialog.tsx,import-result.tsx}`, `web/src/components/assets/export-menu.tsx`
- Modify: `web/src/app/(app)/assets/page.tsx`
- Test: `web/src/components/import/import-dialog.test.tsx`

**Interfaces:**
- **Import Dialog, langkah 1:**
  - drop zone dengan input file `accept=".xlsx,.xls,.csv"`;
  - validasi ukuran 10 MB di klien;
  - tautan "Unduh template" ke `/api/imports/template.xlsx`.
- **Langkah 2:**
  - Progress (shadcn) bernilai `processed_rows/total_rows`, di-poll setiap 1 detik dari `GET /api/imports/{id}`.
  - Dialog boleh ditutup. Job id disimpan di `sessionStorage` dan polling dilanjutkan dengan toast saat selesai.
- **Langkah 3 (`<ImportResult>`):**
  - 4 angka: Baru, Diperbarui, Tidak berubah, Dilewati. Saat selesai, ada satu efek `glow-star-md` sebagai "achievement".
  - Daftar yang bisa dilipat (collapsible): kolom tidak dikenali, organization tidak cocok (nama × jumlah baris), nilai tidak dikenali, supplier dan lokasi baru, serta baris dilewati (nomor baris dan alasan terjemahan).
  - Tombol unduh laporan CSV.
  - Saat ditutup, query asset dan summary di-invalidate.
- **Error upload:** `unsupported_format`, `file_too_large`, `header_not_found`, dan `too_many_rows` ditampilkan sebagai teks terjemahan di langkah 1.
- **`<ExportMenu filters>`:**
  - DropdownMenu berisi CSV, Excel, dan PDF.
  - Memanggil `apiBlob("/api/assets/export?format=…&"+toSearchParams(filters))` lalu `downloadBlob`.
  - Selama berjalan, tombol menampilkan spinner.

- [ ] **Step 1: Write failing tests** (mock api, fake timers):
  - upload memicu polling;
  - progress 50% ditampilkan;
  - saat `done`, angka hasil tampil;
  - error `header_not_found` menampilkan pesan terjemahan;
  - file 11 MB ditolak tanpa request ke server.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan test. Expected: PASS. Cek manual:
  - import file contoh berantakan (CSV `;` + judul);
  - lalu export hasil filter ke ketiga format.
- [ ] **Step 5:** Commit `feat(web): import dialog with progress and export menu`.

---

### Task 20: Dashboard dan lonceng notifikasi

Pakai skill `dataviz` untuk grafik.

**Files:**
- Create:
  - `web/src/app/(app)/dashboard/page.tsx`;
  - `web/src/components/dashboard/{attention-list.tsx,upcoming-maintenance.tsx,composition-charts.tsx}`;
  - `web/src/components/shell/notification-bell.tsx`.
- Modify: `web/src/app/(app)/layout.tsx`
- Test: `web/src/components/shell/notification-bell.test.tsx`

**Interfaces:**
- **Dashboard:**
  - SummaryCards (dipakai ulang dari Task 16). Klik kartu membuka `/assets?status=…`.
  - "Perlu Perhatian": list StarChart berisi tag, nama, status, lokasi, dan waktu relatif. Klik membuka detail.
  - "Maintenance Mendatang": baris overdue diberi chip Warning/Error.
  - Grafik komposisi per kategori (bar horizontal) dan per status (bar, warna status §3), mengikuti tema.
  - Responsif: 1 kolom di HP, 2 kolom di desktop.
- **Lonceng:**
  - Ikon `Bell` dengan badge jumlah belum dibaca (≥ 100 ditampilkan "99+").
  - Data di-poll setiap 60 detik dan saat window focus.
  - Popover berisi 20 notifikasi. Klik notifikasi menandai dibaca (POST read) lalu membuka `/assets/{asset_id}`.
  - Ada aksi "Tandai semua dibaca".

- [ ] **Step 1: Write failing tests:**
  - badge menampilkan "3";
  - unread 150 ditampilkan "99+";
  - klik item memanggil `/read` lalu `router.push`;
  - "Tandai semua dibaca" memanggil `read-all`.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan test. Expected: PASS. Cek manual dashboard di kedua tema dan di lebar 360px.
- [ ] **Step 5:** Commit `feat(web): dashboard and notification bell`.

---

### Task 21: Panel Super Admin (web)

**Files:**
- Create:
  - `web/src/app/admin/(auth)/login/page.tsx`;
  - `web/src/app/admin/(panel)/layout.tsx`;
  - `web/src/app/admin/(panel)/tenants/page.tsx`, `web/src/app/admin/(panel)/tenants/[id]/page.tsx`;
  - `web/src/components/admin/{tenant-dialog.tsx,user-dialog.tsx,reset-password-dialog.tsx}`.
- Test: `web/src/components/admin/user-dialog.test.tsx`

**Interfaces:**
- **Login admin:** memakai `/api/admin/auth/*`. Semua endpoint admin lain dipanggil dari panel ini.
- **Halaman Tenants:**
  - tabel berisi nama, status aktif, jumlah user, jumlah asset, dan tanggal dibuat;
  - tombol tambah;
  - toggle aktif dengan konfirmasi.
- **Halaman tenant:**
  - ubah nama tenant;
  - tabel user dengan tombol tambah user (nama, email, jabatan, password awal);
  - menu per user: edit, nonaktifkan/aktifkan, reset password.
- **Desain:** StarChart yang sama, dengan header tersendiri bertuliskan "AssetKu Admin". Panel admin juga mengikuti bahasa dari cookie dan tema.

- [ ] **Step 1: Write failing test:**
  - user-dialog menampilkan error `email=taken`;
  - password < 8 karakter ditolak di klien.
- [ ] **Step 2:** Jalankan test. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan test. Expected: PASS. Cek manual:
  - buat tenant dan user lewat panel;
  - login sebagai user di jendela incognito.
- [ ] **Step 5:** Commit `feat(web): super admin panel`.

---

### Task 22: Deploy produksi dan e2e golden path

**Files:**
- Create:
  - `api/Dockerfile` (multi-stage, distroless);
  - `web/Dockerfile` (Next.js `output: "standalone"`);
  - `docker-compose.yml`:
    - service `db` (postgres:17, volume `pgdata`, healthcheck);
    - service `api` (depends_on db healthy);
    - service `web`;
    - service `nginx` (port 80/443, mount `deploy/nginx/nginx.conf` dan sertifikat);
    - service `backup` (postgres:17 + loop `pg_dump` harian ke volume `backups`, menyimpan 14 hari).
  - `deploy/nginx/nginx.conf`:
    - `client_max_body_size 11m`;
    - `/api/` → api:8080;
    - `/` → web:3000;
    - redirect http → https.
  - `.env.example`: `POSTGRES_PASSWORD`, `DATABASE_URL`, `APP_BASE_URL`, `COOKIE_SECURE=true`, `TZ=Asia/Jakarta`.
  - `README.md`:
    - prasyarat VPS;
    - penerbitan sertifikat TLS dengan certbot (standalone) lalu path mount;
    - `docker compose up -d`;
    - `docker compose exec api /server create-superadmin …`;
    - restore dari backup.
  - `web/e2e/golden-path.spec.ts`, `web/playwright.config.ts`, `docker-compose.e2e.yml` (stack lengkap + seed).
- Test: `web/e2e/golden-path.spec.ts`

**Interfaces:**
- **Variabel environment:**
  - `APP_BASE_URL`: dipakai untuk URL QR di label.
  - `COOKIE_SECURE`: harus `true` di produksi.
- **Seed e2e:** script `api/cmd/server` subcommand `seed-demo --tenant "PT Demo" --email demo@assetku.test --password demo12345`. Script ini membuat tenant, user, 3 lokasi, dan 1 organization. Subcommand ini juga berguna untuk demo ke calon pembeli.

- [ ] **Step 1: Write failing e2e test** `golden-path.spec.ts`:
  1. Buka `/assets/<id-tidak-ada>` tanpa login. Harus redirect ke `/login?next=…`. Setelah login, kembali ke URL itu dan menampilkan "Asset tidak ditemukan".
  2. Buka `/login?next=//evil.com`, login. Harus mendarat di `/dashboard`.
  3. Import CSV berantakan (fixture `e2e/fixtures/messy.csv`: `;`, BOM, judul, 5 baris). Hasilnya Baru = 5.
  4. Di `/assets`, klik kartu Needs Attention. URL berisi `status=maintenance,damaged,lost` dan jumlah baris sesuai.
  5. Buka asset, log movement ke "Gudang Bekasi". Header menampilkan lokasi baru, dan History berisi entri perpindahan.
  6. Generate Label. Unduhan bernama `label-*.pdf`.
  7. Ganti bahasa ke English. Judul "Assets" dan sidebar dalam bahasa Inggris.
  8. Ganti tema ke terang. Atribut `class` pada `html` tidak mengandung `dark`.
  9. Ulangi langkah 4 di viewport 360×740. Daftar tampil sebagai kartu dan menu dibuka lewat Sheet.
- [ ] **Step 2:** Jalankan `docker compose -f docker-compose.e2e.yml up -d --build && cd web && npx playwright test`. Expected: FAIL sebelum Dockerfile dan config selesai.
- [ ] **Step 3:** Implementasikan Dockerfile, compose, nginx, backup, README, dan seed-demo.
- [ ] **Step 4:** Jalankan perintah yang sama. Expected: semua langkah PASS. Lalu `docker compose config` tanpa error, dan `curl -k https://localhost/api/health` mengembalikan `{"ok":true}` dari stack produksi lokal.
- [ ] **Step 5:** Jalankan seluruh test sekali lagi: `cd api && go test ./... && cd ../web && npx vitest run && npm run build`. Expected: semua PASS.
- [ ] **Step 6:** Commit `chore: production docker compose, nginx, backups, e2e golden path`.
