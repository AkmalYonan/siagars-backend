# SiagaRS Backend

**Temukan rumah sakit yang tepat, bukan sekadar yang terdekat.**

SiagaRS membantu keluarga pasien mencari rumah sakit berdasarkan kebutuhan fasilitas, melihat sumber dan umur setiap informasi, serta melacak progres rujukan simulasi. Ini prototipe lomba Web In Action (MIA 2026) oleh Tim MalasNgoding; seluruh data rujukan, operator, peserta BPJS, dan data fasilitas contoh bersifat sintetis.

> Bukan layanan medis. SiagaRS tidak memberikan diagnosis atau triase, tidak menyimpan rekam medis, tidak memproses rujukan medis resmi, dan tidak terhubung ke BPJS Kesehatan.

Versi API: **0.3.0** di branch `develop` (staging), **0.2.1** di production (`main`). Riwayat versi: [`backend/docs/CHANGELOG.md`](backend/docs/CHANGELOG.md).

| | URL |
|---|---|
| API production | https://siagars-backend.vercel.app/api/v1 |
| Dokumentasi API (Scalar) | https://siagars-backend.vercel.app/docs |
| Frontend (repo terpisah) | https://frontend-siagars.vercel.app |

## Struktur repo

| Folder | Isi |
|---|---|
| `backend/` | API Go (net/http) + MongoDB Atlas, migrasi, seed, kurasi data, dokumentasi OpenAPI, adapter Vercel |
| `qa/` | Tes API Playwright (TypeScript) dan checklist QA manual |
| `lomba/` | Dokumen lomba (checklist, draft submission, template README, pitch) |

## Tech stack

- Go 1.27.1, `net/http` ServeMux
- MongoDB Atlas (replica set) lewat `go.mongodb.org/mongo-driver/v2`
- Auth: session cookie HttpOnly + CSRF + validasi Origin, password bcrypt
- Dokumentasi API: OpenAPI 3.0.3 (`backend/docs/openapi.yaml`), Scalar API Reference di `/docs`
- Deployment: Vercel serverless dengan CI/CD dari GitHub (`main` = production, `develop` = staging Preview)
- Integrasi opsional: SATUSEHAT Master Sarana Index Sandbox (read-only, manual, nonaktif secara default)

## Cara menjalankan

Prasyarat: Go 1.27.1 dan cluster MongoDB Atlas (replica set).

```powershell
git clone <url-repo>
cd <repo>/backend

# Salin .env.example menjadi .env (tidak masuk git), lalu isi nilai milik sendiri.
# Untuk development pakai MONGODB_DATABASE=siagars_dev dan PUBLIC_ORIGIN=http://localhost:3000.
go mod download
go run . migrate     # migration 001-005
go run . seed        # 30 RS simulasi, rujukan SR-DEMO-001, peserta BPJS sintetis
go run . operator    # opsional: akun operator dari OPERATOR_EMAIL/OPERATOR_PASSWORD/OPERATOR_HOSPITAL_IDS
go run .             # API di http://localhost:8080
```

Cek: `GET /api/v1/health` dan `/api/v1/ready`. Dokumentasi interaktif tersedia di `/docs`. Perintah lain (`sync-satusehat`, `curate`, `dummy-facilities`) dan detail backend ada di [`backend/README.md`](backend/README.md); panduan integrasi frontend ada di [`backend/docs/API_FRONTEND.md`](backend/docs/API_FRONTEND.md).

## Pengujian

```powershell
cd backend
go build ./...
go vet ./...
go test ./...
```

Tes integrasi MongoDB bersifat opt-in lewat `TEST_MONGODB_URI` dan memakai database sementara. Tes API Playwright: lihat [`qa/`](qa/) (`npm ci`, lalu `npx playwright test` dengan `SIAGARS_BASE_URL`).

## Progres

- [x] REST API Go + MongoDB Atlas: pencarian RS (filter layanan AND, BPJS, wilayah, bed, sort nama/jarak/kecocokan), detail, compare 2-3 RS, autocomplete, master layanan dan wilayah
- [x] Provenance dan kesegaran data per item, `facility_summary`, label `simulation` pada data contoh
- [x] 516 RS nyata Jabodetabek dari SATUSEHAT MSI Sandbox (read-only) + data contoh fasilitas berlabel simulasi
- [x] Tracking rujukan sintetis, auth operator, dashboard admin RS dalam scope
- [x] Cek kepesertaan BPJS **simulasi** (peserta sintetis) dan riwayat operator dengan enkripsi + TTL 30 hari (API 0.3.0, baru di staging)
- [x] Migrasi 001-005, seed idempotent, pipeline kurasi `curate`, dokumentasi OpenAPI
- [x] Deployment Vercel production dan staging; uji publik (32 pemeriksaan) dan login operator dari origin FE (15 pemeriksaan) lulus di production pada 25 Okt 2026 ([`backend/docs/HANDOVER_PRODUCTION_v0.2.1.md`](backend/docs/HANDOVER_PRODUCTION_v0.2.1.md))
- [ ] Rilis 0.3.0 (cek BPJS simulasi) ke production
- [ ] Suite `qa/` dijalankan terhadap production/staging
- [ ] Kurasi data RS nyata bersumber (layanan/bed/BPJS)
- [ ] Docker Compose dan race detector

## Keamanan

Jangan commit `.env`, `atlas-credentials.env`, `BPJS_HISTORY_KEY`, atau password operator. Hanya `.env.example` yang boleh masuk repo.
