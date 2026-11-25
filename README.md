# easyPOS-Go

POS + backoffice dengan Go dan React. Fase 0 = fondasi: skema DB, login/sesi, matriks akses, penomoran,
Konfigurasi Sistem, dan shell frontend (Backoffice + POS).

## Prasyarat
Go >= 1.25, Node >= 20, Docker Desktop. (Opsional: `sqlc` untuk regenerasi kode query.)

## Menjalankan lokal (Windows / PowerShell)
```powershell
Copy-Item .env.example .env          # lalu muat variabelnya; lihat catatan di bawah
docker compose up -d db              # Postgres 16 di port 5433
go run ./cmd/easypos migrate up
go run ./cmd/easypos create-owner --company "PT Contoh" --store-code UTM --store-name "Gerai Utama" --username owner --password "GantiIni123"
npm --prefix web install
npm --prefix web run build
go run ./cmd/easypos serve
```
Buka http://localhost:8080/backoffice/ (POS: http://localhost:8080/pos/).

Go tidak membaca file `.env` sendiri; `scripts/dev.ps1` memuat `.env` ke environment lalu menjalankan perintah:
```powershell
.\scripts\dev.ps1 migrate up
.\scripts\dev.ps1 serve
```
(Tanpa skrip, set manual: `$env:EASYPOS_DATABASE_URL = "postgres://easypos:easypos@localhost:5433/easypos?sslmode=disable"`.)

Pengembangan frontend dengan hot reload: jalankan `serve` di satu terminal dan `npm --prefix web run dev` di terminal lain
(Vite mem-proxy `/api` ke `localhost:8080`).

## Tes
```powershell
docker exec easypos-go-db-1 psql -U easypos -d postgres -c "CREATE DATABASE easypos_test"   # sekali saja
$env:EASYPOS_TEST_DATABASE_URL = "postgres://easypos:easypos@localhost:5433/easypos_test?sslmode=disable"
go vet ./...
go test ./...
npm --prefix web test
```
Tes integrasi membuat database sementara di server tersebut dan menghapusnya setelah selesai.
Tanpa `EASYPOS_TEST_DATABASE_URL`, tes yang butuh DB dilewati.

## Konfigurasi (environment)
| Variabel | Default | Keterangan |
|---|---|---|
| `EASYPOS_DATABASE_URL` | (wajib) | URL Postgres |
| `EASYPOS_HTTP_ADDR` | `:8080` | alamat server |
| `EASYPOS_APP_TIMEZONE` | `Asia/Jakarta` | zona waktu penomoran |
| `EASYPOS_ENV` | `dev` | `dev` atau `prod` |
| `EASYPOS_COOKIE_SECURE` | `false` | set `true` di balik HTTPS |

## API singkat (Fase 0)
`POST /api/auth/login` · `GET /api/auth/me` · `POST /api/auth/logout` · `GET /api/stores/accessible` ·
`POST /api/session/store` · `GET|PUT /api/settings` · `GET|PUT /api/numberings` · `POST /api/numberings/{type}/next`.
Token kompatibel Laravel Sanctum (`{id}|{token}`); klien backoffice/pos memakai cookie httpOnly + header `X-EasyPOS-Client: 1`.

Aturan untuk agen/pengembang: lihat `CLAUDE.md`. Spesifikasi fase: `docs/tasks/`.
