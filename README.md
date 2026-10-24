# SiagaRS Backend v0.1

**Temukan rumah sakit yang tepat, bukan sekadar yang terdekat.**

SiagaRS membantu keluarga pasien mencari rumah sakit berdasarkan kebutuhan fasilitas, melihat sumber dan umur setiap informasi, serta melacak progres rujukan simulasi. Versi 0.1 adalah prototipe lomba Web In Action (MIA 2026) oleh Tim MalasNgoding; seluruh data rujukan dan operator bersifat sintetis dan tidak ada data pasien nyata.

> Bukan layanan medis. SiagaRS tidak memberikan diagnosis atau triase, tidak menyimpan rekam medis, dan tidak memproses rujukan medis resmi.

## Struktur repo

| Folder | Isi |
|---|---|
| `backend/` | API Go (net/http) + MongoDB Atlas, migrasi, seed, dokumentasi OpenAPI |
| `qa/` | Tes API Playwright (TypeScript) |
| `lomba/` | Dokumen lomba (checklist, draft submission, pitch) |

## Tech stack

- Go 1.27.1, `net/http` ServeMux
- MongoDB Atlas (replica set) lewat `go.mongodb.org/mongo-driver/v2`
- Auth: session cookie HttpOnly + CSRF, password bcrypt
- Dokumentasi API: OpenAPI 3.0.3 (`backend/docs/openapi.yaml`), Swagger UI di `/docs`
- Integrasi opsional: SATUSEHAT MSI Sandbox (read-only, nonaktif secara default)

## Cara menjalankan

Prasyarat: Go 1.27.1 dan cluster MongoDB Atlas (replica set).

```powershell
git clone <url-repo>
cd <repo>/backend

# Salin .env.example menjadi .env (tidak masuk git), lalu isi nilai milik sendiri
go mod download
go run . migrate
go run . seed
go run .        # API di http://localhost:8080
```

Cek: `GET /api/v1/health` dan `/api/v1/ready`. Dokumentasi interaktif di `/docs`. Detail lengkap ada di [`backend/README.md`](backend/README.md).

## Pengujian

```powershell
cd backend
go build ./...
go vet ./...
go test ./...
```

## Progres

- [x] REST API Go + MongoDB Atlas (pencarian RS, detail, master layanan)
- [x] Provenance dan kesegaran data per item
- [x] Tracking rujukan sintetis dan auth operator
- [x] Migrasi, seed idempotent, dokumentasi OpenAPI
- [x] Adapter SATUSEHAT MSI Sandbox read-only
- [x] Deployment produksi di Vercel dengan CI/CD dari GitHub: https://siagars-backend.vercel.app (smoke test dasar lulus; detail dan risiko di [`backend/deploy/VERCEL.md`](backend/deploy/VERCEL.md))
- [ ] Uji login operator dan tes `qa/` terhadap produksi

## Keamanan

Jangan commit `.env` atau `atlas-credentials.env`. Hanya `.env.example` yang boleh masuk repo.
