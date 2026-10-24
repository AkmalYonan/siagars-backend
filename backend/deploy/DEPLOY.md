# Deployment & operasional SiagaRS Backend

Status: panduan dan contoh konfigurasi. **Belum teruji** di lingkungan nyata (Docker tidak tersedia saat dokumen ini disusun; `docker build`, `docker compose config` dan proxy belum dijalankan). Jangan deploy publik tanpa izin pemilik proyek. Proyek ini DEMO_MODE: data bersifat simulasi, bukan data RS/pasien nyata.

## Arsitektur
- Satu origin publik (`https://<domain>`): reverse proxy meneruskan `/api/v1/*` ke Go (:8080) dan sisanya ke Next.js (:3000). Contoh: `deploy/Caddyfile`, `deploy/nginx.conf`.
- MongoDB di Atlas (replica set, wajib untuk transaksi). Tidak ada container database.
- Binary: `/app/api` (server) dan `/app/manage` (migrate, seed, operator, sync-satusehat). Keduanya hanya membaca process environment, tidak memuat `.env`.
- `/docs` dan `/docs/openapi.json` ada di port Go dan sengaja tidak diproksikan.

## Variabel environment produksi
Semua diberikan lewat environment proses/secret manager, bukan file di image.

| Variabel | Nilai produksi |
|---|---|
| APP_ENV | `production` (menolak `PUBLIC_ORIGIN` non-HTTPS) |
| PUBLIC_ORIGIN | `https://<domain>` persis, tanpa path/slash akhir. Origin login/mutasi harus sama persis |
| MONGODB_URI | URI Atlas (secret) |
| MONGODB_DATABASE | `siagars` (atau database terpisah per lingkungan) |
| PORT | `8080` |
| SESSION_TTL_HOURS | default 8 |
| DEMO_MODE | `true` (wajib) |
| SATUSEHAT_ENABLED | `false` kecuali ada izin; hanya `SATUSEHAT_ENV=sandbox` diterima |
| TRUSTED_PROXIES | (ditambah worker B) daftar IP/CIDR reverse proxy agar `X-Forwarded-For` hanya dipercaya dari proxy. Setel ke alamat proxy; kosongkan bila API diakses langsung. Lihat README/DECISIONS untuk format final |

Cookie sesi: `HttpOnly`, `SameSite=Lax`, `Path=/api/v1`, `Secure` pada HTTPS. Jadi pastikan proxy mengirim `X-Forwarded-Proto: https` dan TLS berakhir di proxy. HTTP hanya untuk `APP_ENV=development`.

## Urutan langkah
Jalankan dari host/CI yang terisolasi; env Atlas diberikan sebagai env proses (jangan `cat` atau commit file kredensial).

1. **Migrate** (sebelum API): `docker compose run --rm --entrypoint /app/manage api migrate`
2. **Seed** (data simulasi, idempotent): `docker compose run --rm --entrypoint /app/manage api seed`
3. **Operator** (akun baru; tidak mereset password, email duplikat ditolak):
   `docker compose run -e OPERATOR_EMAIL -e OPERATOR_PASSWORD -e OPERATOR_HOSPITAL_IDS --rm --entrypoint /app/manage api operator`
   Password 12..72 byte. Unset `OPERATOR_PASSWORD` dari shell setelahnya.
4. **API**: `docker compose up -d --build api`
5. **Proxy + Next.js**: pasang Caddy/nginx dari `deploy/`, jalankan Next.js di :3000.
6. Verifikasi: `GET /api/v1/health` 200, `GET /api/v1/ready` 200 (cek koneksi dan schema_migrations), login operator dari browser pada origin publik.

Healthcheck container memanggil `/api/v1/health` (liveness). Readiness (`/ready`) dipakai monitor/orkestrator.

## Checklist go-live
- [ ] Izin eksplisit pemilik proyek untuk deploy publik.
- [ ] `APP_ENV=production`, `PUBLIC_ORIGIN` HTTPS persis sama dengan origin browser.
- [ ] Sertifikat TLS valid; HSTS aktif; HTTP dialihkan ke HTTPS.
- [ ] Migrate sudah diterapkan; `/ready` 200.
- [ ] Atlas Network Access hanya IP egress server (bukan `0.0.0.0/0`); user DB Atlas berhak minimal untuk database `siagars`.
- [ ] Secret hanya di secret manager/env proses; tidak ada `.env`/`atlas-credentials.env` di image (`docker run --rm --entrypoint sh <img> -c 'ls -a /app'` untuk memeriksa).
- [ ] `TRUSTED_PROXIES` hanya berisi alamat proxy; proxy menimpa (bukan menambahkan) `X-Forwarded-For` dari klien.
- [ ] Port 8080 dan 3000 tidak terbuka ke publik (compose mengikat 127.0.0.1).
- [ ] `/docs` tidak terjangkau dari internet.
- [ ] Akun operator dibuat dengan password kuat; tidak ada kredensial default.
- [ ] Banner/penanda simulasi terlihat; `DEMO_MODE=true`; `SATUSEHAT_ENABLED=false`.
- [ ] Backup Atlas aktif dan pemulihan pernah dicoba di cluster/database terpisah.
- [ ] Uji asap: health, ready, login, CSRF, satu mutasi di database non-produksi.

## Rollback
- **Aplikasi**: tag image per rilis; rollback = `docker compose up -d` dengan image tag sebelumnya (atau build ulang dari rilis sebelumnya). Perubahan skema selalu berupa migration bernomor baru yang kompatibel mundur; jangan edit 001/002.
- **Skema/data**: tidak ada migration "down". Bila migration baru bermasalah, hentikan API, pulihkan dari backup/snapshot Atlas ke cluster atau database baru, uji, lalu arahkan `MONGODB_URI`/`MONGODB_DATABASE` ke sana. Jangan drop/reset database yang sedang dipakai.
- **Proxy**: simpan konfigurasi proxy sebelumnya; reload dengan versi lama.

## Backup Atlas
- Aktifkan Cloud Backup (snapshot terjadwal) atau Continuous Backup/PITR pada cluster; atur retensi.
- Sebelum migrate di produksi: ambil snapshot manual on-demand.
- Latihan restore berkala ke cluster/database sementara, lalu verifikasi `/ready` dan hitungan koleksi utama. Tier gratis (M0) tidak mendukung Cloud Backup; gunakan `mongodump` terjadwal ke penyimpanan terenkripsi.

## Atlas Network Access
- Tambahkan hanya IP egress statis server/NAT. Hindari `0.0.0.0/0` di produksi.
- Bila platform tidak punya IP statis, gunakan private endpoint/VPC peering (tier berbayar).
- Hapus entri sementara (mis. IP laptop) setelah selesai.

## DILARANG tanpa izin eksplisit
- Deploy ke internet publik atau membuka port 8080/3000 ke publik.
- Membaca, mencetak, mengirim, atau meng-commit `.env` dan `atlas-credentials.env`.
- Menulis, mereset, atau men-drop database Atlas `siagars` di luar migrate/seed/operator yang terdokumentasi.
- Mengedit migration 001/002 yang sudah diterapkan.
- Mengirim data ke SATUSEHAT, atau memakai SATUSEHAT production (ditolak oleh konfigurasi; hanya Sandbox read-only via `manage sync-satusehat`).
- Memasukkan data pasien nyata atau data RS karangan sebagai data nyata.
- Mengubah `DEMO_MODE`, endpoint, enum, atau makna `null` tanpa memperbarui `docs/openapi.yaml` dan `docs/fixtures.json`.
- Membuka Atlas Network Access ke `0.0.0.0/0` atau menonaktifkan `Secure` cookie/pemeriksaan Origin di produksi.

## Verifikasi konfigurasi (saat Docker tersedia)
```sh
docker compose config            # butuh MONGODB_URI di environment
docker build -t siagars-api:test .
docker run --rm --entrypoint sh siagars-api:test -c 'id; ls -a /app'
```
Status saat ini: **belum teruji** (lihat atas).
