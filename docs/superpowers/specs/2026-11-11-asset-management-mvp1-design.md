# PRD — Asset Management SaaS, MVP 1 (Aplikasi Asset)

Tanggal: 2026-11-11
Status: Disetujui pemilik (2026-11-11)
Nama produk: "AssetKu" (sementara; nama dan logo final menyusul, di luar MVP 1)

Catatan penanda:
- **[Keputusan pemilik]** = sudah diputuskan oleh pemilik produk.
- **[Keputusan Claude]** = diputuskan oleh penulis dokumen karena belum dibahas. Silakan dikoreksi.
- Pada 2026-11-11 pemilik sudah mengonfirmasi keputusan utama: Assigned To berupa teks bebas, akun dibuat lewat `/admin`, aturan import, reminder, BAST, dan batas file import.

---

## 1. Latar belakang dan tujuan

### Masalah
Perusahaan skala menengah masih mencatat asset secara manual atau di spreadsheet. Akibatnya:
- sulit tahu barang ada di mana dan dipegang siapa;
- riwayat servis tercecer;
- serah terima barang (BAST) dibuat ulang di Word setiap kali;
- jadwal maintenance sering terlewat.

### Siapa penggunanya
- **Pembeli:** perusahaan menengah dengan ratusan sampai beberapa ribu asset, yang punya cabang atau anak perusahaan. **[Keputusan pemilik]**
- **Pengguna harian:** staf General Affair (GA) atau admin. Jumlahnya ditentukan perusahaan pembeli. **[Keputusan pemilik]**
- **Operator SaaS:** pemilik produk, yang membuat akun perusahaan dan user lewat panel super admin.

### Tujuan MVP 1
Aplikasi asset yang lengkap dan siap dipakai satu atau beberapa pelanggan pertama, yang onboarding-nya dilakukan manual oleh pemilik produk.

### Ukuran keberhasilan
- Pelanggan bisa meng-import spreadsheet lama mereka dan langsung punya data yang rapi, tanpa perlu bantuan teknis.
- Staf GA bisa menemukan asset, mencetak label dan BAST, serta mencatat servis dan perpindahan tanpa kembali ke Excel.

### Peta jalan (di luar MVP 1)
**[Keputusan pemilik]**
- **MVP 2:** landing page perusahaan, dengan produk Asset sebagai produk pertama dan produk lain berstatus "coming soon".
- **MVP 3:** alur pelanggan baru, yaitu pendaftaran, paket (tier berdasarkan jumlah user dan asset), dan undangan user.
- **MVP 4:** payment gateway Midtrans.

### Non-goals MVP 1
- Landing page.
- Pendaftaran mandiri.
- Paket langganan dan batas kuota.
- Pembayaran.
- Notifikasi email, WhatsApp, atau push.
- Aplikasi mobile native. Versi web tetap harus responsif.
- Upload foto atau dokumen asset.
- Perbedaan hak akses antar user (role).
- Depresiasi atau akuntansi asset.
- Audit atau stock opname.
- Peminjaman (check-out/check-in).

### Anti-goals
Hal-hal yang membuat MVP 1 gagal walaupun secara teknis berjalan:
- Data perusahaan A bisa terlihat atau tersentuh oleh perusahaan B.
- Import menolak spreadsheet "berantakan" yang wajar dari pelanggan, sehingga mereka menyerah di hari pertama.
- Halaman QR asset bisa dibuka publik tanpa login.

---

## 2. Teknologi dan arsitektur

**[Keputusan pemilik]**
- **Frontend:** Next.js (App Router, TypeScript), memakai komponen **shadcn/ui** dan Tailwind CSS.
- **Backend:** Go, berupa REST JSON API.
- **Database:** PostgreSQL.
- **Deployment:** VPS dengan Docker Compose, berisi service `web` (Next.js), `api` (Go), `db` (Postgres), dan `nginx` (reverse proxy + TLS).
- **Responsif:** semua halaman nyaman dipakai di HP, tablet, dan desktop.

**[Keputusan Claude]**
- Satu domain. Nginx meneruskan `/api/*` ke Go dan sisanya ke Next.js, sehingga cookie sesi berada di domain yang sama.
- Next.js berperan sebagai UI saja. Semua logika bisnis, validasi, dan otorisasi ada di Go.
- Pemilihan library diserahkan ke builder. Usulannya:
  - router: `chi`;
  - akses DB: `pgx` + `sqlc`;
  - migrasi: `golang-migrate`;
  - membaca dan menulis Excel: `excelize` (.xlsx) ditambah library untuk .xls lama;
  - PDF: dibuat di Go, misalnya `maroto` atau `gofpdf`;
  - QR: `go-qrcode`;
  - i18n frontend: `next-intl`.
- Job harian untuk reminder maintenance berjalan di dalam proses Go (scheduler internal), bukan cron OS.

### Multi-tenant
Fondasi multi-tenant sudah ada di MVP 1 supaya MVP 3 tidak perlu migrasi besar. **[Keputusan Claude]**
- Tabel `tenants` mewakili perusahaan pelanggan.
- Semua data bisnis punya kolom `tenant_id`. Setiap query di API **wajib** difilter berdasarkan `tenant_id` milik user yang login.
- Builder wajib menyertakan test yang membuktikan user tenant A tidak bisa membaca atau mengubah data tenant B, baik lewat ID langsung maupun lewat import dan export.
- Asset Tag unik **per tenant**, bukan global.

---

## 3. Desain visual

**[Keputusan pemilik]** Desain mengikuti file `starchart-DESIGN.md` milik pemilik, memakai komponen shadcn/ui yang token-nya diganti.

**[Keputusan pemilik]** StarChart diterapkan **persis seperti aslinya** (opsi 1 di mockup), bukan versi kantor:
- **Font:** Fredoka untuk judul, DM Sans untuk body, Space Mono untuk Asset Tag, serial number, dan nomor BAST.
- **Ukuran:** skala tipografi, spacing (basis 8px), dan radius sesuai spesifikasi.
- **Bentuk:** tombol pill, kartu radius 16px, input radius 12px.
- **Efek:** glow nebula dan glow star sesuai level di spesifikasi.
- **Tap target:** minimal 44px. Tinggi baris tabel menyesuaikan, sehingga tabel tidak padat.
- **Komponen:** Buttons, Cards, Inputs, Chips, Lists, Checkbox, Radio, dan Tooltip mengikuti spesifikasi.
- **Star yellow:** hanya untuk CTA utama (Add Asset, tombol simpan), sesuai aturan Do's and Don'ts.
- **Ikon:** **tidak ada emoji di UI maupun PDF** **[Keputusan pemilik]**. Semua ikon memakai ikon garis lucide (bawaan shadcn/ui).
- **Animasi:** gaya "achievement" (glow pulse) dipakai hemat, misalnya saat import selesai. **[Keputusan Claude]**

**Tema gelap dan terang** **[Keputusan pemilik]**
User bisa memilih tema gelap atau terang dari menu user. Pilihan disimpan per user, dan default-nya gelap.

**Tema gelap:** persis palet StarChart.
- Background Deep Space `#1E1B4B`.
- Surface raised `#2E2A6E`, sunken `#141136`, dan overlay `#3D3890`.
- Nebula `#A78BFA`.
- Star `#FDE047`.
- Warna status: Success `#4ADE80`, Warning `#FBBF24`, Error `#F87171`, Info `#60A5FA`, Locked `#8B82C3`.

**Tema terang:** StarChart tidak mendefinisikan tema terang, jadi palet ini diturunkan oleh **[Keputusan Claude]**. Nuansanya tetap ungu dan tidak memakai putih murni, mengikuti semangat aturan "Don't use pure white":
- **Permukaan:** background `#F5F3FF`, surface `#FFFFFF` di atas background lavender, sunken `#EDE9FE`, border `#DDD6FE`.
- **Teks:** utama Deep Space `#1E1B4B`, sekunder `#4C4799`.
- **Nebula untuk teks dan border interaktif:** digelapkan menjadi `#7C3AED` supaya kontras ≥ 4.5:1. Glow tetap memakai `#A78BFA`.
- **Tombol Primary:** fill Star `#FDE047` dengan teks Deep Space, sama seperti di tema gelap.
- **Warna status untuk teks chip:** versi lebih gelap, yaitu Success `#15803D`, Warning `#B45309`, Error `#DC2626`, Info `#2563EB`, Locked `#6B63A8`. Fill chip 15–20% dari warna dasarnya.

Semua warna diimplementasikan sebagai CSS variable token shadcn/ui (`--background`, `--primary`, dll.) untuk kedua tema.

**Warna status asset**

| Status | Warna |
| --- | --- |
| Available | Success |
| In Use | Info |
| Maintenance | Warning |
| Damaged | Error |
| Lost | Error |
| Archived | Locked `#8B82C3` |

**Responsif**
- **Desktop:** sidebar kiri.
- **Tablet dan HP:** sidebar menjadi drawer (Sheet).
- **Tabel asset di HP:** tampil sebagai daftar kartu, berisi tag, nama, dan status.
- **Kartu ringkasan:** 4 kolom di desktop, 2×2 di HP.
- **Tombol aksi kanan atas:** digabung ke menu "⋯" di HP, kecuali Add Asset.

PDF (label, BAST, export) memakai **latar putih** karena untuk dicetak.

---

## 4. Pengguna, login, dan bahasa

### Pengguna

**User tenant** **[Keputusan pemilik]**
- Semua user di satu perusahaan punya hak yang sama: lihat, tambah, edit, hapus, import, export, cetak, dan mengelola master data.

**Super admin** **[Keputusan Claude]**
- Berupa akun khusus milik pemilik produk, terpisah dari tenant.
- Halaman `/admin` sederhana bisa:
  - membuat, mengubah nama, dan menonaktifkan tenant;
  - membuat user untuk tenant (nama, email, jabatan, password awal);
  - reset password user;
  - menonaktifkan user.
- Super admin tidak bisa melihat data asset tenant di MVP 1.

### Login **[Keputusan Claude]**
- Login memakai email dan password. Sesi disimpan di cookie httpOnly.
- User bisa mengganti password sendiri.
- Fitur lupa password lewat email belum ada. Reset dilakukan super admin.
- User di tenant yang nonaktif tidak bisa login.

### Profil user
Berisi nama, email, jabatan, preferensi bahasa, dan preferensi tema (gelap atau terang). **Jabatan** dipakai di BAST. **[Keputusan Claude]**

### Bahasa **[Keputusan pemilik]**
- Antarmuka tersedia dalam bahasa Indonesia dan Inggris, bisa diganti kapan saja dari menu user.
- Default-nya Indonesia, dan pilihan disimpan per user. **[Keputusan Claude]**
- Format tanggal `dd MMM yyyy`, mata uang Rupiah (`Rp 12.500.000`).
- Label nilai enum (kategori, status, kondisi) ikut diterjemahkan, tetapi nilai yang disimpan di database tetap kode bahasa Inggris.
- PDF BAST selalu dalam bahasa Indonesia, karena merupakan dokumen resmi. **[Keputusan Claude]** Label QR tidak bergantung bahasa.

---

## 5. Model data

Semua tabel bisnis punya `tenant_id`, `created_at`, dan `updated_at`.

### Asset
| Field | Tipe | Aturan |
| --- | --- | --- |
| name | teks | wajib |
| asset_tag | teks | unik per tenant. Kalau kosong, dibuat otomatis `AST-` + 8 karakter acak huruf besar dan angka (`AST-8F2K1Q9Z`), diulang bila bentrok |
| category | enum | wajib: `it_equipment`, `vehicle`, `building_facility`, `furniture`, `electronics`, `other` |
| manufacturer | teks | opsional, isian bebas |
| model | teks | opsional |
| serial_number | teks | opsional, tidak wajib unik |
| status | enum | wajib: `available`, `in_use`, `maintenance`, `damaged`, `lost`, `archived`. Default `available` |
| location_id | FK Location | opsional |
| assigned_to | teks | opsional. Nama pemegang. **[Keputusan Claude]** Isian bebas dengan saran otomatis dari nama yang pernah dipakai di tenant. Master karyawan ditunda |
| organization_id | FK Organization | opsional |
| supplier_id | FK Supplier | opsional |
| order_number | teks | No. PO/Order, opsional |
| purchase_date | tanggal | opsional |
| purchase_cost | desimal (Rupiah) | opsional, ≥ 0 |
| warranty_months | integer | opsional, ≥ 0 |
| warranty_expires | turunan | `purchase_date + warranty_months`, dihitung dan tidak disimpan. Tampil "—" bila salah satu kosong. Ditandai "Expired" bila sudah lewat |
| notes | teks panjang | opsional |

### Master data (per tenant, dikelola lewat menu sendiri)

**[Keputusan Claude]** untuk menu ini dan aturan hapusnya.

- **Location:** nama (unik per tenant) dan alamat atau keterangan (opsional).
- **Organization:** nama (unik per tenant), yaitu cabang atau anak perusahaan.
- **Supplier:** nama (unik per tenant, dicocokkan tanpa membedakan huruf besar-kecil), kontak, telepon, email, dan alamat (semua opsional).
- **Aturan hapus:** master yang masih dipakai asset tidak bisa dihapus. Muncul pesan berisi jumlah asset yang memakainya.

### Maintenance record (per asset)
- **Field:** tanggal, jenis, pelaksana (teks), biaya (Rupiah), tanggal maintenance berikutnya (opsional), reminder H-berapa (integer hari, opsional, default 7 bila tanggal berikutnya diisi), dan catatan.
- **Jenis:** `Preventive`, `Corrective`, `Inspection`, `Other`. **[Keputusan Claude]**
- Catatan bisa dihapus dan tidak bisa diedit. **[Keputusan Claude]** Kalau salah, hapus lalu buat ulang.

### Movement record (per asset)
Sebelumnya bernama "Asset Transfer". Diganti menjadi **Movements**: "Asset Movement" / "Perpindahan Asset". **[Keputusan pemilik]**
- **Field:** lokasi asal (diisi otomatis dari lokasi saat ini, boleh kosong), lokasi tujuan (wajib), tanggal, dipindahkan oleh (teks, default nama user login, bisa diubah), dan alasan.
- Saat disimpan, `location_id` asset otomatis berubah ke lokasi tujuan dan tercatat di History.
- Kalau catatan movement dihapus, lokasi asset **tidak** dikembalikan.

### History (activity log per asset)
- **Field:** jenis event, pelaku (user id dan nama saat itu), waktu, dan detail (JSON berisi nilai lama dan baru).
- **Event yang dicatat:**
  - asset dibuat;
  - status berubah;
  - lokasi berubah;
  - pemegang berubah;
  - detail lain diubah, dengan daftar field yang berubah;
  - label dicetak;
  - BAST dicetak, beserta nomor BAST;
  - maintenance dicatat atau dihapus;
  - movement dicatat atau dihapus;
  - asset dibuat atau diperbarui lewat import.
- Satu kali simpan form yang mengubah status, lokasi, dan pemegang sekaligus menghasilkan satu entri per jenis perubahan.

### Notification
- **Field:** user, tipe, judul, isi, link ke asset, status dibaca, waktu.

### Delete asset
Penghapusan bersifat permanen (hard delete), termasuk History, Maintenance, dan Movement milik asset itu. Wajib ada dialog konfirmasi yang menyebut nama dan tag asset.

---

## 6. Halaman Daftar Asset (`/assets`)

### Kartu ringkasan
Empat kartu, masing-masing bisa diklik untuk memfilter tabel:

| Kartu | Isi |
| --- | --- |
| Total Assets | semua asset |
| Available | status Available |
| In Use | status In Use |
| Needs Attention | status Maintenance, Damaged, atau Lost |

- Kartu yang aktif diberi highlight. Klik Total Assets menghapus filter status dari kartu.
- Angka di kartu menghitung **seluruh asset tenant**, tidak terpengaruh filter lain. **[Keputusan Claude]**
- Asset Archived ikut dihitung di Total, tetapi tidak masuk kartu lain.

### Pencarian dan filter
- **Pencarian:** satu kotak, mencocokkan sebagian teks tanpa membedakan huruf besar-kecil pada Asset Tag, nama, serial number, model, dan nama pemegang. Pencarian dijalankan setelah jeda ketik sekitar 300 ms.
- **Filter:** dropdown Category, Status, Location, dan Organization. Category dan Status bisa pilih lebih dari satu. **[Keputusan Claude]**
- **Kartu dan filter Status:** keduanya saling sinkron. Mengklik kartu Needs Attention sama dengan memilih tiga status.
- **Reset:** menghapus pencarian, semua filter, dan pilihan kartu.
- **URL:** keadaan filter, urutan, dan halaman disimpan di URL, sehingga bisa di-bookmark dan tombol back berfungsi. **[Keputusan Claude]**

### Tabel
- **Kolom:** Asset Tag, Asset Name, Category, Model, Serial Number, Status.
- **Urutan:** semua kolom bisa diurutkan. Default-nya terbaru dibuat lebih dulu.
- **Pagination:** 20 baris per halaman, server-side, dengan info "1–20 dari 1.248".
- **Navigasi:** klik baris membuka halaman detail.
- **Menu aksi (⋯) per baris:** View Details, Edit, Delete (dengan konfirmasi).
- **Keadaan kosong:**
  - belum ada asset sama sekali: tampil ajakan "Import dari spreadsheet" dan "Add Asset";
  - hasil filter kosong: tampil pesan dan tombol Reset.

### Tombol kanan atas
- **Add Asset:** membuka halaman form (bagian 9).
- **Import:** membuka dialog import (bagian 10).
- **Export:** menu CSV, Excel (.xlsx), atau PDF.
  - Yang diekspor adalah **seluruh** hasil filter dan pencarian yang aktif (semua halaman, bukan hanya 20 baris), dengan urutan yang sama.
  - **[Keputusan Claude]** CSV dan Excel berisi semua field asset, memakai header yang bisa di-import ulang.
  - PDF berisi kolom tabel ditambah Location dan Assigned To, landscape A4, dengan judul, nama tenant, tanggal cetak, dan ringkasan filter.

---

## 7. Halaman Detail Asset (`/assets/{id}`)

### Header
- **QR code** berisi URL penuh halaman detail asset tersebut.
  - Memindai QR saat belum login akan membuka halaman login, lalu diarahkan kembali ke asset itu.
  - Kalau user login di tenant lain, tampil "Asset tidak ditemukan".
- **Info utama:** nama, status (chip), Asset Tag, kategori, dan serial number.
- **Info tambahan:** manufacturer, model, lokasi, dan pemegang.

### Tombol header
- Generate Label.
- Print BAST.
- Edit.
- Delete (permanen, dengan konfirmasi).
- Di HP, tombol-tombol ini digabung ke menu.

### Generate Label
- Menghasilkan PDF berukuran tepat **90×50 mm** (satu label per halaman).
- **Isi:** QR code, Asset Tag (mono, tebal), nama, model, dan serial. Teks yang terlalu panjang dipotong dengan "…".
- **[Keputusan Claude]** Nama tenant ditampilkan kecil di label.
- Setiap kali dibuat, tercatat event "Label dicetak" di History.

### Print BAST
Membuka dialog berisi form, lalu menghasilkan PDF A4.

**Isian dialog**
- **Nomor:** otomatis `BAST-<tag>-<YYYYMMDD>`, contoh `BAST-AST-8F2K1Q9Z-20261111`. Tidak bisa diedit.
- **Tanggal serah terima:** default hari ini.
- **Pihak Pertama:** terisi otomatis dari nama dan jabatan user yang login. Bisa diedit di dialog.
- **Pihak Kedua:** terisi otomatis dari pemegang asset. Jabatan dan departemen diisi manual. Kalau pemegang kosong, nama wajib diisi.
- **Kondisi barang:** "Baik dan Layak Digunakan", "Baru (Segel)", atau "Bekas (Normal)".
- **Garansi:** teks, terisi otomatis dari tanggal garansi habis bila ada (contoh "s.d. 11 Nov 2027"). Bisa diedit.
- **Catatan tambahan:** opsional.

**Isi PDF**
- Kop berisi nama tenant.
- Judul "BERITA ACARA SERAH TERIMA BARANG" dan nomor.
- Paragraf pembuka dengan tanggal dalam kata.
- Identitas kedua pihak.
- Tabel barang: tag, nama, kategori, merek/model, serial, kondisi.
- Garansi dan catatan.
- Pernyataan serah terima.
- Dua kolom tanda tangan (Pihak Pertama dan Pihak Kedua).

**Aturan lain**
- Tidak ada perubahan otomatis pada pemegang atau status. **[Keputusan Claude]**
- Tercatat event "BAST dicetak" beserta nomornya.
- Kalau dicetak dua kali di hari yang sama, nomornya sama. Ini konsekuensi format yang diminta.

### Tab

**Information**
Empat bagian:
- General Information.
- Status & Placement.
- Purchase & Warranty: Supplier, No. PO, tanggal beli, harga, garansi (bulan), dan **garansi berakhir** (dihitung otomatis).
- Notes.

**History**
- Daftar event terbaru di atas. Setiap event menampilkan ikon, kalimat yang mudah dibaca, nama pelaku, dan waktu.
- Contoh kalimat: "Budi mengubah status dari Available ke In Use", "Lokasi berubah: Kantor Jakarta → Gudang Bekasi".
- Memuat 20 event per halaman dengan tombol "Muat lagi".

**Maintenance**
- Tabel atau daftar riwayat servis.
- Tombol **Tambah Catatan** membuka dialog berisi field Maintenance.
- Setiap catatan bisa dihapus dengan konfirmasi.
- Maintenance berikutnya yang terdekat ditampilkan di atas daftar.

**Movements**
- Daftar riwayat perpindahan.
- Tombol **Log Movement** membuka dialog.
- Setiap catatan bisa dihapus dengan konfirmasi. Lokasi asset tidak dikembalikan.

---

## 8. Dashboard (`/dashboard`) dan notifikasi

### Dashboard **[Keputusan Claude]** untuk isi selain daftar perhatian
- **Kartu ringkasan:** sama seperti di halaman asset. Klik kartu membuka `/assets` dengan filter yang sesuai.
- **Perlu Perhatian:** daftar asset dengan status Maintenance, Damaged, atau Lost, berisi tag, nama, status, lokasi, dan terakhir diubah. Klik baris membuka detail asset.
- **Maintenance mendatang:** asset yang tanggal maintenance berikutnya jatuh dalam 30 hari ke depan atau sudah lewat.
- **Komposisi:** jumlah asset per kategori dan per status, dalam bentuk grafik sederhana.

### Reminder maintenance
- Job berjalan setiap 24 jam (sekitar 07:00 WIB), dan juga sekali saat server start.
- Untuk setiap catatan maintenance yang punya tanggal berikutnya: bila `hari ini ≥ next_date − reminder_days` dan reminder untuk catatan itu belum pernah dikirim, kirim notifikasi ke **semua user aktif di tenant tersebut** (semua user dianggap staf GA), lalu tandai sudah dikirim.
  - Contoh isi: "Maintenance AC Daikin 1 PK (AST-Q81LX2VB) jatuh tempo 18 Nov 2026".
  - Kalau tanggal berikutnya diubah dengan menghapus lalu membuat catatan baru, reminder dihitung ulang.
- Hanya catatan maintenance **terbaru** per asset yang dicek. Catatan lama yang sudah digantikan tidak memicu reminder. **[Keputusan Claude]**
- **Lonceng notifikasi** di header menampilkan jumlah belum dibaca dan dropdown 20 notifikasi terakhir. Klik notifikasi membuka asset dan menandainya dibaca. Ada pilihan "Tandai semua dibaca".

---

## 9. Form Tambah dan Edit Asset (`/assets/new`, `/assets/{id}/edit`)

- **Susunan:** empat bagian, yaitu General Information, Status & Placement, Purchase & Warranty, dan Notes. Satu kolom di HP, dua kolom di desktop.
- **Validasi:** dilakukan di frontend dan backend.
  - Field wajib: Asset Name, Category, Status.
  - Asset Tag harus unik per tenant. Kalau bentrok, tampil pesan error di field.
- **Kolom pilihan master:** Location, Organization, dan Supplier berupa combobox yang bisa dicari, dengan opsi "+ Tambah baru" langsung dari form. **[Keputusan Claude]**
- **Assigned To:** isian teks dengan saran otomatis.
- **Garansi berakhir:** ditampilkan langsung saat tanggal beli dan garansi diisi.
- **Setelah simpan:** diarahkan ke halaman detail asset.
- **Peringatan perubahan:** kalau user meninggalkan form yang sudah diubah, muncul peringatan.

---

## 10. Import

### Alur
1. Dialog Import terbuka. User mengunggah file .xlsx, .xls, atau .csv (maks. 10 MB, maks. 10.000 baris). **[Keputusan Claude]** Tersedia tautan "Unduh template".
   - Untuk .xlsx/.xls, sheet pertama yang dibaca.
   - Baris header dideteksi otomatis sebagai baris pertama yang berisi minimal satu header yang dikenali, dalam 10 baris teratas. Ini supaya judul di atas tabel tidak mengganggu.
2. File diproses sebagai job di server. UI menampilkan **progress bar** (baris diproses / total) yang diperbarui berkala. Dialog boleh ditutup tanpa membatalkan job.
3. Setelah selesai, tampil **hasil**.

### Pengenalan header
- Pencocokan header tanpa membedakan huruf besar-kecil, spasi, tanda baca, dan garis bawah.
- Setiap field menerima beberapa alias Indonesia dan Inggris. Daftar minimal:

| Field | Alias (contoh, builder boleh menambah) |
| --- | --- |
| name | Asset Name, Nama Asset, Nama Aset, Nama Barang, Name, Item Name |
| asset_tag | Asset Tag, Tag, Kode Asset, Kode Aset, Nomor Asset, Asset Code |
| category | Category, Kategori, Jenis |
| manufacturer | Manufacturer, Merek, Merk, Brand, Pabrikan |
| model | Model, Tipe, Type, Model No., Model Number |
| serial_number | Serial Number, Serial, No Seri, Nomor Seri, SN |
| status | Status, Kondisi |
| location | Location, Lokasi, Default Location |
| assigned_to | Assigned To, Pemegang, Pengguna, Checked Out To, PIC |
| organization | Organization, Organisasi, Company, Perusahaan, Cabang, Branch |
| supplier | Supplier, Vendor, Pemasok |
| order_number | Order Number, No PO, Nomor PO, PO Number |
| purchase_date | Purchase Date, Tanggal Beli, Tanggal Pembelian |
| purchase_cost | Purchase Cost, Harga, Harga Beli, Harga Pembelian |
| warranty_months | Warranty, Warranty Months, Garansi, Garansi (Bulan) |
| notes | Notes, Catatan, Keterangan |

### Konversi nilai
- **Kategori:** dicocokkan ke enum lewat alias Indonesia dan Inggris (misal "Kendaraan" menjadi vehicle, "Peralatan IT"/"Komputer"/"Laptop" menjadi it_equipment). Yang tidak cocok menjadi `other`. **[Keputusan Claude]**
- **Status:** dicocokkan lewat alias Indonesia dan Inggris (Tersedia, Digunakan, Dipakai, Perbaikan, Rusak, Hilang, Diarsipkan, dll.) dan pemetaan Snipe-IT (lihat di bawah). Yang kosong atau tidak dikenali menjadi `available`, dan dicatat di hasil sebagai "nilai tidak dikenali".
- **Tanggal:** menerima tanggal Excel, `dd/mm/yyyy`, `dd-mm-yyyy`, `yyyy-mm-dd`, dan nama bulan Indonesia/Inggris.
- **Harga:** menerima "Rp 12.500.000", "12,500,000.00", dan "12500000".
- **Garansi:** menerima angka atau teks seperti "12 bulan" / "12 months" / "1 tahun".

### Format Snipe-IT **[Keputusan pemilik]**
- **Snipe-IT:** header ekspor standarnya (Asset Tag, Asset Name / Name, Category, Manufacturer, Model, Serial, Status, Location, Checked Out To, Company, Supplier, Order Number, Purchase Date, Purchase Cost, Warranty, Notes) dikenali.
  - Pemetaan status: "Ready to Deploy" menjadi available, "Deployed" menjadi in_use, "Pending" menjadi maintenance, "Un-deployable"/"Broken" menjadi damaged, "Lost"/"Stolen" menjadi lost, "Archived" menjadi archived.

### Pencocokan data
- **Asset sama:** baris dengan Asset Tag yang sudah ada di tenant akan **memperbarui** asset itu. Hanya kolom yang ada di file dan berisi nilai yang ditimpa; sel kosong tidak menghapus data yang ada. **[Keputusan Claude]** Kalau semua nilainya sama, baris dihitung **tidak berubah**.
- **Asset baru:** baris tanpa tag, atau dengan tag baru, membuat asset baru. Tag otomatis dibuat bila kosong.
- **Organization:** dicocokkan berdasarkan nama tanpa membedakan huruf besar-kecil dan spasi di ujung. Kalau tidak ditemukan, field dikosongkan, asset tetap masuk, dan namanya dicatat sebagai "nama yang tidak cocok".
- **Supplier:** yang belum ada **dibuat otomatis**.
- **Location:** yang belum ada **dibuat otomatis**. **[Keputusan Claude]** Ini konsisten dengan Supplier, dan lokasi wajar beragam di spreadsheet lama.
- **Baris dilewati:** baris tanpa nama asset, atau baris yang sepenuhnya kosong. Baris kosong tidak dihitung.
- **Duplikat dalam file:** kalau Asset Tag yang sama muncul dua kali di satu file, baris kedua dilewati dengan alasan "tag duplikat di file".
- **Transaksi:** setiap baris diproses sendiri-sendiri, sehingga satu baris gagal tidak membatalkan seluruh import.
- **History:** setiap asset yang dibuat atau diperbarui mendapat entri History "dibuat/diperbarui lewat import".

### Layar hasil
- **Angka ringkas:** **Baru**, **Diperbarui**, **Tidak berubah**, **Dilewati**.
- **Kolom tidak dikenali:** daftar header yang diabaikan.
- **Nama tidak cocok:** daftar nama Organization yang tidak ditemukan beserta jumlah barisnya, juga nilai status atau kategori yang tidak dikenali.
- **Detail baris dilewati:** nomor baris dan alasannya.
- **Unduh laporan:** tombol untuk mengunduh laporan hasil sebagai CSV. **[Keputusan Claude]**

---

## 11. Navigasi

- **Sidebar:** Dashboard, Assets, Locations, Organizations, Suppliers.
- **Menu user:** profil, ganti password, bahasa, logout.
- **Header:** lonceng notifikasi.
- **Panel super admin `/admin`:** tata letak sendiri, dengan menu Tenants dan Users.

---

## 12. Kualitas dan non-fungsional

- **Performa:** daftar asset, termasuk pencarian dan filter, merespons di bawah 500 ms untuk tenant dengan 10.000 asset. Ini butuh index yang sesuai, misalnya trigram untuk pencarian teks.
- **Keamanan:**
  - password di-hash dengan bcrypt/argon2;
  - perlindungan CSRF untuk cookie sesi;
  - rate limit login;
  - semua input divalidasi di server;
  - isolasi tenant diuji.
- **Test:**
  - unit test untuk parser import (alias, tanggal, harga, status), pembuat tag, perhitungan garansi, dan logika reminder;
  - integration test API untuk alur utama dan isolasi tenant.
- **Deploy:**
  - satu `docker compose up -d` menjalankan semuanya;
  - migrasi DB berjalan otomatis saat API start;
  - konfigurasi lewat `.env`;
  - README berisi langkah setup VPS dan backup Postgres harian (`pg_dump`).
- **Seed:** perintah untuk membuat akun super admin pertama.

---

## 13. Diserahkan ke builder

- Struktur folder dan pemilihan library akhir.
- Bentuk persis endpoint API.
- Detail tata letak setiap halaman, selama mengikuti desain di bagian 3 dan komponen shadcn/ui.
- Mekanisme progress import (polling atau SSE).
