# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# aturan tetap easyPOS-Go

Produk: POS + backoffice dengan Go (backend) dan React/TS (frontend), DB PostgreSQL. Perilaku bisnis mengacu ke
aplikasi Laravel di `D:\Devs\easyPOS` (acuan baca-saja; jangan diubah dari repo ini).

## Struktur
- `cmd/easypos` — CLI cobra: `serve`, `migrate up|down|status`, `create-owner`, `version`.
- `internal/platform/{config,database,httpx}` — konfigurasi env (`EASYPOS_*`), pgxpool + helper transaksi + goose, helper HTTP.
- `internal/db` — kode hasil **sqlc**. Jangan diedit tangan.
- `internal/{identity,audit,numbering,settings}` — domain. `internal/httpapi` — router chi, middleware, handler.
- `db/migrations` — goose; `db/queries` — SQL untuk sqlc. `web/` — Vite + React (entri `/backoffice/` dan `/pos/`), dibuild ke `web/dist` dan di-embed.

## Aturan
1. **sqlc**: ubah/tambah query di `db/queries/*.sql`, lalu `sqlc generate`. Commit hasilnya.
2. **Migration**: setiap perubahan skema = file goose baru (`000NN_nama.sql`). Jangan mengubah migration yang sudah ada.
3. **Uang dan qty** selalu `shopspring/decimal` (kolom `numeric`), tidak pernah float.
4. **ID** = UUID v7 yang dibuat aplikasi (`uuid.NewV7()`), bukan default DB.
5. **Harga dan total selalu dihitung server**; jangan percaya angka dari klien.
6. **Waktu**: kolom `timestamp(0) without time zone` disimpan UTC (`time.Now().UTC()`); zona tampilan/penomoran = `EASYPOS_APP_TIMEZONE`.
7. **Akses**: setiap rute backoffice punya route key gaya Laravel (mis. `admin.settings.update`) dan dibatasi `identity.Allows`. Rute baru otomatis read-only bagi store_manager sampai didaftarkan di `writablePrefixes`.
8. **Tes**: setiap aturan bisnis wajib punya tes. Tes integrasi memakai Postgres sungguhan (`EASYPOS_TEST_DATABASE_URL`, DB sementara dibuat otomatis oleh `internal/testutil`).
9. **UI**: bahasa Indonesia. **Tidak ada warna hijau.** Dark dan light wajib untuk setiap komponen baru — pakai token di `web/src/shared/theme.css`, jangan hardcode warna. Semua request lewat `web/src/shared/api.ts`. Kontras teks >= 4.5:1.
10. Jangan commit secret; `.env` di-gitignore, contoh di `.env.example`.
11. **Commit per fase/task**: setiap fase atau task dari `docs/tasks/` selesai (vet + test hijau, termasuk tes DB) langsung di-commit lokal dengan pesan `fase-N: <ringkasan>`; satu commit per fase/task, tidak digabung. Jangan push kecuali diminta.

## Perintah
- `go vet ./...` · `go test ./...` · `sqlc generate` · `make build` (web + binary ke `bin/easypos`)
- Satu tes: `go test ./internal/httpapi -run TestNama -v`; frontend: `npm --prefix web test -- <pola-file>`
- `npm --prefix web run build` (menjalankan `tsc --noEmit` lalu vite) · `npm --prefix web test` · dev hot reload: `npm --prefix web run dev` (proxy `/api` ke :8080)
- Go tidak membaca `.env`: pakai `.\scripts\dev.ps1 <perintah easypos>` (mis. `migrate up`, `serve`) di PowerShell.
- Tes integrasi butuh `docker compose up -d db` (Postgres 16, port 5433) dan `EASYPOS_TEST_DATABASE_URL`; tanpa itu tes DB di-skip (bukan gagal), jadi `go test` hijau belum berarti DB teruji.

## Arsitektur (butuh baca beberapa file)
- **Mesin CRUD generik** (`internal/crud`): resource master data dideklarasikan sebagai `crud.Resource` (tabel, `Fields`, `Search`, `Filters`, `Expand`, hook `Validate/BeforeWrite/AfterSave/CanDelete/Locked/Decorate`). `Engine` menangani list/detail/create/update/delete, validasi 422, cek dipakai 409, audit `activity_logs`, dan filter `company_id` — jangan menulis handler CRUD manual untuk entitas baru. `Binder.Mount` memasang rute dengan route key. `internal/org` adalah contoh resource (company singleton, pajak, mata uang, gudang, gerai, pengguna).
- **Status Fase 1**: spesifikasi di `docs/tasks/fase-1.md` (bentuk API, aturan hapus, audit, tes wajib per entitas). `org.Mount` belum dipasang di `internal/httpapi/router.go` dan belum dites.
- **Auth/akses**: `identity` memegang token (kompatibel Sanctum `{id}|{token}`), `Allows` (matriks route key per peran) dan `writablePrefixes`. Cookie terpisah `easypos_bo_token` (backoffice) dan `easypos_pos_token` (`/api/pos/*`); Bearer didahulukan.
- **Frontend** di-embed ke binary lewat `web/dist` (`internal/httpapi/web.go`), jadi build web dulu sebelum `go build`.
- `internal/testutil.NewDB` membuat DB sementara + seed (company, 2 gerai, owner/manager/cashier/inactive; password `rahasia123`) per tes.

## Peran kerja
Coding dikerjakan Sonnet berdasarkan spesifikasi (`docs/tasks/`) dan direview Opus.
