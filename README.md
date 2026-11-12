# AssetKu

Aplikasi manajemen asset multi-tenant: Next.js (`web/`), Go REST API (`api/`), PostgreSQL 17, dan nginx sebagai reverse proxy + TLS. Semua berjalan dengan satu `docker compose up -d`.

Nginx meneruskan `/api/*` ke Go dan sisanya ke Next.js di domain yang sama, jadi cookie sesi tetap first-party. Migrasi database berjalan otomatis saat API start. Reminder maintenance berjalan di dalam proses API (07:00 WIB).

## Deploy ke VPS

### 1. Prasyarat

- VPS Linux 64-bit (Ubuntu 22.04/24.04 atau Debian 12). Minimal 2 vCPU, 2 GB RAM, dan 20 GB disk.
- Docker Engine 24+ dengan plugin Compose v2 (`docker compose version`).
- Domain (misal `aset.example.co.id`) dengan record A/AAAA yang mengarah ke IP VPS.
- Port 80 dan 443 terbuka di firewall.
- `certbot` di host (`sudo apt install certbot`).

### 2. Ambil kode dan isi konfigurasi

```bash
git clone <repo> assetku && cd assetku
cp .env.example .env
# Isi POSTGRES_PASSWORD (misal `openssl rand -hex 24`) dan APP_BASE_URL=https://aset.example.co.id.
# COOKIE_SECURE harus tetap true.
nano .env
```

`APP_BASE_URL` dipakai untuk URL di QR code pada label asset. Jadi isi dengan alamat publik HTTPS, tanpa `/` di akhir.

### 3. Sertifikat TLS (certbot standalone)

Certbot standalone membuka port 80 sendiri, jadi jalankan sebelum nginx hidup:

```bash
sudo certbot certonly --standalone --cert-name assetku \
  -d aset.example.co.id --agree-tos -m admin@example.co.id --no-eff-email
```

Sertifikat tersimpan di `/etc/letsencrypt/live/assetku/{fullchain,privkey}.pem`. Seluruh `/etc/letsencrypt` di-mount read-only ke container nginx (`CERTS_DIR` di `.env` untuk lokasi lain). `deploy/nginx/nginx.conf` membaca path itu. Nama `--cert-name assetku` wajib sama.

Perpanjangan otomatis lewat timer certbot. Karena memakai mode standalone, nginx perlu dihentikan sebentar saat renew. Pasang hook sekali (ganti `/opt/assetku` dengan direktori repo):

```bash
sudo tee /etc/letsencrypt/renewal-hooks/pre/assetku.sh >/dev/null <<'SH'
#!/bin/sh
docker compose -f /opt/assetku/docker-compose.yml stop nginx
SH
sudo tee /etc/letsencrypt/renewal-hooks/post/assetku.sh >/dev/null <<'SH'
#!/bin/sh
docker compose -f /opt/assetku/docker-compose.yml start nginx
SH
sudo chmod +x /etc/letsencrypt/renewal-hooks/pre/assetku.sh /etc/letsencrypt/renewal-hooks/post/assetku.sh
sudo certbot renew --dry-run
```

### 4. Jalankan

```bash
docker compose up -d --build
docker compose ps                     # semua service healthy
curl https://aset.example.co.id/api/health   # {"ok":true}
```

Service yang berjalan: `db` (postgres:17, volume `pgdata`), `api`, `web`, `nginx` (port 80 → redirect ke 443), dan `backup`.

### 5. Akun super admin pertama

```bash
docker compose exec api /server create-superadmin \
  --email admin@example.co.id --name "Super Admin" --password 'password-yang-kuat'
```

Masuk di `https://aset.example.co.id/admin/login`, lalu buat tenant dan user-nya. Perintah ini idempoten: kalau email yang sama dijalankan ulang, nama dan password-nya diperbarui.

Untuk demo ke calon pembeli, buat tenant contoh berisi 1 user, 3 lokasi, dan 1 organization:

```bash
docker compose exec api /server seed-demo --tenant "PT Demo" --email demo@assetku.test --password demo12345
```

Kalau dijalankan ulang, password user dan preferensinya (Indonesia, tema gelap) di-reset.

### Update versi

```bash
git pull
docker compose up -d --build
```

## Backup dan restore

Service `backup` menjalankan `pg_dump` (format custom) saat start lalu setiap 24 jam. Hasilnya disimpan ke volume `backups` sebagai `assetku-YYYYMMDD-HHMMSS.dump`, dan dump yang lebih tua dari 14 hari dihapus otomatis.

Lihat dan salin backup ke host (sebaiknya juga simpan salinannya di luar VPS):

```bash
docker compose exec backup ls -lh /backups
docker compose cp backup:/backups/assetku-20261111-070000.dump ./
```

Backup manual saat itu juga:

```bash
docker compose exec backup sh -c 'pg_dump --format=custom --file=/backups/assetku-manual-$(date +%Y%m%d-%H%M%S).dump'
```

### Restore

Restore menimpa seluruh isi database dengan isi dump.

```bash
# 1. Hentikan aplikasi supaya tidak ada tulisan baru.
docker compose stop nginx web api

# 2. Restore (file dump ada di host, hasil `docker compose cp` di atas).
docker compose exec -T db pg_restore -U assetku -d assetku --clean --if-exists --no-owner \
  < ./assetku-20261111-070000.dump

# 3. Hidupkan lagi. API menjalankan migrasi yang belum ada secara otomatis.
docker compose up -d
```

Untuk restore ke VPS baru, jalankan `docker compose up -d db` dulu dan tunggu `healthy`, lalu ikuti langkah 2 dan 3.

## Konfigurasi

| Variabel | Keterangan |
| --- | --- |
| `POSTGRES_PASSWORD` | Password user `assetku` di Postgres, juga dipakai service `backup`. |
| `DATABASE_URL` | Koneksi API ke Postgres. Default `.env.example` sudah menyusunnya dari `POSTGRES_PASSWORD`. |
| `APP_BASE_URL` | Origin publik HTTPS, dipakai untuk URL QR pada label. |
| `COOKIE_SECURE` | `true` di produksi. Cookie sesi dan CSRF hanya dikirim lewat HTTPS. |
| `TZ` | `Asia/Jakarta`. |
| `TRUSTED_PROXIES` | Opsional. Daftar CIDR yang boleh mengirim `X-Real-IP` (dipakai rate limit login). Default compose memakai rentang privat, yaitu nginx di network compose. Kosong berarti tidak ada yang dipercaya. |
| `CERTS_DIR` | Opsional. Direktori keluaran certbot, default `/etc/letsencrypt`. |

Catatan IPv6: nginx hanya listen di IPv4. Bila Docker berjalan tanpa IPv6, klien IPv6 masuk lewat proxy Docker dan nginx melihat IP gateway, sehingga semua klien IPv6 berbagi satu IP untuk rate limit login (batas gagal per IP: 50 per 15 menit). Aktifkan IPv6 di Docker bila banyak pengguna datang lewat IPv6.

## Pengembangan lokal (tanpa Docker)

Satu perintah menjalankan database, API, dan web. `Ctrl+C` mematikan ketiganya:

```bash
./dev.sh    # buka http://localhost:3000
```

`dev.sh` membuat `api/.env` dari `api/.env.example` bila belum ada, dan menjalankan `npm install` bila `web/node_modules` belum ada.

Menjalankan satu per satu, masing-masing di terminal sendiri:

```bash
cd api && go run ./cmd/devdb           # Postgres embedded di :55432, data di ~/.cache/assetku-devdb
cd api && go run ./cmd/server          # API di :8080, membaca api/.env
cd web && npm install && npm run dev   # http://localhost:3000, /api diproxy ke :8080
```

API membaca `api/.env` bila dijalankan dari folder `api/`. Variabel yang sudah di-set di shell tetap menang. Produksi tidak memakai file ini.

Akun pertama di database baru (dari folder `api/`):

```bash
go run ./cmd/server seed-demo --tenant "PT Demo" --email demo@assetku.test --password demo12345
go run ./cmd/server create-superadmin --email admin@assetku.test --name "Super Admin" --password admin12345
```

Test:

```bash
cd api && go test ./...
cd web && npx vitest run && npm run build
```

### E2E golden path (Playwright)

Dengan Docker, memakai topologi produksi lewat HTTP biasa di port 8088:

```bash
docker compose -f docker-compose.e2e.yml up -d --build
cd web && npx playwright install chromium   # sekali saja
E2E_BASE_URL=http://localhost:8088 npx playwright test
docker compose -f docker-compose.e2e.yml down -v
```

Tanpa Docker (stack lokal di :3000):

```bash
# terminal 1: database sekali pakai
cd api && DEVDB_DIR=/tmp/assetku-e2e-db go run ./cmd/devdb
# terminal 2: seed dan API
cd api && export DATABASE_URL='postgres://postgres:postgres@localhost:55432/assetku?sslmode=disable' \
  APP_BASE_URL=http://localhost:3000 COOKIE_SECURE=false
go run ./cmd/server seed-demo --tenant "PT Demo" --email demo@assetku.test --password demo12345
go run ./cmd/server
# terminal 3: build produksi yang memproxy /api ke API lokal
cd web && API_PROXY_ORIGIN=http://localhost:8080 npm run build && npm start
# terminal 4
cd web && npx playwright test
```

Jalankan ulang `seed-demo` sebelum mengulang test: perintah itu mengembalikan bahasa user ke Indonesia dan tema ke gelap.
