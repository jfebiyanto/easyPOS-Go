# Fase 0 — Fondasi easyPOS-Go (spesifikasi untuk eksekutor Sonnet)

## Aturan kerja (wajib)
- Kerja HANYA di `D:\Devs\easyPOS-go` (repo baru) dan, untuk membaca saja, `D:\Devs\easyPOS` (repo Laravel acuan). Jangan ubah apa pun di `D:\Devs\easyPOS` (boleh `git worktree` ke folder baru `D:\Devs\easyPOS-baseline` bila perlu checkout commit lain).
- JANGAN push ke GitHub, jangan buka PR. Januar akan memeriksa dan testing lokal. Commit lokal per tugas (T0.x) dengan pesan jelas berbahasa Indonesia, di branch `fase-0`.
- Tidak ada secret di repo. `.env` di-gitignore; sediakan `.env.example`.
- Jangan bertanya ke user di tengah jalan. Bila ragu, pilih opsi paling sederhana yang sesuai spesifikasi dan catat di laporan akhir bagian "Keputusan & keraguan".
- Uang/qty selalu `shopspring/decimal` (bukan float). UUID v7 dibuat aplikasi (`uuid.NewV7()`), bukan default DB.
- Timestamp kolom mengikuti skema Laravel (`timestamp(0) without time zone`, disimpan dalam UTC).

## T0.1 Persiapan repo dan cek tools
1. Catat versi: `go version`, `node -v`, `npm -v`, `git --version`, `docker version`, `psql --version`, `php -v`, `composer --version` (yang tidak ada, tulis "tidak ada").
2. Bila `D:\Devs\easyPOS-go` belum ada: `git clone https://github.com/jfebiyanto/easyPOS-Go.git D:\Devs\easyPOS-go`. Bila sudah ada, pastikan working tree bersih (kalau tidak bersih: berhenti dan laporkan).
3. `git checkout -b fase-0` dari `main`. Hapus SEMUA isi lama yang ter-track (`git rm -r -q .`), sisakan `.git`. Commit: "Kosongkan repo untuk produk baru".

## T0.2 Struktur proyek
Module Go: `github.com/jfebiyanto/easyPOS-Go`, Go >= 1.25.
```
cmd/easypos/main.go            # cobra: serve | migrate up|down|status | create-owner | version
internal/platform/config       # caarlos0/env: EASYPOS_DATABASE_URL, EASYPOS_HTTP_ADDR(:8080), EASYPOS_APP_TIMEZONE(Asia/Jakarta), EASYPOS_ENV(dev|prod), EASYPOS_COOKIE_SECURE
internal/platform/database     # pgxpool, helper tx
internal/platform/httpx        # JSON response/error helper, request ID, slog logging middleware
internal/db                    # kode hasil sqlc (jangan diedit tangan)
internal/identity              # login, token, me, logout, switch store, access matrix
internal/audit                 # tulis activity_logs
internal/numbering             # penomoran transaksi
internal/settings              # system_settings
internal/httpapi               # router chi + middleware + handler wiring
db/migrations/                 # goose SQL
db/queries/                    # query sqlc
db/embed.go                    # embed migrations
web/                           # Vite + React + TS (lihat T0.8)
web/embed.go                   # embed web/dist
docs/tasks/fase-0.md           # salinan spesifikasi ini
CLAUDE.md                      # aturan tetap untuk agen (lihat T0.10)
docker-compose.yml             # postgres:16 untuk dev, port 5433
.env.example, Makefile, scripts/dev.ps1 (Windows), README.md
```
Library: chi v5, httprate, pgx v5 (pgxpool), sqlc (sqlc.yaml, engine postgresql, pgx/v5), goose v3, shopspring/decimal, golang.org/x/crypto/bcrypt, google/uuid, caarlos0/env v11, cobra, log/slog.

## T0.3 Migration
1. 00001_laravel_baseline.sql — skema persis dari migration Laravel pada commit `0d78191`:
   - Jalankan Postgres sementara: `docker run -d --name easypos-baseline -e POSTGRES_PASSWORD=baseline -p 55432:5432 postgres:16`.
   - Buat worktree acuan: `git -C D:\Devs\easyPOS worktree add D:\Devs\easyPOS-baseline 0d78191`, lalu di folder itu `composer install --no-interaction` dan buat `.env` dari `.env.example` (`php artisan key:generate`).
   - Jalankan dengan env var (override .env): DB_CONNECTION=pgsql DB_HOST=127.0.0.1 DB_PORT=55432 DB_DATABASE=postgres DB_USERNAME=postgres DB_PASSWORD=baseline lalu `php artisan migrate --force`.
   - `docker exec easypos-baseline pg_dump -U postgres --schema-only --no-owner --no-privileges --exclude-table=migrations postgres` → simpan, buang baris SET/set_config yang tidak perlu, bungkus `-- +goose Up` (+ `-- +goose StatementBegin`/`StatementEnd` bila ada fungsi), dan `-- +goose Down` berisi `SELECT 'baseline tidak bisa di-down';`.
   - Bersihkan: `docker rm -f easypos-baseline`, `git -C D:\Devs\easyPOS worktree remove D:\Devs\easyPOS-baseline --force`.
   - Bila PHP 8.4+ tidak tersedia: coba image `php:8.4-cli` via docker dengan ekstensi pdo_pgsql. Bila tetap gagal, BERHENTI di T0.3 dan laporkan (jangan menulis skema dengan tangan).
2. 00002_drop_unused.sql dan 00003_new_features.sql — pakai PERSIS file yang disertakan (`db/migrations/`). Bila ada error saat migrate up, perbaiki seminimal mungkin dan catat di laporan.
3. `easypos migrate up` di DB kosong harus sukses; `migrate status` menampilkan 3 migration.

## T0.4 Auth dan sesi (kompatibel Laravel Sanctum)
- Login: `POST /api/auth/login` body `{username, password, device_name?, client: "backoffice"|"pos"|"api"}`.
  - Cari `users.username`. Bandingkan bcrypt (hash Laravel `$2y$` didukung `x/crypto/bcrypt`, sudah diuji Opus). Bila user tidak ada, tetap jalankan bcrypt terhadap hash dummy (cegah timing attack). Gagal bila user tidak ada / password salah / `is_active=false` → 422 `{"message":"Username atau password salah, atau akun nonaktif."}`.
  - Token: 40 karakter acak (crypto/rand, alfanumerik). Simpan di `personal_access_tokens`: tokenable_type='App\Models\User', tokenable_id=user.id, name=device_name||client, token=sha256 hex(plain), abilities='["*"]', client_kind=client, active_store_id = user.store_id ?? store pertama perusahaan (order by code), created_at/updated_at=now. Plain token ke klien: "{id}|{plain}" (format Sanctum).
  - Klien api: token di body. Klien backoffice/pos: token dipasang di cookie httpOnly `easypos_token` (SameSite=Lax, Path=/, Secure bila EASYPOS_COOKIE_SECURE=true), body tidak membawa token.
  - Rate limit: 5 percobaan/menit per IP+username (httprate) → 429.
  - Tulis activity log `auth.login` / `auth.login_failed`.
- Middleware auth: baca `Authorization: Bearer {id}|{plain}` atau cookie. Cari baris by id, bandingkan sha256 (constant-time), cek expires_at, user aktif. Update last_used_at paling sering 1x/menit.
- CSRF untuk cookie: request non-GET yang memakai cookie WAJIB header `X-EasyPOS-Client: 1`; kalau tidak → 403.
- `GET /api/auth/me` → {id, name, username, email, role, store_id, active_store: {id, code, name}, company: {id, name}, permissions: {...}}.
- `POST /api/auth/logout` → hapus baris token, hapus cookie, activity log `auth.logout`.
- `GET /api/stores/accessible` dan `POST /api/session/store {store_id}`: owner boleh pilih gerai mana pun di perusahaannya; store_manager dan kasir hanya users.store_id (kalau beda → 403). Simpan ke personal_access_tokens.active_store_id.

## T0.5 Matriks akses (port app/Support/BackofficeAccess.php)
- Peran: owner, store_manager, cashier. Backoffice hanya owner + store_manager; POS untuk semua peran yang punya store.
- Setiap rute backoffice didaftarkan dengan route key gaya Laravel (mis. admin.company.edit, admin.stock-adjustments.store). Aturan Allows(role, routeKey, method):
  - owner → selalu boleh; selain owner dan store_manager → tidak boleh.
  - store_manager: prefix CLOSED (admin.company., admin.numberings., admin.sap-import., admin.sap-postings., admin.settings.) → tolak; prefix WRITABLE (admin.eod., admin.stock-review., admin.stock-adjustments., admin.stock-transfer-shipments., admin.stock-transfer-receipts., admin.stock-transfer-returns., admin.deliveries.) → boleh; selain itu hanya GET/HEAD dan bukan rute form (.create/.edit, kecuali admin.promotion-hierarchy.edit).
  - Catatan: GET /api/settings boleh dibaca store_manager (route key khusus admin.settings-view.index di luar prefix CLOSED); PUT hanya owner.
- me.permissions mengembalikan peta route key → {visible, readOnly} untuk menu frontend.
- Tes tabel (table-driven) yang mencakup semua cabang di atas.

## T0.6 Activity log dan penomoran
- audit.Log(ctx, tx, {companyID, actor, action, subjectType, subjectID, description, properties, storeID}) menulis ke activity_logs (kolom persis skema Laravel, termasuk store_id).
- Penomoran: port D:\Devs\easyPOS\app\Models\NumberingConfig.php (tipe, reset_period daily/monthly/yearly/never, format prefix{sep}period{sep}padded). Next(ctx, tx, companyID, type, now) WAJIB atomik: SELECT ... FOR UPDATE pada baris config, reset last_number bila last_period beda, lalu increment. Tes paralel: 50 goroutine → 50 nomor unik berurutan.
- Endpoint: GET /api/numberings, PUT /api/numberings (owner), POST /api/numberings/{type}/next (owner).

## T0.7 Konfigurasi Sistem
- Tabel system_settings (lihat 00003). Paket settings dengan definisi key bertipe + default:
  - pos.serial_not_in_stock: "reject" (default) | "manager_approval" | "allow".
- GET /api/settings (owner & store_manager baca), PUT /api/settings (owner saja; route key admin.settings.update), validasi nilai enum, activity log settings.updated.

## T0.8 Frontend shell (Vite + React 18 + TypeScript)
- web/ satu proyek Vite, dua entri: /backoffice/ dan /pos/. React Router, TanStack Query, Tailwind v4 (@tailwindcss/vite), Radix (dialog, dropdown), ikon lucide-react. Font Inter (Google Fonts) dengan fallback system-ui.
- Build ke web/dist, di-embed Go (web/embed.go), disajikan: /backoffice/* dan /pos/* → index.html masing-masing (SPA fallback), / → redirect /backoffice/. Dev: npm run dev dengan proxy /api → http://localhost:8080.
- Token tema sebagai CSS variables di web/src/shared/theme.css, dipakai Tailwind via @theme. Format: token = terang / gelap:
  - --bg = #F5F6F8 / #0E141B
  - --surface = #FFFFFF / #151C25
  - --surface-2 = #F9FAFB / #1B2430
  - --border = #E6E8EC / #2A3442
  - --text = #101828 / #E7ECF2
  - --text-muted = #5B6474 / #9AA6B5
  - --primary (bg tombol) = #1E3A5F / #3D6A9E
  - --primary-hover = #142940 / #4A78AD
  - --on-primary = #FFFFFF / #FFFFFF
  - --primary-text (link/aksen) = #1E3A5F / #9DBCE3
  - --primary-soft = #EEF2F7 / #1C2B3D
  - --warn = #B54708 / #F5A35C
  - --warn-soft = #FEF6E7 / #3A2614
  - --danger = #B42318 / #F97066
  - --danger-soft = #FEF3F2 / #3A1715
  - Tidak ada warna hijau sama sekali. Tema dipasang lewat data-theme="light|dark" di `<html>`.
- ThemeToggle: pilihan Ikuti sistem (default) / Terang / Gelap, disimpan di localStorage['easypos.theme'] (bungkus try/catch). Skrip kecil inline di `<head>` memasang tema sebelum render (tanpa kedip). Ada di header POS dan menu user Backoffice. Tes vitest untuk resolusi tema.
- Backoffice: halaman Login (username + password), layout sidebar + header mengikuti draft desain yang disetujui (latar abu terang, kartu putih radius 14-16px, aksen navy, font Inter): grup menu Ringkasan, Penjualan, Katalog, Stok, Lainnya; item menu ditampilkan berdasarkan me.permissions.visible. Halaman yang belum dibangun → komponen "Segera hadir". Halaman jadi di fase 0: Ringkasan (placeholder sapaan + tanggal), Konfigurasi Sistem (form opsi pos.serial_not_in_stock, read-only untuk store_manager), Penomoran (owner). Switcher gerai di header (owner).
- POS: halaman Login, lalu shell kosong dengan header (logo, gerai · kasir, status online, jam, ThemeToggle, Keluar) dan area kosong "POS akan dibangun di fase 3".
- Semua request lewat satu apiClient yang menambah header X-EasyPOS-Client: 1, credentials: 'include', dan menangani 401 → ke halaman login.
- Aksesibilitas: label untuk input, tombol asli, fokus terlihat, kontras >= 4.5:1 di kedua tema.

## T0.9 CLI dan data awal
- `easypos create-owner --company "Nama PT" --store-code UTM --store-name "Gerai Utama" --username owner --password ...` untuk DB kosong: buat company_details, satu stores, user owner. Isi kolom wajib skema Laravel dengan default wajar (cek NOT NULL di baseline).
- Dokumentasikan di README langkah lokal Windows: docker compose up -d db → go run ./cmd/easypos migrate up → create-owner → npm --prefix web install && npm --prefix web run build → go run ./cmd/easypos serve → buka http://localhost:8080/backoffice/.

## T0.10 CLAUDE.md (aturan tetap untuk agen berikutnya)
Tulis ringkas: struktur modul, sqlc (edit db/queries, jalankan sqlc generate), migration goose baru per perubahan (jangan ubah migration lama), decimal untuk uang/qty, UUID v7, harga & total selalu dihitung server, setiap aturan bisnis wajib punya tes, Laravel D:\Devs\easyPOS adalah acuan perilaku, bahasa UI Indonesia, tidak ada hijau di UI, dark/light wajib untuk komponen baru, coding dikerjakan Sonnet dengan spesifikasi dan review dari Opus.

## T0.11 Tes dan definisi selesai
Tes integrasi Go memakai Postgres sungguhan (EASYPOS_TEST_DATABASE_URL, DB terpisah, migrate up di awal):
- login sukses dengan hash $2y$ buatan PHP (php -r 'echo password_hash("rahasia123", PASSWORD_BCRYPT);', simpan sebagai fixture), password salah, user nonaktif, username tidak ada;
- token: me, logout (token tak berlaku lagi), token kedaluwarsa, cookie tanpa header CSRF → 403;
- switch store per peran; matriks akses (table-driven); penomoran paralel; settings owner-only + validasi enum.
Selesai bila semua ini lulus dan dicatat di laporan: go vet ./..., go test ./..., npm --prefix web run build, npm --prefix web test, migrate up di DB kosong, serve jalan, login owner berhasil (uji dengan curl ke /api/auth/login + /api/auth/me), halaman /backoffice/ dan /pos/ tersaji.
Biarkan container Postgres dev (docker compose) tetap jalan dan buat satu user owner dengan username `owner` dan password acak; tulis password itu HANYA di laporan balasan (bukan di repo) supaya Januar bisa langsung mencoba.

## Laporan akhir (tulis juga ke docs/tasks/fase-0-laporan.md, tanpa password)
1. Versi tools. 2. Daftar commit. 3. Hasil setiap perintah di "definisi selesai" (lulus/gagal + potongan error). 4. Keputusan & keraguan. 5. Hal yang belum selesai.
