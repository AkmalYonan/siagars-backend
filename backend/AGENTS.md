# AGENTS.md - SiagaRS Backend

Instruksi untuk coding agent di repository backend Go. Versi kerja V0.1 diperbarui 24 Oktober 2026. Baca PRD.md, status workspace, go.mod, README.md, dan docs/DECISIONS.md sebelum mengubah kode.

## Mandat dan kepemilikan

- Akmal Bariq Yonanda pemilik backend; M. Tauriq Arya pemilik frontend/integrasi; Radja Admiral ketua/reviewer scope.
- Bangun Go REST API + MongoDB (Atlas) untuk pencarian RS, provenance, tracking sintetis, auth/operator dan audit.
- Implementasikan Must dahulu. Integrasi sandbox opsional dan dinonaktifkan default; layanan inti harus berjalan dengan seed data.
- Tidak ada data pasien nyata, NIK, diagnosis, pembayaran atau rujukan produksi pada V0.1.

## Kontrak

- `docs/openapi.yaml` adalah sumber kontrak saat implementasi dimulai; sertakan contoh sukses/galat dan enum.
- Prefix `/api/v1`, JSON snake_case, UUID opaque, RFC3339 UTC. FE menggunakan snapshot versi yang sama.
- API memiliki 15 operasi: health/readiness; layanan; daftar, detail, dan compare RS; lookup tracking; auth; serta operasi admin. `docs/openapi.yaml` memuat daftar lengkap dan menjadi sumber kontrak.
- Jangan mengubah endpoint, enum, pagination atau makna null tanpa memperbarui OpenAPI, fixture dan koordinasi frontend.
- `services` AND; limit default 20/max50; parameter unknown 422. Jarak Haversine garis lurus, bukan ETA atau ukuran kualitas klinis.
- Unknown = null/status unknown. Bed 0 bukan unknown. Freshness terpisah dari origin/verifikasi dan dihitung berdasarkan source_updated_at, bukan synced_at.

## Desain kode dan database

- Pisahkan HTTP handlers, business service, repository dan adapter. Gunakan context/deadline dan filter BSON terstruktur (tanpa merakit query dari string input).
- Migration bernomor dan reproducible; jangan edit migration 001-003 yang sudah tercatat. Seed idempotent tanpa reset database default.
- Constraint FK/unique/check dan index harus mendukung integritas yang tertulis pada PRD.
- Mutasi status+history+audit satu transaksi; optimistic locking version menghasilkan 409 untuk stale write.
- State machine submitted->verified->searching->confirmed->completed. verified/searching->rejected; confirmed->searching wajib alasan pengalihan. completed/rejected terminal.
- Scope RS operator diperiksa server pada setiap read/write admin; jangan mempercayai hospital_id dari frontend.
- Endpoint lookup hanya memproyeksikan history/catatan publik yang aman. Jangan membocorkan operator identity, internal note atau token hash.

## Auth, data dan integrasi

- Session cookie HttpOnly, Secure pada HTTPS, SameSite=Lax, Path=/api/v1; server menyimpan hash token, bukan token mentah. TTL default 8 jam dan revoke saat logout.
- Mutasi session wajib Origin validation + X-CSRF-Token. Login juga memeriksa Origin/content-type dan rate limit.
- Gunakan hash password dari library tepercaya. Tidak ada akun demo dengan password hardcoded di git; credential lokal dikonfigurasi terpisah.
- Tracking token acak >=128 bit; kode yang mudah ditebak hanya boleh berisi skenario demo publik sintetis. Rate limit lookup/login; jangan log token/kode mentah.
- SATUSEHAT client_id/client_secret/token hanya di backend environment; jangan commit atau mengirim ke browser.
- Jangan menganggap public Postman collection memberi akses produksi. Gunakan credential sah dan hak akses spesifik. Jangan meminjam Organization ID pihak lain.
- `SATUSEHAT_ENABLED=false` dan `DEMO_MODE=true` secara default. Tidak ada transaksi rujukan resmi atau akses rekam medis saat demo.
- Deployment produksi saat ini memakai adapter serverless Vercel. Docker dan reverse proxy adalah jalur alternatif yang belum tervalidasi.
- Adapter bertipe, timeout/retry bounded, token expiry dari response dan provenance field-level. Kegagalan upstream mempertahankan data lama dengan label stale; jangan memalsukan pembaruan/verifikasi.
- Batasi body, validasi enum/range, redact log, gunakan request ID dan percayai forwarding header hanya dari proxy terdaftar.

## Verifikasi dan handoff

Target command setelah setup: `go run ./cmd/api`, `go build ./cmd/api`, `go test ./...`, `go vet ./...`. Format file Go yang diubah dengan gofmt. Jalankan race detector jika toolchain mendukung; laporkan bila tidak tersedia. Migration/seed command mengikuti tool nyata yang dipilih dan README.

Uji bermakna: state machine, race dua update, transaksi rollback, operator lintas RS ditolak, invalid CSRF, expiry/logout, null/0, filter AND, freshness boundary, response DTO, adapter timeout/429. Gunakan test DB terpisah; jangan reset DB pengguna. Uji adapter memakai mock, bukan credential produksi.

Setiap pekerjaan selesai mencantumkan: perilaku berubah, file, perubahan kontrak/migration, check aktual, cara menjalankan dan keterbatasan. Jangan menyatakan endpoint live atau SATUSEHAT tersambung jika hanya mock.

## Batas tindakan

Jangan deploy publik, melakukan destructive migration, reset database, kirim data ke SATUSEHAT atau mempublikasikan credential tanpa otorisasi pengguna. Pertahankan pekerjaan pengguna yang ada. Jika akses upstream terblokir, lanjutkan jalur seed/mock yang disepakati dan nyatakan sumber data secara akurat.
