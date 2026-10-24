# PRD - SiagaRS Backend

Versi: 0.1 (revisi 24 Oktober 2026) | Baseline: 23 Oktober 2026 | Status: API Must dan fitur Should backend sudah diimplementasikan di MongoDB Atlas. Benchmark pencarian dan deployment Vercel tersedia; frontend, Docker, dan kurasi RS nyata belum terbukti. Lihat bagian 11 untuk status aktual.

Catatan revisi: penyimpanan pada baseline awal PostgreSQL; sejak 23 Oktober 2026 tim memindahkannya ke MongoDB Atlas (lihat docs/DECISIONS.md). Kontrak HTTP, enum dan makna null tidak berubah; parameter `dataset` (aditif) kini didokumentasikan di bagian 3.

Tim MalasNgoding. Ketua: Radja Admiral. Backend: Akmal Bariq Yonanda. Frontend/integrasi: M. Tauriq Arya.

## 1. Tujuan dan cakupan

Sediakan REST API Go untuk direktori pilot RS, pencarian fasilitas, provenance dan kesegaran data, tracking rujukan sintetis serta pengelolaan operator. API harus tetap berfungsi dari data tersimpan di MongoDB Atlas (seed sintetis dan hasil impor sandbox terakhir) ketika akses SATUSEHAT belum tersedia atau gagal. Next.js adalah proyek terpisah. MongoDB Atlas, Next.js dan Go merupakan pilihan tim; PostgreSQL pada baseline awal sudah digantikan.

Target produk: pengguna menemukan profil RS yang cocok dalam kurang dari 60 detik pada uji usability. Target ini belum terbukti. V0.1 tidak menyediakan diagnosis, triase, rekam medis, data pasien nyata, pembayaran, klaim BPJS atau rujukan medis produksi. Tidak ada endpoint membuat rujukan publik; seed/demo admin menyiapkan skenario sintetis.

Must: API publik, auth operator, update layanan/bed/status, audit, validasi, migrations, seed, dokumentasi OpenAPI dan deployment reproducible. Should: query bed, sort jarak/kecocokan, dukungan compare melalui detail maksimal tiga ID. Could: adapter SATUSEHAT sandbox; adapter MSI Sandbox read-only ini sudah ada di kode (perintah `manage sync-satusehat`, lihat bagian 7 dan 11). Integrasi produksi memerlukan izin, verifikasi akses dan scope penggunaan tersendiri dan tidak ada di kode.

## 2. Pemilik dan arsitektur

Akmal memiliki model data, REST API, autentikasi, migrations dan adapter. Tauriq mereview kebutuhan DTO dan integrasi. Radja memvalidasi scope/konten/QA sebagai usulan pembagian kerja.

Struktur: `cmd/api`, `cmd/manage`, `internal/app`, `internal/config`, `internal/http`, `internal/service`, `internal/repository`, `internal/domain`, `internal/integrations/satusehat`, `migrations`, `seed`, `docs/openapi.yaml`, `tests/integration`. Gunakan satu service Go dan MongoDB Atlas (replica set, karena transaksi multi-dokumen dipakai); Redis/microservices tidak diperlukan V0.1.

Alur: HTTP handler -> service business rules -> repository -> MongoDB Atlas. Adapter upstream hanya dipanggil import job (`manage sync-satusehat`), bukan oleh server API, dan tidak diekspos sebagai proxy FHIR bebas. Context, deadline, filter BSON terstruktur (bukan query dari string input) dan structured log digunakan konsisten. Router memakai net/http ServeMux dan driver go.mongodb.org/mongo-driver/v2; versi dikunci pada go.mod/go.sum.

Deployment utama same-origin: `/api/v1/*` diteruskan reverse proxy ke Go; path lain ke Next.js. Development FE rewrite ke localhost:8080. Forwarding Next.js yang tipis dapat dipakai jika hosting perlu; Go tetap pemilik auth dan bisnis. Cross-origin bukan default; jika terpaksa harus meninjau cookie, CORS allowlist, CSRF dan HTTPS sebelum rilis.

## 3. Kontrak API V0.1

Prefix `/api/v1`. JSON snake_case. ID opaque string UUID; timestamp RFC3339 UTC; FE memformat Asia/Jakarta. Sukses `{data,meta?}`; error `{error:{code,message,fields?},request_id}`. Error tidak memuat detail database/secret. Request ID juga pada header respons.

| Method | Path | Otorisasi dan hasil |
|---|---|---|
| GET | /health | Liveness, tanpa detail sensitif |
| GET | /ready | Readiness DB; 503 jika belum siap |
| GET | /services | Master slug layanan |
| GET | /hospitals | Search/filter/pagination |
| GET | /hospitals/{id} | Detail termasuk layanan dan bed |
| POST | /referrals/lookup | Kode demo di body; rate limited |
| POST | /auth/login | Login operator; set cookie session |
| GET | /auth/me | Session valid; profil+RS izin |
| GET | /auth/csrf | Session valid; token CSRF |
| POST | /auth/logout | Session+CSRF; revoke |
| GET | /admin/referrals | Session, hanya RS operator |
| PATCH | /admin/referrals/{id}/status | Session+CSRF+RS scope+version |
| PATCH | /admin/hospitals/{id}/services/{slug} | Session+CSRF+RS scope+version |
| PATCH | /admin/hospitals/{id}/beds/{category} | Session+CSRF+RS scope+version |

Semua path tabel relatif terhadap prefix. Method salah menghasilkan 405, resource hilang 404, JSON malformed 400, validasi 422, tanpa session 401, scope salah/CSRF gagal 403, version/transisi bentrok 409, rate limit 429, server error 500. Kode lookup tidak valid/hilang memakai pesan generik 404.

### Query rumah sakit

`dataset=real|simulation|all` (default `real`): real = semua `data_origin` selain simulation (curated, satusehat_sandbox, satusehat_production), simulation = hanya 30 RS fiktif seed, all = keduanya. Parameter ini aditif, tetapi default membuat RS simulasi tidak tampil tanpa `dataset=simulation|all`; frontend demo harus mengirimnya. `meta.dataset` memantulkan nilai yang berlaku. Pemisahan hanya pada daftar; detail `/hospitals/{id}` dan tujuan lookup SR-DEMO-001 tetap dapat memuat RS simulasi. Setiap item tetap membawa `data_origin`; `satusehat_sandbox` adalah data Sandbox, bukan produksi dan bukan bukti kapasitas.

`q` max 100 karakter; `province_code`, `city_code` dari master wilayah lokal (RS hasil MSI memakai kode MSI tanpa titik, mis. 3171, sedangkan seed sintetis memakai kode bertitik; keduanya region terpisah); `services=nicu,icu` menggunakan AND; `bpjs=true|false` menyaring nilai diketahui; `has_available_beds=true` opsional hanya bed fresh, non-simulation dan available>0. Nilai false/unknown tidak tertukar. `lat` [-90,90], `lng` [-180,180] harus berpasangan. `sort=name|distance|match`; distance butuh lat/lng; default name dengan ID sebagai tie-break. `page>=1`, `limit` default 20, maksimum 50. Query tak didukung ditolak atau diabaikan sesuai OpenAPI yang disepakati, bukan diam-diam mengubah semantics; baseline menolak parameter tidak dikenal dengan 422.

Map V0.1 memakai halaman aktif yang sama dengan daftar; tidak membutuhkan endpoint bbox. `meta` berisi page, limit, total, total_pages, as_of, data_scope=pilot dan dataset. Jarak Haversine garis lurus dalam kilometer. Match sort jika diaktifkan: cocok filter wajib, status layanan, freshness, lalu jarak jika ada; bukan skor kualitas klinis atau jaminan diterima. Data simulasi tidak dianggap bukti layanan nyata.

### Bentuk DTO

Hospital summary: id, name, city_code, city_name, province_code, province_name, latitude/longitude nullable, phone nullable, bpjs nullable, distance_km nullable, data_origin, services ringkas. Detail menambah address, source_url, source_updated_at, synced_at, verified_at, services dan beds. Setiap item layanan/bed punya provenance sendiri karena pembaruan tidak selalu serempak.

Service item: slug, name, status, data_origin, source_url, source_updated_at, verified_at, synced_at, freshness, version. Bed item: category, total nullable, available nullable, metadata yang sama. Tidak tersedia pada sumber -> null, tidak diubah menjadi nol.

Freshness `fresh | aging | stale | unknown`: sumber timestamp valid <1 jam fresh; 1-24 jam aging (tepat 24 jam masih aging); >24 jam stale; null/future tidak valid unknown. Threshold adalah kebijakan demo yang dikonfigurasi, bukan standar medis. Gunakan waktu server dan meta.as_of; usia sinkronisasi bukan usia sumber.

Data origin `curated | simulation | satusehat_sandbox | satusehat_production`. Simulasi selalu berlabel sekalipun timestamp fresh. `verified_at` hanya diisi setelah pemeriksaan nyata; jangan auto-verify hasil import. `source_updated_at` operator demo boleh diisi waktu update, origin tetap simulation. Produksi tidak otomatis boleh dipublikasikan hanya karena API bisa dibaca.

### Lookup tracking

Body `{tracking_code: string}`. Kode demo tidak memuat identitas, minimal 128 bit randomness untuk token aktual; kode ramah manusia seperti SR-DEMO-001 hanya untuk skenario publik sintetis. Hash token disimpan, kode tidak masuk log/URL. Respons hanya public_status, public_note, next_step, destination hospital summary, updated_at dan history yang disanitasi. `Cache-Control: no-store`; rate limit 10 lookup/menit/IP adalah default awal yang dituning. Tidak ada internal note, actor identity atau data medis dalam respons.

### Mutasi contoh

Status body `{status:"confirmed",public_note:"Konfirmasi simulasi diterima",version:3}`. Respons memberi DTO baru dengan version 4. Operator catatan publik max 500 karakter, tanpa identitas/medis. Layanan body `{status:"limited",version:2}`. Bed body `{total:10,available:2,version:1}`. Waktu mutasi berasal dari server. Validasi 0<=available<=total jika kedua nilai diketahui; total/available null diizinkan sebagai unknown. Nilai negatif ditolak.

## 4. State machine rujukan sintetis

`submitted -> verified -> searching -> confirmed -> completed`.

Transisi tambahan: `verified -> rejected`; `searching -> rejected`; `confirmed -> searching` (pengalihan, catatan alasan wajib). `completed` dan `rejected` terminal. Transisi lain 409. Pengalihan menggunakan event dan tetap status searching; jangan menambah enum tanpa kontrak bersama. Referrals memakai RS pemilik untuk scope operator, destination dapat berubah dalam demo yang di-seed; perubahan tujuan harus diaudit jika endpoint dibuat kelak.

Status update dan insert history+audit dijalankan dalam satu transaksi. Gunakan optimistic locking (filter `_id` + `version` pada update, transaksi multi-dokumen MongoDB), increment version saat sukses. Dua request dengan version sama: hanya satu berhasil, lainnya 409; tidak boleh ada dua history untuk perubahan yang sama. Current status harus konsisten dengan history terakhir.

## 5. Model data dan constraint

Penyimpanan: MongoDB Atlas, database `siagars` (default `MONGODB_DATABASE`). Koleksi:

- regions: `_id`=kode wilayah, parent_code, kind, name; pilot memuat provinsi/kota yang dipakai serta region baru dari impor MSI.
- services: `_id`=slug, name, category.
- hospitals: `_id` UUID lokal, external_id nullable + source_namespace (unik), name, address, kode wilayah, koordinat nullable berpasangan, phone, bpjs nullable, provenance, `source_active`, `services[]` dan `beds[]` embedded. Setiap item embedded punya provenance dan version sendiri; update per item atomik (positional `$` + `$elemMatch` pada version). Slug layanan dan kategori bed unik per RS; status layanan enum; bed total/available nullable nonnegatif dengan available<=total jika known.
- operators: email normalized unik, password_hash, active, `hospital_ids[]` (membatasi tindakan ke RS yang ditugaskan).
- sessions: `_id`=hash token, csrf_hash, operator_id, expires_at, revoked_at; tanpa token mentah; TTL index menghapus dokumen 1 hari setelah expires_at.
- referrals: id, owner_hospital_id, destination_hospital_id nullable, tracking_token_hash unik, status, public_note, next_step, version, is_demo=true.
- referral_status_history: referral_id, from_status nullable, to_status, public_note, actor_id nullable, created_at.
- audit_logs: actor, action, entity_id, perubahan field aman, request_id, created_at; akses internal saja.
- schema_migrations dan locks: catatan migration bernomor (dengan checksum) dan lock migrate.

Integritas ditegakkan oleh validator server `$jsonSchema`/`$expr` (enum, range, null vs 0, available<=total, lat/lng berpasangan, aturan origin non-curated/simulation wajib external_id dan source_namespace) dan unique index. MongoDB tidak punya FK: keberadaan RS dicek aplikasi saat membuat operator, dan seed konsisten dengan master wilayah. Tidak ada endpoint penghapusan sehingga tidak ada cascade yang menghilangkan audit/riwayat. Nama kota/provinsi/layanan di-join di Go dari koleksi master.

Pencarian mendorong filter dasar, pencarian teks, pengurutan nama, hitungan, dan paginasi ke MongoDB. `sort=distance` dan `sort=match` masih mengurutkan himpunan yang sudah difilter di Go. Migration 003 menyediakan index pencarian; benchmark Atlas untuk 3.030 dokumen tercatat di `docs/DECISIONS.md`. Haversine tetap dihitung di Go; geo index tidak dipakai. Seed idempotent tidak menghapus database dan tidak menimpa pembaruan operator.

## 6. Autentikasi dan operasi

Hash password melalui library tepercaya (Argon2id atau bcrypt dengan parameter tercatat), bukan plain text. Login rate limit default 5 gagal/15 menit/IP+akun; pesan generik. Session random >=256 bit, hash disimpan; TTL awal 8 jam; revoke pada logout dan rotate setelah login. Cookie `siagars_session` HttpOnly, SameSite=Lax, Secure pada HTTPS, Path=/api/v1, tanpa Domain luas. Development HTTP lokal boleh Secure=false hanya APP_ENV=development.

Mutasi terautentikasi wajib validasi Origin same-origin dan CSRF token session. Login memeriksa Origin dan content-type JSON; JSON mutasi memerlukan X-CSRF-Token. Session tanpa scope rumah sakit tidak dapat update. Jangan mengandalkan hide button di UI. Batasi body 64 KB, request timeout lokal 10 detik, upstream 5 detik sebagai baseline. Trusted proxy eksplisit agar rate limit tidak mempercayai X-Forwarded-For sembarang.

Konfigurasi (nama sesuai kode): APP_ENV, PORT=8080, MONGODB_URI (wajib), MONGODB_DATABASE (default siagars), PUBLIC_ORIGIN, SESSION_TTL_HOURS=8, DEMO_MODE=true, SATUSEHAT_ENABLED=false, SATUSEHAT_ENV=sandbox, SATUSEHAT_CLIENT_ID, SATUSEHAT_CLIENT_SECRET, serta opsional SATUSEHAT_PAGE_LIMIT, SATUSEHAT_HTTP_TIMEOUT_SECONDS dan SATUSEHAT_KABKOTA. SATUSEHAT_ORGANIZATION_ID tidak dipakai karena MSI tidak memerlukannya. `.env.example` hanya placeholder; credential Atlas dan SATUSEHAT tidak masuk repo. Konfigurasi menolak SATUSEHAT_ENV=production dan tidak ada base URL production di kode; tidak ada fallback diam-diam ke data pasien nyata.

## 7. Adapter SATUSEHAT

Public collection adalah dokumentasi yang terbuka, bukan pemberian akses tanpa syarat. Master Sarana Index memakai OAuth2 client_credentials server-to-server dan credential partner terverifikasi. FHIR Location/Organization menjelaskan struktur organisasi/lokasi; tidak membuktikan setiap RS punya kapasitas/bed/BPJS lengkap yang boleh dipublikasikan. Rujukan resmi menggunakan Task, ServiceRequest, CarePlan dan Encounter; mesin status lokal bukan pengganti protokol itu.

Implementasi saat ini (`internal/integrations/satusehat`, `internal/service/satusehat_sync.go`, `internal/repository/satusehat_import.go`): TokenProvider OAuth2 client_credentials (token hanya di memory, cache berdasarkan expires_in dengan margin, singleflight), client MSI Sandbox read-only untuk Rumah Sakit (`jenis_sarana=104`, `status_aktif=true`), mapper defensif dan importer upsert per halaman dengan kunci (source_namespace, external_id). Dijalankan manual lewat `manage sync-satusehat`; server API tidak memanggil SATUSEHAT. Field yang tidak tersedia dari MSI tetap unknown: bpjs null, services/beds kosong, source_updated_at dan verified_at null sehingga freshness `unknown`; synced_at hanya waktu pengambilan. Field milik pengguna/operator (`bpjs`, services, beds, `_id`) tidak pernah ditimpa impor.

SATUSEHAT_ENABLED=false secara default; bila true hanya SATUSEHAT_ENV=sandbox diterima dan client ID/secret wajib. Tidak ada POST/PUT/PATCH ke SATUSEHAT. Timeout/retry bounded dengan backoff untuk GET aman; 401 refresh sekali, 429 patuhi Retry-After, request OAuth tidak di-retry. Command memberi ringkasan angka (pages, fetched, created, updated, unchanged, skipped, failed, durasi) dan log tanpa credential. Last good data tetap disajikan dengan timestamp lama jika upstream gagal; jangan membuat timestamp segar palsu. Fasilitas yang tidak muncul pada satu run tidak dihapus.

Belum ada: FHIR Location/Organization, rujukan resmi, produksi, dan sinkronisasi nasional.

## 8. Pengujian dan Definition of Done

- Unit: freshness boundaries, null/0, bed constraints, semua transisi legal/ilegal dan comparator jarak.
- Integration MongoDB Atlas (database tes sementara yang dibuat dan di-drop oleh suite itu sendiri, bukan database `siagars`): migrations up pada DB kosong, seed idempotent, rollback transaksi, optimistic lock, RS scope, unique/validator dan auth/session/CSRF.
- Contract: respons sesuai OpenAPI, filter AND, pagination, 401/403/409/422/429, public DTO tidak membocorkan catatan internal.
- Adapter: mock HTTP (`httptest.Server`) untuk timeout, 401, 429, invalid payload dan fallback; tes tidak memerlukan credential nyata.
- E2E bersama FE: login operator -> update status -> lookup menampilkan event terbaru. Map search tetap bekerja ketika adapter dimatikan.
- Target internal API p95 <=500 ms untuk read lokal 50 RS pada lingkungan dan concurrency tercatat; tidak menghitung upstream/tiles. Ukur dahulu, jangan mengklaim SLA produksi.

Definition of Done: build berhasil; gofmt, go vet, tests relevan lulus; migrations, seed, health/readiness, README, .env.example dan OpenAPI tersedia; tidak ada secret dalam repo/log; fresh install teruji.

Command nyata (lihat README.md): `go run .` (memuat .env), `go run ./cmd/manage migrate|seed|operator|sync-satusehat`, `go run ./cmd/api`, `gofmt -l cmd internal migrations seed tests`, `go build ./...`, `go vet ./...`, `go test ./...`; `go test -race ./...` membutuhkan CGO dan GCC (belum tersedia di lingkungan implementasi awal).

## 9. Delivery dan risiko

23-24 Okt kontrak/schema; 25-26 Okt API publik+seed+migration; 27-29 Okt auth/operator/tracking/integrasi; 30 Okt QA; 31 Okt freeze draft; 1 Nov First Draft. 3-6 Nov review; 7 Nov feedback; 8-10 Nov revisi; 11 Nov final submission. Jam pengumpulan belum dikonfirmasi (lihat lomba/CHECKLIST_LOMBA.md). Jangan menunda Must karena akses upstream.

Risiko terbesar: data pilot tidak lengkap, konfigurasi origin frontend, drift kontrak, limiter per instance pada Vercel, dan scope berlebih. Mitigasi: metadata field-level, same-origin, OpenAPI sebagai sumber kontrak, query pushdown teruji, adapter opsional, dan prioritas Must.

## 10. Referensi

- Dokumen lomba GuideBook: Guidebook WIA, PPT Technical Meeting MIA 2026, Q_A MIA 2026 - Sheet2, [WIA] Mentoring Class MIA 2026.
- https://www.postman.com/satusehat/satusehat-public/overview
- https://satusehat.kemkes.go.id/platform/docs/id/master-data/master-sarana-index/rest-api-msi/
- https://satusehat.kemkes.go.id/platform/docs/id/interoperability/rujukan/
- https://satusehat.kemkes.go.id/platform/docs/id/fhir/resources/location/

Referensi teknis diakses 23 Oktober 2026. Endpoint eksternal, field yang dapat diakses dan izin penggunaan harus diuji memakai akses sah sebelum implementasi adapter dinyatakan selesai.

## 11. Status aktual (24 Oktober 2026)

Sumber: README.md dan docs/DECISIONS.md; angka berasal dari log pengujian yang tercatat di sana, bukan dari pengukuran baru.

- Terbukti: gofmt, build, vet, unit test, contract test, dan suite integrasi Atlas tercatat lulus. Migration 001-003, seed idempotent, query pushdown, compare 2-3 RS, trusted proxy, dan benchmark pencarian tersedia. Adapter MSI teruji dengan mock dan live Sandbox 23 Oktober 2026 untuk Jabodetabek: 14 kab/kota, 516 RS dibuat, 0 skip/gagal; run kedua 516 unchanged. Deployment Vercel aktif dan endpoint health, ready, hospitals, services, serta OpenAPI merespons pada verifikasi 24 Oktober 2026.
- Seed sintetis: 30 RS fiktif (RS Simulasi Siaga 01..30), 5 layanan, 4 kategori bed, 1 rujukan demo; seluruh data berlabel simulation.
- Belum terbukti: E2E dengan frontend, Docker/Compose, race detector, kurasi RS nyata, usability <60 detik, sinkronisasi nasional, produksi SATUSEHAT, FHIR, dan rujukan resmi.
- Keterbatasan diketahui: limiter berada di memory setiap instance; threshold freshness belum konfigurabel; fasilitas yang hilang dari hasil upstream tidak otomatis dinonaktifkan; data MSI belum memiliki layanan, bed, atau BPJS; query `q` berawalan `$` memiliki temuan terbuka pada jalur agregasi MongoDB.
