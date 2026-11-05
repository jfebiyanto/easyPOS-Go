# easyPOS (Go)

Backend Go + PostgreSQL untuk easyPOS SaaS (lihat PRD v1.6). Satu binary melayani API dan frontend.

## Deploy ke VPS (Ubuntu) — satu perintah

```bash
git clone https://github.com/jfebiyanto/easyPOS-Go.git && cd easyPOS-Go
./deploy/install.sh                              # akses lewat IP (HTTP)
# atau dengan domain + HTTPS otomatis:
EASYPOS_DOMAIN=pos.contoh.com ./deploy/install.sh
```

Skrip memasang Docker bila belum ada, membuat `.env` dengan password acak, membangun dan menjalankan
Postgres + aplikasi + Caddy, lalu mencetak URL dan login Owner. Update: `git pull && ./deploy/install.sh`.
Backup database: `docker compose exec db pg_dump -U easypos easypos > backup.sql`.

## Development

```bash
cp .env.example .env   # lalu export variabelnya, butuh Postgres lokal
make run
make test              # butuh EASYPOS_TEST_DATABASE_URL / TESTDB
```
