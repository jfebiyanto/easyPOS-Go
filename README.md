# easyPOS (Go)

Backend Go + PostgreSQL untuk easyPOS SaaS (lihat PRD v1.6). Satu binary melayani API dan frontend.

## Deploy ke VPS (Ubuntu) — satu perintah

```bash
git clone https://github.com/jfebiyanto/easyPOS-Go.git && cd easyPOS-Go
./deploy/install.sh                              # akses lewat IP (HTTP)
# atau dengan domain + HTTPS otomatis:
EASYPOS_DOMAIN=pos.contoh.com ./deploy/install.sh
```

Tambahkan `DEMO=1` di depan perintah untuk memuat data contoh (gerai, 14 produk, user `manager` dan `kasir`, password `demo1234`).
Skrip memasang Docker bila belum ada, membuat `.env` dengan password acak, membangun dan menjalankan
Postgres + aplikasi + Caddy, lalu mencetak URL dan login Owner. Update: `git pull && ./deploy/install.sh`.
Backup database: `docker compose exec db pg_dump -U easypos easypos > backup.sql`.

## Development

```bash
cp .env.example .env   # lalu export variabelnya, butuh Postgres lokal
make run
make test              # butuh EASYPOS_TEST_DATABASE_URL / TESTDB
```

## Yang sudah bisa dipakai

| Peran | Layar |
|---|---|
| Kasir | POS: scan/cari produk, keranjang, bayar tunai/QRIS/EDC (multi metode), kembalian, struk cetak, daftar transaksi hari ini, tombol F3/F5/F6/F8/F9/F11/F12 |
| Store Manager | Produk (tambah/ubah, SKU tidak bisa diubah), Stok (penyesuaian per gerai), Laporan penjualan |
| Owner | Sama dengan Store Manager, lintas gerai |

Aturan yang sudah ditegakkan di server: harga dihitung server (bukan dari klien), PPN dijumlah lalu dibulatkan 2 desimal di dokumen,
checkout idempoten lewat `client_uuid`, stok tidak bisa terjual melebihi saldo (aman untuk checkout bersamaan), ledger stok append-only.
