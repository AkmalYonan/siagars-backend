# Deployment produksi di Vercel

Status: **berjalan** sejak 2026-10-24. Dokumen ini mencatat apa yang benar-benar dikerjakan dan diuji. Panduan Docker + reverse proxy tetap ada di [`DEPLOY.md`](DEPLOY.md) sebagai jalur alternatif yang belum teruji.

- URL produksi: https://siagars-backend.vercel.app
- Repo: https://github.com/AkmalYonan/siagars-backend (privat), branch produksi `main`
- Proyek Vercel: `siagars-backend` (tim `akmal-yonans-projects`, plan Hobby)
- Proyek ini DEMO_MODE: data bersifat simulasi, bukan data RS/pasien nyata.

## Arsitektur

```
GitHub (main) --push--> Vercel build (Go 1.27.1) --> Serverless Function (region sin1)
                                                        |
Browser/Frontend --HTTPS--> siagars-backend.vercel.app --+--> MongoDB Atlas (Network Access 0.0.0.0/0)
```

Aplikasi aslinya server persisten (`cmd/api`, `ListenAndServe`). Di Vercel yang dipakai adalah adapter serverless, bukan `cmd/api`:

| File | Peran |
|---|---|
| `vercel.json` | Rewrite semua path `/(.*)` ke fungsi `/api` |
| `api/index.go` | Entrypoint Vercel (`func Handler`), meneruskan request ke `serverless` |
| `serverless/serverless.go` | Paket publik tipis. Wajib ada: Vercel mengompilasi `api/index.go` di luar path module, sehingga tidak boleh mengimpor `internal/...` langsung (error `use of internal package not allowed`) |
| `internal/app/serverless.go` | Membuat router dan klien Mongo saat request pertama, lalu dipakai ulang oleh invocation hangat. Inisialisasi yang gagal dicoba ulang di request berikutnya (balasan 503 `service_unavailable`) |

`cmd/api`, `Dockerfile`, dan `compose.yaml` tidak dipakai di Vercel.

## CI/CD

- Setiap push ke `main` memicu build dan deploy produksi otomatis. Branch lain, termasuk `develop`, menghasilkan Preview deployment (lihat Staging). Rilis = merge `develop` ke `main`.
- Pengaturan proyek: **Root Directory = `backend`**, framework `null`, deploy dari Git `enabled`.
- Catatan: `vercel link` dari CLI membuat proyek dengan `gitProviderOptions.createDeployments = disabled`. Pastikan sudah `enabled`, kalau tidak push tidak memicu deploy.
- Rollback: Vercel dashboard > Deployments > pilih deployment sebelumnya > Promote to Production (atau `vercel rollback`).

## Environment variables (Production)

Diisi di Vercel > Settings > Environment Variables. Tipe `sensitive` kecuali dicatat lain. Nilai tidak disimpan di repo.

| Variabel | Catatan |
|---|---|
| `APP_ENV` | `production` |
| `PUBLIC_ORIGIN` | Origin browser FE; boleh daftar dipisah koma, pertama = utama (production: `https://frontend-siagars.vercel.app,http://localhost:3000`). Awalnya `https://siagars-backend.vercel.app`. Tipe **biasa**: Vercel menolak `PUBLIC_*` bertipe sensitive. Harus sama persis dengan origin browser; ubah bila memakai domain kustom atau frontend di domain lain |
| `MONGODB_URI`, `MONGODB_DATABASE`, `MONGODB_USERNAME`, `MONGODB_PASSWORD` | Atlas |
| `SESSION_TTL_HOURS`, `DEMO_MODE` | Seperti `.env.example` |
| `SATUSEHAT_*` | Sandbox read-only |
| `OPERATOR_HOSPITAL_IDS` | Untuk `manage operator` (dijalankan dari lokal) |

Tidak diisi: `PORT` (diatur Vercel), `OPERATOR_EMAIL` dan `OPERATOR_PASSWORD` (kosong di `.env`; hanya diperlukan saat membuat operator), `TRUSTED_PROXIES` (lihat Risiko).

Variabel di tabel ini hanya ada di environment **Production**. Preview (termasuk `develop`) memakai env terpisah; lihat bagian Staging.

Mengubah env var tidak berlaku pada deployment lama; picu deploy ulang (push commit atau Redeploy di dashboard).

## Setup dari nol

```powershell
vercel login
vercel link --yes --project siagars-backend       # dari root repo
# Root Directory = backend, dan createDeployments = enabled (dashboard: Settings > Build and Deployment, Git)
vercel env add NAMA --environment production --sensitive --value <nilai>
```

Lalu di MongoDB Atlas: Network Access > tambah `0.0.0.0/0`. Terakhir `git push origin main`.

Migrate, seed, dan pembuatan operator **tidak** berjalan di Vercel. Jalankan dari lokal dengan env menunjuk ke Atlas (lihat `README.md`, "Jalankan lokal"):

```powershell
go run . migrate
go run . seed
go run ./cmd/manage operator     # butuh OPERATOR_EMAIL, OPERATOR_PASSWORD (12..72 byte), OPERATOR_HOSPITAL_IDS
```

## Verifikasi produksi 24 Oktober 2026

| Endpoint | Hasil |
|---|---|
| `GET /api/v1/health` | 200 `{"data":{"status":"ok"}}` |
| `GET /api/v1/ready` | 200 `{"data":{"status":"ready"}}` (Mongo terjangkau) |
| `GET /api/v1/hospitals?limit=1` | 200, data RS |
| `GET /api/v1/services` | 200, daftar layanan |
| `GET /docs/openapi.json` | 200, kontrak OpenAPI JSON |

Health dan readiness juga lulus tiga kali berturut-turut pada audit dokumentasi terakhir. Belum diuji di produksi: login operator, CSRF, mutasi, tampilan Swagger UI di browser, dan tes Playwright `qa/`.

## Pemecahan masalah

| Gejala | Penyebab dan solusi |
|---|---|
| Build gagal `use of internal package ... not allowed` | `api/index.go` mengimpor `internal/...`. Impor lewat `siagars/backend/serverless` |
| `/health` 200 tetapi `/ready` 503 dan data 500, durasi 2-5 detik | Atlas menolak koneksi: tambahkan IP ke Network Access (di sini `0.0.0.0/0`) |
| 503 `service_unavailable` di semua endpoint | Env var hilang atau tidak valid (`config.Load` gagal). Cek env Production lalu redeploy |
| Push tidak memicu deploy | `createDeployments` masih `disabled`, atau Root Directory bukan `backend` |
| Login ditolak (Origin) | `PUBLIC_ORIGIN` tidak sama persis dengan origin browser |

Log runtime: `vercel logs --deployment <url-deployment>` dari root repo (folder yang punya `.vercel/`; dari `backend/` CLI menjawab "not linked").

## Deviasi dari `DEPLOY.md` dan risiko yang diketahui

Pemilik proyek menyetujui deploy ini. Beberapa hal berbeda dari checklist go-live di `DEPLOY.md`:

1. **Atlas `0.0.0.0/0`**: IP egress Vercel tidak statis, jadi allowlist per IP tidak praktis. Mitigasi: user DB Atlas berhak minimal untuk database `siagars`, password kuat. Alternatif aman butuh Vercel Secure Compute atau private endpoint (berbayar).
2. **`/docs` dan `/docs/openapi.json` publik** (200), karena rewrite mengirim semua path ke Go. `DEPLOY.md` menghendaki keduanya tidak terjangkau. Bila tidak diinginkan, blokir lewat `vercel.json` (mis. rewrite ke 404) atau nonaktifkan rute di router.
3. **Rate limit in-memory** (`internal/http/server.go`): tidak dibagi antar instance dan reset saat cold start. `TRUSTED_PROXIES` tidak diisi, sehingga `X-Forwarded-For` tidak dipercaya dan identitas klien memakai alamat peer langsung; kemungkinan semua klien berbagi satu bucket per instance. Belum diverifikasi. Cukup untuk demo, tidak untuk produksi sungguhan.
4. **Tanpa reverse proxy, TLS, dan HSTS buatan sendiri**: TLS ditangani Vercel. Cookie sesi `Secure` karena `PUBLIC_ORIGIN` HTTPS.
5. **Deployment Protection**: `ssoProtection` proyek bernilai `all_except_custom_domains`, tetapi domain produksi terbukti dapat diakses publik tanpa login Vercel (uji di atas). URL deployment unik dan Preview bisa berperilaku berbeda.
6. **Kredensial**: `.env` dan `atlas-credentials.env` ada di disk pengembang. Rotasi password Atlas dan secret SatuSehat sebelum repo dibuka atau dibagikan, lalu perbarui env var di Vercel dan redeploy.
7. **Region**: fungsi berjalan di `sin1`, sedangkan region default proyek `iad1`. Latensi ke Atlas bergantung pada region cluster.

## Staging (branch `develop`)

Branch `develop` dideploy sebagai Preview dan dipakai frontend (Arya) selama pengembangan; `main` tetap production. Push ke `develop` memicu deploy Preview otomatis, push ke `main` memicu production.

Preview deployment tidak memakai env Production. Tambahkan di Vercel > Settings > Environment Variables dengan **Environments = Preview** (bila ada pilihan branch, isi `develop`):

| Variabel | Nilai staging |
|---|---|
| `APP_ENV` | `development` bila `PUBLIC_ORIGIN` berupa `http://localhost:3000`; `production` bila FE di-deploy dengan https (lalu password operator min 12 byte) |
| `PUBLIC_ORIGIN` | Origin browser FE, persis (mis. `http://localhost:3000` atau `https://fe-siagars.vercel.app`). Tipe **biasa** |
| `MONGODB_URI` | URI Atlas yang sama |
| `MONGODB_DATABASE` | `siagars_dev` (**bukan** `siagars`) |
| `SESSION_TTL_HOURS`, `DEMO_MODE` | `8`, `true` |
| `SATUSEHAT_ENABLED` | `false` (sync dijalankan dari lokal) |

Setelah menambah env, picu deploy ulang (Redeploy atau push). URL stabil branch: `https://<project>-git-develop-<team>.vercel.app` (lihat tab Deployments). Preview dilindungi *Vercel Authentication* (tetap aktif). Frontend mengaksesnya dengan *Protection Bypass for Automation*: header `x-vercel-protection-bypass: <secret>` dikirim dari server FE (lihat `docs/API_FRONTEND.md` bagian 2a). Secret dibaca di dashboard (Settings > Deployment Protection) atau lewat `vercel api /v9/projects/<id>` (kunci di `protectionBypass`), dibagikan lewat kanal privat, dan tidak masuk git. Env Preview (branch `develop`) sudah diisi lewat CLI: `MONGODB_URI`, `MONGODB_DATABASE=siagars_dev`, `APP_ENV=development`, `PUBLIC_ORIGIN=http://localhost:3000`, `SESSION_TTL_HOURS`, `DEMO_MODE`, `SATUSEHAT_ENABLED=false`. Env hanya berlaku pada deployment baru (push baru atau `vercel redeploy <url>`).

Akun uji operator dibuat dari lokal ke `siagars_dev` (`go run . operator` dengan `.env` dev) dan dibagikan lewat kanal privat, tidak di git. Rilis: merge `develop` ke `main`, jalankan `migrate` terhadap database production lebih dulu.
