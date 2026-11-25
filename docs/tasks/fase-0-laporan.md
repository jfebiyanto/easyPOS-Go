# Laporan Fase 0 — easyPOS-Go

Branch lokal: `fase-0` (tidak di-push).

## 1. Versi tools
go 1.27.4 · node v24.22.1 · npm 12.0.2 · git 2.56.1.windows.1 · docker (server) 29.9.3 · psql: tidak ada (dipakai via `docker exec`) · php 8.5.7 · composer 2.10.3 · sqlc (go install, versi terbaru saat itu) · Postgres 16 (container).

## 2. Commit (branch fase-0)
1. Kosongkan repo untuk produk baru
2. T0.3: migration baseline Laravel (0d78191), drop_unused, new_features + spec fase 0
3. T0.2/T0.4-T0.7/T0.9: struktur Go, sqlc, auth Sanctum-compatible, matriks akses, penomoran, settings, CLI
4. T0.11: tes integrasi Go
5. T0.8-T0.10: frontend shell, README, CLAUDE.md, scripts/dev.ps1
6. Laporan ini

## 3. Hasil definisi selesai
| Perintah | Hasil |
|---|---|
| `go vet ./...` | lulus, tanpa temuan |
| `go test ./...` (dengan `EASYPOS_TEST_DATABASE_URL`) | lulus: httpapi, identity, numbering |
| `npm --prefix web run build` | lulus (`tsc --noEmit` + `vite build`) |
| `npm --prefix web test` | lulus, 35 tes (resolusi tema + kontras >= 4.5:1 kedua tema + tanpa hijau) |
| `migrate up` di DB kosong | lulus: 00001, 00002, 00003 tanpa perlu perbaikan |
| `migrate status` | menampilkan 3 migration, semuanya `applied` |
| `serve` | jalan di :8080 |
| login owner + `/api/auth/me` (curl) | lulus (klien api: token di body; klien backoffice: cookie, CSRF 403 tanpa header, 204 dengan header) |
| `/backoffice/` dan `/pos/` | tersaji (200), SPA fallback `/backoffice/numberings` 200, `/` redirect 302 ke `/backoffice/`; dicek juga visual di browser (tema gelap dan terang) |

Cakupan tes integrasi Go: login hash `$2y$` buatan PHP (fixture `internal/testutil/php_hash.txt`), password salah / user nonaktif / user tidak ada, rate limit 5/menit (429), format dan penyimpanan token (sha256), me, logout, token kedaluwarsa, user dinonaktifkan setelah login, cookie tanpa header CSRF = 403, switch store per peran, matriks akses table-driven, penomoran 50 goroutine + reset periode, settings owner-only + validasi enum, penomoran owner-only.

## 4. Keputusan & keraguan
- **Port baseline**: 55432 pada spesifikasi ditolak Windows (rentang port terpesan). Memakai 45432 untuk container baseline sementara; hasil dump sama.
- **Baseline di-dump**: `composer install` dengan `--no-scripts --ignore-platform-req=ext-*` (PHP 8.5 vs composer.json `^8.3`, ekstensi pdo_pgsql tersedia). `migrations` tidak ikut dump; `migrations_id_seq` tetap ada sebagai sequence yatim (tidak berbahaya, bisa dibuang di migration berikutnya). Header SET/`\restrict` dari pg_dump dibuang.
- Rate limit login memakai IP dari `RemoteAddr` (bukan header `X-Forwarded-For`, agar tidak bisa dipalsukan). Di balik reverse proxy, semua klien tampak satu IP; perlu penyesuaian saat deploy.
- Cookie `easypos_token` dipakai bersama oleh `/backoffice/` dan `/pos/` (satu host): login di salah satunya menimpa sesi yang lain di browser yang sama.
- Kasir ditolak login klien `backoffice` (403). Non-owner tanpa `users.store_id` ditolak login (403) agar tidak jatuh ke gerai pertama perusahaan; owner tanpa gerai tetap boleh.
- Jika 401, frontend menampilkan form login di tempat (URL tidak berpindah ke `/login`).
- Bentuk body `PUT /api/settings`: `{"settings": {"pos.serial_not_in_stock": "allow"}}`; `PUT /api/numberings`: `{"items": [{transaction_type, prefix, padding, separator, reset_period}]}` (spesifikasi tidak merinci).
- Penomoran: `GET /api/numberings` membuat konfigurasi default untuk tipe yang belum ada (seperti `allTypes()` Laravel). Prefix default `stock_adjustment` dan `stock_counting` sama-sama `STO` dan label keduanya "Stock Counting" (diwarisi dari Laravel; bisa membingungkan, belum diubah).
- `create-owner` tidak membuat warehouse (`stores.warehouse_id` nullable); `--password` minimal 8 karakter (tambahan di luar spesifikasi). Password muncul di riwayat shell bila diketik di command line.
- `go.mod` berisi `go 1.27.4` (versi toolchain lokal); spesifikasi minimal 1.25.
- `.gitattributes` memaksa LF.

## 5. Belum selesai / catatan
- Batasan Fase 0 yang disengaja: halaman selain Ringkasan, Konfigurasi Sistem, dan Penomoran berupa "Segera hadir"; POS hanya shell.
- Tidak ada tes frontend selain tema (belum ada komponen berlogika berat); tidak ada tes aksesibilitas otomatis (kontras dicek lewat tes token).
- `GET /api/auth/me` belum membatasi gerai aktif jika penugasan `users.store_id` berubah setelah login (token lama tetap membawa gerai lama sampai logout).
- Font Inter dimuat dari Google Fonts (fallback system-ui); butuh internet untuk Inter.
- Belum ada CI dan Dockerfile (repo lama dikosongkan sesuai instruksi).
