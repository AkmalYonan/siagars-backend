# PRD Backend — SiagaRS v1 (`siagars-backend`)

| | |
|---|---|
| Versi dokumen | 1.0 · 26 Okt 2026 |
| Owner | Akmal Bariq Yonanda (BE) |
| Approver | Radja Admiral (PM) |
| Menggantikan | `backend/PRD.md` v0.1 (23 Okt 2026) — menjadi arsip |
| Acuan bersama | [01_SCOPE](01_SCOPE_SIAGARS_A-Z.md) · [02_ROADMAP](02_ROADMAP_PHASE_VERSI.md) · [03_KONTRAK_API](03_KONTRAK_API_v1.md) |

> Catatan path: di repo `siagars-backend`, dokumen bersama berada di folder `new/` yang sama dengan file ini.

## 1. Ringkasan

Backend SiagaRS adalah REST API Go + MongoDB yang menjadi **mesin keputusan & sumber kebenaran data**: pencarian RS, provenance per nilai, **skor rekomendasi yang dapat dijelaskan**, **skor kualitas data**, **agregasi wawasan**, tracking rujukan sintetis, cek BPJS simulasi, dan operasi operator — dengan keamanan dan privasi yang dapat dipertanggungjawabkan.

## 2. Tujuan & metrik BE

| Tujuan | Metrik |
|---|---|
| Rekomendasi cepat & benar | p95 ≤ 500 ms `/recommendations` (warm); ≥ 15 kasus test tabel scoring lulus |
| Data jujur | 0 nilai simulasi tanpa label; ≥ 30 RS `curated` di prod (v0.5) |
| Andal saat dinilai | Smoke test prod lulus di setiap rilis; uptime ≥ 99% di 11 & 28 Nov |
| Aman | 0 secret di git; rate limit bersama (v0.6); tidak ada NIK mentah tersimpan |

## 3. Stack & arsitektur (tetap)

- Go 1.27.1, `net/http` ServeMux (tanpa framework), `mongo-driver/v2`, `x/crypto`, `x/sync`.
- MongoDB Atlas (replica set → transaksi). DB: `siagars` (prod), `siagars_dev` (dev/staging), `siagars_test_<ts>` (integrasi).
- Lapisan: `internal/http` (handler, DTO, validasi, middleware) → `internal/service` (logika bisnis) → `internal/repository` (Mongo) → `internal/domain` (model & fungsi murni). Adapter eksternal di `internal/integrations/satusehat`.
- Deploy: Vercel Serverless (`api/index.go` → `serverless/` → `internal/app/serverless.go`), region `sin1`. CLI `cmd/manage` untuk migrate/seed/operator/sync/curate/dummy.

## 4. Kebutuhan fungsional

### 4.1 v0.3.1 — Stabilisasi
| Tiket | Kebutuhan | Acceptance criteria |
|---|---|---|
| BE-01 | BPJS tanpa NIK mentah | Migrasi `006_bpjs_no_raw`: `$unset: {raw_enc: ""}` di `bpjs_check_history`, validator diperbarui; route reveal dihapus (404); `BPJS_HISTORY_KEY` hanya untuk HMAC; test: dokumen baru tidak memuat `raw_enc` |
| BE-02 | Rilis prod | `migrate` (005, 006) + `seed` peserta sintetis di `siagars`; env prod lengkap; smoke publik + operator lulus |
| BE-03 | Kode wilayah tunggal | Migrasi `007_region_codes` mengubah kode bertitik → tanpa titik di `regions` & `hospitals`, idempoten; seed menulis format baru; `/regions` tanpa duplikat |
| BE-04 | CI | `.github/workflows/ci.yml`: gofmt, vet, `go test ./...`, `go test -race ./...` (CGO on di ubuntu) |
| BE-05 | Regenerasi simulasi | `POST /internal/jobs/refresh-simulation` (header `X-Job-Secret` = env `JOB_SECRET`, constant-time compare) memanggil logika `dummy-facilities` untuk item `data_origin=simulation` saja; dijadwalkan Vercel Cron harian 05.00 WIB (`vercel.json` `crons`); audit log; 401 tanpa secret |
| BE-06 | Kontrak | OpenAPI 0.3.1: enum `DataOrigin \| null`, `csrf_token` optional, skema Error seragam |
| BE-08 | `hospital_name` | `GET /admin/referrals` menyertakan nama RS (lookup batch, bukan N+1) |

### 4.2 v0.4.0 — Decision Core
| Tiket | Kebutuhan | Acceptance criteria |
|---|---|---|
| BE-10 | `GET /recommendations` | Sesuai kontrak §4: filter keras, 5 komponen, renormalisasi bobot, penalti simulasi 0,8, level, urutan, `meta.excluded`, `algorithm_version`. Scoring = **fungsi murni** di `internal/domain/recommend.go` (tanpa I/O); kandidat diambil via repository dengan filter wilayah/radius (bounding box kasar di DB, Haversine di Go) |
| BE-11 | `data_quality` | Fungsi murni `domain.DataQuality(h, now)` sesuai kontrak §5; dihitung saat render DTO (bukan disimpan) |
| BE-12 | `GET /hospitals/markers` | bbox valid & ≤ 2°×2°, maks 500, proyeksi field minimal; index `{dataset:1, lat:1, lng:1}` (migrasi `008_geo_insights_indexes`) |
| BE-13 | Rilis 0.4.0 | OpenAPI + fixtures (contoh rekomendasi minimal 3 RS, termasuk simulasi & unknown) + CHANGELOG + tag |

**Test wajib BE-10:** layanan unavailable dikeluarkan · bpjs=false dikeluarkan saat required · null tidak pernah = 0 · tanpa lat/lng → komponen jarak dihapus & bobot dinormalisasi · penalti simulasi · seri diurutkan jarak lalu nama · radius boundary · 422 tanpa lokasi · contoh perhitungan di kontrak §4.4 menghasilkan 92 dan 80.

### 4.3 v0.5.0 — Insights & Data
| Tiket | Kebutuhan | Acceptance criteria |
|---|---|---|
| BE-20 | `/insights/{summary,coverage,service-gaps,freshness}` | Aggregation pipeline di repository; filter `province_code`, `dataset`; `Cache-Control: public, max-age=300` + cache in-memory 5 menit; `meta.generated_at`, `meta.sources`; test integrasi terhadap dataset seed menghasilkan angka yang diharapkan |
| BE-21 | Import kurasi | `manage curate <file>` dry-run → review PM → `--apply`; ≥ 30 RS `curated` di prod; `verified_at` tetap tidak di-set |
| BE-22 | Audit curate & job | `audit_logs` entri per eksekusi (aktor = `system:curate`/`system:cron`, jumlah perubahan) |
| BE-23 | Riwayat BPJS | `GET /admin/bpjs-checks` hanya field tersamar + paginasi `page/limit` |

### 4.4 v0.6.0 — Hardening
| Tiket | Kebutuhan | Acceptance criteria |
|---|---|---|
| BE-30 | Rate limiter bersama | Koleksi `rate_limits {_id: key+window, count, expires_at}` + TTL (migrasi `009_rate_limits`), `$inc` atomik; fallback in-memory bila DB error (fail-open untuk publik, fail-closed untuk login); `TRUSTED_PROXIES` di-set sesuai Vercel |
| BE-31 | Load test | Skrip `tests/load/` (k6 atau hey) untuk `/hospitals`, `/recommendations`, `/insights/*`; laporan p50/p95 di `docs/PERF.md` |
| BE-32 | Hardening | Rotasi kredensial Atlas/SATUSEHAT/operator; keputusan `/docs` publik dicatat; catatan Atlas network |
| X-30 | E2E API | `qa/` diperbaiki: hapus spec UI palsu-lulus (UI pindah ke repo FE), tambah spec `/recommendations` & `/insights`, jalan di CI terhadap staging |

### 4.5 v1.0.0 / v1.1.0
- BE-40: rilis prod + smoke + tag `v1.0.0`.
- BE-50: monitoring — cron ping `/ready` (mis. GitHub Actions terjadwal / UptimeRobot), alert ke kanal tim, warm-up 30 menit sebelum demo.
- BE-51: penyesuaian bobot skor dari usability (naikkan `algorithm_version`).

## 5. Model data (delta)

| Koleksi | Perubahan |
|---|---|
| `bpjs_check_history` | − `raw_enc` (006). Field: `nik_masked`, `name_masked`, `birth_year`, `nik_hmac`, `result`, `created_at` (TTL 30 hari) |
| `regions`, `hospitals` | kode wilayah tanpa titik (007) |
| `hospitals` | index `{dataset, lat, lng}` + index pendukung insights `{dataset, city_code, "services.slug", "services.status"}` (008) |
| `rate_limits` | baru (009), TTL di `expires_at` |
| `audit_logs` | jenis aksi baru: `curate.apply`, `job.refresh_simulation` |

Migrasi 001–005 **tidak boleh diubah**. Semua migrasi baru bernomor, idempoten, tercatat checksum.

## 6. Kebutuhan non-fungsional

- **Performa:** p95 ≤ 500 ms endpoint publik (warm, Atlas `sin1`/terdekat); `/recommendations` maks kandidat 500 sebelum scoring.
- **Keamanan:** session HttpOnly + hash, CSRF per sesi, Origin allowlist (`PUBLIC_ORIGIN`), bcrypt 12, scope operator di server, mutasi + history + audit dalam satu transaksi, optimistic locking (409), body ≤ 64 KB, filter BSON terstruktur (tanpa string query dari input; `$literal` untuk teks).
- **Privasi:** tidak ada NIK mentah; log tanpa input BPJS/kode tracking; data pasien nyata dilarang.
- **Kejujuran data:** null ≠ 0; freshness dari `source_updated_at`; simulasi tidak pernah di-set `verified_at`; job tidak pernah mengubah data `curated`.
- **Observabilitas:** `X-Request-ID` di setiap response & log; log terstruktur level info/warn/error.

## 7. Konfigurasi (env)

`MONGODB_URI`, `MONGODB_DB`, `APP_ENV`, `PUBLIC_ORIGIN` (csv), `TRUSTED_PROXIES`, `SESSION_TTL`, `BPJS_HISTORY_KEY` (HMAC), **`JOB_SECRET`** (baru), `SATUSEHAT_*` (lokal saja, `SATUSEHAT_ENABLED=false` default), `DEMO_MODE=true`, `OPERATOR_*` (hanya untuk `manage operator`). Tidak ada yang di-commit.

## 8. Test plan

| Lapisan | Lokasi | Fokus |
|---|---|---|
| Unit domain | `internal/domain/*_test.go` | scoring, data quality, freshness boundary, masking |
| Service | `internal/service/*_test.go` | filter keras, kandidat, insights shaping |
| HTTP | `internal/http/*_test.go` | validasi 422/400, auth, CSRF, rate limit, header |
| Kontrak | `tests/contract` | OpenAPI ↔ DTO ↔ fixtures |
| Integrasi | `tests/integration` (`TEST_MONGODB_URI`) | migrasi 006–009, transaksi, agregasi insights, rate limiter bersama |
| E2E API | `qa/` Playwright | alur publik & operator terhadap staging |
| Load | `tests/load` | p95 |

CI wajib: gofmt, vet, test, race. Integrasi berjalan di CI bila secret `TEST_MONGODB_URI` tersedia.

## 9. Rilis & deploy
- `develop` → Vercel Preview (staging, `siagars_dev`); `main` → production (`siagars`).
- Urutan rilis: PR → CI hijau → merge `develop` → `migrate` + uji staging → Go/No-Go PM → merge `main` → `migrate` prod (lokal ke Atlas) → smoke → tag `vX.Y.Z` (sama dengan FE) → CHANGELOG.
- Migrasi destruktif (006 `$unset`) wajib backup/snapshot Atlas sebelum dijalankan di prod.

## 10. Tiket BE (ringkas)

| Versi | Tiket |
|---|---|
| v0.3.1 | BE-01 … BE-08, X-01 |
| v0.4.0 | BE-10, BE-11, BE-12, BE-13 |
| v0.5.0 | BE-20, BE-21, BE-22, BE-23 |
| v0.6.0 | BE-30, BE-31, BE-32, X-30 (API) |
| v1.0.0 | BE-40 |
| v1.1.0 | BE-50, BE-51 |

## 11. Pertanyaan terbuka
1. Vercel Hobby cron hanya harian — cukup untuk BE-05? (asumsi: ya; bila perlu lebih sering, pakai GitHub Actions terjadwal).
2. `/docs` (Scalar) tetap publik? Usul: ya — dokumentasi API publik adalah nilai plus di rubrik (Q&A no. 34).
