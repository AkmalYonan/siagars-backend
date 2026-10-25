# SiagaRS Backend v0.1

Backend Go untuk direktori rumah sakit pilot, provenance, tracking rujukan sintetis, dan operator. Acuan: [PRD.md](PRD.md), [AGENTS.md](AGENTS.md), [keputusan implementasi](docs/DECISIONS.md), dan [kontrak OpenAPI](docs/openapi.yaml).

## Stack dan struktur

Go 1.27.1 (toolchain yang terpasang dan diuji), net/http ServeMux, MongoDB Atlas melalui go.mongodb.org/mongo-driver/v2 v2.9.2, bcrypt cost 12. Versi dependency dikunci dalam go.mod/go.sum.

- main.go (root): shortcut development `go run .` yang memuat .env lalu menjalankan API atau perintah manage.
- cmd/api: HTTP server dengan graceful shutdown (entrypoint produksi, hanya membaca process environment).
- cmd/manage: migration, seed, dan provisioning operator lokal.
- internal/http: routing, DTO, validasi input, session, CSRF, Origin, limiter.
- internal/service: pencarian, AND filter, pagination, sort.
- internal/domain: entity, state machine, freshness, null/0, Haversine.
- internal/repository: MongoDB, scope RS, transaksi status/history/audit, optimistic locking.
- internal/integrations/satusehat: OAuth2 TokenProvider, client MSI Sandbox read-only, DTO dan mapper.
- internal/service/satusehat_sync.go: pagination + summary sinkronisasi; internal/repository/satusehat_import.go: upsert MongoDB.
- migrations: file JSON bernomor (create collection + validator `$jsonSchema`, index) embedded, lock dokumen dan checksum.
- seed: fixture sintetis idempotent.
- internal/app: logika bersama untuk root main, cmd/api dan cmd/manage.
- docs/openapi.yaml: kontrak OpenAPI 3.0.3 (JSON adalah subset YAML), di-embed dan disajikan Scalar API Reference di /docs.
- tests/integration: pengujian DB dalam database temporer terisolasi.

## Cara cepat

Isi `.env` di root project (salin dari `.env.example`; file ini git-ignored), lalu dari folder backend:

~~~powershell
go run .                  # muat .env lalu jalankan API di http://localhost:8080
go run . migrate          # tersedia: migrate | seed | operator | sync-satusehat | curate | dummy-facilities
~~~

`go run .` membaca `.env` tanpa menimpa variabel yang sudah di-set di terminal, jadi override sementara tetap bisa, misalnya `$env:SATUSEHAT_ENABLED='true'; $env:SATUSEHAT_KABKOTA='jabodetabek'; go run . sync-satusehat`. Log hanya mencatat jumlah variabel yang dimuat, bukan nilainya. `cmd/api` dan `cmd/manage` tetap tersedia dan sengaja tidak memuat `.env` (untuk container/produksi).

## Dokumentasi API (Scalar)

Saat API berjalan, buka http://localhost:8080/docs. Spec mentah ada di http://localhost:8080/docs/openapi.json (dari `docs/openapi.yaml` yang di-embed ke binary, dengan server di-set relatif `/api/v1`). Kedua path berada di luar `/api/v1`, jadi tidak mengubah kontrak API; di deployment same-origin, reverse proxy hanya meneruskan `/api/v1/*`, sehingga /docs hanya tersedia bila port Go diakses langsung.

- UI Scalar (3 kolom, pencarian Ctrl+K, dark mode, contoh kode, tombol Test Request) dimuat dari jsDelivr (`@scalar/api-reference@1.67.0`) dengan hash SRI dan CSP ketat; halaman butuh internet untuk aset tersebut. Endpoint dikelompokkan per tag Publik/Auth/Admin.
- "Try it out" untuk endpoint GET langsung jalan. Login dan mutasi memeriksa Origin terhadap daftar `PUBLIC_ORIGIN` (cocok persis); dari halaman /docs Origin-nya `http://localhost:8080`, jadi akan 403 kecuali saat development PUBLIC_ORIGIN di-set ke `http://localhost:8080`. Mutasi juga butuh header X-CSRF-Token dari `/auth/csrf`.

## Development vs Production

Kode sama, environment berbeda lewat env var dan database:

| | Development | Production |
|---|---|---|
| Jalan di | laptop (`go run .`) | Vercel (branch `main`) |
| Config | `backend/.env` (git-ignored) | env var dashboard Vercel |
| Database | `siagars_dev` | `siagars` |
| `PUBLIC_ORIGIN` | `http://localhost:3000` | `https://siagars-backend.vercel.app` |
| Operator | akun uji lokal (password min 8 byte saat `APP_ENV=development`) | akun kuat, min 12 byte |

Alur: kerja dan push ke branch `develop` (dideploy sebagai staging Vercel Preview untuk frontend, env dan database terpisah; lihat `deploy/VERCEL.md` bagian Staging); merge `develop` ke `main` hanya saat siap rilis (production). Riwayat versi API untuk FE: `docs/CHANGELOG.md`. Setup DB dev: set `MONGODB_DATABASE=siagars_dev` di `.env`, lalu `go run . migrate`, `go run . seed`, `go run . operator`.

## Jalankan lokal (PowerShell)

Prasyarat: Go 1.27.1 dan cluster MongoDB Atlas (replica set; transaksi multi-dokumen wajib, sehingga mongod standalone tidak didukung). IP mesin harus masuk Network Access Atlas. Go module download membutuhkan internet pada instalasi pertama. Tidak ada auto-migration/seed saat startup.

Credential Atlas disimpan di atlas-credentials.env (git-ignored lewat pola `*.env`). Program hanya membaca process environment; muat file itu ke sesi PowerShell:

~~~powershell
cd backend
Get-Content atlas-credentials.env | Where-Object { $_ -match '^\s*([A-Z_]+)\s*=\s*"?(.*?)"?\s*$' } | ForEach-Object { Set-Item "Env:$($Matches[1])" $Matches[2] }
$env:MONGODB_DATABASE='siagars'
$env:APP_ENV='development'
$env:PUBLIC_ORIGIN='http://localhost:3000'
$env:DEMO_MODE='true'
$env:SATUSEHAT_ENABLED='false'
go mod download
go run ./cmd/manage migrate
go run ./cmd/manage seed
go run ./cmd/api
~~~

MONGODB_URI wajib (mongodb:// atau mongodb+srv://); MONGODB_DATABASE default siagars. MONGODB_USERNAME/MONGODB_PASSWORD di file Atlas tidak dibaca terpisah karena sudah termuat dalam URI. GET http://localhost:8080/api/v1/health untuk liveness; /api/v1/ready memeriksa koneksi dan schema_migrations. Tanpa DB, health tetap 200 dan ready 503. Readiness bukan verifikasi checksum penuh; jalankan migrate sebelum API.

## Model data MongoDB

| Koleksi | Isi |
|---|---|
| regions | `_id`=kode wilayah, parent_code, kind, name |
| services | `_id`=slug, name, category |
| hospitals | profil + provenance, `services[]` dan `beds[]` embedded dengan provenance dan version per item |
| operators | email unik, password_hash, active, `hospital_ids[]` (scope RS) |
| sessions | `_id`=hash token, csrf_hash, expires_at (TTL 1 hari setelah kedaluwarsa), revoked_at |
| referrals, referral_status_history, audit_logs | rujukan sintetis, riwayat publik, audit internal |
| schema_migrations, locks | catatan migration dan lock migrate |

Constraint ditegakkan validator server (enum, range, null vs 0, available<=total, slug/kategori unik per RS) dan unique index. Tidak ada FK di MongoDB; keberadaan RS diperiksa aplikasi saat membuat operator.

## Jalankan menggunakan Docker Compose

Docker tidak tersedia di lingkungan implementasi awal; konfigurasi ini disiapkan tetapi belum dieksekusi. Compose hanya menjalankan API; database tetap di Atlas. Setelah env Atlas dimuat seperti di atas:

~~~powershell
docker compose build api
docker compose run --rm --entrypoint /app/manage api migrate
docker compose run --rm --entrypoint /app/manage api seed
docker compose up -d api
~~~

Password pada URI harus di-percent-encode jika mengandung karakter reserved URL. Tidak ada perintah reset/drop dalam workflow ini.

## Produksi (Vercel)

API produksi berjalan di https://siagars-backend.vercel.app lewat adapter serverless (`api/index.go`, `serverless/`, `vercel.json`) dan CI/CD dari GitHub (push ke `main` memicu deploy). Smoke test `health`, `ready`, `hospitals` dan `services` lulus pada 24 Okt 2026. Setup, env vars, pemecahan masalah, dan deviasi keamanan (Atlas `0.0.0.0/0`, `/docs` publik, rate limit per instance) ada di [`deploy/VERCEL.md`](deploy/VERCEL.md). Migrate, seed, dan pembuatan operator tetap dijalankan dari lokal.

## Operator lokal

Tidak ada akun/password bawaan. Jalankan setelah seed, dengan email baru:

~~~powershell
$env:OPERATOR_EMAIL='operator@example.invalid'
$env:OPERATOR_PASSWORD='ISI_PASSWORD_LOKAL_MINIMAL_12_KARAKTER'
$env:OPERATOR_HOSPITAL_IDS='10000000-0000-4000-8000-000000000001'
go run ./cmd/manage operator
Remove-Item Env:OPERATOR_PASSWORD
~~~

Password 12..72 byte, bcrypt cost 12. Perintah operator hanya membuat akun baru dan penugasan dalam satu transaksi; email duplikat ditolak, tidak mereset password. Daftar beberapa UUID dipisah koma. Semua ID RS harus sudah ada. Di Compose, teruskan env operator dengan docker compose run -e OPERATOR_EMAIL -e OPERATOR_PASSWORD -e OPERATOR_HOSPITAL_IDS --rm --entrypoint /app/manage api operator.

## Sinkronisasi SATUSEHAT MSI Sandbox

Perintah manual read-only yang mengimpor Rumah Sakit (`jenis_sarana=104`, `status_aktif=true`) dari Master Sarana Index Sandbox ke koleksi `hospitals`. API publik tetap membaca MongoDB; kegagalan SATUSEHAT tidak memengaruhi API dan tidak menghapus data lama.

Prasyarat: `go run ./cmd/manage migrate` sudah menerapkan `002_satusehat_msi.json`, serta credential Sandbox sah milik tim (bukan dari Postman publik). Jangan menyimpan credential di repo.

~~~powershell
# MONGODB_URI/MONGODB_DATABASE sudah dimuat seperti di atas
$env:SATUSEHAT_ENABLED='true'
$env:SATUSEHAT_ENV='sandbox'
$env:SATUSEHAT_CLIENT_ID='...'
$env:SATUSEHAT_CLIENT_SECRET='...'
# opsional: SATUSEHAT_PAGE_LIMIT (1..2000, default 200), SATUSEHAT_HTTP_TIMEOUT_SECONDS (1..60, default 10)
# opsional: SATUSEHAT_KABKOTA = kosong (nasional) | jabodetabek | daftar kode 4 digit, mis. 3171,3276
$env:SATUSEHAT_KABKOTA='jabodetabek'
go run ./cmd/manage sync-satusehat
Remove-Item Env:SATUSEHAT_CLIENT_SECRET
~~~

Output hanya angka: `pages fetched created updated unchanged skipped failed duration` dan alasan skip. Exit non-zero bila halaman gagal (dilaporkan nomor halaman + kategori, misalnya `auth_rejected`, `rate_limited`, `upstream_unavailable`, `timeout`, `invalid_response`) atau ada record ditolak database.

Perilaku:
- OAuth2 client_credentials ke `api-satusehat-stg.dto.kemkes.go.id`; token hanya di memory, cache berdasarkan `expires_in` dengan margin 60 detik (seperempat umur untuk token pendek), singleflight. Request OAuth tidak di-retry.
- MSI: timeout per request, body dibatasi 8 MB, 401 refresh token sekali, 403/400/422 tanpa retry, 429 mematuhi Retry-After (maks 60 detik dan dalam deadline), 5xx/timeout maksimal 2 retry dengan exponential backoff + jitter. Timeout total command 30 menit; Ctrl+C membatalkan.
- Cakupan wilayah: dengan SATUSEHAT_KABKOTA, setiap kab/kota diminta terpisah memakai parameter resmi `kode_kabkota` dan ditelusuri sampai halaman terakhir. Record yang kab/kotanya di luar cakupan di-skip (`out_of_scope`). Preset `jabodetabek` = 3101, 3171–3175, 3201, 3271, 3276, 3216, 3275, 3603, 3671, 3674.
- Mapping: `kode_satusehat`→external_id (namespace `satusehat:msi:sandbox`), nama, alamat, telp, provinsi/kabkota (kode 2/4 digit, kab/kota harus berawalan kode provinsi), latitude/longitude berpasangan (kosong/invalid/out of range/0,0 → keduanya null), status_aktif, status_sarana, kode_sarana, jenis/subjenis/kelas sarana. Record tanpa ID/nama/wilayah valid atau bukan 104 di-skip dan dihitung.
- Tidak diisi dari MSI: bpjs (null), services/beds (array kosong), source_updated_at dan verified_at (null) sehingga freshness `unknown`. synced_at/last_seen_at hanya waktu pengambilan.
- Upsert per halaman dengan kunci (source_namespace, external_id). `$set` hanya field milik MSI; `_id` UUID lokal, bpjs, services, beds dibuat sekali via `$setOnInsert` dan tidak pernah ditimpa. Record yang isinya sama hanya memperbarui synced_at/last_seen_at (dihitung unchanged).
- Region baru disisipkan bila belum ada; region yang sudah ada tidak diganti namanya. Kode yang bentrok kind/parent menolak record terkait (failed), bukan menimpa.
- Fasilitas yang tidak muncul pada satu run tidak dihapus/nonaktifkan. `source_active=false` hanya bila sumber eksplisit mengirim nonaktif; dokumen tersebut tidak tampil di API. Karena query memakai `status_aktif=true`, fasilitas yang dinonaktifkan upstream saat ini tidak terdeteksi otomatis.

Status: adapter teruji dengan mock `httptest.Server`; importer teruji integration MongoDB Atlas. **Live Sandbox teruji 23 Okt 2026** untuk Jabodetabek: 14 kab/kota, 14 halaman, 516 RS dibuat, 0 skip/gagal (±6 detik); run kedua 516 unchanged (idempotent). Hanya MSI Sandbox read-only; production, FHIR dan rujukan resmi tidak disentuh. Sync nasional belum dijalankan.

## Kontrak dan alur frontend

Panduan implementasi untuk tim frontend tersedia di [docs/API_FRONTEND.md](docs/API_FRONTEND.md). Prefix /api/v1. Lihat docs/openapi.yaml untuk seluruh endpoint, enum, request dan response. Semua response memiliki X-Request-ID dan Cache-Control: no-store. JSON snake_case, timestamp UTC, UUID lowercase. Data list saat ini memberi Hospital lengkap (termasuk services/beds); frontend dapat memakai subset summary. Map memakai halaman aktif yang sama.

1. GET /services, lalu /hospitals?services=nicu,icu&bpjs=true. Daftar memakai `dataset=real` secara default (semua data selain simulasi); `dataset=simulation` hanya 30 RS fiktif demo, `dataset=all` keduanya. Filter layanan/BPJS pada dataset real hanya cocok untuk RS yang datanya memuat nilai tersebut; RS MSI belum punya layanan/bed/BPJS, jadi hasilnya kosong sampai ada sumber kurasi. Filter layanan AND mencakup available/limited, termasuk simulasi berlabel. Unknown berbeda dari false, null bed berbeda dari 0.
2. GET /hospitals/{id} untuk detail. source_updated_at menentukan freshness; synced_at dan verified_at tidak menggantikannya.
3. POST /auth/login dengan JSON email/password dan Origin yang sama persis dengan PUBLIC_ORIGIN. Simpan cookie melalui browser (credentials: include).
4. GET /auth/me untuk scope; GET /auth/csrf untuk token. Login juga mengembalikan csrf_token. Semua PATCH operator dan POST logout membawa Origin + X-CSRF-Token.
5. GET /admin/referrals; PATCH /admin/referrals/{id}/status dengan version saat ini. Konflik 409 harus diikuti refetch.
6. POST /referrals/lookup dengan {"tracking_code":"SR-DEMO-001"} menampilkan status/history publik terbaru. Ini skenario sintetis terbuka.
7. POST /auth/logout mencabut session; cookie dihapus.

Next.js development menggunakan rewrite /api/v1/:path* ke http://localhost:8080/api/v1/:path*. Pertahankan Origin browser http://localhost:3000. Tidak ada CORS wildcard. Cookie Path=/api/v1, HttpOnly, SameSite=Lax, Secure pada HTTPS. HTTP hanya di APP_ENV=development.

Mutasi demo selalu menandai origin simulation serta mengosongkan source_url/verified_at. Catatan publik hanya untuk teks simulasi aman; jangan memasukkan data personal/medis. Tidak ada endpoint pembuatan rujukan publik.

## Seed dan data

30 RS fiktif bernama RS Simulasi Siaga 01..30, 5 master layanan, 4 kategori bed per RS, dan 1 rujukan dengan history submitted -> verified -> searching. Wilayah master memakai pilot Jabodetabek; alamat dan koordinat RS adalah sintetis, telepon null. Sepuluh profil pertama lengkap untuk skenario status/bed, tetapi bukan profil kurasi RS nyata dan belum memenuhi target verifikasi dataset.

Semua provenance berlabel simulation, verified_at/source_url null. Sebagian BPJS/bed null, sebagian available=0 untuk menguji unknown. Kode SR-DEMO-001 bukan token rahasia; hanya referensi skenario publik sintetis. Token session acak 256 bit disimpan SHA-256; token tracking hanya disimpan hash.

Seed memakai upsert `$setOnInsert` dan `$push` bersyarat dalam satu transaksi. Pengulangan tidak mengubah status, password, timestamp ataupun pembaruan operator yang sudah ada. Seed tidak menyegarkan timestamp lama.

## Kebijakan pencarian dan keamanan

- Query hospitals tidak dikenal, kosong, berulang, enum/range salah: 422. Unknown field body / malformed JSON: 400. Body >64 KB: 413.
- Pagination default20/max50; page maksimal1.000.000 sebagai batas defensif.
- Sort name lalu ID. distance membutuhkan lat/lng; jarak garis lurus Haversine, koordinat unknown terakhir.
- Match: layanan non-simulation available +10 atau limited +5, freshness fresh +3/aging +1 per layanan; lalu jarak jika ada, nama/ID. Bukan skor kualitas klinis. Semua simulasi tidak mendapat bonus faktual.
- has_available_beds=true hanya available>0, fresh dan non-simulation. Dengan seed simulasi, hasil filter ini memang kosong. false tidak memfilter.
- Freshness: <1 jam fresh; 1..24 jam aging; >24 jam stale; null/future unknown. Threshold baseline tetap pada domain; belum menjadi setting environment.
- Login maksimum 5 gagal/15 menit/IP+akun, ceiling 100 request/15 menit/IP; lookup 10/menit/IP. Limiter memory per instance (bounded 10.000 key). Retry-After 60 detik adalah interval coba ulang, bukan jaminan window sudah habis.
- `X-Forwarded-For` hanya dipercaya bila peer langsung cocok dengan `TRUSTED_PROXIES`. Nilai kosong mengabaikan header. Limiter tetap berada di memory setiap instance, sehingga deployment multi-instance memerlukan limiter bersama untuk perlindungan produksi.
- Request context timeout10 detik; header5 detik; transport read/write15 detik. Tidak mencatat body, email, cookie, password, kode tracking, URL query atau raw DB error.
- State machine dan pemeriksaan versi dijalankan saat row lock, status+history+audit satu transaksi.
- Operator services/bed hanya dapat update item yang sudah ada pada RS scope. Data admin read juga dibatasi scope.
- DEMO_MODE=true wajib. SATUSEHAT_ENABLED=false default; bila true hanya SATUSEHAT_ENV=sandbox yang diterima (production ditolak) dan client ID/secret wajib. API server tidak memanggil SATUSEHAT; hanya `manage sync-satusehat`.

## Verifikasi

~~~powershell
gofmt -l cmd internal migrations seed tests
go build ./...
go vet ./...
go test ./...
go test -race ./...
~~~

DB integration bersifat opt-in; gunakan database pengujian terpisah. Test membuat database siagars_test_<timestamp> pada cluster yang sama dan hanya men-drop database yang dibuatnya sendiri. Tidak mereset database pengguna:

~~~powershell
$env:TEST_MONGODB_URI=$env:MONGODB_URI
go test -v ./tests/integration
~~~

Tes: seluruh pasangan transisi, freshness boundary, Haversine, null/0, filter AND/BPJS/bed, pagination, input invalid, Origin/CSRF/session, limiter, body limit dan DTO publik. Suite DB memeriksa migration/seed idempotent, rollback audit failure, dua concurrent update dengan versi sama, scope, expiry/revoke, bed constraint dan DTO joins.

Status aktual 24 Okt 2026: gofmt, build, vet, unit test, contract test, dan suite integrasi Atlas tercatat lulus. Migration 001-003, seed idempotent, query pushdown, compare RS, trusted proxy, serta benchmark pencarian sudah tersedia. Race detector tidak berjalan karena CGO nonaktif dan tidak ada GCC. E2E frontend, Docker, dan kurasi RS nyata belum terbukti. Pembaruan 25 Okt 2026: migration 004 (layanan `rawat-inap`, index `external_id`), 19 operasi (tambah `/regions`, `/hospitals/suggest`, `/admin/hospitals`, `/admin/hospitals/{id}`), compare/detail menerima kode SATUSEHAT, dan `facility_summary` pada DTO RS; detail di `docs/DECISIONS.md` dan `docs/API_FRONTEND.md`. Data ICU/NICU/PICU/bed/BPJS RS nyata tetap unknown sampai dikurasi.

Status fase SATUSEHAT MSI (23 Okt 2026): gofmt, build, vet dan seluruh unit test lulus, termasuk adapter OAuth/MSI dengan mock httptest dan workflow sync dengan fake source. Suite integrasi MongoDB (TestMongoTransactions dan TestSATUSEHATImport) lulus terhadap Atlas pada database tes terisolasi. Migration 002 sudah diterapkan ke database siagars. Live Sandbox Jabodetabek berhasil (lihat bagian Sinkronisasi SATUSEHAT).

## Handoff

Must API dan fitur Should backend tersedia dalam kode. Alur frontend tetap menjadi gate berikutnya. Jangan menyatakan integrasi medis produksi siap dari hasil pengujian demo. Migration 001-003 tidak boleh diedit; perubahan skema berikutnya wajib memakai migration baru. Deployment Vercel berjalan dan lolos smoke test dasar (lihat [deploy/VERCEL.md](deploy/VERCEL.md)); login operator, mutasi, dan tes `qa/` terhadap produksi belum dijalankan. Deployment container Docker belum diuji pada mesin ini.

docs/fixtures.json berisi contoh sintetis deterministik untuk handoff frontend; bukan snapshot database yang sedang berjalan.
## Kurasi data RS (`curate`)

Pipeline untuk menambahkan fakta layanan/bed/BPJS dari sumber publik yang sah ke RS hasil MSI. Detail format, cara mencari sumber, kriteria 10 profil lengkap, dan checklist verifikasi manusia ada di [data/curation/README.md](data/curation/README.md). Berkas contoh seluruhnya fiktif (`.invalid`).

- Default **dry-run**: validasi dan laporan, tanpa koneksi database. `--apply` baru menulis, dan menolak data bersumber placeholder.
- Validator menolak data tanpa `source_url`/`source_updated_at`, selalu mengisi `data_origin=curated`, **tidak pernah** mengisi `verified_at` (berkas yang memuatnya ditolak), dan menjaga `null` berbeda dari `0`.
- Apply hanya menambah ke RS MSI yang sudah ada (cocok `source_namespace`+`external_id`): layanan/bed yang sudah ada, termasuk pembaruan operator, tidak ditimpa; `bpjs` hanya diisi bila masih null.
- Tanpa perubahan endpoint, enum, migration, OpenAPI, atau fixture.

~~~powershell
go run ./cmd/manage curate data/curation/example.placeholder.json   # dry-run; contoh placeholder akan ditolak untuk apply
~~~

Argumen `curate <file> [--apply]` didukung oleh `go run .` dan `go run ./cmd/manage`. `CURATE_FILE` serta `CURATE_APPLY=true` tetap tersedia untuk otomasi berbasis environment. `go test ./seed` menguji validator dan dry-run. `tests/integration/curate_test.go` memerlukan `TEST_MONGODB_URI` dan menguji apply, idempotensi, serta non-overwrite pada database sementara.

## Perbandingan RS (compare)

`GET /api/v1/hospitals/compare?ids=<uuid>,<uuid>[,<uuid>]` mengembalikan detail lengkap 2-3 RS dengan urutan sesuai `ids` (`{data:[...], meta:{as_of,count}}`). ID duplikat, kurang dari 2, lebih dari 3 atau bukan UUID lowercase: 422; ada ID tidak ditemukan: 404 dengan pesan menyebut ID itu dan tanpa hasil parsial. RS simulasi dan real dapat dibandingkan sekaligus. Alasan desain: `docs/DECISIONS.md` (paket G).

## Data contoh fasilitas (`dummy-facilities`)

SATUSEHAT sandbox tidak punya ICU/NICU/PICU/bed/rawat inap/BPJS. `go run ./cmd/manage dummy-facilities` menempelkan data contoh deterministik ke RS impor (bukan RS simulasi) di database yang dipilih `MONGODB_DATABASE`. Semua item berlabel `data_origin=simulation`, tanpa sumber, tanpa `verified_at`; BPJS memakai `bpjs_data_origin=simulation`. Hanya menambah, tidak menimpa data yang ada, aman diulang. **Jalankan hanya pada database dev/staging (`siagars_dev`), bukan production.** `curate` menggantikan item simulasi yang belum diubah operator dengan data bersumber.
