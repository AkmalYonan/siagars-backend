# AGENTS.md — SiagaRS Backend (v1)

Instruksi untuk developer & coding agent di repo `siagars-backend`. Berlaku sejak 26 Okt 2026, menggantikan "Versi kerja V0.1" di `backend/AGENTS.md`. **Baca dulu:** `new/PRD.md`, `new/02_ROADMAP_PHASE_VERSI.md` (cari tiket `BE-xx` Anda), `new/03_KONTRAK_API_v1.md`, lalu `backend/docs/openapi.yaml`, `backend/docs/DECISIONS.md`, `go.mod`.

## Mandat & kepemilikan
- BE: **Akmal Bariq Yonanda**. FE: M. Tauriq Arya. PM/approver scope: Radja Admiral.
- Kerjakan tiket ber-owner **BE** atau **X** (bagian API). Perubahan di luar tiket → tanya PM.
- Sub-tema: **Data-Driven Decision Support**. BE adalah mesin keputusan: `/recommendations`, `data_quality`, `/insights/*`.

## Kontrak
- `backend/docs/openapi.yaml` = sumber kebenaran; `new/03_KONTRAK_API_v1.md` = ringkasan keputusan. Keduanya harus konsisten.
- Prefix `/api/v1`, JSON snake_case, UUID lowercase, RFC3339 UTC, response `no-store` (kecuali insights: `public, max-age=300`).
- Jangan ubah endpoint, enum, pagination, makna null, atau rumus skor tanpa: update OpenAPI + `docs/fixtures.json` + `docs/CHANGELOG.md` + kontrak `new/`, dan persetujuan FE. Rumus berubah → naikkan `algorithm_version`.
- Param query tak dikenal/kosong/berulang → 422; field body tak dikenal → 400; error memakai skema `Error` seragam.
- `services` AND; limit default 20 / maks 50 (rekomendasi maks 20, markers maks 500).
- Jarak = Haversine garis lurus, bukan ETA atau kualitas klinis.

## Desain kode
- Lapisan: `internal/http` → `internal/service` → `internal/repository` → `internal/domain`. Handler tidak menyentuh Mongo langsung.
- **Scoring & data quality = fungsi murni** di `internal/domain` (input struct + `now`, tanpa I/O) dengan test tabel.
- Pakai `context` dengan deadline; filter BSON terstruktur; teks pengguna lewat `$literal`/regex ter-escape — **tidak pernah** merakit query dari string input.
- Migrasi JSON bernomor di `migrations/`, embedded, checksum, idempoten. **Jangan edit 001–005.** Baru: 006 (bpjs no raw), 007 (region codes), 008 (geo/insights index), 009 (rate limits).
- Mutasi status + history + audit dalam **satu transaksi**; optimistic locking `version` → 409.
- State machine rujukan: submitted→verified→searching→confirmed→completed; verified/searching→rejected; confirmed→searching wajib alasan; completed/rejected terminal.
- Scope operator diperiksa server di setiap read/write admin; jangan percaya `hospital_id` dari klien.

## Data & kejujuran
- `null` = tidak diketahui, tidak pernah 0/false. Bed 0 ≠ unknown.
- Freshness dari `source_updated_at` (fresh < 1 jam, aging 1–24 jam, stale > 24 jam, unknown null/masa depan).
- Data `simulation` tidak pernah di-set `verified_at`; job refresh **hanya** menyentuh item `simulation`, tidak pernah `curated`.
- Kurasi wajib `source_url` https + `source_updated_at`; dry-run dulu, `--apply` setelah review PM.
- Jangan menyatakan SATUSEHAT tersambung/produksi bila hanya sandbox/mock.

## Auth, privasi, keamanan
- Session cookie `siagars_session` HttpOnly, Secure (HTTPS), SameSite=Lax, Path=/api/v1; simpan hash token; TTL 8 jam; revoke saat logout.
- Mutasi: Origin allowlist (`PUBLIC_ORIGIN`) + `X-CSRF-Token`. Login: rate limit + Origin + content-type.
- bcrypt cost 12; tidak ada akun/password hardcoded; akun via `manage operator` dari env.
- **BPJS:** tidak menyimpan NIK/nama mentah dalam bentuk apa pun (termasuk terenkripsi). Simpan tersamar + `nik_hmac` saja. Tidak ada endpoint reveal. Jangan log input BPJS atau kode tracking.
- `/internal/jobs/*` hanya dengan header `X-Job-Secret` (constant-time compare), tidak masuk OpenAPI publik.
- Secret (`MONGODB_URI`, `BPJS_HISTORY_KEY`, `JOB_SECRET`, `SATUSEHAT_*`) hanya di env — jangan commit, jangan kirim ke browser.
- Percayai `X-Forwarded-For` hanya dari `TRUSTED_PROXIES`.

## Perintah
```bash
cd backend
go run ./cmd/api                 # API lokal :8080
go run ./cmd/manage migrate      # juga: seed | operator | sync-satusehat | curate <file> [--apply] | dummy-facilities
gofmt -l . && go vet ./...
go test ./...
go test -race ./...              # di CI (ubuntu)
TEST_MONGODB_URI=... go test ./tests/integration/...
cd ../qa && npx playwright test  # E2E API terhadap staging
```
Jalankan yang relevan sebelum PR; laporkan bila tidak bisa dijalankan (mis. race lokal tanpa CGO).

## Git & rilis
- Branch `feat/BE-xx-slug` / `fix/BE-xx-slug` dari `develop`; PR ke `develop`; judul memuat ID tiket.
- Conventional Commits. Rilis: `develop` → staging → Go/No-Go PM → `main` → migrate prod → smoke → tag `vX.Y.Z` (sama dengan FE).
- **Batas tindakan:** jangan deploy production, menjalankan migrasi destruktif, reset database, mengirim data ke SATUSEHAT, atau mempublikasikan kredensial tanpa otorisasi PM/owner. Backup Atlas sebelum migrasi 006 di prod.

## Definition of Done (BE)
gofmt/vet/test hijau · test bermakna untuk perilaku baru (state machine, null vs 0, boundary, 409, 422, scope operator, scoring) · OpenAPI + fixtures + CHANGELOG diperbarui bila kontrak berubah · diuji di staging · PR berisi: perilaku berubah, file, perubahan kontrak/migrasi, check aktual yang dijalankan, cara menjalankan, keterbatasan.

## Dokumen
Sumber kebenaran: folder `new/` di root repo. `backend/PRD.md` (v0.1) dan handover lama di `backend/docs/` menjadi arsip di tiket BE-07.
