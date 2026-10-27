# Template README untuk repository publik

Panduan: aturan panitia (Guidebook bagian J, PPT Technical Meeting) mewajibkan README.md di repository publik (GitHub/GitLab) berisi empat hal: judul + deskripsi singkat 1-2 kalimat, tech stack, cara menjalankan (clone, install, run), dan progres saat ini. Template di bawah garis memenuhi keempatnya. Salin isi di bawah garis ke README.md repo yang akan dikumpulkan, lalu ganti semua tanda **[ISI]**.

Aturan isi:
- Tanpa secret: jangan memasukkan isi `.env`, `atlas-credentials.env`, MONGODB_URI, password, client secret SATUSEHAT, atau token apa pun. Hanya nama variabel dan nilai placeholder.
- Periksa sebelum push: `.gitignore` backend sudah mengecualikan `.env`, `.env.*` dan `*.env` (kecuali `.env.example`). Pastikan riwayat commit juga bersih, bukan hanya berkas terbaru.
- Tulis hanya yang benar-benar berjalan. Perintah yang belum diuji harus diberi catatan.
- Backend dan frontend adalah dua proyek terpisah (Go dan Next.js). Putuskan apakah dikumpulkan sebagai dua repo atau satu; panitia hanya meminta satu link repo publik. **[ISI: keputusan tim]**
- Bagian cara menjalankan di bawah diambil dari backend/README.md. Bagian frontend sengaja kosong karena tidak ada di dokumen backend; frontend di-deploy terpisah di `https://frontend-siagars.vercel.app`.
- Root `README.md` repo backend sudah memakai isi template ini (diperbarui 26 Oktober 2026) dan dapat dipakai langsung bila repo backend yang dikumpulkan.

---

# SiagaRS

**Temukan rumah sakit yang tepat, bukan sekadar yang terdekat.**

SiagaRS membantu keluarga pasien mencari rumah sakit berdasarkan kebutuhan fasilitas, melihat sumber dan umur setiap informasi, serta melacak progres rujukan simulasi. Ini prototipe lomba Web In Action (MIA 2026) oleh Tim MalasNgoding; seluruh data rujukan, operator, peserta BPJS, dan data fasilitas contoh bersifat sintetis.

> Bukan layanan medis. SiagaRS tidak memberikan diagnosis atau triase, tidak menyimpan rekam medis, dan tidak memproses rujukan medis resmi. Ketersediaan bed pada prototipe tidak menjamin penerimaan pasien; konfirmasi selalu lewat kontak rumah sakit. Cek BPJS adalah simulasi dengan peserta sintetis, bukan layanan BPJS Kesehatan.

## Tim

Radja Admiral (ketua), M. Tauriq Arya (frontend/integrasi), Akmal Bariq Yonanda (backend). **[ISI: sesuaikan dengan nama lengkap pada pendaftaran]**

## Tech stack

| Bagian | Teknologi |
|---|---|
| Backend | Go 1.27.1, net/http ServeMux |
| Database | MongoDB Atlas (replica set) lewat go.mongodb.org/mongo-driver/v2 v2.9.2 |
| Auth | session cookie HttpOnly + CSRF, password bcrypt cost 12 |
| Dokumentasi API | OpenAPI 3.0.3 (`backend/docs/openapi.yaml`), Scalar API Reference di `/docs` |
| Deployment | Vercel serverless, CI/CD GitHub: `main` = production, `develop` = staging Preview |
| Integrasi opsional | SATUSEHAT Master Sarana Index Sandbox, read-only, nonaktif secara default |
| Frontend | Next.js, di-deploy di `https://frontend-siagars.vercel.app` **[ISI: versi, UI library, peta; tidak tercatat di dokumen backend]** |

Versi dependency Go dikunci di `go.mod`/`go.sum`.

## Cara menjalankan

### Backend

Prasyarat: Go 1.27.1 dan cluster MongoDB Atlas (replica set; mongod standalone tidak didukung karena transaksi multi-dokumen). IP mesin harus diizinkan di Network Access Atlas.

```powershell
git clone [ISI: URL repo]
cd [ISI: folder backend]

# 1. Salin .env.example menjadi .env (berkas ini tidak masuk git), lalu isi:
#    MONGODB_URI=mongodb+srv://<user>:<password>@<cluster>/...   (placeholder, isi milik sendiri)
#    MONGODB_DATABASE=siagars_dev    (production memakai siagars)
#    APP_ENV=development
#    PUBLIC_ORIGIN=http://localhost:3000
#    DEMO_MODE=true
#    SATUSEHAT_ENABLED=false
#    BPJS_HISTORY_KEY=<32 byte acak base64, opsional; tanpa kunci riwayat cek BPJS hanya tersamar>

go mod download
go run . migrate     # terapkan migration bernomor 001-005
go run . seed        # seed sintetis idempotent (30 RS fiktif, SR-DEMO-001, peserta BPJS sintetis)
go run .             # API di http://localhost:8080
```

Cek: `GET http://localhost:8080/api/v1/health` (liveness) dan `/api/v1/ready` (DB siap). Dokumentasi interaktif: http://localhost:8080/docs. Versi publik: `https://siagars-backend.vercel.app/api/v1` dan `https://siagars-backend.vercel.app/docs`.

Akun operator demo tidak disediakan dan tidak ada password bawaan. Buat sendiri setelah seed (email dan password milik sendiri, minimal 12 karakter; 8 karakter saat `APP_ENV=development`) dengan `go run . operator` dan variabel `OPERATOR_EMAIL`, `OPERATOR_PASSWORD`, `OPERATOR_HOSPITAL_IDS`; lihat backend/README.md.

Perintah `migrate`/`seed` tidak mereset atau menghapus database.

### Frontend

**[ISI: clone, install, run frontend. Di development, Next.js meneruskan `/api/v1/*` ke `http://localhost:8080` lewat rewrite; Origin browser harus sama dengan PUBLIC_ORIGIN.]**

### Data contoh fasilitas (opsional, hanya database dev/staging)

`go run . dummy-facilities` menempelkan data contoh ICU/NICU/PICU/bed/BPJS ke RS hasil impor SATUSEHAT. Semua item berlabel `simulation`, tanpa sumber, tidak menimpa data yang ada.

### Sinkronisasi data Sandbox SATUSEHAT (opsional)

Hanya jika tim punya credential Sandbox yang sah. Tidak diperlukan untuk menjalankan demo; kosongkan semua variabel SATUSEHAT jika tidak punya. Rincian dan batasannya ada di backend/README.md, bagian "Sinkronisasi SATUSEHAT MSI Sandbox".

## Pengujian

```powershell
gofmt -l cmd internal migrations seed tests
go build ./...
go vet ./...
go test ./...
```

Tes integrasi berbasis database bersifat opt-in lewat `TEST_MONGODB_URI` dan membuat database sementara sendiri; tes tidak menyentuh database `siagars`. Tes API Playwright ada di folder `qa/` (`npm ci`, lalu `npx playwright test` dengan `SIAGARS_BASE_URL`).

## Progres saat ini

Diisi sesuai bukti pada hari draft dibekukan. Status backend per 26 Oktober 2026 (dari backend/README.md, CHANGELOG.md dan HANDOVER_PRODUCTION_v0.2.1.md):

- [x] REST API Go + MongoDB Atlas, 22 operasi: pencarian RS (filter layanan AND, BPJS, wilayah, bed, sort nama/jarak/kecocokan, pagination), detail, compare 2-3 RS, autocomplete (`/hospitals/suggest`), master layanan dan wilayah (`/regions`)
- [x] Provenance per item (asal data, waktu sumber, waktu sinkronisasi, verifikasi), kesegaran fresh/aging/stale/unknown, dan `facility_summary` untuk badge UI
- [x] Tracking rujukan sintetis dengan state machine dan riwayat publik
- [x] Auth operator (session, CSRF, Origin, scope RS), dashboard RS dalam scope, update status rujukan/layanan/bed dengan optimistic locking
- [x] Migration 001-005, seed idempotent, pipeline kurasi `curate`, dokumentasi OpenAPI (Scalar)
- [x] Adapter SATUSEHAT MSI Sandbox read-only: 516 RS Jabodetabek, ditambah data contoh fasilitas berlabel simulasi
- [x] Cek kepesertaan BPJS **simulasi** (peserta sintetis) + riwayat operator terenkripsi, TTL 30 hari (API 0.3.0, staging; belum dirilis ke production)
- [x] Benchmark kinerja pencarian backend
- [x] Deployment Vercel production (API 0.2.1) dan staging `develop` (API 0.3.0): uji publik 32 pemeriksaan dan login/mutasi operator dari origin FE 15 pemeriksaan lulus di production (25 Okt 2026)
- [ ] Frontend Next.js: **[ISI: landing, pencarian, detail, lacak rujukan, cek BPJS, operator; centang hanya yang sudah berjalan]**
- [ ] Uji end-to-end UI frontend dengan backend (belum ada bukti di repo ini)
- [ ] Suite `qa/` terhadap production/staging (belum dijalankan)
- [ ] Kurasi data RS nyata bersumber, termasuk layanan/bed/BPJS (pipeline ada, data belum)
- [ ] Docker Compose dan race detector (belum teruji)

## Sumber data dan batasan

- Seed berisi 30 RS fiktif ("RS Simulasi Siaga 01..30") yang selalu berlabel `simulation`.
- Data dari SATUSEHAT MSI Sandbox berlabel `satusehat_sandbox`; itu data Sandbox, bukan data produksi, dan tidak memuat layanan, bed, atau BPJS. Layanan/bed/BPJS pada RS tersebut adalah **data contoh** berlabel `simulation` (`facility_summary.data_origin`, `bpjs_data_origin`), bukan fakta RS.
- Nilai yang tidak diketahui ditampilkan sebagai tidak diketahui (null), bukan nol.
- Cek BPJS hanya mencocokkan ke peserta sintetis (NIK fiktif berawalan `9999`). Input pengguna dicatat tersamar dan terenkripsi untuk riwayat operator lalu dihapus otomatis setelah 30 hari; jangan memasukkan NIK asli.
- Tidak ada integrasi produksi SATUSEHAT, FHIR, BPJS Kesehatan, maupun rujukan resmi.

## Dokumen

- `backend/PRD.md` - kebutuhan dan kontrak backend
- `backend/docs/openapi.yaml` - kontrak API
- `backend/docs/DECISIONS.md` - keputusan implementasi
- `backend/docs/CHANGELOG.md` - riwayat versi API untuk frontend
- `backend/docs/archive/API_FRONTEND.md` - panduan integrasi frontend
- `backend/deploy/VERCEL.md` - deployment production dan staging

## Lisensi

**[ISI: belum ditentukan di dokumen. Hak cipta karya tetap milik peserta menurut Guidebook; pilih lisensi atau tulis "semua hak dilindungi".]**
